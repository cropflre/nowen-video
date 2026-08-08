package sourceorigin

import "testing"

func TestContractValidatesStableCFRSource(t *testing.T) {
	contract := validContract()
	if err := contract.Validate(); err != nil {
		t.Fatal(err)
	}
	version, hash, canonical, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	if version != SchemaVersion || len(hash) != 64 || canonical == "" {
		t.Fatalf("unexpected identity: version=%s hash=%s", version, hash)
	}
}

func TestContractRejectsCadenceMismatch(t *testing.T) {
	contract := validContract()
	contract.SourceMode = ModeVFR
	if err := contract.Validate(); err == nil {
		t.Fatal("VFR source without variable packet duration was accepted")
	}
}

func TestContractRejectsOriginMismatch(t *testing.T) {
	contract := validContract()
	contract.SourceOffsetMicros = 5_000_000
	contract.OriginClass = OriginPositive
	if err := contract.Validate(); err == nil {
		t.Fatal("source origin mismatch was accepted")
	}
}

func TestContractRejectsSeamlessAuthorization(t *testing.T) {
	contract := validContract()
	contract.SeamlessAllowed = true
	if err := contract.Validate(); err == nil {
		t.Fatal("source origin evidence authorized seamless playback")
	}
}

func validContract() Contract {
	return Contract{
		SchemaVersion:                 SchemaVersion,
		CaseID:                        "source-cfr-25-origin-zero-v1",
		FixtureID:                     "source-origin-cfr-25-v1",
		SourceMode:                    ModeCFR,
		DeclaredFrameRateNumerator:    25,
		DeclaredFrameRateDenominator:  1,
		DeclaredFrameRateMilli:        25_000,
		OriginClass:                   OriginZero,
		OriginToleranceMicros:         MaxOriginErrorMicros,
		ExpectedBoundaryMicros:        30_000_000,
		FFmpegVersion:                 "ffmpeg test",
		FFprobeVersion:                "ffprobe test",
		TimestampPlanVersion:          "hls-timestamp-normalization-v1",
		TimestampPlanHash:             repeatHex('a'),
		BoundaryEvidenceVersion:       "hls-boundary-packet-evidence-v1",
		BoundaryEvidenceHash:          repeatHex('b'),
		AVSyncEvidenceVersion:         "hls-av-boundary-sync-evidence-v1",
		AVSyncEvidenceHash:            repeatHex('c'),
		SourceVideo:                   stream(StreamVideo, 40),
		SourceAudio:                   stream(StreamAudio, 21),
		NormalizedStartupVideoStartMS: 1_400,
		NormalizedStartupAudioStartMS: 1_379,
		NormalizedContinuationVideoMS: 31_400,
		NormalizedContinuationAudioMS: 31_379,
		DiscontinuityRequired:         true,
	}
}

func stream(kind string, duration int64) StreamEvidence {
	return StreamEvidence{
		Kind:                    kind,
		TimeBase:                "1/1000",
		PacketCount:             100,
		FirstPTS:                0,
		FirstDTS:                0,
		FirstPTSMicros:          0,
		FirstDTSMicros:          0,
		MinPacketDurationTicks:  duration,
		MaxPacketDurationTicks:  duration,
		MinPacketDurationMicros: duration * 1000,
		MaxPacketDurationMicros: duration * 1000,
		DistinctDurations:       1,
	}
}

func repeatHex(value byte) string {
	result := make([]byte, 64)
	for index := range result {
		result[index] = value
	}
	return string(result)
}
