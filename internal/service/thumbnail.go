package service

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

// ThumbnailService 关键帧预览图服务
// 使用 FFmpeg 提取视频关键帧生成缩略图条（Sprite Sheet）
// 供前端播放器在进度条悬停时展示预览
type ThumbnailService struct {
	cfg    *config.Config
	logger *zap.SugaredLogger
	mu     sync.Mutex
	cache  map[string]string // mediaID -> sprite 文件路径
}

func NewThumbnailService(cfg *config.Config, logger *zap.SugaredLogger) *ThumbnailService {
	return &ThumbnailService{
		cfg:    cfg,
		logger: logger,
		cache:  make(map[string]string),
	}
}

// GenerateSprite 为指定媒体生成缩略图精灵图
// 默认每 10 秒截取一帧，缩放到 160x90，拼成一个网格图
func (s *ThumbnailService) GenerateSprite(media *model.Media) (string, error) {
	s.mu.Lock()
	if cached, ok := s.cache[media.ID]; ok {
		s.mu.Unlock()
		if _, err := os.Stat(cached); err == nil {
			return cached, nil
		}
	}
	s.mu.Unlock()

	// 创建缩略图目录
	outputDir := filepath.Join(s.cfg.Cache.CacheDir, "thumbnails", media.ID)
	os.MkdirAll(outputDir, 0755)

	spritePath := filepath.Join(outputDir, "sprite.jpg")

	// 计算帧提取间隔（确保不超过100帧）
	interval := 10.0 // 每10秒一帧
	if media.Duration > 1000 {
		interval = media.Duration / 100
	}

	// FFmpeg 命令：每 N 秒截取一帧 → 缩放到 160x90 → 拼成 10 列的 tile
	args := []string{
		"-i", media.FilePath,
		"-vf", fmt.Sprintf("fps=1/%d,scale=160:90,tile=10x10", int(interval)),
		"-frames:v", "1",
		"-q:v", "5",
		"-y",
		spritePath,
	}

	cmd := exec.Command(s.cfg.App.FFmpegPath, args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		s.logger.Debugf("生成缩略图失败: %s - %v\n%s", media.Title, err, string(output))
		return "", fmt.Errorf("生成缩略图失败: %w", err)
	}

	// 缓存结果
	s.mu.Lock()
	s.cache[media.ID] = spritePath
	s.mu.Unlock()

	s.logger.Debugf("缩略图生成完成: %s -> %s", media.Title, spritePath)
	return spritePath, nil
}

// GenerateVTT 生成 WebVTT 格式的缩略图时间轴文件
// 与精灵图配合使用，播放器可以通过 VTT 定位到精灵图中的具体位置
func (s *ThumbnailService) GenerateVTT(media *model.Media) (string, error) {
	outputDir := filepath.Join(s.cfg.Cache.CacheDir, "thumbnails", media.ID)
	os.MkdirAll(outputDir, 0755)

	vttPath := filepath.Join(outputDir, "thumbnails.vtt")

	interval := 10.0
	if media.Duration > 1000 {
		interval = media.Duration / 100
	}

	var sb strings.Builder
	sb.WriteString("WEBVTT\n\n")

	thumbW, thumbH := 160, 90
	cols := 10

	frameCount := int(media.Duration / interval)
	if frameCount > 100 {
		frameCount = 100
	}

	for i := 0; i < frameCount; i++ {
		startTime := float64(i) * interval
		endTime := startTime + interval

		col := i % cols
		row := i / cols

		x := col * thumbW
		y := row * thumbH

		sb.WriteString(fmt.Sprintf("%s --> %s\n",
			formatVTTTime(startTime), formatVTTTime(endTime)))
		sb.WriteString(fmt.Sprintf("sprite.jpg#xywh=%d,%d,%d,%d\n\n",
			x, y, thumbW, thumbH))
	}

	if err := os.WriteFile(vttPath, []byte(sb.String()), 0644); err != nil {
		return "", err
	}

	return vttPath, nil
}

// GetSpritePath 获取缩略图精灵图路径
func (s *ThumbnailService) GetSpritePath(mediaID string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cache[mediaID]
}

// unused: satisfy compiler
var _ = strconv.Atoi
