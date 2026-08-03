package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

const artifactPublishResolveRetryDelay = 5 * time.Millisecond

// HLSArtifactVersionQuery is attached to every managed HLS media URI. It pins
// subsequent segment requests to the exact immutable/still-live Artifact that
// supplied the playlist rather than resolving whichever version is current at
// request time.
const HLSArtifactVersionQuery = "artifact"

var ErrArtifactNotReady = errors.New("transcode artifact is not ready")

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
	} else if !errors.Is(readErr, ErrArtifactNotReady) {
		return "", fmt.Errorf("读取转码 Artifact 失败: %w", readErr)
	}
	if _, err := s.transcoder.StartTranscode(media, quality); err != nil {
		return "", fmt.Errorf("启动转码失败: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.transcoder.WaitForFirstSegmentForMedia(ctx, media, quality); err == nil {
		if content, readErr := s.readResolvedHLSManifest(media, quality); readErr == nil {
			return content, nil
		} else if !errors.Is(readErr, ErrArtifactNotReady) {
			return "", fmt.Errorf("读取首片 Artifact 失败: %w", readErr)
		}
	}

	s.logger.Warnf("HLS Artifact 首片等待超时: %s/%s，返回事件型占位 playlist", mediaID, quality)
	return "#EXTM3U\n#EXT-X-VERSION:3\n#EXT-X-TARGETDURATION:2\n#EXT-X-MEDIA-SEQUENCE:0\n#EXT-X-PLAYLIST-TYPE:EVENT\n", nil
}

func (s *StreamService) readResolvedHLSManifest(media *model.Media, quality string) (string, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		artifact, outputDir, err := s.transcoder.resolveHLSArtifactSnapshot(media, quality)
		if err != nil {
			lastErr = err
		} else {
			manifestPath := filepath.Join(outputDir, "stream.m3u8")
			content, readErr := os.ReadFile(manifestPath)
			if readErr == nil {
				if !strings.Contains(string(content), ".ts") {
					return "", ErrArtifactNotReady
				}
				versioned, bindErr := bindHLSArtifactVersion(string(content), artifact.ID)
				if bindErr != nil {
					return "", bindErr
				}
				// Persist a throttled access signal before returning the versioned
				// playlist. Pressure cleanup therefore protects the full playback
				// window, not only the currently open segment descriptor.
				s.transcoder.TouchArtifactAccess(artifact.ID)
				return versioned, nil
			}
			lastErr = readErr
		}
		if attempt == 0 && artifactReadinessError(lastErr) {
			time.Sleep(artifactPublishResolveRetryDelay)
			continue
		}
		break
	}
	if artifactReadinessError(lastErr) {
		return "", ErrArtifactNotReady
	}
	return "", lastErr
}

func bindHLSArtifactVersion(content, artifactID string) (string, error) {
	if strings.TrimSpace(artifactID) == "" {
		return "", fmt.Errorf("artifact identity is required for managed HLS playlist")
	}
	lines := strings.Split(content, "\n")
	for index, line := range lines {
		carriageReturn := ""
		if strings.HasSuffix(line, "\r") {
			line = strings.TrimSuffix(line, "\r")
			carriageReturn = "\r"
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		parsed, err := url.Parse(trimmed)
		if err != nil {
			return "", fmt.Errorf("parse HLS media URI %q: %w", trimmed, err)
		}
		if parsed.IsAbs() || parsed.Host != "" || parsed.Path == "" || filepath.Base(parsed.Path) != parsed.Path || strings.ContainsAny(parsed.Path, `/\\`) {
			return "", fmt.Errorf("managed HLS media URI is not a local basename: %s", trimmed)
		}
		query := parsed.Query()
		query.Set(HLSArtifactVersionQuery, artifactID)
		parsed.RawQuery = query.Encode()
		lines[index] = parsed.String() + carriageReturn
	}
	return strings.Join(lines, "\n"), nil
}

func (s *StreamService) ServeArtifactSegment(mediaID, quality, segment string, w http.ResponseWriter, r *http.Request) error {
	return s.ServeArtifactSegmentVersion(mediaID, quality, "", segment, w, r)
}

func (s *StreamService) ServeArtifactSegmentVersion(mediaID, quality, artifactID, segment string, w http.ResponseWriter, r *http.Request) error {
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

	segmentPath, outputDir, artifact, info, err := s.resolveArtifactFile(media, quality, artifactID, segment)
	if err != nil {
		return err
	}
	if info.IsDir() || info.Size() <= 0 {
		return ErrArtifactNotReady
	}

	// Open the file before sending headers. On Unix an already-open descriptor
	// remains readable if retention cleanup unlinks the path; on Windows and
	// mounts that reject deletion of an open file, cleanup persists a busy retry.
	// Either behavior protects an in-flight response from a partial transfer.
	file, openErr := os.Open(segmentPath)
	if openErr != nil {
		if artifactReadinessError(openErr) {
			return ErrArtifactNotReady
		}
		return openErr
	}
	defer file.Close()
	openedInfo, statErr := file.Stat()
	if statErr != nil {
		return statErr
	}
	if openedInfo.IsDir() || openedInfo.Size() <= 0 {
		return ErrArtifactNotReady
	}

	if artifact != nil && artifact.ID != "" {
		s.transcoder.TouchArtifactAccess(artifact.ID)
	}
	if filepath.Ext(segment) == ".ts" {
		w.Header().Set("Content-Type", "video/mp2t")
	}
	if artifact != nil && artifact.ID != "" {
		w.Header().Set("X-Nowen-Artifact-ID", artifact.ID)
	}
	if artifact != nil && (artifact.Status == "published" || artifact.Status == "superseded") {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else if strings.Contains(outputDir, string(filepath.Separator)+"artifacts"+string(filepath.Separator)) {
		w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	} else {
		w.Header().Set("Cache-Control", "no-cache")
	}
	http.ServeContent(w, r, segment, openedInfo.ModTime(), file)
	return nil
}

func (s *StreamService) resolveArtifactFile(media *model.Media, quality, artifactID, name string) (string, string, *model.TranscodeArtifactRecord, os.FileInfo, error) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		var artifact *model.TranscodeArtifactRecord
		var outputDir string
		var err error
		if artifactID != "" {
			artifact, outputDir, err = s.transcoder.resolveHLSArtifactVersion(media, quality, artifactID)
		} else {
			artifact, outputDir, err = s.transcoder.resolveHLSArtifactSnapshot(media, quality)
		}
		if err != nil {
			lastErr = err
		} else {
			path := filepath.Join(outputDir, name)
			info, statErr := os.Stat(path)
			if statErr == nil {
				return path, outputDir, artifact, info, nil
			}
			lastErr = statErr
		}
		if attempt == 0 && artifactReadinessError(lastErr) {
			time.Sleep(artifactPublishResolveRetryDelay)
			continue
		}
		break
	}
	if artifactReadinessError(lastErr) {
		return "", "", nil, nil, ErrArtifactNotReady
	}
	return "", "", nil, nil, lastErr
}

func artifactReadinessError(err error) bool {
	return err == nil ||
		errors.Is(err, ErrArtifactNotReady) ||
		errors.Is(err, gorm.ErrRecordNotFound) ||
		errors.Is(err, os.ErrNotExist)
}
