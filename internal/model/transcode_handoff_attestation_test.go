package model

import (
	"fmt"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func TestAutoMigrateTranscodeExecutionCreatesHandoffAttestationTable(t *testing.T) {
	dsn := fmt.Sprintf("file:handoff-migration-%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := AutoMigrateTranscodeExecution(db); err != nil {
		t.Fatal(err)
	}
	if !db.Migrator().HasTable(&TranscodeHandoffAttestationRecord{}) {
		t.Fatal("handoff attestation table was not created")
	}
	for _, column := range []string{
		"startup_artifact_id",
		"continuation_artifact_id",
		"contract_hash",
		"contract_json",
		"discontinuity_required",
		"decision_reason",
	} {
		if !db.Migrator().HasColumn(&TranscodeHandoffAttestationRecord{}, column) {
			t.Fatalf("handoff attestation column is missing: %s", column)
		}
	}
}
