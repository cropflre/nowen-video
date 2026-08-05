#!/usr/bin/env python3
from pathlib import Path

root = Path(__file__).resolve().parents[1]


def read(path: str) -> str:
    return (root / path).read_text(encoding="utf-8")


def write(path: str, content: str) -> None:
    (root / path).write_text(content, encoding="utf-8")


def replace_once(path: str, old: str, new: str, label: str) -> None:
    content = read(path)
    count = content.count(old)
    if count != 1:
        raise RuntimeError(f"{path} {label}: anchor count {count}, expected 1")
    write(path, content.replace(old, new, 1))


# Durable state separates cumulative failures from consecutive retry pressure,
# and gives completed generations an explicit next source-check timestamp.
replace_once(
    "internal/model/transcode_execution.go",
    '''\tFailureCount         int        `json:"failure_count"`
\tLastErrorCode        string     `json:"last_error_code" gorm:"type:text"`
''',
    '''\tFailureCount         int        `json:"failure_count"`
\tConsecutiveFailures  int        `json:"consecutive_failures"`
\tLastErrorCode        string     `json:"last_error_code" gorm:"type:text"`
''',
    "consecutive failure field",
)
replace_once(
    "internal/model/transcode_execution.go",
    '''\tNextAttemptAt        *time.Time `json:"next_attempt_at,omitempty" gorm:"index"`
\tLeaseOwner           string     `json:"lease_owner" gorm:"type:text"`
''',
    '''\tNextAttemptAt        *time.Time `json:"next_attempt_at,omitempty" gorm:"index"`
\tNextSourceCheckAt    *time.Time `json:"next_source_check_at,omitempty" gorm:"index"`
\tLeaseOwner           string     `json:"lease_owner" gorm:"type:text"`
''',
    "source check field",
)

# Repository state machine: source-check schedule, consecutive failures, and
# renewable Lease with expired-token fencing.
path = "internal/repository/repo_legacy_transcode_projection_migration.go"
content = read(path)
content = content.replace(
'''\tretirementWindow time.Duration,
) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {''',
'''\tretirementWindow time.Duration,
\tsourceCheckInterval time.Duration,
) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {''',
1,
)
content = content.replace(
'''\tvar state model.LegacyTranscodeProjectionMigrationState
\tchanged := false
''',
'''\tif sourceCheckInterval <= 0 {
\t\tsourceCheckInterval = 15 * time.Minute
\t}
\tvar state model.LegacyTranscodeProjectionMigrationState
\tchanged := false
''',
1,
)
content = content.replace(
'''\t\t\t\t\t"source_retire_after": retireAfter,
\t\t\t\t\t"next_attempt_at":     nil,
\t\t\t\t\t"updated_at":          now,
''',
'''\t\t\t\t\t"source_retire_after": retireAfter,
\t\t\t\t\t"next_attempt_at":     nil,
\t\t\t\t\t"next_source_check_at": now.Add(sourceCheckInterval),
\t\t\t\t\t"updated_at":          now,
''',
1,
)
content = content.replace(
'''\t\t\t\t"next_attempt_at":       now,
\t\t\t\t"completed_at":          nil,
''',
'''\t\t\t\t"next_attempt_at":       now,
\t\t\t\t"next_source_check_at":  nil,
\t\t\t\t"completed_at":          nil,
''',
1,
)
content = content.replace(
'''\t\t\t\t\t"next_attempt_at":       now,
\t\t\t\t\t"last_error_code":       "",
''',
'''\t\t\t\t\t"next_attempt_at":       now,
\t\t\t\t\t"next_source_check_at":  nil,
\t\t\t\t\t"consecutive_failures":  int64(0),
\t\t\t\t\t"last_error_code":       "",
''',
1,
)
old = '''\t\t\tif state.Status == LegacyProjectionMigrationCompleted && LegacyProjectionCursorAfter(*highWater, currentHigh) {
\t\t\t\tupdates = map[string]any{'''
if old not in content:
    raise RuntimeError("completed high-water branch missing")
