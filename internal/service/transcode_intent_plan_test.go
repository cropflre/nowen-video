package service

import (
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
)

func TestStartupIntentUsesDedicatedArtifactIdentityAndDuration(t *testing.T) {
	job := &TranscodeJob{
		Media:   &model.Media{ID: "media-startup", FilePath: "/media/movie.mkv", Duration: 7200},
		Quality: "720p",
		ExecutionJob: &model.TranscodeJobRecord{
			Intent:         string(transcodedomain.IntentStartupHLS),
			DurationMS:     startupStreamDurationMS,
			PlannerVersion: startupStreamPlannerVersion,
		},
	}
	if got := transcodeArtifactKind(job); got != startupStreamArtifactKind {
		t.Fatalf("startup job used runtime artifact kind: %s", got)
	}
	if got := transcodeArtifactDurationMS(job); got != startupStreamDurationMS {
		t.Fatalf("startup artifact duration mismatch: %d", got)
	}
}

func TestStartupIntentBuildsBoundedVODOutputPlan(t *testing.T) {
	service := &TranscodeService{}
	job := &TranscodeJob{
		Media:   &model.Media{ID: "media-startup", FilePath: "/media/movie.mkv", Duration: 7200},
		Quality: "720p",
		ExecutionJob: &model.TranscodeJobRecord{
			Intent:     string(transcodedomain.IntentStartupHLS),
			DurationMS: startupStreamDurationMS,
		},
	}
	// buildJobFFmpegArgs needs only fields consumed by the pure argument builder.
	service.cfg = testTranscodeConfig()
	args := service.buildJobFFmpegArgs(job, "/cache/workspaces/job/attempt/hls", "none")
	joined := strings.Join(args, " ")
	for _, expected := range []string{"-t 30.000", "-hls_playlist_type vod"} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("startup output plan missing %q: %s", expected, joined)
		}
	}
}

func testTranscodeConfig() *config.Config {
	return &config.Config{}
}
