#!/usr/bin/env python3
from __future__ import annotations

import argparse
import pathlib
import re
import sys

ROOT = pathlib.Path(__file__).resolve().parents[1]

SYMBOLS = (
    "TranscodeService",
    "ArtifactMaintenanceService",
    "TranscodeJob",
    "transcodePriorityQueue",
    "recoverPendingTasks",
    "leaseRecoveryLoop",
    "StartTranscode",
    "NewTranscodeService",
    "NewArtifactMaintenanceService",
    "ExecutionRuntime",
    "GetHWAccelInfo",
)

DELETE_FILES = (
    "internal/service/transcode.go",
    "internal/service/transcode_artifact_recovery.go",
    "internal/service/transcode_artifact_version.go",
    "internal/service/transcode_artifacts.go",
    "internal/service/transcode_attestation.go",
    "internal/service/transcode_concurrent_artifact_test.go",
    "internal/service/transcode_intent_plan.go",
    "internal/service/transcode_intent_plan_test.go",
    "internal/service/transcode_lease.go",
    "internal/service/transcode_persistence.go",
    "internal/service/transcode_probe.go",
    "internal/service/transcode_probe_warmup_events.go",
    "internal/service/transcode_process_shutdown.go",
    "internal/service/transcode_profile_catalog_test.go",
    "internal/service/transcode_progress.go",
    "internal/service/transcode_quality.go",
    "internal/service/transcode_quality_probe_test.go",
    "internal/service/transcode_queue.go",
    "internal/service/transcode_queue_retired_test.go",
    "internal/service/transcode_queue_test.go",
    "internal/service/transcode_runtime_creation_retired_test.go",
    "internal/service/transcode_runtime_test.go",
    "internal/service/transcode_shutdown.go",
    "internal/service/transcode_startup.go",
    "internal/service/transcode_startup_continuation.go",
    "internal/service/transcode_startup_test.go",
    "internal/service/transcode_storage_observation.go",
    "internal/service/transcode_storage_observation_test.go",
    "internal/service/transcode_storage_reservation.go",
    "internal/service/transcode_storage_reservation_test.go",
    "internal/service/transcode_task_actions.go",
    "internal/service/transcode_throttle.go",
    "internal/service/transcode_timestamp_plan.go",
    "internal/service/transcode_timestamp_plan_test.go",
    "internal/service/throttle_unix.go",
    "internal/service/throttle_windows.go",
)

MAINTENANCE_FILES = (
    "internal/service/transcode_cleanup.go",
    "internal/service/transcode_cleanup_admin.go",
    "internal/service/transcode_cleanup_state.go",
    "internal/service/transcode_disk_pressure.go",
    "internal/service/transcode_runtime_retirement.go",
    "internal/service/transcode_stats.go",
    "internal/service/transcode_storage_fault_reporting.go",
    "internal/service/transcode_storage_health.go",
)

