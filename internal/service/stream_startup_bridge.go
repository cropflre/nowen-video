package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

const startupBridgeProfilePrefix = "startup-"
const startupBridgeSegmentPrefix = "startup__"
const startupContinuationSegmentPrefix = "continuation__"

// StartupBridgeInfo is safe for API responses. It intentionally excludes
// artifact filesystem paths and internal Job identifiers.
type StartupBridgeInfo struct {
	Available   bool   `json:"available"`
	ProfileID   string `json:"profile_id,omitempty"`
	DurationMS  int64  `json:"duration_ms,omitempty"`
	PlaylistURL string `json:"playlist_url,omitempty"`
}

type StartupBridgeFile struct {
	Path      string
	Immutable bool
}

type hlsPlaylistSnapshot struct {
	TargetDuration int
	Segments       []hlsPlaylistSegment
	EndList        bool
}

type hlsPlaylistSegment struct {
	Duration string
	URI      string
}

func (s *StreamService) GetStartupBridgeInfo(mediaID string) (*StartupBridgeInfo, error) {
	if s == nil || s.mediaRepo == nil || s.transcoder == nil {
		return &StartupBridgeInfo{}, nil
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	startup, err := s.transcoder.ResolvePublishedStartupStream(media)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &StartupBridgeInfo{}, nil
		}
		return nil, err
	}
	return &StartupBridgeInfo{
		Available:   true,
		ProfileID:   startup.ProfileID,
		DurationMS:  startup.DurationMS,
		PlaylistURL: startupBridgePlaylistURL(media.ID, startup.ProfileID),
	}, nil
}

// GetStartupBridgePlaylist exposes one append-only EVENT timeline. Startup
// segments are immutable and immediately available; continuation is submitted
// through the durable orchestrator and appended as its Lease-valid Artifact
// grows. The bridge never writes into either Artifact directory.
func (s *StreamService) GetStartupBridgePlaylist(mediaID, profileID string) (string, error) {
	if s == nil || s.mediaRepo == nil || s.transcoder == nil {
		return "", gorm.ErrRecordNotFound
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return "", ErrMediaNotFound
	}
	startup, err := s.transcoder.ResolvePublishedStartupStream(media)
	if err != nil {
		return "", err
	}
	if startup.ProfileID != profileID {
		return "", gorm.ErrRecordNotFound
	}
	startupPlaylist, err := readHLSPlaylist(startup.ManifestPath)
	if err != nil {
		return "", fmt.Errorf("read startup playlist: %w", err)
	}
	if len(startupPlaylist.Segments) == 0 {
		return "", fmt.Errorf("startup playlist contains no segments")
	}

	// Submission is idempotent. A transient queue/database error does not hide
	// the already-published startup media; the next playlist reload retries.
	if _, submitErr := s.transcoder.SubmitStartupContinuation(media, startup); submitErr != nil && !errors.Is(submitErr, gorm.ErrRecordNotFound) {
		if s.logger != nil {
			s.logger.Warnf("提交 Startup Continuation 失败 media=%s profile=%s: %v", media.ID, profileID, submitErr)
		}
	}

	var continuation *hlsPlaylistSnapshot
	if artifact, resolveErr := s.transcoder.ResolveReadableStartupContinuation(media, startup); resolveErr == nil && artifact != nil {
		manifestPath := artifact.ManifestPath
		if manifestPath == "" {
			directory := readableArtifactDirectory(artifact)
			if directory != "" {
				manifestPath = filepath.Join(directory, "stream.m3u8")
			}
		}
		if manifestPath != "" {
			if snapshot, readErr := readHLSPlaylist(manifestPath); readErr == nil && len(snapshot.Segments) > 0 {
				continuation = &snapshot
			}
		}
	} else if resolveErr != nil && !errors.Is(resolveErr, gorm.ErrRecordNotFound) && s.logger != nil {
		s.logger.Warnf("解析 Startup Continuation Artifact 失败 media=%s: %v", media.ID, resolveErr)
	}

	return buildStartupBridgePlaylist(media.ID, profileID, startupPlaylist, continuation), nil
}

