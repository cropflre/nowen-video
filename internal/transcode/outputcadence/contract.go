package outputcadence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"strings"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const SchemaVersion = "hls-output-cadence-evidence-v1"

const (
	TimelineSource             = "source_full"
	TimelineSourceStartup      = "source_startup"
	TimelineSourceContinuation = "source_continuation"
	TimelineStartup            = "output_startup"
	TimelineContinuation       = "output_continuation"

	MappingAligned             = "aligned"
	MappingWithinTolerance     = "within_tolerance"
	MappingDuplicateProjection = "duplicate_projection"
	MappingDropProjection      = "drop_projection"

	PreservationExact               = "preserved_exact"
	PreservationWithCadenceOutliers = "preserved_with_cadence_outliers"
	PreservationWithCountTolerance  = "preserved_with_count_tolerance"
	PreservationChanged             = "changed"

	ContentDuplicateNotMeasured = "not_measured"

	FrameCountTolerance                = 1
	SignificantDeltaPercentDenominator = 100
	MinimumSignificantDeltaCount       = 2
	NearZeroDeltaThresholdMicros       = 1_000
	CadenceDeltaToleranceMicros        = 1_000
)

// Contract records complete source/output video cadence and frame-count
// projections. Frame-count deltas are diagnostic projections only; v1 does not
// claim content-level duplicate-frame detection and cannot authorize seamless HLS.
type Contract struct {
	SchemaVersion                              string           `json:"schema_version"`
	CaseID                                    string           `json:"case_id"`
	FixtureID                                 string           `json:"fixture_id"`
	SourceMode                                string           `json:"source_mode"`
	DeclaredFrameRateNumerator                int64            `json:"declared_frame_rate_numerator"`
	DeclaredFrameRateDenominator              int64            `json:"declared_frame_rate_denominator"`
	DeclaredFrameRateMilli                    int              `json:"declared_frame_rate_milli"`
	ExpectedBoundaryMicros                    int64            `json:"expected_boundary_micros"`
	ExpectedStartupMaterialVariable           bool             `json:"expected_startup_material_variable"`
	ExpectedContinuationMaterialVariable      bool             `json:"expected_continuation_material_variable"`
	FFmpegVersion                             string           `json:"ffmpeg_version"`
	FFprobeVersion                            string           `json:"ffprobe_version"`
	SourceOriginVersion                       string           `json:"source_origin_version"`
	SourceOriginHash                          string           `json:"source_origin_hash"`
	TimestampPlanVersion                      string           `json:"timestamp_plan_version"`
	TimestampPlanHash                         string           `json:"timestamp_plan_hash"`
	BoundaryEvidenceVersion                   string           `json:"boundary_evidence_version"`
	BoundaryEvidenceHash                      string           `json:"boundary_evidence_hash"`
	AVSyncEvidenceVersion                     string           `json:"av_sync_evidence_version"`
	AVSyncEvidenceHash                        string           `json:"av_sync_evidence_hash"`
	SourceTimeline                            TimelineEvidence `json:"source_timeline"`
	SourceStartupTimeline                     TimelineEvidence `json:"source_startup_timeline"`
	SourceContinuationTimeline                TimelineEvidence `json:"source_continuation_timeline"`
	StartupTimeline                           TimelineEvidence `json:"startup_timeline"`
	ContinuationTimeline                      TimelineEvidence `json:"continuation_timeline"`
	StartupMapping                            FrameMapping     `json:"startup_mapping"`
	ContinuationMapping                       FrameMapping     `json:"continuation_mapping"`
	PreservationStatus                        string           `json:"preservation_status"`
	ContentDuplicateClassification            string           `json:"content_duplicate_classification"`
	SeamlessAllowed                           bool             `json:"seamless_allowed"`
	DiscontinuityRequired                     bool             `json:"discontinuity_required"`
}

type TimelineEvidence struct {
	Kind                          string        `json:"kind"`
	TimeBase                      string        `json:"time_base"`
	WindowStartMicros             int64         `json:"window_start_micros"`
	WindowEndMicros               int64         `json:"window_end_micros"`
	FrameCount                    int           `json:"frame_count"`
	FirstPTS                      int64         `json:"first_pts"`
	LastPTS                       int64         `json:"last_pts"`
	FirstPTSMicros                int64         `json:"first_pts_micros"`
	LastPTSMicros                 int64         `json:"last_pts_micros"`
	MinDeltaTicks                 int64         `json:"min_delta_ticks"`
	MaxDeltaTicks                 int64         `json:"max_delta_ticks"`
	MinDeltaMicros                int64         `json:"min_delta_micros"`
	MaxDeltaMicros                int64         `json:"max_delta_micros"`
	DurationSpreadMicros          int64         `json:"duration_spread_micros"`
	DistinctDeltas                int           `json:"distinct_deltas"`
	VariableDuration              bool          `json:"variable_duration"`
	SignificantBucketMinimumCount int           `json:"significant_bucket_minimum_count"`
	SignificantDeltaCount         int           `json:"significant_delta_count"`
	OutlierDeltaCount             int           `json:"outlier_delta_count"`
	NearZeroDeltaCount            int           `json:"near_zero_delta_count"`
	DominantDeltaTicks            int64         `json:"dominant_delta_ticks"`
	DominantDeltaMicros           int64         `json:"dominant_delta_micros"`
	DominantDeltaCount            int           `json:"dominant_delta_count"`
	MaterialVariableDuration      bool          `json:"material_variable_duration"`
	DeltaHistogram                []DeltaBucket `json:"delta_histogram"`
	DuplicatePTSCount             int           `json:"duplicate_pts_count"`
	NonMonotonicPTSCount          int           `json:"non_monotonic_pts_count"`
}

