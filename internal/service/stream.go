package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// MediaPlayInfo 媒体播放信息（前端根据此信息决定播放模式）
type MediaPlayInfo struct {
	MediaID          string  `json:"media_id"`
	DirectPlayURL    string  `json:"direct_play_url"`    // 直接播放地址（如果支持）
	HlsURL           string  `json:"hls_url"`            // HLS转码播放地址
	CanDirectPlay    bool    `json:"can_direct_play"`    // 浏览器是否可直接播放
	FileExt          string  `json:"file_ext"`           // 文件扩展名
	VideoCodec       string  `json:"video_codec"`        // 视频编码
	AudioCodec       string  `json:"audio_codec"`        // 音频编码
	Duration         float64 `json:"duration"`           // 时长（秒）
	IsSTRM           bool    `json:"is_strm"`            // 是否为 STRM 远程流
	IsPreprocessed   bool    `json:"is_preprocessed"`    // 是否已预处理
	PreprocessedURL  string  `json:"preprocessed_url"`   // 预处理后的 HLS 地址
	PreprocessStatus string  `json:"preprocess_status"`  // 预处理状态: none / pending / running / completed
	ThumbnailURL     string  `json:"thumbnail_url"`      // 预处理封面缩略图
	SpriteURL        string  `json:"sprite_url"`         // 进度条雪碧图地址（预处理完成后可用）
	SpriteVTTURL     string  `json:"sprite_vtt_url"`     // 进度条雪碧图 WebVTT 索引地址
	PreferDirectPlay bool    `json:"prefer_direct_play"` // 系统设置：优先直接播放（禁用自动转码）
	CanRemux         bool    `json:"can_remux"`          // 是否支持 remux（MKV等容器内编码兼容但容器不兼容）
	RemuxURL         string  `json:"remux_url"`          // Remux 播放地址（零转码，仅转封装）
}

// 浏览器可直接播放的文件格式
var directPlayableExts = map[string]bool{
	".mp4":  true,
	".webm": true,
	".m4v":  true,
}

// 可通过 remux（转封装）播放的格式：容器不兼容但内部编码兼容
// 这些格式内部通常是 H.264/H.265+AAC，只需换容器为 fMP4 即可在浏览器播放
var remuxableExts = map[string]bool{
	".mkv": true,
	".avi": true,
	".mov": true,
	".flv": true,
	".wmv": true,
	".ts":  true,
}

// 浏览器可解码的视频编码（用于判断是否可 remux）
var browserCompatibleVideoCodecs = map[string]bool{
	"h264": true, "avc": true, "avc1": true,
	"h265": true, "hevc": true, // Chrome 108+ 部分支持
	"vp8": true, "vp9": true,
	"av1": true,
}

// 浏览器可解码的音频编码
var browserCompatibleAudioCodecs = map[string]bool{
	"aac": true, "mp3": true, "opus": true,
	"vorbis": true, "flac": true, "ac3": true, "eac3": true,
}

// 文件扩展名 -> MIME类型映射
var mimeTypes = map[string]string{
	".mp4":  "video/mp4",
	".webm": "video/webm",
	".m4v":  "video/mp4",
	".mkv":  "video/x-matroska",
	".avi":  "video/x-msvideo",
	".mov":  "video/quicktime",
}

// StreamService 流媒体服务
type StreamService struct {
	mediaRepo   *repository.MediaRepo
	seriesRepo  *repository.SeriesRepo
	transcoder  *TranscodeService
	preprocess  *PreprocessService
	settingRepo *repository.SystemSettingRepo
	cfg         *config.Config
	logger      *zap.SugaredLogger
	vfsMgr      *VFSManager // V2.1: VFS 管理器，支持 webdav:// 路径
}

func NewStreamService(
	mediaRepo *repository.MediaRepo,
	seriesRepo *repository.SeriesRepo,
	transcoder *TranscodeService,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) *StreamService {
	return &StreamService{
		mediaRepo:  mediaRepo,
		seriesRepo: seriesRepo,
		transcoder: transcoder,
		cfg:        cfg,
		logger:     logger,
	}
}

