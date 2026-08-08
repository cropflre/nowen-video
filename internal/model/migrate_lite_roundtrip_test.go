package model

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func openProfileDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite profile database: %v", err)
	}
	return db
}

func closeProfileDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite connection: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatalf("checkpoint sqlite database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sqlite database: %v", err)
	}
}

func migrateFullProfile(db *gorm.DB) error {
	if err := AutoMigrate(db); err != nil {
		return fmt.Errorf("full profile: %w", err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		return fmt.Errorf("transcode execution: %w", err)
	}
	if err := AutoMigrateTranscodeStorageReservation(db); err != nil {
		return fmt.Errorf("storage reservation: %w", err)
	}
	if err := AutoMigrateTranscodeStorageIncidents(db); err != nil {
		return fmt.Errorf("storage incidents: %w", err)
	}
	return nil
}

func migrateLiteProfile(db *gorm.DB) error {
	if err := AutoMigrateLite(db, false); err != nil {
		return fmt.Errorf("lite profile: %w", err)
	}
	// Lite creates these through the transcode runtime after the core profile
	// migration. Include them here so the certification matches real startup.
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		return fmt.Errorf("transcode execution: %w", err)
	}
	if err := AutoMigrateTranscodeStorageReservation(db); err != nil {
		return fmt.Errorf("storage reservation: %w", err)
	}
	if err := AutoMigrateTranscodeStorageIncidents(db); err != nil {
		return fmt.Errorf("storage incidents: %w", err)
	}
	return nil
}

func mustCreateProfileRow(t *testing.T, db *gorm.DB, name string, value any) {
	t.Helper()
	if err := db.Create(value).Error; err != nil {
		t.Fatalf("seed %s: %v", name, err)
	}
}

func seedFullProfile(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	user := &User{
		ID: "legacy-user", Username: "legacy-admin", Password: "legacy-password-hash",
		Role: "admin", Nickname: "Legacy Admin", CreatedAt: now, UpdatedAt: now,
	}
	library := &Library{
		ID: "legacy-library", Name: "Legacy Movies", Path: "/legacy/movies",
		Type: "movie", CreatedAt: now, UpdatedAt: now,
	}
	collection := &MovieCollection{
		ID: "legacy-collection", Name: "Legacy Collection", MediaCount: 1,
		FileCount: 1, CreatedAt: now, UpdatedAt: now,
	}
	series := &Series{
		ID: "legacy-series", LibraryID: library.ID, Title: "Legacy Series",
		FolderPath: "/legacy/series", CreatedAt: now, UpdatedAt: now,
	}
	media := &Media{
		ID: "legacy-media", LibraryID: library.ID, Title: "Legacy Movie",
		FilePath: "/legacy/movies/movie.mkv", FileSize: 8 * 1024 * 1024 * 1024,
		MediaType: "movie", VideoCodec: "hevc", AudioCodec: "aac",
		Resolution: "4K", Duration: 7200, CollectionID: collection.ID,
		SeriesID: series.ID, CreatedAt: now, UpdatedAt: now,
	}
	history := &WatchHistory{
		ID: "legacy-history", UserID: user.ID, MediaID: media.ID,
		Position: 1234.5, Duration: media.Duration, CreatedAt: now, UpdatedAt: now,
	}
	preprocess := &PreprocessTask{
		ID: "legacy-preprocess", MediaID: media.ID, Status: "completed",
		Phase: "done", Progress: 100, MediaTitle: media.Title, CreatedAt: now, UpdatedAt: now,
	}
	subtitle := &SubtitlePreprocessTask{
		ID: "legacy-subtitle-preprocess", MediaID: media.ID, Status: "completed",
		Phase: "done", Progress: 100, MediaTitle: media.Title, CreatedAt: now, UpdatedAt: now,
	}
	activeKey := "legacy-media|runtime_hls|1080p"
	job := &TranscodeJobRecord{
		ID: "legacy-transcode-job", MediaID: media.ID, Intent: "runtime_hls",
		ProfileID: "1080p", Status: "queued", DesiredState: "running",
		ActiveKey: &activeKey, SourceFingerprint: "legacy-source-fingerprint",
		PlannerVersion: "runtime-hls-v2", CreatedAt: now, UpdatedAt: now,
	}
	reservation := &TranscodeStorageReservationRecord{
		JobID: job.ID, MediaID: media.ID, ProfileID: job.ProfileID, Intent: job.Intent,
		EstimatedBytes: 900 * 1024 * 1024, ReservedBytes: 900 * 1024 * 1024,
		State: TranscodeStorageReservationActive, AcquiredAt: now, CreatedAt: now, UpdatedAt: now,
	}
	incident := &TranscodeStorageIncidentRecord{
		ID: "legacy-storage-incident", Code: "io_error", Severity: "critical",
		Operation: "publish_artifact", Path: "/cache/transcode",
		Message: "legacy recovered incident", Retryable: true,
		AdmissionBlocked: true, QueuePaused: true, Occurrences: 2,
		FirstSeenAt: now, LastSeenAt: now, RecoveredAt: &now,
		Status: TranscodeStorageIncidentRecovered, CreatedAt: now, UpdatedAt: now,
	}

	// Keep dependency order explicit. Foreign keys remain enabled throughout the
	// certification so an invalid fixture cannot hide a migration regression.
	mustCreateProfileRow(t, db, "user", user)
	mustCreateProfileRow(t, db, "library", library)
	mustCreateProfileRow(t, db, "movie collection", collection)
	mustCreateProfileRow(t, db, "series", series)
	mustCreateProfileRow(t, db, "media", media)
	mustCreateProfileRow(t, db, "watch history", history)
	mustCreateProfileRow(t, db, "full preprocess task", preprocess)
	mustCreateProfileRow(t, db, "full subtitle task", subtitle)
	mustCreateProfileRow(t, db, "durable transcode job", job)
	mustCreateProfileRow(t, db, "storage reservation", reservation)
	mustCreateProfileRow(t, db, "recovered storage incident", incident)
}

