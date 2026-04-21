package emby

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// ==================== PlaybackInfo ====================
//
// Infuse / Emby 在真正播放之前，会调用 `/Items/{id}/PlaybackInfo` 获取：
//   - 所有可用的 MediaSources（直通/remux/HLS）
//   - 每个 source 的 MediaStreams（视频/音频/字幕轨道）
//   - 一个 PlaySessionId（随后上报进度时携带）
//
// 官方 Emby 同时支持 GET（query string）和 POST（body 含 DeviceProfile），
// 我们不解析 DeviceProfile——直接把 MediaSource 全部开放，由客户端自行挑选。

// PlaybackInfoRequest Emby 官方 POST body 字段（只保留我们用到的）。
type PlaybackInfoRequest struct {
	UserId              string `json:"UserId"`
	MaxStreamingBitrate int    `json:"MaxStreamingBitrate"`
	StartTimeTicks      int64  `json:"StartTimeTicks"`
	AudioStreamIndex    int    `json:"AudioStreamIndex"`
	SubtitleStreamIndex int    `json:"SubtitleStreamIndex"`
	MediaSourceId       string `json:"MediaSourceId"`
	LiveStreamId        string `json:"LiveStreamId"`
	EnableDirectPlay    bool   `json:"EnableDirectPlay"`
	EnableDirectStream  bool   `json:"EnableDirectStream"`
	EnableTranscoding   bool   `json:"EnableTranscoding"`
	AllowVideoStreamCopy bool  `json:"AllowVideoStreamCopy"`
	AllowAudioStreamCopy bool  `json:"AllowAudioStreamCopy"`
}

// PlaybackInfoHandler 对应 GET/POST /Items/{id}/PlaybackInfo。
func (h *Handler) PlaybackInfoHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}

	m, err := h.mediaRepo.FindByID(uuid)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Media not found"})
		return
	}

	// 可选：把 body 吞掉，但不强行解析——不同客户端的字段差异很大。
	if c.Request.Method == http.MethodPost && c.Request.ContentLength > 0 {
		var req PlaybackInfoRequest
		_ = c.ShouldBindJSON(&req)
		_ = req // 暂时忽略 DeviceProfile 细节，按"服务端已 remux 好"的思路返回
	}

	src := h.buildMediaSource(m, c)
	// 标注一个随会话唯一的 PlaySessionId，客户端后续在上报进度时会原样回传。
	playSessionID := newSessionID(c.GetString("user_id"), c.GetString("emby_device_id"))

	c.JSON(http.StatusOK, PlaybackInfoResponse{
		MediaSources:  []MediaSourceInfo{src},
		PlaySessionId: playSessionID,
	})
}
