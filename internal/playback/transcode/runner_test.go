package transcode

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	playbacksession "github.com/nowen-video/nowen-video/internal/playback/session"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
	transcodeexecutor "github.com/nowen-video/nowen-video/internal/transcode/executor"
	transcodegovernor "github.com/nowen-video/nowen-video/internal/transcode/governor"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

type runtimeCall struct {
	Kind    transcodegovernor.Kind
	Command transcodeexecutor.Command
}

type fakeRuntime struct {
	mu       sync.Mutex
	calls    []runtimeCall
	behavior func(int, context.Context, transcodegovernor.Kind, transcodeexecutor.Command, transcodeexecutor.Callbacks) transcodeexecutor.Result
}

func (f *fakeRuntime) Run(
	ctx context.Context,
	kind transcodegovernor.Kind,
	command transcodeexecutor.Command,
	callbacks transcodeexecutor.Callbacks,
) transcodeexecutor.Result {
	f.mu.Lock()
	callIndex := len(f.calls)
	f.calls = append(f.calls, runtimeCall{Kind: kind, Command: command})
	behavior := f.behavior
	f.mu.Unlock()
	return behavior(callIndex, ctx, kind, command, callbacks)
}

func (f *fakeRuntime) snapshotCalls() []runtimeCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	calls := make([]runtimeCall, len(f.calls))
	copy(calls, f.calls)
	return calls
}

func TestRunnerPublishesFirstSegmentAndRunsUntilSessionClose(t *testing.T) {
	manager := newSessionManager(t)
	created := createSession(t, manager, "1080p")

	runtime := &fakeRuntime{}
	runtime.behavior = func(
		_ int,
		ctx context.Context,
		_ transcodegovernor.Kind,
		command transcodeexecutor.Command,
		callbacks transcodeexecutor.Callbacks,
	) transcodeexecutor.Result {
		if callbacks.OnStarted != nil {
			callbacks.OnStarted(nil)
		}
		require.NoError(t, materializeFirstSegment(command))
		if callbacks.OnProgress != nil {
			callbacks.OnProgress(transcodeexecutor.Progress{OutTimeMS: 2_000, Speed: "2.5x", State: "continue"})
		}
		<-ctx.Done()
		return transcodeexecutor.Result{Err: ctx.Err(), Cancelled: errors.Is(ctx.Err(), context.Canceled)}
	}

	runner := newTestRunner(t, manager, runtime, Config{})
	execution, err := runner.Start(context.Background(), StartRequest{
		SessionID:    created.ID,
		GenerationID: created.PendingGenerationID,
		InputPath:    "movie.mkv",
		ProfileID:    "1080p",
		FPS:          25,
	})
	require.NoError(t, err)

	readyCtx, readyCancel := context.WithTimeout(context.Background(), time.Second)
	defer readyCancel()
	ready, err := execution.WaitReady(readyCtx)
	require.NoError(t, err)
	require.Equal(t, playbacksession.SessionStateReady, ready.Session.State)
	require.Equal(t, uint64(1), ready.Session.CurrentGenerationID)
	require.NotNil(t, ready.Generation.FirstSegmentAt)

	calls := runtime.snapshotCalls()
	require.Len(t, calls, 1)
	require.Equal(t, transcodegovernor.KindSoftwareTranscode, calls[0].Kind)
	requireArgValue(t, calls[0].Command.Args, "-hls_list_size", "30")
	requireArgValue(t, calls[0].Command.Args, "-hls_delete_threshold", "10")
	requireArgValue(t, calls[0].Command.Args, "-hls_flags", "delete_segments+temp_file+independent_segments+program_date_time")
	requireArgValue(t, calls[0].Command.Args, "-g", "50")

	select {
	case <-execution.Done():
		t.Fatal("runner stopped after startup deadline instead of following session lifetime")
	case <-time.After(80 * time.Millisecond):
	}

	closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
	defer closeCancel()
	require.NoError(t, manager.Close(closeCtx, created.ID, "playback_ended"))

	select {
	case result := <-execution.Done():
		require.True(t, result.Cancelled)
	case <-time.After(time.Second):
		t.Fatal("runner did not stop after session close")
	}
}

