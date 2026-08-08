package service

import (
	"testing"
	"time"
)

func TestNormalizeTaskStatus(t *testing.T) {
	cases := map[string]string{
		"pending":     TaskStatusQueued,
		"running":     TaskStatusRunning,
		"scraping":    TaskStatusRunning,
		"translating": TaskStatusRunning,
		"done":        TaskStatusCompleted,
		"scraped":     TaskStatusCompleted,
		"failed":      TaskStatusFailed,
		"cancelled":   TaskStatusCancelled,
	}
	for input, expected := range cases {
		if actual := normalizeTaskStatus(input); actual != expected {
			t.Fatalf("normalizeTaskStatus(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestScanPhaseToUnifiedTaskUsesCurrentProgress(t *testing.T) {
	now := time.Now()
	task := scanPhaseToUnifiedTask(ScanPhaseData{
		LibraryID:   "lib-1",
		LibraryName: "电影",
		Phase:       "scanning",
		StepCurrent: 1,
		StepTotal:   4,
		Current:     25,
		Total:       100,
		Message:     "扫描中",
	}, &now)

	if task.ID != "scan:lib-1" || task.Kind != TaskKindScan || task.Status != TaskStatusRunning {
		t.Fatalf("unexpected scan task: %+v", task)
	}
	if task.Progress != 25 {
		t.Fatalf("scan progress = %v, want 25", task.Progress)
	}
}

func TestClampProgress(t *testing.T) {
	if clampProgress(-1) != 0 || clampProgress(101) != 100 || clampProgress(55) != 55 {
		t.Fatal("clampProgress must keep values between 0 and 100")
	}
}
