package service

import (
	"context"
	"testing"
	"time"

	playbacksession "github.com/nowen-video/nowen-video/internal/playback/session"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestPlaybackSessionResultDoesNotPublishOldGenerationDuringRestart(t *testing.T) {
	manager, err := playbacksession.NewManager(playbacksession.Config{
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

	created, err := manager.Create(context.Background(), playbacksession.CreateRequest{
		UserID:    "user-1",
		MediaID:   "media-1",
		ProfileID: "720p",
	})
	require.NoError(t, err)
	require.NoError(t, manager.MarkFirstSegmentReady(created.ID, 1))
	active, err := manager.ActivateGeneration(created.ID, 1)
	require.NoError(t, err)

	service := &PlaybackSessionService{
		manager:   manager,
		heartbeat: defaultPlaybackHeartbeatInterval,
	}
	initial := service.result(active, 0)
	require.True(t, initial.FirstSegmentReady)
	require.Contains(t, initial.PlaylistURL, "/generations/1/")

	pending, err := manager.BeginGeneration(created.ID, playbacksession.BeginGenerationRequest{
		ProfileID:       "720p",
		StartPositionMS: 120_000,
		Reason:          "progress_seek",
	})
	require.NoError(t, err)

	preparingSnapshot, err := manager.GetSnapshot(created.ID)
	require.NoError(t, err)
	preparing := service.result(preparingSnapshot, 0)
	require.False(t, preparing.FirstSegmentReady)
	require.Empty(t, preparing.PlaylistURL)
	require.Equal(t, pending.ID, preparing.Session.PendingGenerationID)
	require.NotNil(t, preparing.Session.Generation)
	require.Equal(t, pending.ID, preparing.Session.Generation.ID)

	require.NoError(t, manager.MarkFirstSegmentReady(created.ID, pending.ID))
	restartedSnapshot, err := manager.ActivateGeneration(created.ID, pending.ID)
	require.NoError(t, err)
	restarted := service.result(restartedSnapshot, 0)
	require.True(t, restarted.FirstSegmentReady)
	require.Contains(t, restarted.PlaylistURL, "/generations/2/")
}
