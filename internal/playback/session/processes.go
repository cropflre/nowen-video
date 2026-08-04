package session

import "context"

// ProcessLease fences generation cleanup against the complete Runner lifetime,
// including hardware startup and any software fallback attempt.
type ProcessLease struct {
	lease *ReaderLease
}

func (l *ProcessLease) Release() {
	if l != nil && l.lease != nil {
		l.lease.Release()
	}
}

func (m *Manager) AcquireProcess(sessionID string, generationID uint64) (*ProcessLease, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return nil, err
	}

	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.closing {
		return nil, ErrSessionClosing
	}
	generation := session.generations[generationID]
	if generation == nil {
		return nil, ErrGenerationNotFound
	}
	gate := generation.ensureProcessGate()
	if err := gate.acquire(); err != nil {
		return nil, err
	}
	return &ProcessLease{lease: &ReaderLease{release: gate.release}}, nil
}

func (g *Generation) ensureProcessGate() *readerGate {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.processGate == nil {
		g.processGate = newReaderGate()
	}
	return g.processGate
}

func (g *Generation) waitForProcessExit(ctx context.Context) error {
	// A suspended process may not observe graceful termination on every
	// platform. Resume first; the cancelled generation context then owns exit.
	_ = g.resumeIfSuspended()
	return g.ensureProcessGate().closeAndWait(ctx)
}
