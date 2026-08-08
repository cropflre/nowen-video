package session

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestSessionSnapshotPrefersPendingAndLatestFailedGeneration(t *testing.T) {
	manager, err := NewManager(Config{
		RootDir:           t.TempDir(),
		ActiveTimeout:     time.Hour,
		PausedTimeout:     time.Hour,
		SweepInterval:     time.Hour,
		CloseDrainTimeout: time.Second,
		CleanupRetries:    3,
		CleanupRetryDelay: time.Millisecond,
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
	require.NoError(t, manager.MarkFirstSegmentReady(created.ID, 1))
	_, err = manager.ActivateGeneration(created.ID, 1)
	require.NoError(t, err)

	pending, err := manager.BeginGeneration(created.ID, BeginGenerationRequest{
		ProfileID:       "720p",
		StartPositionMS: 90_000,
		Reason:          "progress_seek",
	})
	require.NoError(t, err)

	snapshot, err := manager.GetSnapshot(created.ID)
	require.NoError(t, err)
	require.Equal(t, uint64(1), snapshot.CurrentGenerationID)
	require.Equal(t, pending.ID, snapshot.PendingGenerationID)
	require.NotNil(t, snapshot.Generation)
	require.Equal(t, pending.ID, snapshot.Generation.ID)
	require.Equal(t, GenerationStatePreparing, snapshot.Generation.State)

	require.NoError(t, manager.MarkGenerationFailed(
		created.ID,
		pending.ID,
		"first_segment_timeout",
		"first segment timed out",
	))

	failed, err := manager.GetSnapshot(created.ID)
	require.NoError(t, err)
	require.Equal(t, uint64(1), failed.CurrentGenerationID)
	require.Zero(t, failed.PendingGenerationID)
	require.NotNil(t, failed.Generation)
	require.Equal(t, pending.ID, failed.Generation.ID)
	require.Equal(t, GenerationStateFailed, failed.Generation.State)
	require.Equal(t, "first_segment_timeout", failed.Generation.ErrorCode)
	require.Equal(t, "first segment timed out", failed.Generation.ErrorMessage)
}