func (s *StreamService) ResolveStartupBridgeSegment(mediaID, profileID, segment string) (*StartupBridgeFile, error) {
	_, startup, err := s.resolveStartupDescriptor(mediaID, profileID)
	if err != nil {
		return nil, err
	}
	path, err := safeHLSSegmentPath(startup.OutputDir, segment)
	if err != nil {
		return nil, err
	}
	return &StartupBridgeFile{Path: path, Immutable: true}, nil
}

func (s *StreamService) ResolveStartupContinuationSegment(mediaID, profileID, segment string) (*StartupBridgeFile, error) {
	media, startup, err := s.resolveStartupDescriptor(mediaID, profileID)
	if err != nil {
		return nil, err
	}
	artifact, err := s.transcoder.ResolveReadableStartupContinuation(media, startup)
	if err != nil {
		return nil, err
	}
	directory := readableArtifactDirectory(artifact)
	if directory == "" {
		return nil, gorm.ErrRecordNotFound
	}
	path, err := safeHLSSegmentPath(directory, segment)
	if err != nil {
		return nil, err
	}
	return &StartupBridgeFile{Path: path, Immutable: artifact.Status == "published"}, nil
}

func (s *StreamService) resolveStartupDescriptor(mediaID, profileID string) (*model.Media, *StartupStreamDescriptor, error) {
	if s == nil || s.mediaRepo == nil || s.transcoder == nil {
		return nil, nil, gorm.ErrRecordNotFound
	}
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, nil, ErrMediaNotFound
	}
	startup, err := s.transcoder.ResolvePublishedStartupStream(media)
	if err != nil {
		return nil, nil, err
	}
	if startup.ProfileID != profileID {
		return nil, nil, gorm.ErrRecordNotFound
	}
	return media, startup, nil
}

func readableArtifactDirectory(artifact *model.TranscodeArtifactRecord) string {
	if artifact == nil {
		return ""
	}
	switch artifact.Status {
	case "staging":
		return artifact.TempPath
	case "publishing":
		if artifact.Path != "" {
			if _, err := os.Stat(artifact.Path); err == nil {
				return artifact.Path
			}
		}
		return artifact.TempPath
	case "published":
		return artifact.Path
	default:
		return ""
	}
}

func readHLSPlaylist(path string) (hlsPlaylistSnapshot, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return hlsPlaylistSnapshot{}, err
	}
	lines := strings.Split(strings.ReplaceAll(string(content), "\r\n", "\n"), "\n")
	snapshot := hlsPlaylistSnapshot{}
	pendingDuration := ""
	for _, rawLine := range lines {
		line := strings.TrimSpace(rawLine)
		switch {
		case strings.HasPrefix(line, "#EXT-X-TARGETDURATION:"):
			value := strings.TrimPrefix(line, "#EXT-X-TARGETDURATION:")
			target, _ := strconv.Atoi(strings.TrimSpace(value))
			if target > snapshot.TargetDuration {
				snapshot.TargetDuration = target
			}
		case strings.HasPrefix(line, "#EXTINF:"):
			pendingDuration = line
		case line == "#EXT-X-ENDLIST":
			snapshot.EndList = true
		case line == "" || strings.HasPrefix(line, "#"):
			continue
		default:
			if pendingDuration == "" {
				continue
			}
			snapshot.Segments = append(snapshot.Segments, hlsPlaylistSegment{Duration: pendingDuration, URI: line})
			pendingDuration = ""
		}
	}
	if snapshot.TargetDuration <= 0 {
		snapshot.TargetDuration = hlsTargetSegmentSeconds
	}
	return snapshot, nil
}