// SetVFSManager 设置 VFS 管理器（V2.1）
func (s *StreamService) SetVFSManager(vfsMgr *VFSManager) {
	s.vfsMgr = vfsMgr
}

// statMediaFile 返回文件判断：同时支持本地路径和 webdav:// 路径
func (s *StreamService) statMediaFile(p string) (os.FileInfo, error) {
	if s.vfsMgr != nil && IsWebDAVPath(p) {
		return s.vfsMgr.Stat(p)
	}
	return os.Stat(p)
}

// OpenMediaFile 打开媒体文件（支持 WebDAV 与本地路径）
// V2.1: 供 handler 层在 webdav:// 路径下通过 VFS 流式服务文件
func (s *StreamService) OpenMediaFile(p string) (VFSFile, error) {
	if s.vfsMgr != nil && IsWebDAVPath(p) {
		return s.vfsMgr.Open(p)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	return &localFile{File: f}, nil
}

// openMediaFile 打开媒体文件（支持 WebDAV）
func (s *StreamService) openMediaFile(p string) (VFSFile, error) {
	return s.OpenMediaFile(p)
}

// SetPreprocessService 注入预处理服务（延迟注入，避免循环依赖）
func (s *StreamService) SetPreprocessService(ps *PreprocessService) {
	s.preprocess = ps
}

// SetSettingRepo 注入系统设置仓储（延迟注入）
func (s *StreamService) SetSettingRepo(repo *repository.SystemSettingRepo) {
	s.settingRepo = repo
}

// ShouldRemux 判断给定媒体在当前客户端 UA 下是否应该走 Remux（零转码）。
// 这是"秒开"的关键判定：一旦返回 true，应让客户端直接请求 /api/stream/:id/remux
// 或 /emby/Videos/:id/stream（Emby 层走 remux 分支），完全绕过 HLS 转码。
//
// 判定规则：
//  1. 文件必须是本地（非 STRM）
//  2. 容器在 remuxableExts 白名单（mkv/avi/mov/flv/wmv/ts）
//  3. 视频编码在浏览器兼容列表
//  4. 音频编码为空（单轨或未探测）或在浏览器兼容列表
//  5. 客户端 UA 对应支持 fMP4 fragmented MP4 流式回放
//
// 当前仅拒绝明确不支持 fMP4 的旧设备（如裸 Safari iOS 13-），
// 其他情况（包括 Infuse/Chrome/Edge/Firefox 等）一律允许。
func (s *StreamService) ShouldRemux(media *model.Media, userAgent string) bool {
	if media == nil || media.StreamURL != "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(media.FilePath))
	if !remuxableExts[ext] {
		return false
	}
	vc := strings.ToLower(media.VideoCodec)
	ac := strings.ToLower(media.AudioCodec)
	if !browserCompatibleVideoCodecs[vc] {
		return false
	}
	if ac != "" && !browserCompatibleAudioCodecs[ac] {
		return false
	}
	// UA 黑名单：这些客户端对 fragmented MP4 支持不完善，强制走 HLS 更稳
	uaLower := strings.ToLower(userAgent)
	if strings.Contains(uaLower, "applecoremedia") {
		// Apple 原生播放器对 fMP4 中 MKV-remux 偶发兼容问题；但 Infuse/Safari 通常用的是 AVFoundation
		// Infuse UA 包含 "Infuse" 字样，优先保留
		if !strings.Contains(uaLower, "infuse") && !strings.Contains(uaLower, "safari") {
			return false
		}
	}
	return true
}

// ClientSupportsHEVC 粗略判断客户端是否能硬解 HEVC。
// 主要用于返回 MediaSource 时选择是否标记支持 DirectPlay。
// Safari 11+/Edge Chromium/Chrome 108+ 都支持 HEVC 硬解。
func (s *StreamService) ClientSupportsHEVC(userAgent string) bool {
	uaLower := strings.ToLower(userAgent)
	switch {
	case strings.Contains(uaLower, "infuse"):
		return true // Infuse 全平台硬解 HEVC
	case strings.Contains(uaLower, "safari") && !strings.Contains(uaLower, "chrome"):
		return true // 原生 Safari（非 Chromium 内核）
	case strings.Contains(uaLower, "edg/"):
		return true // Edge Chromium（>=80 支持）
	case strings.Contains(uaLower, "exoplayer"):
		return true // ExoPlayer 硬解
	}
	return false
}

// ShouldRemux 对外接口：基于 mediaID 查询并判断。
// 给 handler 层直接用，不需要每次自己读库。
func (s *StreamService) ShouldRemuxByID(mediaID, userAgent string) (bool, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return false, ErrMediaNotFound
	}
	return s.ShouldRemux(media, userAgent), nil
}

