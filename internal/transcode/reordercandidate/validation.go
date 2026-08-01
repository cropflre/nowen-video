package reordercandidate

import (
	"fmt"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

func (c Contract) Validate() error {
	if err := c.ValidateIdentity(); err != nil {
		return err
	}
	if len(c.Cases) == 0 {
		return fmt.Errorf("reorder matrix has no cases")
	}
	seen := make(map[string]struct{}, len(c.Cases))
	for index, evidence := range c.Cases {
		if _, exists := seen[evidence.Case.Base.ID]; exists {
			return fmt.Errorf("duplicate reorder case %q", evidence.Case.Base.ID)
		}
		seen[evidence.Case.Base.ID] = struct{}{}
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("validate reorder case %d: %w", index, err)
		}
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
	base := c.Case.Base
	if c.SourceStartupTimeline.WindowStartMicros != base.SourceOffsetMicros ||
		c.SourceStartupTimeline.WindowEndMicros != base.SourceOffsetMicros+base.ExpectedBoundaryMicros ||
		c.SourceContinuationTimeline.WindowStartMicros != base.SourceOffsetMicros+base.ExpectedBoundaryMicros ||
		c.SourceContinuationTimeline.WindowEndMicros != base.SourceOffsetMicros+base.DurationMicros {
		return fmt.Errorf("source cadence windows are inconsistent")
	}
	if len(c.Candidates) != 2 {
		return fmt.Errorf("reorder case %s must contain two candidates", base.ID)
	}
	for _, candidate := range c.Candidates {
		if err := candidate.Validate(c.Case, c.SourceStartupTimeline, c.SourceContinuationTimeline); err != nil {
			return err
		}
	}
	want := BuildCandidateComparison(c.Candidates[0], c.Candidates[1])
	if c.Comparison != want {
		return fmt.Errorf("reorder candidate comparison is inconsistent")
	}
	if !c.Comparison.Equivalent {
		return fmt.Errorf("reorder candidates diverged for case %s", base.ID)
	}
	return nil
}

func (c CandidateEvidence) Validate(caseSpec CaseSpec, sourceStartup, sourceContinuation transcodeoutputcadence.TimelineEvidence) error {
	if err := c.Spec.Validate(); err != nil {
		return err
	}
	if len(c.Runs) != RepeatCount {
		return fmt.Errorf("reorder candidate %s has %d repeats, want %d", c.Spec.ID, len(c.Runs), RepeatCount)
	}
	for index, run := range c.Runs {
		if err := run.Validate(caseSpec, c.Spec, sourceStartup, sourceContinuation, index+1); err != nil {
			return fmt.Errorf("validate reorder candidate %s run %d: %w", c.Spec.ID, index+1, err)
		}
	}
	want := BuildCandidateSummary(c.Runs)
	if c.Summary != want {
		return fmt.Errorf("reorder candidate %s summary is inconsistent", c.Spec.ID)
	}
	if !c.Summary.Stable {
		return fmt.Errorf("reorder candidate %s is not stable", c.Spec.ID)
	}
	return nil
}

func (r RunEvidence) Validate(caseSpec CaseSpec, candidate transcodetimebase.CandidateSpec, sourceStartup, sourceContinuation transcodeoutputcadence.TimelineEvidence, ordinal int) error {
	if r.Ordinal != ordinal || r.Base.Ordinal != ordinal {
		return fmt.Errorf("reorder repeat ordinal is invalid")
	}
	if err := r.Base.Validate(caseSpec.Base, candidate, sourceStartup, sourceContinuation, ordinal); err != nil {
		return fmt.Errorf("base candidate evidence: %w", err)
	}
	for window, evidence := range map[string]PacketOrderEvidence{
		"startup": r.StartupPacketOrder,
		"continuation": r.ContinuationPacketOrder,
	} {
		if evidence.Kind != PacketKind(caseSpec.Base.ID, candidate.ID, ordinal, window) {
			return fmt.Errorf("%s packet-order identity is invalid", window)
		}
		if err := evidence.Validate(); err != nil {
			return fmt.Errorf("%s packet-order evidence: %w", window, err)
		}
		if evidence.DTSNonMonotonicCount != 0 || evidence.DTSDuplicateCount != 0 {
			return fmt.Errorf("%s DTS is not strictly monotonic", window)
		}
		if evidence.ReorderedPacketCount == 0 || evidence.AdjacentPTSInversionCount == 0 || evidence.MaxPresentationReorderDepth == 0 {
			return fmt.Errorf("%s did not exercise B-frame presentation reordering", window)
		}
	}
	if r.StartupPacketOrder.PacketCount != r.Base.StartupTimeline.FrameCount ||
		r.ContinuationPacketOrder.PacketCount != r.Base.ContinuationTimeline.FrameCount {
		return fmt.Errorf("packet-order counts do not match cadence evidence")
	}
	return nil
}

func PacketKind(caseID, candidateID string, ordinal int, window string) string {
	return fmt.Sprintf("reorder_%s_%s_run_%02d_%s", caseID, candidateID, ordinal, window)
}
