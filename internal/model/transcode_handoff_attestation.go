package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// TranscodeHandoffAttestationRecord stores the server-owned packet timeline
// decision for one Startup Artifact and one Continuation Artifact. The canonical
// contract does not expose filesystem paths, Job IDs, Attempt IDs or Lease
// tokens. Artifact IDs are retained only as internal foreign identity for
// invalidation, observability and deterministic upsert.
type TranscodeHandoffAttestationRecord struct {
	ID                             string    `json:"id" gorm:"primaryKey;type:text"`
	MediaID                        string    `json:"media_id" gorm:"index;type:text;not null"`
	ProfileID                      string    `json:"profile_id" gorm:"index;type:text"`
	StartupArtifactID              string    `json:"startup_artifact_id" gorm:"uniqueIndex:idx_transcode_handoff_identity,priority:1;index;type:text;not null"`
	ContinuationArtifactID         string    `json:"continuation_artifact_id" gorm:"uniqueIndex:idx_transcode_handoff_identity,priority:2;index;type:text;not null"`
	SchemaVersion                  string    `json:"schema_version" gorm:"uniqueIndex:idx_transcode_handoff_identity,priority:3;index;type:text;not null"`
	EncodingPlanVersion            string    `json:"encoding_plan_version" gorm:"type:text;not null"`
	EncodingPlanHash               string    `json:"encoding_plan_hash" gorm:"index;type:text;not null"`
	TimestampPlanVersion           string    `json:"timestamp_plan_version" gorm:"type:text;not null"`
	TimestampPlanHash              string    `json:"timestamp_plan_hash" gorm:"index;type:text;not null"`
	StartupTimelineOriginMS        int64     `json:"startup_timeline_origin_ms" gorm:"index"`
	ContinuationTimelineOriginMS   int64     `json:"continuation_timeline_origin_ms" gorm:"index"`
	ExpectedBoundaryMS             int64     `json:"expected_boundary_ms" gorm:"index"`
	StartupAttestationVersion      string    `json:"startup_attestation_version" gorm:"type:text;not null"`
	StartupAttestationHash         string    `json:"startup_attestation_hash" gorm:"index;type:text;not null"`
	ContinuationAttestationVersion string    `json:"continuation_attestation_version" gorm:"type:text;not null"`
	ContinuationAttestationHash    string    `json:"continuation_attestation_hash" gorm:"index;type:text;not null"`
	Status                         string    `json:"status" gorm:"index;type:text;not null"`
	ContractHash                   string    `json:"contract_hash" gorm:"index;type:text;not null"`
	ContractJSON                   string    `json:"contract_json" gorm:"type:text;not null"`
	VideoPresentationDeltaMicros   int64     `json:"video_presentation_delta_micros"`
	VideoDecodeDeltaMicros         int64     `json:"video_decode_delta_micros"`
	AudioPresentationDeltaMicros   int64     `json:"audio_presentation_delta_micros"`
	AudioDecodeDeltaMicros         int64     `json:"audio_decode_delta_micros"`
	SeamlessAllowed                bool      `json:"seamless_allowed" gorm:"index"`
	DiscontinuityRequired          bool      `json:"discontinuity_required" gorm:"index"`
	DecisionReason                 string    `json:"decision_reason" gorm:"index;type:text;not null"`
	EvaluatedAt                    time.Time `json:"evaluated_at" gorm:"index;not null"`
	CreatedAt                      time.Time `json:"created_at"`
	UpdatedAt                      time.Time `json:"updated_at"`
}

func (TranscodeHandoffAttestationRecord) TableName() string {
	return "transcode_handoff_attestations"
}

func (r *TranscodeHandoffAttestationRecord) BeforeCreate(*gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.NewString()
	}
	return nil
}
