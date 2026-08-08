package service

import (
	"errors"
	"net/http"
)

// HLSArtifactVersionQuery is retained for source compatibility with old
// clients and migrations. Runtime playback no longer resolves Artifact-pinned
// playlists or segments.
const HLSArtifactVersionQuery = "artifact"

var ErrArtifactNotReady = errors.New("transcode artifact is not ready")

// Runtime Artifact serving has been physically removed. These compatibility
// methods fail closed without reading metadata, touching disk, starting jobs or
// waiting for segments.
func (s *StreamService) GetArtifactSegmentPlaylist(_ string, _ string) (string, error) {
	return "", ErrPersistentRuntimeTranscodeRetired
}

func (s *StreamService) ServeArtifactSegment(_ string, _ string, _ string, _ http.ResponseWriter, _ *http.Request) error {
	return ErrPersistentRuntimeTranscodeRetired
}

func (s *StreamService) ServeArtifactSegmentVersion(_ string, _ string, _ string, _ string, _ http.ResponseWriter, _ *http.Request) error {
	return ErrPersistentRuntimeTranscodeRetired
}
