package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager owns temporary playback sessions. It intentionally keeps runtime
// state in memory: a process restart invalidates every playback connection, so
// recovering old sessions would only preserve orphaned files and stale FFmpeg
// work.
type Manager struct {
	mu        sync.RWMutex
	sessions  map[string]*PlaybackSession
	accepting bool

	cfg          Config
	sessionsRoot string
	deletingRoot string
	logger       *zap.SugaredLogger

	ctx         context.Context
	cancel      context.CancelFunc
	janitorDone chan struct{}

	now   func() time.Time
	newID func() string
}

func NewManager(cfg Config, logger *zap.SugaredLogger) (*Manager, error) {
	normalized, err := cfg.normalized()
	if err != nil {
		return nil, err
	}
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}

	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		sessions:     make(map[string]*PlaybackSession),
		accepting:    true,
		cfg:          normalized,
		sessionsRoot: filepath.Join(normalized.RootDir, "sessions"),
		deletingRoot: filepath.Join(normalized.RootDir, "deleting"),
		logger:       logger,
		ctx:          ctx,
		cancel:       cancel,
		janitorDone:  make(chan struct{}),
		now:          time.Now,
		newID:        uuid.NewString,
	}

	if err := os.MkdirAll(m.sessionsRoot, 0o755); err != nil {
		cancel()
		return nil, fmt.Errorf("create playback sessions root: %w", err)
	}
	if err := os.MkdirAll(m.deletingRoot, 0o755); err != nil {
		cancel()
		return nil, fmt.Errorf("create playback deleting root: %w", err)
	}
	if err := m.cleanupOrphans(); err != nil {
		cancel()
		return nil, fmt.Errorf("cleanup orphan playback sessions: %w", err)
	}

	go m.runJanitor()
	return m, nil
}

func (m *Manager) Create(ctx context.Context, req CreateRequest) (SessionSnapshot, error) {
	if err := validateCreateRequest(req); err != nil {
		return SessionSnapshot{}, err
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return SessionSnapshot{}, ctx.Err()
		default:
		}
	}

	m.mu.RLock()
	accepting := m.accepting
	m.mu.RUnlock()
	if !accepting {
		return SessionSnapshot{}, ErrManagerClosed
	}

	now := m.now()
	sessionID := m.newID()
	sessionDir := m.sessionDirectory(sessionID)
	generationDir := m.generationDirectory(sessionID, 1)
	if err := os.MkdirAll(generationDir, 0o755); err != nil {
		return SessionSnapshot{}, fmt.Errorf("create initial playback generation: %w", err)
	}

	sessionCtx, sessionCancel := context.WithCancel(m.ctx)
	generationCtx, generationCancel := context.WithCancel(sessionCtx)
	generation := &Generation{
		ID:              1,
		SessionID:       sessionID,
		ProfileID:       normalizedProfile(req.ProfileID),
		StartPositionMS: nonNegative(req.StartPositionMS),
		AudioTrack:      req.AudioTrack,
		SubtitleTrack:   req.SubtitleTrack,
		BurnSubtitle:    req.BurnSubtitle,
		MaxBitrate:      nonNegativeInt(req.MaxBitrate),
		Reason:          "initial_playback",
		OutputDir:       generationDir,
		state:           GenerationStatePreparing,
		createdAt:       now,
		updatedAt:       now,
		gate:            newReaderGate(),
		ctx:             generationCtx,
		cancel:          generationCancel,
	}
	session := &PlaybackSession{
		ID:                  sessionID,
		UserID:              req.UserID,
		MediaID:             req.MediaID,
		state:               SessionStateStarting,
		createdAt:           now,
		updatedAt:           now,
		lastSeen:            now,
		pendingGenerationID: generation.ID,
		generationCounter:   generation.ID,
		generations:         map[uint64]*Generation{generation.ID: generation},
		closed:              make(chan struct{}),
		ctx:                 sessionCtx,
		cancel:              sessionCancel,
	}

	m.mu.Lock()
	if !m.accepting {
		m.mu.Unlock()
		sessionCancel()
		_ = os.RemoveAll(sessionDir)
		return SessionSnapshot{}, ErrManagerClosed
	}
	m.sessions[sessionID] = session
	m.mu.Unlock()

	m.logger.Infow("playback session created",
		"session_id", sessionID,
		"media_id", req.MediaID,
		"user_id", req.UserID,
		"generation_id", generation.ID,
	)
	return session.snapshot(), nil
}

