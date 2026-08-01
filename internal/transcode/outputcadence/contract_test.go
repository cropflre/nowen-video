package outputcadence

import (
	"strings"
	"testing"

	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func TestContractIdentityIsDeterministic(t *testing.T) {
	contract := validContract()
	versionA, hashA, canonicalA, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	versionB, hashB, canonicalB, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	if versionA != SchemaVersion || versionA != versionB || hashA != hashB || canonicalA != canonicalB {
		t.Fatal("output cadence identity is not deterministic")
	}
}

func TestFrameMappingProjection(t *testing.T) {
	for _, test := range []struct {
		input, output int
		status        string
		duplicates    int
		drops         int
	}{
		{100, 100, MappingAligned, 0, 0},
		{100, 101, MappingWithinTolerance, 1, 0},
		{100, 102, MappingDuplicateProjection, 2, 0},
		{100, 98, MappingDropProjection, 0, 2},
	} {
		mapping := NewFrameMapping(test.input, test.output)
		if mapping.Status != test.status || mapping.ProjectedDuplicateFrames != test.duplicates || mapping.ProjectedDroppedFrames != test.drops {
			t.Fatalf("mapping %+v does not match expected projection", mapping)
		}
		if err := mapping.validate(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestTimelineSeparatesRareOutlierFromMaterialCadence(t *testing.T) {
	pts := make([]int64, 0, 101)
	current := int64(0)
	pts = append(pts, current)
	for index := 0; index < 100; index++ {
		delta := int64(33_333)
		if index == 50 {
			delta = 11
		}
		current += delta
		pts = append(pts, current)
	}
	evidence, err := NewTimelineEvidence(TimelineStartup, "1/1000000", 0, 4_000_000, pts)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.VariableDuration || evidence.MaterialVariableDuration {
		t.Fatalf("rare outlier classification = raw:%t material:%t", evidence.VariableDuration, evidence.MaterialVariableDuration)
	}
	if evidence.OutlierDeltaCount != 1 || evidence.SignificantDeltaCount != 1 || evidence.DominantDeltaMicros != 33_333 {
		t.Fatalf("unexpected outlier evidence: %+v", evidence)
	}
}

func TestTimelineRecognizesMaterialVFR(t *testing.T) {
	pts := make([]int64, 0, 101)
	current := int64(0)
	pts = append(pts, current)
	for index := 0; index < 100; index++ {
		if index < 50 {
			current += 41_667
		} else {
			current += 33_333
		}
		pts = append(pts, current)
	}
	evidence, err := NewTimelineEvidence(TimelineStartup, "1/1000000", 0, 4_000_000, pts)
	if err != nil {
		t.Fatal(err)
	}
	if !evidence.VariableDuration || !evidence.MaterialVariableDuration || evidence.OutlierDeltaCount != 0 {
		t.Fatalf("material VFR classification is invalid: %+v", evidence)
	}
}

func TestContractRejectsContentDuplicateClaim(t *testing.T) {
	contract := validContract()
	contract.ContentDuplicateClassification = "no_duplicates"
	if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "content-level") {
		t.Fatalf("unexpected validation result: %v", err)
	}
}

func TestContractRejectsSeamlessAuthorization(t *testing.T) {
	contract := validContract()
	contract.SeamlessAllowed = true
	if err := contract.Validate(); err == nil {
		t.Fatal("output cadence evidence authorized seamless playback")
	}
}

func validContract() Contract {
	hash := strings.Repeat("0", 64)
	contract := Contract{
		SchemaVersion: SchemaVersion,
		CaseID: "case-v1", FixtureID: "fixture-v1", SourceMode: transcodesourceorigin.ModeCFR,
		DeclaredFrameRateNumerator: 30, DeclaredFrameRateDenominator: 1, DeclaredFrameRateMilli: 30_000,
		ExpectedBoundaryMicros: 30_000_000,
		FFmpegVersion: "ffmpeg test", FFprobeVersion: "ffprobe test",
		SourceOriginVersion: transcodesourceorigin.SchemaVersion, SourceOriginHash: hash,
		TimestampPlanVersion: transcodetimestamp.SchemaVersion, TimestampPlanHash: hash,
		BoundaryEvidenceVersion: transcodeboundary.SchemaVersion, BoundaryEvidenceHash: hash,
		AVSyncEvidenceVersion: transcodeavsync.SchemaVersion, AVSyncEvidenceHash: hash,
		SourceTimeline: timeline(TimelineSource, 0, 40_000_000, 1_200),
		StartupTimeline: timeline(TimelineStartup, 0, 30_000_000, 900),
		ContinuationTimeline: timeline(TimelineContinuation, 30_000_000, 40_000_000, 300),
		StartupMapping: NewFrameMapping(900, 900),
		ContinuationMapping: NewFrameMapping(300, 300),
		ContentDuplicateClassification: ContentDuplicateNotMeasured,
		DiscontinuityRequired: true,
	}
	contract.PreservationStatus = PreservationFor(contract)
	return contract
}

func timeline(kind string, start, end int64, count int) TimelineEvidence {
	pts := make([]int64, count)
	for index := range pts {
		pts[index] = start + int64(index)*33_333
	}
	evidence, err := NewTimelineEvidence(kind, "1/1000000", start, end, pts)
	if err != nil {
		panic(err)
	}
	return evidence
}