func TestRunnerFallsBackToSoftwareBeforePublishingTimeline(t *testing.T) {
	manager := newSessionManager(t)
	created := createSession(t, manager, "720p")

	runtime := &fakeRuntime{}
	runtime.behavior = func(
		callIndex int,
		ctx context.Context,
		_ transcodegovernor.Kind,
		command transcodeexecutor.Command,
		callbacks transcodeexecutor.Callbacks,
	) transcodeexecutor.Result {
		if callIndex == 0 {
			return transcodeexecutor.Result{Err: errors.New("qsv device unavailable")}
		}
		if callbacks.OnStarted != nil {
			callbacks.OnStarted(nil)
		}
		require.NoError(t, materializeFirstSegment(command))
		<-ctx.Done()
		return transcodeexecutor.Result{Err: ctx.Err(), Cancelled: true}
	}

	runner := newTestRunner(t, manager, runtime, Config{
		HWAccel:             ffmpeg.HWAccelQSV,
		HardwareStartBudget: 100 * time.Millisecond,
		FirstSegmentTimeout: time.Second,
	})
	execution, err := runner.Start(context.Background(), StartRequest{
		SessionID:    created.ID,
		GenerationID: created.PendingGenerationID,
		InputPath:    "movie.mkv",
		ProfileID:    "720p",
	})
	require.NoError(t, err)

	readyCtx, readyCancel := context.WithTimeout(context.Background(), time.Second)
	defer readyCancel()
	ready, err := execution.WaitReady(readyCtx)
	require.NoError(t, err)
	require.Equal(t, ffmpeg.HWAccelNone, ready.Generation.Backend)

	calls := runtime.snapshotCalls()
	require.Len(t, calls, 2)
	require.Equal(t, transcodegovernor.KindHardwareTranscode, calls[0].Kind)
	require.Equal(t, transcodegovernor.KindSoftwareTranscode, calls[1].Kind)

	closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
	defer closeCancel()
	require.NoError(t, manager.Close(closeCtx, created.ID, "playback_ended"))
	select {
	case <-execution.Done():
	case <-time.After(time.Second):
		t.Fatal("software fallback did not stop after session close")
	}
}

func TestRunnerFailsWhenFirstSegmentNeverAppears(t *testing.T) {
	manager := newSessionManager(t)
	created := createSession(t, manager, "720p")

	runtime := &fakeRuntime{}
	runtime.behavior = func(
		_ int,
		ctx context.Context,
		_ transcodegovernor.Kind,
		_ transcodeexecutor.Command,
		_ transcodeexecutor.Callbacks,
	) transcodeexecutor.Result {
		<-ctx.Done()
		return transcodeexecutor.Result{Err: ctx.Err(), Cancelled: true}
	}

	runner := newTestRunner(t, manager, runtime, Config{
		FirstSegmentTimeout: 60 * time.Millisecond,
		FirstSegmentPoll:    5 * time.Millisecond,
	})
	execution, err := runner.Start(context.Background(), StartRequest{
		SessionID:    created.ID,
		GenerationID: created.PendingGenerationID,
		InputPath:    "movie.mkv",
		ProfileID:    "720p",
	})
	require.NoError(t, err)

	readyCtx, readyCancel := context.WithTimeout(context.Background(), time.Second)
	defer readyCancel()
	_, err = execution.WaitReady(readyCtx)
	require.Error(t, err)

	snapshot, snapshotErr := manager.GetSnapshot(created.ID)
	require.NoError(t, snapshotErr)
	require.Equal(t, playbacksession.SessionStateFailed, snapshot.State)
	require.NotNil(t, snapshot.Generation)
	require.Equal(t, "first_segment_timeout", snapshot.Generation.ErrorCode)

	closeCtx, closeCancel := context.WithTimeout(context.Background(), time.Second)
	defer closeCancel()
	require.NoError(t, manager.Close(closeCtx, created.ID, "test_complete"))
}

func newSessionManager(t *testing.T) *playbacksession.Manager {
	t.Helper()
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
	return manager
}

func createSession(t *testing.T, manager *playbacksession.Manager, profile string) playbacksession.SessionSnapshot {
	t.Helper()
	created, err := manager.Create(context.Background(), playbacksession.CreateRequest{
		UserID:    "user-1",
		MediaID:   "media-1",
		ProfileID: profile,
	})
	require.NoError(t, err)
	return created
}

func newTestRunner(t *testing.T, manager *playbacksession.Manager, runtime Runtime, override Config) *Runner {
	t.Helper()
	if override.FFmpegPath == "" {
		override.FFmpegPath = "ffmpeg"
	}
	if override.SegmentDuration == 0 {
		override.SegmentDuration = 2
	}
	if override.PlaylistWindow == 0 {
		override.PlaylistWindow = 30
	}
	if override.DeleteThreshold == 0 {
		override.DeleteThreshold = 10
	}
	if override.FirstSegmentTimeout == 0 {
		override.FirstSegmentTimeout = time.Second
	}
	if override.HardwareStartBudget == 0 {
		override.HardwareStartBudget = 100 * time.Millisecond
	}
	if override.FirstSegmentPoll == 0 {
		override.FirstSegmentPoll = 5 * time.Millisecond
	}
	runner, err := NewRunner(manager, runtime, override, zap.NewNop().Sugar())
	require.NoError(t, err)
	return runner
}

func materializeFirstSegment(command transcodeexecutor.Command) error {
	if len(command.Args) == 0 {
		return errors.New("missing ffmpeg arguments")
	}
	manifestPath := command.Args[len(command.Args)-1]
	outputDir := filepath.Dir(manifestPath)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return err
	}
	segmentName := "seg_000001.ts"
	if err := os.WriteFile(filepath.Join(outputDir, segmentName), []byte("segment"), 0o600); err != nil {
		return err
	}
	return os.WriteFile(
		manifestPath,
		[]byte("#EXTM3U\n#EXT-X-TARGETDURATION:2\n#EXTINF:2.0,\n"+segmentName+"\n"),
		0o600,
	)
}

func requireArgValue(t *testing.T, args []string, key, expected string) {
	t.Helper()
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key {
			require.Equal(t, expected, args[index+1])
			return
		}
	}
	t.Fatalf("argument %s not found in %v", key, args)
}
