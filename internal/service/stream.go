package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

// canMediaPlayDirectly 包级判定：判断该媒体是否可在浏览器端零转码播放。
//
// 抽成包级函数的原因：PreprocessService 需要在"自动批量提交"路径上据此跳过
// 已经能直接播放/Remux 的媒体，但 PreprocessService 与 StreamService 之间
// 不宜增加方法调用依赖，因此两者都走这个包级函数，语义保持一致。
//
// 判定等级（任一满足即返回 true）：
//  1. 容器在 directPlayableExts（mp4/webm/m4v），视频+音频编码兼容浏览器
//     → 浏览器原生 Direct Play
//  2. 容器在 remuxableExts（mkv/avi/mov/flv/wmv/ts），且内部编码兼容浏览器
//     → 可通过零转码 Remux 秒开
//
// 返回 false 的典型场景：
//   - STRM 远程流（预处理不适用）
//   - 容器是 rmvb/vob/iso/3gp 等非白名单
//   - 编码是 MPEG-2 / WMV3 / DTS / TrueHD 等浏览器不支持的格式
//   - 媒体元信息缺失（codec 字段为空，保守起见认为需要预处理）
func canMediaPlayDirectly(media *model.Media) bool {
	if media == nil {
		return false
	}
	// STRM 远程流走独立通道，这里统一返回 false
	if media.StreamURL != "" {
		return false
	}
	ext := strings.ToLower(filepath.Ext(media.FilePath))
	vc := strings.ToLower(media.VideoCodec)
	ac := strings.ToLower(media.AudioCodec)

	// 编码信息缺失时保守处理：还没探测出来 → 允许预处理
	if vc == "" {
		return false
	}
	if !browserCompatibleVideoCodecs[vc] {
		return false
	}
	if ac != "" && !browserCompatibleAudioCodecs[ac] {
		return false
	}

	// 等级 1：mp4/webm/m4v 直接播
	if directPlayableExts[ext] {
		return true
	}
	// 等级 2：mkv/avi/mov 等可零转码 Remux
	if remuxableExts[ext] {
		return true
	}
	return false
}

