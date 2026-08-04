package service

import (
	"strings"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestStorageIncidentTaskProjectionExposesOperationalDiagnostics(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	incident := &model.TranscodeStorageIncidentRecord{
		ID:          "incident-1",
		Code:        "read_only",
		Severity:    "critical",
		Operation:   storageHealthProbeOperation,
		Path:        "/cache/transcode",
		Message:     "read-only file system",
		Retryable:   false,
		Occurrences: 3,
		FirstSeenAt: now.Add(-time.Minute),
		LastSeenAt:  now,
		Status:      model.TranscodeStorageIncidentActive,
	}
	task := storageIncidentToUnifiedTask(incident)
	if task.Kind != TaskKindStorageIncident || task.Status != TaskStatusFailed || task.SourceID != incident.ID {
		t.Fatalf("unexpected storage incident projection: %+v", task)
	}
	for _, expected := range []string{"只读文件系统", "已出现 3 次", incident.Message, incident.Path, "需要管理员"} {
		if !strings.Contains(task.Subtitle+" "+task.Message, expected) {
			t.Fatalf("storage incident projection missing %q: %+v", expected, task)
		}
	}
	if actions := AvailableTaskActions(task.Kind, task.Status); len(actions) != 0 {
		t.Fatalf("storage incidents must not expose unsafe bypass actions: %v", actions)
	}
}
