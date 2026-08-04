package session

import (
	"context"
	"os"
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
	return m.updateGeneration(sessionID, generationID, func(session *PlaybackSession, generation *Generation) {
		now := m.now()
		generation.backend = backend
		generation.process = nil
		generation.processPID = 0
		generation.transcodedMS = 0
		generation.aheadMS = 0
		generation.suspended = false
		generation.speed = ""
		generation.errorCode = ""
		generation.errorMessage = ""
		generation.startedAt = nil
		generation.firstSegmentAt = nil
		generation.completedAt = nil
		generation.updatedAt = now
		session.updatedAt = now
	})
}

func (m *Manager) MarkGenerationStarted(sessionID string, generationID uint64, backend string, processPID int) error {
	return m.updateGeneration(sessionID, generationID, func(session *PlaybackSession, generation *Generation) {
		now := m.now()
		generation.backend = backend
		generation.processPID = processPID
		generation.process = nil
		if processPID > 0 {
			if process, err := os.FindProcess(processPID); err == nil {
				generation.process = process
			}
		}
		generation.startedAt = &now
		generation.updatedAt = now
		session.updatedAt = now
	})
}

func (m *Manager) MarkGenerationProgress(sessionID string, generationID uint64, transcodedMS int64, speed string) error {
	if transcodedMS < 0 {
		transcodedMS = 0
	}
	if err := m.updateGeneration(sessionID, generationID, func(_ *PlaybackSession, generation *Generation) {
		generation.transcodedMS = transcodedMS
		generation.speed = speed
		generation.updatedAt = m.now()
	}); err != nil {
		return err
	}
	return m.ReconcileThrottle(sessionID)
}

func (m *Manager) MarkFirstSegmentReady(sessionID string, generationID uint64) error {
	return m.updateGeneration(sessionID, generationID, func(_ *PlaybackSession, generation *Generation) {
		now := m.now()
		if generation.firstSegmentAt == nil {
			generation.firstSegmentAt = &now
		}
		generation.updatedAt = now
	})
}

// MarkGenerationCompleted records process completion but intentionally leaves
// the generation readable. A fully encoded temporary playlist remains owned by
// the playback session and is deleted only when that session closes.
func (m *Manager) MarkGenerationCompleted(sessionID string, generationID uint64) error {
	return m.updateGeneration(sessionID, generationID, func(_ *PlaybackSession, generation *Generation) {
		now := m.now()
		generation.process = nil
		generation.processPID = 0
		generation.suspended = false
		generation.completedAt = &now
		generation.updatedAt = now
	})
}

func (m *Manager) MarkGenerationFailed(sessionID string, generationID uint64, errorCode, errorMessage string) error {
	return m.updateGeneration(sessionID, generationID, func(session *PlaybackSession, generation *Generation) {
		now := m.now()
		generation.process = nil
		generation.processPID = 0
		generation.suspended = false
		generation.errorCode = errorCode
		generation.errorMessage = errorMessage
		generation.completedAt = &now
		generation.updatedAt = now
		generation.cancel()

		// Once a timeline has been published, keep its already-materialized
		// playlist and segments readable. The client can consume buffered media
		// or explicitly restart a new Generation instead of being cut off by a
		// late encoder failure.
		published := generation.firstSegmentAt != nil && session.currentGenerationID == generationID
		if published {
			session.updatedAt = now
			return
		}

		generation.state = GenerationStateFailed
		if session.pendingGenerationID == generationID {
			session.pendingGenerationID = 0
		}
		if session.currentGenerationID == generationID || session.currentGenerationID == 0 {
			session.state = SessionStateFailed
			session.closeReason = errorCode
		}
		session.updatedAt = now
	})
}

func (m *Manager) CancelGeneration(sessionID string, generationID uint64) error {
	return m.updateGeneration(sessionID, generationID, func(_ *PlaybackSession, generation *Generation) {
		generation.cancel()
	})
}

// updateGeneration is the only execution callback mutation path. It always
// locks Session before Generation, matching activation, heartbeat and cleanup
// paths and preventing lock-order inversion under concurrent FFmpeg callbacks.
func (m *Manager) updateGeneration(
	sessionID string,
	generationID uint64,
	update func(*PlaybackSession, *Generation),
) error {
	session, err := m.getSession(sessionID)
	if err != nil {
		return err
	}
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closing {
		return ErrSessionClosing
	}
	generation := session.generations[generationID]
	if generation == nil {
		return ErrGenerationNotFound
	}
	generation.mu.Lock()
	defer generation.mu.Unlock()
	update(session, generation)
	return nil
}
