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