# Schedule the next tail check when the completed generation has no newer row.
branch_end = '''\t\t\t\t}
\t\t\t}
\t\t}
\t\tif len(updates) > 0 {'''
replacement_end = '''\t\t\t\t}
\t\t\t} else if state.Status == LegacyProjectionMigrationCompleted {
\t\t\t\tupdates = map[string]any{
\t\t\t\t\t"next_source_check_at": now.Add(sourceCheckInterval),
\t\t\t\t\t"updated_at": now,
\t\t\t\t}
\t\t\t}
\t\t}
\t\tif len(updates) > 0 {'''
if content.count(branch_end) != 1:
    raise RuntimeError("completed no-change schedule anchor mismatch")
content = content.replace(branch_end, replacement_end, 1)

renew_anchor = '''func (r *TranscodeExecutionRepo) CompleteLegacyProjectionMigrationBatch('''
renew_method = '''func (r *TranscodeExecutionRepo) RenewLegacyProjectionMigrationLease(source, token string, now time.Time, leaseDuration time.Duration) (bool, error) {
\tif leaseDuration <= 0 {
\t\tleaseDuration = 2 * time.Minute
\t}
\tresult := r.db.Model(&model.LegacyTranscodeProjectionMigrationState{}).
\t\tWhere(
\t\t\t"source = ? AND status = ? AND lease_token = ? AND lease_expires_at IS NOT NULL AND lease_expires_at > ?",
\t\t\tsource,
\t\t\tLegacyProjectionMigrationRunning,
\t\t\ttoken,
\t\t\tnow,
\t\t).
\t\tUpdates(map[string]any{
\t\t\t"lease_expires_at": now.Add(leaseDuration),
\t\t\t"updated_at": now,
\t\t})
\treturn result.RowsAffected == 1, result.Error
}

'''
if renew_method not in content:
    if content.count(renew_anchor) != 1:
        raise RuntimeError("lease renew insertion anchor mismatch")
    content = content.replace(renew_anchor, renew_method + renew_anchor, 1)
content = content.replace(
'''\tcompleted bool, now time.Time, retirementWindow time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {''',
'''\tcompleted bool, now time.Time, retirementWindow, sourceCheckInterval time.Duration) (*model.LegacyTranscodeProjectionMigrationState, bool, error) {
\tif sourceCheckInterval <= 0 {
\t\tsourceCheckInterval = 15 * time.Minute
\t}''',
1,
)
content = content.replace(
'''\t\t"last_error_code":         "",
\t\t"last_error_message":      "",
''',
'''\t\t"consecutive_failures":   int64(0),
\t\t"last_error_code":         "",
\t\t"last_error_message":      "",
''',
1,
)
content = content.replace(
'''\t\tupdates["source_retire_after"] = now.Add(retirementWindow)
\t\tupdates["next_attempt_at"] = nil
''',
'''\t\tupdates["source_retire_after"] = now.Add(retirementWindow)
\t\tupdates["next_attempt_at"] = nil
\t\tupdates["next_source_check_at"] = now.Add(sourceCheckInterval)
''',
1,
)
content = content.replace(
'''\t\tupdates["source_retire_after"] = nil
\t\tupdates["next_attempt_at"] = now
''',
'''\t\tupdates["source_retire_after"] = nil
\t\tupdates["next_attempt_at"] = now
\t\tupdates["next_source_check_at"] = nil
''',
1,
)
content = content.replace(
'''\t\t\t"failure_count":           gorm.Expr("failure_count + 1"),
\t\t\t"last_error_code":         code,
''',
'''\t\t\t"failure_count":           gorm.Expr("failure_count + 1"),
\t\t\t"consecutive_failures":    gorm.Expr("consecutive_failures + 1"),
\t\t\t"last_error_code":         code,
''',
1,
)
content = content.replace(
'''\t\t\t"next_attempt_at":         nextAttemptAt,
\t\t\t"last_batch_completed_at": now,
''',
'''\t\t\t"next_attempt_at":         nextAttemptAt,
\t\t\t"next_source_check_at":    nil,
\t\t\t"last_batch_completed_at": now,
''',
1,
)
content = content.replace(
'''\t\t\t"last_error_message": "",
\t\t\t"lease_owner":        "",
''',
'''\t\t\t"last_error_message":    "",
\t\t\t"consecutive_failures":  int64(0),
\t\t\t"next_source_check_at":  nil,
\t\t\t"lease_owner":           "",
''',
1,
)
write(path, content)

