package service

import (
	"context"
	"time"

	"github.com/nowen-video/nowen-video/internal/repository"
)

// attachProbeWarmup connects scan completion to the same Probe service used by
// runtime transcoding. The warmup is owned by the transcode scheduler lifecycle:
// closing the durable scheduler stops new warmups and waits for workers to exit.
func (s *TranscodeService) attachProbeWarmup(hub *WSHub) {
	if s == nil || hub == nil || s.repo == nil || s.mediaProbe == nil || s.jobs == nil {
		return
	}
	s.probeWarmupOnce.Do(func() {
		warmup := NewMediaProbeWarmupService(
			repository.NewMediaProbeWarmupRepo(s.repo.DB()),
			s.mediaProbe,
			s.logger,
			s.jobs.Done(),
		)
		warmup.SetOnProbed(func(media, probe interfaceMediaProbeRecord) (bool, error) {
			return false, nil
		})
		s.probeWarmup = warmup

		unsubscribe := hub.SubscribeInternal(EventScanCompleted, func(event WSEvent) {
			libraryID := taskLifecycleSourceID(event.Data)
			if libraryID == "" {
				if s.logger != nil {
					s.logger.Warn("扫描完成事件缺少 library_id，跳过媒体 Probe 预热")
				}
				return
			}
			submitted, err := warmup.SubmitLibrary(libraryID)
			if err != nil {
				if s.logger != nil {
					s.logger.Warnf("提交媒体 Probe 预热失败 library=%s: %v", libraryID, err)
				}
				return
			}
			if submitted && s.logger != nil {
				s.logger.Infof("扫描完成，已提交媒体 Probe 预热 library=%s", libraryID)
			}
		})

		go func() {
			<-s.jobs.Done()
			unsubscribe()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := warmup.Shutdown(shutdownCtx); err != nil && s.logger != nil {
				s.logger.Warnf("媒体 Probe 预热服务关闭超时: %v", err)
			}
		}()
	})
}

func (s *TranscodeService) GetMediaProbeWarmupStats() MediaProbeWarmupStats {
	if s == nil || s.probeWarmup == nil {
		return MediaProbeWarmupStats{}
	}
	return s.probeWarmup.Stats()
}
