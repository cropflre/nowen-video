package service

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
)

func TestStartupEncodingIdentityIsSharedAcrossExecutionRanges(t *testing.T) {
	probe := &model.MediaProbeRecord{
		VideoCodec:     "hevc",
		Width:          3840,
		Height:         2160,
		FrameRateNum:   24000,
		FrameRateDen:   1001,
		BitDepth:       10,
		HDR:            true,
		ColorTransfer:  "smpte2084",
		ColorPrimaries: "bt2020",
		ColorSpace:     "bt2020nc",
	}
	startup, err := startupEncodingIdentity(probe, "720p")
	if err != nil {
		t.Fatal(err)
	}
	continuation, err := startupEncodingIdentity(probe, "720p")
	if err != nil {
		t.Fatal(err)
	}
	if startup.Version != encodingplan.SchemaVersion || startup.Hash != continuation.Hash || startup.Canonical != continuation.Canonical {
		t.Fatalf("startup and continuation plan identity drifted: %+v %+v", startup, continuation)
	}
}

func TestStartupEncodingPlanCapturesTimingAndColorPolicy(t *testing.T) {
	probe := &model.MediaProbeRecord{
		FrameRateNum: 30000,
		FrameRateDen: 1001,
		HDR:          true,
	}
	plan, err := startupEncodingPlan(probe, "720p")
	if err != nil {
		t.Fatal(err)
	}
	if plan.Video.Width != 1280 || plan.Video.Height != 720 {
		t.Fatalf("unexpected dimensions: %+v", plan.Video)
	}
	if plan.Video.SourceFrameRateMilli != 29970 || plan.Video.GOPSize != 60 || plan.Video.KeyframeIntervalMS != 2000 {
		t.Fatalf("unexpected timing contract: %+v", plan.Video)
	}
	if plan.Video.ColorPolicy != "hdr_to_bt709" || plan.Video.ColorPrimaries != "bt709" || plan.Video.Transfer != "bt709" || plan.Video.Matrix != "bt709" {
		t.Fatalf("unexpected HDR contract: %+v", plan.Video)
	}
	if plan.Audio.Codec != "aac" || plan.Audio.Bitrate != "128k" || plan.Audio.Channels != 2 || plan.Audio.Track != -1 {
		t.Fatalf("unexpected audio contract: %+v", plan.Audio)
	}
}

func TestSameEncodingPlanRejectsMissingOrMismatchedIdentity(t *testing.T) {
	if sameEncodingPlan("", "", "", encodingplan.SchemaVersion, "hash", "json") {
		t.Fatal("blank historical plan must not be bridge eligible")
	}
	if sameEncodingPlan(encodingplan.SchemaVersion, "hash-a", "json", encodingplan.SchemaVersion, "hash-b", "json") {
		t.Fatal("different hashes must not be compatible")
	}
	if sameEncodingPlan(encodingplan.SchemaVersion, "hash", "json-a", encodingplan.SchemaVersion, "hash", "json-b") {
		t.Fatal("different canonical plans must not be compatible")
	}
	if !sameEncodingPlan(encodingplan.SchemaVersion, "hash", "json", encodingplan.SchemaVersion, "hash", "json") {
		t.Fatal("identical plans must be compatible")
	}
}