# Service: fetch durable state first, freeze active generations, defer completed
# tail checks, count only when a new generation is actually required, and renew
# the Lease throughout filesystem inventory.
path = "internal/service/legacy_transcode_projection_migration.go"
content = read(path)
content = content.replace(
'''\tlegacyProjectionMigrationLease         = 2 * time.Minute
\tlegacyProjectionDefaultBatchSize       = 250
''',
'''\tlegacyProjectionMigrationLease         = 2 * time.Minute
\tlegacyProjectionLeaseHeartbeat         = 30 * time.Second
\tlegacyProjectionSourceCheckInterval    = 15 * time.Minute
\tlegacyProjectionDefaultBatchSize       = 250
''',
1,
)
old_snapshot = '''\thighWater, err := s.repo.LegacyProjectionHighWater()
\tif err != nil {
\t\treturn report, fmt.Errorf("read legacy projection high-water: %w", err)
\t}
\tvar targetRows int64
\tif highWater != nil {
\t\ttargetRows, err = s.repo.CountLegacyTerminalWithOutputThrough(*highWater)
\t\tif err != nil {
\t\t\treturn report, fmt.Errorf("count legacy projection target rows: %w", err)
\t\t}
\t}
\tstate, _, err := s.executionRepo.PrepareLegacyProjectionMigration(
\t\trepository.LegacyTranscodeArtifactMigrationSource,
\t\thighWater,
\t\ttargetRows,
\t\tbatchSize,
\t\tnow,
\t\tlegacyProjectionSourceRetirementWindow,
\t)
'''
new_snapshot = '''\tcurrentState, err := s.executionRepo.LegacyProjectionMigrationState(repository.LegacyTranscodeArtifactMigrationSource)
\tif err != nil {
\t\treturn report, fmt.Errorf("read legacy projection migration state: %w", err)
\t}
\tif legacyProjectionStateDeferred(currentState, now) {
\t\treturn legacyProjectionReportFromState(currentState), nil
\t}

\tvar highWater *repository.LegacyProjectionCursor
\ttargetRows := int64(0)
\tif currentState != nil {
\t\ttargetRows = currentState.TargetRows
\t}
\tif frozen := legacyProjectionFrozenHighWater(currentState); frozen != nil {
\t\thighWater = frozen
\t} else {
\t\thighWater, err = s.repo.LegacyProjectionHighWater()
\t\tif err != nil {
\t\t\treturn report, fmt.Errorf("read legacy projection high-water: %w", err)
\t\t}
\t\tif shouldRefreshLegacyProjectionTarget(currentState, highWater) {
\t\t\ttargetRows, err = s.repo.CountLegacyTerminalWithOutputThrough(*highWater)
\t\t\tif err != nil {
\t\t\t\treturn report, fmt.Errorf("count legacy projection target rows: %w", err)
\t\t\t}
\t\t}
\t}
\tstate, _, err := s.executionRepo.PrepareLegacyProjectionMigration(
\t\trepository.LegacyTranscodeArtifactMigrationSource,
\t\thighWater,
\t\ttargetRows,
\t\tbatchSize,
\t\tnow,
\t\tlegacyProjectionSourceRetirementWindow,
\t\tlegacyProjectionSourceCheckInterval,
\t)
'''
if content.count(old_snapshot) != 1:
    raise RuntimeError("service source snapshot anchor mismatch")
content = content.replace(old_snapshot, new_snapshot, 1)

