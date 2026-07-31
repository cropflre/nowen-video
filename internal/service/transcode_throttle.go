package service

import (
	"time"
)

const (
	throttleAheadHighWatermark = 60.0
	throttleAheadLowWatermark  = 15.0
	throttleTickInterval       = 2 * time.Second
)

type ThrottleStats struct {
	ActiveSuspended     int    `json:"active_suspended"`
	TotalSuspendCount   uint64 `json:"total_suspend_count"`
	TotalSuspendSeconds uint64 `json:"total_suspend_seconds"`
}

func (s *TranscodeService) GetThrottleStats() ThrottleStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	active := 0
	for _, job := range s.running {
		if job.suspended.Load() == 1 {
			active++
		}
	}
	return ThrottleStats{
		ActiveSuspended:     active,
		TotalSuspendCount:   s.throttleSuspendCount.Load(),
		TotalSuspendSeconds: s.throttleSuspendSeconds.Load(),
	}
}

func (s *TranscodeService) throttleLoop(job *TranscodeJob) {
	ticker := time.NewTicker(throttleTickInterval)
	defer ticker.Stop()
	var suspendedAt time.Time

	for {
		select {
		case <-job.throttleDone:
			if !suspendedAt.IsZero() {
				s.throttleSuspendSeconds.Add(uint64(time.Since(suspendedAt).Seconds()))
			}
			return
		case <-job.ctx.Done():
			if !suspendedAt.IsZero() {
				s.throttleSuspendSeconds.Add(uint64(time.Since(suspendedAt).Seconds()))
			}
			return
		case <-ticker.C:
		}

		playback := job.getPlaybackPosition()
		if playback <= 0 {
			continue
		}
		transcoded := job.getTranscodedPosition()
		ahead := transcoded - playback
		wasSuspended := job.suspended.Load() == 1
		process := job.currentProcess()
		if process == nil {
			continue
		}

		switch {
		case !wasSuspended && ahead > throttleAheadHighWatermark:
			if err := suspendProcess(process); err == nil {
				job.suspended.Store(1)
				suspendedAt = time.Now()
				s.throttleSuspendCount.Add(1)
				s.logger.Debugf("[throttle] suspend ffmpeg media=%s quality=%s ahead=%.1fs", job.Media.ID, job.Quality, ahead)
			} else {
				s.logger.Warnf("[throttle] suspend failed: %v", err)
			}
		case wasSuspended && ahead < throttleAheadLowWatermark:
			if err := resumeProcess(process); err == nil {
				job.suspended.Store(0)
				if !suspendedAt.IsZero() {
					s.throttleSuspendSeconds.Add(uint64(time.Since(suspendedAt).Seconds()))
					suspendedAt = time.Time{}
				}
				s.logger.Debugf("[throttle] resume ffmpeg media=%s quality=%s ahead=%.1fs", job.Media.ID, job.Quality, ahead)
			} else {
				s.logger.Warnf("[throttle] resume failed: %v", err)
			}
		}
	}
}

func (s *TranscodeService) SetPlaybackPosition(mediaID string, positionSec float64) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, job := range s.running {
		if job.Media.ID == mediaID {
			job.SetPlaybackPosition(positionSec)
		}
	}
}

type MediaThrottleStatus struct {
	MediaID           string   `json:"media_id"`
	Running           bool     `json:"running"`
	ActiveQualityList []string `json:"active_qualities"`
	SuspendedCount    int      `json:"suspended_count"`
	PlaybackPos       float64  `json:"playback_pos"`
	TranscodedPos     float64  `json:"transcoded_pos"`
	AheadSeconds      float64  `json:"ahead_seconds"`
}

func (s *TranscodeService) GetMediaThrottleStatus(mediaID string) MediaThrottleStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	status := MediaThrottleStatus{MediaID: mediaID}
	var maxTranscoded float64
	var playback float64
	for _, job := range s.running {
		if job.Media.ID != mediaID {
			continue
		}
		status.Running = true
		status.ActiveQualityList = append(status.ActiveQualityList, job.Quality)
		if job.suspended.Load() == 1 {
			status.SuspendedCount++
		}
		if value := job.getTranscodedPosition(); value > maxTranscoded {
			maxTranscoded = value
		}
		if value := job.getPlaybackPosition(); value > playback {
			playback = value
		}
	}
	status.PlaybackPos = playback
	status.TranscodedPos = maxTranscoded
	if maxTranscoded > playback {
		status.AheadSeconds = maxTranscoded - playback
	}
	return status
}

func (s *TranscodeService) FindRunningJob(mediaID, quality string) *TranscodeJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, job := range s.running {
		if job.Media.ID == mediaID && job.Quality == quality {
			return job
		}
	}
	return nil
}

func (s *TranscodeService) GetRunningJobs() []*TranscodeJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*TranscodeJob, 0, len(s.running))
	for _, job := range s.running {
		jobs = append(jobs, job)
	}
	return jobs
}
