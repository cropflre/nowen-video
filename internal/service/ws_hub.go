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
	EventScanPhase     = "scan_phase"     // 扫描阶段变更（多步骤流程通知）

	// 刮削事件
	EventScrapeStarted   = "scrape_started"   // 刮削开始
	EventScrapeProgress  = "scrape_progress"  // 刮削进度
	EventScrapeCompleted = "scrape_completed" // 刮削完成

	// 转码事件
	EventTranscodeStarted   = "transcode_started"   // 转码开始
	EventTranscodeProgress  = "transcode_progress"  // 转码进度
	EventTranscodeCompleted = "transcode_completed" // 转码完成
	EventTranscodeFailed    = "transcode_failed"    // 转码失败

	// 媒体库变更事件
	EventLibraryDeleted = "library_deleted" // 媒体库被删除
	EventLibraryUpdated = "library_updated" // 媒体库内容有变更

	// 注意：预处理事件常量定义在 preprocess.go 中
	// EventPreprocessStarted / EventPreprocessProgress / EventPreprocessCompleted / EventPreprocessFailed / EventPreprocessPaused / EventPreprocessCancelled
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
	Phase       string `json:"phase"`     // scanning / scraping / cleaning
	Current     int    `json:"current"`   // 当前处理数
	Total       int    `json:"total"`     // 总数
	NewFound    int    `json:"new_found"` // 新发现的文件数
	Cleaned     int    `json:"cleaned"`   // 清理的已删除文件数
	Message     string `json:"message"`   // 描述信息
}

// ScanPhaseData 扫描阶段变更数据（用于多步骤流程通知）
type ScanPhaseData struct {
	LibraryID   string `json:"library_id"`
	LibraryName string `json:"library_name"`
	Phase       string `json:"phase"`        // scanning / ai_organizing / scraping / merging / matching / completed
	StepCurrent int    `json:"step_current"` // 当前步骤序号（从1开始）
	StepTotal   int    `json:"step_total"`   // 总步骤数
	Current     int    `json:"current"`      // 当前阶段进度（可选）
	Total       int    `json:"total"`        // 当前阶段总量（可选）
	Message     string `json:"message"`      // 阶段描述信息
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

// LibraryChangedData 媒体库变更事件数据
type LibraryChangedData struct {
	LibraryID   string `json:"library_id"`
	LibraryName string `json:"library_name"`
	Action      string `json:"action"` // deleted / updated
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
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

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

// WSInternalObserver receives an in-process lifecycle event. Observers must be
// non-blocking; they should enqueue durable/background work instead of doing
// media processing on the broadcaster goroutine.
type WSInternalObserver func(WSEvent)

// ==================== WebSocket Hub ====================

type WSHub struct {
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex
	logger     *zap.SugaredLogger

	observerMu  sync.RWMutex
	observers   map[string]map[uint64]WSInternalObserver
	observerSeq uint64
}

func NewWSHub(logger *zap.SugaredLogger) *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		logger:     logger,
		observers:  make(map[string]map[uint64]WSInternalObserver),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			if h.logger != nil {
				h.logger.Debugf("WebSocket客户端连接，当前在线: %d", len(h.clients))
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			if h.logger != nil {
				h.logger.Debugf("WebSocket客户端断开，当前在线: %d", len(h.clients))
			}

		case message := <-h.broadcast:
			h.mu.RLock()
			var staleClients []*WSClient
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					staleClients = append(staleClients, client)
				}
			}
			h.mu.RUnlock()

			if len(staleClients) > 0 {
				h.mu.Lock()
				for _, client := range staleClients {
					if _, ok := h.clients[client]; ok {
						close(client.send)
						delete(h.clients, client)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

// SubscribeInternal registers an in-process observer for one exact event type.
// The returned function is idempotent and removes the observer.
func (h *WSHub) SubscribeInternal(eventType string, observer WSInternalObserver) func() {
	if h == nil || eventType == "" || observer == nil {
		return func() {}
	}
	h.observerMu.Lock()
	h.observerSeq++
	id := h.observerSeq
	if h.observers[eventType] == nil {
		h.observers[eventType] = make(map[uint64]WSInternalObserver)
	}
	h.observers[eventType][id] = observer
	h.observerMu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			h.observerMu.Lock()
			if listeners := h.observers[eventType]; listeners != nil {
				delete(listeners, id)
				if len(listeners) == 0 {
					delete(h.observers, eventType)
				}
			}
			h.observerMu.Unlock()
		})
	}
}

// BroadcastEvent broadcasts to external clients and synchronously notifies
// internal non-blocking observers. Task lifecycle mapping remains a second
// event with the same timestamp.
func (h *WSHub) BroadcastEvent(eventType string, data interface{}) {
	timestamp := time.Now().UnixMilli()
	h.publishEvent(WSEvent{Type: eventType, Data: data, Timestamp: timestamp})

	if update, ok := taskLifecycleUpdateForEvent(eventType, data); ok {
		h.publishEvent(WSEvent{Type: EventTaskUpdated, Data: update, Timestamp: timestamp})
	}
}

func (h *WSHub) publishEvent(event WSEvent) {
	h.dispatchInternal(event)
	h.enqueueEvent(event)
}

func (h *WSHub) dispatchInternal(event WSEvent) {
	if h == nil {
		return
	}
	h.observerMu.RLock()
	listeners := h.observers[event.Type]
	snapshot := make([]WSInternalObserver, 0, len(listeners))
	for _, observer := range listeners {
		snapshot = append(snapshot, observer)
	}
	h.observerMu.RUnlock()
	for _, observer := range snapshot {
		func(callback WSInternalObserver) {
			defer func() {
				if recovered := recover(); recovered != nil && h.logger != nil {
					h.logger.Errorf("内部事件订阅者 panic event=%s: %v", event.Type, recovered)
				}
			}()
			callback(event)
		}(observer)
	}
}

func (h *WSHub) enqueueEvent(event WSEvent) {
	msg, err := json.Marshal(event)
	if err != nil {
		if h.logger != nil {
			h.logger.Warnf("序列化WebSocket事件失败: %v", err)
		}
		return
	}

	select {
	case h.broadcast <- msg:
	default:
		if h.logger != nil {
			h.logger.Warn("WebSocket广播通道已满，丢弃事件")
		}
	}
}

func (h *WSHub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}

func (h *WSHub) RegisterClient(conn *websocket.Conn, userID string) {
	client := &WSClient{
		hub:    h,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
	}

	h.register <- client
	go client.writePump()
	go client.readPump()
}
