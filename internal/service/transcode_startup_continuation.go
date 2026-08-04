package service

import (
	"errors"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// Retained only so the startup-runtime retirement migration can identify and
// remove historical rows. No production code creates or resolves this kind.
const startupContinuationArtifactKind = "startup_continuation_hls"

// StartupStreamDescriptor remains as a decoding boundary for historical data
// and downstream source compatibility. It carries no active playback role.
type StartupStreamDescriptor struct {
	MediaID              string
	ProfileID            string
	SourceFingerprint    string
	DurationMS           int64
	ArtifactID           string
	ManifestPath         string
	OutputDir            string
	EncodingPlanVersion  string
	EncodingPlanHash     string
	EncodingPlanJSON     string
	TimestampPlanVersion string
	TimestampPlanHash    string
	TimestampPlanJSON    string
	TimelineOriginMS     int64
	AttestationVersion   string
	AttestationHash      string
	AttestationJSON      string
	TimelineStartMS      int64
	TimelineEndMS        int64
	Probe                *model.MediaProbeRecord
}

func (s *TranscodeService) ResolvePublishedStartupStream(_ *model.Media) (*StartupStreamDescriptor, error) {
	return nil, gorm.ErrRecordNotFound
}

func (s *TranscodeService) SubmitStartupContinuation(_ *model.Media, _ *StartupStreamDescriptor) (*model.TranscodeTask, error) {
	return nil, ErrPersistentRuntimeTranscodeRetired
}

func (s *TranscodeService) ResolveReadableStartupContinuation(_ *model.Media, _ *StartupStreamDescriptor) (*model.TranscodeArtifactRecord, error) {
	return nil, gorm.ErrRecordNotFound
}

func errorsIsRecordNotFound(err error) bool {
	return errors.Is(err, gorm.ErrRecordNotFound)
}
