package service

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

const artifactPublishResolveRetryDelay = 5 * time.Millisecond

// GetArtifactSegmentPlaylist is the Artifact-aware runtime HLS entry. It never
// captures a mutable physical path across Job submission: every read resolves
// the current Lease-valid staging Artifact or immutable published version.
func (s *StreamService) GetArtifactSegmentPlaylist(mediaID, quality string) (string, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	if media.StreamURL != "" {
		return "", fmt.Errorf("STRM 远程流不支持转码")
	}
	if s.transcoder == nil {
		return "", fmt.Errorf("转码服务不可用")
	}
	if _, ok := qualityPresets[quality]; !ok {
		return "", fmt.Errorf("未知转码档位: %s", quality)
	}

	if content, readErr := s.readResolvedHLSManifest(media, quality); readErr == nil {
		return content, nil
	}
	if _, err := s.transcoder.StartTranscode(media, quality); err != nil {
		return "", fmt.Errorf("启动转码失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.transcoder.WaitForFirstSegmentForMedia(ctx, media, quality); err == nil {
		if content, readErr := s.readResolvedHLSManifest(media, quality); readErr == nil {
			return content, nil
		}
	}

	s.logger.Warnf("HLS Artifact 首片等待超时: %s/%s，返回事件型占位 playlist", mediaID, quality)
	return "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:EVENT\n", nil
}

func (s *StreamService) readResolvedHLSManifest(media *model.Media, quality string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		outputDir, err := s.transcoder.ResolveHLSOutputDir(media, quality)
		if err != nil {
			lastErr = err
		} else {
			manifestPath := filepath.Join(outputDir, "stream.m3u8")
			content, readErr := os.ReadFile(manifestPath)
			if readErr == nil {
				if !strings.Contains(string(content), ".ts") {
					return "", fmt.Errorf("HLS Artifact 尚未包含已完成分片")
				}
				return string(content), nil
			}
			lastErr = readErr
		}
		if attempt == 0 {
			time.Sleep(artifactPublishResolveRetryDelay)
		}
	}
	return "", lastErr
}

// ServeArtifactSegment resolves the Artifact for every request. A segment from
// an abandoned old Attempt becomes unreadable as soon as its Lease is fenced;
// a new Worker can only expose files from its own workspace.
func (s *StreamService) ServeArtifactSegment(mediaID, quality, segment string, w http.ResponseWriter, r *http.Request) error {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return ErrMediaNotFound
	}
	if s.transcoder == nil {
		return fmt.Errorf("转码服务不可用")
	}
	if _, ok := qualityPresets[quality]; !ok {
		return fmt.Errorf("未知转码档位: %s", quality)
	}
	if segment == "" || filepath.Base(segment) != segment || strings.ContainsAny(segment, `/\\`) {
		return fmt.Errorf("无效的分片名: %s", segment)
	}

	segmentPath, outputDir, info, err := s.resolveArtifactFile(media, quality, segment)
	if err != nil {
		return fmt.Errorf("分片文件不存在: %s: %w", segment, err)
	}
	if info.IsDir() || info.Size() <= 0 {
		return fmt.Errorf("分片文件无效: %s", segment)
	}
	if filepath.Ext(segment) == ".ts" {
		w.Header().Set("Content-Type", "video/mp2t")
	}
	if strings.Contains(outputDir, string(filepath.Separator)+"artifacts"+string(filepath.Separator)) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeFile(w, r, segmentPath)
	return nil
}

func (s *StreamService) resolveArtifactFile(media *model.Media, quality, name string) (string, string, os.FileInfo, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		outputDir, err := s.transcoder.ResolveHLSOutputDir(media, quality)
		if err != nil {
			lastErr = err
		} else {
			path := filepath.Join(outputDir, name)
			info, statErr := os.Stat(path)
			if statErr == nil {
				return path, outputDir, info, nil
			}
			lastErr = statErr
		}
		if attempt == 0 {
			time.Sleep(artifactPublishResolveRetryDelay)
		}
	}
	return "", "", nil, lastErr
}
