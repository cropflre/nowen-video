package service

import (
	"time"

	"github.com/nowen-video/nowen-video/internal/repository"
	transcodestoragefault "github.com/nowen-video/nowen-video/internal/transcode/storagefault"
)

const (
	storageOperationPrepareWorkspace = "prepare_workspace"
	storageOperationPublishArtifact  = "publish_artifact"
	storageOperationCleanupArtifact  = "cleanup_artifact"
)

// reportStorageOperationFailure promotes a live filesystem failure immediately
// instead of waiting for the next periodic write probe. Unknown logical or
// invariant errors are deliberately ignored here and remain owned by their
// domain-specific Artifact/Cleanup state.
func (s *ArtifactMaintenanceService) reportStorageOperationFailure(operation, path string, cause error, now time.Time) {
	if s == nil || s.executionRepo == nil || cause == nil {
		return
	}
	classified := transcodestoragefault.Classify(cause)
	if classified.Code == "" || classified.Code == transcodestoragefault.CodeUnknown {
		return
	}
	incident, err := s.executionRepo.ReportStorageIncident(repository.TranscodeStorageIncidentInput{
		Code:             classified.Code,
		Severity:         classified.Severity,
		Operation:        operation,
		Path:             path,
		Message:          cause.Error(),
		Retryable:        classified.Retryable,
		AdmissionBlocked: true,
		QueuePaused:      true,
	}, now)
	if err != nil {
		if s.logger != nil {
			s.logger.Warnf("持久化 Artifact Store 操作故障失败 operation=%s path=%s: %v", operation, path, err)
		}
		return
	}
	// Force the in-memory gate to observe the failure immediately. The real
	// probe remains the only authority that may recover the incident.
	state := s.storageHealthState()
	state.mu.Lock()
	state.status.State = "critical"
	state.status.Code = classified.Code
	state.status.Severity = classified.Severity
	state.status.Operation = operation
	state.status.Path = path
	state.status.Message = cause.Error()
	state.status.Retryable = classified.Retryable
	state.status.Writable = false
	state.status.AdmissionBlocked = true
	state.status.QueuePaused = true
	state.status.LastCheckedAt = now
	if incident != nil {
		state.status.IncidentID = incident.ID
		state.status.Occurrences = incident.Occurrences
	}
	status := state.status
	state.mu.Unlock()
	if s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventStorageHealthUpdated, status)
	}
}
