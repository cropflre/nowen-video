package encodingplan

import "testing"

func testPlan() Plan {
	return Plan{
		SchemaVersion: SchemaVersion,
		ProfileID:     "720p",
		Transport: TransportPlan{
			Protocol:          "hls",
			Container:         "mpegts",
			SegmentFormat:     "mpegts",
			SegmentDurationMS: 2000,
		},
		Video: VideoPlan{
			Codec:                "h264",
			Width:                1280,
			Height:               720,
			PixelFormatContract:  "yuv420p-8bit",
			FrameRatePolicy:      "source",
			SourceFrameRateMilli: 23976,
			GOPSize:              48,
			KeyframeIntervalMS:   2000,
			ForceKeyframes:       true,
			SceneCut:             false,
			ColorPolicy:          "source_sdr",
			ColorPrimaries:       "source",
			Transfer:             "source",
			Matrix:               "source",
		},
		Audio: AudioPlan{
			Codec:            "aac",
			Bitrate:          "128k",
			Channels:         2,
			Track:            -1,
			SampleRatePolicy: "source",
		},
	}
}

func TestIdentityIsDeterministic(t *testing.T) {
	plan := testPlan()
	versionA, hashA, jsonA, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	versionB, hashB, jsonB, err := Identity(plan)
	if err != nil {
		t.Fatal(err)
	}
	if versionA != SchemaVersion || versionA != versionB || hashA != hashB || jsonA != jsonB {
		t.Fatalf("identity is not deterministic: %q %q", hashA, hashB)
	}
	if len(hashA) != 64 {
		t.Fatalf("expected SHA-256 hex hash, got %q", hashA)
	}
}

func TestCompatibilityFieldChangesHash(t *testing.T) {
	base := testPlan()
	baseHash, err := base.Hash()
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name   string
		mutate func(*Plan)
	}{
		{name: "profile", mutate: func(p *Plan) { p.ProfileID = "480p" }},
		{name: "dimensions", mutate: func(p *Plan) { p.Video.Width = 854; p.Video.Height = 480 }},
		{name: "frame-rate", mutate: func(p *Plan) { p.Video.SourceFrameRateMilli = 25000; p.Video.GOPSize = 50 }},
		{name: "color", mutate: func(p *Plan) { p.Video.ColorPolicy = "hdr_to_bt709"; p.Video.ColorPrimaries = "bt709" }},
		{name: "audio", mutate: func(p *Plan) { p.Audio.Bitrate = "96k" }},
		{name: "segment", mutate: func(p *Plan) { p.Transport.SegmentDurationMS = 4000 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			tc.mutate(&candidate)
			hash, err := candidate.Hash()
			if err != nil {
				t.Fatal(err)
			}
			if hash == baseHash {
				t.Fatalf("compatibility change did not alter hash")
			}
		})
	}
}

func TestExecutionDetailsAreNotPartOfPlan(t *testing.T) {
	// There are intentionally no start, duration, priority, retry, backend,
	// Worker, Lease, Attempt or path fields in Plan. The same value object is
	// therefore reusable by Startup and Continuation Jobs.
	plan := testPlan()
	first, err := plan.Hash()
	if err != nil {
		t.Fatal(err)
	}
	second, err := plan.Hash()
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatalf("execution-independent plan identity drifted")
	}
}

func TestValidateRejectsIncompletePlan(t *testing.T) {
	plan := testPlan()
	plan.Audio.Codec = ""
	if _, err := plan.Hash(); err == nil {
		t.Fatal("expected incomplete audio contract to fail")
	}
}
