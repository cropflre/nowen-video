package serverprofile

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
)

func TestLiteCoreAndUnsupportedCapabilities(t *testing.T) {
	cfg := &config.Config{}
	manifest := Lite(cfg)

	if manifest.Profile != "lite" {
		t.Fatalf("expected lite profile, got %q", manifest.Profile)
	}
	if manifest.SchemaVersion != SchemaVersion {
		t.Fatalf("expected schema version %d, got %d", SchemaVersion, manifest.SchemaVersion)
	}

	for _, name := range []string{"library", "playback", "transcode", "metadata", "subtitles"} {
		capability := manifest.Capabilities[name]
		if !capability.Available || !capability.Enabled {
			t.Fatalf("core capability %q must be available and enabled: %+v", name, capability)
		}
	}

	for _, name := range []string{"preprocess", "emby_compat", "adult_scraper", "cast", "music", "photos", "federation", "plugins", "pulse"} {
		capability := manifest.Capabilities[name]
		if capability.Available || capability.Enabled {
			t.Fatalf("lite-only exclusion %q must be unavailable: %+v", name, capability)
		}
	}
}

func TestLiteOptionalCapabilitiesFollowConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = true
	cfg.AI.EnableSmartSearch = true
	cfg.Storage.WebDAV.Enabled = true
	cfg.Storage.Alist.Enabled = true
	cfg.Storage.S3.Enabled = false

	manifest := Lite(cfg)
	if !manifest.Capabilities["ai"].Enabled || !manifest.Capabilities["ai"].RequiresRestart {
		t.Fatalf("AI should be enabled and restart-bound in lite: %+v", manifest.Capabilities["ai"])
	}
	if !manifest.Capabilities["webdav"].Enabled || !manifest.Capabilities["alist"].Enabled {
		t.Fatal("configured remote storage capabilities should be enabled")
	}
	if manifest.Capabilities["s3"].Enabled {
		t.Fatal("disabled S3 capability must remain disabled")
	}

	legacy := manifest.LegacyFeatures(cfg)
	if legacy["profile"] != "lite" {
		t.Fatalf("legacy feature profile mismatch: %#v", legacy["profile"])
	}
	if legacy["ai_enabled"] != true || legacy["smart_search"] != true {
		t.Fatalf("legacy AI flags should follow enabled config: %#v", legacy)
	}
	if legacy["preprocess"] != false || legacy["emby_compat"] != false {
		t.Fatalf("legacy unsupported flags must remain false: %#v", legacy)
	}
}
