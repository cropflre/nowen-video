package repository

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const handoffTimelineSchemaV2 = "startup-handoff-timeline-v2"

func (r *TranscodeExecutionRepo) UpsertHandoffAttestation(record *model.TranscodeHandoffAttestationRecord) error {
	if r == nil || r.db == nil || record == nil {
		return gorm.ErrInvalidData
	}
	if record.MediaID == "" || record.StartupArtifactID == "" || record.ContinuationArtifactID == "" ||
		record.SchemaVersion == "" || record.ContractHash == "" || record.ContractJSON == "" ||
		record.EncodingPlanVersion == "" || record.EncodingPlanHash == "" ||
		record.StartupAttestationVersion == "" || record.StartupAttestationHash == "" ||
		record.ContinuationAttestationVersion == "" || record.ContinuationAttestationHash == "" ||
		record.Status == "" || record.DecisionReason == "" || record.EvaluatedAt.IsZero() {
		return gorm.ErrInvalidData
	}
	if record.SchemaVersion == handoffTimelineSchemaV2 &&
		(record.TimestampPlanVersion == "" || record.TimestampPlanHash == "" ||
			record.StartupTimelineOriginMS < 0 || record.ContinuationTimelineOriginMS <= record.StartupTimelineOriginMS ||
			record.ExpectedBoundaryMS != record.ContinuationTimelineOriginMS) {
		return gorm.ErrInvalidData
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{
			{Name: "startup_artifact_id"},
			{Name: "continuation_artifact_id"},
			{Name: "schema_version"},
		},
		DoUpdates: clause.AssignmentColumns([]string{
			"media_id",
			"profile_id",
			"encoding_plan_version",
			"encoding_plan_hash",
			"timestamp_plan_version",
			"timestamp_plan_hash",
			"startup_timeline_origin_ms",
			"continuation_timeline_origin_ms",
			"expected_boundary_ms",
			"startup_attestation_version",
			"startup_attestation_hash",
			"continuation_attestation_version",
			"continuation_attestation_hash",
			"status",
			"contract_hash",
			"contract_json",
			"video_presentation_delta_micros",
			"video_decode_delta_micros",
			"audio_presentation_delta_micros",
			"audio_decode_delta_micros",
			"seamless_allowed",
			"discontinuity_required",
			"decision_reason",
			"evaluated_at",
			"updated_at",
		}),
	}).Create(record).Error
}

func (r *TranscodeExecutionRepo) FindHandoffAttestation(
	startupArtifactID,
	continuationArtifactID,
	schemaVersion string,
) (*model.TranscodeHandoffAttestationRecord, error) {
	if r == nil || r.db == nil || startupArtifactID == "" || continuationArtifactID == "" || schemaVersion == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var record model.TranscodeHandoffAttestationRecord
	result := r.db.Where(
		"startup_artifact_id = ? AND continuation_artifact_id = ? AND schema_version = ?",
		startupArtifactID,
		continuationArtifactID,
		schemaVersion,
	).Limit(1).Find(&record)
	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, gorm.ErrRecordNotFound
	}
	return &record, nil
}

func (r *TranscodeExecutionRepo) DeleteHandoffAttestationsForArtifact(artifactID string, deletedAt time.Time) error {
	if r == nil || r.db == nil || artifactID == "" {
		return nil
	}
	return r.db.Where(
		"startup_artifact_id = ? OR continuation_artifact_id = ?",
		artifactID,
		artifactID,
	).Delete(&model.TranscodeHandoffAttestationRecord{}).Error
}
