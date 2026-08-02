package recoverystress

import (
	"fmt"
	"slices"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func (c ScenarioContract) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	if c.SchemaVersion != ScenarioSchemaVersion {
		return fmt.Errorf("unsupported recovery stress scenario schema %q", c.SchemaVersion)
	}
	if err := manifest.ValidateFor(spec); err != nil {
		return err
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return err
	}
	manifestVersion, manifestHash, _, err := transcodecorpus.ManifestIdentity(manifest, spec)
	if err != nil {
		return err
	}
	if c.SpecVersion != specVersion || c.SpecHash != specHash || c.ManifestVersion != manifestVersion || c.ManifestHash != manifestHash {
		return fmt.Errorf("recovery stress source identity is invalid")
	}
	if c.SourceGeneratorVersion != manifest.GeneratorVersion || c.SourceFFmpegVersion != manifest.FFmpegVersion || c.SourceFFprobeVersion != manifest.FFprobeVersion {
		return fmt.Errorf("recovery stress source toolchain differs from manifest")
	}
	if strings.TrimSpace(c.CertificationFFmpegVersion) == "" || strings.TrimSpace(c.CertificationFFprobeVersion) == "" {
		return fmt.Errorf("recovery stress certification toolchain is incomplete")
	}
	if strings.TrimSpace(c.TimestampPlanVersion) == "" || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("recovery stress timestamp plan identity is invalid")
	}
	expected, ok := LookupScenario(c.Scenario.ID)
	if !ok || c.Scenario != expected {
		return fmt.Errorf("recovery stress scenario identity is invalid")
	}
	asset, ok := findAsset(manifest, c.Source.CaseID)
	if !ok {
		return fmt.Errorf("recovery stress source asset is missing")
	}
	assetHash, err := CanonicalHash(asset)
	if err != nil {
		return err
	}
	if c.Source.RelativePath != asset.RelativePath || c.Source.SHA256 != asset.SHA256 || c.Source.SizeBytes != asset.SizeBytes || c.Source.AssetEvidenceHash != assetHash {
		return fmt.Errorf("recovery stress source identity differs from manifest")
	}
	if !isSHA256(c.Source.SHA256) || !isSHA256(c.Source.AssetEvidenceHash) {
		return fmt.Errorf("recovery stress source hashes are invalid")
	}
	if len(c.Transitions) < 3 {
		return fmt.Errorf("recovery stress state transition evidence is incomplete")
	}
	for index, transition := range c.Transitions {
		if transition.Sequence != index+1 || strings.TrimSpace(transition.JobStatus) == "" || strings.TrimSpace(transition.DesiredState) == "" || strings.TrimSpace(transition.Reason) == "" {
			return fmt.Errorf("recovery stress state transition %d is invalid", index+1)
		}
	}
	if len(c.Processes) != c.Scenario.ExpectedProcessCount {
		return fmt.Errorf("recovery stress has %d processes, want %d", len(c.Processes), c.Scenario.ExpectedProcessCount)
	}
	for index, process := range c.Processes {
		if process.AttemptOrdinal != index+1 || !isSHA256(process.CommandHash) || !isSHA256(process.StderrSHA256) {
			return fmt.Errorf("recovery stress process evidence %d is invalid", index+1)
		}
		if process.MaximumProgressMicros < 0 || process.TriggerObservedMicros < 0 || process.SegmentCount < 0 || process.MaxRSSBytes < 0 || process.ElapsedMillis < 0 {
			return fmt.Errorf("recovery stress process metrics %d are invalid", index+1)
		}
	}
	if c.Artifact.FinalJobStatus != c.Scenario.ExpectedFinalJobStatus || c.Artifact.FinalArtifactStatus != c.Scenario.ExpectedFinalArtifactStatus {
		return fmt.Errorf("recovery stress final state differs from scenario contract")
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("recovery stress cannot authorize seamless playback")
	}
	if !c.Passed {
		return fmt.Errorf("recovery stress scenario did not pass")
	}
	return c.validateScenarioOutcome()
}

func (c ScenarioContract) validateScenarioOutcome() error {
	first := c.Processes[0]
	switch c.Scenario.ID {
	case ScenarioCancelActiveWrite:
		if !first.Cancelled || first.TriggerObservedMicros < c.Scenario.TriggerMicros || first.SegmentCount <= 0 || c.Artifact.ReadableArtifactID != "" || !c.Artifact.PartialWorkspaceQuarantined || !c.Artifact.CleanupEligible {
			return fmt.Errorf("cancel-active-write outcome is invalid")
		}
	case ScenarioSIGKILLRecovery:
		if first.Signal != "SIGKILL" || first.ExitCode == 0 || c.Processes[1].ExitCode != 0 || !c.Fence.LeaseExpiredRequeued || !c.Fence.OldPrepareRejected || !c.Fence.OldCommitRejected || !c.Fence.ReplacementLeaseDifferent || !c.Fence.ReplacementPublishCommitted || c.Artifact.ReadableArtifactID == "" || !c.Artifact.PartialWorkspaceQuarantined {
			return fmt.Errorf("sigkill recovery outcome is invalid")
		}
	case ScenarioENOSPCWrite:
		if first.ExitCode == 0 || c.ErrorCode != "write_enospc" || !slices.Contains(first.StderrMarkers, "ENOSPC") || c.Artifact.ReadableArtifactID != "" || !c.Artifact.CleanupEligible {
			return fmt.Errorf("ENOSPC outcome is invalid")
		}
	case ScenarioBoundedResources:
		if first.ExitCode != 0 || first.MaxRSSBytes <= 0 || first.MaxRSSBytes > c.Scenario.Limits.AddressSpaceBytes || c.Artifact.ReadableArtifactID == "" || !c.Fence.ReplacementPublishCommitted {
			return fmt.Errorf("bounded resource outcome is invalid")
		}
	case ScenarioStaleLeaseFence:
		if first.ExitCode != 0 || c.Processes[1].ExitCode != 0 || !c.Fence.LeaseExpiredRequeued || !c.Fence.OldPrepareRejected || !c.Fence.OldCommitRejected || !c.Fence.ReplacementLeaseDifferent || !c.Fence.ReplacementPublishCommitted || c.Artifact.ReadableArtifactID == "" || !c.Artifact.PartialWorkspaceQuarantined {
			return fmt.Errorf("stale lease fence outcome is invalid")
		}
	default:
		return fmt.Errorf("unsupported recovery stress scenario %q", c.Scenario.ID)
	}
	if !isSHA256(c.Fence.FirstTokenHash) {
		return fmt.Errorf("first Lease token hash is invalid")
	}
	if c.Scenario.RequiresReplacementAttempt {
		if !isSHA256(c.Fence.SecondTokenHash) || c.Fence.FirstTokenHash == c.Fence.SecondTokenHash {
			return fmt.Errorf("replacement Lease identity is invalid")
		}
	}
	return nil
}
