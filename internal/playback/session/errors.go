package session

import "errors"

var (
	ErrManagerClosed       = errors.New("playback session manager is closed")
	ErrInvalidRequest      = errors.New("invalid playback session request")
	ErrSessionNotFound     = errors.New("playback session not found")
	ErrSessionClosing      = errors.New("playback session is closing")
	ErrGenerationNotFound  = errors.New("playback generation not found")
	ErrGenerationNotReady  = errors.New("playback generation is not ready")
	ErrGenerationNotActive = errors.New("playback generation is not active")
)
