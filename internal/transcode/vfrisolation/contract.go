package vfrisolation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

const SchemaVersion = "vfr-layer-isolation-evidence-v1"

const (
	ClassificationPreserved = "preserved"
	ClassificationChanged   = "changed"

	SequenceReferenceNone     = "none"
	SequenceReferenceBaseline = "baseline"
	SequenceReferenceParent   = "parent"
)

// Contract records one certification-only VFR continuation-window experiment.
// It deliberately cannot change production policy or authorize seamless HLS.
type Contract struct {
	SchemaVersion                string                                  `json:"schema_version"`
	CaseID                       string                                  `json:"case_id"`
	FixtureID                    string                                  `json:"fixture_id"`
	WindowStartMicros            int64                                   `json:"window_start_micros"`
	WindowEndMicros              int64                                   `json:"window_end_micros"`
	FFmpegVersion                string                                  `json:"ffmpeg_version"`
	FFprobeVersion               string                                  `json:"ffprobe_version"`
	BaselineOutputCadenceVersion string                                  `json:"baseline_output_cadence_version"`
	BaselineOutputCadenceHash    string                                  `json:"baseline_output_cadence_hash"`
	SourceTimeline               transcodeoutputcadence.TimelineEvidence `json:"source_timeline"`
	Variants                     []VariantEvidence                       `json:"variants"`
	SeamlessAllowed              bool                                    `json:"seamless_allowed"`
	DiscontinuityRequired        bool                                    `json:"discontinuity_required"`
}

type VariantSpec struct {
	ID              string `json:"id"`
	Description     string `json:"description"`
	Layer           string `json:"layer"`
	Container       string `json:"container"`
	FPSMode         string `json:"fps_mode"`
	EncoderTimeBase string `json:"encoder_time_base"`
	CopyOnly        bool   `json:"copy_only"`
	ParentVariantID string `json:"parent_variant_id,omitempty"`
}

type VariantEvidence struct {
	Spec                       VariantSpec                              `json:"spec"`
	CommandHash                string                                   `json:"command_hash"`
	Timeline                   transcodeoutputcadence.TimelineEvidence  `json:"timeline"`
	Mapping                    transcodeoutputcadence.FrameMapping      `json:"mapping"`
	Fingerprint                FrameFingerprint                         `json:"fingerprint"`
	CadenceClassification      string                                   `json:"cadence_classification"`
	SequenceReference          string                                   `json:"sequence_reference"`
	SequenceReferenceVariantID string                                   `json:"sequence_reference_variant_id,omitempty"`
	SequenceMatchesReference   bool                                     `json:"sequence_matches_reference"`
}

type FrameFingerprint struct {
	FrameCount              int    `json:"frame_count"`
	UniqueFrameCount        int    `json:"unique_frame_count"`
	AdjacentDuplicateCount  int    `json:"adjacent_duplicate_count"`
	SequenceSHA256          string `json:"sequence_sha256"`
	FirstFrameSHA256        string `json:"first_frame_sha256"`
	LastFrameSHA256         string `json:"last_frame_sha256"`
}

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported VFR isolation schema %q", c.SchemaVersion)
	}
	for label, value := range map[string]string{
		"case ID": c.CaseID,
		"fixture ID": c.FixtureID,
		"FFmpeg version": c.FFmpegVersion,
		"FFprobe version": c.FFprobeVersion,
		"baseline output cadence version": c.BaselineOutputCadenceVersion,
		"baseline output cadence hash": c.BaselineOutputCadenceHash,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if c.BaselineOutputCadenceVersion != transcodeoutputcadence.SchemaVersion || !isSHA256(c.BaselineOutputCadenceHash) {
		return fmt.Errorf("baseline output cadence identity is invalid")
	}
	if c.WindowStartMicros < 0 || c.WindowEndMicros <= c.WindowStartMicros {
		return fmt.Errorf("VFR isolation window is invalid")
	}
	if err := c.SourceTimeline.ValidateFor(transcodeoutputcadence.TimelineSourceContinuation); err != nil {
		return fmt.Errorf("source continuation cadence is invalid: %w", err)
	}
	if c.SourceTimeline.WindowStartMicros != c.WindowStartMicros || c.SourceTimeline.WindowEndMicros != c.WindowEndMicros {
		return fmt.Errorf("source continuation window is inconsistent")
	}
	if c.SourceTimeline.DuplicatePTSCount != 0 || c.SourceTimeline.NonMonotonicPTSCount != 0 {
		return fmt.Errorf("source continuation PTS is not clean")
	}
	if len(c.Variants) < 2 {
		return fmt.Errorf("VFR isolation matrix is incomplete")
	}
	byID := make(map[string]VariantEvidence, len(c.Variants))
	for index, variant := range c.Variants {
		if err := variant.validate(c.SourceTimeline); err != nil {
			return fmt.Errorf("validate VFR isolation variant %d: %w", index, err)
		}
		if _, exists := byID[variant.Spec.ID]; exists {
			return fmt.Errorf("duplicate VFR isolation variant %q", variant.Spec.ID)
		}
		byID[variant.Spec.ID] = variant
	}
	baseline, ok := byID["production-hls-v1"]
	if !ok || c.Variants[0].Spec.ID != baseline.Spec.ID {
		return fmt.Errorf("production HLS baseline must be the first variant")
	}
	for _, variant := range c.Variants {
		switch variant.SequenceReference {
		case SequenceReferenceNone:
			if variant.SequenceReferenceVariantID != "" || variant.SequenceMatchesReference {
				return fmt.Errorf("variant %q has an invalid empty sequence reference", variant.Spec.ID)
			}
		case SequenceReferenceBaseline:
			if variant.SequenceReferenceVariantID != baseline.Spec.ID {
				return fmt.Errorf("variant %q does not reference the production baseline", variant.Spec.ID)
			}
			want := variant.Fingerprint.SequenceSHA256 == baseline.Fingerprint.SequenceSHA256
			if variant.SequenceMatchesReference != want {
				return fmt.Errorf("variant %q baseline sequence comparison is inconsistent", variant.Spec.ID)
			}
		case SequenceReferenceParent:
			parent, exists := byID[variant.SequenceReferenceVariantID]
			if !exists || variant.Spec.ParentVariantID != parent.Spec.ID {
				return fmt.Errorf("variant %q parent sequence reference is invalid", variant.Spec.ID)
			}
			want := variant.Fingerprint.SequenceSHA256 == parent.Fingerprint.SequenceSHA256
			if variant.SequenceMatchesReference != want {
				return fmt.Errorf("variant %q parent sequence comparison is inconsistent", variant.Spec.ID)
			}
		default:
			return fmt.Errorf("variant %q has unsupported sequence reference %q", variant.Spec.ID, variant.SequenceReference)
		}
		if variant.Spec.CopyOnly && (!variant.SequenceMatchesReference || variant.SequenceReference != SequenceReferenceParent) {
			return fmt.Errorf("copy-only variant %q did not preserve its parent decoded sequence", variant.Spec.ID)
		}
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("VFR isolation evidence cannot authorize seamless playback")
	}
	return nil
}

