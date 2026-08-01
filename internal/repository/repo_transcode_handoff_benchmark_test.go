package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// BenchmarkFindHandoffAttestation is the EVENT playlist reload baseline after a
// Startup/Continuation pair has already been evaluated. The hot path should
// read one small immutable contract rather than rerun ffprobe or rewrite rows.
func BenchmarkFindHandoffAttestation(b *testing.B) {
	dsn := fmt.Sprintf("file:handoff-benchmark-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		b.Fatal(err)
	}
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now()
	record := &model.TranscodeHandoffAttestationRecord{
		MediaID:                        "benchmark-media",
		ProfileID:                      "720p",
		StartupArtifactID:              "benchmark-startup",
		ContinuationArtifactID:         "benchmark-continuation",
		SchemaVersion:                  "startup-handoff-timeline-v1",
		EncodingPlanVersion:            "hls-encoding-plan-v1",
		EncodingPlanHash:               "benchmark-plan",
		StartupAttestationVersion:      "hls-produced-media-attestation-v1",
		StartupAttestationHash:         "benchmark-startup-attestation",
		ContinuationAttestationVersion: "hls-produced-media-attestation-v1",
		ContinuationAttestationHash:    "benchmark-continuation-attestation",
		Status:                         "overlap",
		ContractHash:                   "benchmark-contract",
		ContractJSON:                   `{"schema_version":"startup-handoff-timeline-v1"}`,
		DiscontinuityRequired:          true,
		DecisionReason:                 "timeline_overlap",
		EvaluatedAt:                    now,
		CreatedAt:                      now,
		UpdatedAt:                      now,
	}
	if err := repo.UpsertHandoffAttestation(record); err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolved, resolveErr := repo.FindHandoffAttestation(
			record.StartupArtifactID,
			record.ContinuationArtifactID,
			record.SchemaVersion,
		)
		if resolveErr != nil || resolved.ContractHash != record.ContractHash {
			b.Fatalf("resolve handoff attestation: record=%+v err=%v", resolved, resolveErr)
		}
	}
}
