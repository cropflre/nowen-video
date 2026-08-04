package service

import "github.com/nowen-video/nowen-video/internal/model"

// Retained only so the startup-runtime retirement migration can identify and
// remove historical rows. No production code creates or resolves this kind.
const startupStreamArtifactKind = "startup_hls"

// SubmitStartupStream remains as a source-compatible hard boundary for old
// callers. Startup media is generated only inside Playback Session Generations.
func (s *TranscodeService) SubmitStartupStream(_ *model.Media, _ *model.MediaProbeRecord) (*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}