func (v VariantEvidence) validate(source transcodeoutputcadence.TimelineEvidence) error {
	for label, value := range map[string]string{
		"variant ID": v.Spec.ID,
		"description": v.Spec.Description,
		"layer": v.Spec.Layer,
		"container": v.Spec.Container,
		"fps mode": v.Spec.FPSMode,
		"encoder time base": v.Spec.EncoderTimeBase,
		"command hash": v.CommandHash,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if !isSHA256(v.CommandHash) {
		return fmt.Errorf("variant command identity is invalid")
	}
	if v.Spec.CopyOnly && strings.TrimSpace(v.Spec.ParentVariantID) == "" {
		return fmt.Errorf("copy-only variant requires a parent")
	}
	if !v.Spec.CopyOnly && v.Spec.ParentVariantID != "" {
		return fmt.Errorf("encoded variant cannot declare a parent")
	}
	if err := v.Timeline.ValidateFor("isolation_" + v.Spec.ID); err != nil {
		return fmt.Errorf("variant timeline is invalid: %w", err)
	}
	if v.Timeline.WindowStartMicros != source.WindowStartMicros || v.Timeline.WindowEndMicros != source.WindowEndMicros {
		return fmt.Errorf("variant window differs from source continuation")
	}
	if err := v.Mapping.Validate(); err != nil {
		return fmt.Errorf("variant frame mapping is invalid: %w", err)
	}
	wantMapping := transcodeoutputcadence.NewFrameMapping(source.FrameCount, v.Timeline.FrameCount)
	if v.Mapping != wantMapping {
		return fmt.Errorf("variant frame mapping is inconsistent")
	}
	if err := v.Fingerprint.validate(v.Timeline.FrameCount); err != nil {
		return err
	}
	wantClassification := ClassificationPreserved
	if cadenceChanged(source, v.Timeline, v.Mapping) {
		wantClassification = ClassificationChanged
	}
	if v.CadenceClassification != wantClassification {
		return fmt.Errorf("variant cadence classification is inconsistent")
	}
	return nil
}

func (f FrameFingerprint) validate(frameCount int) error {
	if f.FrameCount != frameCount || f.FrameCount <= 0 || f.UniqueFrameCount <= 0 || f.UniqueFrameCount > f.FrameCount {
		return fmt.Errorf("decoded frame fingerprint counts are invalid")
	}
	if f.AdjacentDuplicateCount < 0 || f.AdjacentDuplicateCount >= f.FrameCount {
		return fmt.Errorf("decoded adjacent duplicate count is invalid")
	}
	for _, value := range []string{f.SequenceSHA256, f.FirstFrameSHA256, f.LastFrameSHA256} {
		if !isSHA256(value) {
			return fmt.Errorf("decoded frame fingerprint hash is invalid")
		}
	}
	return nil
}

func CadenceClassification(source, output transcodeoutputcadence.TimelineEvidence, mapping transcodeoutputcadence.FrameMapping) string {
	if cadenceChanged(source, output, mapping) {
		return ClassificationChanged
	}
	return ClassificationPreserved
}

func cadenceChanged(source, output transcodeoutputcadence.TimelineEvidence, mapping transcodeoutputcadence.FrameMapping) bool {
	return mapping.Status != transcodeoutputcadence.MappingAligned ||
		output.DuplicatePTSCount != 0 || output.NonMonotonicPTSCount != 0 ||
		output.MaterialVariableDuration != source.MaterialVariableDuration ||
		output.NearZeroDeltaCount > source.NearZeroDeltaCount ||
		abs64(output.DominantDeltaMicros-source.DominantDeltaMicros) > transcodeoutputcadence.CadenceDeltaToleranceMicros
}

func (c Contract) CanonicalJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal VFR isolation evidence: %w", err)
	}
	return string(content), nil
}

func Identity(c Contract) (version, hash, canonical string, err error) {
	canonical, err = c.CanonicalJSON()
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256([]byte(canonical))
	return c.SchemaVersion, hex.EncodeToString(digest[:]), canonical, nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}
