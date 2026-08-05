package session

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestReconcileThrottleSuspendsAndResumesCurrentGeneration(t *testing.T) {
	originalSuspend := suspendGenerationProcess
	originalResume := resumeGenerationProcess
	t.Cleanup(func() {
		suspendGenerationProcess = originalSuspend
		resumeGenerationProcess = originalResume
	})

	suspendCalls := 0
	resumeCalls := 0
	suspendGenerationProcess = func(*os.Process) error {
		suspendCalls++
		return nil
	}
	resumeGenerationProcess = func(*os.Process) error {
		resumeCalls++
		return nil
	}

	manager, err := NewManager(Config{
		RootDir:            t.TempDir(),
		ActiveTimeout:      time.Hour,
		PausedTimeout:      time.Hour,
		SweepInterval:      time.Hour,
		CloseDrainTimeout:  time.Second,
		AheadHighWatermark: 5 * time.Second,
		AheadLowWatermark:  2 * time.Second,
		CleanupRetries:     2,
		CleanupRetryDelay:  time.Millisecond,
	}, zap.NewNop().Sugar())
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = manager.Shutdown(ctx)
	})

	created, err := manager.Create(context.Background(), CreateRequest{
		UserID:    "user-1",
		MediaID:   "media-1",
		ProfileID: "720p",
	})
	require.NoError(t, err)
	active, err := manager.ActivateGeneration(created.ID, created.PendingGenerationID)
	require.NoError(t, err)
	require.NoError(t, manager.MarkGenerationStarted(created.ID, active.CurrentGenerationID, "none", 999999))

	require.NoError(t, manager.MarkGenerationProgress(created.ID, active.CurrentGenerationID, 6_000, "4.0x"))
	require.Equal(t, 1, suspendCalls)
	require.Equal(t, 0, resumeCalls)

	snapshot, err := manager.GetSnapshot(created.ID)
	require.NoError(t, err)
	require.NotNil(t, snapshot.Generation)
	require.True(t, snapshot.Generation.Suspended)
	require.Equal(t, int64(6_000), snapshot.Generation.AheadMS)

	_, err = manager.Heartbeat(created.ID, Heartbeat{
		GenerationID:  active.CurrentGenerationID,
		PositionMS:    4_500,
		BufferedEndMS: 6_000,
	})
	require.NoError(t, err)
	require.NoError(t, manager.ReconcileThrottle(created.ID))
	require.Equal(t, 1, suspendCalls)
	require.Equal(t, 1, resumeCalls)

	snapshot, err = manager.GetSnapshot(created.ID)
	require.NoError(t, err)
	require.False(t, snapshot.Generation.Suspended)
	require.Equal(t, int64(1_500), snapshot.Generation.AheadMS)
}
