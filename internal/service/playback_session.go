package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	playbacksession "github.com/nowen-video/nowen-video/internal/playback/session"
	playbacktranscode "github.com/nowen-video/nowen-video/internal/playback/transcode"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	"go.uber.org/zap"
)

const (
	defaultPlaybackHeartbeatInterval = 15 * time.Second
	defaultPlaybackStartupWait       = 3 * time.Second
)

type PlaybackSessionCreateRequest struct {
	MediaID         string `json:"media_id" binding:"required"`
	ProfileID       string `json:"profile_id"`
	StartPositionMS int64  `json:"start_position_ms"`
	AudioTrack      int    `json:"audio_track"`
	SubtitleTrack   int    `json:"subtitle_track"`
	BurnSubtitle    bool   `json:"burn_subtitle"`
	MaxBitrate      int    `json:"max_bitrate"`
}

type PlaybackSessionRestartRequest struct {
	ProfileID       string `json:"profile_id"`
	StartPositionMS int64  `json:"start_position_ms"`
	AudioTrack      int    `json:"audio_track"`
	SubtitleTrack   int    `json:"subtitle_track"`
	BurnSubtitle    bool   `json:"burn_subtitle"`
	MaxBitrate      int    `json:"max_bitrate"`
	Reason          string `json:"reason"`
}

type PlaybackSessionHeartbeatRequest struct {
	GenerationID  uint64 `json:"generation_id"`
	PositionMS    int64  `json:"position_ms"`
	BufferedEndMS int64  `json:"buffered_end_ms"`
	Paused        bool   `json:"paused"`
}

type PlaybackSessionResult struct {
	Session              playbacksession.SessionSnapshot `json:"session"`
	PlaylistURL          string                          `json:"playlist_url,omitempty"`
	StatusURL            string                          `json:"status_url"`
	HeartbeatIntervalSec int                             `json:"heartbeat_interval_sec"`
	FirstSegmentReady    bool                            `json:"first_segment_ready"`
	StartupMS            int64                           `json:"startup_ms,omitempty"`
}

type PlaybackSessionFile struct {
	Path        string
	ContentType string
	Lease       *playbacksession.ReaderLease
}

func (f *PlaybackSessionFile) Release() {
	if f != nil && f.Lease != nil {
		f.Lease.Release()
	}
}

// PlaybackSessionService is the application boundary for ephemeral runtime
// playback. It owns session orchestration but reuses the existing FFmpeg
// Runtime, Governor, hardware detection, remote-input adapters and profile
// catalog. It never creates a persistent transcode Job or Artifact.
type PlaybackSessionService struct {
	mediaRepo   *repository.MediaRepo
	cfg         *config.Config
	transcoder  *TranscodeService
	manager     *playbacksession.Manager
	runner      *playbacktranscode.Runner
	logger      *zap.SugaredLogger
	startupWait time.Duration
	heartbeat   time.Duration
}