claim_end = '''\tstate = claimed

\tafter := legacyProjectionCursor(state.CursorUpdatedAt, state.CursorID)
'''
claim_new = '''\tstate = claimed
\tcheckpoint := newLegacyProjectionLeaseCheckpoint(s.executionRepo, state.Source, token, now)

\tafter := legacyProjectionCursor(state.CursorUpdatedAt, state.CursorID)
'''
if content.count(claim_end) != 1:
    raise RuntimeError("checkpoint construction anchor mismatch")
content = content.replace(claim_end, claim_new, 1)
content = content.replace(
'''\t\titem, importErr := s.importLegacyProjectionTask(&tasks[index], now)
''',
'''\t\titem, importErr := s.importLegacyProjectionTask(&tasks[index], now, checkpoint)
''',
1,
)
content = content.replace(
'''\t\tlegacyProjectionSourceRetirementWindow,
\t)
''',
'''\t\tlegacyProjectionSourceRetirementWindow,
\t\tlegacyProjectionSourceCheckInterval,
\t)
''',
1,
)
content = content.replace(
'''\tbackoff := legacyProjectionRetryBackoff(state.FailureCount + 1)
''',
'''\tbackoff := legacyProjectionRetryBackoff(state.ConsecutiveFailures + 1)
''',
1,
)
helper_anchor = '''func legacyProjectionRetryBackoff(failureCount int) time.Duration {'''
helpers = '''func legacyProjectionStateDeferred(state *model.LegacyTranscodeProjectionMigrationState, now time.Time) bool {
\tif state == nil {
\t\treturn false
\t}
\tif state.Status == repository.LegacyProjectionMigrationFailed && state.NextAttemptAt != nil && now.Before(*state.NextAttemptAt) {
\t\treturn true
\t}
\treturn state.Status == repository.LegacyProjectionMigrationCompleted && state.NextSourceCheckAt != nil && now.Before(*state.NextSourceCheckAt)
}

func legacyProjectionFrozenHighWater(state *model.LegacyTranscodeProjectionMigrationState) *repository.LegacyProjectionCursor {
\tif state == nil || state.HighWaterUpdatedAt == nil || state.Status == repository.LegacyProjectionMigrationCompleted {
\t\treturn nil
\t}
\treturn &repository.LegacyProjectionCursor{UpdatedAt: *state.HighWaterUpdatedAt, ID: state.HighWaterID}
}

func shouldRefreshLegacyProjectionTarget(state *model.LegacyTranscodeProjectionMigrationState, highWater *repository.LegacyProjectionCursor) bool {
\tif highWater == nil {
\t\treturn false
\t}
\tif state == nil || state.HighWaterUpdatedAt == nil {
\t\treturn true
\t}
\tif state.Status != repository.LegacyProjectionMigrationCompleted {
\t\treturn false
\t}
\tcurrent := repository.LegacyProjectionCursor{UpdatedAt: *state.HighWaterUpdatedAt, ID: state.HighWaterID}
\treturn repository.LegacyProjectionCursorAfter(*highWater, current)
}

type legacyProjectionLeaseCheckpoint func(force bool) error

func newLegacyProjectionLeaseCheckpoint(repo *repository.TranscodeExecutionRepo, source, token string, claimedAt time.Time) legacyProjectionLeaseCheckpoint {
\tlastRenewed := claimedAt
\treturn func(force bool) error {
\t\tcurrent := time.Now()
\t\tif current.Before(lastRenewed) {
\t\t\tcurrent = lastRenewed
\t\t}
\t\tif !force && current.Sub(lastRenewed) < legacyProjectionLeaseHeartbeat {
\t\t\treturn nil
\t\t}
\t\trenewed, err := repo.RenewLegacyProjectionMigrationLease(source, token, current, legacyProjectionMigrationLease)
\t\tif err != nil {
\t\t\treturn fmt.Errorf("renew legacy migration Lease: %w", err)
\t\t}
\t\tif !renewed {
\t\t\treturn fmt.Errorf("legacy migration Lease expired or changed owner")
\t\t}
\t\tlastRenewed = current
\t\treturn nil
\t}
}

'''
if helpers not in content:
    if content.count(helper_anchor) != 1:
        raise RuntimeError("service helper insertion anchor mismatch")
    content = content.replace(helper_anchor, helpers + helper_anchor, 1)
