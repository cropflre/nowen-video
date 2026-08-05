package model

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestTranscodeExecutionMigrationCreatesLegacyProjectionStateOnly(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file:legacy-projection-state?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&LegacyTranscodeProjectionMigrationState{}) {
		t.Fatal("migration state table missing")
	}
	if db.Migrator().HasTable(&TranscodeTask{}) {
		t.Fatal("execution migration recreated legacy transcode_tasks")
	}
}
