package session

import (
	"context"
	"time"
)

// GenerationRuntime is the process-facing view of one generation. The context
// is owned by the session manager and is cancelled by seek replacement,
// explicit close, timeout cleanup, or server shutdown.
type GenerationRuntime struct {
	Context   context.Context
	OutputDir string
	Snapshot  GenerationSnapshot
}

func (m *Manager) Runtime(sessionID string, generationID uint64) (GenerationRuntime, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return GenerationRuntime{}, err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.closing {
		return GenerationRuntime{}, ErrSessionClosing
	}
	generation := session.generations[generationID]
	if generation == nil {
		return GenerationRuntime{}, ErrGenerationNotFound
	}
	return GenerationRuntime{
		Context:   generation.ctx,
		OutputDir: generation.OutputDir,
		Snapshot:  generation.snapshot(),
	}, nil
}

func (m *Manager) ResetGenerationAttempt(sessionID string, generationID uint64, backend string) error {
	session, generation, err := m.executionGeneration(sessionID, generationID)
	if err != nil {
		return err
	}
	now := m.now()
	generation.mu.Lock()
	generation.backend = backend
	generation.processPID = 0
	generation.transcodedMS = 0
	generation.speed = ""
	generation.errorCode = ""
	generation.errorMessage = ""
	generation.startedAt = nil
	generation.firstSegmentAt = nil
	generation.completedAt = nil
	generation.updatedAt = now
	generation.mu.Unlock()

	session.mu.Lock()
	session.updatedAt = now
	session.mu.Unlock()
	return nil
}

func (m *Manager) MarkGenerationStarted(sessionID string, generationID uint64, backend string, processPID int) error {
	session, generation, err := m.executionGeneration(sessionID, generationID)
	if err != nil {
		return err
	}
	now := m.now()
	generation.mu.Lock()
	generation.backend = backend
	generation.processPID = processPID
	generation.startedAt = &now
	generation.updatedAt = now
	generation.mu.Unlock()

	session.mu.Lock()
	session.updatedAt = now
	session.mu.Unlock()
	return nil
}

func (m *Manager) MarkGenerationProgress(sessionID string, generationID uint64, transcodedMS int64, speed string) error {
	_, generation, err := m.executionGeneration(sessionID, generationID)
	if err != nil {
		return err
	}
	if transcodedMS < 0 {
		transcodedMS = 0
	}
	generation.mu.Lock()
	generation.transcodedMS = transcodedMS
	generation.speed = speed
	generation.updatedAt = m.now()
	generation.mu.Unlock()
	return nil
}

func (m *Manager) MarkFirstSegmentReady(sessionID string, generationID uint64) error {
	_, generation, err := m.executionGeneration(sessionID, generationID)
	if err != nil {
		return err
	}
	now := m.now()
	generation.mu.Lock()
	if generation.firstSegmentAt == nil {
		generation.firstSegmentAt = &now
	}
	generation.updatedAt = now
	generation.mu.Unlock()
	return nil
}

// MarkGenerationCompleted records process completion but intentionally leaves
// the generation readable. A fully encoded temporary playlist remains owned by
// the playback session and is deleted only when that session closes.
func (m *Manager) MarkGenerationCompleted(sessionID string, generationID uint64) error {
	_, generation, err := m.executionGeneration(sessionID, generationID)
	if err != nil {
		return err
	}
	now := m.now()
	generation.mu.Lock()
	generation.processPID = 0
	generation.completedAt = &now
	generation.updatedAt = now
	generation.mu.Unlock()
	return nil
}

func (m *Manager) MarkGenerationFailed(sessionID string, generationID uint64, errorCode, errorMessage string) error {
	session, generation, err := m.executionGeneration(sessionID, generationID)
	if err != nil {
		return err
	}
	now := m.now()
	generation.mu.Lock()
	generation.state = GenerationStateFailed
	generation.processPID = 0
	generation.errorCode = errorCode
	generation.errorMessage = errorMessage
	generation.completedAt = &now
	generation.updatedAt = now
	generation.cancel()
	generation.mu.Unlock()

	session.mu.Lock()
	if session.pendingGenerationID == generationID {
		session.pendingGenerationID = 0
	}
	if session.currentGenerationID == generationID || session.currentGenerationID == 0 {
		session.state = SessionStateFailed
		session.closeReason = errorCode
	}
	session.updatedAt = now
	session.mu.Unlock()
	return nil
}

func (m *Manager) CancelGeneration(sessionID string, generationID uint64) error {
	_, generation, err := m.executionGeneration(sessionID, generationID)
	if err != nil {
		return err
	}
	generation.cancel()
	return nil
}

func (m *Manager) executionGeneration(sessionID string, generationID uint64) (*PlaybackSession, *Generation, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return nil, nil, err
	}
	session.mu.RLock()
	defer session.mu.RUnlock()
	if session.closing {
		return nil, nil, ErrSessionClosing
	}
	generation := session.generations[generationID]
	if generation == nil {
		return nil, nil, ErrGenerationNotFound
	}
	return session, generation, nil
}

func elapsedMilliseconds(startedAt time.Time) int64 {
	if startedAt.IsZero() {
		return 0
	}
	return time.Since(startedAt).Milliseconds()
}
