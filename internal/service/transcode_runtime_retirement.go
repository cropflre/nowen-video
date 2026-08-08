package service

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
	"gorm.io/gorm"
)

var ErrPersistentRuntimeTranscodeRetired = errors.New("persistent runtime transcoding has been retired; create a playback session")

const (
	retiredRuntimePlaybackIntent    = "retired_runtime_playback"
	startupStreamArtifactKind       = "startup_hls"
	startupContinuationArtifactKind = "startup_continuation_hls"
)

var retiredRuntimePlaybackIntents = []string{
	string(transcodedomain.IntentRuntimeHLS),
	string(transcodedomain.IntentStartupHLS),
	string(transcodedomain.IntentStartupContinuationHLS),
	string(transcodedomain.IntentVideoSegment),
	string(transcodedomain.IntentAudioSegment),
}

var retiredRuntimeArtifactKinds = []string{
	"hls_variant",
	startupStreamArtifactKind,
	startupContinuationArtifactKind,
}

type runtimePlaybackRetirementReport struct {
	JobsFound        int
	JobsCancelled    int
	JobsDeferred     int
	ArtifactsDeleted int
	AttemptsRetired  int
	PathsRemoved     int
}

func (r runtimePlaybackRetirementReport) Changed() bool {
	return r.JobsCancelled > 0 || r.ArtifactsDeleted > 0 || r.AttemptsRetired > 0 || r.PathsRemoved > 0
}

func isRetiredRuntimePlaybackIntent(intent string) bool {
	for _, retired := range retiredRuntimePlaybackIntents {
		if intent == retired {
			return true
		}
	}
	return false
}

func runtimePlaybackJobTerminal(status string) bool {
	switch status {
	case "completed", "failed", "cancelled":
		return true
	default:
		return false
	}
}

func runtimePlaybackJobHasLiveLease(job *model.TranscodeJobRecord, now time.Time) bool {
	if job == nil || runtimePlaybackJobTerminal(job.Status) {
		return false
	}
	return strings.TrimSpace(job.LeaseToken) != "" && job.LeaseExpiresAt != nil && job.LeaseExpiresAt.After(now)
}

