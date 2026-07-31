package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTranscodeExecutionTestRepo(t *testing.T) *TranscodeExecutionRepo {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	return NewTranscodeExecutionRepo(db)
}

func TestTranscodeExecutionRepoActiveKeyLifecycle(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	activeKey := "media-1|runtime_hls|720p"
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