func (m *Manager) BeginGeneration(sessionID string, req BeginGenerationRequest) (GenerationSnapshot, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return GenerationSnapshot{}, err
	}

	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closing {
		return GenerationSnapshot{}, ErrSessionClosing
	}
	if session.pendingGenerationID != 0 {
		return GenerationSnapshot{}, fmt.Errorf("%w: generation %d is still preparing", ErrGenerationNotReady, session.pendingGenerationID)
	}

	profileID := normalizedProfile(req.ProfileID)
	if req.ProfileID == "" && session.currentGenerationID != 0 {
		if current := session.generations[session.currentGenerationID]; current != nil {
			profileID = current.ProfileID
		}
	}

	now := m.now()
	session.generationCounter++
	generationID := session.generationCounter
	outputDir := m.generationDirectory(sessionID, generationID)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		session.generationCounter--
		return GenerationSnapshot{}, fmt.Errorf("create playback generation: %w", err)
	}
	generationCtx, generationCancel := context.WithCancel(session.ctx)
	generation := &Generation{
		ID:              generationID,
		SessionID:       sessionID,
		ProfileID:       profileID,
		StartPositionMS: nonNegative(req.StartPositionMS),
		AudioTrack:      req.AudioTrack,
		SubtitleTrack:   req.SubtitleTrack,
		BurnSubtitle:    req.BurnSubtitle,
		MaxBitrate:      nonNegativeInt(req.MaxBitrate),
		Reason:          req.Reason,
		OutputDir:       outputDir,
		state:           GenerationStatePreparing,
		createdAt:       now,
		updatedAt:       now,
		gate:            newReaderGate(),
		ctx:             generationCtx,
		cancel:          generationCancel,
	}
	session.generations[generationID] = generation
	session.pendingGenerationID = generationID
	session.updatedAt = now
	return generation.snapshot(), nil
}

// ActivateGeneration atomically switches the session timeline only after the
// caller has produced a readable first segment. The previous generation stops
// accepting new readers and is removed after its in-flight readers drain.
func (m *Manager) ActivateGeneration(sessionID string, generationID uint64) (SessionSnapshot, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}

	now := m.now()
	var previous *Generation
	session.mu.Lock()
	if session.closing {
		session.mu.Unlock()
		return SessionSnapshot{}, ErrSessionClosing
	}
	generation := session.generations[generationID]
	if generation == nil {
		session.mu.Unlock()
		return SessionSnapshot{}, ErrGenerationNotFound
	}
	if session.pendingGenerationID != generationID {
		session.mu.Unlock()
		return SessionSnapshot{}, ErrGenerationNotReady
	}
	generation.mu.Lock()
	if generation.state != GenerationStatePreparing {
		generation.mu.Unlock()
		session.mu.Unlock()
		return SessionSnapshot{}, ErrGenerationNotReady
	}
	generation.state = GenerationStateRunning
	generation.updatedAt = now
	generation.mu.Unlock()

	if session.currentGenerationID != 0 && session.currentGenerationID != generationID {
		previous = session.generations[session.currentGenerationID]
		if previous != nil {
			previous.mu.Lock()
			previous.state = GenerationStateDraining
			previous.updatedAt = now
			previous.mu.Unlock()
		}
	}
	session.currentGenerationID = generationID
	session.pendingGenerationID = 0
	session.state = SessionStateReady
	session.updatedAt = now
	session.lastSeen = now
	snapshot := session.snapshotLocked()
	session.mu.Unlock()

	if previous != nil {
		go m.retireGeneration(session, previous)
	}
	return snapshot, nil
}

func (m *Manager) FailGeneration(sessionID string, generationID uint64, reason string) error {
	session, err := m.getSession(sessionID)
	if err != nil {
		return err
	}
	now := m.now()
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
	generation.state = GenerationStateFailed
	generation.updatedAt = now
	generation.cancel()
	generation.mu.Unlock()
	if session.pendingGenerationID == generationID {
		session.pendingGenerationID = 0
	}
	if session.currentGenerationID == generationID || session.currentGenerationID == 0 {
		session.state = SessionStateFailed
		session.closeReason = reason
	}
	session.updatedAt = now
	return nil
}

func (m *Manager) Heartbeat(sessionID string, heartbeat Heartbeat) (SessionSnapshot, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	now := m.now()
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closing {
		return SessionSnapshot{}, ErrSessionClosing
	}
	generationID := heartbeat.GenerationID
	if generationID == 0 {
		generationID = session.currentGenerationID
	}
	if session.currentGenerationID == 0 || generationID != session.currentGenerationID {
		return SessionSnapshot{}, ErrGenerationNotActive
	}
	session.positionMS = nonNegative(heartbeat.PositionMS)
	session.bufferedEndMS = nonNegative(heartbeat.BufferedEndMS)
	session.paused = heartbeat.Paused
	session.lastSeen = now
	session.updatedAt = now
	if session.state == SessionStateReady || session.state == SessionStateStarting {
		session.state = SessionStateActive
	}
	return session.snapshotLocked(), nil
}

// Touch refreshes liveness from playlist and segment requests. Client
// heartbeats are not the sole source of truth because browsers can delay timers
// in background tabs while media requests are still active.
func (m *Manager) Touch(sessionID string) error {
	session, err := m.getSession(sessionID)
	if err != nil {
		return err
	}
	now := m.now()
	session.mu.Lock()
	defer session.mu.Unlock()
	if session.closing {
		return ErrSessionClosing
	}
	session.lastSeen = now
	session.updatedAt = now
	return nil
}

