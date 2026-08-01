package service

import (
	"context"
	"errors"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeprobe "github.com/nowen-video/nowen-video/internal/transcode/probe"
)

// probeMediaForPlan supports compatibility callers that build FFmpeg arguments
// outside a claimed Job. Production Jobs receive a single Probe snapshot in
// processJob and hardware fallback reuses that immutable snapshot.
func (s *TranscodeService) probeMediaForPlan(media *model.Media) *model.MediaProbeRecord {
	if s == nil || s.mediaProbe == nil || media == nil {
		return nil
	}
	record, err := s.mediaProbe.Probe(context.Background(), media)
	if err != nil {
		if !errors.Is(err, transcodeprobe.ErrUnsupportedSource) && s.logger != nil {
			s.logger.Warnf("媒体 Probe 失败，使用兼容转码参数 media=%s: %v", media.ID, err)
		}
		return nil
	}
	transcodeprobe.ApplyToMedia(media, record)
	return record
}

// GetCachedMediaProbe is a non-blocking lookup for playback planning. It never
// launches FFprobe and only returns metadata matching the current source
// fingerprint and parser version.
func (s *TranscodeService) GetCachedMediaProbe(media *model.Media) *model.MediaProbeRecord {
	if s == nil || s.mediaProbe == nil || media == nil {
		return nil
	}
	record, err := s.mediaProbe.Cached(media)
	if err != nil {
		return nil
	}
	return record
}

func (s *TranscodeService) GetMediaProbeStats() transcodeprobe.Stats {
	if s == nil || s.mediaProbe == nil {
		return transcodeprobe.Stats{}
	}
	return s.mediaProbe.Stats()
}
