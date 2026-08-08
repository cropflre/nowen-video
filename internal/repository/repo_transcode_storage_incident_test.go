package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestStorageIncidentDeduplicatesActiveFaultAndPreservesRecurrence(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	if err := model.AutoMigrateTranscodeStorageIncidents(repo.db); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	input := TranscodeStorageIncidentInput{
		Code:             "read_only",
		Severity:         "critical",
		Operation:        "artifact_store_probe",
		Path:             "/cache/transcode",
		Message:          "read-only file system",
		Retryable:        false,
		AdmissionBlocked: true,
		QueuePaused:      true,
	}
	first, err := repo.ReportStorageIncident(input, now)
	if err != nil {
		t.Fatal(err)
	}
	second, err := repo.ReportStorageIncident(input, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if first.ID != second.ID || second.Occurrences != 2 {
		t.Fatalf("active incident was not deduplicated: first=%+v second=%+v", first, second)
	}
	recovered, err := repo.RecoverStorageIncidents(input.Operation, now.Add(2*time.Second))
	if err != nil || recovered != 1 {
		t.Fatalf("incident recovery failed: recovered=%d err=%v", recovered, err)
	}
	third, err := repo.ReportStorageIncident(input, now.Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if third.ID == first.ID || third.Occurrences != 1 {
		t.Fatalf("recurrent outage did not create new evidence: first=%+v third=%+v", first, third)
	}
	summary, err := repo.StorageIncidentSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.ActiveCount != 1 || summary.CriticalCount != 1 || summary.RecoveredCount != 1 || summary.TotalOccurrences != 3 {
		t.Fatalf("unexpected incident summary: %+v", summary)
	}
}
