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
