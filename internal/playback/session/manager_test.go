package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestManagerCreateActivateAndClose(t *testing.T) {
	manager := newTestManager(t, Config{})

	created, err := manager.Create(context.Background(), CreateRequest{
		UserID:    "user-1",
		MediaID:   "media-1",
		ProfileID: "1080p",
	})
	require.NoError(t, err)
	require.Equal(t, SessionStateStarting, created.State)
	require.Equal(t, uint64(1), created.PendingGenerationID)

	activated, err := manager.ActivateGeneration(created.ID, 1)
	require.NoError(t, err)
	require.Equal(t, SessionStateReady, activated.State)
	require.Equal(t, uint64(1), activated.CurrentGenerationID)

	sessionDir := manager.sessionDirectory(created.ID)
	require.DirExists(t, sessionDir)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, manager.Close(ctx, created.ID, "playback_ended"))
	require.NoDirExists(t, sessionDir)
	_, err = manager.GetSnapshot(created.ID)
	require.ErrorIs(t, err, ErrSessionNotFound)
}

func TestCloseWaitsForActiveReader(t *testing.T) {
	manager := newTestManager(t, Config{CloseDrainTimeout: time.Second})
	created := createActiveSession(t, manager)

	lease, _, err := manager.AcquireReader(created.ID, created.CurrentGenerationID)
	require.NoError(t, err)

	closed := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		closed <- manager.Close(ctx, created.ID, "playback_ended")
	}()

	time.Sleep(50 * time.Millisecond)
	require.DirExists(t, manager.sessionDirectory(created.ID))
	lease.Release()
	require.NoError(t, <-closed)
	require.NoDirExists(t, manager.sessionDirectory(created.ID))
}

func TestGenerationSwitchDrainsPreviousReaders(t *testing.T) {
	manager := newTestManager(t, Config{CloseDrainTimeout: time.Second})
	created := createActiveSession(t, manager)
	oldGenerationDir := manager.generationDirectory(created.ID, created.CurrentGenerationID)

	lease, _, err := manager.AcquireReader(created.ID, created.CurrentGenerationID)
	require.NoError(t, err)

	pending, err := manager.BeginGeneration(created.ID, BeginGenerationRequest{
		ProfileID:       "720p",
		StartPositionMS: 3_600_000,
		Reason:          "seek",
	})
	require.NoError(t, err)
	require.Equal(t, uint64(2), pending.ID)

	activated, err := manager.ActivateGeneration(created.ID, pending.ID)
	require.NoError(t, err)
	require.Equal(t, pending.ID, activated.CurrentGenerationID)
	require.DirExists(t, oldGenerationDir)

	lease.Release()
	require.Eventually(t, func() bool {
		_, err := os.Stat(oldGenerationDir)
		return os.IsNotExist(err)
	}, time.Second, 10*time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, manager.Close(ctx, created.ID, "test_complete"))
}

func TestJanitorExpiresInactiveSession(t *testing.T) {
	manager := newTestManager(t, Config{
		ActiveTimeout: 40 * time.Millisecond,
		SweepInterval: 10 * time.Millisecond,
	})
	created := createActiveSession(t, manager)

	require.Eventually(t, func() bool {
		_, err := manager.GetSnapshot(created.ID)
		return err == ErrSessionNotFound
	}, time.Second, 10*time.Millisecond)
	require.NoDirExists(t, manager.sessionDirectory(created.ID))
}

func TestPausedSessionUsesLongerTimeout(t *testing.T) {
	manager := newTestManager(t, Config{
		ActiveTimeout: 30 * time.Millisecond,
		PausedTimeout: 300 * time.Millisecond,
		SweepInterval: 10 * time.Millisecond,
	})
	created := createActiveSession(t, manager)
	_, err := manager.Heartbeat(created.ID, Heartbeat{
		GenerationID: created.CurrentGenerationID,
		Paused:       true,
	})
	require.NoError(t, err)

	time.Sleep(80 * time.Millisecond)
	_, err = manager.GetSnapshot(created.ID)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, manager.Close(ctx, created.ID, "test_complete"))
}

func TestStartupRemovesOrphanSessions(t *testing.T) {
	root := t.TempDir()
	orphanDir := filepath.Join(root, "sessions", "orphan-session", "generations", "1")
	require.NoError(t, os.MkdirAll(orphanDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(orphanDir, "seg_000001.ts"), []byte("orphan"), 0o600))

	manager := newTestManager(t, Config{RootDir: root})
	require.NoDirExists(t, filepath.Join(root, "sessions", "orphan-session"))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, manager.Shutdown(ctx))
}

func TestCloseIsIdempotent(t *testing.T) {
	manager := newTestManager(t, Config{})
	created := createActiveSession(t, manager)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	require.NoError(t, manager.Close(ctx, created.ID, "first_close"))
	require.NoError(t, manager.Close(ctx, created.ID, "second_close"))
}

func newTestManager(t *testing.T, override Config) *Manager {
	t.Helper()
	if override.RootDir == "" {
		override.RootDir = t.TempDir()
	}
	if override.ActiveTimeout == 0 {
		override.ActiveTimeout = time.Hour
	}
	if override.PausedTimeout == 0 {
		override.PausedTimeout = time.Hour
	}
	if override.SweepInterval == 0 {
		override.SweepInterval = time.Hour
	}
	if override.CloseDrainTimeout == 0 {
		override.CloseDrainTimeout = time.Second
	}
	if override.CleanupRetries == 0 {
		override.CleanupRetries = 3
	}
	if override.CleanupRetryDelay == 0 {
		override.CleanupRetryDelay = time.Millisecond
	}

	manager, err := NewManager(override, zap.NewNop().Sugar())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})
	return manager
}

func createActiveSession(t *testing.T, manager *Manager) SessionSnapshot {
	t.Helper()
	created, err := manager.Create(context.Background(), CreateRequest{
		UserID:    "user-1",
		MediaID:   "media-1",
		ProfileID: "1080p",
	})
	require.NoError(t, err)
	activated, err := manager.ActivateGeneration(created.ID, created.PendingGenerationID)
	require.NoError(t, err)
	return activated
}
