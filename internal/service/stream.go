package service

import (
	"fmt"
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
	MediaID       string  `json:"media_id"`
	DirectPlayURL string  `json:"direct_play_url"` // 直接播放地址（如果支持）
	HlsURL        string  `json:"hls_url"`         // HLS转码播放地址
	CanDirectPlay bool    `json:"can_direct_play"` // 浏览器是否可直接播放
	FileExt       string  `json:"file_ext"`        // 文件扩展名
	VideoCodec    string  `json:"video_codec"`     // 视频编码
	AudioCodec    string  `json:"audio_codec"`     // 音频编码
	Duration      float64 `json:"duration"`        // 时长（秒）
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
	mediaRepo  *repository.MediaRepo
	transcoder *TranscodeService
	cfg        *config.Config
	logger     *zap.SugaredLogger
}

func NewStreamService(
	mediaRepo *repository.MediaRepo,
	transcoder *TranscodeService,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) *StreamService {
	return &StreamService{
		mediaRepo:  mediaRepo,
		transcoder: transcoder,
		cfg:        cfg,
		logger:     logger,
	}
}

// GetMediaPlayInfo 获取播放信息，前端根据此判断使用哪种播放方式
func (s *StreamService) GetMediaPlayInfo(mediaID string) (*MediaPlayInfo, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
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
	}

	if canDirect {
		info.DirectPlayURL = fmt.Sprintf("/api/stream/%s/direct", mediaID)
	}

	return info, nil
}

// GetDirectStreamInfo 获取直接播放的文件路径和MIME类型
func (s *StreamService) GetDirectStreamInfo(mediaID string) (string, string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", "", ErrMediaNotFound
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
func (s *StreamService) GetPosterPath(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}

	// 优先查找同名海报文件
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

	return "", nil
}

// GetMasterPlaylist 获取HLS主播放列表（多码率自适应）
func (s *StreamService) GetMasterPlaylist(mediaID string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
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