content = content.replace(
'''func (s *ArtifactMaintenanceService) importLegacyProjectionTask(task *model.TranscodeTask, now time.Time) (legacyProjectionInventoryReport, error) {
\treport := legacyProjectionInventoryReport{TasksFound: 1}
''',
'''func (s *ArtifactMaintenanceService) importLegacyProjectionTask(task *model.TranscodeTask, now time.Time, checkpoint legacyProjectionLeaseCheckpoint) (legacyProjectionInventoryReport, error) {
\treport := legacyProjectionInventoryReport{TasksFound: 1}
''',
1,
)
content = content.replace(
'''\tif task == nil {
\t\treturn report, nil
\t}
\troot := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
''',
'''\tif task == nil {
\t\treturn report, nil
\t}
\tif checkpoint != nil {
\t\tif err := checkpoint(true); err != nil {
\t\t\treturn report, err
\t\t}
\t}
\troot := filepath.Join(s.cfg.Cache.CacheDir, "transcode")
''',
1,
)
content = content.replace(
'''\t\tartifact.SizeBytes, _ = directorySize(outputDir)
''',
'''\t\tartifact.SizeBytes, err = directorySizeWithCheckpoint(outputDir, func() error {
\t\t\tif checkpoint == nil {
\t\t\t\treturn nil
\t\t\t}
\t\t\treturn checkpoint(false)
\t\t})
\t\tif err != nil {
\t\t\treturn report, fmt.Errorf("inventory legacy directory size: %w", err)
\t\t}
''',
1,
)
content = content.replace(
'''\tif err := s.executionRepo.ImportLegacyHLSArtifact(artifact); err != nil {
''',
'''\tif checkpoint != nil {
\t\tif err := checkpoint(true); err != nil {
\t\t\treturn report, err
\t\t}
\t}
\tif err := s.executionRepo.ImportLegacyHLSArtifact(artifact); err != nil {
''',
1,
)
old_size = '''func directorySize(root string) (int64, error) {
\tvar total int64
\terr := filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
\t\tif walkErr != nil {
\t\t\treturn walkErr
\t\t}
\t\tif !info.IsDir() {
\t\t\ttotal += info.Size()
\t\t}
\t\treturn nil
\t})
\treturn total, err
}
'''
new_size = '''func directorySize(root string) (int64, error) {
\treturn directorySizeWithCheckpoint(root, nil)
}

func directorySizeWithCheckpoint(root string, checkpoint func() error) (int64, error) {
\tvar total int64
\terr := filepath.Walk(root, func(_ string, info os.FileInfo, walkErr error) error {
\t\tif walkErr != nil {
\t\t\treturn walkErr
\t\t}
\t\tif checkpoint != nil {
\t\t\tif err := checkpoint(); err != nil {
\t\t\t\treturn err
\t\t\t}
\t\t}
\t\tif !info.IsDir() {
\t\t\ttotal += info.Size()
\t\t}
\t\treturn nil
\t})
\treturn total, err
}
'''
if content.count(old_size) != 1:
    raise RuntimeError("directory size helper anchor mismatch")
content = content.replace(old_size, new_size, 1)
write(path, content)