type DeltaBucket struct {
	DeltaTicks  int64 `json:"delta_ticks"`
	DeltaMicros int64 `json:"delta_micros"`
	Count       int   `json:"count"`
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
	if c.DeclaredFrameRateNumerator <= 0 || c.DeclaredFrameRateDenominator <= 0 || c.ExpectedBoundaryMicros <= 0 {
		return fmt.Errorf("output cadence media policy is invalid")
	}
	wantRateMilli := int(math.Round(float64(c.DeclaredFrameRateNumerator) * 1000 / float64(c.DeclaredFrameRateDenominator)))
	if c.DeclaredFrameRateMilli != wantRateMilli {
		return fmt.Errorf("declared frame rate projection is inconsistent")
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
	for kind, timeline := range map[string]TimelineEvidence{
		TimelineSource: c.SourceTimeline,
		TimelineSourceStartup: c.SourceStartupTimeline,
		TimelineSourceContinuation: c.SourceContinuationTimeline,
		TimelineStartup: c.StartupTimeline,
		TimelineContinuation: c.ContinuationTimeline,
	} {
		if err := timeline.validate(kind); err != nil {
			return fmt.Errorf("%s timeline: %w", kind, err)
		}
	}
	if err := c.StartupMapping.validate(); err != nil {
		return fmt.Errorf("startup mapping: %w", err)
	}
	if err := c.ContinuationMapping.validate(); err != nil {
		return fmt.Errorf("continuation mapping: %w", err)
	}
	if c.StartupMapping.InputFrames != c.SourceStartupTimeline.FrameCount ||
		c.ContinuationMapping.InputFrames != c.SourceContinuationTimeline.FrameCount ||
		c.StartupMapping.OutputFrames != c.StartupTimeline.FrameCount ||
		c.ContinuationMapping.OutputFrames != c.ContinuationTimeline.FrameCount ||
		c.SourceTimeline.FrameCount != c.SourceStartupTimeline.FrameCount+c.SourceContinuationTimeline.FrameCount {
		return fmt.Errorf("source/output frame mapping does not match timeline evidence")
	}
	if c.PreservationStatus != preservationFor(c) {
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
	if t.FirstPTSMicros != first || t.LastPTSMicros != last {
		return fmt.Errorf("timeline PTS projection is inconsistent")
	}
	if t.DuplicatePTSCount < 0 || t.NonMonotonicPTSCount < 0 {
		return fmt.Errorf("timeline PTS counters are invalid")
	}
	positiveDeltaCount := t.FrameCount - 1 - t.DuplicatePTSCount - t.NonMonotonicPTSCount
	if positiveDeltaCount <= 0 || len(t.DeltaHistogram) == 0 {
		return fmt.Errorf("timeline has no positive cadence evidence")
	}

	minimumTicks := int64(0)
	maximumTicks := int64(0)
	dominantTicks := int64(0)
	dominantCount := 0
	totalCount := 0
	nearZeroCount := 0
	previousTicks := int64(0)
	for index, bucket := range t.DeltaHistogram {
		if bucket.DeltaTicks <= 0 || bucket.Count <= 0 {
			return fmt.Errorf("timeline delta bucket is invalid")
		}
		if index > 0 && bucket.DeltaTicks <= previousTicks {
			return fmt.Errorf("timeline delta histogram is not strictly ordered")
		}
		micros, err := transcodeboundary.TicksToMicros(bucket.DeltaTicks, t.TimeBase)
		if err != nil || bucket.DeltaMicros != micros {
			return fmt.Errorf("timeline delta bucket projection is inconsistent")
		}
		if index == 0 {
			minimumTicks = bucket.DeltaTicks
		}
		maximumTicks = bucket.DeltaTicks
		if bucket.Count > dominantCount || (bucket.Count == dominantCount && (dominantTicks == 0 || bucket.DeltaTicks < dominantTicks)) {
			dominantTicks = bucket.DeltaTicks
			dominantCount = bucket.Count
		}
		if bucket.DeltaMicros < NearZeroDeltaThresholdMicros {
			nearZeroCount += bucket.Count
		}
		totalCount += bucket.Count
		previousTicks = bucket.DeltaTicks
	}
	if totalCount != positiveDeltaCount {
		return fmt.Errorf("timeline delta histogram count is inconsistent")
	}
	minimumMicros, err := transcodeboundary.TicksToMicros(minimumTicks, t.TimeBase)
	if err != nil {
		return err
	}
	maximumMicros, err := transcodeboundary.TicksToMicros(maximumTicks, t.TimeBase)
	if err != nil {
		return err
	}
	dominantMicros, err := transcodeboundary.TicksToMicros(dominantTicks, t.TimeBase)
	if err != nil {
		return err
	}
	if t.MinDeltaTicks != minimumTicks || t.MaxDeltaTicks != maximumTicks ||
		t.MinDeltaMicros != minimumMicros || t.MaxDeltaMicros != maximumMicros ||
		t.DistinctDeltas != len(t.DeltaHistogram) || t.NearZeroDeltaCount != nearZeroCount ||
		t.DominantDeltaTicks != dominantTicks || t.DominantDeltaMicros != dominantMicros || t.DominantDeltaCount != dominantCount {
		return fmt.Errorf("timeline cadence summary is inconsistent")
	}
	spread := maximumMicros - minimumMicros
	if t.DurationSpreadMicros != spread || t.VariableDuration != (spread >= transcodesourceorigin.VFRSpreadThresholdMicros) {
		return fmt.Errorf("timeline raw cadence projection is inconsistent")
	}
	threshold := significantBucketMinimumCount(positiveDeltaCount)
	significantCount := 0
	outlierCount := 0
	significantMinMicros := int64(0)
	significantMaxMicros := int64(0)
	for _, bucket := range t.DeltaHistogram {
		if bucket.Count >= threshold {
			significantCount++
			if significantCount == 1 {
				significantMinMicros = bucket.DeltaMicros
			}
			significantMaxMicros = bucket.DeltaMicros
		} else {
			outlierCount += bucket.Count
		}
	}
	materialVariable := significantCount >= 2 && significantMaxMicros-significantMinMicros >= transcodesourceorigin.VFRSpreadThresholdMicros
	if t.SignificantBucketMinimumCount != threshold || t.SignificantDeltaCount != significantCount ||
		t.OutlierDeltaCount != outlierCount || t.MaterialVariableDuration != materialVariable {
		return fmt.Errorf("timeline significant cadence projection is inconsistent")
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
	return preservationFor(c)
}

func preservationFor(c Contract) string {
	sourcePolicyMatches := c.SourceTimeline.MaterialVariableDuration == (c.SourceMode == transcodesourceorigin.ModeVFR) &&
		c.SourceStartupTimeline.MaterialVariableDuration == c.ExpectedStartupMaterialVariable &&
		c.SourceContinuationTimeline.MaterialVariableDuration == c.ExpectedContinuationMaterialVariable
	outputCadenceMatches := c.StartupTimeline.MaterialVariableDuration == c.SourceStartupTimeline.MaterialVariableDuration &&
		c.ContinuationTimeline.MaterialVariableDuration == c.SourceContinuationTimeline.MaterialVariableDuration &&
		abs64(c.StartupTimeline.DominantDeltaMicros-c.SourceStartupTimeline.DominantDeltaMicros) <= CadenceDeltaToleranceMicros &&
		abs64(c.ContinuationTimeline.DominantDeltaMicros-c.SourceContinuationTimeline.DominantDeltaMicros) <= CadenceDeltaToleranceMicros
	introducedNearZero := c.StartupTimeline.NearZeroDeltaCount > c.SourceStartupTimeline.NearZeroDeltaCount ||
		c.ContinuationTimeline.NearZeroDeltaCount > c.SourceContinuationTimeline.NearZeroDeltaCount
	cleanPTS := timelinePTSIsClean(c.SourceTimeline) && timelinePTSIsClean(c.SourceStartupTimeline) &&
		timelinePTSIsClean(c.SourceContinuationTimeline) && timelinePTSIsClean(c.StartupTimeline) && timelinePTSIsClean(c.ContinuationTimeline)
	withinTolerance := absInt(c.StartupMapping.FrameCountDelta) <= FrameCountTolerance &&
		absInt(c.ContinuationMapping.FrameCountDelta) <= FrameCountTolerance
	exactCount := c.StartupMapping.FrameCountDelta == 0 && c.ContinuationMapping.FrameCountDelta == 0
	outputOutliers := c.StartupTimeline.OutlierDeltaCount+c.ContinuationTimeline.OutlierDeltaCount > 0
	if !sourcePolicyMatches || !outputCadenceMatches || introducedNearZero || !cleanPTS || !withinTolerance {
		return PreservationChanged
	}
	if outputOutliers {
		return PreservationWithCadenceOutliers
	}
	if exactCount {
		return PreservationExact
	}
	return PreservationWithCountTolerance
}

func timelinePTSIsClean(t TimelineEvidence) bool {
	return t.DuplicatePTSCount == 0 && t.NonMonotonicPTSCount == 0
}

func significantBucketMinimumCount(positiveDeltaCount int) int {
	threshold := (positiveDeltaCount + SignificantDeltaPercentDenominator - 1) / SignificantDeltaPercentDenominator
	if threshold < MinimumSignificantDeltaCount {
		threshold = MinimumSignificantDeltaCount
	}
	return threshold
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

func abs64(value int64) int64 {
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
