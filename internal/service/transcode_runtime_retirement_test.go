package service

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	transcodeartifactstore "github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
	"go.uber.org/zap"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newRuntimeRetirementTestService(t *testing.T) (*TranscodeService, *gorm.DB, string) {
	t.Helper()
	base := t.TempDir()
	db, err := gorm.Open(sqlite.Open(filepath.Join(base, "retirement.db")), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(
		&model.TranscodeTask{},
		&model.TranscodeJobRecord{},
		&model.TranscodeAttemptRecord{},
		&model.TranscodeArtifactRecord{},
	); err != nil {
		t.Fatal(err)
	}
	cacheRoot := filepath.Join(base, "cache")
	store, err := transcodeartifactstore.New(filepath.Join(cacheRoot, "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	repos := repository.NewRepositories(db)
	return &TranscodeService{
		repo:          repos.Transcode,
		executionRepo: repository.NewTranscodeExecutionRepo(db),
		cfg:           &config.Config{Cache: config.CacheConfig{CacheDir: cacheRoot}},
		artifactStore: store,
		logger:        zap.NewNop().Sugar(),
	}, db, cacheRoot
}

func writeRetirementFixture(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(path, "fixture.bin"), []byte("runtime"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRetirePersistentRuntimePlaybackRemovesOnlyRuntimeStorage(t *testing.T) {
	service, db, cacheRoot := newRuntimeRetirementTestService(t)
	now := time.Now().UTC()
	transcodeRoot := filepath.Join(cacheRoot, "transcode")
	workspace := filepath.Join(transcodeRoot, "workspaces", "job-runtime", "attempt-runtime", "hls")
	artifactDir := filepath.Join(transcodeRoot, "artifacts", "media-runtime", "720p", "artifact-runtime")
	legacyDir := filepath.Join(transcodeRoot, "media-runtime", "720p")
	audioDir := filepath.Join(transcodeRoot, "media-runtime", "audio", "0")
	onDemandDir := filepath.Join(transcodeRoot, "ondemand", "media-runtime", "720p")
	preprocessDir := filepath.Join(cacheRoot, "preprocess", "media-runtime")
	playbackDir := filepath.Join(cacheRoot, "playback-temp", "session-runtime")
	for _, path := range []string{workspace, artifactDir, legacyDir, audioDir, onDemandDir, preprocessDir, playbackDir} {
		writeRetirementFixture(t, path)
	}

	task := &model.TranscodeTask{
		ID:        "task-runtime",
		MediaID:   "media-runtime",
		Status:    "running",
		Quality:   "720p",
		OutputDir: legacyDir,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	activeKey := "runtime-active"
	job := &model.TranscodeJobRecord{
		ID:                "job-runtime",
		LegacyTaskID:      &task.ID,
		MediaID:           task.MediaID,
		Intent:            "runtime_hls",
		ProfileID:         "720p",
		Status:            "running",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		CurrentAttemptID:  "attempt-runtime",
		SourceFingerprint: "source",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	attempt := &model.TranscodeAttemptRecord{
		ID:            "attempt-runtime",
		JobID:         job.ID,
		Number:        1,
		Status:        "running",
		WorkspacePath: workspace,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	if err := db.Create(attempt).Error; err != nil {
		t.Fatal(err)
	}
	artifact := &model.TranscodeArtifactRecord{
		ID:           "artifact-runtime",
		JobID:        job.ID,
		AttemptID:    attempt.ID,
		MediaID:      task.MediaID,
		Kind:         "hls_variant",
		ProfileID:    "720p",
		Path:         artifactDir,
		ManifestPath: filepath.Join(artifactDir, "stream.m3u8"),
		Status:       "published",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}

	report, err := service.retirePersistentRuntimePlayback(now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if report.JobsCancelled != 1 || report.ArtifactsDeleted != 1 || report.TasksRetired != 1 || report.AttemptsRetired != 1 {
		t.Fatalf("unexpected retirement report: %+v", report)
	}
	for _, path := range []string{workspace, artifactDir, legacyDir, audioDir, onDemandDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("runtime path was not removed: %s err=%v", path, err)
		}
	}
	for _, path := range []string{preprocessDir, playbackDir} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("protected path was removed: %s err=%v", path, err)
		}
	}

	var retiredJob model.TranscodeJobRecord
	if err := db.First(&retiredJob, "id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retiredJob.Intent != retiredRuntimePlaybackIntent || retiredJob.Status != "cancelled" || retiredJob.ActiveKey != nil {
		t.Fatalf("runtime job was not fenced: %+v", retiredJob)
	}
	var retiredTask model.TranscodeTask
	if err := db.First(&retiredTask, "id = ?", task.ID).Error; err != nil {
		t.Fatal(err)
	}
	if retiredTask.Status != "cancelled" || retiredTask.OutputDir != "" {
		t.Fatalf("legacy task projection was not retired: %+v", retiredTask)
	}
	var artifactCount int64
	if err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifact.ID).Count(&artifactCount).Error; err != nil {
		t.Fatal(err)
	}
	if artifactCount != 0 {
		t.Fatalf("runtime artifact metadata remains: %d", artifactCount)
	}
}

func TestRetirePersistentRuntimePlaybackDefersLiveLease(t *testing.T) {
	service, db, cacheRoot := newRuntimeRetirementTestService(t)
	now := time.Now().UTC()
	workspace := filepath.Join(cacheRoot, "transcode", "workspaces", "job-live", "attempt-live", "hls")
	artifactDir := filepath.Join(cacheRoot, "transcode", "artifacts", "media-live", "720p", "artifact-live")
	writeRetirementFixture(t, workspace)
	writeRetirementFixture(t, artifactDir)

	task := &model.TranscodeTask{ID: "task-live", MediaID: "media-live", Status: "running", Quality: "720p", OutputDir: workspace, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(task).Error; err != nil {
		t.Fatal(err)
	}
	activeKey := "live-key"
	leaseExpiry := now.Add(time.Minute)
	job := &model.TranscodeJobRecord{
		ID:                "job-live",
		LegacyTaskID:      &task.ID,
		MediaID:           task.MediaID,
		Intent:            "startup_hls",
		ProfileID:         "720p",
		Status:            "running",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		WorkerID:          "old-instance",
		LeaseToken:        "live-token",
		LeaseExpiresAt:    &leaseExpiry,
		CurrentAttemptID:  "attempt-live",
		SourceFingerprint: "source",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if err := db.Create(job).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TranscodeAttemptRecord{ID: "attempt-live", JobID: job.ID, Number: 1, Status: "running", WorkspacePath: workspace, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&model.TranscodeArtifactRecord{ID: "artifact-live", JobID: job.ID, AttemptID: "attempt-live", MediaID: task.MediaID, Kind: startupStreamArtifactKind, ProfileID: "720p", Path: artifactDir, Status: "staging", CreatedAt: now, UpdatedAt: now}).Error; err != nil {
		t.Fatal(err)
	}

	first, err := service.retirePersistentRuntimePlayback(now)
	if err != nil {
		t.Fatal(err)
	}
	if first.JobsDeferred != 1 || first.PathsRemoved != 0 {
		t.Fatalf("live lease was not deferred: %+v", first)
	}
	for _, path := range []string{workspace, artifactDir} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("live lease path was removed: %s err=%v", path, err)
		}
	}
	var fenced model.TranscodeJobRecord
	if err := db.First(&fenced, "id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if fenced.Status != "cancel_requested" || fenced.DesiredState != "cancelled" || fenced.ActiveKey != nil {
		t.Fatalf("live job was not cancellation-fenced: %+v", fenced)
	}

	second, err := service.retirePersistentRuntimePlayback(leaseExpiry.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if second.JobsDeferred != 0 || second.ArtifactsDeleted != 1 {
		t.Fatalf("expired lease was not retired: %+v", second)
	}
	for _, path := range []string{workspace, artifactDir} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Fatalf("expired lease path remains: %s err=%v", path, err)
		}
	}
}

func TestRuntimeRetirementPathBoundary(t *testing.T) {
	root := filepath.Join(t.TempDir(), "cache", "transcode")
	if !runtimeRetirementPathAllowed(root, filepath.Join(root, "artifacts", "media", "artifact")) {
		t.Fatal("artifact child must be removable")
	}
	for _, path := range []string{
		root,
		filepath.Join(root, "artifacts"),
		filepath.Join(root, "workspaces"),
		filepath.Join(filepath.Dir(root), "playback-temp", "session"),
		filepath.Join(filepath.Dir(filepath.Dir(root)), "outside"),
	} {
		if runtimeRetirementPathAllowed(root, path) {
			t.Fatalf("unsafe retirement path accepted: %s", path)
		}
	}
}
