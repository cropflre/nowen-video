package handler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
)

// StreamHandler 流媒体处理器
type StreamHandler struct {
	streamService    *service.StreamService
	transcodeService *service.TranscodeService // 用于接收播放进度上报驱动节流
	logger           *zap.SugaredLogger
}

// Direct 直接提供原始文件流（支持Range请求，用于MP4等浏览器兼容格式）
// 对于 STRM 远程流，通过后端代理转发
// 对于 WebDAV 路径（webdav://），通过 VFS 打开并使用 http.ServeContent（支持 Range）
func (h *StreamHandler) Direct(c *gin.Context) {
	id := c.Param("id")
	filePath, contentType, err := h.streamService.GetDirectStreamInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	// STRM 远程流：通过后端代理转发
	if filePath == "__strm__" {
		remoteURL := contentType // GetDirectStreamInfo 对 STRM 返回的第二个值是远程 URL
		h.logger.Debugf("STRM 代理播放: %s -> %s", id, remoteURL)
		if err := h.streamService.ProxyRemoteStream(remoteURL, c.Writer, c.Request); err != nil {
			h.logger.Warnf("STRM 代理播放失败: %s, 错误: %v", id, err)
			// 如果还没写入响应头，返回错误
			c.JSON(http.StatusBadGateway, gin.H{"error": "远程流播放失败: " + err.Error()})
		}
		return
	}

	c.Header("Content-Type", contentType)
	c.Header("Accept-Ranges", "bytes")
	c.Header("Cache-Control", "public, max-age=86400")

	// WebDAV 远程文件：使用 VFS 打开，并借助 http.ServeContent 提供 Range 支持
	if service.IsWebDAVPath(filePath) {
		vfsFile, err := h.streamService.OpenMediaFile(filePath)
		if err != nil {
			h.logger.Warnf("WebDAV 打开文件失败: %s, 错误: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "打开远程文件失败: " + err.Error()})
			return
		}
		defer vfsFile.Close()

		stat, err := vfsFile.Stat()
		if err != nil {
			h.logger.Warnf("WebDAV 获取文件信息失败: %s, 错误: %v", filePath, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "获取文件信息失败: " + err.Error()})
			return
		}

		// webdavFile 实现了 io.ReadSeeker（通过 ReadAt + 自实现 Seeker 适配）
		// 这里用 seekAdapter 包裹 VFSFile 为 io.ReadSeeker
		reader := service.NewVFSReadSeeker(vfsFile, stat.Size())
		http.ServeContent(c.Writer, c.Request, filepath.Base(filePath), stat.ModTime(), reader)
		return
	}

	// 本地文件：使用http.ServeFile自动处理Range请求（断点续播、拖动进度条）
	http.ServeFile(c.Writer, c.Request, filePath)
}

