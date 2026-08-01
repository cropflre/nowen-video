package service

import "time"

func (s *TranscodeService) abandonAttemptArtifacts(attemptID, errorCode, errorMessage string, at time.Time) {
	if s == nil || s.executionRepo == nil || attemptID == "" {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	if err := s.executionRepo.AbandonArtifactsForAttempt(attemptID, errorCode, errorMessage, at); err != nil {
		s.logger.Warnf("标记失效 Attempt Artifact 失败 attempt=%s code=%s: %v", attemptID, errorCode, err)
	}
}

func (s *TranscodeService) abandonJobArtifacts(job *TranscodeJob, errorCode, errorMessage string, at time.Time) {
	if job == nil {
		return
	}
	attemptID := ""
	if job.CurrentAttempt != nil {
		attemptID = job.CurrentAttempt.ID
	}
	if attemptID == "" && job.ExecutionJob != nil {
		attemptID = job.ExecutionJob.CurrentAttemptID
	}
	s.abandonAttemptArtifacts(attemptID, errorCode, errorMessage, at)
}
