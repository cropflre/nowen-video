package service

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestPlaybackTechnicalFromProbeUsesDefaultAudioAndPreservesDiagnostics(t *testing.T) {
	probe := &model.MediaProbeRecord{
		ProbeVersion: model.MediaProbeVersion,
		VideoCodec:  "hevc",
		Width:       3840,
		Height:      2160,
		FrameRateNum: 24000,
		FrameRateDen: 1001,
		PixelFormat: "yuv420p10le",
		BitDepth:    10,
		HDR:         true,
	}
	if err := probe.SetAudioStreams([]model.MediaProbeAudioStream{
		{Index: 1, Codec: "aac"},
		{Index: 2, Codec: "truehd", Default: true},
		{Index: 3, Codec: "dts"},
	}); err != nil {
		t.Fatal(err)
	}
	technical, preferred := playbackTechnicalFromProbe(probe)
	if preferred != "truehd" {
		t.Fatalf("default audio was not selected: %s", preferred)
	}
	if technical == nil || technical.VideoCodec != "hevc" || technical.BitDepth != 10 || !technical.HDR {
		t.Fatalf("unexpected technical projection: %+v", technical)
	}
	if len(technical.AudioCodecs) != 3 {
		t.Fatalf("all audio codecs must remain visible for diagnostics: %+v", technical.AudioCodecs)
	}
}

func TestApplyProbeRecomputesDirectAndRemuxCapabilities(t *testing.T) {
	probe := &model.MediaProbeRecord{VideoCodec: "h264", DurationMS: 90000}
	if err := probe.SetAudioStreams([]model.MediaProbeAudioStream{{Codec: "aac", Default: true}}); err != nil {
		t.Fatal(err)
	}
	info := &MediaPlayInfo{FileExt: ".mp4", VideoCodec: "unknown", AudioCodec: "unknown"}
	applyProbeToPlaybackInfo("media-1", info, probe)
	if !info.CanDirectPlay || info.CanRemux || info.DirectPlayURL == "" {
		t.Fatalf("fresh probe did not enable direct play: %+v", info)
	}
	if info.Duration != 90 {
		t.Fatalf("duration was not refreshed: %f", info.Duration)
	}

	info = &MediaPlayInfo{FileExt: ".mkv"}
	applyProbeToPlaybackInfo("media-1", info, probe)
	if info.CanDirectPlay || !info.CanRemux || info.RemuxURL == "" {
		t.Fatalf("fresh probe did not enable zero-copy remux: %+v", info)
	}
}

func TestApplyProbeSelectsSmartRemuxForIncompatibleDefaultAudio(t *testing.T) {
	probe := &model.MediaProbeRecord{VideoCodec: "h264"}
	if err := probe.SetAudioStreams([]model.MediaProbeAudioStream{
		{Codec: "aac"},
		{Codec: "truehd", Default: true},
	}); err != nil {
		t.Fatal(err)
	}
	info := &MediaPlayInfo{FileExt: ".mkv"}
	applyProbeToPlaybackInfo("media-1", info, probe)
	if info.CanRemux {
		t.Fatalf("TrueHD default audio must not use zero-copy remux: %+v", info)
	}
	if !canSmartRemuxInfo(info, PlaybackClientCapabilities{SupportsRemux: true}) {
		t.Fatalf("H.264 + TrueHD should use smart remux: %+v", info)
	}
}
