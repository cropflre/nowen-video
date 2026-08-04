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
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
	"gorm.io/gorm"
)

var ErrPersistentRuntimeTranscodeRetired = errors.New("persistent runtime transcoding has been retired; create a playback session")

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
	JobsFound       int
	JobsCancelled   int
	JobsDeferred    int
	ArtifactsDeleted int
	AttemptsRetired int
	TasksRetired    int
	PathsRemoved    int
}

func (r runtimePlaybackRetirementReport) Empty() bool {
	return r.JobsFound == 0 && r.ArtifactsDeleted == 0 && r.AttemptsRetired == 0 && r.TasksRetired == 0 && r.PathsRemoved == 0
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
func (s *TranscodeService) retirePersistentRuntimePlayback(now time.Time) (runtimePlaybackRetirementReport, error) {
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
	legacyTaskIDs := make([]string, 0, len(jobs))
	for index := range jobs {
		job := &jobs[index]
		allJobIDs = append(allJobIDs, job.ID)
		if job.LegacyTaskID != nil && strings.TrimSpace(*job.LegacyTaskID) != "" {
			legacyTaskIDs = append(legacyTaskIDs, strings.TrimSpace(*job.LegacyTaskID))
		}
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
			if err := tx.Model(&model.TranscodeJobRecord{}).
				Where("id IN ?", allJobIDs).
				Updates(map[string]any{
					"desired_state":       "cancelled",
					"active_key":          nil,
					"cancel_requested_at": now,
					"updated_at":          now,
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
				if err := tx.Model(&model.TranscodeJobRecord{}).
					Where("id IN ?", cancelJobIDs).
					Updates(map[string]any{
						"status":            "cancelled",
						"worker_id":         "",
						"lease_token":       "",
						"claimed_at":        nil,
						"last_heartbeat_at": nil,
						"lease_expires_at":  nil,
						"completed_at":      now,
						"updated_at":        now,
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
	artifactQuery := db.Where("kind IN ?", retiredRuntimeArtifactKinds)
	if err := artifactQuery.Find(&artifacts).Error; err != nil {
		return report, fmt.Errorf("list retired runtime playback artifacts: %w", err)
	}
	cleanupArtifacts := make([]model.TranscodeArtifactRecord, 0, len(artifacts))
	for index := range artifacts {
		artifact := artifacts[index]
		if _, live := liveJobIDs[artifact.JobID]; live {
			continue
		}
		cleanupArtifacts = append(cleanupArtifacts, artifact)
	}

	var tasks []model.TranscodeTask
	if len(legacyTaskIDs) > 0 {
		if err := db.Where("id IN ?", legacyTaskIDs).Find(&tasks).Error; err != nil {
			return report, fmt.Errorf("list retired runtime playback task projections: %w", err)
		}
	}

	root := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
	paths := make(map[string]struct{})
	addPath := func(path string) {
		path = strings.TrimSpace(path)
		if path != "" && runtimeRetirementPathAllowed(root, path) {
			paths[filepath.Clean(path)] = struct{}{}
		}
	}
	for index := range attempts {
		addPath(attempts[index].WorkspacePath)
	}
	for index := range cleanupArtifacts {
		addPath(cleanupArtifacts[index].TempPath)
		addPath(cleanupArtifacts[index].Path)
		if cleanupArtifacts[index].ManifestPath != "" {
			addPath(filepath.Dir(cleanupArtifacts[index].ManifestPath))
		}
	}
	for index := range tasks {
		addPath(tasks[index].OutputDir)
	}
	addPath(filepath.Join(root, "ondemand"))
	collectLegacyRuntimeDirectories(root, report.JobsDeferred == 0, addPath)

	orderedPaths := make([]string, 0, len(paths))
	for path := range paths {
		orderedPaths = append(orderedPaths, path)
	}
	sort.Slice(orderedPaths, func(i, j int) bool {
		return len(orderedPaths[i]) > len(orderedPaths[j])
	})
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
	if len(cleanupErrors) > 0 {
		return report, fmt.Errorf("retire runtime playback storage: %s", strings.Join(cleanupErrors, "; "))
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		if len(cleanupArtifacts) > 0 {
			artifactIDs := make([]string, 0, len(cleanupArtifacts))
			for index := range cleanupArtifacts {
				artifactIDs = append(artifactIDs, cleanupArtifacts[index].ID)
			}
			if err := tx.Where("id IN ?", artifactIDs).Delete(&model.TranscodeArtifactRecord{}).Error; err != nil {
				return err
			}
			report.ArtifactsDeleted = len(artifactIDs)
		}
		if len(attempts) > 0 {
			attemptIDs := make([]string, 0, len(attempts))
			for index := range attempts {
				attemptIDs = append(attemptIDs, attempts[index].ID)
			}
			if err := tx.Model(&model.TranscodeAttemptRecord{}).
				Where("id IN ?", attemptIDs).
				Updates(map[string]any{
					"workspace_path": "",
					"updated_at":     now,
				}).Error; err != nil {
				return err
			}
			if err := tx.Model(&model.TranscodeAttemptRecord{}).
				Where("id IN ? AND status NOT IN ?", attemptIDs, []string{"completed", "failed", "cancelled"}).
				Updates(map[string]any{
					"status":        "cancelled",
					"completed_at":  now,
					"error_code":    "runtime_playback_retired",
					"error_message": ErrPersistentRuntimeTranscodeRetired.Error(),
					"updated_at":    now,
				}).Error; err != nil {
				return err
			}
			report.AttemptsRetired = len(attemptIDs)
		}
		if len(tasks) > 0 {
			taskIDs := make([]string, 0, len(tasks))
			for index := range tasks {
				taskIDs = append(taskIDs, tasks[index].ID)
			}
			if err := tx.Model(&model.TranscodeTask{}).
				Where("id IN ?", taskIDs).
				Updates(map[string]any{
					"status":       "cancelled",
					"output_dir":   "",
					"error":        ErrPersistentRuntimeTranscodeRetired.Error(),
					"completed_at": now,
					"updated_at":   now,
				}).Error; err != nil {
				return err
			}
			report.TasksRetired = len(taskIDs)
		}
		return nil
	}); err != nil {
		return report, fmt.Errorf("finalize runtime playback retirement: %w", err)
	}

	s.InvalidateCacheDiskUsage()
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

func collectLegacyRuntimeDirectories(root string, includeQualityDirs bool, add func(string)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		switch entry.Name() {
		case "artifacts", "workspaces", "ondemand":
			continue
		}
		mediaRoot := filepath.Join(root, entry.Name())
		add(filepath.Join(mediaRoot, "audio"))
		if !includeQualityDirs {
			continue
		}
		for quality := range qualityPresets {
			add(filepath.Join(mediaRoot, quality))
		}
	}
}
