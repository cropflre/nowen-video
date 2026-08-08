package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestReadableArtifactByEncodingPlanRequiresAttestedCurrentAttempt(t *testing.T) {
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
		PlannerVersion:      "startup-continuation-hls-v3",
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

	if _, err := repo.FindReadableArtifactByEncodingPlan(
		job.MediaID,
		job.ProfileID,
		job.SourceFingerprint,
		job.PlannerVersion,
		artifact.Kind,
		job.EncodingPlanVersion,
		job.EncodingPlanHash,
		now.Add(2*time.Second),
	); !IsNotFound(err) {
		t.Fatalf("unattested active artifact became readable: %v", err)
	}

	attestedAt := now.Add(2 * time.Second)
	stored, err := repo.RecordOwnedArtifactAttestation(
		job.ID,
		attempt.ID,
		artifact.ID,
		claimed.LeaseToken,
		"hls-produced-media-attestation-v1",
		"provisional",
		"attestation-a",
		`{"scope":"first_segment"}`,
		1400,
		3400,
		attestedAt,
	)
	if err != nil || !stored {
		t.Fatalf("record attestation: stored=%v err=%v", stored, err)
	}

	resolved, err := repo.FindReadableArtifactByEncodingPlan(
		job.MediaID,
		job.ProfileID,
		job.SourceFingerprint,
		job.PlannerVersion,
		artifact.Kind,
		job.EncodingPlanVersion,
		job.EncodingPlanHash,
		now.Add(3*time.Second),
	)
	if err != nil || resolved.ID != artifact.ID {
		t.Fatalf("matching attested plan was not resolved: artifact=%+v err=%v", resolved, err)
	}
	if resolved.AttestationStatus != "provisional" || resolved.TimelineStartMS != 1400 || resolved.TimelineEndMS != 3400 {
		t.Fatalf("attestation evidence was not persisted: %+v", resolved)
	}
	if _, err := repo.FindReadableArtifactByEncodingPlan(
		job.MediaID,
		job.ProfileID,
		job.SourceFingerprint,
		job.PlannerVersion,
		artifact.Kind,
		job.EncodingPlanVersion,
		"plan-b",
		now.Add(3*time.Second),
	); !IsNotFound(err) {
		t.Fatalf("mismatched plan remained readable: %v", err)
	}
}

func TestPublishedArtifactByEncodingPlanRequiresVerifiedAttestation(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	now := time.Date(2026, 8, 1, 1, 30, 0, 0, time.UTC)
	artifact := &model.TranscodeArtifactRecord{
		JobID:               "legacy:published-plan",
		MediaID:             "media-published-plan",
		Kind:                "startup_hls",
		ProfileID:           "720p",
		SourceFingerprint:   "source-published-plan",
		PlannerVersion:      "startup-hls-v2",
		EncodingPlanVersion: "hls-encoding-plan-v1",
		EncodingPlanHash:    "plan-published",
		EncodingPlanJSON:    `{"plan":"published"}`,
		Path:                "/cache/published/plan",
		ManifestPath:        "/cache/published/plan/stream.m3u8",
		Status:              "published",
		PublishedAt:         &now,
		CreatedAt:           now,
		UpdatedAt:           now,
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}

	if _, err := repo.FindPublishedArtifactByEncodingPlan(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		artifact.Kind,
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
	); !IsNotFound(err) {
		t.Fatalf("unattested published artifact became readable: %v", err)
	}

	if err := repo.db.Model(&model.TranscodeArtifactRecord{}).
		Where("id = ?", artifact.ID).
		Updates(map[string]any{
			"attestation_version": "hls-produced-media-attestation-v1",
			"attestation_status":  "verified",
			"attestation_hash":    "attestation-published",
			"attestation_json":    `{"scope":"complete"}`,
			"timeline_start_ms":   1400,
			"timeline_end_ms":     31400,
			"attested_at":         now,
		}).Error; err != nil {
		t.Fatal(err)
	}

	resolved, err := repo.FindPublishedArtifactByEncodingPlan(
		artifact.MediaID,
		artifact.ProfileID,
		artifact.SourceFingerprint,
		artifact.PlannerVersion,
		artifact.Kind,
		artifact.EncodingPlanVersion,
		artifact.EncodingPlanHash,
	)
	if err != nil || resolved.ID != artifact.ID {
		t.Fatalf("verified published artifact was not resolved: artifact=%+v err=%v", resolved, err)
	}
}
