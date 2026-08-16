package serverprofile

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
)

func TestLiteMediaAnalysisIsLocalCoreCapabilityWithoutAI(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = false

	manifest := Lite(cfg)
	capability, ok := manifest.Capabilities["media_analysis"]
	if !ok {
		t.Fatal("media_analysis capability must be present in Lite")
	}
	if !capability.Available || !capability.Enabled || !capability.Configured {
		t.Fatalf("media_analysis must be available without AI: %+v", capability)
	}
	if capability.Mode != "local_ffmpeg" {
		t.Fatalf("expected local_ffmpeg mode, got %q", capability.Mode)
	}
	if legacy := manifest.LegacyFeatures(cfg)["media_analysis"]; legacy != true {
		t.Fatalf("legacy media_analysis flag must be true, got %#v", legacy)
	}
	if manifest.Capabilities["ai"].Enabled {
		t.Fatal("test precondition: AI must remain disabled")
	}
}
