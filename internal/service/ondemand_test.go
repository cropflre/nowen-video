package service

import (
	"sync"
	"testing"
	"time"
)

func TestParseSegmentIndex(t *testing.T) {
	tests := map[string]int{
		"seg_0003.ts":    3,
		"seg3.ts":        3,
		"segment_12.aac": 12,
	}
	for name, expected := range tests {
		actual, err := parseSegmentIndex(name)
		if err != nil || actual != expected {
			t.Fatalf("name=%s actual=%d err=%v", name, actual, err)
		}
	}
	if _, err := parseSegmentIndex("segment.ts"); err == nil {
		t.Fatal("segment without index must be rejected")
	}
}

func TestOnDemandLimiterSerializesAndRemovesKeys(t *testing.T) {
	limiter := &onDemandLimiter{keys: make(map[string]*onDemandKeyLock)}
	firstRelease := limiter.acquire("media/720p/seg_1")

	acquiredSecond := make(chan struct{})
	var secondRelease func()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		secondRelease = limiter.acquire("media/720p/seg_1")
		close(acquiredSecond)
	}()

	select {
	case <-acquiredSecond:
		t.Fatal("duplicate artifact work must be serialized")
	case <-time.After(30 * time.Millisecond):
	}
	firstRelease()

	select {
	case <-acquiredSecond:
	case <-time.After(time.Second):
		t.Fatal("second waiter did not acquire after release")
	}
	secondRelease()
	wg.Wait()
	if size := limiter.size(); size != 0 {
		t.Fatalf("released keyed locks must be removed, size=%d", size)
	}
}
