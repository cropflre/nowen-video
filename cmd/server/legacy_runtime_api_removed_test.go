package main

import (
	"os"
	"strings"
	"testing"
)

func TestLegacyRuntimeCompatibilitySurfaceIsPhysicallyRemoved(t *testing.T) {
	sources := []string{
		"main.go",
		"../server-lite/routes_core.go",
		"../server-lite/routes_admin.go",
		"../../internal/handler/stream.go",
		"../../internal/handler/admin.go",
		"../../web/src/api/stream.ts",
		"../../web/src/api/admin.ts",
		"../../web/src/pages/PlayerPage.tsx",
		"../../web/src/pages/PreprocessPage.tsx",
		"../../web/src/components/VideoPlayer.tsx",
		"../../web/src/components/SessionVideoPlayer.tsx",
		"../../web/src/components/WebCodecsPlayerShell.tsx",
		"../../web/src/hooks/usePlaybackSessionSource.ts",
	}
	forbidden := []string{
		"RetiredRuntimeHLS",
		"RetiredRuntimeTranscode",
		`GET("/stream/:id/master.m3u8"`,
		`GET("/stream/:id/:quality/:segment"`,
		`POST("/stream/:id/playback"`,
		`POST("/stream/:id/bandwidth"`,
		`GET("/stream/:id/throttle"`,
		`GET("/audio-track/:id/:trackIdx"`,
		"/admin/transcode/status",
		"/admin/transcode-tasks",
		"reportPlayback(",
		"reportBandwidth(",
		"getThrottleStatus(",
		"fallbackSrc",
		"fallbackSource",
		"TranscodeJobsPanel",
	}
	for _, path := range sources {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, marker := range forbidden {
			if strings.Contains(string(content), marker) {
				t.Fatalf("%s still exposes retired Runtime marker %q", path, marker)
			}
		}
	}
	if _, err := os.Stat("../../web/src/components/preprocess/TranscodeJobsPanel.tsx"); !os.IsNotExist(err) {
		t.Fatalf("retired TranscodeJobsPanel still exists: %v", err)
	}
}

func TestModernPlaybackAndHistoryContractsRemainRegistered(t *testing.T) {
	checks := map[string][]string{
		"main.go": {
			`api.GET("/stream/:id/direct"`,
			`api.GET("/stream/:id/remux"`,
			`playbackRuntime.Register(api, guardByMediaID)`,
			`admin.GET("/runtime-history"`,
			`api.GET("/preprocess/media/:id/master.m3u8"`,
		},
		"../server-lite/routes_core.go": {
			`api.GET("/stream/:id/info"`,
			`api.GET("/stream/:id/plan"`,
			`api.GET("/stream/:id/direct"`,
			`api.GET("/stream/:id/remux"`,
			`api.POST("/playback/sessions"`,
			`api.GET("/playback/sessions/:sessionID/generations/:generationID/stream.m3u8"`,
		},
		"../server-lite/routes_admin.go": {
			`admin.GET("/runtime-history"`,
			`admin.GET("/runtime-history/summary"`,
			`admin.GET("/runtime-history/jobs/:id"`,
		},
	}
	for path, markers := range checks {
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		for _, marker := range markers {
			if !strings.Contains(string(content), marker) {
				t.Fatalf("%s lost required modern contract %q", path, marker)
			}
		}
	}
}
