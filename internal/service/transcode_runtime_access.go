package service

import transcoderuntime "github.com/nowen-video/nowen-video/internal/transcode/runtime"

// ExecutionRuntime exposes the shared process and resource-admission boundary
// to session-scoped playback orchestration. Callers cannot mutate persistent
// Job, Lease, Attempt, or Artifact state through this accessor.
func (s *TranscodeService) ExecutionRuntime() *transcoderuntime.Runtime {
	if s == nil {
		return nil
	}
	return s.executionRuntime
}
