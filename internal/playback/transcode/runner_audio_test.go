package transcode

import (
	"context"
	"testing"

	playbacksession "github.com/nowen-video/nowen-video/internal/playback/session"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	"github.com/stretchr/testify/require"
)

func TestGenerationAudioTrackIsEncodedIntoRollingHLSArgs(t *testing.T) {
	manager := newSessionManager(t)
	created, err := manager.Create(context.Background(), playbacksession.CreateRequest{
		UserID:     "user-audio",
		MediaID:    "media-audio",
		ProfileID:  "720p",
		AudioTrack: 2,
	})
	require.NoError(t, err)
	runtimeView, err := manager.Runtime(created.ID, created.PendingGenerationID)
	require.NoError(t, err)

	runner := &Runner{cfg: Config{
		SegmentDuration: 2,
		PlaylistWindow:  30,
		DeleteThreshold: 10,
		X264Preset:      "veryfast",
	}}
	args, err := runner.buildArgs(runtimeView, StartRequest{
		SessionID:       created.ID,
		GenerationID:    created.PendingGenerationID,
		InputPath:       "movie.mkv",
		ProfileID:       "720p",
		StartPositionMS: 12_000,
		FPS:             25,
	}, ffmpeg.HWAccelNone)
	require.NoError(t, err)
	requireArgValue(t, args, "-map", "0:v:0")
	require.Contains(t, args, "0:a:2?")
	requireArgValue(t, args, "-ss", "12.00")
}
