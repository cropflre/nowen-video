package emby

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== Sessions 列表 ====================

// SessionsHandler 对应 GET /Sessions。
// Emby 官方客户端登录后会请求 /Sessions?DeviceId=... 来验证会话是否建立。
// 返回当前用户当前设备的 active session，而不是空数组。
func (h *Handler) SessionsHandler(c *gin.Context) {
	userID := c.GetString("user_id")
	if userID == "" {
		c.JSON(http.StatusOK, []SessionInfo{})
		return
	}

	authHdr := parseEmbyAuthHeader(c.GetHeader("X-Emby-Authorization"))
	sess := SessionInfo{
		Id:                    newSessionID(userID, authHdr.DeviceId),
		UserId:                h.idMap.ToEmbyID(userID),
		UserName:              c.GetString("username"),
		Client:                authHdr.Client,
		DeviceName:            authHdr.Device,
		DeviceId:              authHdr.DeviceId,
		ApplicationVersion:    authHdr.Version,
		ServerId:              h.serverID,
		IsActive:              true,
		SupportsRemoteControl: true,
		PlayableMediaTypes:    []string{"Video", "Audio"},
		SupportedCommands:     []string{},
		LastActivityDate:      formatEmbyTime(nowUTC()),
	}

	c.JSON(http.StatusOK, []SessionInfo{sess})
}

// ==================== Playback 会话上报 ====================

type playbackReport struct {
	ItemId          string  `json:"ItemId"`
	MediaSourceId   string  `json:"MediaSourceId"`
	PositionTicks   int64   `json:"PositionTicks"`
	PlaybackRate    float64 `json:"PlaybackRate"`
	IsPaused        bool    `json:"IsPaused"`
	IsMuted         bool    `json:"IsMuted"`
	EventName       string  `json:"EventName"`
	PlaySessionId   string  `json:"PlaySessionId"`
	VolumeLevel     int     `json:"VolumeLevel"`
	CanSeek         bool    `json:"CanSeek"`
}

func (h *Handler) PlayingStartHandler(c *gin.Context) {
	h.recordProgress(c, false)
}

func (h *Handler) PlayingProgressHandler(c *gin.Context) {
	h.recordProgress(c, false)
}

func (h *Handler) PlayingStoppedHandler(c *gin.Context) {
	h.recordProgress(c, true)
}

// recordProgress writes WatchHistory and, when the playback is runtime HLS,
// drives the exact internal Playback Session selected by PlaySessionId. It no
// longer broadcasts a media-level position to every user playing the same item.
func (h *Handler) recordProgress(c *gin.Context, isStop bool) {
	var rpt playbackReport
	if err := c.ShouldBindJSON(&rpt); err != nil {
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
	completed := isStop && duration > 0 && position/duration >= 0.9

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

	if runtime := h.playbackSessionRuntime(); runtime != nil {
		if mapping, ok := runtime.find(userID, rpt.PlaySessionId, uuid); ok {
			positionMS := int64(position * 1000)
			if isStop {
				closeCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				if err := runtime.close(closeCtx, mapping, "emby_playback_stopped"); err != nil {
					h.logger.Debugf("[emby] playback session close failed play_session=%s err=%v", mapping.ExternalID, err)
				}
				cancel()
			} else if err := runtime.heartbeat(c.Request.Context(), mapping, positionMS, rpt.IsPaused); err != nil {
				h.logger.Debugf("[emby] playback session heartbeat failed play_session=%s err=%v", mapping.ExternalID, err)
			}
			c.Status(http.StatusNoContent)
			return
		}
	}

	// Compatibility fallback for direct/remux playback and older servers.
	if h.transcode != nil && !isStop {
		h.transcode.SetPlaybackPosition(uuid, position)
	}
	c.Status(http.StatusNoContent)
}

// ==================== GET 形式（部分客户端走 GET /PlayingItems/{id}） ====================

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
	if runtime := h.playbackSessionRuntime(); runtime != nil {
		if mapping, ok := runtime.find(userID, firstQuery(c, "PlaySessionId", "playSessionId"), uuid); ok {
			_ = runtime.heartbeat(c.Request.Context(), mapping, mapping.LastPosition, false)
		}
	}
	c.Status(http.StatusNoContent)
}

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

	if runtime := h.playbackSessionRuntime(); runtime != nil {
		if mapping, ok := runtime.find(userID, firstQuery(c, "PlaySessionId", "playSessionId"), uuid); ok {
			if err := runtime.heartbeat(c.Request.Context(), mapping, int64(position*1000), false); err != nil {
				h.logger.Debugf("[emby] GET progress heartbeat failed play_session=%s err=%v", mapping.ExternalID, err)
			}
			c.Status(http.StatusNoContent)
			return
		}
	}
	if h.transcode != nil {
		h.transcode.SetPlaybackPosition(uuid, position)
	}
	c.Status(http.StatusNoContent)
}

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

	if runtime := h.playbackSessionRuntime(); runtime != nil {
		if mapping, ok := runtime.find(userID, firstQuery(c, "PlaySessionId", "playSessionId"), uuid); ok {
			closeCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			if err := runtime.close(closeCtx, mapping, "emby_playing_item_stopped"); err != nil {
				h.logger.Debugf("[emby] GET stop close failed play_session=%s err=%v", mapping.ExternalID, err)
			}
			cancel()
		}
	}
	c.Status(http.StatusNoContent)
}
