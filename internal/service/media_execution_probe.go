package service

import (
	"context"
	"fmt"

	"github.com/nowen-video/nowen-video/internal/model"
)

// ProbeMedia returns authoritative technical metadata for playback planning.
// The underlying probe service persists successful results and singleflights
// concurrent requests, so a cold planner probe also warms later playback.
func (s *MediaExecutionService) ProbeMedia(ctx context.Context, media *model.Media) (*model.MediaProbeRecord, error) {
	if s == nil || s.mediaProbe == nil {
		return nil, fmt.Errorf("media probe service is unavailable")
	}
	if media == nil {
		return nil, fmt.Errorf("media is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	return s.mediaProbe.Probe(ctx, media)
}
