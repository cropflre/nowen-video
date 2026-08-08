package governor

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestGovernorSeparatesResourcePools(t *testing.T) {
	g := New(Config{SoftwareTranscodes: 1, HardwareTranscodes: 1, RemuxStreams: 1, OnDemandSegments: 1})
	software, err := g.Acquire(context.Background(), KindSoftwareTranscode)
	if err != nil {
		t.Fatal(err)
	}
	defer software.Release()

	remux, err := g.Acquire(context.Background(), KindRemux)
	if err != nil {
		t.Fatal(err)
	}
	defer remux.Release()

	snapshot := g.Snapshot()
	if snapshot.InUse[KindSoftwareTranscode] != 1 || snapshot.InUse[KindRemux] != 1 {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if snapshot.PeakInUse[KindSoftwareTranscode] != 1 || snapshot.PeakInUse[KindRemux] != 1 {
		t.Fatalf("unexpected peak snapshot: %+v", snapshot)
	}
}

func TestGovernorAcquireHonorsCancellation(t *testing.T) {
	g := New(Config{SoftwareTranscodes: 1})
	lease, err := g.Acquire(context.Background(), KindSoftwareTranscode)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Release()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err = g.Acquire(ctx, KindSoftwareTranscode)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected deadline exceeded, got %v", err)
	}
	if waiting := g.Snapshot().Waiting[KindSoftwareTranscode]; waiting != 0 {
		t.Fatalf("cancelled waiter leaked from snapshot: %d", waiting)
	}
}

func TestGovernorReportsContentionWithoutOversubscription(t *testing.T) {
	g := New(Config{SoftwareTranscodes: 1})
	first, err := g.Acquire(context.Background(), KindSoftwareTranscode)
	if err != nil {
		t.Fatal(err)
	}

	secondLease := make(chan *Lease, 1)
	secondErr := make(chan error, 1)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	go func() {
		lease, acquireErr := g.Acquire(ctx, KindSoftwareTranscode)
		if acquireErr != nil {
			secondErr <- acquireErr
			return
		}
		secondLease <- lease
	}()

	waitForGovernorSnapshot(t, g, time.Second, func(snapshot Snapshot) bool {
		return snapshot.InUse[KindSoftwareTranscode] == 1 &&
			snapshot.Waiting[KindSoftwareTranscode] == 1 &&
			snapshot.PeakInUse[KindSoftwareTranscode] == 1
	})

	first.Release()

	var second *Lease
	select {
	case second = <-secondLease:
	case acquireErr := <-secondErr:
		t.Fatalf("second acquire failed: %v", acquireErr)
	case <-time.After(time.Second):
		t.Fatal("second acquire did not resume after release")
	}

	snapshot := g.Snapshot()
	if snapshot.InUse[KindSoftwareTranscode] != 1 {
		t.Fatalf("expected exactly one active lease, got %+v", snapshot)
	}
	if snapshot.Waiting[KindSoftwareTranscode] != 0 {
		t.Fatalf("expected contention queue to drain, got %+v", snapshot)
	}
	if snapshot.PeakInUse[KindSoftwareTranscode] != 1 {
		t.Fatalf("pool oversubscribed: %+v", snapshot)
	}

	second.Release()
	if inUse := g.Snapshot().InUse[KindSoftwareTranscode]; inUse != 0 {
		t.Fatalf("lease release did not drain pool: %d", inUse)
	}
}

func TestGovernorCancellationRemovesWaitingAdmission(t *testing.T) {
	g := New(Config{OnDemandSegments: 1})
	active, err := g.Acquire(context.Background(), KindOnDemand)
	if err != nil {
		t.Fatal(err)
	}
	defer active.Release()

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() {
		_, acquireErr := g.Acquire(ctx, KindOnDemand)
		result <- acquireErr
	}()

	waitForGovernorSnapshot(t, g, time.Second, func(snapshot Snapshot) bool {
		return snapshot.Waiting[KindOnDemand] == 1
	})
	cancel()

	select {
	case acquireErr := <-result:
		if !errors.Is(acquireErr, context.Canceled) {
			t.Fatalf("expected context cancellation, got %v", acquireErr)
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled admission did not return")
	}

	snapshot := g.Snapshot()
	if snapshot.Waiting[KindOnDemand] != 0 || snapshot.InUse[KindOnDemand] != 1 {
		t.Fatalf("unexpected snapshot after cancellation: %+v", snapshot)
	}
}

func TestLeaseReleaseIsIdempotent(t *testing.T) {
	g := New(Config{OnDemandSegments: 1})
	lease, err := g.Acquire(context.Background(), KindOnDemand)
	if err != nil {
		t.Fatal(err)
	}
	lease.Release()
	lease.Release()

	next, err := g.Acquire(context.Background(), KindOnDemand)
	if err != nil {
		t.Fatal(err)
	}
	next.Release()
}

func waitForGovernorSnapshot(t *testing.T, g *Governor, timeout time.Duration, predicate func(Snapshot) bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if predicate(g.Snapshot()) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("governor snapshot did not reach expected state: %+v", g.Snapshot())
}