// GetMediaPlayInfo 获取播放信息，前端根据此判断使用哪种播放方式
func (s *StreamService) GetMediaPlayInfo(mediaID string) (*MediaPlayInfo, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}

	// STRM 远程流：始终通过后端代理直接播放
	if media.StreamURL != "" {
		info := &MediaPlayInfo{
			MediaID:       mediaID,
			DirectPlayURL: fmt.Sprintf("/api/stream/%s/direct", mediaID),
			CanDirectPlay: true,
			FileExt:       ".strm",
			VideoCodec:    media.VideoCodec,
			AudioCodec:    media.AudioCodec,
			Duration:      media.Duration,
			IsSTRM:        true,
		}
		// 如果远程 URL 是 HLS 流，也提供 HLS 地址（后端代理）
		urlLower := strings.ToLower(media.StreamURL)
		if strings.Contains(urlLower, ".m3u8") {
			info.HlsURL = fmt.Sprintf("/api/stream/%s/direct", mediaID)
		}
		return info, nil
	}

	ext := strings.ToLower(filepath.Ext(media.FilePath))
	canDirect := directPlayableExts[ext]

	info := &MediaPlayInfo{
		MediaID:       mediaID,
		HlsURL:        fmt.Sprintf("/api/stream/%s/master.m3u8", mediaID),
		FileExt:       ext,
		VideoCodec:    media.VideoCodec,
		AudioCodec:    media.AudioCodec,
		Duration:      media.Duration,
		CanDirectPlay: canDirect,
		IsSTRM:        false,
	}

	if canDirect {
		info.DirectPlayURL = fmt.Sprintf("/api/stream/%s/direct", mediaID)
	}

	// 判断是否可通过 remux 播放（容器不兼容但编码兼容）
	if !canDirect && remuxableExts[ext] {
		videoCodec := strings.ToLower(media.VideoCodec)
		audioCodec := strings.ToLower(media.AudioCodec)
		if browserCompatibleVideoCodecs[videoCodec] && (audioCodec == "" || browserCompatibleAudioCodecs[audioCodec]) {
			info.CanRemux = true
			info.RemuxURL = fmt.Sprintf("/api/stream/%s/remux", mediaID)
		}
	}

	// 读取系统设置：是否优先直接播放
	if s.settingRepo != nil {
		if val, err := s.settingRepo.Get("prefer_direct_play"); err == nil {
			info.PreferDirectPlay = val == "true" || val == "1"
		} else {
			info.PreferDirectPlay = true // 默认优先直接播放
		}
	} else {
		info.PreferDirectPlay = true
	}

	// 检查是否有预处理内容（优先使用预处理的 HLS 流，实现秒开）
	if s.preprocess != nil && s.preprocess.IsPreprocessed(mediaID) {
		info.IsPreprocessed = true
		info.PreprocessedURL = fmt.Sprintf("/api/preprocess/media/%s/master.m3u8", mediaID)
		info.PreprocessStatus = "completed"
		info.ThumbnailURL = fmt.Sprintf("/api/preprocess/media/%s/thumbnail", mediaID)
		info.SpriteURL = fmt.Sprintf("/api/preprocess/media/%s/sprite.jpg", mediaID)
		info.SpriteVTTURL = fmt.Sprintf("/api/preprocess/media/%s/sprite.vtt", mediaID)
		// 预处理完成时，HLS 地址指向预处理内容（秒开）
		info.HlsURL = info.PreprocessedURL
	} else if s.preprocess != nil {
		// 查询预处理状态
		if task, err := s.preprocess.GetMediaTask(mediaID); err == nil {
			info.PreprocessStatus = task.Status
		} else {
			info.PreprocessStatus = "none"
		}
	}

	return info, nil
}

