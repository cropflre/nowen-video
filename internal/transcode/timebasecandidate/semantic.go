package timebasecandidate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

const SemanticCaseSchemaVersion = "encoder-time-base-semantic-candidate-evidence-v1"

func (c CaseEvidence) ValidateSemanticEquivalence() error {
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
	if !c.Comparison.FrameMappingEquivalent || !c.Comparison.CadenceEquivalent || !c.Comparison.AVSyncWithinTolerance {
		return fmt.Errorf("candidate semantic outputs diverged for case %s", c.Case.ID)
	}
	return nil
}

func SemanticCaseIdentity(c CaseEvidence) (version, hash, canonical string, err error) {
	if err := c.ValidateSemanticEquivalence(); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", "", "", fmt.Errorf("marshal semantic encoder time-base evidence: %w", err)
	}
	digest := sha256.Sum256(content)
	return SemanticCaseSchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}
