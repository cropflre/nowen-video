package service

import (
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

func TestRuntimeHistoryIsReadOnlyPagedAndRedacted(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:runtime-history?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&model.Media{}, &model.TranscodeTask{}); err != nil {
		t.Fatal(err)
	}
	if err := model.AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	media := model.Media{ID: "media-history", Title: "历史影片", FilePath: "/private/media/history.mkv", MediaType: "movie"}
	if err := db.Create(&media).Error; err != nil {
		t.Fatal(err)
	}
	legacy := model.TranscodeTask{ID: "legacy-history", MediaID: media.ID, Status: "cancelled", Quality: "720p", CreatedAt: now.Add(-time.Hour)}
	if err := db.Create(&legacy).Error; err != nil {
		t.Fatal(err)
	}
	legacyID := legacy.ID
	completed := now.Add(-30 * time.Minute)
	job := model.TranscodeJobRecord{
		ID: "job-history", LegacyTaskID: &legacyID, MediaID: media.ID,
		Intent: "retired_runtime_playback", ProfileID: "720p", Status: "cancelled",
		DesiredState: "cancelled", EncodingPlanHash: "encoding-hash",
		TimestampPlanHash: "timestamp-hash", CreatedAt: now.Add(-time.Hour),
		UpdatedAt: completed, CompletedAt: &completed,
	}
	if err := db.Create(&job).Error; err != nil {
		t.Fatal(err)
	}
	attempt := model.TranscodeAttemptRecord{
		ID: "attempt-history", JobID: job.ID, Number: 1, Backend: "nvenc",
		Status: "failed", CommandJSON: `["ffmpeg","-i","secret"]`,
		WorkspacePath: "/private/workspace", ErrorCode: "retired_runtime",
		ErrorMessage: "runtime retired", StderrTail: "diagnostic tail",
		ExitCode: 1, CreatedAt: now.Add(-50 * time.Minute), UpdatedAt: completed,
		CompletedAt: &completed,
	}
	if err := db.Create(&attempt).Error; err != nil {
		t.Fatal(err)
	}
	artifact := model.TranscodeArtifactRecord{
		ID: "artifact-history", JobID: job.ID, AttemptID: attempt.ID, MediaID: media.ID,
		Kind: "hls_variant", ProfileID: "720p", Status: "retired",
		Path: "/private/artifact", TempPath: "/private/temp", ManifestPath: "/private/manifest.m3u8",
		SizeBytes: 4096, CreatedAt: now.Add(-45 * time.Minute), UpdatedAt: completed,
	}
	if err := db.Create(&artifact).Error; err != nil {
		t.Fatal(err)
	}

	history := NewRuntimeHistoryService(repository.NewRuntimeHistoryRepo(db), zap.NewNop().Sugar())
	list, err := history.List(RuntimeHistoryQuery{Page: 1, PageSize: 10, Search: "media-history"})
	if err != nil {
		t.Fatal(err)
	}
	if list.Total != 1 || len(list.Items) != 1 {
		t.Fatalf("unexpected history list: %+v", list)
	}
	item := list.Items[0]
	if item.MediaTitle != media.Title || item.AttemptCount != 1 || item.ArtifactCount != 1 || item.ArtifactBytes != 4096 {
		t.Fatalf("history projection incomplete: %+v", item)
	}
	if item.IntegrityState != "legacy_projection_linked" || item.LastBackend != "nvenc" || item.LastErrorCode != "retired_runtime" {
		t.Fatalf("history integrity evidence missing: %+v", item)
	}
	if list.Retention.AutomaticMetadataPrune || list.Retention.MetadataMode != "indefinite_audit_history" {
		t.Fatalf("unsafe retention policy: %+v", list.Retention)
	}

	detail, err := history.Detail(job.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(detail.Attempts) != 1 || len(detail.Artifacts) != 1 {
		t.Fatalf("history detail incomplete: %+v", detail)
	}
	if detail.Attempts[0].StderrTail != "diagnostic tail" {
		t.Fatalf("diagnostic evidence missing: %+v", detail.Attempts[0])
	}

	summary, err := history.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.Jobs != 1 || summary.Attempts != 1 || summary.Artifacts != 1 || summary.LegacyTasks != 1 || summary.OrphanLegacyTasks != 0 {
		t.Fatalf("unexpected history summary: %+v", summary)
	}
	if summary.ArtifactBytes != 4096 || summary.ByStatus["cancelled"] != 1 {
		t.Fatalf("history summary evidence missing: %+v", summary)
	}
}
