package service

import (
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func TestDurableRuntimeJobDoesNotApplyStartupTimestampPolicy(t *testing.T) {
	service := &TranscodeService{cfg: &config.Config{}, hwAccel: "qsv"}
	record := &model.TranscodeJobRecord{
		Intent:               "startup_continuation_hls",
		PlannerVersion:       "startup-continuation-hls-v4",
		StartMS:              30_000,
		TimelineOriginMS:     30_000,
		TimestampPlanVersion: "historical",
		TimestampPlanHash:    "historical",
		TimestampPlanJSON:    "{}",
	}
	job := &TranscodeJob{
		Media:        &model.Media{ID: "media-history", FilePath: "/media/movie.mkv", Duration: 7200},
		Quality:      "720p",
		startOffset:  30,
		ExecutionJob: record,
	}
	args, err := service.buildJobFFmpegArgsChecked(job, "/cache/workspace", "none")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, forbidden := range []string{"-copyts", "-start_at_zero", "-avoid_negative_ts disabled"} {
		if strings.Contains(joined, forbidden) {
			t.Fatalf("retired startup timestamp policy remains in durable args: %s", joined)
		}
	}
	if timestampNormalizationRequired(record) {
		t.Fatal("retired runtime job still requests timestamp normalization")
	}
	if got := service.preferredAttemptBackend(job); got != "qsv" {
		t.Fatalf("historical runtime record changed backend selection: %q", got)
	}
}

func TestArtifactTimestampAttestationStillValidatesHistoricalEvidence(t *testing.T) {
	plan := transcodetimestamp.Default()
	version, hash, canonical, err := transcodetimestamp.Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	artifact := &model.TranscodeArtifactRecord{
		TimestampPlanVersion: version,
		TimestampPlanHash:    hash,
		TimestampPlanJSON:    canonical,
		TimelineOriginMS:     30_000,
	}
	value := transcodeattestation.Attestation{
		First: transcodeattestation.SegmentEvidence{
			Timeline: transcodeattestation.Timeline{
				Video: transcodeattestation.PacketRange{StartMS: 31_400},
				Audio: transcodeattestation.PacketRange{StartMS: 31_379},
			},
		},
	}
	if err := verifyArtifactTimestampContract(artifact, value); err != nil {
		t.Fatalf("historical normalized evidence rejected: %v", err)
	}
	value.First.Timeline.Video.StartMS = 1400
	value.First.Timeline.Audio.StartMS = 1379
	if err := verifyArtifactTimestampContract(artifact, value); err == nil {
		t.Fatal("reset historical evidence was accepted")
	}
}
