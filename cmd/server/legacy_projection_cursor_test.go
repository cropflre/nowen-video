package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLegacyProjectionMigrationKeepsDurableCursorBoundary(t *testing.T) {
	root := filepath.Join("..", "..")
	checks := map[string][]string{
		"internal/model/transcode_execution.go":                             {"legacy_transcode_projection_migrations", "HighWaterUpdatedAt", "SourceRetireAfter"},
		"internal/repository/repo_legacy_transcode_projection_migration.go": {"ClaimLegacyProjectionMigration", "RenewLegacyProjectionMigrationLease", "CompleteLegacyProjectionMigrationBatch", "RetryLegacyProjectionMigration"},
		"internal/service/legacy_transcode_projection_migration.go":         {"ListLegacyProjectionSourceAfter", "legacyProjectionSourceRetirementWindow", "legacyProjectionSourceCheckInterval", "legacyProjectionMigrationLease"},
	}
	for name, needles := range checks {
		content, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		text := string(content)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				t.Fatalf("%s missing %s", name, needle)
			}
		}
	}
	serviceContent, err := os.ReadFile(filepath.Join(root, "internal/service/legacy_transcode_projection_migration.go"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(serviceContent), "ListLegacyTerminalWithOutput(500)") {
		t.Fatal("legacy migration reintroduced head-of-table rescans")
	}
}