func buildStartupBridgePlaylist(
	mediaID,
	profileID string,
	startup hlsPlaylistSnapshot,
	continuation *hlsPlaylistSnapshot,
) string {
	targetDuration := startup.TargetDuration
	if continuation != nil && continuation.TargetDuration > targetDuration {
		targetDuration = continuation.TargetDuration
	}
	if targetDuration <= 0 {
		targetDuration = hlsTargetSegmentSeconds
	}
	var builder strings.Builder
	builder.WriteString("#EXTM3U\n")
	builder.WriteString("#EXT-X-VERSION:3\n")
	builder.WriteString(fmt.Sprintf("#EXT-X-TARGETDURATION:%d\n", targetDuration))
	builder.WriteString("#EXT-X-MEDIA-SEQUENCE:0\n")
	builder.WriteString("#EXT-X-PLAYLIST-TYPE:EVENT\n")
	for _, segment := range startup.Segments {
		builder.WriteString(segment.Duration)
		builder.WriteByte('\n')
		builder.WriteString(startupBridgeSegmentURL(mediaID, profileID, filepath.Base(segment.URI)))
		builder.WriteByte('\n')
	}
	if continuation != nil && len(continuation.Segments) > 0 {
		builder.WriteString("#EXT-X-DISCONTINUITY\n")
		for _, segment := range continuation.Segments {
			builder.WriteString(segment.Duration)
			builder.WriteByte('\n')
			builder.WriteString(startupContinuationSegmentURL(mediaID, profileID, filepath.Base(segment.URI)))
			builder.WriteByte('\n')
		}
		if continuation.EndList {
			builder.WriteString("#EXT-X-ENDLIST\n")
		}
	}
	return builder.String()
}

func startupVirtualProfile(profileID string) string {
	return startupBridgeProfilePrefix + profileID
}

func parseStartupVirtualProfile(quality string) (string, bool) {
	if !strings.HasPrefix(quality, startupBridgeProfilePrefix) {
		return "", false
	}
	profileID := strings.TrimPrefix(quality, startupBridgeProfilePrefix)
	if profileID == "" || profileID != filepath.Base(profileID) {
		return "", false
	}
	return profileID, true
}

func startupBridgePlaylistURL(mediaID, profileID string) string {
	return fmt.Sprintf("/api/stream/%s/%s/stream.m3u8", mediaID, startupVirtualProfile(profileID))
}

func startupBridgeSegmentURL(mediaID, profileID, segment string) string {
	return fmt.Sprintf(
		"/api/stream/%s/%s/%s%s",
		mediaID,
		startupVirtualProfile(profileID),
		startupBridgeSegmentPrefix,
		segment,
	)
}

func startupContinuationSegmentURL(mediaID, profileID, segment string) string {
	return fmt.Sprintf(
		"/api/stream/%s/%s/%s%s",
		mediaID,
		startupVirtualProfile(profileID),
		startupContinuationSegmentPrefix,
		segment,
	)
}

func parseStartupBridgeSegment(segment string) (source string, actual string, ok bool) {
	switch {
	case strings.HasPrefix(segment, startupBridgeSegmentPrefix):
		actual = strings.TrimPrefix(segment, startupBridgeSegmentPrefix)
		return "startup", actual, actual != ""
	case strings.HasPrefix(segment, startupContinuationSegmentPrefix):
		actual = strings.TrimPrefix(segment, startupContinuationSegmentPrefix)
		return "continuation", actual, actual != ""
	default:
		return "", "", false
	}
}

func safeHLSSegmentPath(directory, segment string) (string, error) {
	if directory == "" || segment == "" || segment != filepath.Base(segment) {
		return "", gorm.ErrRecordNotFound
	}
	lower := strings.ToLower(segment)
	if !strings.HasPrefix(lower, "seg") || !strings.HasSuffix(lower, ".ts") {
		return "", gorm.ErrRecordNotFound
	}
	path := filepath.Join(directory, segment)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || !info.Mode().IsRegular() {
		return "", gorm.ErrRecordNotFound
	}
	return path, nil
}