package emby

// websocket.go 实现 Emby/Jellyfin 客户端的 WebSocket 通道。
//
// 为什么需要这个端点？
//   Emby/Jellyfin 官方客户端（iOS/Android/Web）在登录成功后会尝试与服务器建立
//   WebSocket 长连接（路径固定为 `/embywebsocket` 或 `/socket`），用于：
//     - 接收服务器推送（新媒体上架、播放控制）
//     - 远程控制（把 iPad 当遥控器控制 Apple TV 播放）
//     - 心跳保活
//
//   如果服务器不提供该端点，客户端会反复重试并在日志里刷错误，但不影响登录和播放。
//   实现一个最小可用的 echo/keepalive server 能消除这些告警，并为将来支持远程控制打基础。

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/websocket"
	"github.com/nowen-video/nowen-video/internal/middleware"
	"go.uber.org/zap"
)

// embyWSUpgrader 允许任意来源（原生客户端不走浏览器同源）。
var embyWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// wsClient 一个已连接的 WebSocket 客户端。
type wsClient struct {
	conn     *websocket.Conn
	userID   string
	deviceID string
	sendMu   sync.Mutex
}

// WSHub 管理所有活跃的 Emby WebSocket 连接。
// 目前只做心跳保活；后续可在 Hub 上挂播放状态 / 远程控制路由。
type WSHub struct {
	logger *zap.SugaredLogger

	mu      sync.RWMutex
	clients map[*wsClient]struct{}
}

// NewWSHub 构造 WebSocket 集线器。
func NewWSHub(logger *zap.SugaredLogger) *WSHub {
	return &WSHub{
		logger:  logger,
		clients: make(map[*wsClient]struct{}),
	}
}

// Handler 返回 gin.HandlerFunc，用于挂到 /embywebsocket 等路径。
//
// 鉴权：客户端在 URL query 中携带 ?api_key=<JWT>&deviceId=<id>。
// 如果 token 无效，升级前返回 401。
func (h *WSHub) Handler(jwtSecret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 从 query / header 提取 token（WebSocket 阶段客户端只会给 query）
		tokenStr := c.Query("api_key")
		if tokenStr == "" {
			tokenStr = c.Query("ApiKey")
		}
		if tokenStr == "" {
			tokenStr = c.Query("X-Emby-Token")
		}
		if tokenStr == "" {
			// 最后尝试 header
			tokenStr, _ = extractToken(c)
		}
		if tokenStr == "" {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		claims := &middleware.Claims{}
		token, err := jwt.ParseWithClaims(tokenStr, claims, func(token *jwt.Token) (interface{}, error) {
			return []byte(jwtSecret), nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatus(http.StatusUnauthorized)
			return
		}

		// 升级协议
		conn, err := embyWSUpgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			if h.logger != nil {
				h.logger.Debugf("[emby-ws] upgrade 失败: %v", err)
			}
			return
		}

		client := &wsClient{
			conn:     conn,
			userID:   claims.UserID,
			deviceID: c.Query("deviceId"),
		}
		h.addClient(client)
		defer h.removeClient(client)

		if h.logger != nil {
			h.logger.Debugf("[emby-ws] 客户端已连接 user=%s device=%s", client.userID, client.deviceID)
		}

		// 发送一条 ForceKeepAlive 消息，Emby 客户端以此确认 WS 可用
		_ = client.writeJSON(gin.H{
			"MessageType": "ForceKeepAlive",
			"Data":        60,
		})

		h.readLoop(client)
	}
}

// addClient 向 hub 注册。
func (h *WSHub) addClient(c *wsClient) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

// removeClient 从 hub 移除并关闭底层连接。
func (h *WSHub) removeClient(c *wsClient) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
	_ = c.conn.Close()
}

// readLoop 持续接收客户端消息：
//   - KeepAlive:      回 KeepAlive
//   - 其他业务消息：  目前仅记录日志；将来可在此接入远程控制
func (h *WSHub) readLoop(c *wsClient) {
	// 设置读超时 + Pong 续期
	c.conn.SetReadLimit(1 << 20) // 1 MiB
	_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		return nil
	})

	// 心跳 ticker：30s 发一次 ping
	pingDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.sendMu.Lock()
				_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := c.conn.WriteMessage(websocket.PingMessage, nil)
				c.sendMu.Unlock()
				if err != nil {
					return
				}
			case <-pingDone:
				return
			}
		}
	}()
	defer close(pingDone)

	for {
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		if msgType != websocket.TextMessage {
			continue
		}
		// 简单解析 MessageType 字段处理心跳
		s := string(data)
		if contains(s, "KeepAlive") {
			_ = c.writeJSON(gin.H{"MessageType": "KeepAlive"})
			continue
		}
		// 其他消息：Identity / PlaybackStart / SessionsStart 等
		// 目前不做实际处理，仅维持连接。
		if h.logger != nil {
			h.logger.Debugf("[emby-ws] 收到消息 user=%s len=%d", c.userID, len(data))
		}
	}
}

// writeJSON 线程安全地写入 JSON 消息。
func (c *wsClient) writeJSON(v interface{}) error {
	c.sendMu.Lock()
	defer c.sendMu.Unlock()
	_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	return c.conn.WriteJSON(v)
}

// contains 避免引入 strings 包——本文件里只用这一处。
func contains(haystack, needle string) bool {
	if len(needle) == 0 {
		return true
	}
	n := len(needle)
	for i := 0; i+n <= len(haystack); i++ {
		if haystack[i:i+n] == needle {
			return true
		}
	}
	return false
}
