package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestArtifactReconciliationKeepsLiveLeaseAndAbandonsExpiredOwner(t *testing.T) {
	repo, _, _, artifact, now := createRunningArtifactFixture(t)

	changed, err := repo.AbandonUnownedArtifacts(now.Add(2 * time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if changed != 0 {
		t.Fatalf("live Lease artifact was abandoned: changed=%d", changed)
	}
	var stored model.TranscodeArtifactRecord
	if err := repo.db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "staging" {
		t.Fatalf("live staging artifact changed state: %+v", stored)
	}

	changed, err = repo.AbandonUnownedArtifacts(now.Add(2 * time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("expired owner artifact was not abandoned: changed=%d", changed)
	}
	if err := repo.db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Status != "abandoned" || stored.ErrorCode != "startup_reconciliation" {
		t.Fatalf("unexpected reconciled artifact: %+v", stored)
	}
}

func TestArtifactReconciliationAbandonsOrphanPublishingRow(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	artifact := &model.TranscodeArtifactRecord{
		JobID:          "missing-job",
		AttemptID:      "missing-attempt",
		MediaID:        "media-orphan",
		Kind:           "hls_variant",
		ProfileID:      "720p",
		PlannerVersion: "runtime-hls-v2",
		Status:         "publishing",
		Path:           "/cache/artifacts/orphan",
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	changed, err := repo.AbandonUnownedArtifacts(time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if changed != 1 {
		t.Fatalf("orphan publishing row was not abandoned: %d", changed)
	}
}
