package service

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
	"gorm.io/gorm"
)

const (
	startupContinuationPlannerVersion = "startup-continuation-hls-v1"
	startupContinuationArtifactKind   = "startup_continuation_hls"
	startupContinuationPriority       = 95
)

// StartupStreamDescriptor is the immutable, source-matched startup artifact
// that may be exposed to playback planning. Filesystem paths are intentionally
// kept inside the service boundary; API clients receive authenticated URLs.
type StartupStreamDescriptor struct {
	MediaID          string
	ProfileID        string
	SourceFingerprint string
	DurationMS       int64
	ArtifactID       string
	ManifestPath     string
	OutputDir        string
	Probe            *model.MediaProbeRecord
}

func (s *TranscodeService) ResolvePublishedStartupStream(media *model.Media) (*StartupStreamDescriptor, error) {
	if s == nil || s.executionRepo == nil || media == nil {
		return nil, gorm.ErrRecordNotFound
	}
	probe := s.GetCachedMediaProbe(media)
	if probe == nil || probe.SourceFingerprint == "" || !StartupStreamEligible(media, probe) {
		return nil, gorm.ErrRecordNotFound
	}
	profileID := startupStreamProfile(probe)
	artifact, err := s.executionRepo.FindPublishedArtifact(
		media.ID,
		profileID,
		probe.SourceFingerprint,
		startupStreamPlannerVersion,
		startupStreamArtifactKind,
	)
	if err != nil {
		return nil, err
	}
	manifestPath := artifact.ManifestPath
	if manifestPath == "" && artifact.Path != "" {
		manifestPath = filepath.Join(artifact.Path, "stream.m3u8")
	}
	if manifestPath == "" {
		return nil, gorm.ErrRecordNotFound
	}
	if _, err := os.Stat(manifestPath); err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	durationMS := artifact.DurationMS
	if durationMS <= 0 {
		durationMS = startupStreamDurationMS
	}
	return &StartupStreamDescriptor{
		MediaID:           media.ID,
		ProfileID:         profileID,
		SourceFingerprint: probe.SourceFingerprint,
		DurationMS:        durationMS,
		ArtifactID:        artifact.ID,
		ManifestPath:      manifestPath,
		OutputDir:         artifact.Path,
		Probe:             probe,
	}, nil
}

func startupContinuationActiveKey(mediaID, profileID, fingerprint string, startMS int64) string {
	return stableHash(strings.Join([]string{
		mediaID,
		string(transcodedomain.IntentStartupContinuationHLS),
		profileID,
		fmt.Sprintf("%d", startMS),
		fingerprint,
		startupContinuationPlannerVersion,
	}, "|"))
}

