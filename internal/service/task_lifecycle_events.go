package service

// TaskLifecycleUpdate is the generic task invalidation envelope emitted beside
// the existing module-specific WebSocket event. Consumers should treat it as a
// signal to refresh the authoritative task snapshot rather than reconstructing
// task state from event payloads.
type TaskLifecycleUpdate struct {
	Kind        string `json:"kind"`
	SourceID    string `json:"source_id,omitempty"`
	Status      string `json:"status"`
	SourceEvent string `json:"source_event"`
}

func taskLifecycleUpdateForEvent(eventType string, data interface{}) (*TaskLifecycleUpdate, bool) {
	update := &TaskLifecycleUpdate{SourceEvent: eventType}

	switch eventType {
	case EventScanStarted, EventScanProgress, EventScanPhase:
		update.Kind = TaskKindScan
		update.Status = TaskStatusRunning
	case EventScanCompleted:
		update.Kind = TaskKindScan
		update.Status = TaskStatusCompleted
	case EventScanFailed:
		update.Kind = TaskKindScan
		update.Status = TaskStatusFailed
	case EventScrapeStarted, EventScrapeProgress:
		update.Kind = TaskKindScrape
		update.Status = TaskStatusRunning
	case EventScrapeCompleted:
		update.Kind = TaskKindScrape
		update.Status = TaskStatusCompleted
	case EventTranscodeStarted, EventTranscodeProgress:
		update.Kind = TaskKindTranscode
		update.Status = TaskStatusRunning
	case EventTranscodeCompleted:
		update.Kind = TaskKindTranscode
		update.Status = TaskStatusCompleted
	case EventTranscodeFailed:
		update.Kind = TaskKindTranscode
		update.Status = TaskStatusFailed
	case EventStorageHealthUpdated:
		update.Kind = TaskKindStorageIncident
		update.Status = TaskStatusFailed
		if status, ok := data.(TranscodeStorageHealthStatus); ok && status.State == "healthy" {
			update.Status = TaskStatusCompleted
		}
		if status, ok := data.(*TranscodeStorageHealthStatus); ok && status != nil && status.State == "healthy" {
			update.Status = TaskStatusCompleted
		}
	default:
		return nil, false
	}

	update.SourceID = taskLifecycleSourceID(data)
	return update, true
}

func taskLifecycleSourceID(data interface{}) string {
	switch value := data.(type) {
	case *ScanProgressData:
		return value.LibraryID
	case ScanProgressData:
		return value.LibraryID
	case *ScanPhaseData:
		return value.LibraryID
	case ScanPhaseData:
		return value.LibraryID
	case *ScrapeProgressData:
		return value.LibraryID
	case ScrapeProgressData:
		return value.LibraryID
	case *TranscodeProgressData:
		return value.TaskID
	case TranscodeProgressData:
		return value.TaskID
	case *TranscodeStorageHealthStatus:
		if value != nil {
			return value.IncidentID
		}
	case TranscodeStorageHealthStatus:
		return value.IncidentID
	default:
		return ""
	}
	return ""
}
