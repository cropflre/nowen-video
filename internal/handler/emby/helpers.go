package emby

import (
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/model"
)

// ==================== 基础时间/字符串工具 ====================

// nowUTC 返回 UTC 当前时间。
func nowUTC() time.Time { return time.Now().UTC() }

// newSessionID 生成一个稳定的 SessionId。
// Emby 客户端在长连接场景会把 SessionId 作为 WebSocket 消息路由的 key，
// 这里基于 userID+deviceID+nanoTime 生成 32 位 hex，足够唯一。
func newSessionID(userID, deviceID string) string {
	seed := fmt.Sprintf("%s|%s|%d", userID, deviceID, time.Now().UnixNano())
	sum := sha1.Sum([]byte(seed))
	return hex.EncodeToString(sum[:16]) // 取前 16 字节 → 32 hex
}

// itoa int → 字符串。
func itoa(n int) string { return strconv.Itoa(n) }

// atoiSafe 字符串 → int，失败返回 0。
func atoiSafe(s string) int {
	n, _ := strconv.Atoi(s)
	return n
}

// ==================== MediaSource 构造 ====================

// buildMediaSource 将 nowen 的 Media 转成 Emby `MediaSourceInfo`。
//
// 核心策略（对齐 Emby 官方行为）：
//   - 通过 UA 探测客户端能力，分情况返回：
//     * 兼容编码（H.264/AAC 等） → 只返 DirectStream/DirectPlay，不给 Transcoding URL，
//       强制客户端走 Remux 秒开路径；
//     * 不兼容编码 → 同时提供 DirectStream 与 Transcoding，允许客户端自行决策；
//   - `DirectStreamUrl` 指向 `/emby/Videos/{embyId}/stream`，StreamVideoHandler
//     会根据编码自动选择 RemuxStream 或 ServeContent；
//   - 对 STRM（远程流）条目，Protocol=Http，Infuse 当作在线源处理。
func (h *Handler) buildMediaSource(m *model.Media, c *gin.Context) MediaSourceInfo {
	container := containerFromPath(m.FilePath)
	isRemote := strings.TrimSpace(m.StreamURL) != ""

	// 构造 stream URL（相对 Host，不带 scheme——Infuse 会用连接时的 scheme）
	streamPath := fmt.Sprintf("/emby/Videos/%s/stream", h.idMap.ToEmbyID(m.ID))
	streamAbs := buildAbsoluteURL(c, streamPath)

	width := parseResolutionWidth(m.Resolution)
	height := parseResolutionHeight(m.Resolution)

	// ==================== 基于 UA 决策播放策略 ====================
	ua := ""
	if c != nil && c.Request != nil {
		ua = c.Request.Header.Get("User-Agent")
	}
	canRemux := false
	if !isRemote && h.stream != nil {
		canRemux = h.stream.ShouldRemux(m, ua)
	}

	src := MediaSourceInfo{
		Protocol:             "File",
		Id:                   h.idMap.ToEmbyID(m.ID),
		Path:                 m.FilePath,
		Type:                 "Default",
		Container:            container,
		Size:                 m.FileSize,
		Name:                 m.Title,
		IsRemote:             isRemote,
		RunTimeTicks:         secondsToTicks(m.Duration),
		SupportsDirectStream: true,
		SupportsDirectPlay:   !isRemote,
		SupportsProbing:      true,
		MediaStreams:         h.buildMediaStreams(m, width, height),
		Formats:              []string{},
		DirectStreamUrl:      streamAbs,
		ETag:                 shortTag(m.ID + "|" + m.FilePath + "|" + strconv.FormatInt(m.FileSize, 10)),
	}

	// 激进策略：兼容编码时关闭 Transcoding，强制走 Direct/Remux
	// 这是"秒开"的关键 —— 客户端收到只有 DirectStreamUrl 的 MediaSource 后，
	// 会直接 GET /Videos/:id/stream，StreamVideoHandler 判断是 mkv 就走 RemuxStream，
	// 首帧延迟等同于 FFmpeg -c copy 启动时间（~百毫秒）
	if canRemux {
		src.SupportsTranscoding = false
	} else {
		// 不兼容编码 → 开放 Transcoding 让客户端决定
		src.SupportsTranscoding = true
	}

	if isRemote {
		src.Protocol = "Http"
		src.Path = m.StreamURL
	}
	if src.RunTimeTicks == 0 && m.Runtime > 0 {
		src.RunTimeTicks = minutesToTicks(m.Runtime)
	}

	return src
}

