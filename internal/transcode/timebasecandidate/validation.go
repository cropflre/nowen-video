package timebasecandidate

import (
	"fmt"
	"strings"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported encoder time-base candidate schema %q", c.SchemaVersion)
	}
	if strings.TrimSpace(c.FFmpegVersion) == "" || strings.TrimSpace(c.FFprobeVersion) == "" {
		return fmt.Errorf("toolchain identity is incomplete")
	}
	if c.RepeatCount != RepeatCount || c.VarianceToleranceMicros != VarianceToleranceMicros || c.CrossCandidateToleranceMicros != CrossCandidateToleranceMicros {
		return fmt.Errorf("candidate variance policy is invalid")
	}
	if len(c.Cases) == 0 {
		return fmt.Errorf("candidate matrix has no cases")
	}
	seen := make(map[string]struct{}, len(c.Cases))
	for index, evidence := range c.Cases {
		if _, exists := seen[evidence.Case.ID]; exists {
			return fmt.Errorf("duplicate candidate case %q", evidence.Case.ID)
		}
		seen[evidence.Case.ID] = struct{}{}
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("validate candidate case %d: %w", index, err)
		}
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("candidate evidence cannot authorize seamless playback")
	}
	return nil
}

func (c CaseEvidence) Validate() error {
	if err := c.Case.Validate(); err != nil {
		return err
	}
	if err := c.SourceStartupTimeline.ValidateFor(transcodeoutputcadence.TimelineSourceStartup); err != nil {
		return fmt.Errorf("source startup cadence: %w", err)
	}
	if err := c.SourceContinuationTimeline.ValidateFor(transcodeoutputcadence.TimelineSourceContinuation); err != nil {
		return fmt.Errorf("source continuation cadence: %w", err)
	}
	if c.SourceStartupTimeline.WindowStartMicros != c.Case.SourceOffsetMicros ||
		c.SourceStartupTimeline.WindowEndMicros != c.Case.SourceOffsetMicros+c.Case.ExpectedBoundaryMicros ||
		c.SourceContinuationTimeline.WindowStartMicros != c.Case.SourceOffsetMicros+c.Case.ExpectedBoundaryMicros ||
		c.SourceContinuationTimeline.WindowEndMicros != c.Case.SourceOffsetMicros+c.Case.DurationMicros {
		return fmt.Errorf("source cadence windows are inconsistent")
	}
	if len(c.Candidates) != 2 {
		return fmt.Errorf("case %s must contain two candidates", c.Case.ID)
	}
	seen := make(map[string]struct{}, len(c.Candidates))
	for _, candidate := range c.Candidates {
		if _, exists := seen[candidate.Spec.ID]; exists {
			return fmt.Errorf("duplicate candidate %q", candidate.Spec.ID)
		}
		seen[candidate.Spec.ID] = struct{}{}
		if err := candidate.Validate(c.Case, c.SourceStartupTimeline, c.SourceContinuationTimeline); err != nil {
			return err
		}
	}
	want := BuildCandidateComparison(c.Candidates[0], c.Candidates[1])
	if c.Comparison != want {
		return fmt.Errorf("candidate comparison is inconsistent")
	}
	if !c.Comparison.Equivalent {
		return fmt.Errorf("candidate outputs diverged for case %s", c.Case.ID)
	}
	return nil
}

func (c CandidateEvidence) Validate(caseSpec CaseSpec, sourceStartup, sourceContinuation transcodeoutputcadence.TimelineEvidence) error {
	if err := c.Spec.Validate(); err != nil {
		return err
	}
	if len(c.Runs) != RepeatCount {
		return fmt.Errorf("candidate %s has %d repeats, want %d", c.Spec.ID, len(c.Runs), RepeatCount)
	}
	for index, run := range c.Runs {
		if err := run.Validate(caseSpec, c.Spec, sourceStartup, sourceContinuation, index+1); err != nil {
			return fmt.Errorf("validate candidate %s run %d: %w", c.Spec.ID, index+1, err)
		}
	}
	want := BuildCandidateSummary(c.Runs)
	if c.Summary != want {
		return fmt.Errorf("candidate %s summary is inconsistent", c.Spec.ID)
	}
	if !c.Summary.Stable {
		return fmt.Errorf("candidate %s is not stable", c.Spec.ID)
	}
	return nil
}

