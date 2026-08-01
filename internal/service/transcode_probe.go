package service

import (
	"context"
	"errors"
	"sync"

	"github.com/nowen-video/nowen-video/internal/model"
	transcodeprobe "github.com/nowen-video/nowen-video/internal/transcode/probe"
)

type transcodeProbeHolder struct {
	once    sync.Once
	service *transcodeprobe.Service
	err     error
}

// The application creates one TranscodeService per database. Keeping the Probe
// service keyed by that owner preserves single-flight and metrics without
// widening the already large constructor surface. Tests using independent
// service instances remain isolated.
var transcodeProbeServices sync.Map // map[*TranscodeService]*transcodeProbeHolder

func (s *TranscodeService) mediaProbeService() (*transcodeprobe.Service, error) {
	if s == nil || s.repo == nil || s.cfg == nil {
		return nil, errors.New("transcode service is unavailable")
	}
	value, _ := transcodeProbeServices.LoadOrStore(s, &transcodeProbeHolder{})
	holder := value.(*transcodeProbeHolder)
	holder.once.Do(func() {
		holder.service, holder.err = transcodeprobe.NewService(
			s.repo.DB(),
			s.cfg.App.FFprobePath,
			s.logger,
		)
	})
	return holder.service, holder.err
}

func (s *TranscodeService) probeMediaForPlan(media *model.Media) *model.MediaProbeRecord {
	probeService, err := s.mediaProbeService()
	if err != nil {
		if s != nil && s.logger != nil {
			s.logger.Warnf("初始化媒体 Probe 服务失败: %v", err)
		}
		return nil
	}
	record, err := probeService.Probe(context.Background(), media)
	if err != nil {
		if !errors.Is(err, transcodeprobe.ErrUnsupportedSource) && s.logger != nil {
			s.logger.Warnf("媒体 Probe 失败，使用兼容转码参数 media=%s: %v", media.ID, err)
		}
		return nil
	}
	transcodeprobe.ApplyToMedia(media, record)
	return record
}

func (s *TranscodeService) GetMediaProbeStats() transcodeprobe.Stats {
	probeService, err := s.mediaProbeService()
	if err != nil || probeService == nil {
		return transcodeprobe.Stats{}
	}
	return probeService.Stats()
}
