package certification

import (
	"testing"

	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func TestLongDurationDriftHLSArgsScopeStreamLoopToInput(t *testing.T) {
	caseSpec := transcodereorder.CaseSpec{
		Base: transcodetimebase.CaseSpec{
			ID:                          transcodelongdrift.SourceCaseID,
			Description:                 "long duration drift",
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
	args, err := longDurationDriftHLSArgs("/tmp/source.mp4", "/tmp/output", transcodetimestamp.Default(), caseSpec, candidate)
	if err != nil {
		t.Fatal(err)
	}
	loopIndex := indexOfArg(args, "-stream_loop")
	inputIndex := indexOfArg(args, "-i")
	if loopIndex < 0 || inputIndex < 0 || loopIndex >= inputIndex {
		t.Fatalf("stream loop is not input-scoped: %v", args)
	}
	if args[loopIndex+1] != "-1" || args[inputIndex+1] != "/tmp/source.mp4" {
		t.Fatalf("stream loop or input value is invalid: %v", args)
	}
	if !containsArgPair(args, "-enc_time_base:v:0", candidate.EncoderTimeBase) {
		t.Fatalf("explicit encoder time base is missing: %v", args)
	}
	if !containsArgPair(args, "-t", "1800.00") {
		t.Fatalf("30-minute output bound is missing: %v", args)
	}
}

func TestBuildLongDriftStreamEvidenceUsesExactCheckpoints(t *testing.T) {
	const ticksPerSecond = int64(90_000)
	points := make([]longDriftPoint, 0, 30*60)
	for second := int64(0); second < 30*60; second++ {
		points = append(points, longDriftPoint{
			PTSTicks:      second * ticksPerSecond,
			DurationTicks: ticksPerSecond,
		})
	}
	evidence, err := buildLongDriftStreamEvidence("video", "1/90000", points)
	if err != nil {
		t.Fatal(err)
	}
	if evidence.StartMicros != 0 || evidence.EndMicros != transcodelongdrift.DurationMicros || evidence.EndErrorMicros != 0 {
		t.Fatalf("unexpected stream boundary evidence: %+v", evidence)
	}
	if len(evidence.Checkpoints) != len(transcodelongdrift.CheckpointTargets()) {
		t.Fatalf("unexpected checkpoint count: %d", len(evidence.Checkpoints))
	}
	for _, checkpoint := range evidence.Checkpoints {
		if checkpoint.PresentationMicros != checkpoint.TargetMicros || checkpoint.ErrorMicros != 0 {
			t.Fatalf("checkpoint drifted: %+v", checkpoint)
		}
	}
}

func indexOfArg(args []string, value string) int {
	for index, candidate := range args {
		if candidate == value {
			return index
		}
	}
	return -1
}

func containsArgPair(args []string, key, value string) bool {
	for index := 0; index+1 < len(args); index++ {
		if args[index] == key && args[index+1] == value {
			return true
		}
	}
	return false
}
