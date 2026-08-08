package repository

import (
	"math"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func prepareObservedReservationJob(t *testing.T, repo *TranscodeExecutionRepo, activeKey, attemptID string, reservedBytes int64) (*model.TranscodeJobRecord, time.Time) {
	t.Helper()
	if err := model.AutoMigrateTranscodeStorageReservation(repo.db); err != nil {
		t.Fatal(err)
	}
	job := createQueuedExecutionJob(t, repo, activeKey)
	now := time.Date(2026, 8, 4, 2, 30, 0, 0, time.UTC)
	if _, err := repo.AcquireJobStorageReservation(
		job.ID,
		reservedBytes,
		TranscodeStorageReservationBudget{AvailableBytes: reservedBytes * 4, SampledAt: now},
		now,
	); err != nil {
		t.Fatal(err)
	}
	claimed, ok, err := repo.ClaimJob(job.ID, "instance/worker-0", now.Add(time.Second), time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim reservation job: job=%+v ok=%v err=%v", claimed, ok, err)
	}
	if running, err := repo.SetJobRunning(job.ID, attemptID, claimed.LeaseToken, now.Add(2*time.Second)); err != nil || !running {
		t.Fatalf("set reservation job running: running=%v err=%v", running, err)
	}
	claimed.CurrentAttemptID = attemptID
	claimed.Status = "running"
	return claimed, now
}

func TestStorageReservationObservedBytesRefundOnlyFutureCommitment(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job, now := prepareObservedReservationJob(t, repo, "consumption-first", "attempt-1", 1_000)

	updated, err := repo.ObserveOwnedJobStorageReservation(
		job.ID,
		"attempt-1",
		job.LeaseToken,
		400,
		now.Add(3*time.Second),
	)
	if err != nil || !updated {
		t.Fatalf("observe reservation: updated=%v err=%v", updated, err)
	}
	summary, err := repo.StorageReservationSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.ReservedBytes != 1_000 || summary.ObservedBytes != 400 || summary.RemainingBytes != 600 || summary.ActiveBytes != 600 {
		t.Fatalf("actual bytes were not refunded from future commitment: %+v", summary)
	}

	second := createQueuedExecutionJob(t, repo, "consumption-second")
	if _, err := repo.AcquireJobStorageReservation(
		second.ID,
		400,
		TranscodeStorageReservationBudget{AvailableBytes: 1_000, SampledAt: now.Add(4 * time.Second)},
		now.Add(4*time.Second),
	); err != nil {
		t.Fatalf("materialized bytes were still double-counted: %v", err)
	}
}

func TestStorageReservationObservationIsLeaseAndAttemptFenced(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job, now := prepareObservedReservationJob(t, repo, "consumption-fence", "attempt-1", 1_000)

	if updated, err := repo.ObserveOwnedJobStorageReservation(job.ID, "attempt-1", "stale-token", 500, now.Add(3*time.Second)); err != nil || updated {
		t.Fatalf("stale lease updated reservation: updated=%v err=%v", updated, err)
	}
	if updated, err := repo.ObserveOwnedJobStorageReservation(job.ID, "attempt-old", job.LeaseToken, 500, now.Add(3*time.Second)); err != nil || updated {
		t.Fatalf("stale attempt updated reservation: updated=%v err=%v", updated, err)
	}
	if updated, err := repo.ObserveOwnedJobStorageReservation(job.ID, "attempt-1", job.LeaseToken, 500, now.Add(3*time.Second)); err != nil || !updated {
		t.Fatalf("current owner failed observation: updated=%v err=%v", updated, err)
	}

	if running, err := repo.SetJobRunning(job.ID, "attempt-2", job.LeaseToken, now.Add(4*time.Second)); err != nil || !running {
		t.Fatalf("advance fallback attempt: running=%v err=%v", running, err)
	}
	if updated, err := repo.ObserveOwnedJobStorageReservation(job.ID, "attempt-2", job.LeaseToken, 100, now.Add(5*time.Second)); err != nil || !updated {
		t.Fatalf("fallback observation failed: updated=%v err=%v", updated, err)
	}
	var stored model.TranscodeStorageReservationRecord
	if err := repo.db.First(&stored, "job_id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.AttemptID != "attempt-2" || stored.ObservedBytes != 100 || stored.PeakObservedBytes != 500 {
		t.Fatalf("attempt reset or peak evidence incorrect: %+v", stored)
	}
}

func TestPublishedReservationRecordsPredictionCalibration(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job, now := prepareObservedReservationJob(t, repo, "consumption-published", "attempt-1", 1_000)
	if updated, err := repo.ObserveOwnedJobStorageReservation(job.ID, "attempt-1", job.LeaseToken, 700, now.Add(3*time.Second)); err != nil || !updated {
		t.Fatalf("observe published reservation: updated=%v err=%v", updated, err)
	}
	publishedAt := now.Add(4 * time.Second)
	artifact := &model.TranscodeArtifactRecord{
		ID:          "artifact-calibration",
		JobID:       job.ID,
		AttemptID:   "attempt-1",
		MediaID:     job.MediaID,
		Kind:        "hls_variant",
		ProfileID:   job.ProfileID,
		Status:      "published",
		SizeBytes:   800,
		PublishedAt: &publishedAt,
		CreatedAt:   now,
		UpdatedAt:   publishedAt,
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	if completed, err := repo.CompleteLeasedJob(job.ID, job.LeaseToken, "completed", publishedAt); err != nil || !completed {
		t.Fatalf("complete published job: completed=%v err=%v", completed, err)
	}
	if err := repo.FinalizePublishedJobStorageReservation(job.ID, "attempt-1", publishedAt); err != nil {
		t.Fatal(err)
	}

	var stored model.TranscodeStorageReservationRecord
	if err := repo.db.First(&stored, "job_id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.State != model.TranscodeStorageReservationReleased || stored.Outcome != "completed" || stored.FinalBytes != 800 || stored.PredictionErrorBytes != -200 {
		t.Fatalf("published calibration evidence incorrect: %+v", stored)
	}
	if math.Abs(stored.ActualToEstimateRatio-0.8) > 0.0001 {
		t.Fatalf("unexpected actual/estimate ratio: %f", stored.ActualToEstimateRatio)
	}
	summary, err := repo.StorageReservationSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.CalibrationSamples != 1 || math.Abs(summary.AverageActualToEstimate-0.8) > 0.0001 || math.Abs(summary.AverageAbsoluteError-0.2) > 0.0001 || summary.UnderpredictedCount != 0 {
		t.Fatalf("calibration aggregate incorrect: %+v", summary)
	}
}

func TestFailedReservationRetainsPeakAndReleasesAudit(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	job, now := prepareObservedReservationJob(t, repo, "consumption-failed", "attempt-1", 1_000)
	if updated, err := repo.ObserveOwnedJobStorageReservation(job.ID, "attempt-1", job.LeaseToken, 450, now.Add(3*time.Second)); err != nil || !updated {
		t.Fatalf("observe failed reservation: updated=%v err=%v", updated, err)
	}
	failedAt := now.Add(4 * time.Second)
	if completed, err := repo.CompleteLeasedJob(job.ID, job.LeaseToken, "failed", failedAt); err != nil || !completed {
		t.Fatalf("complete failed job: completed=%v err=%v", completed, err)
	}
	if err := repo.ReleaseJobStorageReservation(job.ID, "failed", failedAt); err != nil {
		t.Fatal(err)
	}
	var stored model.TranscodeStorageReservationRecord
	if err := repo.db.First(&stored, "job_id = ?", job.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.State != model.TranscodeStorageReservationReleased || stored.Outcome != "failed" || stored.ObservedBytes != 450 || stored.PeakObservedBytes != 450 || stored.FinalBytes != 0 {
		t.Fatalf("failed reservation evidence incorrect: %+v", stored)
	}
}
