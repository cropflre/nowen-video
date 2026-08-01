package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestPublishedArtifactExecutionContractRequiresTimestampIdentityAndOrigin(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	now := time.Date(2026, 8, 1, 6, 0, 0, 0, time.UTC)
	artifact := &model.TranscodeArtifactRecord{
		JobID:                "legacy:timestamp-published",
		MediaID:              "media-timestamp-published",
		Kind:                 "startup_continuation_hls",
		ProfileID:            "720p",
		SourceFingerprint:    "source-timestamp-published",
		PlannerVersion:       "startup-continuation-hls-v4",
		EncodingPlanVersion:  "hls-encoding-plan-v1",
		EncodingPlanHash:     "encoding-plan",
		EncodingPlanJSON:     `{"plan":"encoding"}`,
		TimestampPlanVersion: "hls-timestamp-normalization-v1",
		TimestampPlanHash:    "timestamp-plan",
		TimestampPlanJSON:    `{"plan":"timestamp"}`,
		TimelineOriginMS:     30_000,
		AttestationVersion:   "hls-produced-media-attestation-v1",
		AttestationStatus:    "verified",
		AttestationHash:      "attestation",
		AttestationJSON:      `{"scope":"complete"}`,
		Path:                 "/cache/published/timestamp",
		ManifestPath:         "/cache/published/timestamp/stream.m3u8",
		Status:               "published",
		PublishedAt:          &now,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	resolved, err := repo.FindPublishedArtifactByExecutionContract(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		artifact.Kind,
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		artifact.TimestampPlanVersion,
		artifact.TimestampPlanHash,
		artifact.TimelineOriginMS,
	)
	if err != nil || resolved.ID != artifact.ID {
		t.Fatalf("exact execution contract did not resolve: artifact=%+v err=%v", resolved, err)
	}
	if _, err := repo.FindPublishedArtifactByExecutionContract(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		artifact.Kind,
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		artifact.TimestampPlanVersion,
		"different-timestamp-plan",
		artifact.TimelineOriginMS,
	); !IsNotFound(err) {
		t.Fatalf("mismatched timestamp plan remained readable: %v", err)
	}
	if _, err := repo.FindPublishedArtifactByExecutionContract(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		artifact.Kind,
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
		artifact.TimestampPlanVersion,
		artifact.TimestampPlanHash,
		0,
	); !IsNotFound(err) {
		t.Fatalf("mismatched timeline origin remained readable: %v", err)
	}
}

func TestReadableArtifactExecutionContractFencesJobAndArtifactIdentity(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	activeKey := "timestamp-active"
	job := &model.TranscodeJobRecord{
		MediaID:              "media-timestamp-active",
		Intent:               "startup_continuation_hls",
		ProfileID:            "720p",
		StartMS:              30_000,
		Status:               "queued",
		DesiredState:         "running",
		ActiveKey:            &activeKey,
		SourceFingerprint:    "source-timestamp-active",
		PlannerVersion:       "startup-continuation-hls-v4",
		EncodingPlanVersion:  "hls-encoding-plan-v1",
		EncodingPlanHash:     "encoding-plan",
		EncodingPlanJSON:     `{"plan":"encoding"}`,
		TimestampPlanVersion: "hls-timestamp-normalization-v1",
		TimestampPlanHash:    "timestamp-plan",
		TimestampPlanJSON:    `{"plan":"timestamp"}`,
		TimelineOriginMS:     30_000,
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 1, 6, 30, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "worker-timestamp", now, time.Minute)
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
		TempPath:          "/cache/workspaces/timestamp",
		Status:            "staging",
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}
	attestedAt := now.Add(2 * time.Second)
	stored, err := repo.RecordOwnedArtifactAttestation(
		job.ID,
		attempt.ID,
		artifact.ID,
		claimed.LeaseToken,
		"hls-produced-media-attestation-v1",
		"provisional",
		"attestation",
		`{"scope":"first_segment"}`,
		31_400,
		33_400,
		attestedAt,
	)
	if err != nil || !stored {
		t.Fatalf("record attestation: stored=%v err=%v", stored, err)
	}
	resolved, err := repo.FindReadableArtifactByExecutionContract(
		job.MediaID,
		job.ProfileID,
		job.SourceFingerprint,
		job.PlannerVersion,
		artifact.Kind,
		job.EncodingPlanVersion,
		job.EncodingPlanHash,
		job.TimestampPlanVersion,
		job.TimestampPlanHash,
		job.TimelineOriginMS,
		now.Add(3*time.Second),
	)
	if err != nil || resolved.ID != artifact.ID {
		t.Fatalf("current timestamp-fenced artifact was not resolved: artifact=%+v err=%v", resolved, err)
	}
	if err := repo.db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ?", artifact.ID).
		Update("timeline_origin_ms", 0).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := repo.FindReadableArtifactByExecutionContract(
		job.MediaID,
		job.ProfileID,
		job.SourceFingerprint,
		job.PlannerVersion,
		artifact.Kind,
		job.EncodingPlanVersion,
		job.EncodingPlanHash,
		job.TimestampPlanVersion,
		job.TimestampPlanHash,
		job.TimelineOriginMS,
		now.Add(3*time.Second),
	); !IsNotFound(err) {
		t.Fatalf("job/artifact origin mismatch remained readable: %v", err)
	}
}
