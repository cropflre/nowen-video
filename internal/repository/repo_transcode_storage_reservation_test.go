package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestStorageReservationLedgerPreventsOvercommit(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	if err := model.AutoMigrateTranscodeStorageReservation(repo.db); err != nil {
		t.Fatal(err)
	}
	first := createQueuedExecutionJob(t, repo, "reservation-first")
	second := createQueuedExecutionJob(t, repo, "reservation-second")
	now := time.Now()
	budget := TranscodeStorageReservationBudget{AvailableBytes: 1_000, SampledAt: now}

	reservation, err := repo.AcquireJobStorageReservation(first.ID, 700, budget, now)
	if err != nil {
		t.Fatal(err)
	}
	if reservation.ReservedBytes != 700 || reservation.State != model.TranscodeStorageReservationActive {
		t.Fatalf("unexpected first reservation: %+v", reservation)
	}
	_, err = repo.AcquireJobStorageReservation(second.ID, 400, budget, now.Add(time.Second))
	if !errors.Is(err, ErrTranscodeStorageReservationCapacity) {
		t.Fatalf("expected capacity fence, got %v", err)
	}
	capacityErr := &TranscodeStorageReservationCapacityError{}
	if !errors.As(err, &capacityErr) || capacityErr.ActiveBytes != 700 || capacityErr.AvailableBytes != 1_000 {
		t.Fatalf("capacity evidence missing: %+v", capacityErr)
	}

	if _, err := repo.AcquireJobStorageReservation(second.ID, 300, budget, now.Add(2*time.Second)); err != nil {
		t.Fatalf("remaining capacity should fit second reservation: %v", err)
	}
	summary, err := repo.StorageReservationSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.ActiveCount != 2 || summary.ActiveBytes != 1_000 || summary.WaitingCount != 0 {
		t.Fatalf("unexpected active reservation summary: %+v", summary)
	}
}

func TestStorageReservationIsIdempotentPerJob(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	if err := model.AutoMigrateTranscodeStorageReservation(repo.db); err != nil {
		t.Fatal(err)
	}
	job := createQueuedExecutionJob(t, repo, "reservation-idempotent")
	now := time.Now()
	budget := TranscodeStorageReservationBudget{AvailableBytes: 1_000, SampledAt: now}
	first, err := repo.AcquireJobStorageReservation(job.ID, 700, budget, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.AcquireJobStorageReservation(job.ID, 900, budget, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if first.ReservedBytes != second.ReservedBytes || second.ReservedBytes != 700 {
		t.Fatalf("idempotent acquire changed reservation: first=%+v second=%+v", first, second)
	}
}

func TestTerminalJobStopsConsumingReservationImmediately(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	if err := model.AutoMigrateTranscodeStorageReservation(repo.db); err != nil {
		t.Fatal(err)
	}
	first := createQueuedExecutionJob(t, repo, "reservation-release-first")
	second := createQueuedExecutionJob(t, repo, "reservation-release-second")
	now := time.Now()
	budget := TranscodeStorageReservationBudget{AvailableBytes: 700, SampledAt: now}
	if _, err := repo.AcquireJobStorageReservation(first.ID, 700, budget, now); err != nil {
		t.Fatal(err)
	}
	if err := repo.CompleteJob(first.ID, "completed", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.AcquireJobStorageReservation(second.ID, 700, budget, now.Add(2*time.Second)); err != nil {
		t.Fatalf("terminal reservation still consumed capacity: %v", err)
	}

	summary, err := repo.StorageReservationSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.ActiveCount != 1 || summary.ActiveBytes != 700 {
		t.Fatalf("terminal reservation remained active: %+v", summary)
	}
	released, err := repo.ReconcileReleasedStorageReservations(now.Add(3 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if released != 1 {
		t.Fatalf("expected one released audit row, got %d", released)
	}
	var stored model.TranscodeStorageReservationRecord
	if err := repo.db.First(&stored, "job_id = ?", first.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.State != model.TranscodeStorageReservationReleased || stored.ReleasedAt == nil {
		t.Fatalf("reservation audit row was not released: %+v", stored)
	}
}

func TestQueuedJobWithoutReservationIsReportedWaiting(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	if err := model.AutoMigrateTranscodeStorageReservation(repo.db); err != nil {
		t.Fatal(err)
	}
	createQueuedExecutionJob(t, repo, "reservation-waiting")
	summary, err := repo.StorageReservationSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.WaitingCount != 1 || summary.ActiveCount != 0 {
		t.Fatalf("waiting reservation was not projected: %+v", summary)
	}
}