// Master 获取HLS主播放列表
//
// 支持通过 ?maxBitrate=3000000 参数声明客户端最大可承受码率（bit/s），
// 服务端将过滤掉超过此上限的档位，配合 hls.js 的 bandwidth-report 实现弱网降档。
func (h *StreamHandler) Master(c *gin.Context) {
	id := c.Param("id")
	maxBitrate := 0
	if mb := c.Query("maxBitrate"); mb != "" {
		if v, err := strconv.Atoi(mb); err == nil && v > 0 {
			maxBitrate = v
		}
	}
	playlist, err := h.streamService.GetMasterPlaylistFiltered(id, maxBitrate)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

// Segment 提供HLS分片或子播放列表
//
// 分片回退顺序：
//  1. 优先命中主转码目录 /cache/transcode/:id/:quality/seg_NNNN.ts（已转码的范围）
//  2. 未命中时，回退到按需切片目录 /cache/transcode/:id/:quality/ondemand/seg_NNNN.ts
//     不存在则独立起 FFmpeg 从 N*2 秒切 2 秒并落盘返回
//
// 这样前端 seek 到远未转码区域时可以秒出分片，不必等主转码追上来。
func (h *StreamHandler) Segment(c *gin.Context) {
	id := c.Param("id")
	quality := c.Param("quality")
	segment := c.Param("segment")

	// 如果请求的是子m3u8播放列表
	if segment == "stream.m3u8" {
		playlist, err := h.streamService.GetSegmentPlaylist(id, quality)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.Header("Content-Type", "application/vnd.apple.mpegurl")
		c.Header("Cache-Control", "no-cache")
		c.String(http.StatusOK, playlist)
		return
	}

	// 先试主转码目录
	if err := h.streamService.ServeSegment(id, quality, segment, c.Writer, c.Request); err == nil {
		return
	}

	// 主转码未产出该分片 → 按需切片兜底
	if err := h.streamService.ServeOnDemandSegment(id, quality, segment, c.Writer, c.Request); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
}

// AudioPlaylist 按需音轨 playlist
// GET /api/stream/:id/audio-track/:trackIdx.m3u8
func (h *StreamHandler) AudioPlaylist(c *gin.Context) {
	id := c.Param("id")
	trackParam := c.Param("trackIdx")
	// trackIdx 在路由里是 ":trackIdx.m3u8" 形式（带扩展名），需要先去掉 .m3u8
	trackParam = strings.TrimSuffix(trackParam, ".m3u8")
	trackIdx, err := strconv.Atoi(trackParam)
	if err != nil || trackIdx < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid track index"})
		return
	}
	playlist, err := h.streamService.GetAudioPlaylist(id, trackIdx)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "application/vnd.apple.mpegurl")
	c.Header("Cache-Control", "no-cache")
	c.String(http.StatusOK, playlist)
}

// AudioSegment 按需音轨分片
// GET /api/stream/:id/audio-track/:trackIdx/:seg
func (h *StreamHandler) AudioSegment(c *gin.Context) {
	id := c.Param("id")
	trackIdx, err := strconv.Atoi(c.Param("trackIdx"))
	if err != nil || trackIdx < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid track index"})
		return
	}
	segName := c.Param("seg")
	if err := h.streamService.ServeAudioSegment(id, trackIdx, segName, c.Writer, c.Request); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
}

// Remux 实时将 MKV 等格式 remux 为 fragmented MP4 流式输出（零转码，仅转封装）
func (h *StreamHandler) Remux(c *gin.Context) {
	id := c.Param("id")
	h.logger.Debugf("Remux 播放请求: %s", id)
	if err := h.streamService.RemuxStream(id, c.Writer, c.Request); err != nil {
		h.logger.Warnf("Remux 播放失败: %s, 错误: %v", id, err)
		// 如果还没写入响应头，返回错误
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Remux 播放失败: " + err.Error()})
	}
}

