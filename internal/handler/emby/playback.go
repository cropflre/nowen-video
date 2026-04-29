package emby

import (
	"fmt"
	"net/http"
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

	// 解析客户端请求，主要是为了拿到 MaxStreamingBitrate —— 决定我们给前端返回
	// 怎样的 TranscodingUrl 上限；GET 走 query string，POST 走 JSON body。
	maxStreamingBitrate := 0
	if c.Request.Method == http.MethodPost && c.Request.ContentLength > 0 {
		var req PlaybackInfoRequest
		_ = c.ShouldBindJSON(&req)
		maxStreamingBitrate = req.MaxStreamingBitrate
	}
	if maxStreamingBitrate <= 0 {
		// 某些客户端（Infuse/老版 Emby Theater）只在 query 里传
		if v := c.Query("MaxStreamingBitrate"); v != "" {
			maxStreamingBitrate = atoiSafe(v)
		}
		if maxStreamingBitrate <= 0 {
			if v := c.Query("maxStreamingBitrate"); v != "" {
				maxStreamingBitrate = atoiSafe(v)
			}
		}
	}

	src := h.buildMediaSource(m, c)

	// 根据客户端声明的 MaxStreamingBitrate 约束 MediaSource：
	//   1. 设置 src.Bitrate，Infuse/hls.js 会参考此上限挑选轨道；
	//   2. TranscodingUrl 带上 ?maxBitrate=xxx，服务端生成的 master.m3u8 只会
	//      包含不超过此上限的档位，避免客户端在 MOBILE 网络上挑到 1080p；
	//   3. 如果原始码率已低于上限，不做额外处理（让 DirectStream 走起来）。
	if maxStreamingBitrate > 0 {
		src.Bitrate = maxStreamingBitrate
		// 追加 TranscodingUrl —— 对应 /emby/Videos/{id}/master.m3u8?maxBitrate=xxx
		// 若 buildMediaSource 未生成 TranscodingUrl（兼容直通场景），则跳过
		if src.TranscodingUrl == "" {
			src.TranscodingUrl = buildAbsoluteURL(c,
				fmt.Sprintf("/emby/Videos/%s/master.m3u8?maxBitrate=%d",
					h.idMap.ToEmbyID(m.ID), maxStreamingBitrate))
			src.TranscodingSubProtocol = "hls"
			src.TranscodingContainer = "ts"
			src.SupportsTranscoding = true
		} else {
			// 已有 URL 时追加 maxBitrate 参数
			sep := "?"
			if strings.Contains(src.TranscodingUrl, "?") {
				sep = "&"
			}
			src.TranscodingUrl = src.TranscodingUrl + sep + fmt.Sprintf("maxBitrate=%d", maxStreamingBitrate)
		}
	}

	// 标注一个随会话唯一的 PlaySessionId，客户端后续在上报进度时会原样回传。
	playSessionID := newSessionID(c.GetString("user_id"), c.GetString("emby_device_id"))

	c.JSON(http.StatusOK, PlaybackInfoResponse{
		MediaSources:  []MediaSourceInfo{src},
		PlaySessionId: playSessionID,
	})
}