// buildMediaStreams 根据 Media 已知的编码字段生成占位 MediaStreams。
// 精准的 MediaStreams 需要 ffprobe，这里只提供 Infuse/Emby 播放前展示所需的最小子集，
// 真实解码信息由客户端自己 probe。
func (h *Handler) buildMediaStreams(m *model.Media, width, height int) []MediaStream {
	out := make([]MediaStream, 0, 3)

	// Video
	videoCodec := strings.ToLower(strings.TrimSpace(m.VideoCodec))
	if videoCodec == "" {
		videoCodec = "h264"
	}
	out = append(out, MediaStream{
		Codec:                  videoCodec,
		Type:                   "Video",
		Index:                  0,
		IsDefault:              true,
		Width:                  width,
		Height:                 height,
		DisplayTitle:           displayVideoTitle(videoCodec, width, height),
		SupportsExternalStream: false,
	})

	// Audio
	audioCodec := strings.ToLower(strings.TrimSpace(m.AudioCodec))
	if audioCodec == "" {
		audioCodec = "aac"
	}
	out = append(out, MediaStream{
		Codec:                  audioCodec,
		Type:                   "Audio",
		Index:                  1,
		IsDefault:              true,
		Channels:               2,
		DisplayTitle:           strings.ToUpper(audioCodec),
		SupportsExternalStream: false,
	})

	// 外挂字幕（基于 media.SubtitlePaths，| 分隔）
	if m.SubtitlePaths != "" {
		for i, sp := range splitPaths(m.SubtitlePaths) {
			ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(sp)), ".")
			if ext == "" {
				ext = "srt"
			}
			out = append(out, MediaStream{
				Codec:                  ext,
				Type:                   "Subtitle",
				Index:                  2 + i,
				IsExternal:             true,
				IsTextSubtitleStream:   true,
				SupportsExternalStream: true,
				DisplayTitle:           filepath.Base(sp),
				DeliveryUrl:            "", // 待字幕接口实现后填充
			})
		}
	}
	return out
}

func displayVideoTitle(codec string, w, h int) string {
	if h > 0 {
		return fmt.Sprintf("%s %dp", strings.ToUpper(codec), h)
	}
	if w > 0 {
		return fmt.Sprintf("%s %dw", strings.ToUpper(codec), w)
	}
	return strings.ToUpper(codec)
}

// splitPaths 按 | 切分并去空白。
func splitPaths(raw string) []string {
	parts := strings.Split(raw, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// buildAbsoluteURL 结合当前请求上下文构造绝对 URL。
// Infuse 客户端读取 DirectStreamUrl 时允许相对路径，但若用户配置了反代/端口，
// 返回绝对 URL 更稳妥。
func buildAbsoluteURL(c *gin.Context, p string) string {
	scheme := "http"
	if c.Request != nil && c.Request.TLS != nil {
		scheme = "https"
	} else if xf := c.GetHeader("X-Forwarded-Proto"); xf != "" {
		scheme = xf
	}
	host := c.GetHeader("X-Forwarded-Host")
	if host == "" && c.Request != nil {
		host = c.Request.Host
	}
	if host == "" {
		// 没有 Host 时退化成相对路径
		return p
	}
	u := &url.URL{Scheme: scheme, Host: host, Path: p}
	return u.String()
}

// parseRangeHeader（预留：用于将来给 Range 请求打点统计）。
// 当前未使用，但保留给后续 metrics 模块调用。
// func parseRangeHeader(r string) bool {
// 	return r != "" && strings.HasPrefix(strings.ToLower(r), "bytes=")
// }
