package main

import (
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/handler"
	"github.com/stretchr/testify/require"
)

func TestFullPlaybackRuntimeRoutesCoexistWithLegacyStreamRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api")
	api.GET("/stream/:id/:quality/:segment", func(c *gin.Context) {})

	runtime := &fullPlaybackRuntime{
		plan:    &handler.PlaybackPlanHandler{},
		handler: &handler.PlaybackSessionHandler{},
	}
	require.NotPanics(t, func() {
		runtime.Register(api, func(c *gin.Context) { c.Next() })
	})

	routes := make(map[string]string)
	for _, route := range router.Routes() {
		routes[route.Method+" "+route.Path] = route.Handler
	}
	for _, route := range []string{
		"GET /api/stream/:id/plan",
		"POST /api/playback/sessions",
		"GET /api/playback/sessions/:sessionID/status",
		"POST /api/playback/sessions/:sessionID/heartbeat",
		"POST /api/playback/sessions/:sessionID/restart",
		"DELETE /api/playback/sessions/:sessionID",
		"GET /api/playback/sessions/:sessionID/generations/:generationID/stream.m3u8",
		"GET /api/playback/sessions/:sessionID/generations/:generationID/:file",
	} {
		_, ok := routes[route]
		require.Truef(t, ok, "missing route %s", route)
	}
}

func TestFullShutdownClosesPlaybackBeforeDurableTranscode(t *testing.T) {
	source, err := os.ReadFile("main.go")
	require.NoError(t, err)
	text := string(source)

	playback := strings.Index(text, "playbackRuntime.Shutdown(playbackCtx)")
	durable := strings.Index(text, "services.Transcode.Shutdown(transcodeCtx)")
	require.GreaterOrEqual(t, playback, 0, "full server must close playback sessions")
	require.GreaterOrEqual(t, durable, 0, "full server must close durable transcode jobs")
	require.Less(t, playback, durable, "ephemeral playback must close before durable jobs")
}
