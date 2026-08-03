package service

import (
	"context"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestPlaylistPinsSegmentsAcrossArtifactReplacement(t *testing.T) {
	transcoder, db := newConcurrentArtifactService(t)
	repos := repository.NewRepositories(db)
	stream := NewStreamService(repos.Media, repos.Series, transcoder, &config.Config{}, zap.NewNop().Sugar())
	media := createReplacementMedia(t, db)

	first := createReplacementArtifactFixture(t, transcoder, db, media, "first")
	published, err := transcoder.publishCurrentHLSArtifact(first.job)
	if err != nil || !published {
		t.Fatalf("publish first artifact: published=%v err=%v", published, err)
	}
	firstPlaylist, err := stream.GetArtifactSegmentPlaylist(media.ID, first.job.Quality)
	if err != nil {
		t.Fatal(err)
	}
	assertPlaylistArtifactVersion(t, firstPlaylist, first.job.CurrentArtifact.ID)

	second := createReplacementArtifactFixture(t, transcoder, db, media, "second")
	published, err = transcoder.publishCurrentHLSArtifact(second.job)
	if err != nil || !published {
		t.Fatalf("publish replacement artifact: published=%v err=%v", published, err)
	}
	secondPlaylist, err := stream.GetArtifactSegmentPlaylist(media.ID, second.job.Quality)
	if err != nil {
		t.Fatal(err)
	}
	assertPlaylistArtifactVersion(t, secondPlaylist, second.job.CurrentArtifact.ID)
	if strings.Contains(secondPlaylist, first.job.CurrentArtifact.ID) {
		t.Fatalf("replacement playlist still references old artifact: %s", secondPlaylist)
	}

	var storedFirst model.TranscodeArtifactRecord
	if err := db.First(&storedFirst, "id = ?", first.job.CurrentArtifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedFirst.Status != "superseded" {
		t.Fatalf("first artifact was not superseded: %+v", storedFirst)
	}

	assertVersionedSegmentBody(t, stream, media.ID, "720p", storedFirst.ID, "segment-first")
	assertVersionedSegmentBody(t, stream, media.ID, "720p", second.job.CurrentArtifact.ID, "segment-second")

	// Both versions intentionally use the same segment basename. The Artifact
	// query must be the only selector and must prevent cross-version mixing.
	request := httptest.NewRequest("GET", "/api/stream/"+media.ID+"/720p/seg0000.ts?artifact="+storedFirst.ID, nil)
	response := httptest.NewRecorder()
	if err := stream.ServeArtifactSegmentVersion(media.ID, "720p", storedFirst.ID, "seg0000.ts", response, request); err != nil {
		t.Fatal(err)
	}
	if got := response.Body.String(); got != "segment-first" {
		t.Fatalf("old playlist crossed into replacement bytes: %q", got)
	}

	// A retained Artifact may serve only files from its own immutable directory.
	// replacement-only.ts exists in version two but not version one.
	if err := stream.ServeArtifactSegmentVersion(media.ID, "720p", storedFirst.ID, "replacement-only.ts", httptest.NewRecorder(), request); !errors.Is(err, ErrArtifactNotReady) {
		t.Fatalf("explicit Artifact unexpectedly fell through to another version: %v", err)
	}

	// A freshly superseded version is not eligible for terminal cleanup until
	// the configured retention cutoff has elapsed.
	terminal, err := transcoder.executionRepo.ListTerminalArtifactsBefore(time.Now().Add(-time.Minute), 100)
	if err != nil {
		t.Fatal(err)
	}
	for _, artifact := range terminal {
		if artifact.ID == storedFirst.ID {
			t.Fatalf("fresh superseded artifact was exposed to cleanup: %+v", artifact)
		}
	}
}

func TestBindHLSArtifactVersionRejectsNonLocalMediaURI(t *testing.T) {
	for _, playlist := range []string{
		"#EXTM3U\nhttps://cdn.example/seg0000.ts\n",
		"#EXTM3U\nsubdir/seg0000.ts\n",
		"#EXTM3U\n../seg0000.ts\n",
	} {
		if _, err := bindHLSArtifactVersion(playlist, "artifact-id"); err == nil {
			t.Fatalf("unsafe playlist URI was accepted: %s", playlist)
		}
	}

	bound, err := bindHLSArtifactVersion("#EXTM3U\nseg0000.ts?token=public\n", "artifact-id")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(bound, "seg0000.ts?artifact=artifact-id&token=public") {
		t.Fatalf("existing query was not preserved: %s", bound)
	}
}

func createReplacementMedia(t *testing.T, db *gorm.DB) *model.Media {
	t.Helper()
	mediaPath := filepath.Join(t.TempDir(), "replacement-source.mkv")
	if err := os.WriteFile(mediaPath, []byte("replacement-source"), 0o644); err != nil {
		t.Fatal(err)
	}
	media := &model.Media{
		ID:         "media-artifact-replacement",
		Title:      "Artifact replacement",
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
	return media
}

func createReplacementArtifactFixture(t *testing.T, service *TranscodeService, db *gorm.DB, media *model.Media, suffix string) concurrentArtifactFixture {
	t.Helper()
	task := &model.TranscodeTask{
		ID:         "replacement-task-" + suffix,
		MediaID:    media.ID,
		Quality:    "720p",
		Status:     "running",
		MediaTitle: media.Title,
		Progress:   100,
	}
	if err := service.repo.Create(task); err != nil {
		t.Fatal(err)
	}

	activeKey := "replacement|" + media.ID + "|720p"
	legacyTaskID := task.ID
	executionJob := &model.TranscodeJobRecord{
		ID:                "replacement-job-" + suffix,
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
	claimed, ok, err := service.executionRepo.ClaimJob(executionJob.ID, "replacement-worker-"+suffix, time.Now(), time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim replacement %s: ok=%v err=%v", suffix, ok, err)
	}

	attempt := &model.TranscodeAttemptRecord{
		ID:       "replacement-attempt-" + suffix,
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
	writeReplacementHLSArtifact(t, workspace, suffix)

	if running, err := service.executionRepo.SetJobRunning(executionJob.ID, attempt.ID, claimed.LeaseToken, time.Now()); err != nil || !running {
		t.Fatalf("set replacement %s running: running=%v err=%v", suffix, running, err)
	}
	claimed.Status = "running"
	claimed.CurrentAttemptID = attempt.ID

	artifact := &model.TranscodeArtifactRecord{
		ID:                "replacement-artifact-" + suffix,
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

func writeReplacementHLSArtifact(t *testing.T, dir, suffix string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "seg0000.ts"), []byte("segment-"+suffix), 0o644); err != nil {
		t.Fatal(err)
	}
	if suffix == "second" {
		if err := os.WriteFile(filepath.Join(dir, "replacement-only.ts"), []byte("replacement-only-"+suffix), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	manifest := "#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2.0,\nseg0000.ts\n#EXT-X-ENDLIST\n"
	if err := os.WriteFile(filepath.Join(dir, "stream.m3u8"), []byte(manifest), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertPlaylistArtifactVersion(t *testing.T, playlist, artifactID string) {
	t.Helper()
	expected := "seg0000.ts?artifact=" + artifactID
	if !strings.Contains(playlist, expected) {
		t.Fatalf("playlist is not pinned to artifact %s: %s", artifactID, playlist)
	}
}

func assertVersionedSegmentBody(t *testing.T, stream *StreamService, mediaID, quality, artifactID, expected string) {
	t.Helper()
	request := httptest.NewRequest("GET", "/api/stream/"+mediaID+"/"+quality+"/seg0000.ts?artifact="+artifactID, nil)
	response := httptest.NewRecorder()
	if err := stream.ServeArtifactSegmentVersion(mediaID, quality, artifactID, "seg0000.ts", response, request); err != nil {
		t.Fatal(err)
	}
	if got := response.Body.String(); got != expected {
		t.Fatalf("unexpected segment bytes for artifact %s: %q", artifactID, got)
	}
	if got := response.Header().Get("X-Nowen-Artifact-ID"); got != artifactID {
		t.Fatalf("artifact response identity mismatch: got=%q want=%q", got, artifactID)
	}
	if got := response.Header().Get("Cache-Control"); !strings.Contains(got, "immutable") {
		t.Fatalf("published Artifact response is not immutable: %q", got)
	}
}