// GetDirectStreamInfo 获取直接播放的文件路径和MIME类型
// 对于 STRM 文件，返回特殊标记，由 handler 层走代理逻辑
func (s *StreamService) GetDirectStreamInfo(mediaID string) (string, string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", "", ErrMediaNotFound
	}

	// STRM 远程流：返回特殊标记
	if media.StreamURL != "" {
		return "__strm__", media.StreamURL, nil
	}

	// 检查文件是否存在（支持 WebDAV 路径）
	if _, err := s.statMediaFile(media.FilePath); err != nil {
		if os.IsNotExist(err) {
			return "", "", fmt.Errorf("文件不存在: %s", media.FilePath)
		}
		return "", "", fmt.Errorf("媒体文件访问失败: %w", err)
	}

	ext := strings.ToLower(filepath.Ext(media.FilePath))
	contentType, ok := mimeTypes[ext]
	if !ok {
		contentType = "application/octet-stream"
	}

	return media.FilePath, contentType, nil
}

// vfsJoinPath 路径拼接：对 webdav:// 使用正斜杠（避免 Windows 下 filepath.Join 破坏前缀）
func vfsJoinPath(base, name string) string {
	if IsWebDAVPath(base) {
		base = strings.TrimRight(base, "/")
		return base + "/" + strings.TrimLeft(name, "/")
	}
	return filepath.Join(base, name)
}

// vfsDir 目录提取：对 webdav:// 使用正斜杠规则
func vfsDir(p string) string {
	if IsWebDAVPath(p) {
		i := strings.LastIndex(p, "/")
		if i <= len("webdav:/") { // 保护 webdav:// 前缀
			return p
		}
		return p[:i]
	}
	return filepath.Dir(p)
}

// GetPosterPath 获取海报文件路径
// 对于剧集（episode），如果自身没有海报，会自动 fallback 到所属 Series 的海报
func (s *StreamService) GetPosterPath(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}

	// 优先查找媒体自身的海报路径
	if media.PosterPath != "" {
		if _, err := s.statMediaFile(media.PosterPath); err == nil {
			return media.PosterPath, nil
		}
	}

	// 查找同目录下的海报文件
	dir := vfsDir(media.FilePath)
	base := strings.TrimSuffix(filepath.Base(media.FilePath), filepath.Ext(media.FilePath))

	posterExts := []string{".jpg", ".jpeg", ".png", ".webp"}
	posterNames := []string{
		base,     // 同名
		"poster", // poster.jpg
		"cover",  // cover.jpg
		"folder", // folder.jpg
	}

	for _, name := range posterNames {
		for _, ext := range posterExts {
			candidate := vfsJoinPath(dir, name+ext)
			if _, err := s.statMediaFile(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	// Fallback：如果是剧集（episode）且自身没有海报，尝试使用所属 Series 的海报
	if media.SeriesID != "" && s.seriesRepo != nil {
		series, err := s.seriesRepo.FindByIDOnly(media.SeriesID)
		if err == nil {
			// 检查 Series 数据库中的海报路径
			if series.PosterPath != "" {
				if _, err := s.statMediaFile(series.PosterPath); err == nil {
					return series.PosterPath, nil
				}
			}
			// 查找 Series 根目录下的海报文件
			if series.FolderPath != "" {
				seriesPosterNames := []string{"poster", "cover", "folder", "show"}
				for _, name := range seriesPosterNames {
					for _, ext := range posterExts {
						candidate := vfsJoinPath(series.FolderPath, name+ext)
						if _, err := s.statMediaFile(candidate); err == nil {
							return candidate, nil
						}
					}
				}
			}
		}
	}

	return "", nil
}

// GetMasterPlaylist 获取HLS主播放列表（多码率自适应）
func (s *StreamService) GetMasterPlaylist(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}

	// STRM 远程流不支持 HLS 转码，应走直接播放路径
	if media.StreamURL != "" {
		return "", fmt.Errorf("STRM 远程流不支持 HLS 转码，请使用直接播放")
	}

	// 根据原始视频分辨率确定可用的质量选项
	qualities := s.getAvailableQualities(media)

	// 生成master.m3u8
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")

	for _, q := range qualities {
		preset := qualityPresets[q]
		bandwidth := s.estimateBandwidth(preset.VideoBitrate)
		builder.WriteString(fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"\n",
			bandwidth, preset.Width, preset.Height, q,
		))
		builder.WriteString(fmt.Sprintf("/api/stream/%s/%s/stream.m3u8\n", mediaID, q))
	}

	return builder.String(), nil
}

