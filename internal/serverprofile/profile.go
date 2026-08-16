package serverprofile

import "github.com/nowen-video/nowen-video/internal/config"

const SchemaVersion = 2

type Capability struct {
	Available       bool   `json:"available"`
	Enabled         bool   `json:"enabled"`
	Configured      bool   `json:"configured"`
	Configurable    bool   `json:"configurable"`
	RequiresRestart bool   `json:"requires_restart"`
	PendingRestart  bool   `json:"pending_restart"`
	Mode            string `json:"mode,omitempty"`
}

type Manifest struct {
	SchemaVersion int                   `json:"schema_version"`
	Profile       string                `json:"profile"`
	Capabilities  map[string]Capability `json:"capabilities"`
}

type LiteRuntime struct { aiStarted bool }
func NewLiteRuntime(cfg *config.Config) LiteRuntime { return LiteRuntime{aiStarted: cfg.AI.Enabled} }

func always(mode string) Capability {
	return Capability{Available: true, Enabled: true, Configured: true, Mode: mode}
}
func hotConfigurable(configured bool, mode string) Capability {
	return Capability{Available: true, Enabled: configured, Configured: configured, Configurable: true, Mode: mode}
}
func restartConfigurable(started, configured bool, mode string) Capability {
	return Capability{Available: true, Enabled: started && configured, Configured: configured, Configurable: true, RequiresRestart: true, PendingRestart: started != configured, Mode: mode}
}
func unavailable(mode string) Capability { return Capability{Available: false, Enabled: false, Configured: false, Mode: mode} }

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
			"media_analysis":      always("local_ffmpeg"),
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
			// Legacy full-only AI scene semantics (semantic chapter titles / cover scoring)
			// remain separate. Highlight extraction now lives in media_analysis.
			"ai_scene":            unavailable("full_only"),
		},
	}
}

func Lite(cfg *config.Config) Manifest { return NewLiteRuntime(cfg).Manifest(cfg) }

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
			"media_analysis":      always("local_ffmpeg"),
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
		},
	}
}

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
		"media_analysis":        enabled("media_analysis"),
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
