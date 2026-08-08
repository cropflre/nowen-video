package realmediacorpus

const (
	CaseMP4CFR24000Over1001 = "real-mp4-h264-aac-cfr-24000-1001-v1"
	CaseMP4CFR30000EditList = "real-mp4-h264-aac-cfr-30000-1001-edit-list-v1"
	CaseMKVVFR24To30        = "real-mkv-h264-aac-vfr-24-30-v1"
	CaseTSCFR30B3           = "real-mpegts-h264-aac-cfr-30-b3-v1"
	CaseMKVCFR25Opus        = "real-mkv-h264-opus-cfr-25-v1"
	CaseMP4CFR30AAC44100    = "real-mp4-h264-aac-cfr-30-aac-44100-v1"
)

const (
	defaultDurationMicros = int64(40_000_000)
	defaultBoundaryMicros = int64(30_000_000)
)

func DefaultSpec() Spec {
	return Spec{
		SchemaVersion: SpecSchemaVersion,
		Cases: []CaseSpec{
			{
				ID:          CaseMP4CFR24000Over1001,
				Description: "MP4 H.264/AAC CFR 24000/1001 with deterministic B-frame reorder",
				Purpose:     "exercise cinematic rational cadence through the MP4 demuxer",
				Tier:        TierDeterministicContainer,
				Source: sourcePlan(
					ContainerMP4,
					FrameRateCFR,
					[]Rational{{Numerator: 24_000, Denominator: 1_001}},
					48, 2, 3,
					CodecAAC, 48_000,
					0, false,
				),
				BoundaryMicros:   defaultBoundaryMicros,
				RequiredEvidence: evidenceSet(),
			},
			{
				ID:          CaseMP4CFR30000EditList,
				Description: "MP4 H.264/AAC CFR 30000/1001 with positive origin and edit-list metadata",
				Purpose:     "exercise MP4 edit-list and non-zero-origin demuxer behavior",
				Tier:        TierDeterministicContainer,
				Source: sourcePlan(
					ContainerMP4,
					FrameRateCFR,
					[]Rational{{Numerator: 30_000, Denominator: 1_001}},
					60, 3, 4,
					CodecAAC, 48_000,
					5_000_000, true,
				),
				BoundaryMicros:   defaultBoundaryMicros,
				RequiredEvidence: evidenceSet(),
			},
			{
				ID:          CaseMKVVFR24To30,
				Description: "Matroska H.264/AAC variable cadence switching from 24 to 30 fps",
				Purpose:     "exercise a file-backed VFR timeline without MP4 edit-list semantics",
				Tier:        TierDeterministicContainer,
				Source: sourcePlan(
					ContainerMatroska,
					FrameRateVFR,
					[]Rational{{Numerator: 24, Denominator: 1}, {Numerator: 30, Denominator: 1}},
					60, 3, 4,
					CodecAAC, 48_000,
					0, false,
				),
				BoundaryMicros:   defaultBoundaryMicros,
				RequiredEvidence: evidenceSet(),
			},
			{
				ID:          CaseTSCFR30B3,
				Description: "MPEG-TS H.264/AAC CFR 30 with three-frame B reorder and positive transport origin",
				Purpose:     "exercise transport-stream packet timestamps before HLS re-encoding",
				Tier:        TierDeterministicContainer,
				Source: sourcePlan(
					ContainerMPEGTS,
					FrameRateCFR,
					[]Rational{{Numerator: 30, Denominator: 1}},
					60, 3, 4,
					CodecAAC, 48_000,
					1_400_000, false,
				),
				BoundaryMicros:   defaultBoundaryMicros,
				RequiredEvidence: evidenceSet(),
			},
			{
				ID:          CaseMKVCFR25Opus,
				Description: "Matroska H.264/Opus CFR 25 with two-frame B reorder",
				Purpose:     "exercise a non-AAC input audio codec and Matroska time bases",
				Tier:        TierDeterministicContainer,
				Source: sourcePlan(
					ContainerMatroska,
					FrameRateCFR,
					[]Rational{{Numerator: 25, Denominator: 1}},
					50, 2, 3,
					CodecOpus, 48_000,
					0, false,
				),
				BoundaryMicros:   defaultBoundaryMicros,
				RequiredEvidence: evidenceSet(),
			},
			{
				ID:          CaseMP4CFR30AAC44100,
				Description: "MP4 H.264/AAC CFR 30 with 44.1 kHz stereo audio",
				Purpose:     "exercise audio resampling and boundary projection from 44.1 kHz input",
				Tier:        TierDeterministicContainer,
				Source: sourcePlan(
					ContainerMP4,
					FrameRateCFR,
					[]Rational{{Numerator: 30, Denominator: 1}},
					60, 3, 4,
					CodecAAC, 44_100,
					0, false,
				),
				BoundaryMicros:   defaultBoundaryMicros,
				RequiredEvidence: evidenceSet(),
			},
		},
		SeamlessAllowed:       false,
		DiscontinuityRequired: true,
	}
}

func LookupCase(id string) (CaseSpec, bool) {
	for _, caseSpec := range DefaultSpec().Cases {
		if caseSpec.ID == id {
			return caseSpec, true
		}
	}
	return CaseSpec{}, false
}

func sourcePlan(
	container,
	frameRateMode string,
	frameRates []Rational,
	gopSize,
	bFrames,
	referenceFrames int,
	audioCodec string,
	audioSampleRate int,
	originMicros int64,
	hasEditList bool,
) SourcePlan {
	return SourcePlan{
		Container: container,
		Video: VideoPlan{
			Codec:           CodecH264,
			Profile:         "high",
			PixelFormat:     "yuv420p",
			Width:           640,
			Height:          360,
			FrameRateMode:   frameRateMode,
			FrameRates:      append([]Rational(nil), frameRates...),
			GOPSize:         gopSize,
			BFrames:         bFrames,
			ReferenceFrames: referenceFrames,
			OpenGOP:         false,
			Interlaced:      false,
			HDR:             false,
			ColorPrimaries:  "bt709",
			ColorTransfer:   "bt709",
			ColorMatrix:     "bt709",
		},
		Audio: AudioPlan{
			Codec:      audioCodec,
			SampleRate: audioSampleRate,
			Channels:   2,
			Layout:     "stereo",
			TrackCount: 1,
		},
		Timeline: TimelinePlan{
			DurationMicros: defaultDurationMicros,
			OriginMicros:   originMicros,
			HasEditList:    hasEditList,
			Discontinuous:  false,
		},
	}
}

func evidenceSet() []string {
	return append([]string(nil), requiredEvidence...)
}
