package service

import (
	"context"
	"time"
)

const defaultTranscodeShutdownCleanupTimeout = 5 * time.Second

// Shutdown stops accepting local queue deliveries and waits for already claimed
// jobs. If the caller deadline expires, owned leases are atomically returned to
// queued before their old contexts are cancelled, allowing the next process to
// resume them without accepting a stale terminal write.
func (s *TranscodeService) Shutdown(ctx context.Context) error {
	if s == nil || s.jobs == nil {
		return nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	drained := s.jobs.CloseAndDrain()
	for _, job := range drained {
		s.removeQueuedLocalJob(job)
	}
	if len(drained) > 0 {
		s.logger.Infof("服务关闭：保留 %d 个未 Claim 转码 Job 到持久队列", len(drained))
	}

	if s.localJobCount() == 0 {
		return nil
	}
	if err := s.waitForLocalJobs(ctx); err == nil {
		return nil
	}

	s.logger.Warnf("转码优雅关闭等待超时，正在释放本机 Worker Lease")
	s.releaseLocalLeasesForShutdown()

	cleanupCtx, cancel := context.WithTimeout(context.Background(), defaultTranscodeShutdownCleanupTimeout)
	defer cancel()
	_ = s.waitForLocalJobs(cleanupCtx)
	return ctx.Err()
}

func (s *TranscodeService) removeQueuedLocalJob(job *TranscodeJob) {
	if job == nil || job.Task == nil {
		return
	}
	s.mu.Lock()
	if s.running[job.Task.ID] == job {
		delete(s.running, job.Task.ID)
	}
	s.mu.Unlock()
	s.markJobQueuedForRecovery(job)
	job.RequestCancel()
}

func (s *TranscodeService) releaseLocalLeasesForShutdown() {
	now := time.Now()
	jobs := s.localJobsSnapshot()
	for _, job := range jobs {
		if job == nil || job.ExecutionJob == nil {
			continue
		}
		job.stopLeaseHeartbeat()
		released := false
		if job.leaseToken != "" {
			ok, err := s.executionRepo.RequeueLeasedJob(job.ExecutionJob.ID, job.leaseToken, now)
			if err != nil {
				s.logger.Warnf("服务关闭释放转码 Lease 失败 job=%s worker=%s: %v", job.ExecutionJob.ID, job.workerID, err)
			} else {
				released = ok
			}
		}
		if released {
			s.markJobQueuedForRecovery(job)
			s.logger.Infof("服务关闭已重新排队转码 Job job=%s worker=%s", job.ExecutionJob.ID, job.workerID)
		}
		job.RequestCancel()
	}
}

func (s *TranscodeService) markJobQueuedForRecovery(job *TranscodeJob) {
	if job == nil || job.Task == nil {
		return
	}
	job.taskMu.Lock()
	job.Task.Status = "pending"
	job.Task.Progress = 0
	job.Task.Error = ""
	job.Task.StartedAt = nil
	job.Task.CompletedAt = nil
	err := s.repo.Update(job.Task)
	job.taskMu.Unlock()
	if err != nil {
		s.logger.Warnf("服务关闭更新兼容任务为待恢复失败 task=%s: %v", job.Task.ID, err)
	}
}

func (s *TranscodeService) waitForLocalJobs(ctx context.Context) error {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		if s.localJobCount() == 0 {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (s *TranscodeService) localJobCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.running)
}

func (s *TranscodeService) localJobsSnapshot() []*TranscodeJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	jobs := make([]*TranscodeJob, 0, len(s.running))
	for _, job := range s.running {
		jobs = append(jobs, job)
	}
	return jobs
}