// SubmitStartupContinuation creates an interactive, durable continuation from
// the exact end of the immutable startup artifact. It never writes into the
// startup artifact directory and uses a distinct Intent and Artifact Kind.
func (s *TranscodeService) SubmitStartupContinuation(
	media *model.Media,
	startup *StartupStreamDescriptor,
) (*model.TranscodeTask, error) {
	if s == nil || s.executionRepo == nil || s.jobs == nil || media == nil || startup == nil {
		return nil, fmt.Errorf("startup continuation inputs are incomplete")
	}
	if startup.MediaID != media.ID || startup.ProfileID == "" || startup.SourceFingerprint == "" {
		return nil, fmt.Errorf("startup descriptor does not match media")
	}
	startMS := startup.DurationMS
	if startMS <= 0 {
		startMS = startupStreamDurationMS
	}
	if startup.Probe != nil && startup.Probe.DurationMS > 0 && startup.Probe.DurationMS <= startMS {
		return nil, gorm.ErrRecordNotFound
	}
	activeKey := startupContinuationActiveKey(media.ID, startup.ProfileID, startup.SourceFingerprint, startMS)

	if artifact, err := s.executionRepo.FindPublishedArtifact(
		media.ID,
		startup.ProfileID,
		startup.SourceFingerprint,
		startupContinuationPlannerVersion,
		startupContinuationArtifactKind,
	); err == nil && artifact != nil {
		if artifact.ManifestPath != "" {
			if _, statErr := os.Stat(artifact.ManifestPath); statErr == nil {
				return completedCompatibilityTask(media, startup.ProfileID, artifact, startupContinuationPriority), nil
			}
		}
	}
	if active, err := s.executionRepo.FindActiveByKey(activeKey); err == nil && active.LegacyTaskID != nil {
		if task, taskErr := s.repo.FindByID(*active.LegacyTaskID); taskErr == nil {
			return task, nil
		}
	}
	if !s.jobs.CanAccept() {
		return nil, fmt.Errorf("transcode queue is full or shutting down")
	}

	s.submitMu.Lock()
	defer s.submitMu.Unlock()
	if active, err := s.executionRepo.FindActiveByKey(activeKey); err == nil && active.LegacyTaskID != nil {
		if task, taskErr := s.repo.FindByID(*active.LegacyTaskID); taskErr == nil {
			return task, nil
		}
	}

	remainingMS := int64(0)
	if startup.Probe != nil && startup.Probe.DurationMS > startMS {
		remainingMS = startup.Probe.DurationMS - startMS
	}
	task := &model.TranscodeTask{
		MediaID:    media.ID,
		Status:     "pending",
		Quality:    startup.ProfileID,
		OutputDir:  s.GetLegacyOutputDir(media.ID, startup.ProfileID),
		MediaTitle: media.DescriptiveTitle(),
		Priority:   startupContinuationPriority,
		MaxRetry:   2,
	}
	if err := s.repo.Create(task); err != nil {
		return nil, err
	}
	legacyID := task.ID
	now := time.Now()
	record := &model.TranscodeJobRecord{
		LegacyTaskID:      &legacyID,
		MediaID:           media.ID,
		Intent:            string(transcodedomain.IntentStartupContinuationHLS),
		ProfileID:         startup.ProfileID,
		AudioTrack:        -1,
		StartMS:           startMS,
		DurationMS:        remainingMS,
		Priority:          startupContinuationPriority,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: startup.SourceFingerprint,
		PlanHash: stableHash(strings.Join([]string{
			startupContinuationPlannerVersion,
			startup.ProfileID,
			fmt.Sprintf("%d", startMS),
			normalizeAttemptBackend(s.hwAccel),
		}, "|")),
		PlannerVersion: startupContinuationPlannerVersion,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	if err := s.executionRepo.CreateJob(record); err != nil {
		_ = s.repo.DeleteByID(task.ID)
		return nil, fmt.Errorf("create startup continuation job: %w", err)
	}
	if !s.jobs.Push(&TranscodeJob{Task: task, ExecutionJob: record}) {
		completedAt := time.Now()
		_, _ = s.executionRepo.CompleteQueuedJob(record.ID, "failed", completedAt)
		task.Status = "failed"
		task.Error = "transcode queue is full or shutting down"
		task.CompletedAt = &completedAt
		_ = s.repo.Update(task)
		return nil, fmt.Errorf("transcode queue is full or shutting down")
	}
	if s.logger != nil {
		s.logger.Infof(
			"已提交 Startup Continuation job=%s media=%s profile=%s start=%dms",
			record.ID,
			media.ID,
			startup.ProfileID,
			startMS,
		)
	}
	return task, nil
}

func completedCompatibilityTask(
	media *model.Media,
	profileID string,
	artifact *model.TranscodeArtifactRecord,
	priority int,
) *model.TranscodeTask {
	return &model.TranscodeTask{
		MediaID:    media.ID,
		Status:     "done",
		Quality:    profileID,
		Progress:   100,
		OutputDir:  artifact.Path,
		MediaTitle: media.DescriptiveTitle(),
		Priority:   priority,
	}
}

func (s *TranscodeService) ResolveReadableStartupContinuation(
	media *model.Media,
	startup *StartupStreamDescriptor,
) (*model.TranscodeArtifactRecord, error) {
	if s == nil || s.executionRepo == nil || media == nil || startup == nil {
		return nil, gorm.ErrRecordNotFound
	}
	return s.executionRepo.FindReadableArtifactByKind(
		media.ID,
		startup.ProfileID,
		startup.SourceFingerprint,
		startupContinuationPlannerVersion,
		startupContinuationArtifactKind,
		time.Now(),
	)
}