ARTIFACT_MAINTENANCE = r'''package service

import (
    "context"
    "fmt"
    "path/filepath"
    "sync"
    "time"

    "github.com/nowen-video/nowen-video/internal/config"
    "github.com/nowen-video/nowen-video/internal/model"
    "github.com/nowen-video/nowen-video/internal/repository"
    transcodeartifactstore "github.com/nowen-video/nowen-video/internal/transcode/artifactstore"
    "go.uber.org/zap"
)

const artifactMaintenanceInterval = 30 * time.Second

// ArtifactMaintenanceService owns only historical Runtime retirement, durable
// Artifact cleanup and storage-health evidence. It cannot submit, claim or run
// media work and contains no FFmpeg runtime, worker queue or playback state.
type ArtifactMaintenanceService struct {
    repo          *repository.TranscodeRepo
    executionRepo *repository.TranscodeExecutionRepo
    cfg           *config.Config
    logger        *zap.SugaredLogger
    wsHub         *WSHub
    artifactStore *transcodeartifactstore.Store

    diskUsageMu    sync.RWMutex
    diskUsageBytes int64
    diskUsageAt    time.Time
    diskUsageTTL   time.Duration

    done         chan struct{}
    shutdownOnce sync.Once
    wg           sync.WaitGroup
}

func NewArtifactMaintenanceService(repo *repository.TranscodeRepo, cfg *config.Config, logger *zap.SugaredLogger) *ArtifactMaintenanceService {
    if repo == nil || repo.DB() == nil {
        panic("transcode repository is required")
    }
    if cfg == nil {
        panic("configuration is required")
    }
    if logger == nil {
        logger = zap.NewNop().Sugar()
    }
    if err := model.AutoMigrateTranscodeExecution(repo.DB()); err != nil {
        panic(fmt.Sprintf("migrate transcode execution schema: %v", err))
    }
    artifactStore, err := transcodeartifactstore.New(filepath.Join(cfg.Cache.CacheDir, "transcode"))
    if err != nil {
        panic(fmt.Sprintf("initialize transcode artifact store: %v", err))
    }
    service := &ArtifactMaintenanceService{
        repo:          repo,
        executionRepo: repository.NewTranscodeExecutionRepo(repo.DB()),
        cfg:           cfg,
        logger:        logger,
        artifactStore: artifactStore,
        diskUsageTTL:  30 * time.Second,
        done:          make(chan struct{}),
    }
    if err := service.initializeStorageHealth(); err != nil {
        panic(fmt.Sprintf("initialize artifact storage health: %v", err))
    }
    if report, retireErr := service.retirePersistentRuntimePlayback(time.Now()); retireErr != nil {
        logger.Warnf("启动退役持久 Runtime 播放状态失败: %v", retireErr)
    } else if report.Changed() {
        logger.Infof("启动退役持久 Runtime 播放状态 cancelled=%d artifacts=%d attempts=%d tasks=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.TasksRetired, report.PathsRemoved)
    }
    service.runDiskPressureGovernorTick(time.Now(), true)
    service.wg.Add(1)
    go service.maintenanceLoop()
    return service
}

func (s *ArtifactMaintenanceService) SetWSHub(hub *WSHub) {
    if s != nil {
        s.wsHub = hub
    }
}

func (s *ArtifactMaintenanceService) maintenanceLoop() {
    defer s.wg.Done()
    ticker := time.NewTicker(artifactMaintenanceInterval)
    defer ticker.Stop()
    for {
        select {
        case now := <-ticker.C:
            s.runStorageHealthTick(now, true)
            s.runDiskPressureGovernorTick(now, false)
            report, err := s.retirePersistentRuntimePlayback(now)
            if err != nil {
                s.logger.Warnf("周期退役持久 Runtime 播放状态失败: %v", err)
            } else if report.Changed() {
                s.logger.Infof("周期退役持久 Runtime 播放状态 cancelled=%d artifacts=%d attempts=%d tasks=%d paths=%d", report.JobsCancelled, report.ArtifactsDeleted, report.AttemptsRetired, report.TasksRetired, report.PathsRemoved)
            }
        case <-s.done:
            return
        }
    }
}

func (s *ArtifactMaintenanceService) Shutdown(ctx context.Context) error {
    if s == nil {
        return nil
    }
    s.shutdownOnce.Do(func() { close(s.done) })
    complete := make(chan struct{})
    go func() {
        s.wg.Wait()
        close(complete)
    }()
    select {
    case <-complete:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}

func (s *ArtifactMaintenanceService) legacyOutputDir(mediaID, quality string) string {
    if s == nil || s.cfg == nil {
        return ""
    }
    return filepath.Join(s.cfg.Cache.CacheDir, "transcode", mediaID, quality)
}
'''

