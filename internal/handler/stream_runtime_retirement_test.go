package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLegacyRuntimeStreamingHandlersRequirePlaybackSession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &ArtifactStreamHandler{}
	cases := map[string]gin.HandlerFunc{
		"master":               handler.Master,
		"segment":              handler.Segment,
		"audio_playlist":       handler.AudioPlaylist,
		"audio_segment":        handler.AudioSegment,
		"startup_playlist":     handler.StartupBridgePlaylist,
		"startup_segment":      handler.StartupBridgeSegment,
		"startup_continuation": handler.StartupContinuationSegment,
	}

	for name, action := range cases {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/retired", nil)
			action(context)

			require.Equal(t, http.StatusGone, recorder.Code)
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			var body map[string]any
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, runtimeTranscodeRetiredCode, body["error_code"])
			require.Equal(t, "/api/playback/sessions", body["session_create_url"])
		})
	}
}
