package service

import (
	"strings"
	"testing"
)

func TestBuildStartupBridgePlaylistOrdersStartupBeforeContinuation(t *testing.T) {
	startup := hlsPlaylistSnapshot{
		TargetDuration: 2,
		Segments: []hlsPlaylistSegment{
			{Duration: "#EXTINF:2.000,", URI: "seg0000.ts"},
			{Duration: "#EXTINF:2.000,", URI: "seg0001.ts"},
		},
		EndList: true,
	}
	continuation := &hlsPlaylistSnapshot{
		TargetDuration: 2,
		Segments: []hlsPlaylistSegment{
			{Duration: "#EXTINF:2.000,", URI: "seg0015.ts"},
			{Duration: "#EXTINF:2.000,", URI: "seg0016.ts"},
		},
		EndList: true,
	}

	playlist := buildStartupBridgePlaylist("media", "720p", startup, continuation)
	startupIndex := strings.Index(playlist, "startup__seg0000.ts")
	continuationIndex := strings.Index(playlist, "continuation__seg0015.ts")
	if startupIndex < 0 || continuationIndex < 0 || startupIndex >= continuationIndex {
		t.Fatalf("startup must precede continuation:\n%s", playlist)
	}
	if strings.Count(playlist, "#EXT-X-DISCONTINUITY") != 1 {
		t.Fatalf("bridge must declare exactly one handoff boundary:\n%s", playlist)
	}
	if !strings.Contains(playlist, "#EXT-X-PLAYLIST-TYPE:EVENT") || !strings.HasSuffix(playlist, "#EXT-X-ENDLIST\n") {
		t.Fatalf("unexpected EVENT lifecycle:\n%s", playlist)
	}
}

func TestBuildStartupBridgePlaylistStaysOpenUntilContinuationCompletes(t *testing.T) {
	startup := hlsPlaylistSnapshot{
		TargetDuration: 2,
		Segments: []hlsPlaylistSegment{{Duration: "#EXTINF:2.000,", URI: "seg0000.ts"}},
		EndList: true,
	}
	playlist := buildStartupBridgePlaylist("media", "720p", startup, nil)
	if strings.Contains(playlist, "#EXT-X-ENDLIST") {
		t.Fatalf("bridge must remain reloadable while continuation is pending:\n%s", playlist)
	}
	if strings.Contains(playlist, "#EXT-X-DISCONTINUITY") {
		t.Fatalf("handoff boundary must not appear before continuation exists:\n%s", playlist)
	}
}

func TestStartupBridgeRouteContract(t *testing.T) {
	profile, ok := ParseStartupBridgeProfile("startup-720p")
	if !ok || profile != "720p" {
		t.Fatalf("unexpected virtual profile parse: profile=%q ok=%v", profile, ok)
	}
	if _, ok := ParseStartupBridgeProfile("720p"); ok {
		t.Fatal("normal HLS profile must not enter startup bridge routing")
	}

	source, actual, ok := ParseStartupBridgeSegment("startup__seg0001.ts")
	if !ok || source != "startup" || actual != "seg0001.ts" {
		t.Fatalf("unexpected startup segment parse: %q %q %v", source, actual, ok)
	}
	source, actual, ok = ParseStartupBridgeSegment("continuation__seg0015.ts")
	if !ok || source != "continuation" || actual != "seg0015.ts" {
		t.Fatalf("unexpected continuation segment parse: %q %q %v", source, actual, ok)
	}
	if _, _, ok := ParseStartupBridgeSegment("../seg0001.ts"); ok {
		t.Fatal("unclassified segment must not be routed into Artifact storage")
	}
}
