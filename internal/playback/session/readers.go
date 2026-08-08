package session

import (
	"context"
	"sync"
)

// readerGate prevents a generation directory from being removed while an HTTP
// response is still reading one of its playlists or segments. Unlike a raw
// sync.WaitGroup, acquire and close are serialized so Add cannot race with Wait.
type readerGate struct {
	mu      sync.Mutex
	closing bool
	readers int
	done    chan struct{}
	once    sync.Once
}

func newReaderGate() *readerGate {
	return &readerGate{done: make(chan struct{})}
}

func (g *readerGate) acquire() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closing {
		return ErrGenerationNotActive
	}
	g.readers++
	return nil
}

func (g *readerGate) release() {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.readers == 0 {
		return
	}
	g.readers--
	if g.closing && g.readers == 0 {
		g.once.Do(func() { close(g.done) })
	}
}

func (g *readerGate) closeAndWait(ctx context.Context) error {
	g.mu.Lock()
	g.closing = true
	if g.readers == 0 {
		g.once.Do(func() { close(g.done) })
	}
	done := g.done
	g.mu.Unlock()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// ReaderLease must be released after the handler has finished serving the
// playlist or segment. Release is idempotent to make handler defer paths safe.
type ReaderLease struct {
	once    sync.Once
	release func()
}

func (l *ReaderLease) Release() {
	if l == nil {
		return
	}
	l.once.Do(l.release)
}
