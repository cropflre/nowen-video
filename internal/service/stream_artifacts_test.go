package service

import (
	"errors"
	"net/http/httptest"
	"testing"
)

func TestRuntimeArtifactServingIsRetired(t *testing.T) {
	stream := &StreamService{}
	if _, err := stream.GetArtifactSegmentPlaylist("media", "720p"); !errors.Is(err, ErrPersistentRuntimeTranscodeRetired) {
		t.Fatalf("artifact playlist did not fail closed: %v", err)
	}

	request := httptest.NewRequest("GET", "/api/stream/media/720p/seg0000.ts", nil)
	for name, serve := range map[string]func() error{
		"unversioned": func() error {
			return stream.ServeArtifactSegment("media", "720p", "seg0000.ts", httptest.NewRecorder(), request)
		},
		"versioned": func() error {
			return stream.ServeArtifactSegmentVersion("media", "720p", "artifact", "seg0000.ts", httptest.NewRecorder(), request)
		},
	} {
		if err := serve(); !errors.Is(err, ErrPersistentRuntimeTranscodeRetired) {
			t.Fatalf("%s artifact segment did not fail closed: %v", name, err)
		}
	}
}
