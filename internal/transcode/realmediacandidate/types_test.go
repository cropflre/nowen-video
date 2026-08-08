package realmediacandidate

import (
	"math"
	"strings"
	"testing"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
)

func TestCaseSpecForCoversDefaultCorpus(t *testing.T) {
	spec := transcodecorpus.DefaultSpec()
	for index, caseSpec := range spec.Cases {
		asset := validAsset(caseSpec, index)
		candidate, err := CaseSpecFor(caseSpec, asset)
		if err != nil {
			t.Fatalf("case %s: %v", caseSpec.ID, err)
		}
		if candidate.Base.ID != caseSpec.ID || candidate.Base.SourceOffsetMicros != asset.Probe.StartMicros {
			t.Fatalf("case %s identity was not preserved", caseSpec.ID)
		}
		if candidate.Base.AudioSampleRate != caseSpec.Source.Audio.SampleRate || candidate.Base.GOPSize != caseSpec.Source.Video.GOPSize {
			t.Fatalf("case %s media policy was not preserved", caseSpec.ID)
		}
		if candidate.BFrames != caseSpec.Source.Video.BFrames || candidate.ReferenceFrames != caseSpec.Source.Video.ReferenceFrames {
			t.Fatalf("case %s reorder policy was not preserved", caseSpec.ID)
		}
		wantMode := transcodesourceorigin.ModeCFR
		if caseSpec.Source.Video.FrameRateMode == transcodecorpus.FrameRateVFR {
			wantMode = transcodesourceorigin.ModeVFR
		}
		if candidate.Base.SourceMode != wantMode {
			t.Fatalf("case %s mode = %s, want %s", caseSpec.ID, candidate.Base.SourceMode, wantMode)
		}
	}
}

func TestCaseSpecForAccepts44100HzCorpusCase(t *testing.T) {
	caseSpec, ok := transcodecorpus.LookupCase(transcodecorpus.CaseMP4CFR30AAC44100)
	if !ok {
		t.Fatal("44.1 kHz corpus case is missing")
	}
	candidate, err := CaseSpecFor(caseSpec, validAsset(caseSpec, 0))
	if err != nil {
		t.Fatal(err)
	}
	if candidate.Base.AudioSampleRate != 44_100 {
		t.Fatalf("audio sample rate = %d, want 44100", candidate.Base.AudioSampleRate)
	}
}

func TestCaseSpecForRejectsAssetIdentityDrift(t *testing.T) {
	caseSpec := transcodecorpus.DefaultSpec().Cases[0]
	asset := validAsset(caseSpec, 0)
	asset.RepeatSHA256[1] = strings.Repeat("c", 64)
	if _, err := CaseSpecFor(caseSpec, asset); err == nil {
		t.Fatal("expected non-deterministic asset rejection")
	}
}

func validAsset(caseSpec transcodecorpus.CaseSpec, index int) transcodecorpus.AssetEvidence {
	plan := caseSpec.Source
	frameCount := 0
	if plan.Video.FrameRateMode == transcodecorpus.FrameRateCFR {
		rate := plan.Video.FrameRates[0]
		frameCount = int(math.Ceil(float64(plan.Timeline.DurationMicros) * float64(rate.Numerator) / (1_000_000 * float64(rate.Denominator))))
	} else {
		segmentMicros := float64(plan.Timeline.DurationMicros) / float64(len(plan.Video.FrameRates))
		for _, rate := range plan.Video.FrameRates {
			frameCount += int(math.Ceil(segmentMicros * float64(rate.Numerator) / (1_000_000 * float64(rate.Denominator))))
		}
	}
	sha := strings.Repeat("b", 64)
	return transcodecorpus.AssetEvidence{
		CaseID:        caseSpec.ID,
		RelativePath:  "assets/" + caseSpec.ID + ".media",
		CommandSHA256: strings.Repeat("a", 64),
		SHA256:        sha,
		RepeatSHA256:  []string{sha, sha},
		SizeBytes:     int64(1024 + index),
		Probe: transcodecorpus.ProbeEvidence{
			Container:                   plan.Container,
			DurationMicros:              plan.Timeline.DurationMicros,
			StartMicros:                 plan.Timeline.OriginMicros,
			VideoCodec:                  plan.Video.Codec,
			VideoProfile:                plan.Video.Profile,
			PixelFormat:                 plan.Video.PixelFormat,
			Width:                       plan.Video.Width,
			Height:                      plan.Video.Height,
			ColorPrimaries:              plan.Video.ColorPrimaries,
			ColorTransfer:               plan.Video.ColorTransfer,
			ColorMatrix:                 plan.Video.ColorMatrix,
			FrameRateMode:               plan.Video.FrameRateMode,
			ObservedRates:               append([]transcodecorpus.Rational(nil), plan.Video.FrameRates...),
			VideoTimeBase:               transcodecorpus.Rational{Numerator: 1, Denominator: 90_000},
			FrameCount:                  frameCount,
			KeyFrameCount:               2,
			MaxKeyFrameInterval:         plan.Video.GOPSize,
			MaxPresentationReorderDepth: plan.Video.BFrames,
			MaxCompositionOffsetMicros:  100_000,
			AudioCodec:                  plan.Audio.Codec,
			AudioSampleRate:             plan.Audio.SampleRate,
			AudioChannels:               plan.Audio.Channels,
			AudioTrackCount:             plan.Audio.TrackCount,
			AudioTimeBase:               transcodecorpus.Rational{Numerator: 1, Denominator: int64(plan.Audio.SampleRate)},
			HasBFrameReorder:            plan.Video.BFrames > 0,
			HasEditList:                 plan.Timeline.HasEditList,
		},
	}
}
