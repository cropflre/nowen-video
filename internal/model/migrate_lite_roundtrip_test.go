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

func openSQLiteProfileDB(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite profile database: %v", err)
	}
	return db
}

func closeSQLiteProfileDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sqlite connection: %v", err)
	}
	if _, err := sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
		t.Fatalf("checkpoint sqlite profile database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close sqlite profile database: %v", err)
	}
}

func migrateFullSQLiteProfile(db *gorm.DB) error {
	if err := AutoMigrate(db); err != nil {
		return fmt.Errorf("migrate full profile: %w", err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		return fmt.Errorf("migrate transcode execution: %w", err)
	}
	if err := AutoMigrateTranscodeStorageReservation(db); err != nil {
		return fmt.Errorf("migrate storage reservation: %w", err)
	}
	if err := AutoMigrateTranscodeStorageIncidents(db); err != nil {
		return fmt.Errorf("migrate storage incidents: %w", err)
	}
	return nil
}

func migrateLiteSQLiteProfile(db *gorm.DB, enableAI bool) error {
	if err := AutoMigrateLite(db, enableAI); err != nil {
		return fmt.Errorf("migrate lite profile: %w", err)
	}
	// These tables are part of Lite's core transcode runtime and are migrated
	// when NewTranscodeService and its storage governor are initialized.
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		return fmt.Errorf("migrate transcode execution: %w", err)
	}
	if err := AutoMigrateTranscodeStorageReservation(db); err != nil {
		return fmt.Errorf("migrate storage reservation: %w", err)
	}
	if err := AutoMigrateTranscodeStorageIncidents(db); err != nil {
		return fmt.Errorf("migrate storage incidents: %w", err)
	}
	return nil
}

