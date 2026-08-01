package repository

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// BenchmarkFindReadableHLSArtifact is the repeatable local/CI performance
// baseline for the playback hot path. Run with:
// go test ./internal/repository -run '^$' -bench FindReadableHLSArtifact -benchmem
func BenchmarkFindReadableHLSArtifact(b *testing.B) {
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
	artifact := &model.TranscodeArtifactRecord{
		JobID:             "benchmark-job",
		AttemptID:         "benchmark-attempt",
		MediaID:           "benchmark-media",
		Kind:              "hls_variant",
		ProfileID:         "1080p",
		SourceFingerprint: "benchmark-source",
		PlannerVersion:    "runtime-hls-v2",
		Path:              "/cache/artifacts/benchmark",
		ManifestPath:      "/cache/artifacts/benchmark/stream.m3u8",
		Status:            "published",
		PublishedAt:       &publishedAt,
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		b.Fatal(err)
	}
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