// CanPlayDirectlyInBrowser 判断当前 media 在浏览器端是否可以"零转码播放"。
//
// 实现委托给包级 canMediaPlayDirectly，保证与 PreprocessService
// 自动跳过逻辑共享同一套判定规则。
func (s *StreamService) CanPlayDirectlyInBrowser(media *model.Media) bool {
	return canMediaPlayDirectly(media)
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
//
// 行为：
//  1. 根据原始分辨率筛选可用档位（不上采样）
//  2. 【ABR 核心】并行预转码所有档位，最低档最早产出保证可播放下限，
//     hls.js 会在客户端带宽允许时无缝切换到更高档位
//  3. 返回 Master Playlist，每个 #EXT-X-STREAM-INF 带 BANDWIDTH/RESOLUTION
func (s *StreamService) GetMasterPlaylist(mediaID string) (string, error) {
	return s.GetMasterPlaylistFiltered(mediaID, 0)
}

// GetMasterPlaylistFiltered 带带宽上限过滤的 Master Playlist 生成。
//
// maxBitrate > 0 时，只返回码率 <= maxBitrate 的档位（单位 bit/s）。
// 该参数来自：
//   - 前端 hls.js 的 bandwidth-report 上报（弱网环境自动降档）
//   - Emby/Infuse 客户端 PlaybackInfo 请求里的 MaxStreamingBitrate 字段
//
// 这样前端/客户端就能在请求 master.m3u8 时通过 ?maxBitrate=xxx 约束返回档位，
// 避免 hls.js 在高档和低档之间频繁抖动切换。
func (s *StreamService) GetMasterPlaylistFiltered(mediaID string, maxBitrate int) (string, error) {
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

	// 根据 maxBitrate 过滤（超过客户端上限的档位不提供）
	if maxBitrate > 0 {
		filtered := make([]string, 0, len(qualities))
		for _, q := range qualities {
			preset, ok := qualityPresets[q]
			if !ok {
				continue
			}
			if s.estimateBandwidth(preset.VideoBitrate) <= maxBitrate {
				filtered = append(filtered, q)
			}
		}
		if len(filtered) > 0 {
			qualities = filtered
		} else if len(qualities) > 0 {
			// 所有档位都超过上限时，至少保留最低档，避免返回空列表
			qualities = qualities[:1]
		}
	}

	// 【ABR 核心】触发并行预转码所有档位
	// startTranscodeInternal 内部会去重（同 media+quality 已有 job 直接复用），
	// 所以重复调用（客户端切换档位时重新请求 master.m3u8）不会导致重复转码。
	if s.transcoder != nil {
		go func() {
			if _, err := s.transcoder.StartABRTranscode(media, qualities); err != nil {
				s.logger.Debugf("ABR 预转码提交失败（可能只因所有档位已完成）: %v", err)
			}
		}()
	}

	// 多音轨支持：探测音轨，≥2 条时输出 #EXT-X-MEDIA:TYPE=AUDIO
	// 单音轨的 media 保持原有行为（音频走主转码的 .ts 里）
	audioTracks := s.GetAudioTracks(mediaID)
	hasMultiAudio := len(audioTracks) >= 2

	// 生成master.m3u8
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")

	if hasMultiAudio {
		builder.WriteString(s.BuildAudioMediaEntries(mediaID, audioTracks))
	}

	for _, q := range qualities {
		preset := qualityPresets[q]
		bandwidth := s.estimateBandwidth(preset.VideoBitrate)
		streamInf := fmt.Sprintf(
			"#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%dx%d,NAME=\"%s\"",
			bandwidth, preset.Width, preset.Height, q,
		)
		if hasMultiAudio {
			streamInf += `,AUDIO="audio"`
		}
		builder.WriteString(streamInf + "\n")
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
//
// 向下兼容：无自定义 Header 时退化为原有逻辑。
func (s *StreamService) ProxyRemoteStream(remoteURL string, w http.ResponseWriter, r *http.Request) error {
	return s.proxyRemoteStreamWithHeaders(remoteURL, "", w, r, nil)
}

// ProxyRemoteStreamForMedia 基于 mediaID 的代理入口，会自动加载 media 的 UA/Referer/Cookie/Header
// 并对 HLS 子 playlist 执行 URL 重写（分片仍走后端代理，继承鉴权）。
func (s *StreamService) ProxyRemoteStreamForMedia(mediaID string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	if media.StreamURL == "" {
		return fmt.Errorf("media %s 不是 STRM 流", mediaID)
	}
	ua, referer, cookie, extra := ResolveSTRMHeaders(s.strmCfg(), media)
	hdr := buildSTRMRequestHeader(ua, referer, cookie, extra)

	// 若客户端请求的是 HLS manifest，走文本代理并重写 URL
	if s.cfg != nil && s.cfg.STRM.RewriteHLS && looksLikeHLSURL(media.StreamURL) {
		return s.proxyHLSPlaylist(mediaID, media.StreamURL, hdr, w, r)
	}
	return s.proxyRemoteStreamWithHeaders(media.StreamURL, cookie, w, r, hdr)
}

// ProxySTRMSegment 基于 media 的任意子资源代理（HLS 分片 / key 文件 / 相邻 playlist）
// 目标 URL 通过 query ?u=<base64url> 或 ?target=<absoluteURL> 传入。
func (s *StreamService) ProxySTRMSegment(mediaID, target string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	if media.StreamURL == "" {
		return fmt.Errorf("media %s 不是 STRM 流", mediaID)
	}
	if target == "" {
		return fmt.Errorf("missing target url")
	}
	// 解析：支持 base64 / 绝对 / 相对
	absURL, err := resolveSTRMSegmentURL(media.StreamURL, target)
	if err != nil {
		return err
	}
	ua, referer, cookie, extra := ResolveSTRMHeaders(s.strmCfg(), media)
	hdr := buildSTRMRequestHeader(ua, referer, cookie, extra)

	// 子 manifest 也需要递归重写
	if looksLikeHLSURL(absURL) {
		return s.proxyHLSPlaylist(mediaID, absURL, hdr, w, r)
	}
	return s.proxyRemoteStreamWithHeaders(absURL, cookie, w, r, hdr)
}

// CheckSTRMHealth 对远程流做一次 HEAD/GET 预探测，返回状态信息（供前端诊断面板使用）
type STRMHealthResult struct {
	MediaID       string            `json:"media_id"`
	URL           string            `json:"url"`
	StatusCode    int               `json:"status_code"`
	OK            bool              `json:"ok"`
	ContentType   string            `json:"content_type,omitempty"`
	ContentLength int64             `json:"content_length,omitempty"`
	AcceptRanges  string            `json:"accept_ranges,omitempty"`
	ResponseMS    int64             `json:"response_ms"`
	Error         string            `json:"error,omitempty"`
	EffectiveURL  string            `json:"effective_url,omitempty"`
	Headers       map[string]string `json:"headers,omitempty"`
}

func (s *StreamService) CheckSTRMHealth(mediaID string) (*STRMHealthResult, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	if media.StreamURL == "" {
		return nil, fmt.Errorf("media %s 不是 STRM 流", mediaID)
	}
	ua, referer, cookie, extra := ResolveSTRMHeaders(s.strmCfg(), media)
	hdr := buildSTRMRequestHeader(ua, referer, cookie, extra)

	start := time.Now()
	result := &STRMHealthResult{
		MediaID: mediaID,
		URL:     media.StreamURL,
		Headers: map[string]string{},
	}

	client := &http.Client{
		Timeout: time.Duration(s.strmConnectTimeoutSec()) * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("重定向次数过多")
			}
			return nil
		},
	}

	// 先 HEAD；HEAD 被拒绝时回退小范围 GET
	tryMethod := func(method string) (*http.Response, error) {
		req, err := http.NewRequest(method, media.StreamURL, nil)
		if err != nil {
			return nil, err
		}
		for k, vs := range hdr {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
		if method == "GET" {
			req.Header.Set("Range", "bytes=0-0")
		}
		return client.Do(req)
	}

	resp, err := tryMethod("HEAD")
	if err != nil || (resp != nil && resp.StatusCode >= 400) {
		if resp != nil {
			resp.Body.Close()
		}
		resp, err = tryMethod("GET")
	}
	result.ResponseMS = time.Since(start).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result, nil
	}
	defer resp.Body.Close()
	result.StatusCode = resp.StatusCode
	result.OK = resp.StatusCode >= 200 && resp.StatusCode < 400
	result.ContentType = resp.Header.Get("Content-Type")
	result.AcceptRanges = resp.Header.Get("Accept-Ranges")
	if cl := resp.Header.Get("Content-Length"); cl != "" {
		var n int64
		fmt.Sscanf(cl, "%d", &n)
		result.ContentLength = n
	}
	if resp.Request != nil && resp.Request.URL != nil {
		result.EffectiveURL = resp.Request.URL.String()
	}
	for k, vs := range resp.Header {
		if len(vs) > 0 {
			result.Headers[k] = vs[0]
		}
	}
	return result, nil
}

// proxyRemoteStreamWithHeaders 实际的字节级透明代理（带自定义 Header）
func (s *StreamService) proxyRemoteStreamWithHeaders(remoteURL, cookie string, w http.ResponseWriter, r *http.Request, extra http.Header) error {
	req, err := http.NewRequestWithContext(r.Context(), "GET", remoteURL, nil)
	if err != nil {
		return fmt.Errorf("创建远程请求失败: %w", err)
	}

	// 转发 Range 头（支持拖动进度条和断点续播）
	if rangeHeader := r.Header.Get("Range"); rangeHeader != "" {
		req.Header.Set("Range", rangeHeader)
	}

	// 注入自定义 Header
	if extra != nil {
		for k, vs := range extra {
			for _, v := range vs {
				req.Header.Add(k, v)
			}
		}
	}
	if cookie != "" && req.Header.Get("Cookie") == "" {
		req.Header.Set("Cookie", cookie)
	}
	if req.Header.Get("User-Agent") == "" {
		req.Header.Set("User-Agent", s.defaultUA())
	}
	// 兜底：外部无 Referer 时尝试使用请求来源
	if req.Header.Get("Referer") == "" {
		if referer := r.Header.Get("Referer"); referer != "" {
			req.Header.Set("Referer", referer)
		}
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
		s.logger.Debugf("STRM 代理流传输结束: %v", err)
		return nil
	}

	return nil
}

// proxyHLSPlaylist 拉取远程 .m3u8 并把所有相对/绝对分片/子 playlist URL 重写为后端代理路径
// 这样客户端请求分片时也会带上媒体对应的 UA/Referer/Cookie，解决跨域/鉴权问题。
func (s *StreamService) proxyHLSPlaylist(mediaID, remoteURL string, hdr http.Header, w http.ResponseWriter, r *http.Request) error {
	client := &http.Client{
		Timeout: time.Duration(s.strmConnectTimeoutSec()) * time.Second,
	}
	req, err := http.NewRequestWithContext(r.Context(), "GET", remoteURL, nil)
	if err != nil {
		return err
	}
	for k, vs := range hdr {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("拉取 HLS manifest 失败: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HLS manifest 返回错误: %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	rewritten := rewriteHLSPlaylist(string(body), remoteURL, mediaID)
	w.Header().Set("Content-Type", "application/vnd.apple.mpegurl")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Stream-Source", "strm-hls-proxy")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(rewritten))
	return nil
}

// strmConnectTimeoutSec 读取配置中的 STRM 连接超时
func (s *StreamService) strmConnectTimeoutSec() int {
	if s.cfg != nil && s.cfg.STRM.ConnectTimeout > 0 {
		return s.cfg.STRM.ConnectTimeout
	}
	return 30
}

// strmCfg 返回 STRM 配置指针（可能为空配置但非 nil）
func (s *StreamService) strmCfg() *config.STRMConfig {
	if s.cfg == nil {
		return nil
	}
	return &s.cfg.STRM
}

func (s *StreamService) defaultUA() string {
	if s.cfg != nil && s.cfg.STRM.DefaultUserAgent != "" {
		return s.cfg.STRM.DefaultUserAgent
	}
	return "nowen-video/1.0"
}

// buildSTRMRequestHeader 构造统一的请求头
func buildSTRMRequestHeader(ua, referer, cookie string, extra map[string]string) http.Header {
	h := http.Header{}
	if ua != "" {
		h.Set("User-Agent", ua)
	}
	if referer != "" {
		h.Set("Referer", referer)
	}
	if cookie != "" {
		h.Set("Cookie", cookie)
	}
	for k, v := range extra {
		if v != "" {
			h.Set(k, v)
		}
	}
	return h
}

// looksLikeHLSURL 粗略判断是否为 HLS 流
func looksLikeHLSURL(u string) bool {
	lower := strings.ToLower(u)
	if idx := strings.Index(lower, "?"); idx > 0 {
		lower = lower[:idx]
	}
	return strings.Contains(lower, ".m3u8")
}

// rewriteHLSPlaylist 把 playlist 中的 URI 改写为 /api/stream/:id/strm-seg?u=...
// 处理：
//   - 纯 URL 行（非以 # 开头）
//   - #EXT-X-KEY:URI="..."（加密密钥）
//   - #EXT-X-MEDIA:URI="..."（audio / subtitle）
//   - #EXT-X-MAP:URI="..."（fMP4 init 段）
//   - #EXT-X-STREAM-INF 紧跟的 URI 行（下一行）
func rewriteHLSPlaylist(body, baseURL, mediaID string) string {
	base, _ := url.Parse(baseURL)
	var out strings.Builder
	out.Grow(len(body) + 128)
	lines := strings.Split(body, "\n")
	for _, raw := range lines {
		line := strings.TrimRight(raw, "\r")
		trim := strings.TrimSpace(line)
		if trim == "" {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if strings.HasPrefix(trim, "#") {
			// 替换形如 URI="xxx" 的部分
			if idx := strings.Index(trim, `URI="`); idx > 0 {
				rest := trim[idx+len(`URI="`):]
				if end := strings.Index(rest, `"`); end >= 0 {
					orig := rest[:end]
					rewritten := buildSegProxyURL(mediaID, resolveURL(base, orig))
					newLine := trim[:idx+len(`URI="`)] + rewritten + `"` + rest[end+1:]
					out.WriteString(newLine)
					out.WriteByte('\n')
					continue
				}
			}
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		// 非注释行 = 分片或子 playlist 的 URI
		abs := resolveURL(base, trim)
		out.WriteString(buildSegProxyURL(mediaID, abs))
		out.WriteByte('\n')
	}
	return out.String()
}

// resolveURL 把 ref 解析为绝对 URL
func resolveURL(base *url.URL, ref string) string {
	if base == nil {
		return ref
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ref
	}
	if u.IsAbs() {
		return ref
	}
	return base.ResolveReference(u).String()
}

// buildSegProxyURL 构造后端代理的分片 URL（使用 query 参数 u 传递 base64 后的目标 URL）
func buildSegProxyURL(mediaID, target string) string {
	encoded := base64URLEncode(target)
	return fmt.Sprintf("/api/stream/%s/strm-seg?u=%s", mediaID, encoded)
}

func base64URLEncode(s string) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(s))
}

func base64URLDecode(s string) (string, error) {
	b, err := base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(s)
	if err != nil {
		// 兼容带 padding 的变体
		if b2, err2 := base64.URLEncoding.DecodeString(s); err2 == nil {
			return string(b2), nil
		}
		return "", err
	}
	return string(b), nil
}

// resolveSTRMSegmentURL 根据 target 参数（base64url 或裸 URL）解析出绝对 URL
func resolveSTRMSegmentURL(baseURL, target string) (string, error) {
	// 先按 base64 解码尝试
	if decoded, err := base64URLDecode(target); err == nil && (strings.HasPrefix(decoded, "http://") || strings.HasPrefix(decoded, "https://")) {
		return decoded, nil
	}
	// 绝对 URL 直接用
	if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
		return target, nil
	}
	// 否则作为相对路径
	base, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u, err := url.Parse(target)
	if err != nil {
		return "", err
	}
	return base.ResolveReference(u).String(), nil
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
