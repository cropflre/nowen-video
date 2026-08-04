package service

import (
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
)

func TestDurableArtifactPlanHasNoStartupSpecialCase(t *testing.T) {
	job := &TranscodeJob{
		Media:   &model.Media{ID: "media-history", FilePath: "/media/movie.mkv", Duration: 7200},
		Quality: "720p",
		ExecutionJob: &model.TranscodeJobRecord{
			Intent:     "startup_hls",
			DurationMS: 30_000,
		},
	}
	if got := transcodeArtifactKind(job); got != "hls_variant" {
		t.Fatalf("durable executor retained a startup artifact kind: %s", got)
	}
	if got := transcodeArtifactDurationMS(job); got != 30_000 {
		t.Fatalf("historical execution duration projection changed: %d", got)
	}

	service := &TranscodeService{cfg: &config.Config{}}
	args := service.buildJobFFmpegArgs(job, "/cache/workspaces/job/attempt/hls", "none")
	joined := strings.Join(args, " ")
	for _, forbidden := range []string{"-t 30.000", "-hls_playlist_type vod"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("startup output semantics remain in durable executor: %s", joined)
		}
	}
}
