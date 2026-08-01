package outputcadence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const SchemaVersion = "hls-output-cadence-evidence-v1"

const (
	TimelineSource       = "source"
	TimelineStartup      = "startup"
	TimelineContinuation = "continuation"

	MappingAligned             = "aligned"
	MappingWithinTolerance     = "within_tolerance"
	MappingDuplicateProjection = "duplicate_projection"
	MappingDropProjection      = "drop_projection"

	PreservationExact         = "preserved_exact"
	PreservationWithTolerance = "preserved_with_count_tolerance"
	PreservationChanged       = "changed"

	ContentDuplicateNotMeasured = "not_measured"

	FrameCountTolerance = 1
)

// Contract records full produced-video cadence and source-to-output frame-count
// projections. Frame-count deltas are diagnostic projections only; v1 does not
// claim content-level duplicate-frame detection and cannot authorize seamless HLS.
type Contract struct {
	SchemaVersion                string            `json:"schema_version"`
	CaseID                      string            `json:"case_id"`
	FixtureID                   string            `json:"fixture_id"`
	SourceMode                  string            `json:"source_mode"`
	DeclaredFrameRateNumerator  int64             `json:"declared_frame_rate_numerator"`
	DeclaredFrameRateDenominator int64            `json:"declared_frame_rate_denominator"`
	DeclaredFrameRateMilli      int               `json:"declared_frame_rate_milli"`
	ExpectedBoundaryMicros      int64             `json:"expected_boundary_micros"`
	ExpectedStartupVariable     bool              `json:"expected_startup_variable"`
	ExpectedContinuationVariable bool             `json:"expected_continuation_variable"`
	FFmpegVersion               string            `json:"ffmpeg_version"`
	FFprobeVersion              string            `json:"ffprobe_version"`
	SourceOriginVersion         string            `json:"source_origin_version"`
	SourceOriginHash            string            `json:"source_origin_hash"`
	TimestampPlanVersion        string            `json:"timestamp_plan_version"`
	TimestampPlanHash           string            `json:"timestamp_plan_hash"`
	BoundaryEvidenceVersion     string            `json:"boundary_evidence_version"`
	BoundaryEvidenceHash        string            `json:"boundary_evidence_hash"`
	AVSyncEvidenceVersion       string            `json:"av_sync_evidence_version"`
	AVSyncEvidenceHash          string            `json:"av_sync_evidence_hash"`
	SourceTimeline              TimelineEvidence  `json:"source_timeline"`
	StartupTimeline             TimelineEvidence  `json:"startup_timeline"`
	ContinuationTimeline        TimelineEvidence  `json:"continuation_timeline"`
	StartupMapping              FrameMapping      `json:"startup_mapping"`
	ContinuationMapping         FrameMapping      `json:"continuation_mapping"`
	PreservationStatus          string            `json:"preservation_status"`
	ContentDuplicateClassification string         `json:"content_duplicate_classification"`
	SeamlessAllowed             bool              `json:"seamless_allowed"`
	DiscontinuityRequired       bool              `json:"discontinuity_required"`
}

type TimelineEvidence struct {
	Kind                    string `json:"kind"`
	TimeBase                string `json:"time_base"`
	WindowStartMicros       int64  `json:"window_start_micros"`
	WindowEndMicros         int64  `json:"window_end_micros"`
	FrameCount              int    `json:"frame_count"`
	FirstPTS                int64  `json:"first_pts"`
	LastPTS                 int64  `json:"last_pts"`
	FirstPTSMicros          int64  `json:"first_pts_micros"`
	LastPTSMicros           int64  `json:"last_pts_micros"`
	MinDeltaTicks           int64  `json:"min_delta_ticks"`
	MaxDeltaTicks           int64  `json:"max_delta_ticks"`
	MinDeltaMicros          int64  `json:"min_delta_micros"`
	MaxDeltaMicros          int64  `json:"max_delta_micros"`
	DurationSpreadMicros    int64  `json:"duration_spread_micros"`
	DistinctDeltas          int    `json:"distinct_deltas"`
	VariableDuration        bool   `json:"variable_duration"`
	DuplicatePTSCount       int    `json:"duplicate_pts_count"`
	NonMonotonicPTSCount    int    `json:"non_monotonic_pts_count"`
}

type FrameMapping struct {
	InputFrames              int    `json:"input_frames"`
	OutputFrames             int    `json:"output_frames"`
	FrameCountDelta          int    `json:"frame_count_delta"`
	CountTolerance           int    `json:"count_tolerance"`
	ProjectedDuplicateFrames int    `json:"projected_duplicate_frames"`
	ProjectedDroppedFrames   int    `json:"projected_dropped_frames"`
	Status                   string `json:"status"`
}

