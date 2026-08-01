package service

import (
	"fmt"
	"os"
	"path/filepath"
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

func TestArtifactResolverStopsServingWorkspaceAfterLeaseFence(t *testing.T) {
	dsn := fmt.Sprintf("file:artifact-resolver-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Media{}, &model.TranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	repos := repository.NewRepositories(db)
	executionRepo := repository.NewTranscodeExecutionRepo(db)
	cacheDir := t.TempDir()
	artifactStore, err := transcodeartifactstore.New(filepath.Join(cacheDir, "transcode"))
	if err != nil {
		t.Fatal(err)
	}
	service := &TranscodeService{
		repo:          repos.Transcode,
		executionRepo: executionRepo,
		artifactStore: artifactStore,
		cfg:           &config.Config{Cache: config.CacheConfig{CacheDir: cacheDir}},
		logger:        zap.NewNop().Sugar(),
	}

	mediaPath := filepath.Join(t.TempDir(), "movie.mkv")
	if err := os.WriteFile(mediaPath, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	media := &model.Media{ID: "media-resolver", FilePath: mediaPath, Duration: 60, Resolution: "720p"}
	if err := repos.Media.Create(media); err != nil {
		t.Fatal(err)
	}
	fingerprint := transcodeSourceFingerprint(media)
	activeKey := "resolver-active-key"
	job := &model.TranscodeJobRecord{
		MediaID:           media.ID,
		Intent:            "runtime_hls",
		ProfileID:         "720p",
		Priority:          100,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: fingerprint,
		PlannerVersion:    transcodePlannerVersion,
	}
	if err := executionRepo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	claimed, ok, err := executionRepo.ClaimJob(job.ID, "worker-a", now, time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	attempt := &model.TranscodeAttemptRecord{JobID: job.ID, Number: 1, Backend: "none", Status: "running", ExitCode: -1}
	if err := executionRepo.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	workspace, err := artifactStore.PrepareWorkspace(job.ID, attempt.ID)
	if err != nil {
		t.Fatal(err)
	}
	writeTestHLSArtifact(t, workspace)
	if running, err := executionRepo.SetJobRunning(job.ID, attempt.ID, claimed.LeaseToken, now.Add(time.Second)); err != nil || !running {
		t.Fatalf("set job running: running=%v err=%v", running, err)
	}
	artifact := &model.TranscodeArtifactRecord{
		JobID:             job.ID,
		AttemptID:         attempt.ID,
		MediaID:           media.ID,
		Kind:              "hls_variant",
		ProfileID:         "720p",
		SourceFingerprint: fingerprint,
		PlannerVersion:    transcodePlannerVersion,
		TempPath:          workspace,
		Status:            "staging",
	}
	if err := executionRepo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}

	resolved, err := service.ResolveHLSOutputDir(media, "720p")
	if err != nil || resolved != workspace {
		t.Fatalf("live workspace not resolved: path=%s err=%v", resolved, err)
	}
	if requeued, err := executionRepo.RequeueLeasedJob(job.ID, claimed.LeaseToken, now.Add(2*time.Second)); err != nil || !requeued {
		t.Fatalf("requeue lease: requeued=%v err=%v", requeued, err)
	}
	if _, err := service.ResolveHLSOutputDir(media, "720p"); err == nil {
		t.Fatal("stale workspace remained readable after Lease requeue")
	}

	publishedDir, err := artifactStore.PublishedDir(media.ID, "720p", "published-version")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(publishedDir, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTestHLSArtifact(t, publishedDir)
	publishedAt := now.Add(3 * time.Second)
	published := &model.TranscodeArtifactRecord{
		ID:                "published-version",
		JobID:             job.ID,
		AttemptID:         attempt.ID,
		MediaID:           media.ID,
		Kind:              "hls_variant",
		ProfileID:         "720p",
		SourceFingerprint: fingerprint,
		PlannerVersion:    transcodePlannerVersion,
		Path:              publishedDir,
		ManifestPath:      filepath.Join(publishedDir, "stream.m3u8"),
		Status:            "published",
		PublishedAt:       &publishedAt,
	}
	if err := executionRepo.CreateArtifact(published); err != nil {
		t.Fatal(err)
	}
	resolved, err = service.ResolveHLSOutputDir(media, "720p")
	if err != nil || resolved != publishedDir {
		t.Fatalf("published artifact not resolved: path=%s err=%v", resolved, err)
	}
}

func writeTestHLSArtifact(t *testing.T, dir string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "seg0000.ts"), []byte("segment"), 0o644); err != nil {
		t.Fatal(err)
	}
	manifest := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2.0,\nseg0000.ts\n"
	if err := os.WriteFile(filepath.Join(dir, "stream.m3u8"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}