// GetSegmentPlaylist 获取指定质量的播放列表或触发转码
// 秒开优化：
//   - hls_time 从 6 降到 2，首片最快 2 秒内产出
//   - 使用 WaitForFirstSegment 替代原 60 秒轮询，响应更快
//   - 10 秒仍无首片时返回占位 playlist（ExoPlayer/HLS.js 会自动重试）
func (s *StreamService) GetSegmentPlaylist(mediaID, quality string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}

	// STRM 远程流不支持转码
	if media.StreamURL != "" {
		return "", fmt.Errorf("STRM 远程流不支持转码")
	}

	outputDir := s.transcoder.GetOutputDir(mediaID, quality)
	m3u8Path := filepath.Join(outputDir, "stream.m3u8")

	// 检查是否已有转码缓存（完成态或运行中均可）
	if _, err := os.Stat(m3u8Path); err == nil {
		content, err := os.ReadFile(m3u8Path)
		if err == nil && strings.Contains(string(content), ".ts") {
			return string(content), nil
		}
	}

	// 触发转码（若已在运行则复用）
	if _, err := s.transcoder.StartTranscode(media, quality); err != nil {
		return "", fmt.Errorf("启动转码失败: %w", err)
	}

	// 等待首片就绪：10 秒硬超时（对比旧版 60 秒，首包延迟大幅下降）
	// 之所以从 60→10 秒：
	//   1) hls_time 从 6→2 秒，理论首片 2~4 秒可出
	//   2) 若 10 秒还没出，要么 FFmpeg 启动失败，要么硬件加速卡住，
	//      返回占位 playlist 让客户端重试是更友好的体验
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := s.transcoder.WaitForFirstSegment(ctx, mediaID, quality); err == nil {
		if content, err := os.ReadFile(m3u8Path); err == nil {
			return string(content), nil
		}
	}

	// 超时：返回 EVENT 类型的空 playlist，客户端会继续轮询
	// 由 HLS 规范，没有 #EXT-X-ENDLIST 的列表会被视为直播/事件流
	s.logger.Warnf("HLS 首片等待超时: %s/%s，返回占位 playlist", mediaID, quality)
	return "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:EVENT\n", nil
}

// ServeSegment 提供HLS分片文件
func (s *StreamService) ServeSegment(mediaID, quality, segment string, w http.ResponseWriter, r *http.Request) error {
	outputDir := s.transcoder.GetOutputDir(mediaID, quality)
	segPath := filepath.Join(outputDir, segment)

	if _, err := os.Stat(segPath); os.IsNotExist(err) {
		return fmt.Errorf("分片文件不存在: %s", segment)
	}

	http.ServeFile(w, r, segPath)
	return nil
}

