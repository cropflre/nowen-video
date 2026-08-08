package repository

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestArtifactAttestationLeaseAndPublishFences(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	activeKey := "attestation-fence-active"
	job := &model.TranscodeJobRecord{
		MediaID:             "media-attestation-fence",
		Intent:              "startup_continuation_hls",
		ProfileID:           "720p",
		Status:              "queued",
		DesiredState:        "running",
		ActiveKey:           &activeKey,
		SourceFingerprint:   "source-attestation-fence",
		PlannerVersion:      "startup-continuation-hls-v3",
		EncodingPlanVersion: "hls-encoding-plan-v1",
		EncodingPlanHash:    "encoding-plan-attestation-fence",
		EncodingPlanJSON:    `{"schema_version":"hls-encoding-plan-v1"}`,
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "worker-attestation", now, time.Minute)
	if err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	attempt := &model.TranscodeAttemptRecord{
		JobID:    job.ID,
		Number:   1,
		Status:   "running",
		ExitCode: -1,
	}
	if err := repo.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	if running, err := repo.SetJobRunning(job.ID, attempt.ID, claimed.LeaseToken, now.Add(time.Second)); err != nil || !running {
		t.Fatalf("set running: running=%v err=%v", running, err)
	}
	artifact := &model.TranscodeArtifactRecord{
		JobID:             job.ID,
		AttemptID:         attempt.ID,
		MediaID:           job.MediaID,
		Kind:              "startup_continuation_hls",
		ProfileID:         job.ProfileID,
		SourceFingerprint: job.SourceFingerprint,
		PlannerVersion:    job.PlannerVersion,
		TempPath:          "/cache/workspaces/attestation-fence",
		Status:            "staging",
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.RecordOwnedArtifactAttestation(
		job.ID,
		attempt.ID,
		artifact.ID,
		"stale-lease-token",
		"hls-produced-media-attestation-v1",
		"verified",
		"stale-attestation",
		`{"scope":"complete"}`,
		1400,
		31400,
		now.Add(2*time.Second),
	)
	if err != nil || stored {
		t.Fatalf("stale lease wrote attestation: stored=%v err=%v", stored, err)
	}

	stored, err = repo.RecordCurrentArtifactAttestation(
		artifact.ID,
		"hls-produced-media-attestation-v1",
		"provisional",
		"provisional-attestation",
		`{"scope":"first_segment"}`,
		1400,
		3400,
		now.Add(3*time.Second),
	)
	if err != nil || !stored {
		t.Fatalf("current attempt provisional attestation: stored=%v err=%v", stored, err)
	}

	prepared, err := repo.PrepareArtifactPublish(
		job.ID,
		attempt.ID,
		artifact.ID,
		claimed.LeaseToken,
		"/cache/artifacts/attestation-fence",
		"/cache/artifacts/attestation-fence/stream.m3u8",
		now.Add(4*time.Second),
	)
	if err != nil || prepared {
		t.Fatalf("provisional artifact entered publishing: prepared=%v err=%v", prepared, err)
	}

	stored, err = repo.RecordOwnedArtifactAttestation(
		job.ID,
		attempt.ID,
		artifact.ID,
		claimed.LeaseToken,
		"hls-produced-media-attestation-v1",
		"verified",
		"verified-attestation",
		`{"scope":"complete"}`,
		1400,
		31400,
		now.Add(5*time.Second),
	)
	if err != nil || !stored {
		t.Fatalf("current lease verified attestation: stored=%v err=%v", stored, err)
	}

	prepared, err = repo.PrepareArtifactPublish(
		job.ID,
		attempt.ID,
		artifact.ID,
		claimed.LeaseToken,
		"/cache/artifacts/attestation-fence",
		"/cache/artifacts/attestation-fence/stream.m3u8",
		now.Add(6*time.Second),
	)
	if err != nil || !prepared {
		t.Fatalf("verified artifact did not enter publishing: prepared=%v err=%v", prepared, err)
	}
}

func TestCurrentArtifactAttestationRejectsExpiredLease(t *testing.T) {
	repo := newTranscodeExecutionTestRepo(t)
	activeKey := "attestation-expired-active"
	job := &model.TranscodeJobRecord{
		MediaID:             "media-attestation-expired",
		Intent:              "startup_continuation_hls",
		ProfileID:           "720p",
		Status:              "queued",
		DesiredState:        "running",
		ActiveKey:           &activeKey,
		SourceFingerprint:   "source-attestation-expired",
		PlannerVersion:      "startup-continuation-hls-v3",
		EncodingPlanVersion: "hls-encoding-plan-v1",
		EncodingPlanHash:    "encoding-plan-attestation-expired",
		EncodingPlanJSON:    `{"schema_version":"hls-encoding-plan-v1"}`,
	}
	if err := repo.CreateJob(job); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 8, 1, 3, 0, 0, 0, time.UTC)
	claimed, ok, err := repo.ClaimJob(job.ID, "worker-expired", now, time.Second)
	if err != nil || !ok {
		t.Fatalf("claim: ok=%v err=%v", ok, err)
	}
	attempt := &model.TranscodeAttemptRecord{JobID: job.ID, Number: 1, Status: "running", ExitCode: -1}
	if err := repo.CreateAttempt(attempt); err != nil {
		t.Fatal(err)
	}
	if running, err := repo.SetJobRunning(job.ID, attempt.ID, claimed.LeaseToken, now); err != nil || !running {
		t.Fatalf("set running: running=%v err=%v", running, err)
	}
	artifact := &model.TranscodeArtifactRecord{
		JobID:             job.ID,
		AttemptID:         attempt.ID,
		MediaID:           job.MediaID,
		Kind:              "startup_continuation_hls",
		ProfileID:         job.ProfileID,
		SourceFingerprint: job.SourceFingerprint,
		PlannerVersion:    job.PlannerVersion,
		TempPath:          "/cache/workspaces/attestation-expired",
		Status:            "staging",
	}
	if err := repo.CreateArtifact(artifact); err != nil {
		t.Fatal(err)
	}

	stored, err := repo.RecordCurrentArtifactAttestation(
		artifact.ID,
		"hls-produced-media-attestation-v1",
		"provisional",
		"expired-attestation",
		`{"scope":"first_segment"}`,
		1400,
		3400,
		now.Add(2*time.Second),
	)
	if err != nil || stored {
		t.Fatalf("expired lease accepted provisional attestation: stored=%v err=%v", stored, err)
	}
}
