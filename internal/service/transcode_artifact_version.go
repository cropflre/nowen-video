package service

import "github.com/nowen-video/nowen-video/internal/model"

// Compatibility fences for callers compiled against the former runtime
// Artifact resolver. They perform no repository lookup or filesystem access.
func (s *TranscodeService) hasPublishedHLSArtifact(_ *model.Media, _ string) bool {
	return false
}

func (s *TranscodeService) ResolveHLSOutputDir(_ *model.Media, _ string) (string, error) {
	return "", ErrPersistentRuntimeTranscodeRetired
}

func (s *TranscodeService) resolveHLSArtifactSnapshot(_ *model.Media, _ string) (*model.TranscodeArtifactRecord, string, error) {
	return nil, "", ErrPersistentRuntimeTranscodeRetired
}

func (s *TranscodeService) resolveHLSArtifactVersion(_ *model.Media, _ string, _ string) (*model.TranscodeArtifactRecord, string, error) {
	return nil, "", ErrPersistentRuntimeTranscodeRetired
}

func readableHLSArtifactDirectory(_ *model.TranscodeArtifactRecord) (string, error) {
	return "", ErrPersistentRuntimeTranscodeRetired
}
