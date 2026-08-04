package repository

import (
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

func TestConcurrentStorageReservationLedgerPreventsDoubleSpend(t *testing.T) {
	dsn := fmt.Sprintf(
		"file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)",
		filepath.Join(t.TempDir(), "storage-reservation.db"),
	)
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

	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeStorageReservation(db); err != nil {
		t.Fatal(err)
	}
	repoA := NewTranscodeExecutionRepo(db.Session(&gorm.Session{}))
	repoB := NewTranscodeExecutionRepo(db.Session(&gorm.Session{}))
	first := createQueuedExecutionJob(t, repoA, "concurrent-reservation-first")
	second := createQueuedExecutionJob(t, repoA, "concurrent-reservation-second")

	budget := TranscodeStorageReservationBudget{AvailableBytes: 1_000, SampledAt: time.Now()}
	start := make(chan struct{})
	type result struct {
		reservation *model.TranscodeStorageReservationRecord
		err         error
	}
	results := make(chan result, 2)
	var wg sync.WaitGroup
	for _, input := range []struct {
		repo  *TranscodeExecutionRepo
		jobID string
	}{
		{repo: repoA, jobID: first.ID},
		{repo: repoB, jobID: second.ID},
	} {
		wg.Add(1)
		go func(repo *TranscodeExecutionRepo, jobID string) {
			defer wg.Done()
			<-start
			reservation, acquireErr := repo.AcquireJobStorageReservation(
				jobID,
				700,
				budget,
				time.Now(),
			)
			results <- result{reservation: reservation, err: acquireErr}
		}(input.repo, input.jobID)
	}
	close(start)
	wg.Wait()
	close(results)

	successes := 0
	capacityFailures := 0
	for item := range results {
		switch {
		case item.err == nil:
			successes++
			if item.reservation == nil || item.reservation.ReservedBytes != 700 {
				t.Fatalf("successful reservation evidence invalid: %+v", item.reservation)
			}
		case errors.Is(item.err, ErrTranscodeStorageReservationCapacity):
			capacityFailures++
		default:
			t.Fatalf("unexpected concurrent reservation error: %v", item.err)
		}
	}
	if successes != 1 || capacityFailures != 1 {
		t.Fatalf("ledger double-spend fence failed: successes=%d capacity_failures=%d", successes, capacityFailures)
	}

	summary, err := repoA.StorageReservationSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.ActiveCount != 1 || summary.ActiveBytes != 700 || summary.WaitingCount != 1 {
		t.Fatalf("unexpected concurrent reservation summary: %+v", summary)
	}
}
