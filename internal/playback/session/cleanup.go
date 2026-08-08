package session

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

func (m *Manager) cleanupSession(session *PlaybackSession, finalState SessionState) {
	startedAt := m.now()
	session.mu.RLock()
	generations := make([]*Generation, 0, len(session.generations))
	for _, generation := range session.generations {
		generations = append(generations, generation)
	}
	reason := session.closeReason
	session.mu.RUnlock()

	for _, generation := range generations {
		generation.cancel()
	}

	// Process completion is a hard filesystem safety boundary. Do not remove a
	// generation while FFmpeg or its hardware/software fallback runner can still
	// write into it. The caller-facing Close may time out, but this cleanup
	// goroutine remains responsible until every process lease is released.
	for _, generation := range generations {
		if err := generation.waitForProcessExit(context.Background()); err != nil {
			m.logger.Warnw("wait for playback generation process failed",
				"session_id", session.ID,
				"generation_id", generation.ID,
				"error", err,
			)
		}
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), m.cfg.CloseDrainTimeout)
	for _, generation := range generations {
		if err := generation.gate.closeAndWait(drainCtx); err != nil && !errors.Is(err, context.DeadlineExceeded) {
			m.logger.Warnw("wait for playback generation readers failed",
				"session_id", session.ID,
				"generation_id", generation.ID,
				"error", err,
			)
		}
	}
	cancel()

	if err := m.moveAndRemove(m.sessionDirectory(session.ID), "session-"+session.ID); err != nil {
		m.logger.Errorw("remove playback session directory failed",
			"session_id", session.ID,
			"error", err,
		)
	}

	now := m.now()
	for _, generation := range generations {
		generation.mu.Lock()
		generation.state = GenerationStateRetired
		generation.updatedAt = now
		generation.mu.Unlock()
	}

	m.mu.Lock()
	if m.sessions[session.ID] == session {
		delete(m.sessions, session.ID)
	}
	m.mu.Unlock()

	session.mu.Lock()
	session.state = finalState
	session.updatedAt = now
	session.closedOnce.Do(func() { close(session.closed) })
	session.mu.Unlock()

	m.logger.Infow("playback session closed",
		"session_id", session.ID,
		"media_id", session.MediaID,
		"state", finalState,
		"reason", reason,
		"cleanup_ms", m.now().Sub(startedAt).Milliseconds(),
	)
}

func (m *Manager) retireGeneration(session *PlaybackSession, generation *Generation) {
	generation.cancel()
	if err := generation.waitForProcessExit(context.Background()); err != nil {
		m.logger.Warnw("wait for retired generation process failed",
			"session_id", session.ID,
			"generation_id", generation.ID,
			"error", err,
		)
	}

	drainCtx, cancel := context.WithTimeout(context.Background(), m.cfg.CloseDrainTimeout)
	err := generation.gate.closeAndWait(drainCtx)
	cancel()
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		m.logger.Warnw("wait for retired generation readers failed",
			"session_id", session.ID,
			"generation_id", generation.ID,
			"error", err,
		)
	}

	if err := m.moveAndRemove(
		generation.OutputDir,
		fmt.Sprintf("generation-%s-%d", session.ID, generation.ID),
	); err != nil {
		m.logger.Errorw("remove retired playback generation failed",
			"session_id", session.ID,
			"generation_id", generation.ID,
			"error", err,
		)
	}

	now := m.now()
	generation.mu.Lock()
	generation.state = GenerationStateRetired
	generation.updatedAt = now
	generation.mu.Unlock()

	session.mu.Lock()
	if session.generations[generation.ID] == generation && session.currentGenerationID != generation.ID {
		delete(session.generations, generation.ID)
	}
	session.updatedAt = now
	session.mu.Unlock()
}

// cleanupOrphans removes every runtime session left by a previous process. A
// playback connection cannot survive server restart, so retaining those files
// has no valid recovery semantics.
func (m *Manager) cleanupOrphans() error {
	entries, err := os.ReadDir(m.sessionsRoot)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		path := filepath.Join(m.sessionsRoot, entry.Name())
		if err := m.moveAndRemove(path, "orphan-"+entry.Name()); err != nil {
			m.logger.Warnw("remove orphan playback session failed", "path", path, "error", err)
		}
	}
	m.cleanupDeleting()
	return nil
}

func (m *Manager) cleanupDeleting() {
	entries, err := os.ReadDir(m.deletingRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			m.logger.Warnw("scan playback deleting directory failed", "error", err)
		}
		return
	}
	for _, entry := range entries {
		path := filepath.Join(m.deletingRoot, entry.Name())
		if err := m.removeWithRetry(path); err != nil {
			m.logger.Warnw("retry playback temporary cleanup failed", "path", path, "error", err)
		}
	}
}

// moveAndRemove first removes a path from the live namespace and then deletes
// it. Rename is atomic on the same filesystem and prevents late playlist or
// segment requests from reopening files while cleanup is in progress.
func (m *Manager) moveAndRemove(path, prefix string) error {
	if path == "" {
		return nil
	}
	if err := ensureChildPath(m.cfg.RootDir, path); err != nil {
		return err
	}
	if err := os.MkdirAll(m.deletingRoot, 0o755); err != nil {
		return err
	}

	target := filepath.Join(m.deletingRoot, prefix+"-"+uuid.NewString())
	if err := os.Rename(path, target); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		// Cross-device mounts and Windows/NAS file semantics can reject rename.
		// Process and Reader gates have already stopped writes and new opens, so
		// bounded direct removal remains safe.
		return m.removeWithRetry(path)
	}
	return m.removeWithRetry(target)
}

func (m *Manager) removeWithRetry(path string) error {
	var lastErr error
	for attempt := 0; attempt < m.cfg.CleanupRetries; attempt++ {
		if err := os.RemoveAll(path); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if attempt+1 < m.cfg.CleanupRetries {
			time.Sleep(m.cfg.CleanupRetryDelay)
		}
	}
	return lastErr
}

func ensureChildPath(root, path string) error {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return fmt.Errorf("resolve cleanup path: %w", err)
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		return fmt.Errorf("refuse to remove path outside playback root: %s", path)
	}
	return nil
}
