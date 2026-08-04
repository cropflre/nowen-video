package service

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestStorageReservationEstimateUsesRemainingDuration(t *testing.T) {
	job := &model.TranscodeJobRecord{
		ProfileID: "720p",
		StartMS:   30 * 60 * 1000,
	}
	media := &model.Media{
		Duration: 2 * 60 * 60,
		FileSize: 20 * 1024 * 1024 * 1024,
	}
	estimate, err := estimateTranscodeJobStorage(job, media)
	if err != nil {
		t.Fatal(err)
	}
	if estimate.DurationMS != 90*60*1000 {
		t.Fatalf("remaining duration was not used: %+v", estimate)
	}
	if estimate.Fallback != "" || estimate.EstimatedBytes <= 0 {
		t.Fatalf("known duration unexpectedly used fallback: %+v", estimate)
	}
}

func TestStorageReservationEstimateUsesJobDurationForStartup(t *testing.T) {
	job := &model.TranscodeJobRecord{
		ProfileID:  "720p",
		DurationMS: 30_000,
	}
	media := &model.Media{Duration: 2 * 60 * 60}
	estimate, err := estimateTranscodeJobStorage(job, media)
	if err != nil {
		t.Fatal(err)
	}
	if estimate.DurationMS != 30_000 || estimate.EstimatedBytes != 64*1024*1024 {
		t.Fatalf("startup duration reservation mismatch: %+v", estimate)
	}
}

func TestStorageReservationPersistsBeforeClaim(t *testing.T) {
	service, db := newConcurrentArtifactService(t)
	if err := service.initializeStorageReservations(); err != nil {
		t.Fatal(err)
	}
	media := &model.Media{
		ID:        "reservation-media",
		LibraryID: "reservation-library",
		Title:     "Reservation Media",
		FilePath:  "/media/reservation.mkv",
		Duration:  120,
		FileSize:  512 * 1024 * 1024,
	}
	if err := db.Create(media).Error; err != nil {
		t.Fatal(err)
	}
	task := &model.TranscodeTask{
		MediaID:   media.ID,
		Quality:   "720p",
		Status:    "pending",
		OutputDir: service.GetLegacyOutputDir(media.ID, "720p"),
	}
	if err := service.repo.Create(task); err != nil {
		t.Fatal(err)
	}
	activeKey := "reservation-before-claim"
	legacyID := task.ID
	job := &model.TranscodeJobRecord{
		LegacyTaskID: &legacyID,
		MediaID:      media.ID,
		Intent:       "startup_hls",
		ProfileID:    "720p",
		DurationMS:   30_000,
		Status:       "queued",
		DesiredState: "running",
		ActiveKey:    &activeKey,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	if err := service.executionRepo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	if err := service.ensureJobStorageReservation(job.ID); err != nil {
		t.Fatal(err)
	}
	reserved, err := service.executionRepo.HasActiveJobStorageReservation(job.ID)
	if err != nil || !reserved {
		t.Fatalf("reservation was not persisted: reserved=%v err=%v", reserved, err)
	}
	stored, err := service.executionRepo.FindJobByID(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.Status != "queued" || stored.LeaseToken != "" {
		t.Fatalf("reservation acquired the Worker Lease too early: %+v", stored)
	}
}
