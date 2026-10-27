package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// WebSocket升级器
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true // 允许所有来源（开发阶段）
	},
}

// WSHandler WebSocket处理器
type WSHandler struct {
	users  *service.UserService
	hub    *service.WSHub
	logger *zap.SugaredLogger
}

// HandleWebSocket 处理WebSocket连接请求
func (h *WSHandler) HandleWebSocket(c *gin.Context) {
	// 获取用户信息（已通过JWT中间件验证）
	userID, _ := c.Get("user_id")

	// Check again on delivery: a pre-existing adult socket must not survive a
	// parent's conversion of the account into a child account.
	uid, _ := userID.(string)
	if h.users == nil {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	initial, err := h.users.GetProfile(uid)
	if err != nil || initial.ChildMode || initial.Disabled {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	authorize := func() bool {
		current, err := h.users.GetProfile(uid)
		return err == nil && !current.ChildMode && !current.Disabled && current.TokenVersion == initial.TokenVersion
	}
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		h.logger.Errorf("WebSocket升级失败: %v", err)
		return
	}

	h.hub.RegisterClient(conn, uid, authorize)
	h.logger.Debugf("新的WebSocket连接: user=%s", uid)
}