TRANSCODE_STATS = r'''package service

import (
    "os"
    "path/filepath"
    "time"
)

// TranscodeStatistics is retained as the wire shape for existing diagnostics,
// but all execution counters are permanently zero because Runtime workers no
// longer exist. The live fields describe Artifact maintenance only.
type TranscodeStatistics struct {
    StatusCounts               map[string]int64             `json:"status_counts"`
    ArtifactStatusCounts       map[string]int64             `json:"artifact_status_counts"`
    ArtifactCleanupStateCounts map[string]int64             `json:"artifact_cleanup_state_counts"`
    RunningCount               int                          `json:"running_count"`
    ActiveWorkers              int                          `json:"active_workers"`
    MaxWorkers                 int                          `json:"max_workers"`
    QueueDepth                 int                          `json:"queue_depth"`
    DurableQueueDepth          int64                        `json:"durable_queue_depth"`
    Scheduler                  string                       `json:"scheduler"`
    QueuePollMS                int64                        `json:"queue_poll_ms"`
    LeaseDurationSeconds       int64                        `json:"lease_duration_seconds"`
    HWAccel                    string                       `json:"hw_accel"`
    DiskUsageBytes             int64                        `json:"disk_usage_bytes"`
    DiskUsageDir               string                       `json:"disk_usage_dir"`
    ArtifactStoreRoot          string                       `json:"artifact_store_root"`
    StorageHealth              TranscodeStorageHealthStatus `json:"storage_health"`
    DiskPressure               TranscodeDiskPressureStatus  `json:"disk_pressure"`
}

func (s *ArtifactMaintenanceService) GetStatistics() TranscodeStatistics {
    counts, _ := s.repo.CountByStatus()
    if counts == nil {
        counts = map[string]int64{}
    }
    artifactCounts, _ := s.executionRepo.ArtifactStatusCounts()
    if artifactCounts == nil {
        artifactCounts = map[string]int64{}
    }
    cleanupCounts, _ := s.executionRepo.ArtifactCleanupStateCounts()
    if cleanupCounts == nil {
        cleanupCounts = map[string]int64{}
    }
    artifactRoot := ""
    if s.artifactStore != nil {
        artifactRoot = s.artifactStore.Root()
    }
    return TranscodeStatistics{
        StatusCounts:               counts,
        ArtifactStatusCounts:       artifactCounts,
        ArtifactCleanupStateCounts: cleanupCounts,
        Scheduler:                  "artifact_maintenance_only",
        HWAccel:                    "none",
        DiskUsageBytes:             s.GetCacheDiskUsage(),
        DiskUsageDir:               filepath.Join(s.cfg.Cache.CacheDir, "transcode"),
        ArtifactStoreRoot:          artifactRoot,
        StorageHealth:              s.GetStorageHealthStatus(),
        DiskPressure:               s.GetDiskPressureStatus(),
    }
}

func (s *ArtifactMaintenanceService) GetCacheDiskUsage() int64 {
    ttl := s.diskUsageTTL
    if ttl <= 0 {
        ttl = 30 * time.Second
    }
    s.diskUsageMu.RLock()
    if !s.diskUsageAt.IsZero() && time.Since(s.diskUsageAt) < ttl {
        value := s.diskUsageBytes
        s.diskUsageMu.RUnlock()
        return value
    }
    s.diskUsageMu.RUnlock()

    dir := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
    var total int64
    if info, err := os.Stat(dir); err == nil && info.IsDir() {
        _ = filepath.Walk(dir, func(_ string, fileInfo os.FileInfo, walkErr error) error {
            if walkErr == nil && fileInfo != nil && !fileInfo.IsDir() {
                total += fileInfo.Size()
            }
            return nil
        })
    }
    s.diskUsageMu.Lock()
    s.diskUsageBytes = total
    s.diskUsageAt = time.Now()
    s.diskUsageMu.Unlock()
    return total
}

func (s *ArtifactMaintenanceService) InvalidateCacheDiskUsage() {
    s.diskUsageMu.Lock()
    s.diskUsageAt = time.Time{}
    s.diskUsageMu.Unlock()
}
'''


def go_files() -> list[pathlib.Path]:
    return [path for path in ROOT.rglob("*.go") if "vendor" not in path.parts and ".git" not in path.parts]


