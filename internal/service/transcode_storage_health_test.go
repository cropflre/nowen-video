package service

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestStorageHealthPausesAdmissionAndRecoversAfterMountRestore(t *testing.T) {
	service, db := newConcurrentArtifactService(t)
	if err := model.AutoMigrateTranscodeStorageIncidents(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	healthy := service.runStorageHealthTick(now, true)
	if healthy.State != "healthy" || !healthy.Writable || healthy.AdmissionBlocked {
		t.Fatalf("unexpected initial storage health: %+v", healthy)
	}

	root := service.artifactStore.Root()
	offline := root + ".offline"
	if err := os.Rename(root, offline); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(root, []byte("disconnected mount"), 0o644); err != nil {
		t.Fatal(err)
	}
	failed := service.runStorageHealthTick(now.Add(time.Second), true)
	if failed.State != "critical" || failed.Writable || !failed.AdmissionBlocked || !failed.QueuePaused || failed.IncidentID == "" {
		t.Fatalf("storage failure was not fenced: %+v", failed)
	}
	if err := service.checkStorageHealthAdmission(); !errors.Is(err, ErrTranscodeStorageUnavailable) {
		t.Fatalf("storage admission did not fail closed: %v", err)
	}
	active, err := service.executionRepo.ListActiveStorageIncidents(10)
	if err != nil || len(active) != 1 {
		t.Fatalf("active incident evidence missing: rows=%+v err=%v", active, err)
	}

	if err := os.Remove(root); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(offline, root); err != nil {
		t.Fatal(err)
	}
	recovered := service.runStorageHealthTick(now.Add(2*time.Second), true)
	if recovered.State != "healthy" || !recovered.Writable || recovered.AdmissionBlocked || recovered.ActiveIncidents != 0 {
		t.Fatalf("restored mount did not recover: %+v", recovered)
	}
	summary, err := service.executionRepo.StorageIncidentSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.RecoveredCount != 1 || summary.ActiveCount != 0 {
		t.Fatalf("incident recovery history missing: %+v", summary)
	}
}

func TestLiveStorageOperationFailureBlocksImmediatelyUntilWriteProbeRecovers(t *testing.T) {
	service, db := newConcurrentArtifactService(t)
	if err := model.AutoMigrateTranscodeStorageIncidents(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	service.runStorageHealthTick(now, true)
	service.reportStorageOperationFailure(
		storageOperationPublishArtifact,
		service.artifactStore.Root(),
		fmt.Errorf("rename artifact: %w", syscall.ENOSPC),
		now.Add(time.Second),
	)
	failed := service.GetStorageHealthStatus()
	if failed.Code != "no_space" || !failed.AdmissionBlocked || failed.Operation != storageOperationPublishArtifact || failed.IncidentID == "" {
		t.Fatalf("live storage failure did not block immediately: %+v", failed)
	}
	active, err := service.executionRepo.ListActiveStorageIncidents(10)
	if err != nil || len(active) != 1 || active[0].Operation != storageOperationPublishArtifact {
		t.Fatalf("live operation incident evidence missing: rows=%+v err=%v", active, err)
	}

	recovered := service.runStorageHealthTick(now.Add(31*time.Second), true)
	if recovered.State != "healthy" || recovered.AdmissionBlocked || recovered.ActiveIncidents != 0 {
		t.Fatalf("real write probe did not recover live incident: %+v", recovered)
	}
}
