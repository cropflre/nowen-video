package governor

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Kind identifies the bottleneck consumed by an execution attempt.
type Kind string

const (
	KindSoftwareTranscode Kind = "software_transcode"
	KindHardwareTranscode Kind = "hardware_transcode"
	KindRemux             Kind = "remux"
	KindOnDemand          Kind = "ondemand"
)

// Config defines independent capacity pools. Full transcodes deliberately do
// not share slots with lightweight remux or urgent on-demand segment work.
type Config struct {
	SoftwareTranscodes int
	HardwareTranscodes int
	RemuxStreams       int
	OnDemandSegments   int
}

func DefaultConfig() Config {
	return Config{
		SoftwareTranscodes: 1,
		HardwareTranscodes: 1,
		RemuxStreams:       4,
		OnDemandSegments:   2,
	}
}

// Snapshot is safe to expose in diagnostics. Waiting makes admission pressure
// visible while PeakInUse proves that a pool has not exceeded its configured
// capacity during the current process lifetime.
type Snapshot struct {
	Capacity  map[Kind]int `json:"capacity"`
	InUse     map[Kind]int `json:"in_use"`
	Waiting   map[Kind]int `json:"waiting"`
	PeakInUse map[Kind]int `json:"peak_in_use"`
}

// Lease releases a resource slot exactly once.
type Lease struct {
	kind     Kind
	acquired time.Time
	release  func()
	once     sync.Once
}

func (l *Lease) Kind() Kind            { return l.kind }
func (l *Lease) AcquiredAt() time.Time { return l.acquired }
func (l *Lease) Release() {
	if l == nil {
		return
	}
	l.once.Do(l.release)
}

// Governor coordinates all media process classes. It is intentionally small;
// priority scheduling is implemented by the orchestrator above this layer.
type Governor struct {
	mu        sync.RWMutex
	capacity  map[Kind]int
	inUse     map[Kind]int
	waiting   map[Kind]int
	peakInUse map[Kind]int
	sems      map[Kind]chan struct{}
}

func New(config Config) *Governor {
	capacity := map[Kind]int{
		KindSoftwareTranscode: normalize(config.SoftwareTranscodes),
		KindHardwareTranscode: normalize(config.HardwareTranscodes),
		KindRemux:             normalize(config.RemuxStreams),
		KindOnDemand:          normalize(config.OnDemandSegments),
	}
	sems := make(map[Kind]chan struct{}, len(capacity))
	for kind, size := range capacity {
		sems[kind] = make(chan struct{}, size)
	}
	return &Governor{
		capacity:  capacity,
		inUse:     make(map[Kind]int, len(capacity)),
		waiting:   make(map[Kind]int, len(capacity)),
		peakInUse: make(map[Kind]int, len(capacity)),
		sems:      sems,
	}
}

func normalize(value int) int {
	if value < 1 {
		return 1
	}
	return value
}

func (g *Governor) Acquire(ctx context.Context, kind Kind) (*Lease, error) {
	if g == nil {
		return nil, fmt.Errorf("resource governor is nil")
	}
	sem, ok := g.sems[kind]
	if !ok {
		return nil, fmt.Errorf("unknown resource kind %q", kind)
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	g.mu.Lock()
	g.waiting[kind]++
	g.mu.Unlock()

	select {
	case sem <- struct{}{}:
		g.mu.Lock()
		g.waiting[kind]--
		g.inUse[kind]++
		if g.inUse[kind] > g.peakInUse[kind] {
			g.peakInUse[kind] = g.inUse[kind]
		}
		g.mu.Unlock()
		return &Lease{
			kind:     kind,
			acquired: time.Now(),
			release: func() {
				// Update accounting before making the semaphore slot visible to the
				// next waiter. Reversing this order creates a false handoff window
				// where two leases appear active and PeakInUse can exceed capacity.
				g.mu.Lock()
				if g.inUse[kind] > 0 {
					g.inUse[kind]--
				}
				g.mu.Unlock()
				<-sem
			},
		}, nil
	case <-ctx.Done():
		g.mu.Lock()
		if g.waiting[kind] > 0 {
			g.waiting[kind]--
		}
		g.mu.Unlock()
		return nil, ctx.Err()
	}
}

func (g *Governor) Snapshot() Snapshot {
	result := Snapshot{
		Capacity:  make(map[Kind]int, len(g.capacity)),
		InUse:     make(map[Kind]int, len(g.capacity)),
		Waiting:   make(map[Kind]int, len(g.capacity)),
		PeakInUse: make(map[Kind]int, len(g.capacity)),
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for kind, value := range g.capacity {
		result.Capacity[kind] = value
		result.InUse[kind] = g.inUse[kind]
		result.Waiting[kind] = g.waiting[kind]
		result.PeakInUse[kind] = g.peakInUse[kind]
	}
	return result
}
