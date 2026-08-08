package model

import (
	"math"
	"testing"
)

func TestMediaProbeFrameRateAndGOP(t *testing.T) {
	probe := &MediaProbeRecord{FrameRateNum: 24000, FrameRateDen: 1001}
	if got := probe.FrameRate(); math.Abs(got-23.976023976) > 0.0001 {
		t.Fatalf("unexpected frame rate: %.9f", got)
	}
	if got := probe.GOPSize(2); got != 48 {
		t.Fatalf("expected 48-frame GOP for 23.976fps/2s, got %d", got)
	}
}

func TestMediaProbeGOPFallbackAndClamp(t *testing.T) {
	if got := (&MediaProbeRecord{}).GOPSize(2); got != 50 {
		t.Fatalf("expected 25fps compatibility fallback, got %d", got)
	}
	if got := (&MediaProbeRecord{FrameRateNum: 1, FrameRateDen: 1}).GOPSize(1); got != 12 {
		t.Fatalf("minimum GOP clamp missing: %d", got)
	}
	if got := (&MediaProbeRecord{FrameRateNum: 1000, FrameRateDen: 1}).GOPSize(2); got != 240 {
		t.Fatalf("maximum GOP clamp missing: %d", got)
	}
}

func TestMediaProbeAudioStreamsRoundTrip(t *testing.T) {
	probe := &MediaProbeRecord{}
	want := []MediaProbeAudioStream{{
		Index:         2,
		Codec:         "truehd",
		Channels:      8,
		ChannelLayout: "7.1",
		SampleRate:    48000,
		Language:      "jpn",
		Default:       true,
	}}
	if err := probe.SetAudioStreams(want); err != nil {
		t.Fatal(err)
	}
	got := probe.AudioStreams()
	if len(got) != 1 || got[0].Codec != "truehd" || got[0].Channels != 8 || !got[0].Default {
		t.Fatalf("unexpected audio streams: %+v", got)
	}
}