func seedFullSQLiteProfile(t *testing.T, db *gorm.DB) {
	t.Helper()
	now := time.Date(2025, time.January, 2, 3, 4, 5, 0, time.UTC)
	user := User{
		ID:        "legacy-user",
		Username:  "legacy-admin",
		Password:  "legacy-password-hash",
		Role:      "admin",
		Nickname:  "Legacy Admin",
		CreatedAt: now,
		UpdatedAt: now,
	}
	library := Library{
		ID:        "legacy-library",
		Name:      "Legacy Movies",
		Path:      "/legacy/movies",
		Type:      "movie",
		CreatedAt: now,
		UpdatedAt: now,
	}
	media := Media{
		ID:         "legacy-media",
		LibraryID:  library.ID,
		Title:      "Legacy Movie",
		FilePath:   "/legacy/movies/movie.mkv",
		FileSize:   8 * 1024 * 1024 * 1024,
		MediaType:  "movie",
		VideoCodec: "hevc",
		AudioCodec: "aac",
		Resolution: "4K",
		Duration:   7200,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	history := WatchHistory{
		ID:        "legacy-history",
		UserID:    user.ID,
		MediaID:   media.ID,
		Position:  1234.5,
		Duration:  media.Duration,
		Completed: false,
		CreatedAt: now,
		UpdatedAt: now,
	}
	preprocess := PreprocessTask{
		ID:         "legacy-preprocess",
		MediaID:    media.ID,
		Status:     "completed",
		Phase:      "done",
		Progress:   100,
		MediaTitle: media.Title,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	subtitle := SubtitlePreprocessTask{
		ID:         "legacy-subtitle-preprocess",
		MediaID:    media.ID,
		Status:     "completed",
		Phase:      "done",
		Progress:   100,
		MediaTitle: media.Title,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	activeKey := "legacy-media|runtime_hls|1080p"
	job := TranscodeJobRecord{
		ID:                "legacy-transcode-job",
		MediaID:           media.ID,
		Intent:            "runtime_hls",
		ProfileID:         "1080p",
		Status:            "queued",
		DesiredState:      "running",
		ActiveKey:         &activeKey,
		SourceFingerprint: "legacy-source-fingerprint",
		PlannerVersion:    "runtime-hls-v2",
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	reservation := TranscodeStorageReservationRecord{
		JobID:          job.ID,
		MediaID:        media.ID,
		ProfileID:      job.ProfileID,
		Intent:         job.Intent,
		EstimatedBytes: 900 * 1024 * 1024,
		ReservedBytes:  900 * 1024 * 1024,
		State:          TranscodeStorageReservationActive,
		AcquiredAt:     now,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	incident := TranscodeStorageIncidentRecord{
		ID:               "legacy-storage-incident",
		Code:             "io_error",
		Severity:         "critical",
		Operation:        "publish_artifact",
		Path:             "/cache/transcode",
		Message:          "legacy recovered incident",
		Retryable:        true,
		AdmissionBlocked: true,
		QueuePaused:      true,
		Occurrences:      2,
		FirstSeenAt:      now,
		LastSeenAt:       now,
		RecoveredAt:      &now,
		Status:           TranscodeStorageIncidentRecovered,
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	for name, value := range map[string]any{
		"user":                      &user,
		"library":                   &library,
		"media":                     &media,
		"watch history":             &history,
		"full preprocess task":      &preprocess,
		"full subtitle task":        &subtitle,
		"durable transcode job":     &job,
		"storage reservation":       &reservation,
		"recovered storage incident": &incident,
	} {
		if err := db.Create(value).Error; err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
}

func assertSQLiteIntegrity(t *testing.T, db *gorm.DB) {
	t.Helper()
	var result string
	if err := db.Raw("PRAGMA integrity_check").Scan(&result).Error; err != nil {
		t.Fatalf("run sqlite integrity check: %v", err)
	}
	if result != "ok" {
		t.Fatalf("sqlite integrity check failed: %s", result)
	}
}

func tableDefinition(t *testing.T, db *gorm.DB, table string) string {
	t.Helper()
	var sql string
	if err := db.Raw("SELECT sql FROM sqlite_master WHERE type = 'table' AND name = ?", table).Scan(&sql).Error; err != nil {
		t.Fatalf("read table definition %s: %v", table, err)
	}
	if sql == "" {
		t.Fatalf("table definition %s is missing", table)
	}
	return sql
}

func assertFullSQLiteProfileData(t *testing.T, db *gorm.DB) {
	t.Helper()
	assertSQLiteIntegrity(t, db)

	var user User
	if err := db.First(&user, "id = ?", "legacy-user").Error; err != nil {
		t.Fatalf("load legacy user: %v", err)
	}
	if user.Username != "legacy-admin" || user.Nickname != "Legacy Admin" || user.Password != "legacy-password-hash" {
		t.Fatalf("legacy user changed: %+v", user)
	}

	var library Library
	if err := db.First(&library, "id = ?", "legacy-library").Error; err != nil {
		t.Fatalf("load legacy library: %v", err)
	}
	if library.Path != "/legacy/movies" || library.Name != "Legacy Movies" {
		t.Fatalf("legacy library changed: %+v", library)
	}

	var media Media
	if err := db.First(&media, "id = ?", "legacy-media").Error; err != nil {
		t.Fatalf("load legacy media: %v", err)
	}
	if media.Title != "Legacy Movie" || media.FilePath != "/legacy/movies/movie.mkv" || media.FileSize != 8*1024*1024*1024 {
		t.Fatalf("legacy media changed: %+v", media)
	}

	var history WatchHistory
	if err := db.First(&history, "id = ?", "legacy-history").Error; err != nil {
		t.Fatalf("load legacy watch history: %v", err)
	}
	if history.Position != 1234.5 || history.Completed {
		t.Fatalf("legacy watch history changed: %+v", history)
	}

	var preprocess PreprocessTask
	if err := db.First(&preprocess, "id = ?", "legacy-preprocess").Error; err != nil {
		t.Fatalf("load full-only preprocess task: %v", err)
	}
	if preprocess.Status != "completed" || preprocess.Progress != 100 {
		t.Fatalf("full-only preprocess task changed: %+v", preprocess)
	}

	var subtitle SubtitlePreprocessTask
	if err := db.First(&subtitle, "id = ?", "legacy-subtitle-preprocess").Error; err != nil {
		t.Fatalf("load full-only subtitle task: %v", err)
	}
	if subtitle.Status != "completed" || subtitle.Progress != 100 {
		t.Fatalf("full-only subtitle task changed: %+v", subtitle)
	}

	var job TranscodeJobRecord
	if err := db.First(&job, "id = ?", "legacy-transcode-job").Error; err != nil {
		t.Fatalf("load durable transcode job: %v", err)
	}
	if job.Status != "queued" || job.ActiveKey == nil || *job.ActiveKey != "legacy-media|runtime_hls|1080p" {
		t.Fatalf("durable transcode job changed: %+v", job)
	}

	var reservation TranscodeStorageReservationRecord
	if err := db.First(&reservation, "job_id = ?", job.ID).Error; err != nil {
		t.Fatalf("load storage reservation: %v", err)
	}
	if reservation.State != TranscodeStorageReservationActive || reservation.ReservedBytes != 900*1024*1024 {
		t.Fatalf("storage reservation changed: %+v", reservation)
	}

	var incident TranscodeStorageIncidentRecord
	if err := db.First(&incident, "id = ?", "legacy-storage-incident").Error; err != nil {
		t.Fatalf("load storage incident: %v", err)
	}
	if incident.Status != TranscodeStorageIncidentRecovered || incident.Occurrences != 2 || incident.RecoveredAt == nil {
		t.Fatalf("storage incident changed: %+v", incident)
	}
}

func copySQLiteProfileFile(src, dst string) error {
	input, err := os.Open(src)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
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

func fileDigest(t *testing.T, path string) [32]byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read sqlite backup: %v", err)
	}
	return sha256.Sum256(data)
}

func TestSQLiteFullLiteFullRoundTripPreservesDataAndBackup(t *testing.T) {
	dir := t.TempDir()
	databasePath := filepath.Join(dir, "nowen.db")
	backupPath := filepath.Join(dir, "nowen-before-lite.db")
	restoredPath := filepath.Join(dir, "nowen-restored-full.db")

	fullDB := openSQLiteProfileDB(t, databasePath)
	if err := migrateFullSQLiteProfile(fullDB); err != nil {
		t.Fatal(err)
	}
	seedFullSQLiteProfile(t, fullDB)
	assertFullSQLiteProfileData(t, fullDB)
	fullPreprocessDDL := tableDefinition(t, fullDB, "preprocess_tasks")
	fullSubtitleDDL := tableDefinition(t, fullDB, "subtitle_preprocess_tasks")
	closeSQLiteProfileDB(t, fullDB)

	if err := copySQLiteProfileFile(databasePath, backupPath); err != nil {
		t.Fatalf("create pre-lite sqlite backup: %v", err)
	}
	backupDigest := fileDigest(t, backupPath)

	liteDB := openSQLiteProfileDB(t, databasePath)
	if err := migrateLiteSQLiteProfile(liteDB, false); err != nil {
		t.Fatal(err)
	}
	assertFullSQLiteProfileData(t, liteDB)
	if got := tableDefinition(t, liteDB, "preprocess_tasks"); got != fullPreprocessDDL {
		t.Fatalf("lite migration rewrote full-only preprocess schema\n got: %s\nwant: %s", got, fullPreprocessDDL)
	}
	if got := tableDefinition(t, liteDB, "subtitle_preprocess_tasks"); got != fullSubtitleDDL {
		t.Fatalf("lite migration rewrote full-only subtitle schema\n got: %s\nwant: %s", got, fullSubtitleDDL)
	}
	closeSQLiteProfileDB(t, liteDB)

	if got := fileDigest(t, backupPath); got != backupDigest {
		t.Fatal("pre-lite sqlite backup was modified during lite startup")
	}

	rollbackDB := openSQLiteProfileDB(t, databasePath)
	if err := migrateFullSQLiteProfile(rollbackDB); err != nil {
		t.Fatal(err)
	}
	assertFullSQLiteProfileData(t, rollbackDB)
	closeSQLiteProfileDB(t, rollbackDB)

	if err := copySQLiteProfileFile(backupPath, restoredPath); err != nil {
		t.Fatalf("restore sqlite backup copy: %v", err)
	}
	restoredDB := openSQLiteProfileDB(t, restoredPath)
	if err := migrateFullSQLiteProfile(restoredDB); err != nil {
		t.Fatal(err)
	}
	assertFullSQLiteProfileData(t, restoredDB)
	closeSQLiteProfileDB(t, restoredDB)
}

func TestFreshLiteMigrationDoesNotCreateFullOnlyTables(t *testing.T) {
	databasePath := filepath.Join(t.TempDir(), "fresh-lite.db")
	db := openSQLiteProfileDB(t, databasePath)
	if err := migrateLiteSQLiteProfile(db, false); err != nil {
		t.Fatal(err)
	}
	assertSQLiteIntegrity(t, db)

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
	closeSQLiteProfileDB(t, db)
}
