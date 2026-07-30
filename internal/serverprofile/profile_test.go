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

	for _, name := range []string{"library", "playback", "transcode", "metadata", "subtitles", "task_center"} {
		capability := manifest.Capabilities[name]
		if !capability.Available || !capability.Enabled || !capability.Configured {
			t.Fatalf("core capability %q must be available, configured and enabled: %+v", name, capability)
		}
	}

	for _, name := range []string{"preprocess", "emby_compat", "adult_scraper", "cast", "music", "photos", "federation", "plugins", "pulse"} {
		capability := manifest.Capabilities[name]
		if capability.Available || capability.Enabled || capability.Configured {
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
	ai := manifest.Capabilities["ai"]
	if !ai.Enabled || !ai.Configured || !ai.RequiresRestart || ai.PendingRestart {
		t.Fatalf("AI should be running without a pending restart: %+v", ai)
	}
	if !manifest.Capabilities["webdav"].Enabled || !manifest.Capabilities["alist"].Enabled {
		t.Fatal("configured remote storage capabilities should be enabled")
	}
	if manifest.Capabilities["s3"].Enabled || manifest.Capabilities["s3"].Configured {
		t.Fatal("disabled S3 capability must remain disabled")
	}

	legacy := manifest.LegacyFeatures(cfg)
	if legacy["profile"] != "lite" {
		t.Fatalf("legacy feature profile mismatch: %#v", legacy["profile"])
	}
	if legacy["ai_enabled"] != true || legacy["smart_search"] != true {
		t.Fatalf("legacy AI flags should follow runtime state: %#v", legacy)
	}
	if legacy["preprocess"] != false || legacy["emby_compat"] != false {
		t.Fatalf("legacy unsupported flags must remain false: %#v", legacy)
	}
}

func TestLiteRuntimeReportsAIEnablePendingRestart(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = false
	runtimeState := NewLiteRuntime(cfg)

	cfg.AI.Enabled = true
	manifest := runtimeState.Manifest(cfg)
	ai := manifest.Capabilities["ai"]

	if !ai.Configured || ai.Enabled || !ai.PendingRestart {
		t.Fatalf("AI enable should be configured but not running before restart: %+v", ai)
	}
	if manifest.LegacyFeatures(cfg)["ai_enabled"] != false {
		t.Fatal("legacy clients must not see AI as enabled before restart")
	}
}

func TestLiteRuntimeReportsAIDisablePendingRestart(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = true
	runtimeState := NewLiteRuntime(cfg)

	cfg.AI.Enabled = false
	manifest := runtimeState.Manifest(cfg)
	ai := manifest.Capabilities["ai"]

	if ai.Configured || ai.Enabled || !ai.PendingRestart {
		t.Fatalf("AI disable should be pending restart and unavailable to callers: %+v", ai)
	}
}

func TestFullManifestExposesAdvancedCapabilities(t *testing.T) {
	cfg := &config.Config{}
	cfg.AI.Enabled = false
	cfg.Storage.WebDAV.Enabled = true

	manifest := Full(cfg)
	if manifest.Profile != "full" || manifest.SchemaVersion != SchemaVersion {
		t.Fatalf("unexpected full manifest identity: %+v", manifest)
	}

	for _, name := range []string{"preprocess", "subtitle_preprocess", "emby_compat", "adult_scraper", "cast", "music", "photos", "federation", "plugins", "offline_download", "user_profiles", "comments", "danmaku", "ai_scene"} {
		capability := manifest.Capabilities[name]
		if !capability.Available || !capability.Enabled || !capability.Configured {
			t.Fatalf("full capability %q must be available and enabled: %+v", name, capability)
		}
	}

	if manifest.Capabilities["task_center"].Available {
		t.Fatal("full keeps its advanced task pages instead of the Lite task center")
	}
	if manifest.Capabilities["pulse"].Available {
		t.Fatal("Pulse must remain permanently removed in Full")
	}
	if manifest.Capabilities["ai"].Enabled || !manifest.Capabilities["ai"].Available {
		t.Fatalf("disabled Full AI should remain configurable but not enabled: %+v", manifest.Capabilities["ai"])
	}
	if !manifest.Capabilities["webdav"].Enabled {
		t.Fatal("configured Full WebDAV must be enabled")
	}
}
