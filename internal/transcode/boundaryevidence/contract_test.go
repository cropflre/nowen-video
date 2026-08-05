package boundaryevidence

import "testing"

func TestContractIdentityIsDeterministicAndFailClosed(t *testing.T) {
	contract := validContract()
	version1, hash1, canonical1, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	version2, hash2, canonical2, err := Identity(contract)
	if err != nil {
		t.Fatal(err)
	}
	if version1 != SchemaVersion || version1 != version2 || hash1 != hash2 || canonical1 != canonical2 {
		t.Fatal("boundary evidence identity is not deterministic")
	}
	contract.SeamlessAllowed = true
	contract.DiscontinuityRequired = false
	if err := contract.Validate(); err == nil {
		t.Fatal("boundary evidence v1 authorized seamless playback")
	}
}

func TestContractRejectsDerivedDeltaDrift(t *testing.T) {
	contract := validContract()
	contract.Video.PresentationDeltaTicks++
	if err := contract.Validate(); err == nil {
		t.Fatal("inconsistent packet delta was accepted")
	}
}

func TestContractRejectsWindowOrdinalDrift(t *testing.T) {
	contract := validContract()
	contract.Audio.Continuation.Packets[0].Ordinal = 2
	if err := contract.Validate(); err == nil {
		t.Fatal("invalid packet window ordering was accepted")
	}
}

func TestContractRejectsAudioDelayProjectionDrift(t *testing.T) {
	contract := validContract()
	contract.Audio.AudioDelay.NominalPacketSamples = 960
	if err := contract.Validate(); err == nil {
		t.Fatal("invalid audio packet sample projection was accepted")
	}
}

func TestContainerSideDataDoesNotCountAsEncoderDelayEvidence(t *testing.T) {
	contract := validContract()
	contract.Audio.Continuation.Packets[0].SideData = []PacketSideData{{Type: "MPEGTS Stream ID"}}
	contract.Audio.AudioDelay.ContinuationSkipSamples = 0
	contract.Audio.AudioDelay.SideDataObserved = false
	if err := contract.Validate(); err != nil {
		t.Fatalf("container-only side data made contract invalid: %v", err)
	}
	if IsAudioDelaySideData(contract.Audio.Continuation.Packets[0].SideData[0]) {
		t.Fatal("MPEG-TS stream identity was treated as encoder delay evidence")
	}
	if !IsAudioDelaySideData(PacketSideData{Type: "Skip Samples"}) {
		t.Fatal("skip-sample side data was not recognized")
	}
}

func TestClassifyPacketQuantization(t *testing.T) {
	for _, test := range []struct {
		name    string
		delta   int64
		nominal int64
		tol     int64
		want    string
	}{
		{name: "aligned", delta: -1000, nominal: 33_333, tol: 4166, want: StatusAligned},
		{name: "one overlap", delta: -21_333, nominal: 21_333, tol: 2666, want: StatusSinglePacketOverlap},
		{name: "many overlap", delta: -58_667, nominal: 21_333, tol: 2666, want: StatusMultiPacketOverlap},
		{name: "one gap", delta: 20_000, nominal: 21_333, tol: 2666, want: StatusSinglePacketGap},
		{name: "many gap", delta: 60_000, nominal: 21_333, tol: 2666, want: StatusMultiPacketGap},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := Classify(test.delta, test.nominal, test.tol); got != test.want {
				t.Fatalf("Classify() = %q, want %q", got, test.want)
			}
		})
	}
}

func validContract() Contract {
	videoStartup := packetWindow("seg0014.ts", WindowTail, 2_811_000, 3000, 6, nil)
	videoContinuation := packetWindow("seg0015.ts", WindowHead, 2_826_000, 3000, 6, nil)
	video := streamEvidence(StreamVideo, "1/90000", 0, 30_000, videoStartup, videoContinuation)

	audioSideData := []PacketSideData{{Type: "Skip Samples", SkipSamples: 1024}}
	audioStartup := packetWindow("seg0014.ts", WindowTail, 2_823_840, 1920, 6, nil)
	audioContinuation := packetWindow("seg0015.ts", WindowHead, 2_824_080, 1920, 6, audioSideData)
	audio := streamEvidence(StreamAudio, "1/90000", 48_000, 0, audioStartup, audioContinuation)
	boundarySamples, _ := TicksToSamples(audio.PresentationDeltaTicks, audio.TimeBase, audio.SampleRate)
	audio.AudioDelay = &AudioDelayEvidence{
		NominalPacketSamples:    1024,
		BoundaryDeltaSamples:    boundarySamples,
		ContinuationSkipSamples: 1024,
		SideDataObserved:        true,
	}

	return Contract{
		SchemaVersion:                  SchemaVersion,
		CaseID:                         "boundary-48k-keyframe-v1",
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

func packetWindow(segmentName, position string, firstPTS, duration int64, count int, firstPacketSideData []PacketSideData) SegmentWindow {
	packets := make([]PacketEvidence, 0, count)
	for index := 0; index < count; index++ {
		pts := firstPTS + int64(index)*duration
		ptsMicros, _ := TicksToMicros(pts, "1/90000")
		durationMicros, _ := TicksToMicros(duration, "1/90000")
		packet := PacketEvidence{
			Ordinal:        index,
			PTS:            pts,
			DTS:            pts,
			Duration:       duration,
			PTSMicros:      ptsMicros,
			DTSMicros:      ptsMicros,
			DurationMicros: durationMicros,
		}
		if index == 0 {
			packet.SideData = firstPacketSideData
		}
		packets = append(packets, packet)
	}
	return SegmentWindow{
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

func streamEvidence(kind, timeBase string, sampleRate, frameRateMilli int, startup, continuation SegmentWindow) StreamEvidence {
	nominalTicks := startup.Packets[0].Duration
	nominalMicros, _ := TicksToMicros(nominalTicks, timeBase)
	presentationTicks := continuation.EarliestPTS - startup.LatestEndPTS
	decodeTicks := continuation.FirstDTS - startup.LastEndDTS
	presentationMicros, _ := TicksToMicros(presentationTicks, timeBase)
	decodeMicros, _ := TicksToMicros(decodeTicks, timeBase)
	return StreamEvidence{
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
		ToleranceMicros:             ToleranceMicros(nominalMicros),
		BoundaryUnitsMilli:          signedRatioMilli(presentationTicks, nominalTicks),
		Status:                      Classify(presentationMicros, nominalMicros, ToleranceMicros(nominalMicros)),
	}
}
