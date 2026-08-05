package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func createRunningArtifactFixture(t *testing.T) (*TranscodeExecutionRepo, *model.TranscodeJobRecord, *model.TranscodeAttemptRecord, *model.TranscodeArtifactRecord, time.Time) {
	t.Helper()
	repo := newTranscodeExecutionTestRepo(t)
	activeKey := "artifact-fixture-key"
	job := &model.TranscodeJobRecord{
		MediaID:           "media-artifact",
		Intent:            "runtime_hls",
		ProfileID:         "720p",
		Priority:          100,
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: "source-v1",
		PlannerVersion:    "runtime-hls-v2",
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "worker-a", now, time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim job: ok=%v err=%v", ok, err)
	}
	attempt := &model.TranscodeAttemptRecord{
		JobID:         job.ID,
		Number:        1,
		Backend:       "none",
		Status:        "running",
		WorkspacePath: "/cache/workspaces/job/attempt/hls",
		ExitCode:      -1,
	}
	if err := repo.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	if running, err := repo.SetJobRunning(job.ID, attempt.ID, claimed.LeaseToken, now.Add(time.Second)); err != nil || !running {
		t.Fatalf("set job running: running=%v err=%v", running, err)
	}
	artifact := &model.TranscodeArtifactRecord{
		JobID:             job.ID,
		AttemptID:         attempt.ID,
		MediaID:           job.MediaID,
		Kind:              "hls_variant",
		ProfileID:         job.ProfileID,
		SourceFingerprint: job.SourceFingerprint,
		PlannerVersion:    job.PlannerVersion,
		TempPath:          attempt.WorkspacePath,
		Status:            "staging",
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	claimed.CurrentAttemptID = attempt.ID
	claimed.Status = "running"
	return repo, claimed, attempt, artifact, now
}

func TestReadableArtifactRequiresCurrentLiveLease(t *testing.T) {
	repo, job, attempt, artifact, now := createRunningArtifactFixture(t)
	resolved, err := repo.FindReadableHLSArtifact(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		now.Add(2*time.Second),
	)
	if err != nil || resolved.ID != artifact.ID {
		t.Fatalf("live staging artifact was not resolved: artifact=%+v err=%v", resolved, err)
	}

	if err := repo.AbandonArtifactsForAttempt(attempt.ID, "lease_lost", "test", now.Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindReadableHLSArtifact(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		now.Add(4*time.Second),
	); !IsNotFound(err) {
		t.Fatalf("abandoned artifact remained readable: %v", err)
	}

	stored, err := repo.FindJobByID(job.ID)
	if err != nil || stored.CurrentAttemptID != attempt.ID {
		t.Fatalf("fixture job changed unexpectedly: job=%+v err=%v", stored, err)
	}
}

func TestArtifactPublishAndJobCompletionShareOneLeaseFencedCommit(t *testing.T) {
	repo, job, attempt, artifact, now := createRunningArtifactFixture(t)
	publishedDir := "/cache/artifacts/media/720p/version"
	manifestPath := publishedDir + "/stream.m3u8"

	if prepared, err := repo.PrepareArtifactPublish(
		job.ID,
		attempt.ID,
		artifact.ID,
		"wrong-token",
		publishedDir,
		manifestPath,
		now.Add(2*time.Second),
	); err != nil || prepared {
		t.Fatalf("wrong Lease prepared publish: prepared=%v err=%v", prepared, err)
	}
	if prepared, err := repo.PrepareArtifactPublish(
		job.ID,
		attempt.ID,
		artifact.ID,
		job.LeaseToken,
		publishedDir,
		manifestPath,
		now.Add(2*time.Second),
	); err != nil || !prepared {
		t.Fatalf("owner could not prepare publish: prepared=%v err=%v", prepared, err)
	}

	if committed, err := repo.CommitArtifactPublishAndCompleteJob(
		job.ID,
		attempt.ID,
		artifact.ID,
		"wrong-token",
		1234,
		60000,
		now.Add(3*time.Second),
	); err != nil || committed {
		t.Fatalf("wrong Lease committed publish: committed=%v err=%v", committed, err)
	}
	if committed, err := repo.CommitArtifactPublishAndCompleteJob(
		job.ID,
		attempt.ID,
		artifact.ID,
		job.LeaseToken,
		1234,
		60000,
		now.Add(3*time.Second),
	); err != nil || !committed {
		t.Fatalf("owner could not commit publish: committed=%v err=%v", committed, err)
	}

	var storedArtifact model.TranscodeArtifactRecord
	if err := repo.db.First(&storedArtifact, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if storedArtifact.Status != "published" || storedArtifact.Path != publishedDir || storedArtifact.ManifestPath != manifestPath || storedArtifact.SizeBytes != 1234 {
		t.Fatalf("unexpected published artifact: %+v", storedArtifact)
	}
	storedJob, err := repo.FindJobByID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if storedJob.Status != "completed" || storedJob.ActiveKey != nil || storedJob.LeaseToken != "" || storedJob.CurrentAttemptID != "" {
		t.Fatalf("job and artifact were not committed together: %+v", storedJob)
	}
}

func TestOldWorkerCannotPublishAfterLeaseRequeue(t *testing.T) {
	repo, job, attempt, artifact, now := createRunningArtifactFixture(t)
	if requeued, err := repo.RequeueLeasedJob(job.ID, job.LeaseToken, now.Add(2*time.Second)); err != nil || !requeued {
		t.Fatalf("requeue failed: requeued=%v err=%v", requeued, err)
	}
	if prepared, err := repo.PrepareArtifactPublish(
		job.ID,
		attempt.ID,
		artifact.ID,
		job.LeaseToken,
		"/cache/artifacts/stale",
		"/cache/artifacts/stale/stream.m3u8",
		now.Add(3*time.Second),
	); err != nil || prepared {
		t.Fatalf("old worker prepared after Lease requeue: prepared=%v err=%v", prepared, err)
	}
}
