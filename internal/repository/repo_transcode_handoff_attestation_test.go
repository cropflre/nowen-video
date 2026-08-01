package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestHandoffAttestationUpsertTracksCurrentContinuationEvidence(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	now := time.Date(2026, 8, 1, 4, 0, 0, 0, time.UTC)
	record := testHandoffAttestationRecord(now)
	if err := repo.UpsertHandoffAttestation(record); err != nil {
		t.Fatal(err)
	}
	found, err := repo.FindHandoffAttestation(record.StartupArtifactID, record.ContinuationArtifactID, record.SchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if found.ContractHash != "contract-provisional" || !found.DiscontinuityRequired || found.SeamlessAllowed {
		t.Fatalf("unexpected persisted handoff: %+v", found)
	}

	record.ContinuationAttestationHash = "continuation-verified"
	record.ContractHash = "contract-verified"
	record.ContractJSON = `{"schema_version":"startup-handoff-timeline-v1","state":"verified"}`
	record.EvaluatedAt = now.Add(time.Minute)
	record.UpdatedAt = record.EvaluatedAt
	if err := repo.UpsertHandoffAttestation(record); err != nil {
		t.Fatal(err)
	}
	found, err = repo.FindHandoffAttestation(record.StartupArtifactID, record.ContinuationArtifactID, record.SchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if found.ContinuationAttestationHash != "continuation-verified" || found.ContractHash != "contract-verified" {
		t.Fatalf("handoff upsert did not replace current evidence: %+v", found)
	}
	var count int64
	if err := repo.db.Model(&model.TranscodeHandoffAttestationRecord{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("handoff identity created duplicate rows: %d", count)
	}
}

func TestDeleteHandoffAttestationsForArtifactPreventsOrphans(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	record := testHandoffAttestationRecord(time.Date(2026, 8, 1, 4, 0, 0, 0, time.UTC))
	if err := repo.UpsertHandoffAttestation(record); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteHandoffAttestationsForArtifact(record.StartupArtifactID, time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindHandoffAttestation(record.StartupArtifactID, record.ContinuationArtifactID, record.SchemaVersion); !IsNotFound(err) {
		t.Fatalf("handoff attestation survived startup artifact cleanup: %v", err)
	}
}

func TestHandoffAttestationRequiresCompleteIdentity(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	if err := repo.UpsertHandoffAttestation(&model.TranscodeHandoffAttestationRecord{}); err == nil {
		t.Fatal("incomplete handoff attestation was accepted")
	}
}

func testHandoffAttestationRecord(now time.Time) *model.TranscodeHandoffAttestationRecord {
	return &model.TranscodeHandoffAttestationRecord{
		MediaID:                        "media-handoff",
		ProfileID:                      "720p",
		StartupArtifactID:              "startup-artifact",
		ContinuationArtifactID:         "continuation-artifact",
		SchemaVersion:                  "startup-handoff-timeline-v1",
		EncodingPlanVersion:            "hls-encoding-plan-v1",
		EncodingPlanHash:               "plan-hash",
		StartupAttestationVersion:      "hls-produced-media-attestation-v1",
		StartupAttestationHash:         "startup-attestation",
		ContinuationAttestationVersion: "hls-produced-media-attestation-v1",
		ContinuationAttestationHash:    "continuation-provisional",
		Status:                         "overlap",
		ContractHash:                   "contract-provisional",
		ContractJSON:                   `{"schema_version":"startup-handoff-timeline-v1","state":"provisional"}`,
		VideoPresentationDeltaMicros:   -30_000_000,
		VideoDecodeDeltaMicros:         -30_000_000,
		AudioPresentationDeltaMicros:   -30_000_000,
		AudioDecodeDeltaMicros:         -30_000_000,
		DiscontinuityRequired:          true,
		DecisionReason:                 "timeline_overlap",
		EvaluatedAt:                    now,
		CreatedAt:                      now,
		UpdatedAt:                      now,
	}
}
