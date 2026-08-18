package handler

import (
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/nowen-video/nowen-video/internal/service"
)

func TestValidateMediaAnalysisWorkerCompleteAcceptsV1AndV2HighlightPayloads(t *testing.T) {
	webp := base64.StdEncoding.EncodeToString([]byte{'R', 'I', 'F', 'F', 0, 0, 0, 0, 'W', 'E', 'B', 'P'})
	highlights := []service.MediaAnalysisWorkerHighlight{{
		StartTime: 10, EndTime: 30, Score: 8,
		ThumbnailBase64: webp, ThumbnailMime: "image/webp",
	}}
	v1, _ := json.Marshal(service.MediaComputeTaskComplete{
		ClaimToken: "claim-v1", Fingerprint: "fp-1", Highlights: highlights,
	})
	if code := validateMediaComputeBody(t, string(v1)); code != http.StatusNoContent {
		t.Fatalf("V1 body status = %d", code)
	}

	result, _ := json.Marshal(map[string]any{"fingerprint": "fp-2", "highlights": highlights})
	v2, _ := json.Marshal(service.MediaComputeTaskComplete{
		ClaimToken: "claim-v2", JobType: service.MediaComputeJobHighlightV1, Result: result,
	})
	if code := validateMediaComputeBody(t, string(v2)); code != http.StatusNoContent {
		t.Fatalf("V2 body status = %d", code)
	}
}

func TestValidateMediaAnalysisWorkerCompleteRejectsSpoofedV2Thumbnail(t *testing.T) {
	fakeWebp := base64.StdEncoding.EncodeToString([]byte{0xff, 0xd8, 0xff, 0, 0, 0, 0, 0, 0, 0, 0, 0})
	result, _ := json.Marshal(map[string]any{
		"fingerprint": "fp",
		"highlights": []service.MediaAnalysisWorkerHighlight{{
			StartTime: 10, EndTime: 20, Score: 7,
			ThumbnailBase64: fakeWebp, ThumbnailMime: "image/webp",
		}},
	})
	body, _ := json.Marshal(service.MediaComputeTaskComplete{
		ClaimToken: "claim", JobType: service.MediaComputeJobHighlightV1, Result: result,
	})
	if code := validateMediaComputeBody(t, string(body)); code != http.StatusUnprocessableEntity {
		t.Fatalf("spoofed thumbnail status = %d", code)
	}
}

func validateMediaComputeBody(t *testing.T, body string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/", ValidateMediaAnalysisWorkerComplete, func(c *gin.Context) {
		c.Status(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	return response.Code
}
