package emby

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestEmbySessionRoutesCoexistWithLegacyPlaybackRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	handler := &Handler{}

	require.NotPanics(t, func() {
		registerEmbyAuthed(router.Group(""), handler)
	})

	routes := make(map[string]struct{})
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = struct{}{}
	}
	for _, expected := range []string{
		"GET /Videos/:id/session/:playSessionID/:generationID/stream.m3u8",
		"HEAD /Videos/:id/session/:playSessionID/:generationID/stream.m3u8",
		"GET /Videos/:id/session/:playSessionID/:generationID/:segment",
		"HEAD /Videos/:id/session/:playSessionID/:generationID/:segment",
		"GET /Videos/:id/hls1/:quality/main.m3u8",
		"GET /Videos/:id/:sourceId/Subtitles/:index/Stream.:ext",
	} {
		_, ok := routes[expected]
		require.Truef(t, ok, "missing route %s", expected)
	}
}

func TestAppendPlaybackQueryPreservesAndReplacesValues(t *testing.T) {
	result := appendPlaybackQuery(
		"/Videos/item/master.m3u8?api_key=old&custom=value",
		url.Values{
			"api_key":        []string{"new"},
			"PlaySessionId":  []string{"play-session"},
			"StartTimeTicks": []string{"36000000000"},
		},
	)
	parsed, err := url.Parse(result)
	require.NoError(t, err)
	require.Equal(t, "/Videos/item/master.m3u8", parsed.Path)
	require.Equal(t, "new", parsed.Query().Get("api_key"))
	require.Equal(t, "value", parsed.Query().Get("custom"))
	require.Equal(t, "play-session", parsed.Query().Get("PlaySessionId"))
	require.Equal(t, "36000000000", parsed.Query().Get("StartTimeTicks"))
}

func TestParseEmbyPlaybackRequestUsesExternalSessionAndAbsoluteStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(
		"GET",
		"/Videos/item/master.m3u8?PlaySessionId=play-1&StartTimeTicks=36000000000&maxBitrate=12000000",
		nil,
	)

	externalID, startMS, hasStart, maxBitrate := parseEmbyPlaybackRequest(context)
	require.Equal(t, "play-1", externalID)
	require.True(t, hasStart)
	require.EqualValues(t, 3_600_000, startMS)
	require.Equal(t, 12_000_000, maxBitrate)
}

func TestRewriteEmbySessionPlaylistPinsGenerationSegments(t *testing.T) {
	gin.SetMode(gin.TestMode)
	context, _ := gin.CreateTestContext(httptest.NewRecorder())
	context.Request = httptest.NewRequest(
		"GET",
		"/emby/Videos/emby-item/session/play-1/7/stream.m3u8?api_key=secret",
		nil,
	)
	mapping := &embyPlaybackMapping{
		ExternalID:   "play-1",
		GenerationID: 7,
	}
	playlist := strings.Join([]string{
		"#EXTM3U",
		"#EXT-X-TARGETDURATION:2",
		"seg_000001.ts",
		"nested/seg_000002.ts?ignored=1",
		"",
	}, "\n")

	rewritten := rewriteEmbySessionPlaylist(context, "emby-item", mapping, 7, []byte(playlist))
	require.Contains(t, rewritten, "/emby/Videos/emby-item/session/play-1/7/seg_000001.ts?api_key=secret")
	require.Contains(t, rewritten, "/emby/Videos/emby-item/session/play-1/7/seg_000002.ts?api_key=secret")
	require.NotContains(t, rewritten, "nested/")
}

func TestEmbyRuntimeMappingIsolatedByUserAndPlaySession(t *testing.T) {
	runtime := &embyPlaybackRuntime{entries: make(map[string]*embyPlaybackMapping)}
	runtime.put(&embyPlaybackMapping{
		ExternalID:   "play-a",
		InternalID:   "internal-a",
		UserID:       "user-a",
		MediaID:      "media",
		GenerationID: 1,
	})
	runtime.put(&embyPlaybackMapping{
		ExternalID:   "play-b",
		InternalID:   "internal-b",
		UserID:       "user-b",
		MediaID:      "media",
		GenerationID: 2,
	})

	first, ok := runtime.find("user-a", "play-a", "media")
	require.True(t, ok)
	require.Equal(t, "internal-a", first.InternalID)

	second, ok := runtime.find("user-b", "play-b", "media")
	require.True(t, ok)
	require.Equal(t, "internal-b", second.InternalID)

	_, ok = runtime.find("user-a", "play-b", "media-missing")
	require.False(t, ok)
}
