package emby

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== Playback 会话上报 ====================
//
// Emby 客户端在播放过程中会周期性上报进度：
//
//   POST /Sessions/Playing             - 播放开始
//   POST /Sessions/Playing/Progress    - 播放进度（通常每 5~10 秒一次）
//   POST /Sessions/Playing/Stopped     - 播放停止
//
// 这些请求体字段繁多，但我们只关心：
//   - ItemId / MediaSourceId        （哪条媒体）
//   - PositionTicks                 （播放位置，1tick=100ns）
//   - PlaySessionId                 （会话标识，暂不持久化）
//
// 所有上报都映射到 WatchHistoryRepo.Upsert。

// playbackReport 是所有三个上报接口共用的字段子集（用 Emby 的 PascalCase 命名）。
type playbackReport struct {
	ItemId           string `json:"ItemId"`
	MediaSourceId    string `json:"MediaSourceId"`
	PositionTicks    int64  `json:"PositionTicks"`
	PlaybackRate     float64 `json:"PlaybackRate"`
	IsPaused         bool   `json:"IsPaused"`
	IsMuted          bool   `json:"IsMuted"`
	EventName        string `json:"EventName"`
	PlaySessionId    string `json:"PlaySessionId"`
	VolumeLevel      int    `json:"VolumeLevel"`
	CanSeek          bool   `json:"CanSeek"`
}

// PlayingStartHandler 对应 POST /Sessions/Playing。
func (h *Handler) PlayingStartHandler(c *gin.Context) {
	h.recordProgress(c, false)
}

// PlayingProgressHandler 对应 POST /Sessions/Playing/Progress。
func (h *Handler) PlayingProgressHandler(c *gin.Context) {
	h.recordProgress(c, false)
}

// PlayingStoppedHandler 对应 POST /Sessions/Playing/Stopped。
// 与 Progress 相比，需要把"已完成"判定写入 completed 字段。
func (h *Handler) PlayingStoppedHandler(c *gin.Context) {
	h.recordProgress(c, true)
}

// recordProgress 将上报映射到 WatchHistory。
// isStop=true 时，如果进度超过 90% 则标记为 completed。
func (h *Handler) recordProgress(c *gin.Context, isStop bool) {
	var rpt playbackReport
	if err := c.ShouldBindJSON(&rpt); err != nil {
		// Emby 的上报有时是空 body（尤其是 Start），这时也直接 204
		c.Status(http.StatusNoContent)
		return
	}
	userID := c.GetString("user_id")
	if userID == "" || rpt.ItemId == "" {
		c.Status(http.StatusNoContent)
		return
	}
	uuid := h.idMap.Resolve(rpt.ItemId)
	if uuid == "" {
		c.Status(http.StatusNoContent)
		return
	}

	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}

	position := ticksToSeconds(rpt.PositionTicks)
	duration := m.Duration
	if duration <= 0 && m.Runtime > 0 {
		duration = float64(m.Runtime) * 60
	}

	completed := false
	if isStop && duration > 0 && position/duration >= 0.9 {
		completed = true
	}

	hist := &model.WatchHistory{
		UserID:    userID,
		MediaID:   uuid,
		Position:  position,
		Duration:  duration,
		Completed: completed,
		UpdatedAt: time.Now(),
	}
	if err := h.watchRepo.Upsert(hist); err != nil {
		h.logger.Warnf("[emby] upsert watch history failed user=%s media=%s err=%v", userID, uuid, err)
	}
	// Throttling：把播放位置喂给所有正在运行的 transcode job
	if h.transcode != nil && !isStop {
		h.transcode.SetPlaybackPosition(uuid, position)
	}
	c.Status(http.StatusNoContent)
}

// ==================== GET 形式（部分客户端走 GET /PlayingItems/{id}） ====================

// PlayingGetStartHandler 对应 GET /Users/{userId}/PlayingItems/{itemId}
//   - 某些旧版 Emby 客户端会发 GET；仅起"标记开始"作用。
func (h *Handler) PlayingGetStartHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("itemId"))
	if userID == "" || uuid == "" {
		c.Status(http.StatusNoContent)
		return
	}
	if m, err := h.mediaRepo.FindByID(uuid); err == nil {
		_ = h.watchRepo.Upsert(&model.WatchHistory{
			UserID:    userID,
			MediaID:   uuid,
			Duration:  m.Duration,
			UpdatedAt: time.Now(),
		})
	}
	c.Status(http.StatusNoContent)
}

// PlayingGetProgressHandler 对应 GET /Users/{userId}/PlayingItems/{itemId}/Progress
// 从 query 读取 PositionTicks。
func (h *Handler) PlayingGetProgressHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("itemId"))
	if userID == "" || uuid == "" {
		c.Status(http.StatusNoContent)
		return
	}
	ticks, _ := strconv.ParseInt(c.Query("PositionTicks"), 10, 64)
	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	position := ticksToSeconds(ticks)
	duration := m.Duration
	if duration <= 0 && m.Runtime > 0 {
		duration = float64(m.Runtime) * 60
	}
	_ = h.watchRepo.Upsert(&model.WatchHistory{
		UserID:    userID,
		MediaID:   uuid,
		Position:  position,
		Duration:  duration,
		UpdatedAt: time.Now(),
	})
	if h.transcode != nil {
		h.transcode.SetPlaybackPosition(uuid, position)
	}
	c.Status(http.StatusNoContent)
}

// PlayingGetStoppedHandler 对应 DELETE /Users/{userId}/PlayingItems/{itemId}
func (h *Handler) PlayingGetStoppedHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	uuid := h.idMap.Resolve(c.Param("itemId"))
	if userID == "" || uuid == "" {
		c.Status(http.StatusNoContent)
		return
	}
	ticks, _ := strconv.ParseInt(c.Query("PositionTicks"), 10, 64)
	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.Status(http.StatusNoContent)
		return
	}
	position := ticksToSeconds(ticks)
	duration := m.Duration
	if duration <= 0 && m.Runtime > 0 {
		duration = float64(m.Runtime) * 60
	}
	completed := duration > 0 && position/duration >= 0.9
	_ = h.watchRepo.Upsert(&model.WatchHistory{
		UserID:    userID,
		MediaID:   uuid,
		Position:  position,
		Duration:  duration,
		Completed: completed,
		UpdatedAt: time.Now(),
	})
	c.Status(http.StatusNoContent)
}
