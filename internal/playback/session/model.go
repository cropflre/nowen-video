package session

import (
	"context"
	"os"
	"sync"
	"time"
)

type SessionState string

const (
	SessionStateCreating SessionState = "creating"
	SessionStateStarting SessionState = "starting"
	SessionStateReady    SessionState = "ready"
	SessionStateActive   SessionState = "active"
	SessionStateClosing  SessionState = "closing"
	SessionStateClosed   SessionState = "closed"
	SessionStateFailed   SessionState = "failed"
	SessionStateExpired  SessionState = "expired"
)

type GenerationState string

const (
	GenerationStatePreparing GenerationState = "preparing"
	GenerationStateRunning   GenerationState = "running"
	GenerationStateCompleted GenerationState = "completed"
	GenerationStateDraining  GenerationState = "draining"
	GenerationStateRetired   GenerationState = "retired"
	GenerationStateFailed    GenerationState = "failed"
)

type CreateRequest struct {
	UserID          string
	MediaID         string
	ProfileID       string
	StartPositionMS int64
	AudioTrack      int
	SubtitleTrack   int
	BurnSubtitle    bool
	MaxBitrate      int
}

type BeginGenerationRequest struct {
	ProfileID       string
	StartPositionMS int64
	AudioTrack      int
	SubtitleTrack   int
	BurnSubtitle    bool
	MaxBitrate      int
	Reason          string
}

type Heartbeat struct {
	GenerationID  uint64
	PositionMS    int64
	BufferedEndMS int64
	Paused        bool
}

type GenerationSnapshot struct {
	ID              uint64          `json:"id"`
	SessionID       string          `json:"session_id"`
	State           GenerationState `json:"state"`
	ProfileID       string          `json:"profile_id"`
	StartPositionMS int64           `json:"start_position_ms"`
	AudioTrack      int             `json:"audio_track"`
	SubtitleTrack   int             `json:"subtitle_track"`
	BurnSubtitle    bool            `json:"burn_subtitle"`
	MaxBitrate      int             `json:"max_bitrate"`
	Reason          string          `json:"reason,omitempty"`
	OutputDir       string          `json:"-"`
	Backend         string          `json:"backend,omitempty"`
	ProcessPID      int             `json:"process_pid,omitempty"`
	TranscodedMS    int64           `json:"transcoded_ms"`
	AheadMS         int64           `json:"ahead_ms"`
	Suspended       bool            `json:"suspended"`
	Speed           string          `json:"speed,omitempty"`
	ErrorCode       string          `json:"error_code,omitempty"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	FirstSegmentAt  *time.Time      `json:"first_segment_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
}

type SessionSnapshot struct {
	ID                  string              `json:"id"`
	UserID              string              `json:"user_id"`
	MediaID             string              `json:"media_id"`
	State               SessionState        `json:"state"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
	LastSeen            time.Time           `json:"last_seen"`
	Paused              bool                `json:"paused"`
	PositionMS          int64               `json:"position_ms"`
	BufferedEndMS       int64               `json:"buffered_end_ms"`
	CurrentGenerationID uint64              `json:"current_generation_id,omitempty"`
	PendingGenerationID uint64              `json:"pending_generation_id,omitempty"`
	CloseReason         string              `json:"close_reason,omitempty"`
	Generation          *GenerationSnapshot `json:"generation,omitempty"`
}

type PlaybackSession struct {
	ID      string
	UserID  string
	MediaID string

	mu                  sync.RWMutex
	state               SessionState
	createdAt           time.Time
	updatedAt           time.Time
	lastSeen            time.Time
	paused              bool
	positionMS          int64
	bufferedEndMS       int64
	currentGenerationID uint64
	pendingGenerationID uint64
	generationCounter   uint64
	generations         map[uint64]*Generation
	closing             bool
	closeReason         string
	closed              chan struct{}
	closedOnce          sync.Once

	ctx    context.Context
	cancel context.CancelFunc
}

type Generation struct {
	ID        uint64
	SessionID string

	ProfileID       string
	StartPositionMS int64
	AudioTrack      int
	SubtitleTrack   int
	BurnSubtitle    bool
	MaxBitrate      int
	Reason          string
	OutputDir       string

	mu             sync.RWMutex
	state          GenerationState
	backend        string
	process        *os.Process
	processPID     int
	transcodedMS   int64
	aheadMS        int64
	suspended      bool
	speed          string
	errorCode      string
	errorMessage   string
	createdAt      time.Time
	updatedAt      time.Time
	startedAt      *time.Time
	firstSegmentAt *time.Time
	completedAt    *time.Time
	gate           *readerGate
	processGate    *readerGate
	ctx            context.Context
	cancel         context.CancelFunc
}

func (g *Generation) snapshot() GenerationSnapshot {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return GenerationSnapshot{
		ID:              g.ID,
		SessionID:       g.SessionID,
		State:           g.state,
		ProfileID:       g.ProfileID,
		StartPositionMS: g.StartPositionMS,
		AudioTrack:      g.AudioTrack,
		SubtitleTrack:   g.SubtitleTrack,
		BurnSubtitle:    g.BurnSubtitle,
		MaxBitrate:      g.MaxBitrate,
		Reason:          g.Reason,
		OutputDir:       g.OutputDir,
		Backend:         g.backend,
		ProcessPID:      g.processPID,
		TranscodedMS:    g.transcodedMS,
		AheadMS:         g.aheadMS,
		Suspended:       g.suspended,
		Speed:           g.speed,
		ErrorCode:       g.errorCode,
		ErrorMessage:    g.errorMessage,
		CreatedAt:       g.createdAt,
		UpdatedAt:       g.updatedAt,
		StartedAt:       cloneTime(g.startedAt),
		FirstSegmentAt:  cloneTime(g.firstSegmentAt),
		CompletedAt:     cloneTime(g.completedAt),
	}
}

func (s *PlaybackSession) snapshot() SessionSnapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Generation describes the latest timeline transition, not merely the
	// currently readable timeline. While a restart is preparing, clients must
	// observe that pending generation so the old readable generation cannot be
	// mistaken for restart success. If the latest restart fails, keep its
	// diagnostics visible even though the previous generation remains readable.
	generationID := s.pendingGenerationID
	if generationID == 0 {
		generationID = s.generationCounter
	}
	if generationID == 0 {
		generationID = s.currentGenerationID
	}
	var generation *GenerationSnapshot
	if current := s.generations[generationID]; current != nil {
		value := current.snapshot()
		generation = &value
	}

	return SessionSnapshot{
		ID:                  s.ID,
		UserID:              s.UserID,
		MediaID:             s.MediaID,
		State:               s.state,
		CreatedAt:           s.createdAt,
		UpdatedAt:           s.updatedAt,
		LastSeen:            s.lastSeen,
		Paused:              s.paused,
		PositionMS:          s.positionMS,
		BufferedEndMS:       s.bufferedEndMS,
		CurrentGenerationID: s.currentGenerationID,
		PendingGenerationID: s.pendingGenerationID,
		CloseReason:         s.closeReason,
		Generation:          generation,
	}
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}