func (c Contract) Validate() error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported output cadence schema %q", c.SchemaVersion)
	}
	for label, value := range map[string]string{
		"case ID": c.CaseID, "fixture ID": c.FixtureID,
		"FFmpeg version": c.FFmpegVersion, "FFprobe version": c.FFprobeVersion,
		"source origin version": c.SourceOriginVersion, "source origin hash": c.SourceOriginHash,
		"timestamp plan version": c.TimestampPlanVersion, "timestamp plan hash": c.TimestampPlanHash,
		"boundary evidence version": c.BoundaryEvidenceVersion, "boundary evidence hash": c.BoundaryEvidenceHash,
		"A/V sync evidence version": c.AVSyncEvidenceVersion, "A/V sync evidence hash": c.AVSyncEvidenceHash,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if c.SourceMode != transcodesourceorigin.ModeCFR && c.SourceMode != transcodesourceorigin.ModeVFR {
		return fmt.Errorf("unsupported source mode %q", c.SourceMode)
	}
	if c.DeclaredFrameRateNumerator <= 0 || c.DeclaredFrameRateDenominator <= 0 || c.DeclaredFrameRateMilli <= 0 || c.ExpectedBoundaryMicros <= 0 {
		return fmt.Errorf("output cadence media policy is invalid")
	}
	if c.SourceOriginVersion != transcodesourceorigin.SchemaVersion || !isSHA256(c.SourceOriginHash) {
		return fmt.Errorf("source origin identity is invalid")
	}
	if c.TimestampPlanVersion != transcodetimestamp.SchemaVersion || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("timestamp plan identity is invalid")
	}
	if c.BoundaryEvidenceVersion != transcodeboundary.SchemaVersion || !isSHA256(c.BoundaryEvidenceHash) {
		return fmt.Errorf("boundary evidence identity is invalid")
	}
	if c.AVSyncEvidenceVersion != transcodeavsync.SchemaVersion || !isSHA256(c.AVSyncEvidenceHash) {
		return fmt.Errorf("A/V sync evidence identity is invalid")
	}
	if err := c.SourceTimeline.validate(TimelineSource); err != nil {
		return fmt.Errorf("source timeline: %w", err)
	}
	if err := c.StartupTimeline.validate(TimelineStartup); err != nil {
		return fmt.Errorf("startup timeline: %w", err)
	}
	if err := c.ContinuationTimeline.validate(TimelineContinuation); err != nil {
		return fmt.Errorf("continuation timeline: %w", err)
	}
	if err := c.StartupMapping.validate(); err != nil {
		return fmt.Errorf("startup mapping: %w", err)
	}
	if err := c.ContinuationMapping.validate(); err != nil {
		return fmt.Errorf("continuation mapping: %w", err)
	}
	if c.StartupMapping.OutputFrames != c.StartupTimeline.FrameCount || c.ContinuationMapping.OutputFrames != c.ContinuationTimeline.FrameCount {
		return fmt.Errorf("output frame mapping does not match produced timelines")
	}
	cadenceMatches := c.StartupTimeline.VariableDuration == c.ExpectedStartupVariable &&
		c.ContinuationTimeline.VariableDuration == c.ExpectedContinuationVariable &&
		c.SourceTimeline.VariableDuration == (c.SourceMode == transcodesourceorigin.ModeVFR)
	cleanPTS := c.SourceTimeline.DuplicatePTSCount == 0 && c.SourceTimeline.NonMonotonicPTSCount == 0 &&
		c.StartupTimeline.DuplicatePTSCount == 0 && c.StartupTimeline.NonMonotonicPTSCount == 0 &&
		c.ContinuationTimeline.DuplicatePTSCount == 0 && c.ContinuationTimeline.NonMonotonicPTSCount == 0
	withinTolerance := absInt(c.StartupMapping.FrameCountDelta) <= FrameCountTolerance &&
		absInt(c.ContinuationMapping.FrameCountDelta) <= FrameCountTolerance
	exact := c.StartupMapping.FrameCountDelta == 0 && c.ContinuationMapping.FrameCountDelta == 0
	wantStatus := PreservationChanged
	if cadenceMatches && cleanPTS && exact {
		wantStatus = PreservationExact
	} else if cadenceMatches && cleanPTS && withinTolerance {
		wantStatus = PreservationWithTolerance
	}
	if c.PreservationStatus != wantStatus {
		return fmt.Errorf("output cadence preservation status is inconsistent")
	}
	if c.ContentDuplicateClassification != ContentDuplicateNotMeasured {
		return fmt.Errorf("v1 cannot claim content-level duplicate detection")
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("output cadence evidence v1 cannot authorize seamless playback")
	}
	return nil
}