// getAvailableQualities 根据媒体信息确定可用质量
// 委托给 TranscodeService 已有的智能实现（会根据原片分辨率筛选，不上采样）
func (s *StreamService) getAvailableQualities(media *model.Media) []string {
	if s.transcoder != nil {
		qs := s.transcoder.GetAvailableQualities(media)
		if len(qs) > 0 {
			return qs
		}
	}
	// fallback
	return []string{"480p", "720p", "1080p"}
}

// estimateBandwidth 估算带宽（bit/s）
func (s *StreamService) estimateBandwidth(bitrate string) int {
	// 简单解析，如 "3000k" -> 3000000
	bitrate = strings.TrimSuffix(bitrate, "k")
	var val int
	fmt.Sscanf(bitrate, "%d", &val)
	return val * 1000
}

// ==================== STRM 远程流代理 ====================

// ProxyRemoteStream 代理远程流媒体（支持 Range 请求，实现拖动进度条）
func (s *StreamService) ProxyRemoteStream(remoteURL string, w http.ResponseWriter, r *http.Request) error {
	// 创建请求，转发客户端的 Range 头
	req, err := http.NewRequestWithContext(r.Context(), "GET", remoteURL, nil)
	if err != nil {
		return fmt.Errorf("创建远程请求失败: %w", err)
	}

	// 转发 Range 头（支持拖动进度条和断点续播）
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	// 设置通用请求头
	req.Header.Set("User-Agent", "nowen-video/1.0")
	if referer := r.Header.Get("Referer"); referer != "" {
		req.Header.Set("Referer", referer)
	}

	client := &http.Client{
		Timeout: 0, // 流媒体不设置超时
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("请求远程流失败: %w", err)
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("远程服务器返回错误: %d", resp.StatusCode)
	}

	// 转发响应头
	if ct := resp.Header.Get("Content-Type"); ct != "" {
		w.Header().Set("Content-Type", ct)
	} else {
		// 根据 URL 推断 Content-Type
		w.Header().Set("Content-Type", s.guessContentType(remoteURL))
	}
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		w.Header().Set("Content-Length", cl)
	}
	if cr := resp.Header.Get("Content-Range"); cr != "" {
		w.Header().Set("Content-Range", cr)
	}
	if ar := resp.Header.Get("Accept-Ranges"); ar != "" {
		w.Header().Set("Accept-Ranges", ar)
	} else {
		w.Header().Set("Accept-Ranges", "bytes")
	}

	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Header().Set("X-Stream-Source", "strm-proxy")

	w.WriteHeader(resp.StatusCode)

	// 流式转发数据
	buf := make([]byte, 128*1024) // 128KB 缓冲区
	_, err = io.CopyBuffer(w, resp.Body, buf)
	if err != nil {
		// 客户端断开连接是正常情况，不记录为错误
		s.logger.Debugf("STRM 代理流传输结束: %v", err)
		return nil
	}

	return nil
}

