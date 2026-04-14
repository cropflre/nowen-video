package service

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	PreferDirectPlay bool    `json:"prefer_direct_play"` // 系统设置：优先直接播放（禁用自动转码）
}

// 浏览器可直接播放的文件格式
var directPlayableExts = map[string]bool{
	".mp4":  true,
	".webm": true,
	".m4v":  true,
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

// SetPreprocessService 注入预处理服务（延迟注入，避免循环依赖）
func (s *StreamService) SetPreprocessService(ps *PreprocessService) {
	s.preprocess = ps
}

// SetSettingRepo 注入系统设置仓储（延迟注入）
func (s *StreamService) SetSettingRepo(repo *repository.SystemSettingRepo) {
	s.settingRepo = repo
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

	// 检查文件是否存在
	if _, err := os.Stat(media.FilePath); os.IsNotExist(err) {
		return "", "", fmt.Errorf("文件不存在: %s", media.FilePath)
	}

	ext := strings.ToLower(filepath.Ext(media.FilePath))
	contentType, ok := mimeTypes[ext]
	if !ok {
		contentType = "application/octet-stream"
	}

	return media.FilePath, contentType, nil
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
		if _, err := os.Stat(media.PosterPath); err == nil {
			return media.PosterPath, nil
		}
	}

	// 查找同目录下的海报文件
	dir := filepath.Dir(media.FilePath)
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
			candidate := filepath.Join(dir, name+ext)
			if _, err := os.Stat(candidate); err == nil {
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
				if _, err := os.Stat(series.PosterPath); err == nil {
					return series.PosterPath, nil
				}
			}
			// 查找 Series 根目录下的海报文件
			if series.FolderPath != "" {
				seriesPosterNames := []string{"poster", "cover", "folder", "show"}
				for _, name := range seriesPosterNames {
					for _, ext := range posterExts {
						candidate := filepath.Join(series.FolderPath, name+ext)
						if _, err := os.Stat(candidate); err == nil {
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

	// 检查是否已有转码缓存
	if _, err := os.Stat(m3u8Path); err == nil {
		content, err := os.ReadFile(m3u8Path)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	// 触发转码
	_, err = s.transcoder.StartTranscode(media, quality)
	if err != nil {
		return "", fmt.Errorf("启动转码失败: %w", err)
	}

	// 返回一个等待中的播放列表（前端会重试）
	return "#EXTM3U\n#EXT-X-TARGETDURATION:6\n#EXT-X-PLAYLIST-TYPE:EVENT\n", nil
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
func (s *StreamService) getAvailableQualities(media *model.Media) []string {
	// TODO: 根据原始视频分辨率智能选择
	// 目前默认提供三种质量
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

// GetMediaStreamURL 获取媒体的远程流 URL（仅用于内部判断）
func (s *StreamService) GetMediaStreamURL(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	return media.StreamURL, nil
}
