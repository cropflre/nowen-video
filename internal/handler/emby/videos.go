package emby

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

// ==================== Video 流接口 ====================
//
// Infuse / Emby 客户端真正发起播放时会对下列 URL 发 GET（或 HEAD）请求：
//
//   GET /emby/Videos/{itemId}/stream                     -- 直通/Remux
//   GET /emby/Videos/{itemId}/stream.{container}         -- 同上（部分客户端加扩展名）
//   GET /emby/Videos/{itemId}/master.m3u8                -- HLS master
//   GET /emby/Videos/{itemId}/hls1/main/{segment}.ts     -- HLS 分片
//
// 为了兼容多种客户端，以下三种变体均映射到同一套 handler。
// 认证方式：
//   - X-Emby-Token 头（iOS/Android Emby App）
//   - api_key query（Infuse 会把 token 直接拼到 URL 查询参数上）
// 这两种都由 EmbyAuth 中间件统一处理。

// StreamVideoHandler 对应 GET/HEAD /Videos/{id}/stream(.{container})?...
// 策略（对齐 Emby "秒开"）：
//   1) STRM 远程流 -> 代理
//   2) 探测编码兼容性：若视频/音频编码都是浏览器可解码的 → 走 RemuxStream（零转码秒开）
//      无论容器是 mkv/avi/mov 都如此，这是 Emby 秒开的核心
//   3) 编码不兼容 或 容器本身可直接播放（mp4/m4v/webm） → ServeContent 直出
//   4) 客户端显式请求 Static=false/TranscodingProtocol=hls 时 → Remux
func (h *Handler) StreamVideoHandler(c *gin.Context) {
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

	// 1) STRM 远程流 → 透明代理
	if strings.TrimSpace(m.StreamURL) != "" {
		if err := h.stream.ProxyRemoteStream(m.StreamURL, c.Writer, c.Request); err != nil {
			h.logger.Warnf("[emby] proxy remote stream failed media=%s err=%v", uuid, err)
			if !c.Writer.Written() {
				c.JSON(http.StatusBadGateway, gin.H{"Error": "Upstream failed"})
			}
		}
		return
	}

	// 2) 本地文件
	filePath := m.FilePath
	if filePath == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "File path not configured"})
		return
	}
	fi, err := os.Stat(filePath)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "File not found"})
		return
	}
	if fi.IsDir() {
		c.JSON(http.StatusBadRequest, gin.H{"Error": "Path is a directory"})
		return
	}

	container := strings.ToLower(containerFromPath(filePath))
	ua := c.Request.Header.Get("User-Agent")

	// 3) 客户端显式要求 remux
	wantRemux := c.Query("Static") == "false" || c.Query("TranscodingProtocol") == "hls"

	// 4) 编码兼容 → 走 Remux（秒开核心路径）
	//    注意：对 mp4/webm 等原本可 ServeContent 的容器，编码兼容时也优先 Remux？
	//    不 —— 它们 Range 请求更高效，保留 ServeContent 更省资源。
	//    ShouldRemux 已在白名单内限定了容器（mkv/avi/mov/flv/wmv/ts）。
	canRemux := h.stream.ShouldRemux(m, ua)

	if wantRemux || canRemux {
		if err := h.stream.RemuxStream(uuid, c.Writer, c.Request); err != nil {
			h.logger.Warnf("[emby] remux failed media=%s err=%v, fallback to direct serve", uuid, err)
			// fallthrough to direct serve，但需确认响应还没写出
			if c.Writer.Written() {
				return
			}
		} else {
			return
		}
	}

	// 5) 直接发送文件（http.ServeContent 自动处理 Range / If-Modified-Since / HEAD）
	mime := mimeFromContainer(container)
	c.Header("Content-Type", mime)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "private, max-age=3600")
	f, err := os.Open(filePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"Error": "Failed to open file"})
		return
	}
	defer f.Close()
	http.ServeContent(c.Writer, c.Request, fi.Name(), fi.ModTime(), f)
}

// OriginalVideoHandler 对应 GET /Videos/{id}/original(.{ext})
// 直接返回文件原始字节，Infuse 少数情况下使用。
func (h *Handler) OriginalVideoHandler(c *gin.Context) {
	h.StreamVideoHandler(c)
}

// ==================== HLS 流 ====================

// HLSMasterHandler 对应 GET /Videos/{id}/master.m3u8。
// 复用现有的 StreamService.GetMasterPlaylist，返回 master playlist 文本。
func (h *Handler) HLSMasterHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	playlist, err := h.stream.GetMasterPlaylist(uuid)
	if err != nil {
		h.logger.Warnf("[emby] master playlist failed media=%s err=%v", uuid, err)
		c.JSON(http.StatusNotFound, gin.H{"Error": "HLS not available"})
		return
	}
	// 将内部 /api/stream/ 路径重写为 /emby/Videos/:id/hls1/:quality/main.m3u8
	// 这样客户端请求分片时会走 Emby 路由，保持路径风格一致
	playlist = rewriteMasterForEmby(playlist, embyID)
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

// rewriteMasterForEmby 把 /api/stream/{uuid}/{quality}/stream.m3u8
// 替换为 /emby/Videos/{embyID}/hls1/{quality}/main.m3u8，
// 让 Emby 客户端的后续分片请求命中 Emby 路由。
func rewriteMasterForEmby(playlist, embyID string) string {
	lines := strings.Split(playlist, "\n")
	for i, ln := range lines {
		ln = strings.TrimSpace(ln)
		if strings.HasPrefix(ln, "/api/stream/") && strings.HasSuffix(ln, "/stream.m3u8") {
			// 形如 /api/stream/{uuid}/{quality}/stream.m3u8
			parts := strings.Split(ln, "/")
			if len(parts) >= 6 {
				quality := parts[4]
				lines[i] = fmt.Sprintf("/emby/Videos/%s/hls1/%s/main.m3u8", embyID, quality)
			}
		}
	}
	return strings.Join(lines, "\n")
}

// HLSPlaylistHandler 对应 GET /Videos/{id}/hls1/{quality}/main.m3u8。
// 返回该码率的 variant playlist；若首片未生成会等待最多 10 秒（详见 GetSegmentPlaylist）。
func (h *Handler) HLSPlaylistHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	quality := c.Param("quality")
	playlist, err := h.stream.GetSegmentPlaylist(uuid, quality)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Quality not available"})
		return
	}
	// 将 FFmpeg 产出的绝对文件路径分片名改写为相对 URL，便于客户端拼接
	// 由于 GetSegmentPlaylist 返回的是 FFmpeg 写入文件内容，分片名为 seg0001.ts 相对路径
	// 这里保留原样即可，客户端会基于当前 URL（.../hls1/:quality/main.m3u8）解析相对路径
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

// HLSSegmentHandler 对应 GET /Videos/{id}/hls1/{quality}/{segment}.ts。
func (h *Handler) HLSSegmentHandler(c *gin.Context) {
	embyID := c.Param("id")
	uuid := h.idMap.Resolve(embyID)
	if uuid == "" {
		c.JSON(http.StatusNotFound, gin.H{"Error": "Item not found"})
		return
	}
	quality := c.Param("quality")
	segment := c.Param("segment")
	if err := h.stream.ServeSegment(uuid, quality, segment, c.Writer, c.Request); err != nil {
		h.logger.Warnf("[emby] segment failed media=%s seg=%s err=%v", uuid, segment, err)
		if !c.Writer.Written() {
			c.JSON(http.StatusNotFound, gin.H{"Error": fmt.Sprintf("Segment failed: %v", err)})
		}
	}
}
