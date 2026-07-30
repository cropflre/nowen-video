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

func TestPlanPlaybackWithInfoDirect(t *testing.T) {
	stream := &StreamService{}
	plan, err := stream.PlanPlaybackWithInfo("m1", &MediaPlayInfo{
		MediaID:          "m1",
		CanDirectPlay:    true,
		PreferDirectPlay: true,
		VideoCodec:       "h264",
		HlsURL:           "/hls",
	}, PlaybackClientCapabilities{SupportsDirectPlay: true, SupportsRemux: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != PlaybackMethodDirect || plan.URL != "/api/stream/m1/direct" {
		t.Fatalf("unexpected direct plan: %+v", plan)
	}
}

func TestPlanPlaybackWithInfoRemux(t *testing.T) {
	stream := &StreamService{}
	plan, err := stream.PlanPlaybackWithInfo("m1", &MediaPlayInfo{
		MediaID:          "m1",
		CanRemux:         true,
		PreferDirectPlay: true,
		VideoCodec:       "h264",
		HlsURL:           "/hls",
	}, PlaybackClientCapabilities{SupportsDirectPlay: true, SupportsRemux: true})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != PlaybackMethodRemux || plan.URL != "/api/stream/m1/remux" {
		t.Fatalf("unexpected remux plan: %+v", plan)
	}
}

func TestPlanPlaybackWithInfoRejectsUnsupportedHEVC(t *testing.T) {
	stream := &StreamService{}
	plan, err := stream.PlanPlaybackWithInfo("m1", &MediaPlayInfo{
		MediaID:          "m1",
		CanDirectPlay:    true,
		CanRemux:         true,
		PreferDirectPlay: true,
		VideoCodec:       "hevc-main10",
		HlsURL:           "/hls",
	}, PlaybackClientCapabilities{SupportsDirectPlay: true, SupportsRemux: true, SupportsHEVC: false})
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != PlaybackMethodTranscode || plan.ReasonCode != "client_hevc_unsupported" {
		t.Fatalf("unexpected HEVC fallback plan: %+v", plan)
	}
}
