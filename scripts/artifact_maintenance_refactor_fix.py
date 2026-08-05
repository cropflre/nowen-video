#!/usr/bin/env python3
from __future__ import annotations

import pathlib

ROOT = pathlib.Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


retirement_path = "internal/service/transcode_runtime_retirement.go"
retirement = read(retirement_path)
marker = 'const retiredRuntimePlaybackIntent = "retired_runtime_playback"\n'
replacement = '''const (
\tretiredRuntimePlaybackIntent    = "retired_runtime_playback"
\tstartupStreamArtifactKind       = "startup_hls"
\tstartupContinuationArtifactKind = "startup_continuation_hls"
)
'''
if marker not in retirement:
    raise RuntimeError("runtime retirement constant marker missing")
write(retirement_path, retirement.replace(marker, replacement))

write(
    "internal/service/media_resolution.go",
    '''package service

import (
\t"strconv"
\t"strings"
)

// parseResolutionHeight normalizes the compact resolution labels stored on a
// Media row. Playback planning owns this helper; it has no relationship with
// the retired persistent Runtime worker.
func parseResolutionHeight(resolution string) int {
\tvalue := strings.TrimSpace(resolution)
\tswitch value {
\tcase "4K":
\t\treturn 2160
\tcase "2K":
\t\treturn 1440
\tcase "1080p":
\t\treturn 1080
\tcase "720p":
\t\treturn 720
\tcase "480p":
\t\treturn 480
\tcase "360p":
\t\treturn 360
\tdefault:
\t\tif strings.HasSuffix(value, "p") {
\t\t\theight, err := strconv.Atoi(strings.TrimSuffix(value, "p"))
\t\t\tif err == nil && height > 0 {
\t\t\t\treturn height
\t\t\t}
\t\t}
\t\treturn 0
\t}
}
''',
)

lifecycle_path = "internal/service/task_lifecycle_events.go"
lifecycle = read(lifecycle_path)
dead_case = '''\tcase EventTranscodeCancelled:
\t\tupdate.Kind = TaskKindTranscode
\t\tupdate.Status = TaskStatusCancelled
'''
if dead_case not in lifecycle:
    raise RuntimeError("retired transcode cancelled event case missing")
write(lifecycle_path, lifecycle.replace(dead_case, ""))

main_path = "cmd/server/main.go"
main_source = read(main_path)
old_emby_call = "embyHandler := embyh.NewHandler(cfg, sugar, services.Auth, services.Stream, services.ArtifactMaintenance, repos)"
new_emby_call = "embyHandler := embyh.NewHandler(cfg, sugar, services.Auth, services.Stream, repos)"
if old_emby_call not in main_source:
    raise RuntimeError("full Emby constructor marker missing")
write(main_path, main_source.replace(old_emby_call, new_emby_call))

write(
    "internal/service/artifact_maintenance_test_helpers_test.go",
    '''package service

import (
\t"fmt"
\t"path/filepath"
\t"testing"

\t"github.com/glebarez/sqlite"
\t"github.com/nowen-video/nowen-video/internal/config"
\t"github.com/nowen-video/nowen-video/internal/model"
\t"github.com/nowen-video/nowen-video/internal/repository"
\ttranscodeartifactstore "github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
\t"go.uber.org/zap"
\t"gorm.io/gorm"
)

func newArtifactMaintenanceTestService(t *testing.T) (*ArtifactMaintenanceService, *gorm.DB) {
\tt.Helper()
\tcacheDir := t.TempDir()
\tdbPath := filepath.Join(t.TempDir(), "artifact-maintenance.db")
\tdsn := fmt.Sprintf("file:%s?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)", dbPath)
\tdb, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tsqlDB, err := db.DB()
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tsqlDB.SetMaxOpenConns(8)
\tsqlDB.SetMaxIdleConns(8)
\tif err := db.AutoMigrate(&model.Media{}, &model.TranscodeTask{}); err != nil {
\t\tt.Fatal(err)
\t}
\tif err := model.AutoMigrateTranscodeExecution(db); err != nil {
\t\tt.Fatal(err)
\t}
\tartifactStore, err := transcodeartifactstore.New(filepath.Join(cacheDir, "transcode"))
\tif err != nil {
\t\tt.Fatal(err)
\t}
\trepos := repository.NewRepositories(db)
\treturn &ArtifactMaintenanceService{
\t\trepo:          repos.Transcode,
\t\texecutionRepo: repository.NewTranscodeExecutionRepo(db),
\t\tartifactStore: artifactStore,
\t\tcfg: &config.Config{
\t\t\tCache: config.CacheConfig{CacheDir: cacheDir},
\t\t},
\t\tlogger:       zap.NewNop().Sugar(),
\t\tdiskUsageTTL: 0,
\t\tdone:         make(chan struct{}),
\t}, db
}
''',
)

