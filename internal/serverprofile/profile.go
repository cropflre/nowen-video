package serverprofile

import "github.com/nowen-video/nowen-video/internal/config"

const SchemaVersion = 2

// Capability describes both the desired configuration and the actual runtime
// state. This distinction is important for modules assembled only at process
// startup: configuration can change immediately while routes, migrations and
// workers remain unchanged until restart.
type Capability struct {
	Available       bool   `json:"available"`
	Enabled         bool   `json:"enabled"`
	Configured      bool   `json:"configured"`
	Configurable    bool   `json:"configurable"`
	RequiresRestart bool   `json:"requires_restart"`
	PendingRestart  bool   `json:"pending_restart"`
	Mode            string `json:"mode,omitempty"`
}

// Manifest is the stable contract consumed by Web, desktop and mobile clients.
// New capabilities can be added without changing the legacy flat feature map.
type Manifest struct {
	SchemaVersion int                   `json:"schema_version"`
	Profile       string                `json:"profile"`
	Capabilities  map[string]Capability `json:"capabilities"`
}

// LiteRuntime captures startup-time decisions that cannot be safely inferred
// from a mutable config later. Keep this value for the life of the process.
type LiteRuntime struct {
	aiStarted bool
}

func NewLiteRuntime(cfg *config.Config) LiteRuntime {
	return LiteRuntime{aiStarted: cfg.AI.Enabled}
}

func always(mode string) Capability {
	return Capability{
		Available:  true,
		Enabled:    true,
		Configured: true,
		Mode:       mode,
	}
}

func hotConfigurable(configured bool, mode string) Capability {
	return Capability{
		Available:    true,
		Enabled:      configured,
		Configured:   configured,
		Configurable: true,
		Mode:         mode,
	}
}

func restartConfigurable(started, configured bool, mode string) Capability {
	return Capability{
		Available:       true,
		Enabled:         started && configured,
		Configured:      configured,
		Configurable:    true,
		RequiresRestart: true,
		PendingRestart:  started != configured,
		Mode:            mode,
	}
}

func unavailable(mode string) Capability {
	return Capability{Available: false, Enabled: false, Configured: false, Mode: mode}
}

// Manifest returns the NAS-oriented default profile using actual startup state
// plus the latest persisted configuration.
func (r LiteRuntime) Manifest(cfg *config.Config) Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Profile:       "lite",
		Capabilities: map[string]Capability{
			"library":             always("core"),
			"metadata":            always("core"),
			"playback":            always("core"),
			"transcode":           always("on_demand"),
			"subtitles":           always("core"),
			"users":               always("core"),
			"collections":         always("core"),
			"task_center":         always("core"),
			"ai":                  restartConfigurable(r.aiStarted, cfg.AI.Enabled, "optional"),
			"webdav":              hotConfigurable(cfg.Storage.WebDAV.Enabled, "optional"),
			"alist":               hotConfigurable(cfg.Storage.Alist.Enabled, "optional"),
			"s3":                  hotConfigurable(cfg.Storage.S3.Enabled, "optional"),
			"preprocess":          unavailable("full_only"),
			"subtitle_preprocess": unavailable("full_only"),
			"emby_compat":         unavailable("full_only"),
			"adult_scraper":       unavailable("full_only"),
			"cast":                unavailable("full_only"),
			"music":               unavailable("full_only"),
			"photos":              unavailable("full_only"),
			"federation":          unavailable("full_only"),
			"plugins":             unavailable("full_only"),
			"offline_download":    unavailable("full_only"),
			"user_profiles":       unavailable("full_only"),
			"comments":            unavailable("full_only"),
			"danmaku":             unavailable("full_only"),
			"ai_scene":            unavailable("full_only"),
			"pulse":               unavailable("removed"),
		},
	}
}

// Lite is convenient for tests and one-shot callers where startup config and
// current config are identical. Long-running servers should keep LiteRuntime.
func Lite(cfg *config.Config) Manifest {
	return NewLiteRuntime(cfg).Manifest(cfg)
}

// Full describes the legacy all-in-one server. Its routes and services are
// assembled unconditionally, so configurable modules such as AI and remote
// storage can report the latest persisted state without a restart boundary.
func Full(cfg *config.Config) Manifest {
	return Manifest{
		SchemaVersion: SchemaVersion,
		Profile:       "full",
		Capabilities: map[string]Capability{
			"library":             always("core"),
			"metadata":            always("core"),
			"playback":            always("core"),
			"transcode":           always("core"),
			"subtitles":           always("core"),
			"users":               always("core"),
			"collections":         always("core"),
			"task_center":         unavailable("lite_only"),
			"ai":                  hotConfigurable(cfg.AI.Enabled, "optional"),
			"webdav":              hotConfigurable(cfg.Storage.WebDAV.Enabled, "optional"),
			"alist":               hotConfigurable(cfg.Storage.Alist.Enabled, "optional"),
			"s3":                  hotConfigurable(cfg.Storage.S3.Enabled, "optional"),
			"preprocess":          always("full"),
			"subtitle_preprocess": always("full"),
			"emby_compat":         always("full"),
			"adult_scraper":       always("full"),
			"cast":                always("full"),
			"music":               always("full"),
			"photos":              always("full"),
			"federation":          always("full"),
			"plugins":             always("full"),
			"offline_download":    always("full"),
			"user_profiles":       always("full"),
			"comments":            always("full"),
			"danmaku":             always("full"),
			"ai_scene":            always("full"),
			"pulse":               unavailable("removed"),
		},
	}
}

// LegacyFeatures keeps older clients working while the typed capability
// manifest is adopted. Flags represent actual runtime state, not merely desired
// configuration, so clients never call routes that still require a restart.
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