func (m *Manager) AcquireReader(sessionID string, generationID uint64) (*ReaderLease, GenerationSnapshot, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return nil, GenerationSnapshot{}, err
	}

	session.mu.RLock()
	if session.closing {
		session.mu.RUnlock()
		return nil, GenerationSnapshot{}, ErrSessionClosing
	}
	if generationID == 0 || generationID != session.currentGenerationID {
		session.mu.RUnlock()
		return nil, GenerationSnapshot{}, ErrGenerationNotActive
	}
	generation := session.generations[generationID]
	if generation == nil {
		session.mu.RUnlock()
		return nil, GenerationSnapshot{}, ErrGenerationNotFound
	}
	generation.mu.RLock()
	active := generation.state == GenerationStateRunning
	generation.mu.RUnlock()
	if !active {
		session.mu.RUnlock()
		return nil, GenerationSnapshot{}, ErrGenerationNotActive
	}
	if err := generation.gate.acquire(); err != nil {
		session.mu.RUnlock()
		return nil, GenerationSnapshot{}, err
	}
	snapshot := generation.snapshot()
	session.mu.RUnlock()

	_ = m.Touch(sessionID)
	return &ReaderLease{release: generation.gate.release}, snapshot, nil
}

func (m *Manager) GetSnapshot(sessionID string) (SessionSnapshot, error) {
	session, err := m.getSession(sessionID)
	if err != nil {
		return SessionSnapshot{}, err
	}
	return session.snapshot(), nil
}

func (m *Manager) ListSnapshots() []SessionSnapshot {
	m.mu.RLock()
	sessions := make([]*PlaybackSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()

	snapshots := make([]SessionSnapshot, 0, len(sessions))
	for _, session := range sessions {
		snapshots = append(snapshots, session.snapshot())
	}
	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].CreatedAt.Before(snapshots[j].CreatedAt)
	})
	return snapshots
}

func (m *Manager) Close(ctx context.Context, sessionID, reason string) error {
	session, err := m.getSession(sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			return nil
		}
		return err
	}
	closed := m.startClose(session, SessionStateClosed, reason)
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) Shutdown(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}
	m.mu.Lock()
	m.accepting = false
	sessions := make([]*PlaybackSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.Unlock()

	m.cancel()
	closed := make([]<-chan struct{}, 0, len(sessions))
	for _, session := range sessions {
		closed = append(closed, m.startClose(session, SessionStateClosed, "server_shutdown"))
	}
	for _, done := range closed {
		select {
		case <-done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	select {
	case <-m.janitorDone:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *Manager) startClose(session *PlaybackSession, finalState SessionState, reason string) <-chan struct{} {
	now := m.now()
	session.mu.Lock()
	closed := session.closed
	if session.closing {
		session.mu.Unlock()
		return closed
	}
	session.closing = true
	session.state = SessionStateClosing
	session.closeReason = reason
	session.updatedAt = now
	session.cancel()
	session.mu.Unlock()

	go m.cleanupSession(session, finalState)
	return closed
}

func (m *Manager) runJanitor() {
	defer close(m.janitorDone)
	ticker := time.NewTicker(m.cfg.SweepInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.reapExpired()
			m.cleanupDeleting()
		}
	}
}

func (m *Manager) reapExpired() {
	now := m.now()
	m.mu.RLock()
	sessions := make([]*PlaybackSession, 0, len(m.sessions))
	for _, session := range m.sessions {
		sessions = append(sessions, session)
	}
	m.mu.RUnlock()

	for _, session := range sessions {
		session.mu.RLock()
		if session.closing {
			session.mu.RUnlock()
			continue
		}
		timeout := m.cfg.ActiveTimeout
		if session.paused {
			timeout = m.cfg.PausedTimeout
		}
		expired := now.Sub(session.lastSeen) >= timeout
		session.mu.RUnlock()
		if expired {
			m.startClose(session, SessionStateExpired, "heartbeat_timeout")
		}
	}
}

func (m *Manager) getSession(sessionID string) (*PlaybackSession, error) {
	m.mu.RLock()
	session := m.sessions[sessionID]
	m.mu.RUnlock()
	if session == nil {
		return nil, ErrSessionNotFound
	}
	return session, nil
}

func (m *Manager) sessionDirectory(sessionID string) string {
	return filepath.Join(m.sessionsRoot, sessionID)
}

func (m *Manager) generationDirectory(sessionID string, generationID uint64) string {
	return filepath.Join(m.sessionDirectory(sessionID), "generations", fmt.Sprintf("%d", generationID))
}

func (s *PlaybackSession) snapshotLocked() SessionSnapshot {
	var generation *GenerationSnapshot
	generationID := s.currentGenerationID
	if generationID == 0 {
		generationID = s.pendingGenerationID
	}
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

func validateCreateRequest(req CreateRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("%w: user id is required", ErrInvalidRequest)
	}
	if req.MediaID == "" {
		return fmt.Errorf("%w: media id is required", ErrInvalidRequest)
	}
	return nil
}

func normalizedProfile(profileID string) string {
	if profileID == "" {
		return "auto"
	}
	return profileID
}

func nonNegative(value int64) int64 {
	if value < 0 {
		return 0
	}
	return value
}

func nonNegativeInt(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
