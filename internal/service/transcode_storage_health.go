package service

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	transcodestoragefault "github.com/nowen-video/nowen-video/internal/transcode/storagefault"
)

const (
	transcodeStorageHealthInterval = 30 * time.Second
	storageHealthProbeOperation    = "artifact_store_probe"
	EventStorageHealthUpdated      = "storage_health_updated"
)

var ErrTranscodeStorageUnavailable = errors.New("transcode artifact store is unavailable")

type TranscodeStorageHealthStatus struct {
	State             string    `json:"state"`
	Code              string    `json:"code,omitempty"`
	Severity          string    `json:"severity,omitempty"`
	Operation         string    `json:"operation,omitempty"`
	Path              string    `json:"path,omitempty"`
	Message           string    `json:"message,omitempty"`
	Retryable         bool      `json:"retryable"`
	Writable          bool      `json:"writable"`
	AdmissionBlocked  bool      `json:"admission_blocked"`
	QueuePaused       bool      `json:"queue_paused"`
	IncidentID        string    `json:"incident_id,omitempty"`
	Occurrences       int64     `json:"occurrences"`
	ProbeLatencyMS    int64     `json:"probe_latency_ms"`
	LastCheckedAt     time.Time `json:"last_checked_at"`
	LastSuccessfulAt  time.Time `json:"last_successful_at,omitempty"`
	ActiveIncidents   int64     `json:"active_incidents"`
	CriticalIncidents int64     `json:"critical_incidents"`
	RecoveredCount    int64     `json:"recovered_count"`
	LastError         string    `json:"last_error,omitempty"`
}

type transcodeStorageHealthState struct {
	mu             sync.Mutex
	startOnce      sync.Once
	status         TranscodeStorageHealthStatus
	lastEvaluation time.Time
}

var transcodeStorageHealthStates sync.Map

func (s *TranscodeService) storageHealthState() *transcodeStorageHealthState {
	if s == nil {
		return &transcodeStorageHealthState{}
	}
	state := &transcodeStorageHealthState{}
	actual, _ := transcodeStorageHealthStates.LoadOrStore(s, state)
	return actual.(*transcodeStorageHealthState)
}

func (s *TranscodeService) initializeStorageHealth() error {
	if s == nil || s.repo == nil || s.repo.DB() == nil || s.executionRepo == nil {
		return fmt.Errorf("transcode storage health dependencies are unavailable")
	}
	if err := model.AutoMigrateTranscodeStorageIncidents(s.repo.DB()); err != nil {
		return err
	}
	state := s.storageHealthState()
	s.runStorageHealthTick(time.Now(), true)
	state.startOnce.Do(func() { go s.storageHealthLoop() })
	return nil
}

func (s *TranscodeService) storageHealthLoop() {
	ticker := time.NewTicker(transcodeStorageHealthInterval)
	defer ticker.Stop()
	for now := range ticker.C {
		if s == nil || s.jobs == nil || s.jobs.IsClosed() {
			return
		}
		s.runStorageHealthTick(now, true)
	}
}

func (s *TranscodeService) runStorageHealthTick(now time.Time, force bool) TranscodeStorageHealthStatus {
	state := s.storageHealthState()
	state.mu.Lock()
	defer state.mu.Unlock()
	if !force && !state.lastEvaluation.IsZero() && now.Sub(state.lastEvaluation) < transcodeStorageHealthInterval {
		return state.status
	}
	state.lastEvaluation = now
	previous := state.status
	status := TranscodeStorageHealthStatus{
		State:         "healthy",
		Writable:      true,
		LastCheckedAt: now,
	}
	if s != nil && s.artifactStore != nil {
		status.Path = s.artifactStore.Root()
	}

	probeStarted := time.Now()
	probe, probeErr := s.artifactStore.ProbeWritable(now)
	status.ProbeLatencyMS = time.Since(probeStarted).Milliseconds()
	if probeErr != nil {
		classified := transcodestoragefault.Classify(probeErr)
		status.State = "degraded"
		if classified.Severity == transcodestoragefault.SeverityCritical {
			status.State = "critical"
		}
		status.Code = classified.Code
		status.Severity = classified.Severity
		status.Operation = storageHealthProbeOperation
		status.Message = probeErr.Error()
		status.Retryable = classified.Retryable
		status.Writable = false
		status.AdmissionBlocked = true
		status.QueuePaused = true
		status.LastSuccessfulAt = previous.LastSuccessfulAt
		incident, reportErr := s.executionRepo.ReportStorageIncident(repository.TranscodeStorageIncidentInput{
			Code:             classified.Code,
			Severity:         classified.Severity,
			Operation:        storageHealthProbeOperation,
			Path:             status.Path,
			Message:          probeErr.Error(),
			Retryable:        classified.Retryable,
			AdmissionBlocked: true,
			QueuePaused:      true,
		}, now)
		if reportErr != nil {
			status.LastError = reportErr.Error()
		} else if incident != nil {
			status.IncidentID = incident.ID
			status.Occurrences = incident.Occurrences
		}
		if s.logger != nil {
			s.logger.Warnf("Artifact Store 写探针失败，转码队列已暂停 code=%s path=%s: %v", status.Code, status.Path, probeErr)
		}
	} else {
		status.Path = probe.Root
		status.Writable = probe.Writable
		status.ProbeLatencyMS = probe.Latency.Milliseconds()
		status.LastSuccessfulAt = now
		// The successful full write/fsync/rename/remove probe is the recovery
		// authority for both periodic probe incidents and immediate failures
		// raised by workspace preparation, publication or cleanup.
		if recovered, recoverErr := s.executionRepo.RecoverStorageIncidents("", now); recoverErr != nil {
			status.LastError = recoverErr.Error()
		} else if recovered > 0 && s.logger != nil {
			s.logger.Infof("Artifact Store 写探针恢复，已关闭 %d 条存储故障 Incident", recovered)
		}
	}

	if summary, summaryErr := s.executionRepo.StorageIncidentSummary(); summaryErr == nil {
		status.ActiveIncidents = summary.ActiveCount
		status.CriticalIncidents = summary.CriticalCount
		status.RecoveredCount = summary.RecoveredCount
	} else if status.LastError == "" {
		status.LastError = summaryErr.Error()
	}
	state.status = status
	if storageHealthTransitioned(previous, status) && s.wsHub != nil {
		s.wsHub.BroadcastEvent(EventStorageHealthUpdated, status)
	}
	return state.status
}

func storageHealthTransitioned(previous, current TranscodeStorageHealthStatus) bool {
	return previous.State != current.State || previous.Code != current.Code || previous.IncidentID != current.IncidentID || previous.ActiveIncidents != current.ActiveIncidents
}

func (s *TranscodeService) checkStorageHealthAdmission() error {
	status := s.runStorageHealthTick(time.Now(), false)
	if !status.AdmissionBlocked {
		return nil
	}
	return fmt.Errorf("%w: code=%s path=%s message=%s", ErrTranscodeStorageUnavailable, status.Code, status.Path, status.Message)
}

func (s *TranscodeService) GetStorageHealthStatus() TranscodeStorageHealthStatus {
	return s.runStorageHealthTick(time.Now(), false)
}