// guessContentType 根据 URL 推断 MIME 类型
func (s *StreamService) guessContentType(url string) string {
	urlLower := strings.ToLower(url)
	// 去掉查询参数
	if idx := strings.Index(urlLower, "?"); idx > 0 {
		urlLower = urlLower[:idx]
	}

	switch {
	case strings.HasSuffix(urlLower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(urlLower, ".mkv"):
		return "video/x-matroska"
	case strings.HasSuffix(urlLower, ".m3u8"):
		return "application/vnd.apple.mpegurl"
	case strings.HasSuffix(urlLower, ".ts"):
		return "video/mp2t"
	case strings.HasSuffix(urlLower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(urlLower, ".avi"):
		return "video/x-msvideo"
	case strings.HasSuffix(urlLower, ".mov"):
		return "video/quicktime"
	default:
		return "video/mp4" // 默认作为 MP4
	}
}

// RemuxStream 实时将 MKV 等格式 remux 为 fragmented MP4 流式输出（零转码，仅转封装）
// 使用 FFmpeg -c copy 模式，CPU 占用极低，速度接近磁盘 I/O
// 支持 ?start=秒数 参数实现快速 Seek 跳转（类似 Emby 的拖动进度条体验）
func (s *StreamService) RemuxStream(mediaID string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}

	if media.StreamURL != "" {
		return fmt.Errorf("STRM 远程流不支持 remux")
	}

	if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
		return fmt.Errorf("文件不存在: %s", media.FilePath)
	}

	// 使用请求的 context，客户端断开时自动终止 FFmpeg
	ctx := r.Context()

	// 解析前端传来的起始时间参数（支持 Seek 跳转）
	startTime := r.URL.Query().Get("start")

	// 构建 FFmpeg remux 命令
	// -ss: 快速跳转到指定时间（放在 -i 前面实现 input seeking，速度极快）
	// -c copy: 不重新编码，仅转封装
	// -movflags frag_mp4+empty_moov+default_base_moof: 生成 fragmented MP4，支持流式输出
	// -f mp4: 输出 MP4 格式
	// pipe:1: 输出到 stdout
	args := []string{}

	// 如果有 start 参数，利用 FFmpeg 的 -ss 实现快速跳转（必须放在 -i 前面以实现 input seeking）
	if startTime != "" && startTime != "0" {
		args = append(args, "-ss", startTime)
	}

	args = append(args,
		"-i", media.FilePath,
		"-map", "0:v:0", // 仅映射第一个视频流
		"-map", "0:a?", // 映射所有音频流（如无则跳过）
		"-c:v", "copy", // 视频直接复制
		"-c:a", "copy", // 音频直接复制
		"-sn", // 忽略字幕流（避免 ASS/SSA 等不兼容 MP4 的字幕导致失败）
		"-dn", // 忽略数据流
		"-movflags", "frag_mp4+empty_moov+default_base_moof",
		"-f", "mp4",
		"-y",
		"pipe:1",
	)

	s.logger.Debugf("Remux 命令: %s %s", s.cfg.App.FFmpegPath, strings.Join(args, " "))

	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath, args...)

	// 获取 stdout 管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建 stdout 管道失败: %w", err)
	}

	// 捕获 stderr 用于错误诊断（FFmpeg 的错误信息输出到这里）
	var stderrBuf bytes.Buffer
	cmd.Stderr = &stderrBuf

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 remux 失败: %w", err)
	}

	// 设置响应头
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Accept-Ranges", "none") // remux 流不支持 Range 请求
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Stream-Mode", "remux")
	w.Header().Set("Transfer-Encoding", "chunked")

	// 流式转发 FFmpeg 输出到 HTTP 响应
	buf := make([]byte, 256*1024) // 256KB 缓冲区
	_, copyErr := io.CopyBuffer(w, stdout, buf)

	// 等待 FFmpeg 进程结束
	waitErr := cmd.Wait()

	// 客户端断开连接是正常情况（ctx cancel、copy 中断、进程被杀都属于此类）
	if ctx.Err() != nil {
		s.logger.Debugf("Remux 流已断开（客户端关闭）: %s", mediaID)
		return nil
	}

	if copyErr != nil {
		s.logger.Debugf("Remux 流传输结束: %v", copyErr)
		return nil
	}

	if waitErr != nil {
		stderrStr := stderrBuf.String()
		if len(stderrStr) > 500 {
			stderrStr = stderrStr[len(stderrStr)-500:]
		}
		s.logger.Warnf("Remux FFmpeg stderr (last 500 chars): %s", stderrStr)
		return fmt.Errorf("remux 进程异常退出: %w", waitErr)
	}

	return nil
}

// GetMediaStreamURL 获取媒体的远程流 URL（仅用于内部判断）
func (s *StreamService) GetMediaStreamURL(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	return media.StreamURL, nil
}
