package certification

import (
	"testing"

	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func TestLongDurationHLSArgsForPolicyUsesTwoHourBound(t *testing.T) {
	policy := transcodelongdrift.DefaultPolicy()
	policy.DurationMicros = 2 * 60 * 60 * 1_000_000
	policy.CheckpointIntervalMicros = 30 * 60 * 1_000_000
	policy.RepeatCount = 1
	policy.RepeatVarianceToleranceMicros = 0
	caseSpec := transcodereorder.CaseSpec{
		Base: transcodetimebase.CaseSpec{
			ID:                          transcodelongdrift.SourceCaseID,
			Description:                 "two hour scaling",
			SourceMode:                  transcodesourceorigin.ModeCFR,
			PrimaryFrameRateNumerator:   30,
			PrimaryFrameRateDenominator: 1,
			AudioSampleRate:             44_100,
			GOPSize:                     60,
			ExpectedBoundaryMicros:      30_000_000,
			DurationMicros:              40_000_000,
		},
		BFrames:         3,
		BAdapt:          0,
		ReferenceFrames: 3,
	}
	candidate := transcodetimebase.CandidateSpec{
		ID:              transcodetimebase.CandidateAVTB,
		Description:     "AVTB",
		EncoderTimeBase: "1/1000000",
	}
	args, err := longDurationHLSArgsForPolicy("/tmp/source.mp4", "/tmp/output", transcodetimestamp.Default(), caseSpec, candidate, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !containsArgPair(args, "-t", "7200.00") {
		t.Fatalf("two-hour output bound is missing: %v", args)
	}
	if indexOfArg(args, "-stream_loop") >= indexOfArg(args, "-i") {
		t.Fatalf("stream loop is not input-scoped: %v", args)
	}
}

func TestBuildLongDriftStreamEvidenceForPolicyUsesSixHourCheckpoints(t *testing.T) {
	tier, ok := transcodelongdrift.LookupScalingTier(transcodelongdrift.ScalingTierDepth6H)
	if !ok {
		t.Fatal("six-hour tier is missing")
	}
	policy := tier.Policy()
	const ticksPerHour = int64(90_000 * 60 * 60)
	points := make([]longDriftPoint, 0, 6)
	for hour := int64(0); hour < 6; hour++ {
		points = append(points, longDriftPoint{
			PTSTicks:      hour * ticksPerHour,
			DurationTicks: ticksPerHour,
		})
	}
	evidence, err := buildLongDriftStreamEvidenceForPolicy("video", "1/90000", points, policy)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.DurationMicros != policy.DurationMicros || evidence.EndErrorMicros != 0 {
		t.Fatalf("unexpected six-hour evidence: %+v", evidence)
	}
	if len(evidence.Checkpoints) != 7 {
		t.Fatalf("unexpected checkpoint count: %d", len(evidence.Checkpoints))
	}
	for _, checkpoint := range evidence.Checkpoints {
		if checkpoint.PresentationMicros != checkpoint.TargetMicros || checkpoint.ErrorMicros != 0 {
			t.Fatalf("checkpoint drifted: %+v", checkpoint)
		}
	}
}

func TestAggregateLongDurationScalingRequiresAllShards(t *testing.T) {
	if _, err := AggregateLongDurationScalingShardReports(nil); err == nil {
		t.Fatal("empty scaling aggregate was accepted")
	}
}
