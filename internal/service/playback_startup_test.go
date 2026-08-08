package service

import "testing"

func TestChooseTranscodeOrStartupAlwaysUsesEphemeralSession(t *testing.T) {
	stream := &StreamService{}
	plan, err := stream.chooseTranscodeOrStartup(
		&PlaybackPlan{MediaID: "media"},
		"media",
		PlaybackClientCapabilities{MaxBitrate: 2_000_000},
		"/api/stream/media/master.m3u8?maxBitrate=2000000",
		"codec_or_container_unsupported",
		"需要兼容转码",
	)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Method != PlaybackMethodTranscode {
		t.Fatalf("runtime playback must use transcode session: %+v", plan)
	}
	if !plan.SessionRequired || plan.SessionTemplate == nil {
		t.Fatalf("runtime playback must require an ephemeral session: %+v", plan)
	}
	if plan.URL != "" || plan.StartupStream != nil {
		t.Fatalf("persistent runtime URLs and startup artifacts must not be planned: %+v", plan)
	}
}
