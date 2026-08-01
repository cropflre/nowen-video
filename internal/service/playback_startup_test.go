package service

import "testing"

func TestStartupStreamCanBePlanned(t *testing.T) {
	valid := &StartupBridgeInfo{
		Available:   true,
		ProfileID:   "720p",
		DurationMS:  30_000,
		PlaylistURL: "/api/stream/media/startup-720p/stream.m3u8",
	}
	if !startupStreamCanBePlanned(valid) {
		t.Fatal("valid startup bridge must be plannable")
	}

	cases := []*StartupBridgeInfo{
		nil,
		{},
		{Available: true, DurationMS: 30_000, PlaylistURL: valid.PlaylistURL},
		{Available: true, ProfileID: "720p", PlaylistURL: valid.PlaylistURL},
		{Available: true, ProfileID: "720p", DurationMS: 30_000},
	}
	for index, startup := range cases {
		if startupStreamCanBePlanned(startup) {
			t.Fatalf("case %d unexpectedly plannable: %+v", index, startup)
		}
	}
}

func TestApplyStartupStreamPlan(t *testing.T) {
	startup := &StartupBridgeInfo{
		Available:   true,
		ProfileID:   "720p",
		DurationMS:  30_000,
		PlaylistURL: "/api/stream/media/startup-720p/stream.m3u8",
	}
	plan := applyStartupStreamPlan(
		&PlaybackPlan{MediaID: "media"},
		"/api/stream/media/master.m3u8",
		startup,
	)

	if plan.Method != PlaybackMethodStartupStream || plan.URL != startup.PlaylistURL {
		t.Fatalf("unexpected startup plan: %+v", plan)
	}
	if !plan.RequiresTranscode || plan.FallbackMethod != PlaybackMethodTranscode || plan.FallbackURL == "" {
		t.Fatalf("startup fallback contract is incomplete: %+v", plan)
	}
	if plan.StartupStream == nil {
		t.Fatal("startup metadata is missing")
	}
	if plan.StartupStream.ProfileID != "720p" ||
		plan.StartupStream.DurationMS != 30_000 ||
		plan.StartupStream.ContinuationMode != StartupContinuationModeEventBridge ||
		!plan.StartupStream.DiscontinuityAtHandoff {
		t.Fatalf("unexpected startup metadata: %+v", plan.StartupStream)
	}
}

func TestChooseTranscodeOrStartupKeepsBitrateCapAuthoritative(t *testing.T) {
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
	if plan.Method != PlaybackMethodTranscode || plan.StartupStream != nil {
		t.Fatalf("bitrate-capped request must use runtime HLS: %+v", plan)
	}
}
