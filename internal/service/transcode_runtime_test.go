package service

import (
	"context"
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/service/ffmpeg"
)

func TestTranscodeJobCancellationIsPersistentAndIdempotent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	job := &TranscodeJob{
		ctx:          ctx,
		cancel:       cancel,
		throttleDone: make(chan struct{}),
	}

	job.RequestCancel()
	job.RequestCancel()
	if !job.CancellationRequested() {
		t.Fatal("cancellation must remain observable after the first request")
	}
	if ctx.Err() != context.Canceled {
		t.Fatalf("expected context cancellation, got %v", ctx.Err())
	}
}

func TestBuildFFmpegArgsForBackendDoesNotMutateDetectedBackend(t *testing.T) {
	service := &TranscodeService{
		cfg:     &config.Config{},
		hwAccel: ffmpeg.HWAccelQSV,
	}
	media := &model.Media{VideoCodec: "h264"}
	args := service.buildFFmpegArgsForBackend(media, "/media/input.mkv", "/cache/output", "720p", 0, ffmpeg.HWAccelNone)

	if service.hwAccel != ffmpeg.HWAccelQSV {
		t.Fatalf("attempt backend must not mutate service backend: %s", service.hwAccel)
	}
	joined := strings.Join(args, " ")
	for _, expected := range []string{"-progress pipe:2", "-nostats"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("missing machine progress option %q in %s", expected, joined)
		}
	}
}

func TestWithMachineProgressPreservesOutputAsLastArgument(t *testing.T) {
	args := withMachineProgress([]string{"-i", "input", "output.m3u8"})
	if got := args[len(args)-1]; got != "output.m3u8" {
		t.Fatalf("output must remain the last FFmpeg argument, got %q", got)
	}
	joined := strings.Join(args, " ")
	if !strings.Contains(joined, "-progress pipe:2 -nostats output.m3u8") {
		t.Fatalf("unexpected arguments: %s", joined)
	}
}
