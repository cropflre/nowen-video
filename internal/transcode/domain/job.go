package domain

import "time"

// Intent describes why media processing was requested. It is deliberately
// independent from transport protocols so Web, desktop, Android and Emby can
// share the same execution model.
type Intent string

const (
	IntentRuntimeHLS             Intent = "runtime_hls"
	IntentStartupHLS             Intent = "startup_hls"
	IntentStartupContinuationHLS Intent = "startup_continuation_hls"
	IntentPreprocessHLS          Intent = "preprocess_hls"
	IntentRemuxStream            Intent = "remux_stream"
	IntentSmartRemux             Intent = "smart_remux"
	IntentVideoSegment           Intent = "ondemand_video_segment"
	IntentAudioSegment           Intent = "ondemand_audio_segment"
)

// Status is the persisted lifecycle of a media processing job.
type Status string

const (
	StatusQueued          Status = "queued"
	StatusClaimed         Status = "claimed"
	StatusPreparing       Status = "preparing"
	StatusRunning         Status = "running"
	StatusSuspended       Status = "suspended"
	StatusCancelRequested Status = "cancel_requested"
	StatusCancelling      Status = "cancelling"
	StatusCancelled       Status = "cancelled"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
)

// DesiredState is the durable operator intent. Cancellation is state, not a
// best-effort channel message, so a queued job cannot lose a cancel request.
type DesiredState string

const (
	DesiredRunning   DesiredState = "running"
	DesiredCancelled DesiredState = "cancelled"
)

// Job is the transport-neutral execution request used by the new orchestrator.
// Legacy TranscodeTask rows are adapted to this model during migration.
type Job struct {
	ID                string
	MediaID           string
	Intent            Intent
	ProfileID         string
	AudioTrack        int
	StartMS           int64
	DurationMS        int64
	Priority          int
	Status            Status
	DesiredState      DesiredState
	ActiveKey         string
	SourceFingerprint string
	PlanHash          string
	PlannerVersion    string
	SessionID         string
	CurrentAttemptID  string
	CancelRequestedAt *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CompletedAt       *time.Time
}

func (j Job) CancellationRequested() bool {
	return j.DesiredState == DesiredCancelled || j.Status == StatusCancelRequested || j.Status == StatusCancelling
}

func (j Job) Terminal() bool {
	switch j.Status {
	case StatusCancelled, StatusCompleted, StatusFailed:
		return true
	default:
		return false
	}
}
