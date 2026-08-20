package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

var portableHighlightAssetLocks sync.Map
var mediaAnalysisEncoderSupport sync.Map

func portableHighlightAssetLock(key string) *sync.Mutex {
	value, _ := portableHighlightAssetLocks.LoadOrStore(key, &sync.Mutex{})
	return value.(*sync.Mutex)
}

func (s *MediaAnalysisService) ffmpegSupportsEncoder(encoder string) bool {
	encoder = strings.TrimSpace(strings.ToLower(encoder))
	if encoder == "" {
		return false
	}
	key := strings.TrimSpace(s.cfg.App.FFmpegPath) + "::" + encoder
	if cached, ok := mediaAnalysisEncoderSupport.Load(key); ok {
		return cached.(bool)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath, "-hide_banner", "-encoders")
	data, err := cmd.CombinedOutput()
	supported := err == nil && strings.Contains(strings.ToLower(string(data)), encoder)
	mediaAnalysisEncoderSupport.Store(key, supported)
	return supported
}

// EnsureHighlightThumbnailPortable guarantees that every highlight can expose a
// thumbnail even when the host FFmpeg was compiled without libwebp. The normal
// analysis path still prefers WebP; this method is the HTTP-time recovery path
// for minimal FFmpeg builds commonly found on macOS/NAS environments.
func (s *MediaAnalysisService) EnsureHighlightThumbnailPortable(mediaID, highlightID string) (string, error) {
	highlight, err := s.highlightRepo.FindByID(highlightID)
	if err != nil || highlight.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := strings.TrimSpace(highlight.Thumbnail); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}

	lock := portableHighlightAssetLock("thumbnail:" + mediaID + ":" + highlightID)
	lock.Lock()
	defer lock.Unlock()

	highlight, err = s.highlightRepo.FindByID(highlightID)
	if err != nil || highlight.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := strings.TrimSpace(highlight.Thumbnail); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	if err := s.ensureSupported(media); err != nil {
		return "", err
	}

	dir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", mediaID, "portable")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	output := filepath.Join(dir, highlightID+".jpg")
	if err := s.generatePortableHighlightThumbnail(media, highlight, output); err != nil {
		_ = os.Remove(output)
		return "", err
	}

	highlight.Thumbnail = output
	if err := s.highlightRepo.Update(highlight); err != nil {
		_ = os.Remove(output)
		return "", err
	}
	return output, nil
}

func (s *MediaAnalysisService) generatePortableHighlightThumbnail(media *model.Media, highlight *model.VideoHighlight, output string) error {
	middle := (highlight.StartTime + highlight.EndTime) / 2
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", middle), "-i", media.FilePath,
		"-frames:v", "1", "-vf", "scale=640:-2:flags=fast_bilinear",
		"-q:v", "3", "-y", output,
	)
	if data, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("portable thumbnail ffmpeg: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return nil
}

// EnsureHighlightPreviewPortable keeps the distributed Animated WebP path on
// capable servers, but skips it entirely when libwebp is unavailable and emits
// an animated GIF instead. This prevents repeated hover 404s on minimal macOS
// FFmpeg builds while preserving the existing Desktop -> Android -> Server path
// everywhere else.
func (s *MediaAnalysisService) EnsureHighlightPreviewPortable(mediaID, highlightID string) (string, error) {
	highlight, err := s.highlightRepo.FindByID(highlightID)
	if err != nil || highlight.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := existingPreviewPath(highlight); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}

	if s.ffmpegSupportsEncoder("libwebp") {
		if path, distributedErr := s.EnsureHighlightPreviewDistributed(mediaID, highlightID); distributedErr == nil {
			return path, nil
		} else if s.logger != nil {
			s.logger.Debugf("distributed webp preview unavailable, falling back to portable gif media=%s highlight=%s: %v", mediaID, highlightID, distributedErr)
		}
	}

	lock := portableHighlightAssetLock("preview:" + mediaID + ":" + highlightID)
	lock.Lock()
	defer lock.Unlock()

	highlight, err = s.highlightRepo.FindByID(highlightID)
	if err != nil || highlight.MediaID != mediaID {
		return "", gorm.ErrRecordNotFound
	}
	if path := existingPreviewPath(highlight); path != "" {
		if _, statErr := os.Stat(path); statErr == nil {
			return path, nil
		}
	}

	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	if err := s.ensureSupported(media); err != nil {
		return "", err
	}

	dir := filepath.Join(s.cfg.Cache.CacheDir, "media-analysis", mediaID, "previews")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	output := filepath.Join(dir, highlightID+"-preview.gif")
	if err := s.generatePortableHighlightPreview(media, highlight, output); err != nil {
		_ = os.Remove(output)
		return "", err
	}

	highlight.PreviewPath = output
	if err := s.highlightRepo.Update(highlight); err != nil {
		_ = os.Remove(output)
		return "", err
	}
	return output, nil
}

func (s *MediaAnalysisService) generatePortableHighlightPreview(media *model.Media, highlight *model.VideoHighlight, output string) error {
	duration := highlight.EndTime - highlight.StartTime
	if duration < 1 {
		duration = 1
	}
	if duration > 2.5 {
		duration = 2.5
	}
	start := highlight.StartTime + (highlight.EndTime-highlight.StartTime-duration)/2
	if start < highlight.StartTime {
		start = highlight.StartTime
	}

	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, s.cfg.App.FFmpegPath,
		"-hide_banner", "-loglevel", "error",
		"-ss", fmt.Sprintf("%.3f", start), "-i", media.FilePath,
		"-t", fmt.Sprintf("%.2f", duration), "-an",
		"-vf", "fps=5,scale=420:-2:flags=fast_bilinear",
		"-y", output,
	)
	if data, err := cmd.CombinedOutput(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("portable preview ffmpeg: %w: %s", err, strings.TrimSpace(string(data)))
	}
	return nil
}
