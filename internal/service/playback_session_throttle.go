package service

// ReconcilePlaybackThrottle reevaluates the current Generation after a client
// heartbeat advances playback position. Progress callbacks handle suspension;
// heartbeats are the authoritative resume signal while FFmpeg is paused.
func (s *PlaybackSessionService) ReconcilePlaybackThrottle(sessionID string) error {
	if s == nil || s.manager == nil {
		return nil
	}
	return s.manager.ReconcileThrottle(sessionID)
}