func (r RunEvidence) Validate(caseSpec CaseSpec, candidate CandidateSpec, sourceStartup, sourceContinuation transcodeoutputcadence.TimelineEvidence, ordinal int) error {
	if r.Ordinal != ordinal {
		return fmt.Errorf("repeat ordinal is invalid")
	}
	if !isSHA256(r.StartupCommandHash) || !isSHA256(r.ContinuationCommandHash) {
		return fmt.Errorf("command identity is invalid")
	}
	startupKind := TimelineKind(caseSpec.ID, candidate.ID, ordinal, "startup")
	continuationKind := TimelineKind(caseSpec.ID, candidate.ID, ordinal, "continuation")
	if err := r.StartupTimeline.ValidateFor(startupKind); err != nil {
		return fmt.Errorf("startup timeline: %w", err)
	}
	if err := r.ContinuationTimeline.ValidateFor(continuationKind); err != nil {
		return fmt.Errorf("continuation timeline: %w", err)
	}
	if r.StartupTimeline.WindowStartMicros != 0 || r.StartupTimeline.WindowEndMicros != caseSpec.ExpectedBoundaryMicros ||
		r.ContinuationTimeline.WindowStartMicros != caseSpec.ExpectedBoundaryMicros || r.ContinuationTimeline.WindowEndMicros != caseSpec.DurationMicros {
		return fmt.Errorf("output cadence windows are inconsistent")
	}
	if err := r.StartupMapping.Validate(); err != nil {
		return err
	}
	if err := r.ContinuationMapping.Validate(); err != nil {
		return err
	}
	if r.StartupMapping != transcodeoutputcadence.NewFrameMapping(sourceStartup.FrameCount, r.StartupTimeline.FrameCount) ||
		r.ContinuationMapping != transcodeoutputcadence.NewFrameMapping(sourceContinuation.FrameCount, r.ContinuationTimeline.FrameCount) {
		return fmt.Errorf("source/output frame mapping is inconsistent")
	}
	if err := validateFingerprint(r.StartupFingerprint, r.StartupTimeline.FrameCount); err != nil {
		return fmt.Errorf("startup fingerprint: %w", err)
	}
	if err := validateFingerprint(r.ContinuationFingerprint, r.ContinuationTimeline.FrameCount); err != nil {
		return fmt.Errorf("continuation fingerprint: %w", err)
	}
	if !runPreserved(r, sourceStartup, sourceContinuation) {
		return fmt.Errorf("candidate run did not preserve cadence and decoded frames")
	}
	if err := r.Boundary.Validate(); err != nil {
		return err
	}
	boundaryVersion, boundaryHash, _, err := transcodeboundary.Identity(r.Boundary)
	if err != nil {
		return err
	}
	if r.BoundaryVersion != boundaryVersion || r.BoundaryHash != boundaryHash {
		return fmt.Errorf("boundary identity is invalid")
	}
	if r.Boundary.CaseID != BoundaryCaseID(caseSpec.ID, candidate.ID, ordinal) || r.Boundary.ExpectedBoundaryMicros != caseSpec.ExpectedBoundaryMicros {
		return fmt.Errorf("boundary case identity is inconsistent")
	}
	if err := r.AVSync.ValidateAgainst(r.Boundary); err != nil {
		return err
	}
	avVersion, avHash, _, err := transcodeavsync.Identity(r.AVSync)
	if err != nil {
		return err
	}
	if r.AVSyncVersion != avVersion || r.AVSyncHash != avHash {
		return fmt.Errorf("A/V sync identity is invalid")
	}
	if r.Boundary.SeamlessAllowed || !r.Boundary.DiscontinuityRequired || r.AVSync.SeamlessAllowed || !r.AVSync.DiscontinuityRequired {
		return fmt.Errorf("nested evidence attempted to authorize seamless playback")
	}
	return nil
}

func TimelineKind(caseID, candidateID string, ordinal int, window string) string {
	return fmt.Sprintf("candidate_%s_%s_run_%02d_%s", caseID, candidateID, ordinal, window)
}

func BoundaryCaseID(caseID, candidateID string, ordinal int) string {
	return fmt.Sprintf("%s/%s/run-%02d", caseID, candidateID, ordinal)
}
