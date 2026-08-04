package service

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	transcodeartifactstore "github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type concurrentArtifactFixture struct {
	job       *TranscodeJob
	workspace string
}

type concurrentPublishResult struct {
	published bool
	err       error
}

func TestConcurrentJobsPublishIndependentArtifacts(t *testing.T) {
	service, db := newConcurrentArtifactService(t)
	first := createConcurrentArtifactFixture(t, service, db, "first")
	second := createConcurrentArtifactFixture(t, service, db, "second")

	results := publishConcurrentArtifacts(first.job, second.job, service)
	for index, result := range results {
		if result.err != nil || !result.published {
			t.Fatalf("publish %d failed: published=%v err=%v", index, result.published, result.err)
		}
	}

	assertConcurrentArtifactPublished(t, service, db, first)
	assertConcurrentArtifactPublished(t, service, db, second)
	if first.job.CurrentArtifact.Path == second.job.CurrentArtifact.Path {
		t.Fatalf("independent jobs published to the same path: %s", first.job.CurrentArtifact.Path)
	}

	counts, err := service.executionRepo.ArtifactStatusCounts()
	if err != nil {
		t.Fatal(err)
	}
	if counts["published"] != 2 {
		t.Fatalf("expected two published artifacts, got %+v", counts)
	}
}

