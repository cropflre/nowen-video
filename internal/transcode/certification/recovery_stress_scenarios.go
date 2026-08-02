package certification

import (
	"context"
	"fmt"
	"strconv"
	"time"

	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
)

func (h *recoveryHarness) runCancel(ctx context.Context) (recoveryScenarioResult, error) {
	job, err := h.createClaimedJob("worker-cancel", time.Now())
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	attempt, err := h.prepareAttempt(job, 1)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	process, err := h.runAttempt(ctx, job, attempt, processControl{CancelAtMicros: h.scenario.TriggerMicros})
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	now := time.Now()
	if updated, err := h.repo.MarkOwnedArtifactTerminal(job.ID, attempt.Record.ID, attempt.Artifact.ID, job.LeaseToken, "cancelled", "cancel_requested", "Certification cancelled active HLS output", now); err != nil || !updated {
		return recoveryScenarioResult{}, fmt.Errorf("mark cancelled Artifact terminal: updated=%t err=%v", updated, err)
	}
	if err := h.repo.CompleteAttempt(attempt.Record.ID, "cancelled", process.ExitCode, "", "cancel_requested", "Certification cancelled active HLS output", now); err != nil {
		return recoveryScenarioResult{}, err
	}
	completed, err := h.repo.CompleteLeasedJob(job.ID, job.LeaseToken, "cancelled", now)
	if err != nil || !completed {
		return recoveryScenarioResult{}, fmt.Errorf("complete cancelled recovery stress job: committed=%t err=%v", completed, err)
	}
	h.transition("cancelled", "cancelled", 1, 1, "cancelled", "current Lease finalized requested cancellation")
	artifact, finalJob, readableID, err := h.finalOutcome(job.ID, attempt.Artifact.ID)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	return recoveryScenarioResult{
		Transitions: h.transitions,
		Processes:   []transcoderecovery.ProcessEvidence{process},
		Fence: transcoderecovery.LeaseFenceEvidence{
			FirstTokenHash: transcoderecovery.TokenHash(job.LeaseToken),
		},
		Artifact: transcoderecovery.ArtifactOutcomeEvidence{
			FinalJobStatus:              finalJob.Status,
			FinalArtifactStatus:         artifact.Status,
			ReadableArtifactID:          readableID,
			PartialWorkspaceQuarantined: artifact.Status == "cancelled" && pathExists(attempt.Workspace),
			CleanupEligible:             artifact.Status == "cancelled",
		},
	}, nil
}

func (h *recoveryHarness) runSIGKILLRecovery(ctx context.Context) (recoveryScenarioResult, error) {
	job, err := h.createClaimedJob("worker-killed", time.Now())
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	first, err := h.prepareAttempt(job, 1)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	firstProcess, err := h.runAttempt(ctx, job, first, processControl{KillAtMicros: h.scenario.TriggerMicros})
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	if err := h.repo.CompleteAttempt(first.Record.ID, "failed", firstProcess.ExitCode, "", "worker_sigkill", "Certification killed owning FFmpeg process", time.Now()); err != nil {
		return recoveryScenarioResult{}, err
	}
	replacement, second, fence, secondProcess, err := h.requeueAndReplace(ctx, job, first, "lease_expired_after_sigkill")
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	artifact, finalJob, readableID, err := h.finalOutcome(job.ID, second.Artifact.ID)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	fence.FirstTokenHash = transcoderecovery.TokenHash(job.LeaseToken)
	fence.SecondTokenHash = transcoderecovery.TokenHash(replacement.LeaseToken)
	fence.ReplacementLeaseDifferent = job.LeaseToken != replacement.LeaseToken
	return recoveryScenarioResult{
		Transitions: h.transitions,
		Processes:   []transcoderecovery.ProcessEvidence{firstProcess, secondProcess},
		Fence:       fence,
		Artifact: transcoderecovery.ArtifactOutcomeEvidence{
			FinalJobStatus:              finalJob.Status,
			FinalArtifactStatus:         artifact.Status,
			ReadableArtifactID:          readableID,
			PartialWorkspaceQuarantined: pathExists(first.Workspace),
			CleanupEligible:             true,
		},
	}, nil
}

