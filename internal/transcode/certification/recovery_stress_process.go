package certification

import (
	"context"
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/transcode/executor"
	"github.com/nowen-video/nowen-video/internal/transcode/governor"
	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
)

type processControl struct {
	CancelAtMicros     int64
	KillAtMicros       int64
	ExtraEnv           []string
	CommandPath        string
	CommandArgs        []string
	MemoryPeakPath     string
	ResourceController string
	CPUCountLimit      int
	MemoryLimitBytes   int64
	FaultBackend       string
}

func (h *recoveryHarness) runAttempt(ctx context.Context, job *model.TranscodeJobRecord, attempt *recoveryAttempt, control processControl) (transcoderecovery.ProcessEvidence, error) {
	processCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	commandPath := h.ffmpegPath
	commandArgs := append([]string(nil), attempt.Args...)
	if control.CommandPath != "" {
		commandPath = control.CommandPath
		commandArgs = append([]string(nil), control.CommandArgs...)
	}
	var (
		process       *os.Process
		triggerOnce   sync.Once
		triggerMicros atomic.Int64
		maximum       atomic.Int64
		maximumRSS    atomic.Int64
	)
	done := make(chan struct{})
	startedAt := time.Now()
	result := h.runtime.Run(processCtx, governor.KindSoftwareTranscode, executor.Command{
		Path:       commandPath,
		Args:       commandArgs,
		Env:        control.ExtraEnv,
		StderrTail: 200,
	}, executor.Callbacks{
		OnStarted: func(started *os.Process) {
			process = started
			_ = h.repo.MarkAttemptStarted(attempt.Record.ID, started.Pid, time.Now())
			sampleRSS(started.Pid, &maximumRSS)
			go monitorRSS(started.Pid, done, &maximumRSS)
		},
		OnProgress: func(progress executor.Progress) {
			observed := progress.OutTimeMS * 1000
			setAtomicMaximum(&maximum, observed)
			if control.CancelAtMicros > 0 && observed >= control.CancelAtMicros {
				triggerOnce.Do(func() {
					triggerMicros.Store(observed)
					_ = h.repo.RequestCancellation(job.ID, time.Now())
					h.transition("cancel_requested", "cancelled", attempt.Ordinal, attempt.Ordinal, "staging", "cancellation requested after active segment writes")
					cancel()
				})
			}
			if control.KillAtMicros > 0 && observed >= control.KillAtMicros {
				triggerOnce.Do(func() {
					triggerMicros.Store(observed)
					if process != nil {
						_ = process.Kill()
					}
				})
			}
		},
	})
	close(done)
	if control.MemoryPeakPath != "" {
		peak, err := readMemoryPeak(control.MemoryPeakPath)
		if err != nil {
			return transcoderecovery.ProcessEvidence{}, fmt.Errorf("read bounded cgroup memory peak: %w", err)
		}
		setAtomicMaximum(&maximumRSS, peak)
	}
	elapsed := time.Since(startedAt).Milliseconds()
	segments, manifestExists := inspectPartialHLS(attempt.Workspace)
	workspaceExists := pathExists(attempt.Workspace)
	markers := stderrMarkers(result.ErrorText())
	signal := ""
	if control.KillAtMicros > 0 && triggerMicros.Load() > 0 {
		signal = "SIGKILL"
	}
	return transcoderecovery.ProcessEvidence{
		AttemptOrdinal:        attempt.Ordinal,
		CommandHash:           recoveryCommandHash(commandPath, commandArgs, control.ExtraEnv, h.workDir, h.sourcePath),
		ExitCode:              result.ExitCode,
		Cancelled:             result.Cancelled,
		TimedOut:              result.TimedOut,
		Signal:                signal,
		TriggerObservedMicros: triggerMicros.Load(),
		MaximumProgressMicros: maximum.Load(),
		SegmentCount:          segments,
		ManifestExists:        manifestExists,
		WorkspaceExists:       workspaceExists,
		StderrSHA256:          sha256Text(result.ErrorText()),
		StderrMarkers:         markers,
		MaxRSSBytes:           maximumRSS.Load(),
		ElapsedMillis:         elapsed,
		ResourceController:    control.ResourceController,
		CPUCountLimit:         control.CPUCountLimit,
		MemoryLimitBytes:      control.MemoryLimitBytes,
		FaultBackend:          control.FaultBackend,
	}, nil
}
