package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestArtifactCleanupOperatorRequeuePreservesEvidence(t *testing.T) {
	db := newArtifactCleanupTestDB(t)
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now().UTC().Truncate(time.Second)
	old := now.Add(-8 * 24 * time.Hour)
	artifact := &model.TranscodeArtifactRecord{
		ID:                  "blocked-operator-artifact",
		JobID:               "blocked-operator-job",
		MediaID:             "blocked-media",
		Kind:                "hls_variant",
		ProfileID:           "1080p",
		SourceFingerprint:   "source",
		PlannerVersion:      "planner",
		Path:                "/cache/transcode/artifacts/blocked",
		Status:              "superseded",
		CleanupState:        ArtifactCleanupBlocked,
		CleanupAttempts:     3,
		CleanupErrorCode:    "cleanup_invariant_violation",
		CleanupErrorMessage: "artifact path escapes store root",
		CreatedAt:           old,
		UpdatedAt:           old,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}

	operations, err := repo.ListArtifactCleanupOperations(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(operations) != 1 || operations[0].ID != artifact.ID {
		t.Fatalf("blocked operation not exposed: %+v", operations)
	}

	requeued, ok, err := repo.RequeueArtifactCleanup(artifact.ID, now)
	if err != nil || !ok {
		t.Fatalf("operator requeue failed: ok=%v err=%v", ok, err)
	}
	if requeued.CleanupState != ArtifactCleanupPending {
		t.Fatalf("cleanup state=%q, want pending", requeued.CleanupState)
	}
	if requeued.CleanupAttempts != 3 {
		t.Fatalf("operator requeue erased attempt evidence: %d", requeued.CleanupAttempts)
	}
	if requeued.CleanupErrorCode != "" || requeued.CleanupErrorMessage != "" {
		t.Fatalf("operator requeue did not clear stale error: %+v", requeued)
	}
	if requeued.CleanupNextAttemptAt == nil || !requeued.CleanupNextAttemptAt.Equal(now) {
		t.Fatalf("operator requeue did not make work immediately eligible: %+v", requeued.CleanupNextAttemptAt)
	}

	if _, ok, err := repo.RequeueArtifactCleanup(artifact.ID, now.Add(time.Second)); err != nil || ok {
		t.Fatalf("pending cleanup was requeued twice: ok=%v err=%v", ok, err)
	}
}

func TestArtifactCleanupOperatorCannotStealLiveClaim(t *testing.T) {
	db := newArtifactCleanupTestDB(t)
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now().UTC().Truncate(time.Second)
	leaseExpiresAt := now.Add(time.Minute)
	artifact := &model.TranscodeArtifactRecord{
		ID:                    "claimed-operator-artifact",
		JobID:                 "claimed-operator-job",
		MediaID:               "claimed-media",
		Kind:                  "hls_variant",
		ProfileID:             "720p",
		SourceFingerprint:     "source",
		PlannerVersion:        "planner",
		Status:                "expired",
		CleanupState:          ArtifactCleanupClaimed,
		CleanupToken:          "live-owner",
		CleanupLeaseExpiresAt: &leaseExpiresAt,
		CreatedAt:             now.Add(-8 * 24 * time.Hour),
		UpdatedAt:             now,
	}
	if err := db.Create(artifact).Error; err != nil {
		t.Fatal(err)
	}

	if _, ok, err := repo.RequeueArtifactCleanup(artifact.ID, now); err != nil || ok {
		t.Fatalf("operator stole live cleanup claim: ok=%v err=%v", ok, err)
	}
	stored, err := repo.FindArtifactCleanupOperation(artifact.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.CleanupState != ArtifactCleanupClaimed || stored.CleanupToken != "live-owner" {
		t.Fatalf("live claim mutated by operator: %+v", stored)
	}
}
