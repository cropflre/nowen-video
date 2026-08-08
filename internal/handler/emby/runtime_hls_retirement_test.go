package emby

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLegacyEmbyQualityHLSRequiresPlaySession(t *testing.T) {
	gin.SetMode(gin.TestMode)
	handler := &Handler{}
	for name, action := range map[string]gin.HandlerFunc{
		"playlist": handler.HLSPlaylistHandler,
		"segment":  handler.HLSSegmentHandler,
	} {
		t.Run(name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/retired", nil)
			action(context)

			require.Equal(t, http.StatusGone, recorder.Code)
			require.Equal(t, "no-store", recorder.Header().Get("Cache-Control"))
			var body map[string]any
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			require.Equal(t, "playback_session_required", body["ErrorCode"])
		})
	}
}