# Repository tests cover Lease renewal fencing and new method signatures.
path = "internal/repository/repo_legacy_transcode_projection_migration_test.go"
content = read(path)
content = content.replace(', now, 30*24*time.Hour)', ', now, 30*24*time.Hour, 15*time.Minute)', 2)
content = content.replace(', true, now, 30*24*time.Hour)', ', true, now, 30*24*time.Hour, 15*time.Minute)', 1)
lease_test = '''
func TestLegacyProjectionLeaseRenewalRejectsExpiredOwner(t *testing.T) {
\t_, execution := newLegacyProjectionRepoTestDB(t)
\tnow := time.Now().UTC().Truncate(time.Millisecond)
\thigh := &LegacyProjectionCursor{UpdatedAt: now, ID: "a"}
\tstate, _, err := execution.PrepareLegacyProjectionMigration(LegacyTranscodeArtifactMigrationSource, high, 1, 10, now, 30*24*time.Hour, 15*time.Minute)
\tif err != nil {
\t\tt.Fatal(err)
\t}
\tif _, ok, err := execution.ClaimLegacyProjectionMigration(state.Source, "one", "token-one", now, time.Minute); err != nil || !ok {
\t\tt.Fatalf("claim one ok=%v err=%v", ok, err)
\t}
\tif ok, err := execution.RenewLegacyProjectionMigrationLease(state.Source, "token-one", now.Add(30*time.Second), time.Minute); err != nil || !ok {
\t\tt.Fatalf("renew current owner ok=%v err=%v", ok, err)
\t}
\tif _, ok, err := execution.ClaimLegacyProjectionMigration(state.Source, "two", "token-two", now.Add(2*time.Minute), time.Minute); err != nil || !ok {
\t\tt.Fatalf("takeover ok=%v err=%v", ok, err)
\t}
\tif ok, err := execution.RenewLegacyProjectionMigrationLease(state.Source, "token-one", now.Add(2*time.Minute), time.Minute); err != nil || ok {
\t\tt.Fatalf("expired owner renewed ok=%v err=%v", ok, err)
\t}
}
'''
if "TestLegacyProjectionLeaseRenewalRejectsExpiredOwner" not in content:
    content += lease_test
write(path, content)

# Service tests prove active generations use frozen high-water and completed
# generations do not refresh target counts until a newer source row exists.
path = "internal/service/legacy_transcode_projection_cursor_test.go"
content = read(path)
helper_test = '''
func TestLegacyProjectionSourceSnapshotPolicy(t *testing.T) {
\tnow := time.Now()
\thighAt := now.Add(time.Hour)
\tsame := &repository.LegacyProjectionCursor{UpdatedAt: now, ID: "a"}
\tnewer := &repository.LegacyProjectionCursor{UpdatedAt: highAt, ID: "b"}
\tstate := &model.LegacyTranscodeProjectionMigrationState{
\t\tStatus: repository.LegacyProjectionMigrationPending,
\t\tHighWaterUpdatedAt: &now,
\t\tHighWaterID: "a",
\t}
\tif shouldRefreshLegacyProjectionTarget(state, newer) {
\t\tt.Fatal("active generation must keep its frozen target")
\t}
\tif frozen := legacyProjectionFrozenHighWater(state); frozen == nil || frozen.ID != "a" {
\t\tt.Fatalf("frozen=%+v", frozen)
\t}
\tstate.Status = repository.LegacyProjectionMigrationCompleted
\tif shouldRefreshLegacyProjectionTarget(state, same) {
\t\tt.Fatal("completed generation refreshed unchanged target")
\t}
\tif !shouldRefreshLegacyProjectionTarget(state, newer) {
\t\tt.Fatal("completed generation did not detect newer source high-water")
\t}
\tnext := now.Add(time.Minute)
\tstate.NextSourceCheckAt = &next
\tif !legacyProjectionStateDeferred(state, now) {
\t\tt.Fatal("completed generation ignored source-check schedule")
\t}
}
'''
if "TestLegacyProjectionSourceSnapshotPolicy" not in content:
    content += helper_test
write(path, content)

