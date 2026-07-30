package serverprofile

import "github.com/nowen-video/nowen-video/internal/config"

const SchemaVersion = 1

// Capability describes whether a subsystem is compiled into the current
// server profile, currently enabled, configurable by the administrator, and
// whether changing its enabled state requires a process restart.
type Capability struct {
	Available       bool   `json:"available"`
	Enabled         bool   `json:"enabled"`
	Configurable    bool   `json:"configurable"`
	RequiresRestart bool   `json:"requires_restart"`
	Mode            string `json:"mode,omitempty"`
}

// Manifest is the stable contract consumed by Web, desktop and mobile clients.
// New capabilities can be added without changing the legacy flat feature map.
type Manifest struct {
	SchemaVersion int                   `json:"schema_version"`
	Profile       string                `json:"profile"`
	Capabilities  map[string]Capability `json:"capabilities"`
}

func always(mode string) Capability {
	return Capability{Available: true, Enabled: true, Mode: mode}
}

func configurable(enabled bool, restart bool, mode string) Capability {
	return Capability{
		Available:       true,
		Enabled:         enabled,
		Configurable:    true,
		RequiresRestart: restart,
		Mode:            mode,
	}
}

func unavailable(mode string) Capability {
	return Capability{Available: false, Enabled: false, Mode: mode}
}

// Lite returns the NAS-oriented default profile. Optional remote storage is
// initialized only when configured. AI configuration is available in Lite, but
// changing its startup state requires a restart so migrations and routes can be
// assembled consistently.
func Lite(cfg *config.Config) Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Profile:       "lite",
		Capabilities: map[string]Capability{
			"library":           always("core"),
			"metadata":          always("core"),
			"playback":          always("core"),
			"transcode":         always("on_demand"),
			"subtitles":         always("core"),
			"users":             always("core"),
			"collections":       always("core"),
			"ai":                configurable(cfg.AI.Enabled, true, "optional"),
			"webdav":            configurable(cfg.Storage.WebDAV.Enabled, false, "optional"),
			"alist":             configurable(cfg.Storage.Alist.Enabled, false, "optional"),
			"s3":                configurable(cfg.Storage.S3.Enabled, false, "optional"),
			"preprocess":        unavailable("full_only"),
			"subtitle_preprocess": unavailable("full_only"),
			"emby_compat":       unavailable("full_only"),
			"adult_scraper":     unavailable("full_only"),
			"cast":              unavailable("full_only"),
			"music":             unavailable("full_only"),
			"photos":            unavailable("full_only"),
			"federation":        unavailable("full_only"),
			"plugins":           unavailable("full_only"),
			"offline_download":  unavailable("full_only"),
			"user_profiles":     unavailable("full_only"),
			"comments":          unavailable("full_only"),
			"danmaku":           unavailable("full_only"),
			"ai_scene":          unavailable("full_only"),
			"pulse":             unavailable("removed"),
		},
	}
}

// LegacyFeatures keeps older clients working while the typed capability
// manifest is adopted. Values intentionally mirror the original health API.
func (m Manifest) LegacyFeatures(cfg *config.Config) map[string]any {
	enabled := func(name string) bool {
		capability, ok := m.Capabilities[name]
		return ok && capability.Available && capability.Enabled
	}
	return map[string]any{
		"profile":               m.Profile,
		"emby_compat":           enabled("emby_compat"),
		"music":                 enabled("music"),
		"photos":                enabled("photos"),
		"federation":            enabled("federation"),
		"plugins":               enabled("plugins"),
		"preprocess":            enabled("preprocess"),
		"adult_scraper":         enabled("adult_scraper"),
		"cast":                  enabled("cast"),
		"ai_scene":              enabled("ai_scene"),
		"ai_enabled":            enabled("ai"),
		"smart_search":          enabled("ai") && cfg.AI.EnableSmartSearch,
		"recommend_reason":      enabled("ai") && cfg.AI.EnableRecommendReason,
		"metadata_enhance":      enabled("ai") && cfg.AI.EnableMetadataEnhance,
		"webdav":                enabled("webdav"),
		"alist":                 enabled("alist"),
		"s3":                    enabled("s3"),
		"strm_hls_rewrite":      cfg.STRM.RewriteHLS,
		"direct_play_preferred": true,
	}
}
