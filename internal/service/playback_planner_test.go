package service

import "testing"

func TestPlaybackPlannerCodecDetection(t *testing.T) {
	for _, codec := range []string{"h265", "hevc", "H.265", "hevc-main10"} {
		if !isHEVCCodec(codec) {
			t.Fatalf("expected %q to be detected as HEVC", codec)
		}
	}
	if isHEVCCodec("h264") {
		t.Fatal("h264 must not be detected as HEVC")
	}
}

func TestPlaybackPlannerAppendQuery(t *testing.T) {
	actual := appendQuery("/api/stream/m1/master.m3u8", "maxBitrate", "3000000")
	if actual != "/api/stream/m1/master.m3u8?maxBitrate=3000000" {
		t.Fatalf("unexpected URL: %s", actual)
	}

	actual = appendQuery("/api/stream/m1/master.m3u8?token=x", "maxBitrate", "3000000")
	if actual != "/api/stream/m1/master.m3u8?maxBitrate=3000000&token=x" && actual != "/api/stream/m1/master.m3u8?token=x&maxBitrate=3000000" {
		t.Fatalf("query parameter should be preserved: %s", actual)
	}
}

func TestChooseTranscode(t *testing.T) {
	plan := chooseTranscode(&PlaybackPlan{MediaID: "m1"}, "/hls", "unsupported", "需要转码")
	if plan.Method != PlaybackMethodTranscode || plan.URL != "/hls" || !plan.RequiresTranscode {
		t.Fatalf("unexpected transcode plan: %+v", plan)
	}
}
