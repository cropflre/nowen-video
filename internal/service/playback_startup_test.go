package service

import "testing"

func validStartupBridgeInfo() *StartupBridgeInfo {
	return &StartupBridgeInfo{
		Available:           true,
		ProfileID:           "720p",
		DurationMS:          30_000,
		PlaylistURL:         "/api/stream/media/startup-720p/stream.m3u8",
		EncodingPlanVersion: "hls-encoding-plan-v1",
		EncodingPlanHash:    "encoding-plan-hash",
	}
}

func TestStartupStreamCanBePlanned(t *testing.T) {
	valid := validStartupBridgeInfo()
	if !startupStreamCanBePlanned(valid) {
		t.Fatal("valid startup bridge must be plannable")
	}

	cases := []*StartupBridgeInfo{
		nil,
		{},
		{Available: true, DurationMS: 30_000, PlaylistURL: valid.PlaylistURL, EncodingPlanVersion: valid.EncodingPlanVersion, EncodingPlanHash: valid.EncodingPlanHash},
		{Available: true, ProfileID: "720p", PlaylistURL: valid.PlaylistURL, EncodingPlanVersion: valid.EncodingPlanVersion, EncodingPlanHash: valid.EncodingPlanHash},
		{Available: true, ProfileID: "720p", DurationMS: 30_000, EncodingPlanVersion: valid.EncodingPlanVersion, EncodingPlanHash: valid.EncodingPlanHash},
		{Available: true, ProfileID: "720p", DurationMS: 30_000, PlaylistURL: valid.PlaylistURL, EncodingPlanHash: valid.EncodingPlanHash},
		{Available: true, ProfileID: "720p", DurationMS: 30_000, PlaylistURL: valid.PlaylistURL, EncodingPlanVersion: valid.EncodingPlanVersion},
	}
	for index, startup := range cases {
		if startupStreamCanBePlanned(startup) {
			t.Fatalf("case %d unexpectedly plannable: %+v", index, startup)
		}
	}
}

func TestApplyStartupStreamPlan(t *testing.T) {
	startup := validStartupBridgeInfo()
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
		!plan.StartupStream.DiscontinuityAtHandoff ||
		plan.StartupStream.EncodingPlanVersion != startup.EncodingPlanVersion ||
		plan.StartupStream.EncodingPlanHash != startup.EncodingPlanHash {
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
