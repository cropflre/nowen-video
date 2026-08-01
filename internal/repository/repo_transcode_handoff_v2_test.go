package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestHandoffV2UpsertPersistsTimestampProjection(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	now := time.Date(2026, 8, 1, 6, 10, 0, 0, time.UTC)
	record := &model.TranscodeHandoffAttestationRecord{
		MediaID:                        "media-handoff-v2",
		ProfileID:                      "720p",
		StartupArtifactID:              "startup-v2",
		ContinuationArtifactID:         "continuation-v2",
		SchemaVersion:                  handoffTimelineSchemaV2,
		EncodingPlanVersion:            "hls-encoding-plan-v1",
		EncodingPlanHash:               "encoding-plan",
		TimestampPlanVersion:           "hls-timestamp-normalization-v1",
		TimestampPlanHash:              "timestamp-plan",
		StartupTimelineOriginMS:        0,
		ContinuationTimelineOriginMS:   30_000,
		ExpectedBoundaryMS:             30_000,
		StartupAttestationVersion:      "hls-produced-media-attestation-v1",
		StartupAttestationHash:         "startup-attestation",
		ContinuationAttestationVersion: "hls-produced-media-attestation-v1",
		ContinuationAttestationHash:    "continuation-provisional",
		Status:                         "aligned",
		ContractHash:                   "contract-provisional",
		ContractJSON:                   `{"schema_version":"startup-handoff-timeline-v2","state":"provisional"}`,
		DiscontinuityRequired:          true,
		DecisionReason:                 "client_certification_pending",
		EvaluatedAt:                    now,
		CreatedAt:                      now,
		UpdatedAt:                      now,
	}
	if err := repo.UpsertHandoffAttestation(record); err != nil {
		t.Fatal(err)
	}

	// Simulate the final verified evidence update and also repair a damaged
	// timestamp projection. ON CONFLICT must update all canonical projections.
	if err := repo.db.Model(&model.TranscodeHandoffAttestationRecord{}).
		Where("startup_artifact_id = ? AND continuation_artifact_id = ? AND schema_version = ?", record.StartupArtifactID, record.ContinuationArtifactID, record.SchemaVersion).
		Updates(map[string]any{
			"timestamp_plan_hash":            "damaged",
			"continuation_timeline_origin_ms": 1,
			"expected_boundary_ms":            1,
		}).Error; err != nil {
		t.Fatal(err)
	}
	record.ContinuationAttestationHash = "continuation-verified"
	record.ContractHash = "contract-verified"
	record.ContractJSON = `{"schema_version":"startup-handoff-timeline-v2","state":"verified"}`
	record.EvaluatedAt = now.Add(time.Minute)
	record.UpdatedAt = record.EvaluatedAt
	if err := repo.UpsertHandoffAttestation(record); err != nil {
		t.Fatal(err)
	}
	found, err := repo.FindHandoffAttestation(record.StartupArtifactID, record.ContinuationArtifactID, record.SchemaVersion)
	if err != nil {
		t.Fatal(err)
	}
	if found.TimestampPlanHash != record.TimestampPlanHash ||
		found.ContinuationTimelineOriginMS != 30_000 || found.ExpectedBoundaryMS != 30_000 ||
		found.ContinuationAttestationHash != "continuation-verified" {
		t.Fatalf("handoff v2 projection was not repaired by upsert: %+v", found)
	}
}

func TestHandoffV2RejectsIncompleteTimestampIdentity(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	record := testHandoffAttestationRecord(time.Now())
	record.SchemaVersion = handoffTimelineSchemaV2
	if err := repo.UpsertHandoffAttestation(record); err == nil {
		t.Fatal("handoff v2 without timestamp identity was accepted")
	}
}
