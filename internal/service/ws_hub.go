package service

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// ==================== 事件类型常量 ====================

const (
	// 扫描事件
	EventScanStarted   = "scan_started"   // 扫描开始
	EventScanProgress  = "scan_progress"  // 扫描进度（发现新文件）
	EventScanCompleted = "scan_completed" // 扫描完成
	EventScanFailed    = "scan_failed"    // 扫描失败

	// 刮削事件
	EventScrapeStarted   = "scrape_started"   // 刮削开始
	EventScrapeProgress  = "scrape_progress"  // 刮削进度
	EventScrapeCompleted = "scrape_completed" // 刮削完成

	// 转码事件
	EventTranscodeStarted   = "transcode_started"   // 转码开始
	EventTranscodeProgress  = "transcode_progress"  // 转码进度
	EventTranscodeCompleted = "transcode_completed" // 转码完成
	EventTranscodeFailed    = "transcode_failed"    // 转码失败
)

// WSEvent WebSocket事件消息
type WSEvent struct {
	Type      string      `json:"type"`      // 事件类型
	Data      interface{} `json:"data"`      // 事件数据
	Timestamp int64       `json:"timestamp"` // 时间戳（毫秒）
}

// ScanProgressData 扫描进度数据
type ScanProgressData struct {
	LibraryID   string `json:"library_id"`
	LibraryName string `json:"library_name"`
	Phase       string `json:"phase"`     // scanning / scraping
	Current     int    `json:"current"`   // 当前处理数
	Total       int    `json:"total"`     // 总数（刮削时有值）
	NewFound    int    `json:"new_found"` // 新发现的文件数
	Message     string `json:"message"`   // 描述信息
}

// ScrapeProgressData 刮削进度数据
type ScrapeProgressData struct {
	LibraryID   string `json:"library_id"`
	LibraryName string `json:"library_name"`
	Current     int    `json:"current"`     // 当前第几个
	Total       int    `json:"total"`       // 总数
	Success     int    `json:"success"`     // 成功数
	Failed      int    `json:"failed"`      // 失败数
	MediaTitle  string `json:"media_title"` // 当前正在刮削的媒体
	Message     string `json:"message"`
}

// TranscodeProgressData 转码进度数据
type TranscodeProgressData struct {
	TaskID   string  `json:"task_id"`
	MediaID  string  `json:"media_id"`
	Title    string  `json:"title"`
	Quality  string  `json:"quality"`
	Progress float64 `json:"progress"` // 0-100
	Speed    string  `json:"speed"`    // 转码速度，如 "2.5x"
	Message  string  `json:"message"`
}

// ==================== WebSocket 客户端 ====================

const (
	writeWait      = 10 * time.Second    // 写入超时
	pongWait       = 60 * time.Second    // 等待pong超时
	pingPeriod     = (pongWait * 9) / 10 // ping间隔（pongWait的90%）
	maxMessageSize = 512                 // 最大消息大小
)

// WSClient WebSocket客户端
type WSClient struct {
	hub    *WSHub
	conn   *websocket.Conn
	send   chan []byte
	userID string
}

// readPump 从WebSocket读取消息
func (c *WSClient) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

// writePump 向WebSocket写入消息
func (c *WSClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub关闭了channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// 批量发送队列中的消息
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ==================== WebSocket Hub ====================

// WSHub WebSocket连接管理中心
type WSHub struct {
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
	logger     *zap.SugaredLogger
}

// NewWSHub 创建WebSocket Hub
func NewWSHub(logger *zap.SugaredLogger) *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		logger:     logger,
	}
}

// Run 启动Hub（在goroutine中运行）
func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.logger.Debugf("WebSocket客户端连接，当前在线: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			h.logger.Debugf("WebSocket客户端断开，当前在线: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					// 发送缓冲满，断开该客户端
					close(client.send)
					delete(h.clients, client)
				}
			}
			h.mu.RUnlock()
		}
	}
}

// BroadcastEvent 广播事件到所有连接的客户端
func (h *WSHub) BroadcastEvent(eventType string, data interface{}) {
	event := WSEvent{
		Type:      eventType,
		Data:      data,
		Timestamp: time.Now().UnixMilli(),
	}

	msg, err := json.Marshal(event)
	if err != nil {
		h.logger.Warnf("序列化WebSocket事件失败: %v", err)
		return
	}

	select {
	case h.broadcast <- msg:
	default:
		h.logger.Warn("WebSocket广播通道已满，丢弃事件")
	}
}

// ClientCount 获取当前连接的客户端数量
func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

// RegisterClient 注册新的WebSocket客户端连接
func (h *WSHub) RegisterClient(conn *websocket.Conn, userID string) {
	client := &WSClient{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
	}

	h.register <- client

	// 启动读写协程
	go client.writePump()
	go client.readPump()
}