func assertProfileIntegrity(t *testing.T, db *gorm.DB) {
	t.Helper()
	var result string
	if err := db.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
		t.Fatalf("sqlite integrity check: %v", err)
	}
	if result != "ok" {
		t.Fatalf("sqlite integrity check returned %q", result)
	}
	var foreignKeyViolations int64
	if err := db.Raw("SELECT COUNT(*) FROM pragma_foreign_key_check").Scan(&foreignKeyViolations).Error; err != nil {
		t.Fatalf("sqlite foreign key check: %v", err)
	}
	if foreignKeyViolations != 0 {
		t.Fatalf("sqlite foreign key violations: %d", foreignKeyViolations)
	}
}

func profileTableDDL(t *testing.T, db *gorm.DB, table string) string {
	t.Helper()
	var ddl string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&ddl).Error; err != nil {
		t.Fatalf("read %s schema: %v", table, err)
	}
	if ddl == "" {
		t.Fatalf("table %s is missing", table)
	}
	return ddl
}

func assertFullProfileRows(t *testing.T, db *gorm.DB) {
	t.Helper()
	assertProfileIntegrity(t, db)

	var user User
	if err := db.First(&user, "id=?", "legacy-user").Error; err != nil {
		t.Fatalf("legacy user: %v", err)
	}
	if user.Username != "legacy-admin" || user.Password != "legacy-password-hash" || user.Nickname != "Legacy Admin" {
		t.Fatalf("legacy user changed: %+v", user)
	}

	var collection MovieCollection
	if err := db.First(&collection, "id=?", "legacy-collection").Error; err != nil {
		t.Fatalf("legacy movie collection: %v", err)
	}
	if collection.Name != "Legacy Collection" || collection.MediaCount != 1 || collection.FileCount != 1 {
		t.Fatalf("legacy movie collection changed: %+v", collection)
	}

	var series Series
	if err := db.First(&series, "id=?", "legacy-series").Error; err != nil {
		t.Fatalf("legacy series: %v", err)
	}
	if series.LibraryID != "legacy-library" || series.FolderPath != "/legacy/series" {
		t.Fatalf("legacy series changed: %+v", series)
	}

	var media Media
	if err := db.First(&media, "id=?", "legacy-media").Error; err != nil {
		t.Fatalf("legacy media: %v", err)
	}
	if media.FilePath != "/legacy/movies/movie.mkv" || media.FileSize != 8*1024*1024*1024 || media.Duration != 7200 {
		t.Fatalf("legacy media changed: %+v", media)
	}
	if media.CollectionID != collection.ID || media.SeriesID != series.ID {
		t.Fatalf("legacy media relationships changed: %+v", media)
	}

	var history WatchHistory
	if err := db.First(&history, "id=?", "legacy-history").Error; err != nil {
		t.Fatalf("legacy watch history: %v", err)
	}
	if history.Position != 1234.5 || history.Completed {
		t.Fatalf("legacy watch history changed: %+v", history)
	}

	var preprocess PreprocessTask
	if err := db.First(&preprocess, "id=?", "legacy-preprocess").Error; err != nil {
		t.Fatalf("full preprocess history: %v", err)
	}
	if preprocess.Status != "completed" || preprocess.Progress != 100 {
		t.Fatalf("full preprocess history changed: %+v", preprocess)
	}

	var subtitle SubtitlePreprocessTask
	if err := db.First(&subtitle, "id=?", "legacy-subtitle-preprocess").Error; err != nil {
		t.Fatalf("full subtitle history: %v", err)
	}
	if subtitle.Status != "completed" || subtitle.Progress != 100 {
		t.Fatalf("full subtitle history changed: %+v", subtitle)
	}

	var job TranscodeJobRecord
	if err := db.First(&job, "id=?", "legacy-transcode-job").Error; err != nil {
		t.Fatalf("durable transcode job: %v", err)
	}
	if job.Status != "queued" || job.ActiveKey == nil || *job.ActiveKey != "legacy-media|runtime_hls|1080p" {
		t.Fatalf("durable transcode job changed: %+v", job)
	}

	var reservation TranscodeStorageReservationRecord
	if err := db.First(&reservation, "job_id=?", job.ID).Error; err != nil {
		t.Fatalf("storage reservation: %v", err)
	}
	if reservation.State != TranscodeStorageReservationActive || reservation.ReservedBytes != 900*1024*1024 {
		t.Fatalf("storage reservation changed: %+v", reservation)
	}

	var incident TranscodeStorageIncidentRecord
	if err := db.First(&incident, "id=?", "legacy-storage-incident").Error; err != nil {
		t.Fatalf("storage incident history: %v", err)
	}
	if incident.Status != TranscodeStorageIncidentRecovered || incident.Occurrences != 2 || incident.RecoveredAt == nil {
		t.Fatalf("storage incident history changed: %+v", incident)
	}
}