# Runtime History exposes operational retry pressure and next tail check.
path = "internal/service/runtime_history.go"
content = read(path)
content = content.replace(
'''\tFailureCount        int        `json:"failure_count"`
\tLastErrorCode       string     `json:"last_error_code,omitempty"`
''',
'''\tFailureCount        int        `json:"failure_count"`
\tConsecutiveFailures int        `json:"consecutive_failures"`
\tLastErrorCode       string     `json:"last_error_code,omitempty"`
''',
1,
)
content = content.replace(
'''\tSourceRetireAfter   *time.Time `json:"source_retire_after,omitempty"`
\tRetirementEligible  bool       `json:"retirement_eligible"`
''',
'''\tSourceRetireAfter   *time.Time `json:"source_retire_after,omitempty"`
\tNextSourceCheckAt   *time.Time `json:"next_source_check_at,omitempty"`
\tRetirementEligible  bool       `json:"retirement_eligible"`
''',
1,
)
content = content.replace(
'''\t\t\tFailureCount: migration.FailureCount, LastErrorCode: migration.LastErrorCode,
''',
'''\t\t\tFailureCount: migration.FailureCount, ConsecutiveFailures: migration.ConsecutiveFailures,
\t\t\tLastErrorCode: migration.LastErrorCode,
''',
1,
)
content = content.replace(
'''\t\t\tCompletedAt: migration.CompletedAt, SourceRetireAfter: migration.SourceRetireAfter,
''',
'''\t\t\tCompletedAt: migration.CompletedAt, SourceRetireAfter: migration.SourceRetireAfter,
\t\t\tNextSourceCheckAt: migration.NextSourceCheckAt,
''',
1,
)
write(path, content)

path = "web/src/api/runtimeHistory.ts"
content = read(path)
content = content.replace(
'''  failure_count: number
  last_error_code?: string
''',
'''  failure_count: number
  consecutive_failures: number
  last_error_code?: string
''',
1,
)
content = content.replace(
'''  source_retire_after?: string
  retirement_eligible: boolean
''',
'''  source_retire_after?: string
  next_source_check_at?: string
  retirement_eligible: boolean
''',
1,
)
write(path, content)

# Task Center completed message includes the next bounded source check.
path = "internal/service/task_center.go"
content = read(path)
content = content.replace(
'''\t\tif now.Before(*state.SourceRetireAfter) {
\t\t\tmessage = "迁移完成；旧表只读观察至 " + state.SourceRetireAfter.Format("2006-01-02 15:04")
''',
'''\t\tif now.Before(*state.SourceRetireAfter) {
\t\t\tmessage = "迁移完成；旧表只读观察至 " + state.SourceRetireAfter.Format("2006-01-02 15:04")
\t\t\tif state.NextSourceCheckAt != nil {
\t\t\t\tmessage += "；下次尾部检查 " + state.NextSourceCheckAt.Format("01-02 15:04")
\t\t\t}
''',
1,
)
write(path, content)

# Regression test requires both renewable Lease and bounded source polling.
path = "cmd/server/legacy_projection_cursor_test.go"
content = read(path)
content = content.replace(
'''"ClaimLegacyProjectionMigration", "CompleteLegacyProjectionMigrationBatch", "RetryLegacyProjectionMigration"''',
'''"ClaimLegacyProjectionMigration", "RenewLegacyProjectionMigrationLease", "CompleteLegacyProjectionMigrationBatch", "RetryLegacyProjectionMigration"''',
1,
)
content = content.replace(
'''"ListLegacyTerminalWithOutputAfter", "legacyProjectionSourceRetirementWindow", "legacyProjectionMigrationLease"''',
'''"ListLegacyTerminalWithOutputAfter", "legacyProjectionSourceRetirementWindow", "legacyProjectionSourceCheckInterval", "legacyProjectionMigrationLease"''',
1,
)
write(path, content)

# Document the final operational semantics.
path = "docs/LEGACY_TRANSCODE_PROJECTION_RETIREMENT.md"
content = read(path)
addition = '''

A running or failed generation uses only its frozen high-water and does not query
the legacy source again. After completion, the maintenance loop reads only the
durable state row until a 15-minute source-check deadline is reached. A new
source count is calculated only when the tail high-water has actually advanced.

The migration Lease is renewed during directory traversal and is checked again
immediately before Artifact insertion. An expired owner cannot renew or commit a
batch after another instance takes over. Retry delay is based on consecutive
failures, while the cumulative failure count remains audit evidence.
'''
if "A running or failed generation uses only its frozen high-water" not in content:
    content += addition
write(path, content)