def read(path: str) -> str:
    return (ROOT / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    target = ROOT / path
    target.parent.mkdir(parents=True, exist_ok=True)
    target.write_text(content, encoding="utf-8")


def replace(path: str, old: str, new: str, required: bool = True) -> None:
    content = read(path)
    if old not in content:
        if required:
            raise RuntimeError(f"missing replacement marker in {path}: {old!r}")
        return
    write(path, content.replace(old, new))


def remove_function(content: str, function_name: str) -> str:
    match = re.search(rf"(?m)^func (?:\([^\n]+\) )?{re.escape(function_name)}\([^\n]*\)(?: [^{{\n]+)? \{{", content)
    if not match:
        return content
    start = match.start()
    brace = content.find("{", match.start(), match.end())
    depth = 0
    index = brace
    while index < len(content):
        if content[index] == "{":
            depth += 1
        elif content[index] == "}":
            depth -= 1
            if depth == 0:
                end = index + 1
                while end < len(content) and content[end] == "\n":
                    end += 1
                return content[:start] + content[end:]
        index += 1
    raise RuntimeError(f"unterminated function {function_name}")


def audit() -> None:
    files = sorted(go_files())
    print("=== artifact maintenance refactor audit ===")
    for symbol in SYMBOLS:
        print(f"\n--- {symbol} ---")
        matches = 0
        for path in files:
            for lineno, line in enumerate(path.read_text(encoding="utf-8").splitlines(), 1):
                if symbol in line:
                    print(f"{path.relative_to(ROOT)}:{lineno}:{line}")
                    matches += 1
        if matches == 0:
            print("<no matches>")


def refactor() -> None:
    write("internal/service/artifact_maintenance.go", ARTIFACT_MAINTENANCE)
    for path in DELETE_FILES:
        target = ROOT / path
        if target.exists():
            target.unlink()

    for path in MAINTENANCE_FILES:
        content = read(path).replace("*TranscodeService", "*ArtifactMaintenanceService")
        write(path, content)

    cleanup = read("internal/service/transcode_cleanup.go")
    cleanup = re.sub(r"\n\t\ts\.mu\.RLock\(\)\n\t\t_, active := s\.running\[task\.ID\]\n\t\ts\.mu\.RUnlock\(\)\n\t\tif active \{\n\t\t\tcontinue\n\t\t\}\n", "\n", cleanup)
    cleanup = cleanup.replace("s.GetLegacyOutputDir(task.MediaID, task.Quality)", "s.legacyOutputDir(task.MediaID, task.Quality)")
    write("internal/service/transcode_cleanup.go", cleanup)

    disk_pressure = read("internal/service/transcode_disk_pressure.go")
    disk_pressure = disk_pressure.replace('    "errors"\n', '').replace('\t"errors"\n', '')
    disk_pressure = re.sub(r"\nvar ErrTranscodeStoragePressure = errors\.New\([^\n]+\)\n", "\n", disk_pressure)
    disk_pressure = disk_pressure.replace("var transcodeDiskPressureOwners sync.Map\n", "")
    for function in (
        "initializeDiskPressureGovernor",
        "transcodePressureOwner",
        "transcodeQueueAdmissionError",
        "transcodeQueueClaimAllowed",
        "checkDiskPressureAdmission",
    ):
        disk_pressure = remove_function(disk_pressure, function)
    old_loop = '''func (s *ArtifactMaintenanceService) artifactAccessTouchLoop(state *transcodeDiskPressureState) {
\tlastWrite := make(map[string]time.Time)
\tfor touch := range state.touches {
'''
    new_loop = '''func (s *ArtifactMaintenanceService) artifactAccessTouchLoop(state *transcodeDiskPressureState) {
\tlastWrite := make(map[string]time.Time)
\tfor {
\t\tvar touch artifactAccessTouch
\t\tselect {
\t\tcase touch = <-state.touches:
\t\tcase <-s.done:
\t\t\treturn
\t\t}
'''
    if old_loop not in disk_pressure:
        raise RuntimeError("disk pressure touch loop marker missing")
    disk_pressure = disk_pressure.replace(old_loop, new_loop)
    write("internal/service/transcode_disk_pressure.go", disk_pressure)

    storage_health = read("internal/service/transcode_storage_health.go")
    storage_health = storage_health.replace("\tstartOnce      sync.Once\n", "")
    storage_health = storage_health.replace(
        "\tstate := s.storageHealthState()\n\ts.runStorageHealthTick(time.Now(), true)\n\tstate.startOnce.Do(func() { go s.storageHealthLoop() })\n\treturn nil\n",
        "\ts.runStorageHealthTick(time.Now(), true)\n\treturn nil\n",
    )
    storage_health = remove_function(storage_health, "storageHealthLoop")
    write("internal/service/transcode_storage_health.go", storage_health)

    retirement = read("internal/service/transcode_runtime_retirement.go")
    retirement = retirement.replace(
        'transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"\n',
        'transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"\n\ttranscodeprofile "github.com/nowen-video/nowen-video/internal/transcode/profile"\n',
    )
    retirement = retirement.replace("for quality := range qualityPresets {", "for _, quality := range transcodeprofile.Names() {")
    write("internal/service/transcode_runtime_retirement.go", retirement)

    write("internal/service/transcode_stats.go", TRANSCODE_STATS)

    for path in (
        "internal/service/transcode_disk_pressure_test.go",
        "internal/service/transcode_runtime_retirement_test.go",
    ):
        write(path, read(path).replace("TranscodeService", "ArtifactMaintenanceService"))

    service_go = read("internal/service/service.go")
    service_go = re.sub(r"\n\tTranscode\s+\*ArtifactMaintenanceService\s+// deprecated alias", "", service_go)
    service_go = re.sub(r"\n\s*Transcode:\s+artifactMaintenance,", "", service_go)
    service_go = service_go.replace("registerFullTranscodeProcessShutdown(artifactMaintenance, logger)\n", "")
    service_go = service_go.replace("使用 TranscodeService 检测后的实际硬件加速模式", "使用 MediaExecutionService 检测后的实际硬件加速模式")
    write("internal/service/service.go", service_go)

    lite_go = read("internal/service/lite.go")
    lite_go = re.sub(r"\n\s*Transcode:\s+artifactMaintenance,", "", lite_go)
    lite_go = lite_go.replace("registerFullTranscodeProcessShutdown(artifactMaintenance, logger)\n", "")
    write("internal/service/lite.go", lite_go)

    for path in ("cmd/server-lite/main.go",):
        content = read(path).replace("TranscodeService is now a compatibility and migration-maintenance shell.", "ArtifactMaintenanceService owns migration and cleanup only.")
        content = content.replace("services.Transcode", "services.ArtifactMaintenance")
        write(path, content)

    for path in go_files():
        rel = str(path.relative_to(ROOT))
        if rel.startswith("internal/service/") and rel in DELETE_FILES:
            continue
        content = path.read_text(encoding="utf-8")
        updated = content.replace("services.Transcode", "services.ArtifactMaintenance")
        if updated != content:
            path.write_text(updated, encoding="utf-8")

    emby_handler = read("internal/handler/emby/handler.go")
    emby_handler = emby_handler.replace("\ttranscode *service.TranscodeService // 用于节流（SetPlaybackPosition）\n", "")
    emby_handler = emby_handler.replace("\ttranscode *service.TranscodeService,\n", "")
    emby_handler = emby_handler.replace("\t\ttranscode:       transcode,\n", "")
    write("internal/handler/emby/handler.go", emby_handler)

    sessions = read("internal/handler/emby/sessions.go")
    sessions = re.sub(r"\n\t// Compatibility fallback for direct/remux playback and older servers\.\n\tif h\.transcode != nil && !isStop \{\n\t\th\.transcode\.SetPlaybackPosition\(uuid, position\)\n\t\}\n", "\n", sessions)
    sessions = re.sub(r"\n\tif h\.transcode != nil \{\n\t\th\.transcode\.SetPlaybackPosition\(uuid, position\)\n\t\}\n", "\n", sessions)
    write("internal/handler/emby/sessions.go", sessions)

    task_actions = read("internal/service/task_actions.go")
    task_actions = task_actions.replace("transcode *TranscodeService,", "maintenance *ArtifactMaintenanceService,")
    task_actions = task_actions.replace("\t\ttranscode:       transcode,\n", "")
    task_actions = task_actions.replace("\tif transcode != nil {\n\t\tdispatcher.artifactCleanup = transcode\n\t}\n", "\tif maintenance != nil {\n\t\tdispatcher.artifactCleanup = maintenance\n\t}\n")
    write("internal/service/task_actions.go", task_actions)

    task_retirement = read("internal/service/task_center_runtime_retirement.go")
    task_retirement = task_retirement.replace("maintenance *TranscodeService", "maintenance *ArtifactMaintenanceService")
    write("internal/service/task_center_runtime_retirement.go", task_retirement)

    gate = read("cmd/server/runtime_worker_removed_test.go")
    gate = gate.replace('requireSource(t, "../../internal/service/transcode.go", "type ArtifactMaintenanceService = TranscodeService")', 'requireSource(t, "../../internal/service/artifact_maintenance.go", "type ArtifactMaintenanceService struct")')
    gate = gate.replace('requireSource(t, "../../internal/service/transcode.go", "NewArtifactMaintenanceService")', 'requireSource(t, "../../internal/service/artifact_maintenance.go", "NewArtifactMaintenanceService")')
    gate = gate.replace(
        'for _, check := range []struct{ path, marker string }{',
        'for _, removed := range []string{"../../internal/service/transcode.go", "../../internal/service/transcode_queue.go", "../../internal/service/transcode_lease.go", "../../internal/service/transcode_progress.go", "../../internal/service/transcode_throttle.go", "../../internal/service/transcode_persistence.go", "../../internal/service/transcode_process_shutdown.go"} {\n\t\tif _, err := os.Stat(removed); !os.IsNotExist(err) {\n\t\t\tt.Fatalf("retired runtime file still exists: %s", removed)\n\t\t}\n\t}\n\n\tfor _, check := range []struct{ path, marker string }{',
    )
    gate = gate.replace('{"../../internal/service/transcode.go", "go service.worker"},', '{"../../internal/service/artifact_maintenance.go", "TranscodeService"},\n\t\t{"../../internal/service/artifact_maintenance.go", "TranscodeJob"},\n\t\t{"../../internal/service/service.go", "Transcode           *ArtifactMaintenanceService"},')
    write("cmd/server/runtime_worker_removed_test.go", gate)

    docs = read("docs/MEDIA_EXECUTION_BOUNDARIES.md")
    docs = docs.replace("`NewTranscodeService` remains temporarily\nas a source-compatible constructor alias, but returns the maintenance-only\nservice.\n\nCompatibility submission methods fail closed with\n`ErrPersistentRuntimeTranscodeRetired`. No production constructor starts the\nlegacy `worker` loop. Dead execution helpers remain isolated for the next\nphysical source-deletion phase and are unreachable from Lite or Full assembly.", "There is no `TranscodeService` alias, Runtime Job value, queue, Lease loop,\nworker, attempt runner, throttle controller or submission API in the service\nassembly. Historical database records remain data only and are consumed by the\nretirement and cleanup sweepers.")
    docs = docs.replace("Compatibility methods and routes may remain during the migration window, but\nthey fail closed and cannot create Jobs, Claim Leases, read Runtime Artifacts,\nor start FFmpeg outside a Playback Session, managed remux request, or explicit\npreprocessing task.", "Compatibility routes may remain during the migration window, but they resolve\nonly to authenticated `410 Gone` tombstones. No service compatibility method\ncan create a Job, Claim a Lease, read a Runtime Artifact or start FFmpeg.")
    write("docs/MEDIA_EXECUTION_BOUNDARIES.md", docs)

    # Constructor calls no longer pass the removed Emby transcode dependency.
    for path in go_files():
        content = path.read_text(encoding="utf-8")
        content = re.sub(r"(emby\.NewHandler\([\s\S]*?services\.Stream,\n)\s*services\.ArtifactMaintenance,\n", r"\1", content)
        path.write_text(content, encoding="utf-8")


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--audit", action="store_true")
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    if args.audit:
        audit()
        return 0
    if args.apply:
        refactor()
        return 0
    print("choose --audit or --apply", file=sys.stderr)
    return 2


if __name__ == "__main__":
    raise SystemExit(main())
