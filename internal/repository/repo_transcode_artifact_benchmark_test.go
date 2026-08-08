package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// BenchmarkFindReadableHLSArtifact preserves the general Runtime HLS playback
// resolver baseline. Run with:
// go test ./internal/repository -run '^$' -bench FindReadableHLSArtifact -benchmem
func BenchmarkFindReadableHLSArtifact(b *testing.B) {
	repo, artifact, now := newArtifactBenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolved, resolveErr := repo.FindReadableHLSArtifact(
			artifact.MediaID,
			artifact.ProfileID,
			artifact.SourceFingerprint,
			artifact.PlannerVersion,
			now,
		)
		if resolveErr != nil || resolved.ID != artifact.ID {
			b.Fatalf("resolve artifact: artifact=%+v err=%v", resolved, resolveErr)
		}
	}
}

// BenchmarkFindReadableArtifactByEncodingPlan preserves the previous attested
// resolver baseline so the cost added by Timestamp Plan predicates remains
// visible rather than being hidden by replacing the old benchmark.
func BenchmarkFindReadableArtifactByEncodingPlan(b *testing.B) {
	repo, artifact, now := newArtifactBenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolved, resolveErr := repo.FindReadableArtifactByEncodingPlan(
			artifact.MediaID,
			artifact.ProfileID,
			artifact.SourceFingerprint,
			artifact.PlannerVersion,
			artifact.Kind,
			artifact.EncodingPlanVersion,
			artifact.EncodingPlanHash,
			now,
		)
		if resolveErr != nil || resolved.ID != artifact.ID {
			b.Fatalf("resolve attested artifact: artifact=%+v err=%v", resolved, resolveErr)
		}
	}
}

// BenchmarkFindReadableArtifactByExecutionContract is the current Startup
// Bridge hot path. It includes Encoding Plan, Timestamp Plan, timeline origin,
// produced-media attestation and normal published fallback predicates.
func BenchmarkFindReadableArtifactByExecutionContract(b *testing.B) {
	repo, artifact, now := newArtifactBenchmarkFixture(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resolved, resolveErr := repo.FindReadableArtifactByExecutionContract(
			artifact.MediaID,
			artifact.ProfileID,
			artifact.SourceFingerprint,
			artifact.PlannerVersion,
			artifact.Kind,
			artifact.EncodingPlanVersion,
			artifact.EncodingPlanHash,
			artifact.TimestampPlanVersion,
			artifact.TimestampPlanHash,
			artifact.TimelineOriginMS,
			now,
		)
		if resolveErr != nil || resolved.ID != artifact.ID {
			b.Fatalf("resolve execution-contract artifact: artifact=%+v err=%v", resolved, resolveErr)
		}
	}
}

func newArtifactBenchmarkFixture(b *testing.B) (*TranscodeExecutionRepo, *model.TranscodeArtifactRecord, time.Time) {
	b.Helper()
	dsn := fmt.Sprintf("file:artifact-benchmark-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		b.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		b.Fatal(err)
	}
	repo := NewTranscodeExecutionRepo(db)
	now := time.Now()
	publishedAt := now
	job := &model.TranscodeJobRecord{
		ID:                   "benchmark-job",
		MediaID:              "benchmark-media",
		Intent:               "startup_continuation_hls",
		ProfileID:            "1080p",
		StartMS:              30_000,
		Status:               "completed",
		DesiredState:         "running",
		SourceFingerprint:    "benchmark-source",
		PlannerVersion:       "startup-continuation-hls-v4",
		EncodingPlanVersion:  "hls-encoding-plan-v1",
		EncodingPlanHash:     "benchmark-encoding-plan",
		EncodingPlanJSON:     `{"schema_version":"hls-encoding-plan-v1","profile_id":"1080p"}`,
		TimestampPlanVersion: "hls-timestamp-normalization-v1",
		TimestampPlanHash:    "benchmark-timestamp-plan",
		TimestampPlanJSON:    `{"schema_version":"hls-timestamp-normalization-v1"}`,
		TimelineOriginMS:     30_000,
		CompletedAt:          &publishedAt,
	}
	if err := repo.CreateJob(job); err != nil {
		b.Fatal(err)
	}
	artifact := &model.TranscodeArtifactRecord{
		JobID:                job.ID,
		AttemptID:            "benchmark-attempt",
		MediaID:              job.MediaID,
		Kind:                 "startup_continuation_hls",
		ProfileID:            job.ProfileID,
		SourceFingerprint:    job.SourceFingerprint,
		PlannerVersion:       job.PlannerVersion,
		EncodingPlanVersion:  job.EncodingPlanVersion,
		EncodingPlanHash:     job.EncodingPlanHash,
		EncodingPlanJSON:     job.EncodingPlanJSON,
		TimestampPlanVersion: job.TimestampPlanVersion,
		TimestampPlanHash:    job.TimestampPlanHash,
		TimestampPlanJSON:    job.TimestampPlanJSON,
		TimelineOriginMS:     job.TimelineOriginMS,
		AttestationVersion:   "hls-produced-media-attestation-v1",
		AttestationStatus:    "verified",
		AttestationHash:      "benchmark-attestation",
		AttestationJSON:      `{"schema_version":"hls-produced-media-attestation-v1","scope":"complete"}`,
		TimelineStartMS:      31_400,
		TimelineEndMS:        61_400,
		AttestedAt:           &publishedAt,
		Path:                 "/cache/artifacts/benchmark",
		ManifestPath:         "/cache/artifacts/benchmark/stream.m3u8",
		Status:               "published",
		PublishedAt:          &publishedAt,
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		b.Fatal(err)
	}
	return repo, artifact, now
}