func (h *recoveryHarness) runENOSPC(ctx context.Context) (recoveryScenarioResult, error) {
	job, err := h.createClaimedJob("worker-enospc", time.Now())
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	attempt, err := h.prepareAttempt(job, 1)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	shim, err := buildENOSPCShim(h.workDir)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	process, err := h.runAttempt(ctx, job, attempt, processControl{
		ExtraEnv: []string{
			"LD_PRELOAD=" + shim,
			"NOWEN_ENOSPC_PATH=" + attempt.Workspace,
			"NOWEN_ENOSPC_AFTER_BYTES=" + strconv.FormatInt(h.scenario.Limits.ENOSPCAfterBytes, 10),
		},
	})
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	if process.ExitCode == 0 || !slicesContains(process.StderrMarkers, "ENOSPC") {
		return recoveryScenarioResult{}, fmt.Errorf("ENOSPC injection did not fail the process as expected")
	}
	now := time.Now()
	if updated, err := h.repo.MarkOwnedArtifactTerminal(job.ID, attempt.Record.ID, attempt.Artifact.ID, job.LeaseToken, "failed", "write_enospc", "No space left on device during segment write", now); err != nil || !updated {
		return recoveryScenarioResult{}, fmt.Errorf("mark ENOSPC Artifact terminal: updated=%t err=%v", updated, err)
	}
	if err := h.repo.CompleteAttempt(attempt.Record.ID, "failed", process.ExitCode, "", "write_enospc", "No space left on device during segment write", now); err != nil {
		return recoveryScenarioResult{}, err
	}
	completed, err := h.repo.CompleteLeasedJob(job.ID, job.LeaseToken, "failed", now)
	if err != nil || !completed {
		return recoveryScenarioResult{}, fmt.Errorf("complete ENOSPC recovery stress job: committed=%t err=%v", completed, err)
	}
	h.transition("failed", "running", 1, 1, "failed", "kernel write shim returned ENOSPC")
	artifact, finalJob, readableID, err := h.finalOutcome(job.ID, attempt.Artifact.ID)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	return recoveryScenarioResult{
		Transitions: h.transitions,
		Processes:   []transcoderecovery.ProcessEvidence{process},
		Fence: transcoderecovery.LeaseFenceEvidence{
			FirstTokenHash: transcoderecovery.TokenHash(job.LeaseToken),
		},
		Artifact: transcoderecovery.ArtifactOutcomeEvidence{
			FinalJobStatus:              finalJob.Status,
			FinalArtifactStatus:         artifact.Status,
			ReadableArtifactID:          readableID,
			PartialWorkspaceQuarantined: pathExists(attempt.Workspace),
			CleanupEligible:             artifact.Status == "failed",
		},
		ErrorCode: "write_enospc",
	}, nil
}

func (h *recoveryHarness) runBoundedResources(ctx context.Context) (recoveryScenarioResult, error) {
	job, err := h.createClaimedJob("worker-bounded", time.Now())
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	attempt, err := h.prepareAttempt(job, 1)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	commandPath, commandArgs, err := boundedCommand(h.ffmpegPath, attempt.Args, h.scenario.Limits)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	process, err := h.runAttempt(ctx, job, attempt, processControl{CommandPath: commandPath, CommandArgs: commandArgs})
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	if process.ExitCode != 0 {
		return recoveryScenarioResult{}, fmt.Errorf("bounded recovery stress process failed with exit code %d", process.ExitCode)
	}
	committed, err := h.publishAttempt(job, attempt)
	if err != nil || !committed {
		return recoveryScenarioResult{}, fmt.Errorf("publish bounded recovery stress artifact: committed=%t err=%v", committed, err)
	}
	process.PublishedExists = true
	h.transition("completed", "running", 1, 1, "published", "bounded process published and completed atomically")
	artifact, finalJob, readableID, err := h.finalOutcome(job.ID, attempt.Artifact.ID)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	return recoveryScenarioResult{
		Transitions: h.transitions,
		Processes:   []transcoderecovery.ProcessEvidence{process},
		Fence: transcoderecovery.LeaseFenceEvidence{
			FirstTokenHash:              transcoderecovery.TokenHash(job.LeaseToken),
			ReplacementPublishCommitted: true,
		},
		Artifact: transcoderecovery.ArtifactOutcomeEvidence{
			FinalJobStatus:      finalJob.Status,
			FinalArtifactStatus: artifact.Status,
			ReadableArtifactID:  readableID,
			CleanupEligible:     false,
		},
	}, nil
}

func (h *recoveryHarness) runStaleLeaseFence(ctx context.Context) (recoveryScenarioResult, error) {
	job, err := h.createClaimedJob("worker-stale", time.Now())
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	first, err := h.prepareAttempt(job, 1)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	firstProcess, err := h.runAttempt(ctx, job, first, processControl{})
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	if firstProcess.ExitCode != 0 {
		return recoveryScenarioResult{}, fmt.Errorf("stale Lease first process failed with exit code %d", firstProcess.ExitCode)
	}
	if err := h.repo.CompleteAttempt(first.Record.ID, "completed", 0, "", "", "", time.Now()); err != nil {
		return recoveryScenarioResult{}, err
	}
	replacement, second, fence, secondProcess, err := h.requeueAndReplace(ctx, job, first, "lease_expired_before_finalize")
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	artifact, finalJob, readableID, err := h.finalOutcome(job.ID, second.Artifact.ID)
	if err != nil {
		return recoveryScenarioResult{}, err
	}
	fence.FirstTokenHash = transcoderecovery.TokenHash(job.LeaseToken)
	fence.SecondTokenHash = transcoderecovery.TokenHash(replacement.LeaseToken)
	fence.ReplacementLeaseDifferent = job.LeaseToken != replacement.LeaseToken
	return recoveryScenarioResult{
		Transitions: h.transitions,
		Processes:   []transcoderecovery.ProcessEvidence{firstProcess, secondProcess},
		Fence:       fence,
		Artifact: transcoderecovery.ArtifactOutcomeEvidence{
			FinalJobStatus:              finalJob.Status,
			FinalArtifactStatus:         artifact.Status,
			ReadableArtifactID:          readableID,
			PartialWorkspaceQuarantined: pathExists(first.Workspace),
			CleanupEligible:             true,
		},
	}, nil
}
