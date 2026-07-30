package service

import (
	"encoding/json"
	"testing"

	"go.uber.org/zap"
)

func TestTaskLifecycleUpdateForEvent(t *testing.T) {
	tests := []struct {
		event    string
		data     interface{}
		kind     string
		status   string
		sourceID string
	}{
		{EventScanStarted, &ScanProgressData{LibraryID: "library-1"}, TaskKindScan, TaskStatusRunning, "library-1"},
		{EventScanCompleted, ScanProgressData{LibraryID: "library-2"}, TaskKindScan, TaskStatusCompleted, "library-2"},
		{EventScrapeProgress, &ScrapeProgressData{LibraryID: "library-3"}, TaskKindScrape, TaskStatusRunning, "library-3"},
		{EventTranscodeCompleted, &TranscodeProgressData{TaskID: "task-1"}, TaskKindTranscode, TaskStatusCompleted, "task-1"},
		{EventTranscodeFailed, TranscodeProgressData{TaskID: "task-2"}, TaskKindTranscode, TaskStatusFailed, "task-2"},
	}

	for _, tt := range tests {
		update, ok := taskLifecycleUpdateForEvent(tt.event, tt.data)
		if !ok {
			t.Fatalf("event %s was not mapped", tt.event)
		}
		if update.Kind != tt.kind || update.Status != tt.status || update.SourceID != tt.sourceID || update.SourceEvent != tt.event {
			t.Fatalf("event=%s update=%+v", tt.event, update)
		}
	}

	if update, ok := taskLifecycleUpdateForEvent(EventLibraryUpdated, nil); ok || update != nil {
		t.Fatalf("non-task event must not be mapped: %+v", update)
	}
	if update, ok := taskLifecycleUpdateForEvent(EventTaskUpdated, nil); ok || update != nil {
		t.Fatalf("task_updated must not recursively map: %+v", update)
	}
}

func TestBroadcastEventAlsoEmitsTaskUpdated(t *testing.T) {
	hub := NewWSHub(zap.NewNop().Sugar())
	hub.BroadcastEvent(EventTranscodeProgress, &TranscodeProgressData{TaskID: "task-9"})

	var original WSEvent
	if err := json.Unmarshal(<-hub.broadcast, &original); err != nil {
		t.Fatal(err)
	}
	if original.Type != EventTranscodeProgress {
		t.Fatalf("unexpected original event: %s", original.Type)
	}

	var unified WSEvent
	if err := json.Unmarshal(<-hub.broadcast, &unified); err != nil {
		t.Fatal(err)
	}
	if unified.Type != EventTaskUpdated {
		t.Fatalf("expected %s, got %s", EventTaskUpdated, unified.Type)
	}

	payload, err := json.Marshal(unified.Data)
	if err != nil {
		t.Fatal(err)
	}
	var update TaskLifecycleUpdate
	if err := json.Unmarshal(payload, &update); err != nil {
		t.Fatal(err)
	}
	if update.Kind != TaskKindTranscode || update.SourceID != "task-9" || update.SourceEvent != EventTranscodeProgress {
		t.Fatalf("unexpected unified update: %+v", update)
	}
}

func TestBroadcastEventDoesNotDuplicateNonTaskEvents(t *testing.T) {
	hub := NewWSHub(zap.NewNop().Sugar())
	hub.BroadcastEvent(EventLibraryUpdated, &LibraryChangedData{LibraryID: "library-1"})

	select {
	case <-hub.broadcast:
	default:
		t.Fatal("expected original event")
	}
	select {
	case unexpected := <-hub.broadcast:
		t.Fatalf("unexpected duplicate event: %s", unexpected)
	default:
	}
}