func (t TimelineEvidence) validate(expectedKind string) error {
	if t.Kind != expectedKind || strings.TrimSpace(t.TimeBase) == "" || t.FrameCount < 2 || t.WindowEndMicros <= t.WindowStartMicros {
		return fmt.Errorf("timeline identity is incomplete")
	}
	first, err := transcodeboundary.TicksToMicros(t.FirstPTS, t.TimeBase)
	if err != nil {
		return err
	}
	last, err := transcodeboundary.TicksToMicros(t.LastPTS, t.TimeBase)
	if err != nil {
		return err
	}
	minimum, err := transcodeboundary.TicksToMicros(t.MinDeltaTicks, t.TimeBase)
	if err != nil || minimum <= 0 {
		return fmt.Errorf("minimum frame delta is invalid")
	}
	maximum, err := transcodeboundary.TicksToMicros(t.MaxDeltaTicks, t.TimeBase)
	if err != nil || maximum < minimum {
		return fmt.Errorf("maximum frame delta is invalid")
	}
	if t.FirstPTSMicros != first || t.LastPTSMicros != last || t.MinDeltaMicros != minimum || t.MaxDeltaMicros != maximum {
		return fmt.Errorf("timeline microsecond projection is inconsistent")
	}
	spread := maximum - minimum
	if t.DurationSpreadMicros != spread || t.VariableDuration != (spread >= transcodesourceorigin.VFRSpreadThresholdMicros) || t.DistinctDeltas <= 0 {
		return fmt.Errorf("timeline cadence projection is inconsistent")
	}
	if t.DuplicatePTSCount < 0 || t.NonMonotonicPTSCount < 0 {
		return fmt.Errorf("timeline PTS counters are invalid")
	}
	return nil
}

func (m FrameMapping) validate() error {
	if m.InputFrames <= 0 || m.OutputFrames <= 0 || m.CountTolerance != FrameCountTolerance {
		return fmt.Errorf("frame mapping policy is invalid")
	}
	delta := m.OutputFrames - m.InputFrames
	if m.FrameCountDelta != delta || m.ProjectedDuplicateFrames != maxInt(delta, 0) || m.ProjectedDroppedFrames != maxInt(-delta, 0) {
		return fmt.Errorf("frame mapping projection is inconsistent")
	}
	want := MappingAligned
	switch {
	case delta > FrameCountTolerance:
		want = MappingDuplicateProjection
	case delta < -FrameCountTolerance:
		want = MappingDropProjection
	case delta != 0:
		want = MappingWithinTolerance
	}
	if m.Status != want {
		return fmt.Errorf("frame mapping status is inconsistent")
	}
	return nil
}

func NewFrameMapping(inputFrames, outputFrames int) FrameMapping {
	delta := outputFrames - inputFrames
	status := MappingAligned
	switch {
	case delta > FrameCountTolerance:
		status = MappingDuplicateProjection
	case delta < -FrameCountTolerance:
		status = MappingDropProjection
	case delta != 0:
		status = MappingWithinTolerance
	}
	return FrameMapping{
		InputFrames: inputFrames, OutputFrames: outputFrames, FrameCountDelta: delta,
		CountTolerance: FrameCountTolerance, ProjectedDuplicateFrames: maxInt(delta, 0),
		ProjectedDroppedFrames: maxInt(-delta, 0), Status: status,
	}
}

func PreservationFor(c Contract) string {
	cadenceMatches := c.StartupTimeline.VariableDuration == c.ExpectedStartupVariable &&
		c.ContinuationTimeline.VariableDuration == c.ExpectedContinuationVariable &&
		c.SourceTimeline.VariableDuration == (c.SourceMode == transcodesourceorigin.ModeVFR)
	cleanPTS := c.SourceTimeline.DuplicatePTSCount == 0 && c.SourceTimeline.NonMonotonicPTSCount == 0 &&
		c.StartupTimeline.DuplicatePTSCount == 0 && c.StartupTimeline.NonMonotonicPTSCount == 0 &&
		c.ContinuationTimeline.DuplicatePTSCount == 0 && c.ContinuationTimeline.NonMonotonicPTSCount == 0
	if cadenceMatches && cleanPTS && c.StartupMapping.FrameCountDelta == 0 && c.ContinuationMapping.FrameCountDelta == 0 {
		return PreservationExact
	}
	if cadenceMatches && cleanPTS && absInt(c.StartupMapping.FrameCountDelta) <= FrameCountTolerance && absInt(c.ContinuationMapping.FrameCountDelta) <= FrameCountTolerance {
		return PreservationWithTolerance
	}
	return PreservationChanged
}

func (c Contract) CanonicalJSON() (string, error) {
	if err := c.Validate(); err != nil {
		return "", err
	}
	content, err := json.Marshal(c)
	if err != nil {
		return "", fmt.Errorf("marshal output cadence evidence: %w", err)
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

func absInt(value int) int {
	if value < 0 {
		return -value
	}
	return value
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
