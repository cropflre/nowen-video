package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

func newTranscodeExecutionTestRepo(t *testing.T) *TranscodeExecutionRepo {
	t.Helper()
	dsn := fmt.Sprintf("file:transcode-execution-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	return NewTranscodeExecutionRepo(db)
}

func createQueuedExecutionJob(t *testing.T, repo *TranscodeExecutionRepo, activeKey string) *model.TranscodeJobRecord {
	t.Helper()
	job := &model.TranscodeJobRecord{
		MediaID:      "media-1",
		Intent:       "runtime_hls",
		ProfileID:    "720p",
		Status:       "queued",
		DesiredState: "running",
		ActiveKey:    &activeKey,
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	return job
}

func TestTranscodeExecutionRepoActiveKeyLifecycle(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	activeKey := "media-1|runtime_hls|720p"
	job := createQueuedExecutionJob(t, repo, activeKey)
	found, err := repo.FindActiveByKey(activeKey)
	if err != nil || found.ID != job.ID {
		t.Fatalf("find active: job=%+v err=%v", found, err)
	}

	now := time.Now()
	if err := repo.CompleteJob(job.ID, "completed", now); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindActiveByKey(activeKey); !IsNotFound(err) {
		t.Fatalf("completed job must release active key, err=%v", err)
	}

	second := &model.TranscodeJobRecord{
		MediaID:      "media-1",
		Intent:       "runtime_hls",
		ProfileID:    "720p",
		Status:       "queued",
		DesiredState: "running",
		ActiveKey:    &activeKey,
	}
	if err := repo.CreateJob(second); err != nil {
		t.Fatalf("active key should be reusable after completion: %v", err)
	}
}

func TestTranscodeExecutionRepoPromotesOnlyUnclaimedQueuedJob(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := createQueuedExecutionJob(t, repo, "promote-key")
	job.Priority = 20
	if err := repo.db.Model(&model.TranscodeJobRecord{}).Where("id = ?", job.ID).Update("priority", 20).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	promoted, err := repo.PromoteQueuedJob(job.ID, 100, now)
	if err != nil || !promoted {
		t.Fatalf("queued job promotion failed: promoted=%v err=%v", promoted, err)
	}
	stored, err := repo.FindJobByID(job.ID)
	if err != nil || stored.Priority != 100 {
		t.Fatalf("priority was not persisted: job=%+v err=%v", stored, err)
	}
	if promoted, err := repo.PromoteQueuedJob(job.ID, 70, now.Add(time.Second)); err != nil || promoted {
		t.Fatalf("lower priority must not demote job: promoted=%v err=%v", promoted, err)
	}

	claimed, ok, err := repo.ClaimJob(job.ID, "worker", now.Add(2*time.Second), time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim failed: job=%+v ok=%v err=%v", claimed, ok, err)
	}
	if promoted, err := repo.PromoteQueuedJob(job.ID, 120, now.Add(3*time.Second)); err != nil || promoted {
		t.Fatalf("claimed job must not be promoted: promoted=%v err=%v", promoted, err)
	}
}

func TestTranscodeExecutionRepoAtomicClaimAndLeaseOwnership(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := createQueuedExecutionJob(t, repo, "claim-key")
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)

	claimed, ok, err := repo.ClaimJob(job.ID, "instance-a/worker-0", now, 20*time.Second)
	if err != nil || !ok {
		t.Fatalf("first claim failed: claimed=%+v ok=%v err=%v", claimed, ok, err)
	}
	if claimed.Status != "claimed" || claimed.WorkerID != "instance-a/worker-0" || claimed.LeaseToken == "" {
		t.Fatalf("unexpected claimed job: %+v", claimed)
	}

	if second, secondOK, secondErr := repo.ClaimJob(job.ID, "instance-b/worker-0", now, 20*time.Second); secondErr != nil || secondOK || second != nil {
		t.Fatalf("second worker must not claim the same job: job=%+v ok=%v err=%v", second, secondOK, secondErr)
	}

	if renewed, renewErr := repo.RenewJobLease(job.ID, "wrong-token", now.Add(5*time.Second), 30*time.Second); renewErr != nil || renewed {
		t.Fatalf("wrong token renewed lease: renewed=%v err=%v", renewed, renewErr)
	}
	if renewed, renewErr := repo.RenewJobLease(job.ID, claimed.LeaseToken, now.Add(5*time.Second), 30*time.Second); renewErr != nil || !renewed {
		t.Fatalf("lease renewal failed: renewed=%v err=%v", renewed, renewErr)
	}

	stored, err := repo.FindJobByID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.LeaseExpiresAt == nil || !stored.LeaseExpiresAt.After(now.Add(20*time.Second)) {
		t.Fatalf("lease was not extended: %+v", stored.LeaseExpiresAt)
	}

	if running, runningErr := repo.SetJobRunning(job.ID, "attempt-1", "wrong-token", now.Add(6*time.Second)); runningErr != nil || running {
		t.Fatalf("wrong token marked job running: running=%v err=%v", running, runningErr)
	}
	if running, runningErr := repo.SetJobRunning(job.ID, "attempt-1", claimed.LeaseToken, now.Add(6*time.Second)); runningErr != nil || !running {
		t.Fatalf("owner failed to mark job running: running=%v err=%v", running, runningErr)
	}
	if running, runningErr := repo.SetJobRunning(job.ID, "attempt-2", claimed.LeaseToken, now.Add(7*time.Second)); runningErr != nil || !running {
		t.Fatalf("fallback attempt failed to retain lease: running=%v err=%v", running, runningErr)
	}
	stored, err = repo.FindJobByID(job.ID)
	if err != nil || stored.CurrentAttemptID != "attempt-2" {
		t.Fatalf("current attempt did not advance: job=%+v err=%v", stored, err)
	}

	completedAt := now.Add(20 * time.Second)
	if completed, completeErr := repo.CompleteLeasedJob(job.ID, "wrong-token", "completed", completedAt); completeErr != nil || completed {
		t.Fatalf("wrong token completed job: completed=%v err=%v", completed, completeErr)
	}
	if completed, completeErr := repo.CompleteLeasedJob(job.ID, claimed.LeaseToken, "completed", completedAt); completeErr != nil || !completed {
		t.Fatalf("lease owner failed to complete job: completed=%v err=%v", completed, completeErr)
	}
	if _, err := repo.FindActiveByKey("claim-key"); !IsNotFound(err) {
		t.Fatalf("leased completion must release active key, err=%v", err)
	}
}

func TestTranscodeExecutionRepoCancellationPreventsClaim(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := createQueuedExecutionJob(t, repo, "cancel-key")
	now := time.Now()
	if err := repo.RequestCancellation(job.ID, now); err != nil {
		t.Fatal(err)
	}
	if claimed, ok, err := repo.ClaimJob(job.ID, "worker", now.Add(time.Second), time.Minute); err != nil || ok || claimed != nil {
		t.Fatalf("cancelled queue entry was claimed: job=%+v ok=%v err=%v", claimed, ok, err)
	}
}

func TestTranscodeExecutionRepoExpiredLeaseRecovery(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := createQueuedExecutionJob(t, repo, "expired-key")
	now := time.Date(2026, 7, 31, 8, 0, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "worker", now, 10*time.Second)
	if err != nil || !ok {
		t.Fatalf("claim failed: ok=%v err=%v", ok, err)
	}

	if recovered, recoverErr := repo.CompleteExpiredLease(job.ID, claimed.LeaseToken, "failed", now.Add(9*time.Second)); recoverErr != nil || recovered {
		t.Fatalf("live lease was recovered early: recovered=%v err=%v", recovered, recoverErr)
	}
	if completed, completeErr := repo.CompleteLeasedJob(job.ID, claimed.LeaseToken, "completed", now.Add(11*time.Second)); completeErr != nil || completed {
		t.Fatalf("expired worker wrote terminal success: completed=%v err=%v", completed, completeErr)
	}
	if recovered, recoverErr := repo.CompleteExpiredLease(job.ID, claimed.LeaseToken, "failed", now.Add(11*time.Second)); recoverErr != nil || !recovered {
		t.Fatalf("expired lease was not recovered: recovered=%v err=%v", recovered, recoverErr)
	}

	stored, err := repo.FindJobByID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failed" || stored.ActiveKey != nil || stored.LeaseToken != "" || stored.WorkerID != "" || stored.LeaseExpiresAt != nil {
		t.Fatalf("expired lease was not fully released: %+v", stored)
	}
	if recovered, recoverErr := repo.CompleteExpiredLease(job.ID, claimed.LeaseToken, "failed", now.Add(12*time.Second)); recoverErr != nil || recovered {
		t.Fatalf("lease recovery must be idempotent: recovered=%v err=%v", recovered, recoverErr)
	}
}

func TestTranscodeExecutionRepoRetainsAttemptEvidence(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job := &model.TranscodeJobRecord{MediaID: "m", Intent: "runtime_hls", Status: "queued", DesiredState: "running"}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	attempt := &model.TranscodeAttemptRecord{JobID: job.ID, Number: 1, Backend: "qsv", Status: "preparing", ExitCode: -1}
	if err := repo.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if err := repo.MarkAttemptStarted(attempt.ID, 1234, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteAttempt(attempt.ID, "failed", 1, "device busy", "process_failed", "exit status 1", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}

	var stored model.TranscodeAttemptRecord
	if err := repo.db.First(&stored, "id = ?", attempt.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "failed" || stored.PID != 1234 || stored.StderrTail != "device busy" || stored.ExitCode != 1 {
		t.Fatalf("unexpected attempt evidence: %+v", stored)
	}
}
