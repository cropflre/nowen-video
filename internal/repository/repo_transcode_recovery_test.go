package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestTranscodeExecutionRepoRequeuesExpiredLeaseAndClaimsAgain(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := createQueuedExecutionJob(t, repo, "expired-requeue-key")
	now := time.Date(2026, 7, 31, 12, 0, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "instance-a/worker-0", now, 10*time.Second)
	if err != nil || !ok {
		t.Fatalf("claim failed: claimed=%+v ok=%v err=%v", claimed, ok, err)
	}
	if running, runningErr := repo.SetJobRunning(job.ID, "attempt-1", claimed.LeaseToken, now.Add(time.Second)); runningErr != nil || !running {
		t.Fatalf("set running failed: running=%v err=%v", running, runningErr)
	}

	requeued, requeueErr := repo.RequeueExpiredLease(job.ID, claimed.LeaseToken, now.Add(11*time.Second))
	if requeueErr != nil || !requeued {
		t.Fatalf("expired lease was not requeued: requeued=%v err=%v", requeued, requeueErr)
	}
	stored, err := repo.FindJobByID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "queued" || stored.DesiredState != "running" || stored.ActiveKey == nil {
		t.Fatalf("requeued job lost durable queue state: %+v", stored)
	}
	if stored.WorkerID != "" || stored.LeaseToken != "" || stored.LeaseExpiresAt != nil || stored.CurrentAttemptID != "" || stored.CompletedAt != nil {
		t.Fatalf("requeued job retained stale ownership: %+v", stored)
	}

	reclaimed, reclaimedOK, reclaimErr := repo.ClaimJob(job.ID, "instance-b/worker-0", now.Add(12*time.Second), 20*time.Second)
	if reclaimErr != nil || !reclaimedOK || reclaimed == nil {
		t.Fatalf("requeued job could not be claimed again: job=%+v ok=%v err=%v", reclaimed, reclaimedOK, reclaimErr)
	}
	if reclaimed.WorkerID != "instance-b/worker-0" || reclaimed.LeaseToken == "" || reclaimed.LeaseToken == claimed.LeaseToken {
		t.Fatalf("reclaimed job did not receive fresh ownership: %+v", reclaimed)
	}
}

func TestTranscodeExecutionRepoGracefulShutdownReleaseIsFenced(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := createQueuedExecutionJob(t, repo, "shutdown-release-key")
	now := time.Date(2026, 7, 31, 12, 30, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "instance-a/worker-0", now, time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim failed: ok=%v err=%v", ok, err)
	}

	if released, releaseErr := repo.RequeueLeasedJob(job.ID, "wrong-token", now.Add(time.Second)); releaseErr != nil || released {
		t.Fatalf("wrong token released lease: released=%v err=%v", released, releaseErr)
	}
	if released, releaseErr := repo.RequeueLeasedJob(job.ID, claimed.LeaseToken, now.Add(time.Second)); releaseErr != nil || !released {
		t.Fatalf("owner failed to release lease: released=%v err=%v", released, releaseErr)
	}
	if completed, completeErr := repo.CompleteLeasedJob(job.ID, claimed.LeaseToken, "completed", now.Add(2*time.Second)); completeErr != nil || completed {
		t.Fatalf("released worker wrote stale terminal state: completed=%v err=%v", completed, completeErr)
	}
	if reclaimed, reclaimedOK, reclaimErr := repo.ClaimJob(job.ID, "instance-b/worker-0", now.Add(2*time.Second), time.Minute); reclaimErr != nil || !reclaimedOK || reclaimed == nil {
		t.Fatalf("released job was not claimable: job=%+v ok=%v err=%v", reclaimed, reclaimedOK, reclaimErr)
	}
}

func TestTranscodeExecutionRepoRequeuesLegacyUnleasedRow(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := createQueuedExecutionJob(t, repo, "legacy-unleased-key")
	now := time.Now()
	if err := repo.db.Model(&model.TranscodeJobRecord{}).Where("id = ?", job.ID).Updates(map[string]any{
		"status":             "running",
		"worker_id":          "legacy-worker",
		"current_attempt_id": "legacy-attempt",
		"claimed_at":         now,
	}).Error; err != nil {
		t.Fatal(err)
	}

	requeued, err := repo.RequeueUnleasedJob(job.ID, now.Add(time.Second))
	if err != nil || !requeued {
		t.Fatalf("legacy unleased row was not requeued: requeued=%v err=%v", requeued, err)
	}
	stored, err := repo.FindJobByID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "queued" || stored.WorkerID != "" || stored.CurrentAttemptID != "" || stored.ClaimedAt != nil {
		t.Fatalf("legacy ownership was not cleared: %+v", stored)
	}
}

func TestTranscodeExecutionRepoAttemptNumbersContinueAcrossRecovery(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := &model.TranscodeJobRecord{
		MediaID:      "media-attempt",
		Intent:       "runtime_hls",
		ProfileID:    "720p",
		Status:       "queued",
		DesiredState: "running",
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}

	if next, err := repo.NextAttemptNumber(job.ID); err != nil || next != 1 {
		t.Fatalf("unexpected first attempt number: next=%d err=%v", next, err)
	}
	for number := 1; number <= 2; number++ {
		attempt := &model.TranscodeAttemptRecord{
			JobID:    job.ID,
			Number:   number,
			Backend:  "software",
			Status:   "failed",
			ExitCode: 1,
		}
		if err := repo.CreateAttempt(attempt); err != nil {
			t.Fatal(err)
		}
	}
	if next, err := repo.NextAttemptNumber(job.ID); err != nil || next != 3 {
		t.Fatalf("attempt history was not continued: next=%d err=%v", next, err)
	}
}