func copyProfileDB(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	if err := output.Sync(); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func profileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sqlite file: %v", err)
	}
	return sha256.Sum256(data)
}

func TestSQLiteFullLiteFullRoundTripPreservesDataAndBackup(t *testing.T) {
	dir := t.TempDir()
	currentPath := filepath.Join(dir, "nowen.db")
	backupPath := filepath.Join(dir, "nowen-before-lite.db")
	restoredPath := filepath.Join(dir, "nowen-restored-full.db")

	fullDB := openProfileDB(t, currentPath)
	if err := migrateFullProfile(fullDB); err != nil {
		t.Fatal(err)
	}
	seedFullProfile(t, fullDB)
	assertFullProfileRows(t, fullDB)
	preprocessDDL := profileTableDDL(t, fullDB, "preprocess_tasks")
	subtitleDDL := profileTableDDL(t, fullDB, "subtitle_preprocess_tasks")
	closeProfileDB(t, fullDB)

	if err := copyProfileDB(currentPath, backupPath); err != nil {
		t.Fatalf("create pre-lite backup: %v", err)
	}
	backupDigest := profileDigest(t, backupPath)

	liteDB := openProfileDB(t, currentPath)
	if err := migrateLiteProfile(liteDB); err != nil {
		t.Fatal(err)
	}
	assertFullProfileRows(t, liteDB)
	if got := profileTableDDL(t, liteDB, "preprocess_tasks"); got != preprocessDDL {
		t.Fatalf("Lite rewrote preprocess_tasks schema\n got: %s\nwant: %s", got, preprocessDDL)
	}
	if got := profileTableDDL(t, liteDB, "subtitle_preprocess_tasks"); got != subtitleDDL {
		t.Fatalf("Lite rewrote subtitle_preprocess_tasks schema\n got: %s\nwant: %s", got, subtitleDDL)
	}
	closeProfileDB(t, liteDB)

	if got := profileDigest(t, backupPath); got != backupDigest {
		t.Fatal("Lite startup modified the pre-upgrade backup")
	}

	rollbackDB := openProfileDB(t, currentPath)
	if err := migrateFullProfile(rollbackDB); err != nil {
		t.Fatal(err)
	}
	assertFullProfileRows(t, rollbackDB)
	closeProfileDB(t, rollbackDB)

	if err := copyProfileDB(backupPath, restoredPath); err != nil {
		t.Fatalf("restore pre-lite backup: %v", err)
	}
	restoredDB := openProfileDB(t, restoredPath)
	if err := migrateFullProfile(restoredDB); err != nil {
		t.Fatal(err)
	}
	assertFullProfileRows(t, restoredDB)
	closeProfileDB(t, restoredDB)
}

func TestFreshLiteMigrationDoesNotCreateFullOnlyTables(t *testing.T) {
	db := openProfileDB(t, filepath.Join(t.TempDir(), "fresh-lite.db"))
	if err := migrateLiteProfile(db); err != nil {
		t.Fatal(err)
	}
	assertProfileIntegrity(t, db)

	for name, table := range map[string]any{
		"preprocess tasks":          &PreprocessTask{},
		"subtitle preprocess tasks": &SubtitlePreprocessTask{},
		"video chapters":            &VideoChapter{},
		"video highlights":          &VideoHighlight{},
		"AI analysis tasks":         &AIAnalysisTask{},
		"AI cache":                  &AICacheEntry{},
	} {
		if db.Migrator().HasTable(table) {
			t.Fatalf("fresh Lite unexpectedly created full-only table: %s", name)
		}
	}

	for name, table := range map[string]any{
		"users":                &User{},
		"libraries":            &Library{},
		"media":                &Media{},
		"transcode jobs":       &TranscodeJobRecord{},
		"storage reservations": &TranscodeStorageReservationRecord{},
		"storage incidents":    &TranscodeStorageIncidentRecord{},
	} {
		if !db.Migrator().HasTable(table) {
			t.Fatalf("fresh Lite did not create core table: %s", name)
		}
	}
	closeProfileDB(t, db)
}