func TestConcurrentPublishIsolatesStaleLease(t *testing.T) {
	service, db := newConcurrentArtifactService(t)
	owner := createConcurrentArtifactFixture(t, service, db, "owner")
	stale := createConcurrentArtifactFixture(t, service, db, "stale")

	if requeued, err := service.executionRepo.RequeueLeasedJob(
		stale.job.ExecutionJob.ID,
		stale.job.leaseToken,
		time.Now(),
	); err != nil || !requeued {
		t.Fatalf("requeue stale job: requeued=%v err=%v", requeued, err)
	}

	results := publishConcurrentArtifacts(owner.job, stale.job, service)
	if results[0].err != nil || !results[0].published {
		t.Fatalf("current owner failed to publish: published=%v err=%v", results[0].published, results[0].err)
	}
	if results[1].err != nil || results[1].published {
		t.Fatalf("stale Lease publish was not fenced: published=%v err=%v", results[1].published, results[1].err)
	}

	assertConcurrentArtifactPublished(t, service, db, owner)

	var staleArtifact model.TranscodeArtifactRecord
	if err := db.First(&staleArtifact, "id = ?", stale.job.CurrentArtifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if staleArtifact.Status != "abandoned" || staleArtifact.ErrorCode != "lease_lost" {
		t.Fatalf("stale artifact was not quarantined: %+v", staleArtifact)
	}
	if _, err := os.Stat(stale.workspace); err != nil {
		t.Fatalf("stale workspace should remain available for cleanup: %v", err)
	}
	if _, err := service.ResolveHLSOutputDir(stale.job.Media, stale.job.Quality); !errors.Is(err, ErrPersistentRuntimeTranscodeRetired) {
		t.Fatalf("runtime Artifact resolver did not remain retired: %v", err)
	}

	storedStaleJob, err := service.executionRepo.FindJobByID(stale.job.ExecutionJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedStaleJob.Status != "queued" || storedStaleJob.LeaseToken != "" || storedStaleJob.ActiveKey == nil {
		t.Fatalf("stale job did not remain recoverable: %+v", storedStaleJob)
	}
}

func newConcurrentArtifactService(t *testing.T) (*TranscodeService, *gorm.DB) {
	t.Helper()
	cacheDir := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "concurrent-artifacts.db")
	dsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	sqlDB.SetMaxOpenConns(8)
	sqlDB.SetMaxIdleConns(8)

	if err := db.AutoMigrate(&model.Media{}, &model.TranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	artifactStore, err := transcodeartifactstore.New(filepath.Join(cacheDir, "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	repos := repository.NewRepositories(db)
	return &TranscodeService{
		repo:          repos.Transcode,
		executionRepo: repository.NewTranscodeExecutionRepo(db),
		artifactStore: artifactStore,
		cfg: &config.Config{
			Cache: config.CacheConfig{CacheDir: cacheDir},
		},
		logger: zap.NewNop().Sugar(),
	}, db
}

func createConcurrentArtifactFixture(t *testing.T, service *TranscodeService, db *gorm.DB, suffix string) concurrentArtifactFixture {
	t.Helper()
	mediaPath := filepath.Join(t.TempDir(), suffix+".mkv")
	if err := os.WriteFile(mediaPath, []byte("source-"+suffix), 0o644); err != nil {
		t.Fatal(err)
	}
	media := &model.Media{
		ID:         "media-" + suffix,
		Title:      "Concurrent " + suffix,
		FilePath:   mediaPath,
		Duration:   4,
		Resolution: "720p",
		VideoCodec: "h264",
		AudioCodec: "aac",
		MediaType:  "movie",
	}
	if err := db.Create(media).Error; err != nil {
		t.Fatal(err)
	}

	task := &model.TranscodeTask{
		ID:         "task-" + suffix,
		MediaID:    media.ID,
		Quality:    "720p",
		Status:     "running",
		MediaTitle: media.Title,
		Progress:   100,
	}
	if err := service.repo.Create(task); err != nil {
		t.Fatal(err)
	}

	activeKey := "concurrent-artifact-" + suffix
	legacyTaskID := task.ID
	executionJob := &model.TranscodeJobRecord{
		ID:                "job-" + suffix,
		LegacyTaskID:      &legacyTaskID,
		MediaID:           media.ID,
		Intent:            "runtime_hls",
		ProfileID:         task.Quality,
		Priority:          100,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: transcodeSourceFingerprint(media),
		PlannerVersion:    transcodePlannerVersion,
	}
	if err := service.executionRepo.CreateJob(executionJob); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := service.executionRepo.ClaimJob(executionJob.ID, "worker-"+suffix, time.Now(), time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim %s: ok=%v err=%v", suffix, ok, err)
	}

	attempt := &model.TranscodeAttemptRecord{
		ID:       "attempt-" + suffix,
		JobID:    executionJob.ID,
		Number:   1,
		Backend:  "none",
		Status:   "completed",
		ExitCode: 0,
	}
	if err := service.executionRepo.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	workspace, err := service.artifactStore.PrepareWorkspace(executionJob.ID, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	attempt.WorkspacePath = workspace
	if err := db.Model(&model.TranscodeAttemptRecord{}).
		Where("id = ?", attempt.ID).
		Update("workspace_path", workspace).Error; err != nil {
		t.Fatal(err)
	}
	writeConcurrentHLSArtifact(t, workspace, suffix)

	if running, err := service.executionRepo.SetJobRunning(
		executionJob.ID,
		attempt.ID,
		claimed.LeaseToken,
		time.Now(),
	); err != nil || !running {
		t.Fatalf("set %s running: running=%v err=%v", suffix, running, err)
	}
	claimed.Status = "running"
	claimed.CurrentAttemptID = attempt.ID

	artifact := &model.TranscodeArtifactRecord{
		ID:                "artifact-" + suffix,
		JobID:             executionJob.ID,
		AttemptID:         attempt.ID,
		MediaID:           media.ID,
		Kind:              "hls_variant",
		ProfileID:         task.Quality,
		SourceFingerprint: executionJob.SourceFingerprint,
		PlannerVersion:    executionJob.PlannerVersion,
		TempPath:          workspace,
		Status:            "staging",
		DurationMS:        4000,
		SegmentDuration:   hlsTargetSegmentSeconds,
	}
	if err := service.executionRepo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	task.OutputDir = workspace

	return concurrentArtifactFixture{
		job: &TranscodeJob{
			Task:            task,
			ExecutionJob:    claimed,
			CurrentAttempt:  attempt,
			CurrentArtifact: artifact,
			Media:           media,
			Quality:         task.Quality,
			ctx:             context.Background(),
			leaseToken:      claimed.LeaseToken,
		},
		workspace: workspace,
	}
}

func publishConcurrentArtifacts(first, second *TranscodeJob, service *TranscodeService) [2]concurrentPublishResult {
	start := make(chan struct{})
	var results [2]concurrentPublishResult
	var wg sync.WaitGroup
	for index, job := range []*TranscodeJob{first, second} {
		wg.Add(1)
		go func(index int, job *TranscodeJob) {
			defer wg.Done()
			<-start
			results[index].published, results[index].err = service.publishCurrentHLSArtifact(job)
		}(index, job)
	}
	close(start)
	wg.Wait()
	return results
}

func assertConcurrentArtifactPublished(t *testing.T, service *TranscodeService, db *gorm.DB, fixture concurrentArtifactFixture) {
	t.Helper()
	artifact := fixture.job.CurrentArtifact
	if artifact.Status != "published" || artifact.Path == "" || artifact.ManifestPath == "" || artifact.TempPath != "" {
		t.Fatalf("in-memory artifact was not published: %+v", artifact)
	}
	if _, err := os.Stat(artifact.ManifestPath); err != nil {
		t.Fatalf("published manifest missing: %v", err)
	}
	if _, err := os.Stat(fixture.workspace); !os.IsNotExist(err) {
		t.Fatalf("workspace was not atomically moved: %v", err)
	}

	var storedArtifact model.TranscodeArtifactRecord
	if err := db.First(&storedArtifact, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedArtifact.Status != "published" || storedArtifact.Path != artifact.Path || storedArtifact.TempPath != "" {
		t.Fatalf("stored artifact mismatch: %+v", storedArtifact)
	}
	storedJob, err := service.executionRepo.FindJobByID(fixture.job.ExecutionJob.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedJob.Status != "completed" || storedJob.ActiveKey != nil || storedJob.LeaseToken != "" || storedJob.CurrentAttemptID != "" {
		t.Fatalf("job and artifact were not committed together: %+v", storedJob)
	}
	if _, err := service.ResolveHLSOutputDir(fixture.job.Media, fixture.job.Quality); !errors.Is(err, ErrPersistentRuntimeTranscodeRetired) {
		t.Fatalf("published historical artifact became runtime-readable: %v", err)
	}
}

func writeConcurrentHLSArtifact(t *testing.T, dir, suffix string) {
	t.Helper()
	segmentName := "seg-" + suffix + ".ts"
	if err := os.WriteFile(filepath.Join(dir, segmentName), []byte("segment-"+suffix), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2.0,\n" + segmentName + "\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(filepath.Join(dir, "stream.m3u8"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
