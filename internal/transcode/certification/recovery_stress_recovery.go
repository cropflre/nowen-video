package certification

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
	"gorm.io/gorm"
)

func (h *recoveryHarness) requeueAndReplace(
	ctx context.Context,
	oldJob *model.TranscodeJobRecord,
	oldAttempt *recoveryAttempt,
	reason string,
) (*model.TranscodeJobRecord, *recoveryAttempt, transcoderecovery.LeaseFenceEvidence, transcoderecovery.ProcessEvidence, error) {
	if oldJob.LeaseExpiresAt == nil {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, fmt.Errorf("old recovery stress Lease has no expiry")
	}
	recoveryAt := oldJob.LeaseExpiresAt.Add(time.Millisecond)
	requeued, err := h.repo.RequeueExpiredLease(oldJob.ID, oldJob.LeaseToken, recoveryAt)
	if err != nil || !requeued {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, fmt.Errorf("requeue expired recovery stress Lease: requeued=%t err=%v", requeued, err)
	}
	h.transition("queued", "running", 0, oldAttempt.Ordinal, "staging", "expired Lease returned the same durable Job to queued")
	replacement, claimed, err := h.repo.ClaimJob(oldJob.ID, "worker-replacement", recoveryAt.Add(time.Millisecond), recoveryStressLeaseDuration)
	if err != nil || !claimed {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, fmt.Errorf("claim replacement recovery stress Lease: claimed=%t err=%v", claimed, err)
	}
	h.transition("claimed", "running", 2, 0, "staging", "replacement worker acquired a new Lease token while the old Artifact remained fenced")
	second, err := h.prepareAttempt(replacement, 2)
	if err != nil {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, err
	}
	oldPrepare, err := h.repo.PrepareArtifactPublish(
		oldJob.ID,
		oldAttempt.Record.ID,
		oldAttempt.Artifact.ID,
		oldJob.LeaseToken,
		filepath.Join(h.workDir, "forbidden-old-publish"),
		filepath.Join(h.workDir, "forbidden-old-publish", "stream.m3u8"),
		time.Now(),
	)
	if err != nil {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, err
	}
	oldCommit, err := h.repo.CommitArtifactPublishAndCompleteJob(
		oldJob.ID,
		oldAttempt.Record.ID,
		oldAttempt.Artifact.ID,
		oldJob.LeaseToken,
		0,
		0,
		time.Now(),
	)
	if err != nil {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, err
	}
	if err := h.repo.MarkArtifactAbandoned(oldAttempt.Artifact.ID, reason, "Old Attempt lost its authoritative Lease", recoveryAt); err != nil {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, err
	}
	h.transition("claimed", "running", 2, oldAttempt.Ordinal, "abandoned", "old Artifact was quarantined after stale finalize attempts were fenced")
	secondProcess, err := h.runAttempt(ctx, replacement, second, processControl{})
	if err != nil {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, err
	}
	if secondProcess.ExitCode != 0 {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, fmt.Errorf("replacement process failed with exit code %d", secondProcess.ExitCode)
	}
	committed, err := h.publishAttempt(replacement, second)
	if err != nil || !committed {
		return nil, nil, transcoderecovery.LeaseFenceEvidence{}, transcoderecovery.ProcessEvidence{}, fmt.Errorf("publish replacement recovery stress artifact: committed=%t err=%v", committed, err)
	}
	secondProcess.PublishedExists = true
	h.transition("completed", "running", 2, 2, "published", "replacement Attempt published and completed atomically")
	return replacement, second, transcoderecovery.LeaseFenceEvidence{
		LeaseExpiredRequeued:        true,
		OldPrepareRejected:          !oldPrepare,
		OldCommitRejected:           !oldCommit,
		ReplacementPublishCommitted: committed,
	}, secondProcess, nil
}

func (h *recoveryHarness) publishAttempt(job *model.TranscodeJobRecord, attempt *recoveryAttempt) (bool, error) {
	validation, err := h.store.ValidateHLS(attempt.Workspace)
	if err != nil {
		return false, err
	}
	target, err := h.store.PublishedDir(job.MediaID, h.profile.ID, attempt.Artifact.ID)
	if err != nil {
		return false, err
	}
	manifestPath := filepath.Join(target, "stream.m3u8")
	now := time.Now()
	prepared, err := h.repo.PrepareArtifactPublish(job.ID, attempt.Record.ID, attempt.Artifact.ID, job.LeaseToken, target, manifestPath, now)
	if err != nil || !prepared {
		return false, err
	}
	if err := h.store.Publish(attempt.Workspace, target); err != nil {
		return false, err
	}
	committed, err := h.repo.CommitArtifactPublishAndCompleteJob(
		job.ID,
		attempt.Record.ID,
		attempt.Artifact.ID,
		job.LeaseToken,
		validation.SizeBytes,
		h.scenario.LogicalDurationMicros/1000,
		time.Now(),
	)
	if err != nil || !committed {
		return committed, err
	}
	if err := h.repo.CompleteAttempt(attempt.Record.ID, "completed", 0, "", "", "", time.Now()); err != nil {
		return false, err
	}
	return true, nil
}

func (h *recoveryHarness) finalOutcome(jobID, artifactID string) (*model.TranscodeArtifactRecord, *model.TranscodeJobRecord, string, error) {
	job, err := h.repo.FindJobByID(jobID)
	if err != nil {
		return nil, nil, "", err
	}
	var artifact model.TranscodeArtifactRecord
	if err := h.db.First(&artifact, "id = ?", artifactID).Error; err != nil {
		return nil, nil, "", err
	}
	readableID := ""
	readable, findErr := h.repo.FindReadableHLSArtifact(job.MediaID, h.profile.ID, job.SourceFingerprint, job.PlannerVersion, time.Now())
	if findErr == nil {
		readableID = readable.ID
	} else if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return nil, nil, "", findErr
	}
	return &artifact, job, readableID, nil
}

func (h *recoveryHarness) transition(jobStatus, desiredState string, leaseGeneration, attemptOrdinal int, artifactStatus, reason string) {
	h.transitions = append(h.transitions, transcoderecovery.StateTransitionEvidence{
		Sequence:        len(h.transitions) + 1,
		JobStatus:       jobStatus,
		DesiredState:    desiredState,
		LeaseGeneration: leaseGeneration,
		AttemptOrdinal:  attemptOrdinal,
		ArtifactStatus:  artifactStatus,
		Reason:          reason,
	})
}
