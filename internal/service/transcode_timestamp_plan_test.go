package service

import (
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodedomain "github.com/nowen-video/nowen-video/internal/transcode/domain"
)

func timestampExecutionJob(t *testing.T, intent transcodedomain.Intent, startMS int64) *model.TranscodeJobRecord {
	t.Helper()
	identity, err := startupTimestampIdentity()
	if err != nil {
		t.Fatal(err)
	}
	planner := startupStreamPlannerVersion
	if intent == transcodedomain.IntentStartupContinuationHLS {
		planner = startupContinuationPlannerVersion
	}
	return &model.TranscodeJobRecord{
		Intent:               string(intent),
		PlannerVersion:       planner,
		StartMS:              startMS,
		TimelineOriginMS:     startMS,
		TimestampPlanVersion: identity.Version,
		TimestampPlanHash:    identity.Hash,
		TimestampPlanJSON:    identity.Canonical,
	}
}

func TestTimestampNormalizedContinuationFFmpegArgs(t *testing.T) {
	service := &TranscodeService{cfg: &config.Config{}}
	record := timestampExecutionJob(t, transcodedomain.IntentStartupContinuationHLS, 30_000)
	job := &TranscodeJob{
		Media:        &model.Media{ID: "media-timestamp", FilePath: "/media/movie.mkv", Duration: 7200},
		Quality:      "720p",
		startOffset:  30,
		ExecutionJob: record,
	}
	args, err := service.buildJobFFmpegArgsChecked(job, "/cache/workspace", "none")
	if err != nil {
		t.Fatal(err)
	}
	joined := strings.Join(args, " ")
	for _, expected := range []string{
		"-y -copyts -start_at_zero -ss 30.00",
		"-avoid_negative_ts disabled",
		"-fps_mode passthrough",
	} {
		if !strings.Contains(joined, expected) {
			t.Fatalf("timestamp plan missing %q: %s", expected, joined)
		}
	}
	if strings.Index(joined, "-copyts") > strings.Index(joined, "-i /media/movie.mkv") {
		t.Fatalf("copyts must be a global/input policy: %s", joined)
	}
}

func TestTimestampNormalizedAttemptRejectsHardwareBeforeExecution(t *testing.T) {
	record := timestampExecutionJob(t, transcodedomain.IntentStartupHLS, 0)
	if _, err := validateTimestampExecution(record, "qsv"); err == nil {
		t.Fatal("timestamp-normalized startup accepted uncertified hardware backend")
	}
	if _, err := validateTimestampExecution(record, "none"); err != nil {
		t.Fatalf("software backend rejected: %v", err)
	}
}

func TestTimestampExecutionRejectsOriginDifferentFromSeek(t *testing.T) {
	record := timestampExecutionJob(t, transcodedomain.IntentStartupContinuationHLS, 30_000)
	record.TimelineOriginMS = 0
	if _, err := validateTimestampExecution(record, "none"); err == nil {
		t.Fatal("continuation origin different from job seek was accepted")
	}
}

func TestArtifactTimestampAttestationRejectsResetContinuation(t *testing.T) {
	identity, err := startupTimestampIdentity()
	if err != nil {
		t.Fatal(err)
	}
	artifact := &model.TranscodeArtifactRecord{
		TimestampPlanVersion: identity.Version,
		TimestampPlanHash:    identity.Hash,
		TimestampPlanJSON:    identity.Canonical,
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
		t.Fatalf("normalized continuation evidence rejected: %v", err)
	}
	value.First.Timeline.Video.StartMS = 1400
	value.First.Timeline.Audio.StartMS = 1379
	if err := verifyArtifactTimestampContract(artifact, value); err == nil {
		t.Fatal("reset continuation evidence was accepted")
	}
}
