package main

import (
	"os"
	"strings"
	"testing"
)

func readWebPlaybackContractFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func requireWebPlaybackContract(t *testing.T, source, path string, fragments ...string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(source, fragment) {
			t.Fatalf("%s must preserve web playback fallback contract fragment %q", path, fragment)
		}
	}
}

func TestAdaptiveWebPlaybackFallbackContract(t *testing.T) {
	const adaptivePath = "../../web/src/components/AdaptiveWebVideoPlayer.tsx"
	adaptive := readWebPlaybackContractFile(t, adaptivePath)
	requireWebPlaybackContract(t, adaptive, adaptivePath,
		"const PLAYBACK_MODE_RANK",
		"direct: 0",
		"remux: 1",
		"smart_remux: 2",
		"hls: 3",
		"const MAX_NETWORK_RETRIES = 1",
		"networkRetryCountRef",
		"analysis.errorType === 'aborted'",
		"analysis.suggestedFallback === 'retry'",
		"supportsDirect: nextDirect",
		"supportsRemux: nextRemux",
		"forceTranscode: nextForceTranscode",
		"failedModesRef.current.has(to)",
		"PLAYBACK_MODE_RANK[to] <= PLAYBACK_MODE_RANK[from]",
		"setResumePosition(position)",
		"volume: video.volume",
		"muted: video.muted",
		"playbackRate: video.playbackRate",
		"startPosition={resumePosition}",
		"initialPlan?: PlaybackPlan",
		"streamApi.getPlaybackPlan",
	)

	for _, forbidden := range []string{
		"container: 'mkv'",
		"setInterval",
	} {
		if strings.Contains(adaptive, forbidden) {
			t.Fatalf("%s must not retain fallback anti-pattern %q", adaptivePath, forbidden)
		}
	}
}

func TestPlayerPageUsesAdaptiveWebPlaybackController(t *testing.T) {
	const pagePath = "../../web/src/pages/PlayerPage.tsx"
	page := readWebPlaybackContractFile(t, pagePath)
	requireWebPlaybackContract(t, page, pagePath,
		"from '@/components/AdaptiveWebVideoPlayer'",
		"<AdaptiveWebVideoPlayer",
		"initialPlan={streamApi.getCachedPlaybackPlan(id)}",
		"initialRequiresSession={requiresSessionTranscode}",
		"onModeChange={handleRuntimeModeChange}",
		"onTransition={handlePlaybackTransition}",
		"setSwitchPosition(currentTimeRef.current)",
	)

	for _, retired := range []string{
		"handleRemuxFallback",
		"remuxFailed",
		"onRemuxFallback=",
	} {
		if strings.Contains(page, retired) {
			t.Fatalf("%s must not retain split fallback state %q", pagePath, retired)
		}
	}
}
