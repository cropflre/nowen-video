package vfrisolation

import (
	"strings"
	"testing"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
)

func TestContractIdentityIsDeterministic(t *testing.T) {
	contract := validContract(t)
	versionA, hashA, canonicalA, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	versionB, hashB, canonicalB, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	if versionA != SchemaVersion || versionA != versionB || hashA != hashB || canonicalA != canonicalB {
		t.Fatal("VFR isolation identity is not deterministic")
	}
}

func TestContractRejectsCopySequenceDrift(t *testing.T) {
	contract := validContract(t)
	contract.Variants[1].Fingerprint.SequenceSHA256 = strings.Repeat("2", 64)
	contract.Variants[1].SequenceMatchesReference = false
	if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "copy-only") {
		t.Fatalf("unexpected validation result: %v", err)
	}
}

func TestContractRejectsCadenceHistogramDrift(t *testing.T) {
	contract := validContract(t)
	contract.Variants[0].Timeline.DeltaHistogram[0].Count--
	if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "histogram count") {
		t.Fatalf("unexpected validation result: %v", err)
	}
}

func TestContractRejectsFrameMappingDrift(t *testing.T) {
	contract := validContract(t)
	contract.Variants[0].Mapping.FrameCountDelta = 1
	if err := contract.Validate(); err == nil || !strings.Contains(err.Error(), "mapping") {
		t.Fatalf("unexpected validation result: %v", err)
	}
}

func TestContractRejectsSeamlessAuthorization(t *testing.T) {
	contract := validContract(t)
	contract.SeamlessAllowed = true
	if err := contract.Validate(); err == nil {
		t.Fatal("VFR isolation evidence authorized seamless playback")
	}
}

func validContract(t *testing.T) Contract {
	t.Helper()
	sourceTicks := make([]int64, 300)
	for index := range sourceTicks {
		sourceTicks[index] = int64(index) * 3_000
	}
	source, err := transcodeoutputcadence.NewTimelineEvidence(
		transcodeoutputcadence.TimelineSourceContinuation,
		"1/90000",
		30_000_000,
		40_000_000,
		sourceTicks,
	)
	if err != nil {
		t.Fatal(err)
	}
	output, err := transcodeoutputcadence.NewTimelineEvidence(
		"isolation_production-hls-v1",
		"1/90000",
		30_000_000,
		40_000_000,
		sourceTicks,
	)
	if err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("0", 64)
	fingerprint := FrameFingerprint{
		FrameCount: 300, UniqueFrameCount: 300,
		SequenceSHA256: hash, FirstFrameSHA256: hash, LastFrameSHA256: hash,
	}
	baseline := VariantEvidence{
		Spec: VariantSpec{
			ID: "production-hls-v1", Description: "baseline", Layer: "production", Container: "hls-mpegts",
			FPSMode: "passthrough", EncoderTimeBase: "auto",
		},
		CommandHash: hash, Timeline: output,
		Mapping: transcodeoutputcadence.NewFrameMapping(300, 300), Fingerprint: fingerprint,
		CadenceClassification: ClassificationPreserved, SequenceReference: SequenceReferenceNone,
	}
	copyVariant := baseline
	copyVariant.Spec = VariantSpec{
		ID: "matroska-remux-mpegts-v1", Description: "copy", Layer: "mpegts_muxer", Container: "mpegts",
		FPSMode: "not_applicable", EncoderTimeBase: "copy", CopyOnly: true, ParentVariantID: baseline.Spec.ID,
	}
	copyVariant.Timeline.Kind = "isolation_" + copyVariant.Spec.ID
	copyVariant.SequenceReference = SequenceReferenceParent
	copyVariant.SequenceReferenceVariantID = baseline.Spec.ID
	copyVariant.SequenceMatchesReference = true
	contract := Contract{
		SchemaVersion: SchemaVersion, CaseID: "source-vfr-24-30-origin-zero-v1", FixtureID: "source-vfr-v1",
		WindowStartMicros: 30_000_000, WindowEndMicros: 40_000_000,
		FFmpegVersion: "ffmpeg test", FFprobeVersion: "ffprobe test",
		BaselineOutputCadenceVersion: transcodeoutputcadence.SchemaVersion, BaselineOutputCadenceHash: hash,
		SourceTimeline: source, Variants: []VariantEvidence{baseline, copyVariant}, DiscontinuityRequired: true,
	}
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	return contract
}
