package handler

import (
	"errors"
	"net/http"
	"testing"

	"github.com/nowen-video/nowen-video/internal/service"
)

func TestPlaybackPlanErrorStatus(t *testing.T) {
	if got := playbackPlanErrorStatus(service.ErrMediaNotFound); got != http.StatusNotFound {
		t.Fatalf("media-not-found status=%d", got)
	}
	if got := playbackPlanErrorStatus(errors.New("artifact database unavailable")); got != http.StatusInternalServerError {
		t.Fatalf("infrastructure status=%d", got)
	}
}
