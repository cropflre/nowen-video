package session

import (
	"fmt"

	"github.com/nowen-video/nowen-video/internal/transcode/processcontrol"
)

// ReconcileThrottle applies the bounded lead window for the current generation.
// Progress callbacks call it to suspend a fast encoder; heartbeat callbacks call
// it to resume the encoder after the player consumes buffered media.
func (m *Manager) ReconcileThrottle(sessionID string) error {
	session, err := m.getSession(sessionID)
	if err != nil {
		return err
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closing || session.currentGenerationID == 0 {
		return nil
	}
	generation := session.generations[session.currentGenerationID]
	if generation == nil {
		return nil
	}

	generation.mu.Lock()
	defer generation.mu.Unlock()
	if generation.process == nil || generation.state != GenerationStateRunning {
		return nil
	}

	playbackPosition := session.positionMS
	if playbackPosition < generation.StartPositionMS {
		playbackPosition = generation.StartPositionMS
	}
	absoluteTranscoded := generation.StartPositionMS + generation.transcodedMS
	generation.aheadMS = absoluteTranscoded - playbackPosition
	if generation.aheadMS < 0 {
		generation.aheadMS = 0
	}

	highMS := m.cfg.AheadHighWatermark.Milliseconds()
	lowMS := m.cfg.AheadLowWatermark.Milliseconds()
	switch {
	case !generation.suspended && generation.aheadMS >= highMS:
		if err := processcontrol.Suspend(generation.process); err != nil {
			return fmt.Errorf("suspend playback transcode process: %w", err)
		}
		generation.suspended = true
		generation.updatedAt = m.now()
		m.logger.Debugw("playback transcode suspended",
			"session_id", sessionID,
			"generation_id", generation.ID,
			"ahead_ms", generation.aheadMS,
		)
	case generation.suspended && generation.aheadMS <= lowMS:
		if err := processcontrol.Resume(generation.process); err != nil {
			return fmt.Errorf("resume playback transcode process: %w", err)
		}
		generation.suspended = false
		generation.updatedAt = m.now()
		m.logger.Debugw("playback transcode resumed",
			"session_id", sessionID,
			"generation_id", generation.ID,
			"ahead_ms", generation.aheadMS,
		)
	}
	return nil
}

func (g *Generation) resumeIfSuspended() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.suspended || g.process == nil {
		g.suspended = false
		return nil
	}
	if err := processcontrol.Resume(g.process); err != nil {
		return err
	}
	g.suspended = false
	return nil
}
