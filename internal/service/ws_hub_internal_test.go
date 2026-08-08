package service

import (
	"sync/atomic"
	"testing"
)

func TestWSHubInternalObserverReceivesOriginalEventAndCanUnsubscribe(t *testing.T) {
	hub := NewWSHub(nil)
	var calls atomic.Int64
	var libraryID string
	unsubscribe := hub.SubscribeInternal(EventScanCompleted, func(event WSEvent) {
		calls.Add(1)
		libraryID = taskLifecycleSourceID(event.Data)
	})

	hub.BroadcastEvent(EventScanCompleted, &ScanProgressData{LibraryID: "library-1"})
	if calls.Load() != 1 || libraryID != "library-1" {
		t.Fatalf("observer did not receive scan completion: calls=%d library=%s", calls.Load(), libraryID)
	}

	unsubscribe()
	unsubscribe()
	hub.BroadcastEvent(EventScanCompleted, &ScanProgressData{LibraryID: "library-2"})
	if calls.Load() != 1 {
		t.Fatalf("unsubscribed observer was called again: %d", calls.Load())
	}
}

func TestWSHubInternalObserverPanicDoesNotBreakBroadcast(t *testing.T) {
	hub := NewWSHub(nil)
	var healthyCalls atomic.Int64
	hub.SubscribeInternal(EventScanCompleted, func(WSEvent) { panic("boom") })
	hub.SubscribeInternal(EventScanCompleted, func(WSEvent) { healthyCalls.Add(1) })

	hub.BroadcastEvent(EventScanCompleted, ScanProgressData{LibraryID: "library-1"})
	if healthyCalls.Load() != 1 {
		t.Fatalf("panic in one observer blocked another: %d", healthyCalls.Load())
	}
}

func TestWSHubInternalObserverIsScopedByEventType(t *testing.T) {
	hub := NewWSHub(nil)
	var calls atomic.Int64
	hub.SubscribeInternal(EventScanCompleted, func(WSEvent) { calls.Add(1) })
	hub.BroadcastEvent(EventScanProgress, ScanProgressData{LibraryID: "library-1"})
	if calls.Load() != 0 {
		t.Fatalf("observer received unrelated event: %d", calls.Load())
	}
}