// retirePersistentRuntimePlayback is an idempotent migration sweep. It fences
// all old runtime/startup jobs first, then removes only storage that is no
// longer protected by a live Lease. A second server still running old code will
// fail its next renewal because desired_state is cancelled; the periodic sweep
// removes its workspace only after the Lease expires.
func (s *ArtifactMaintenanceService) retirePersistentRuntimePlayback(now time.Time) (runtimePlaybackRetirementReport, error) {
	report := runtimePlaybackRetirementReport{}
	if s == nil || s.repo == nil || s.repo.DB() == nil || s.cfg == nil {
		return report, nil
	}
	db := s.repo.DB()
	var jobs []model.TranscodeJobRecord
	if err := db.Where("intent IN ?", retiredRuntimePlaybackIntents).Find(&jobs).Error; err != nil {
		return report, fmt.Errorf("list retired runtime playback jobs: %w", err)
	}
	report.JobsFound = len(jobs)
	allJobIDs := make([]string, 0, len(jobs))
	cleanupJobIDs := make([]string, 0, len(jobs))
	cancelJobIDs := make([]string, 0, len(jobs))
	liveJobIDs := make(map[string]struct{})
	for index := range jobs {
		job := &jobs[index]
		allJobIDs = append(allJobIDs, job.ID)
		if runtimePlaybackJobHasLiveLease(job, now) {
			liveJobIDs[job.ID] = struct{}{}
			report.JobsDeferred++
			continue
		}
		cleanupJobIDs = append(cleanupJobIDs, job.ID)
		if !runtimePlaybackJobTerminal(job.Status) {
			cancelJobIDs = append(cancelJobIDs, job.ID)
		}
	}
	if len(allJobIDs) > 0 {
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := tx.Model(&model.TranscodeJobRecord{}).Where("id IN ?", allJobIDs).Updates(map[string]any{
				"desired_state": "cancelled", "active_key": nil,
				"cancel_requested_at": now, "updated_at": now,
			}).Error; err != nil {
				return err
			}
			if len(liveJobIDs) > 0 {
				ids := make([]string, 0, len(liveJobIDs))
				for id := range liveJobIDs {
					ids = append(ids, id)
				}
				if err := tx.Model(&model.TranscodeJobRecord{}).
					Where("id IN ? AND status NOT IN ?", ids, []string{"completed", "failed", "cancelled"}).
					Updates(map[string]any{"status": "cancel_requested", "updated_at": now}).Error; err != nil {
					return err
				}
			}
			if len(cancelJobIDs) > 0 {
				if err := tx.Model(&model.TranscodeJobRecord{}).Where("id IN ?", cancelJobIDs).Updates(map[string]any{
					"status": "cancelled", "worker_id": "", "lease_token": "",
					"claimed_at": nil, "last_heartbeat_at": nil, "lease_expires_at": nil,
					"completed_at": now, "updated_at": now,
				}).Error; err != nil {
					return err
				}
			}
			return nil
		})
		if err != nil {
			return report, fmt.Errorf("fence retired runtime playback jobs: %w", err)
		}
		report.JobsCancelled = len(cancelJobIDs)
	}

	var attempts []model.TranscodeAttemptRecord
	if len(cleanupJobIDs) > 0 {
		if err := db.Where("job_id IN ?", cleanupJobIDs).Find(&attempts).Error; err != nil {
			return report, fmt.Errorf("list retired runtime playback attempts: %w", err)
		}
	}
	var artifacts []model.TranscodeArtifactRecord
	if err := db.Where("kind IN ?", retiredRuntimeArtifactKinds).Find(&artifacts).Error; err != nil {
		return report, fmt.Errorf("list retired runtime playback artifacts: %w", err)
	}
	cleanupArtifacts := make([]model.TranscodeArtifactRecord, 0, len(artifacts))
	for index := range artifacts {
		if _, live := liveJobIDs[artifacts[index].JobID]; !live && artifacts[index].CleanupState != repository.ArtifactCleanupCompleted {
			cleanupArtifacts = append(cleanupArtifacts, artifacts[index])
		}
	}
	if len(cleanupArtifacts) > 0 {
		ids := make([]string, 0, len(cleanupArtifacts))
		for index := range cleanupArtifacts {
			ids = append(ids, cleanupArtifacts[index].ID)
		}
		if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id IN ?", ids).Updates(map[string]any{
			"status":     gorm.Expr("CASE WHEN status = ? THEN ? ELSE ? END", "published", "expired", "cancelled"),
			"updated_at": now,
		}).Error; err != nil {
			return report, fmt.Errorf("terminalize retired artifacts: %w", err)
		}
	}

	root := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
	paths := make(map[string]struct{})
	for index := range attempts {
		path := strings.TrimSpace(attempts[index].WorkspacePath)
		if path != "" && runtimeRetirementPathAllowed(root, path) {
			paths[filepath.Clean(path)] = struct{}{}
		}
	}
	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Slice(orderedPaths, func(i, j int) bool { return len(orderedPaths[i]) > len(orderedPaths[j]) })
	cleanupErrors := make([]string, 0)
	for _, path := range orderedPaths {
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("inspect %s: %v", path, err))
			continue
		}
		if err := os.RemoveAll(path); err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("remove %s: %v", path, err))
			continue
		}
		report.PathsRemoved++
	}
	for index := range cleanupArtifacts {
		removed, err := s.cleanupArtifactRecord(&cleanupArtifacts[index])
		report.PathsRemoved += removed
		if err != nil {
			cleanupErrors = append(cleanupErrors, fmt.Sprintf("artifact %s: %v", cleanupArtifacts[index].ID, err))
			continue
		}
		report.ArtifactsDeleted++
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(attempts) > 0 {
			attemptIDs := make([]string, 0, len(attempts))
			for index := range attempts {
				attemptIDs = append(attemptIDs, attempts[index].ID)
			}
			if err := tx.Model(&model.TranscodeAttemptRecord{}).Where("id IN ?", attemptIDs).
				Updates(map[string]any{"workspace_path": "", "updated_at": now}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.TranscodeAttemptRecord{}).
				Where("id IN ? AND status NOT IN ?", attemptIDs, []string{"completed", "failed", "cancelled"}).
				Updates(map[string]any{"status": "cancelled", "completed_at": now,
					"error_code": "runtime_playback_retired", "error_message": ErrPersistentRuntimeTranscodeRetired.Error(), "updated_at": now}).Error; err != nil {
				return err
			}
			report.AttemptsRetired = len(attemptIDs)
		}
		if len(cleanupJobIDs) > 0 {
			if err := tx.Model(&model.TranscodeJobRecord{}).Where("id IN ?", cleanupJobIDs).
				Updates(map[string]any{"intent": retiredRuntimePlaybackIntent, "current_attempt_id": "", "updated_at": now}).Error; err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return report, fmt.Errorf("finalize runtime playback retirement: %w", err)
	}
	s.InvalidateCacheDiskUsage()
	if len(cleanupErrors) > 0 {
		return report, fmt.Errorf("retire runtime playback storage: %s", strings.Join(cleanupErrors, "; "))
	}
	return report, nil
}

func runtimeRetirementPathAllowed(root, candidate string) bool {
	rootAbs, err := filepath.Abs(filepath.Clean(root))
	if err != nil {
		return false
	}
	candidateAbs, err := filepath.Abs(filepath.Clean(candidate))
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(rootAbs, candidateAbs)
	if err != nil || relative == "." || relative == ".." || filepath.IsAbs(relative) || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	for _, protected := range []string{"artifacts", "workspaces"} {
		if relative == protected {
			return false
		}
	}
	return true
}