for test_path in (
    "internal/service/transcode_cleanup_state_test.go",
    "internal/service/transcode_storage_health_test.go",
):
    source = read(test_path)
    write(test_path, source.replace("newConcurrentArtifactService(t)", "newArtifactMaintenanceTestService(t)"))

write(
    "internal/service/transcode_disk_pressure_test.go",
    '''package service

import (
\t"os"
\t"path/filepath"
\t"testing"
\t"time"

\t"github.com/nowen-video/nowen-video/internal/model"
\ttranscodediskpressure "github.com/nowen-video/nowen-video/internal/transcode/diskpressure"
\t"gorm.io/gorm"
)

func TestDiskPressureGovernorReclaimsOldPublishedArtifact(t *testing.T) {
\tservice, db := newArtifactMaintenanceTestService(t)
\tservice.cfg.Cache.MaxDiskUsageMB = 1
\tservice.diskUsageTTL = time.Nanosecond
\tartifact, path := createDiskPressureArtifact(t, service, db, "pressure-old", time.Now().Add(-48*time.Hour), 2*1024*1024)

\tstatus := service.runDiskPressureGovernorTick(time.Now(), true)
\tif status.LastReclaimedRows == 0 || status.LastReclaimedBytes < artifact.SizeBytes {
\t\tt.Fatalf("pressure reclaim evidence missing: %+v", status)
\t}
\tif _, err := os.Stat(path); !os.IsNotExist(err) {
\t\tt.Fatalf("artifact directory survived pressure reclaim: %v", err)
\t}
\tvar count int64
\tif err := db.Model(&model.TranscodeArtifactRecord{}).Where("id = ?", artifact.ID).Count(&count).Error; err != nil {
\t\tt.Fatal(err)
\t}
\tif count != 0 {
\t\tt.Fatalf("artifact metadata survived pressure cleanup: %d", count)
\t}
}

func TestDiskPressureGovernorProtectsRecentArtifact(t *testing.T) {
\tservice, db := newArtifactMaintenanceTestService(t)
\tservice.cfg.Cache.MaxDiskUsageMB = 1
\tservice.diskUsageTTL = time.Nanosecond
\tartifact, path := createDiskPressureArtifact(t, service, db, "pressure-active", time.Now(), 2*1024*1024)

\tstatus := service.runDiskPressureGovernorTick(time.Now(), true)
\tif status.Level == transcodediskpressure.LevelNormal || !status.AdmissionBlocked || !status.QueuePaused {
\t\tt.Fatalf("recent artifact should keep pressure visible: %+v", status)
\t}
\tif _, err := os.Stat(path); err != nil {
\t\tt.Fatalf("recently accessed artifact was removed: %v", err)
\t}
\tvar stored model.TranscodeArtifactRecord
\tif err := db.First(&stored, "id = ?", artifact.ID).Error; err != nil {
\t\tt.Fatal(err)
\t}
\tif stored.Status != "published" || stored.CleanupState != "" {
\t\tt.Fatalf("recent artifact entered cleanup: %+v", stored)
\t}
}

func createDiskPressureArtifact(
\tt *testing.T,
\tservice *ArtifactMaintenanceService,
\tdb *gorm.DB,
\tid string,
\tupdatedAt time.Time,
\tsize int,
) (*model.TranscodeArtifactRecord, string) {
\tt.Helper()
\tpath, err := service.artifactStore.PublishedDir("pressure-media", "720p", id)
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif err := os.MkdirAll(path, 0o755); err != nil {
\t\tt.Fatal(err)
\t}
\tpayload := make([]byte, size)
\tif err := os.WriteFile(filepath.Join(path, "seg00001.ts"), payload, 0o644); err != nil {
\t\tt.Fatal(err)
\t}
\tpublishedAt := time.Now().Add(-48 * time.Hour)
\tartifact := &model.TranscodeArtifactRecord{
\t\tID:                "artifact-" + id,
\t\tJobID:             "job-" + id,
\t\tMediaID:           "pressure-media",
\t\tKind:              "hls_variant",
\t\tProfileID:         "720p",
\t\tSourceFingerprint: "source",
\t\tPlannerVersion:    "planner",
\t\tStatus:            "published",
\t\tPath:              path,
\t\tSizeBytes:         int64(size),
\t\tPublishedAt:       &publishedAt,
\t\tCreatedAt:         publishedAt,
\t\tUpdatedAt:         updatedAt,
\t}
\tif err := db.Create(artifact).Error; err != nil {
\t\tt.Fatal(err)
\t}
\treturn artifact, path
}
''',
)