// MediaInfo 获取媒体的播放信息（前端用于决定播放模式）
func (h *StreamHandler) MediaInfo(c *gin.Context) {
	id := c.Param("id")
	info, err := h.streamService.GetMediaPlayInfo(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": info})
}

// Poster 提供媒体海报/缩略图
// posterPlaceholderSVG 美观的海报占位图 SVG（深色渐变背景 + 电影图标 + 提示文字）
const posterPlaceholderSVG = `<svg xmlns="http://www.w3.org/2000/svg" width="300" height="450" viewBox="0 0 300 450">
  <defs>
    <linearGradient id="bg" x1="0" y1="0" x2="0" y2="1">
      <stop offset="0%" stop-color="#1a1b2e"/>
      <stop offset="100%" stop-color="#0f1019"/>
    </linearGradient>
    <linearGradient id="icon" x1="0" y1="0" x2="1" y2="1">
      <stop offset="0%" stop-color="#3b82f6" stop-opacity="0.4"/>
      <stop offset="100%" stop-color="#8b5cf6" stop-opacity="0.25"/>
    </linearGradient>
  </defs>
  <rect fill="url(#bg)" width="300" height="450" rx="0"/>
  <rect x="0" y="0" width="300" height="450" fill="url(#icon)" opacity="0.08"/>
  <!-- 电影胶片图标 -->
  <g transform="translate(150,200)" fill="none" stroke="#4a5568" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round" opacity="0.5">
    <rect x="-24" y="-18" width="48" height="36" rx="3"/>
    <path d="M-24,-10 L-16,-10 M-24,-2 L-16,-2 M-24,6 L-16,6"/>
    <path d="M24,-10 L16,-10 M24,-2 L16,-2 M24,6 L16,6"/>
    <circle cx="-4" cy="0" r="6"/>
    <circle cx="10" cy="0" r="3"/>
  </g>
  <text fill="#4a5568" font-family="-apple-system,BlinkMacSystemFont,sans-serif" font-size="12" font-weight="500" text-anchor="middle" x="150" y="248">暂无海报</text>
</svg>`

func (h *StreamHandler) Poster(c *gin.Context) {
	id := c.Param("id")
	posterPath, err := h.streamService.GetPosterPath(id)
	if err != nil || posterPath == "" {
		// 返回美观的占位图（禁止缓存，确保海报就绪后能立即生效）
		c.Header("Content-Type", "image/svg+xml")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("Pragma", "no-cache")
		c.Header("X-Poster-Placeholder", "true")
		c.String(http.StatusOK, posterPlaceholderSVG)
		return
	}

	// V2.1: WebDAV 海报 —— 通过 VFS 读取并流式输出
	if service.IsWebDAVPath(posterPath) {
		vfsFile, openErr := h.streamService.OpenMediaFile(posterPath)
		if openErr != nil {
			c.Header("Content-Type", "image/svg+xml")
			c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
			c.Header("X-Poster-Placeholder", "true")
			c.String(http.StatusOK, posterPlaceholderSVG)
			return
		}
		defer vfsFile.Close()

		stat, statErr := vfsFile.Stat()
		if statErr != nil {
			c.Header("Content-Type", "image/svg+xml")
			c.Header("X-Poster-Placeholder", "true")
			c.String(http.StatusOK, posterPlaceholderSVG)
			return
		}
		etag := fmt.Sprintf(`"%x-%x"`, stat.ModTime().UnixNano(), stat.Size())
		c.Header("ETag", etag)
		if match := c.GetHeader("If-None-Match"); match == etag {
			c.Status(http.StatusNotModified)
			return
		}
		setPosterContentType(c, posterPath)
		c.Header("Cache-Control", "public, max-age=86400, must-revalidate")
		// 通过 VFSReadSeeker 提供完整文件（支持 Range，但海报一般很小）
		reader := service.NewVFSReadSeeker(vfsFile, stat.Size())
		http.ServeContent(c.Writer, c.Request, filepath.Base(posterPath), stat.ModTime(), reader)
		return
	}

	// 基于文件修改时间生成 ETag，支持条件请求（If-None-Match）
	fileInfo, statErr := os.Stat(posterPath)
	if statErr != nil {
		c.Header("Content-Type", "image/svg+xml")
		c.Header("Cache-Control", "no-cache, no-store, must-revalidate")
		c.Header("X-Poster-Placeholder", "true")
		c.String(http.StatusOK, posterPlaceholderSVG)
		return
	}

	etag := fmt.Sprintf(`"%x-%x"`, fileInfo.ModTime().UnixNano(), fileInfo.Size())
	c.Header("ETag", etag)

	// 客户端缓存命中时返回 304
	if match := c.GetHeader("If-None-Match"); match == etag {
		c.Status(http.StatusNotModified)
		return
	}

	setPosterContentType(c, posterPath)
	c.Header("Cache-Control", "public, max-age=86400, must-revalidate") // 缓存1天，但必须重新验证
	c.File(posterPath)
}

// setPosterContentType 根据扩展名设置海报 Content-Type
func setPosterContentType(c *gin.Context, posterPath string) {
	ext := strings.ToLower(filepath.Ext(posterPath))
	switch ext {
	case ".jpg", ".jpeg":
		c.Header("Content-Type", "image/jpeg")
	case ".png":
		c.Header("Content-Type", "image/png")
	case ".webp":
		c.Header("Content-Type", "image/webp")
	default:
		c.Header("Content-Type", "application/octet-stream")
	}
}

// Playback 接收前端上报的播放位置（秒），驱动 Throttling 决策。
// POST /api/stream/:id/playback?position=123.4
//
// 前端（HLS.js/video.js/Shaka 等）只需每 2~5 秒调用一次即可，
// 服务端会对比这个位置与 ffmpeg 当前转码位置，决定是否挂起/恢复进程。
//
// 为什么单独开一个接口而不复用 /api/watch/history？
//  1. watch history 写 DB 是带副作用的；节流只需要内存态的位置。
//  2. 前端可以高频调（<5s 一次），不污染观看历史。
//  3. 解耦：不触发推荐系统的"已观看"判定。
func (h *StreamHandler) Playback(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	positionStr := c.Query("position")
	if positionStr == "" {
		// 也支持 form / JSON 体
		positionStr = c.PostForm("position")
	}
	position, err := strconv.ParseFloat(positionStr, 64)
	if err != nil || position < 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid position"})
		return
	}
	if h.transcodeService != nil {
		h.transcodeService.SetPlaybackPosition(id, position)
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// Bandwidth 接收前端 hls.js 上报的网络带宽评估（bit/s），
// 返回带建议的 maxBitrate。前端可在下次请求 master.m3u8 时带上 ?maxBitrate=xxx。
//
// POST /api/stream/:id/bandwidth?bitrate=2500000
//
// 实现策略：
//   - 服务端不存储历史带宽（避免成为状态源），仅对客户端的上报做回显/策略建议
//   - 策略：在上报值基础上留 20% 余量（乘以 0.8），作为推荐 maxBitrate
//   - 前端 hls.js 的 bandwidthEstimate 已经是 EWMA 平滑后的值，无需再次平滑
//
// 这样设计的好处：
//  1. 幂等、无副作用，重复上报不污染任何状态
//  2. 弱网切换 master.m3u8 的决策权完全由客户端（hls.js）掌握，
//     服务端只提供对"档位应该取哪个上限"的建议
func (h *StreamHandler) Bandwidth(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	bitrateStr := c.Query("bitrate")
	if bitrateStr == "" {
		bitrateStr = c.PostForm("bitrate")
	}
	bitrate, err := strconv.Atoi(bitrateStr)
	if err != nil || bitrate <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid bitrate"})
		return
	}
	// 留 20% 余量作为推荐上限
	recommended := bitrate * 80 / 100

	// 同时返回该 media 的节流/转码状态，前端 UI 就可以一次请求拿齐所有信息
	// 避免单独再开一个查询端点
	resp := gin.H{
		"ok":               true,
		"reported_bitrate": bitrate,
		"recommended_max":  recommended,
	}
	if h.transcodeService != nil {
		resp["throttle"] = h.transcodeService.GetMediaThrottleStatus(id)
	}
	c.JSON(http.StatusOK, resp)
}

// ThrottleStatus 返回单个 media 的节流/转码快照。
// GET /api/stream/:id/throttle
//
// 前端播放器 Settings 菜单用此端点做可视化。与 Bandwidth 不同，
// 这里是纯查询接口、低频（通常 5s 一次），不改变任何状态。
func (h *StreamHandler) ThrottleStatus(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing id"})
		return
	}
	if h.transcodeService == nil {
		c.JSON(http.StatusOK, gin.H{"data": gin.H{"media_id": id, "running": false}})
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": h.transcodeService.GetMediaThrottleStatus(id)})
}
