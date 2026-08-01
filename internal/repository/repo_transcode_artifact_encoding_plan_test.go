package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestReadableArtifactByEncodingPlanRejectsMismatchedCurrentAttempt(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	activeKey := "encoding-plan-active"
	job := &model.TranscodeJobRecord{
		MediaID:             "media-plan",
		Intent:              "startup_continuation_hls",
		ProfileID:           "720p",
		Status:              "queued",
		DesiredState:        "running",
		ActiveKey:           &activeKey,
		SourceFingerprint:   "source-plan",
		PlannerVersion:      "startup-continuation-hls-v2",
		EncodingPlanVersion: "hls-encoding-plan-v1",
		EncodingPlanHash:    "plan-a",
		EncodingPlanJSON:    `{"plan":"a"}`,
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 1, 1, 0, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "worker-plan", now, time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	attempt := &model.TranscodeAttemptRecord{JobID: job.ID, Number: 1, Status: "running", ExitCode: -1}
	if err := repo.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	if running, err := repo.SetJobRunning(job.ID, attempt.ID, claimed.LeaseToken, now.Add(time.Second)); err != nil || !running {
		t.Fatalf("running=%v err=%v", running, err)
	}
	artifact := &model.TranscodeArtifactRecord{
		JobID:             job.ID,
		AttemptID:         attempt.ID,
		MediaID:           job.MediaID,
		Kind:              "startup_continuation_hls",
		ProfileID:         job.ProfileID,
		SourceFingerprint: job.SourceFingerprint,
		PlannerVersion:    job.PlannerVersion,
		TempPath:          "/cache/workspaces/plan",
		Status:            "staging",
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.EncodingPlanHash != "plan-a" {
		t.Fatalf("artifact did not inherit encoding plan: %+v", artifact)
	}

	resolved, err := repo.FindReadableArtifactByEncodingPlan(
		job.MediaID,
		job.ProfileID,
		job.SourceFingerprint,
		job.PlannerVersion,
		artifact.Kind,
		job.EncodingPlanVersion,
		job.EncodingPlanHash,
		now.Add(2*time.Second),
	)
	if err != nil || resolved.ID != artifact.ID {
		t.Fatalf("matching plan was not resolved: artifact=%+v err=%v", resolved, err)
	}
	if _, err := repo.FindReadableArtifactByEncodingPlan(
		job.MediaID,
		job.ProfileID,
		job.SourceFingerprint,
		job.PlannerVersion,
		artifact.Kind,
		job.EncodingPlanVersion,
		"plan-b",
		now.Add(2*time.Second),
	); !IsNotFound(err) {
		t.Fatalf("mismatched plan remained readable: %v", err)
	}
}
