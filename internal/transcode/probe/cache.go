package probe

import (
	"github.com/nowen-video/nowen-video/internal/model"
)

// Cached returns only a fresh persisted record and never executes FFprobe. It
// is safe for latency-sensitive playback planning and API response paths.
func (s *Service) Cached(media *model.Media) (*model.MediaProbeRecord, error) {
	if s == nil || s.repo == nil {
		return nil, ErrUnsupportedSource
	}
	identity, err := identifySource(media)
	if err != nil {
		return nil, err
	}
	return s.repo.FindFresh(identity.MediaID, identity.Fingerprint, model.MediaProbeVersion)
}
