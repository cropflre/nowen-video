package emby

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== PlaybackInfo ====================
//
// Infuse / Emby 在真正播放之前，会调用 `/Items/{id}/PlaybackInfo` 获取：
//   - 所有可用的 MediaSources（直通/remux/HLS）
//   - 每个 source 的 MediaStreams（视频/音频/字幕轨道）
//   - 一个 PlaySessionId（随后上报进度时携带）
//
// 官方 Emby 同时支持 GET（query string）和 POST（body 含 DeviceProfile）。

// PlaybackInfoRequest Emby 官方 POST body 字段（只保留我们用到的）。
type PlaybackInfoRequest struct {
	UserId               string `json:"UserId"`
	MaxStreamingBitrate  int    `json:"MaxStreamingBitrate"`
	StartTimeTicks       int64  `json:"StartTimeTicks"`
	AudioStreamIndex     int    `json:"AudioStreamIndex"`
	SubtitleStreamIndex  int    `json:"SubtitleStreamIndex"`
	MediaSourceId        string `json:"MediaSourceId"`
	LiveStreamId         string `json:"LiveStreamId"`
	EnableDirectPlay     bool   `json:"EnableDirectPlay"`
	EnableDirectStream   bool   `json:"EnableDirectStream"`
	EnableTranscoding    bool   `json:"EnableTranscoding"`
	AllowVideoStreamCopy bool   `json:"AllowVideoStreamCopy"`
	AllowAudioStreamCopy bool   `json:"AllowAudioStreamCopy"`
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

	maxStreamingBitrate := 0
	startTimeTicks := int64(0)
	if c.Request.Method == http.MethodPost && c.Request.ContentLength > 0 {
		var req PlaybackInfoRequest
		_ = c.ShouldBindJSON(&req)
		maxStreamingBitrate = req.MaxStreamingBitrate
		startTimeTicks = req.StartTimeTicks
	}
	if maxStreamingBitrate <= 0 {
		maxStreamingBitrate = firstPositiveIntQuery(
			c,
			"MaxStreamingBitrate",
			"maxStreamingBitrate",
			"maxBitrate",
		)
	}
	if startTimeTicks <= 0 {
		startTimeTicks = firstInt64Query(c, "StartTimeTicks", "startTimeTicks", "starttimeticks")
	}

	src := h.buildMediaSource(m, c)
	playSessionID := newSessionID(c.GetString("user_id"), c.GetString("emby_device_id"))

	// A transcode-capable MediaSource must always expose a concrete URL. The
	// URL carries the external PlaySessionId and requested start position so
	// master.m3u8, progress reports and stop events resolve the same internal
	// Playback Session instead of a media-level shared task.
	if src.SupportsTranscoding {
		if src.TranscodingUrl == "" {
			src.TranscodingUrl = fmt.Sprintf("/Videos/%s/master.m3u8", h.idMap.ToEmbyID(m.ID))
			src.TranscodingSubProtocol = "hls"
			src.TranscodingContainer = "ts"
		}
		values := url.Values{}
		values.Set("PlaySessionId", playSessionID)
		if startTimeTicks > 0 {
			values.Set("StartTimeTicks", strconv.FormatInt(startTimeTicks, 10))
		}
		if maxStreamingBitrate > 0 {
			values.Set("maxBitrate", strconv.Itoa(maxStreamingBitrate))
			src.Bitrate = maxStreamingBitrate
		}
		if token, _ := extractToken(c); token != "" {
			values.Set("api_key", token)
		}
		src.TranscodingUrl = appendPlaybackQuery(src.TranscodingUrl, values)
	}

	c.JSON(http.StatusOK, PlaybackInfoResponse{
		MediaSources:  []MediaSourceInfo{src},
		PlaySessionId: playSessionID,
	})
}

func appendPlaybackQuery(raw string, values url.Values) string {
	if raw == "" || len(values) == 0 {
		return raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		separator := "?"
		if strings.Contains(raw, "?") {
			separator = "&"
		}
		return raw + separator + values.Encode()
	}
	query := parsed.Query()
	for key, entries := range values {
		query.Del(key)
		for _, value := range entries {
			query.Add(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func firstPositiveIntQuery(c *gin.Context, names ...string) int {
	for _, name := range names {
		if value := atoiSafe(c.Query(name)); value > 0 {
			return value
		}
	}
	return 0
}

func firstInt64Query(c *gin.Context, names ...string) int64 {
	for _, name := range names {
		raw := strings.TrimSpace(c.Query(name))
		if raw == "" {
			continue
		}
		value, err := strconv.ParseInt(raw, 10, 64)
		if err == nil {
			return value
		}
	}
	return 0
}
