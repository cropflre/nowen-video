package domain

import "time"

// Backend is the concrete media backend used by one execution attempt.
type Backend string

const (
	BackendSoftware Backend = "software"
	BackendNVENC    Backend = "nvenc"
	BackendQSV      Backend = "qsv"
	BackendVAAPI    Backend = "vaapi"
)

// AttemptStatus is intentionally separate from Job status. A job can survive a
// failed hardware attempt and complete through a software fallback attempt.
type AttemptStatus string

const (
	AttemptPreparing AttemptStatus = "preparing"
	AttemptRunning   AttemptStatus = "running"
	AttemptCompleted AttemptStatus = "completed"
	AttemptFailed    AttemptStatus = "failed"
	AttemptCancelled AttemptStatus = "cancelled"
)

// Attempt captures process-level evidence required for diagnostics and future
// persistence. CommandArgs must be redacted before it is exposed externally.
type Attempt struct {
	ID          string
	JobID       string
	Number      int
	Backend     Backend
	Status      AttemptStatus
	PID         int
	CommandPath string
	CommandArgs []string
	StartedAt   time.Time
	CompletedAt time.Time
	ExitCode    int
	Signal      string
	StderrTail  []string
	ErrorCode   string
	Error       string
}