func NewPlaybackSessionService(
	mediaRepo *repository.MediaRepo,
	transcoder *TranscodeService,
	cfg *config.Config,
	logger *zap.SugaredLogger,
) (*PlaybackSessionService, error) {
	if mediaRepo == nil {
		return nil, fmt.Errorf("media repository is required")
	}
	if transcoder == nil || transcoder.ExecutionRuntime() == nil {
		return nil, fmt.Errorf("transcode execution runtime is required")
	}
	if cfg == nil {
		return nil, fmt.Errorf("configuration is required")
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	manager, err := playbacksession.NewManager(
		playbacksession.DefaultConfig(cfg.Cache.CacheDir),
		logger,
	)
	if err != nil {
		return nil, err
	}
	runnerConfig := playbacktranscode.DefaultConfig(
		cfg.App.FFmpegPath,
		transcoder.GetHWAccelInfo(),
		cfg.App.VAAPIDevice,
		ffmpeg.CalcThreads(cfg),
	)
	runner, err := playbacktranscode.NewRunner(
		manager,
		transcoder.ExecutionRuntime(),
		runnerConfig,
		logger,
	)
	if err != nil {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = manager.Shutdown(shutdownCtx)
		return nil, err
	}
	return &PlaybackSessionService{
		mediaRepo:   mediaRepo,
		cfg:         cfg,
		transcoder:  transcoder,
		manager:     manager,
		runner:      runner,
		logger:      logger,
		startupWait: defaultPlaybackStartupWait,
		heartbeat:   defaultPlaybackHeartbeatInterval,
	}, nil
}

func (s *PlaybackSessionService) Create(
	ctx context.Context,
	userID string,
	request PlaybackSessionCreateRequest,
) (PlaybackSessionResult, error) {
	if strings.TrimSpace(userID) == "" {
		return PlaybackSessionResult{}, fmt.Errorf("user id is required")
	}
	media, err := s.loadTranscodeMedia(request.MediaID)
	if err != nil {
		return PlaybackSessionResult{}, err
	}
	profileID := request.ProfileID
	if profileID == "" || profileID == "auto" {
		profileID = s.defaultProfile(media)
	}

	created, err := s.manager.Create(ctx, playbacksession.CreateRequest{
		UserID:          userID,
		MediaID:         media.ID,
		ProfileID:       profileID,
		StartPositionMS: request.StartPositionMS,
		AudioTrack:      request.AudioTrack,
		SubtitleTrack:   request.SubtitleTrack,
		BurnSubtitle:    request.BurnSubtitle,
		MaxBitrate:      request.MaxBitrate,
	})
	if err != nil {
		return PlaybackSessionResult{}, err
	}

	execution, err := s.startGeneration(ctx, media, created.ID, created.PendingGenerationID, profileID, request.StartPositionMS)
	if err != nil {
		s.closeAfterStartFailure(created.ID)
		return PlaybackSessionResult{}, err
	}
	return s.waitForStartup(ctx, created.ID, execution)
}

func (s *PlaybackSessionService) Restart(
	ctx context.Context,
	userID,
	sessionID string,
	request PlaybackSessionRestartRequest,
) (PlaybackSessionResult, error) {
	snapshot, err := s.ownedSnapshot(userID, sessionID)
	if err != nil {
		return PlaybackSessionResult{}, err
	}
	media, err := s.loadTranscodeMedia(snapshot.MediaID)
	if err != nil {
		return PlaybackSessionResult{}, err
	}
	profileID := request.ProfileID
	if profileID == "" || profileID == "auto" {
		if snapshot.Generation != nil && snapshot.Generation.ProfileID != "" {
			profileID = snapshot.Generation.ProfileID
		} else {
			profileID = s.defaultProfile(media)
		}
	}
	generation, err := s.manager.BeginGeneration(sessionID, playbacksession.BeginGenerationRequest{
		ProfileID:       profileID,
		StartPositionMS: request.StartPositionMS,
		AudioTrack:      request.AudioTrack,
		SubtitleTrack:   request.SubtitleTrack,
		BurnSubtitle:    request.BurnSubtitle,
		MaxBitrate:      request.MaxBitrate,
		Reason:          normalizedRestartReason(request.Reason),
	})
	if err != nil {
		return PlaybackSessionResult{}, err
	}
	execution, err := s.startGeneration(ctx, media, sessionID, generation.ID, profileID, request.StartPositionMS)
	if err != nil {
		_ = s.manager.MarkGenerationFailed(sessionID, generation.ID, "generation_start_failed", err.Error())
		return PlaybackSessionResult{}, err
	}
	return s.waitForStartup(ctx, sessionID, execution)
}

func (s *PlaybackSessionService) Heartbeat(
	userID,
	sessionID string,
	request PlaybackSessionHeartbeatRequest,
) (PlaybackSessionResult, error) {
	if _, err := s.ownedSnapshot(userID, sessionID); err != nil {
		return PlaybackSessionResult{}, err
	}
	snapshot, err := s.manager.Heartbeat(sessionID, playbacksession.Heartbeat{
		GenerationID:  request.GenerationID,
		PositionMS:    request.PositionMS,
		BufferedEndMS: request.BufferedEndMS,
		Paused:        request.Paused,
	})
	if err != nil {
		return PlaybackSessionResult{}, err
	}
	return s.result(snapshot, 0), nil
}

func (s *PlaybackSessionService) Status(userID, sessionID string) (PlaybackSessionResult, error) {
	snapshot, err := s.ownedSnapshot(userID, sessionID)
	if err != nil {
		return PlaybackSessionResult{}, err
	}
	return s.result(snapshot, 0), nil
}

func (s *PlaybackSessionService) Close(ctx context.Context, userID, sessionID, reason string) error {
	if _, err := s.ownedSnapshot(userID, sessionID); err != nil {
		if errors.Is(err, playbacksession.ErrSessionNotFound) {
			return nil
		}
		return err
	}
	return s.manager.Close(ctx, sessionID, normalizedCloseReason(reason))
}

func (s *PlaybackSessionService) OpenPlaylist(userID, sessionID string, generationID uint64) (*PlaybackSessionFile, error) {
	return s.openGenerationFile(userID, sessionID, generationID, "stream.m3u8", "application/vnd.apple.mpegurl")
}

func (s *PlaybackSessionService) OpenSegment(userID, sessionID string, generationID uint64, name string) (*PlaybackSessionFile, error) {
	contentType := "video/mp2t"
	switch strings.ToLower(filepath.Ext(name)) {
	case ".m4s":
		contentType = "video/iso.segment"
	case ".mp4":
		contentType = "video/mp4"
	case ".aac":
		contentType = "audio/aac"
	}
	return s.openGenerationFile(userID, sessionID, generationID, name, contentType)
}

func (s *PlaybackSessionService) Shutdown(ctx context.Context) error {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.Shutdown(ctx)
}

func (s *PlaybackSessionService) startGeneration(
	ctx context.Context,
	media *model.Media,
	sessionID string,
	generationID uint64,
	profileID string,
	startPositionMS int64,
) (*playbacktranscode.Execution, error) {
	inputPath := ResolveRemoteFFmpegURL(s.cfg, media.FilePath)
	if unresolvedRemoteInput(inputPath) {
		return nil, fmt.Errorf("remote media input is unavailable for transcoding")
	}
	fps := 25.0
	if probe := s.transcoder.GetCachedMediaProbe(media); probe != nil {
		if value := probe.FrameRate(); value > 0 {
			fps = value
		}
	}
	return s.runner.Start(ctx, playbacktranscode.StartRequest{
		SessionID:       sessionID,
		GenerationID:    generationID,
		InputPath:       inputPath,
		ExtraInput:      BuildFFmpegInputArgs(inputPath),
		ProfileID:       profileID,
		StartPositionMS: startPositionMS,
		FPS:             fps,
	})
}

func (s *PlaybackSessionService) waitForStartup(
	ctx context.Context,
	sessionID string,
	execution *playbacktranscode.Execution,
) (PlaybackSessionResult, error) {
	waitCtx, cancel := context.WithTimeout(context.Background(), s.startupWait)
	if ctx != nil {
		waitCtx, cancel = context.WithTimeout(ctx, s.startupWait)
	}
	defer cancel()
	ready, err := execution.WaitReady(waitCtx)
	if err == nil {
		return s.result(ready.Session, ready.StartupMS), nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		snapshot, snapshotErr := s.manager.GetSnapshot(sessionID)
		if snapshotErr != nil {
			return PlaybackSessionResult{}, snapshotErr
		}
		return s.result(snapshot, 0), nil
	}
	snapshot, snapshotErr := s.manager.GetSnapshot(sessionID)
	if snapshotErr == nil && snapshot.Generation != nil && snapshot.Generation.ErrorMessage != "" {
		return PlaybackSessionResult{}, errors.New(snapshot.Generation.ErrorMessage)
	}
	return PlaybackSessionResult{}, err
}

func (s *PlaybackSessionService) result(snapshot playbacksession.SessionSnapshot, startupMS int64) PlaybackSessionResult {
	result := PlaybackSessionResult{
		Session:              snapshot,
		StatusURL:            fmt.Sprintf("/api/playback/sessions/%s/status", snapshot.ID),
		HeartbeatIntervalSec: int(s.heartbeat / time.Second),
		StartupMS:            startupMS,
	}
	if snapshot.CurrentGenerationID != 0 && (snapshot.State == playbacksession.SessionStateReady || snapshot.State == playbacksession.SessionStateActive) {
		result.FirstSegmentReady = true
		result.PlaylistURL = generationPlaylistURL(snapshot.ID, snapshot.CurrentGenerationID)
	}
	return result
}

func (s *PlaybackSessionService) openGenerationFile(
	userID,
	sessionID string,
	generationID uint64,
	name,
	contentType string,
) (*PlaybackSessionFile, error) {
	if _, err := s.ownedSnapshot(userID, sessionID); err != nil {
		return nil, err
	}
	if name == "" || filepath.Base(name) != name || strings.ContainsAny(name, `/\\`) {
		return nil, fmt.Errorf("invalid playback file name")
	}
	lease, generation, err := s.manager.AcquireReader(sessionID, generationID)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(generation.OutputDir, name)
	if err := ensurePlaybackChildPath(generation.OutputDir, path); err != nil {
		lease.Release()
		return nil, err
	}
	info, err := filepathStat(path)
	if err != nil || !info.Mode().IsRegular() {
		lease.Release()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("playback file is not regular")
	}
	return &PlaybackSessionFile{Path: path, ContentType: contentType, Lease: lease}, nil
}

func (s *PlaybackSessionService) ownedSnapshot(userID, sessionID string) (playbacksession.SessionSnapshot, error) {
	snapshot, err := s.manager.GetSnapshot(sessionID)
	if err != nil {
		return playbacksession.SessionSnapshot{}, err
	}
	if snapshot.UserID != userID {
		return playbacksession.SessionSnapshot{}, ErrForbidden
	}
	return snapshot, nil
}

func (s *PlaybackSessionService) loadTranscodeMedia(mediaID string) (*model.Media, error) {
	media, err := s.mediaRepo.FindByID(mediaID)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	if media.StreamURL != "" {
		return nil, fmt.Errorf("STRM media must use direct proxy playback")
	}
	if strings.TrimSpace(media.FilePath) == "" {
		return nil, fmt.Errorf("media file path is empty")
	}
	return media, nil
}

func (s *PlaybackSessionService) defaultProfile(media *model.Media) string {
	if media == nil {
		return "720p"
	}
	height := parseResolutionHeight(media.Resolution)
	if height >= 1080 {
		return "1080p"
	}
	if height >= 720 {
		return "720p"
	}
	if height >= 480 {
		return "480p"
	}
	return "360p"
}

func (s *PlaybackSessionService) closeAfterStartFailure(sessionID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.manager.Close(ctx, sessionID, "generation_start_failed")
}

func generationPlaylistURL(sessionID string, generationID uint64) string {
	return fmt.Sprintf(
		"/api/playback/sessions/%s/generations/%d/stream.m3u8",
		url.PathEscape(sessionID),
		generationID,
	)
}

func normalizedRestartReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "restart"
	}
	return reason
}

func normalizedCloseReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "client_closed"
	}
	return reason
}

func unresolvedRemoteInput(input string) bool {
	for _, prefix := range []string{WebDAVScheme, AlistScheme, S3Scheme} {
		if strings.HasPrefix(input, prefix) {
			return true
		}
	}
	return false
}

func ensurePlaybackChildPath(root, path string) error {
	rel, err := filepath.Rel(filepath.Clean(root), filepath.Clean(path))
	if err != nil {
		return err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("playback path escapes generation directory")
	}
	return nil
}

var filepathStat = func(path string) (interface {
	Mode() interface{ IsRegular() bool }
}, error) {
	return nil, fmt.Errorf("uninitialized file stat adapter for %s", path)
}
