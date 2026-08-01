package avsync

import (
	"testing"

	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
)

func TestContractIdentityIsDeterministicAndFailClosed(t *testing.T) {
	source := validBoundaryContract()
	contract, err := FromBoundary(source)
	if err != nil {
		t.Fatal(err)
	}
	version1, hash1, canonical1, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	version2, hash2, canonical2, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	if version1 != SchemaVersion || version1 != version2 || hash1 != hash2 || canonical1 != canonical2 {
		t.Fatal("A/V boundary sync identity is not deterministic")
	}
	if err := contract.ValidateAgainst(source); err != nil {
		t.Fatalf("A/V boundary sync evidence did not match source: %v", err)
	}
	contract.SeamlessAllowed = true
	contract.DiscontinuityRequired = false
	if err := contract.Validate(); err == nil {
		t.Fatal("A/V boundary sync evidence authorized seamless playback")
	}
}

func TestContractRejectsSkewDrift(t *testing.T) {
	contract, err := FromBoundary(validBoundaryContract())
	if err != nil {
		t.Fatal(err)
	}
	contract.BoundaryDeltaSkewMicros++
	if err := contract.Validate(); err == nil {
		t.Fatal("inconsistent A/V skew transition was accepted")
	}
}

func TestClassifySkew(t *testing.T) {
	for _, test := range []struct {
		name string
		skew int64
		want string
	}{
		{name: "aligned", skew: -1000, want: StatusAligned},
		{name: "within packet", skew: 20_000, want: StatusWithinOnePacket},
		{name: "outside packet", skew: 50_000, want: StatusOutsideOnePacket},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Classify(test.skew, 2666, 33_333); got != test.want {
				t.Fatalf("Classify() = %q, want %q", got, test.want)
			}
		})
	}
}

func validBoundaryContract() transcodeboundary.Contract {
	videoStartup := packetWindow("seg0014.ts", transcodeboundary.WindowTail, 2_811_000, 3000, 6)
	videoContinuation := packetWindow("seg0015.ts", transcodeboundary.WindowHead, 2_826_000, 3000, 6)
	video := streamEvidence(transcodeboundary.StreamVideo, "1/90000", 0, 30_000, videoStartup, videoContinuation)

	audioStartup := packetWindow("seg0014.ts", transcodeboundary.WindowTail, 2_823_840, 1920, 6)
	audioContinuation := packetWindow("seg0015.ts", transcodeboundary.WindowHead, 2_824_080, 1920, 6)
	audio := streamEvidence(transcodeboundary.StreamAudio, "1/90000", 48_000, 0, audioStartup, audioContinuation)
	boundarySamples, _ := transcodeboundary.TicksToSamples(audio.PresentationDeltaTicks, audio.TimeBase, audio.SampleRate)
	audio.AudioDelay = &transcodeboundary.AudioDelayEvidence{
		NominalPacketSamples: 1024,
		BoundaryDeltaSamples: boundarySamples,
	}

	return transcodeboundary.Contract{
		SchemaVersion:                   transcodeboundary.SchemaVersion,
		CaseID:                         "shape-48k-per-stream-v1",
		FixtureID:                      "cfr-h264-aac-48k-software-zerolatency-v1",
		ExpectedBoundaryMicros:         30_000_000,
		FFmpegVersion:                  "ffmpeg version fixture",
		FFprobeVersion:                 "ffprobe version fixture",
		EncodingPlanVersion:            "hls-encoding-plan-v1",
		EncodingPlanHash:               "encoding-plan",
		TimestampPlanVersion:           "hls-timestamp-normalization-v1",
		TimestampPlanHash:              "timestamp-plan",
		StartupAttestationVersion:      "hls-produced-media-attestation-v1",
		StartupAttestationHash:         "startup-attestation",
		ContinuationAttestationVersion: "hls-produced-media-attestation-v1",
		ContinuationAttestationHash:    "continuation-attestation",
		Video:                          video,
		Audio:                          audio,
		DiscontinuityRequired:          true,
	}
}

func packetWindow(segmentName, position string, firstPTS, duration int64, count int) transcodeboundary.SegmentWindow {
	packets := make([]transcodeboundary.PacketEvidence, 0, count)
	for index := 0; index < count; index++ {
		pts := firstPTS + int64(index)*duration
		ptsMicros, _ := transcodeboundary.TicksToMicros(pts, "1/90000")
		durationMicros, _ := transcodeboundary.TicksToMicros(duration, "1/90000")
		packets = append(packets, transcodeboundary.PacketEvidence{
			Ordinal:        index,
			PTS:            pts,
			DTS:            pts,
			Duration:       duration,
			PTSMicros:      ptsMicros,
			DTSMicros:      ptsMicros,
			DurationMicros: durationMicros,
		})
	}
	return transcodeboundary.SegmentWindow{
		SegmentName:  segmentName,
		PacketCount:  count,
		Position:     position,
		Packets:      packets,
		EarliestPTS:  firstPTS,
		LatestEndPTS: firstPTS + int64(count)*duration,
		FirstDTS:     firstPTS,
		LastEndDTS:   firstPTS + int64(count)*duration,
	}
}

func streamEvidence(kind, timeBase string, sampleRate, frameRateMilli int, startup, continuation transcodeboundary.SegmentWindow) transcodeboundary.StreamEvidence {
	nominalTicks := startup.Packets[0].Duration
	nominalMicros, _ := transcodeboundary.TicksToMicros(nominalTicks, timeBase)
	presentationTicks := continuation.EarliestPTS - startup.LatestEndPTS
	decodeTicks := continuation.FirstDTS - startup.LastEndDTS
	presentationMicros, _ := transcodeboundary.TicksToMicros(presentationTicks, timeBase)
	decodeMicros, _ := transcodeboundary.TicksToMicros(decodeTicks, timeBase)
	tolerance := transcodeboundary.ToleranceMicros(nominalMicros)
	return transcodeboundary.StreamEvidence{
		Kind:                        kind,
		TimeBase:                    timeBase,
		SampleRate:                  sampleRate,
		FrameRateMilli:              frameRateMilli,
		Startup:                     startup,
		Continuation:                continuation,
		PresentationDeltaTicks:      presentationTicks,
		PresentationDeltaMicros:     presentationMicros,
		DecodeDeltaTicks:            decodeTicks,
		DecodeDeltaMicros:           decodeMicros,
		NominalPacketDurationTicks:  nominalTicks,
		NominalPacketDurationMicros: nominalMicros,
		ToleranceMicros:             tolerance,
		BoundaryUnitsMilli:          presentationTicks * 1000 / nominalTicks,
		Status:                      transcodeboundary.Classify(presentationMicros, nominalMicros, tolerance),
	}
}
