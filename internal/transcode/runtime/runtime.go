package runtime

import (
	"context"
	"fmt"

	"github.com/nowen-video/nowen-video/internal/transcode/executor"
	"github.com/nowen-video/nowen-video/internal/transcode/governor"
)

// Runtime is the single process execution boundary shared by runtime HLS,
// remux, smart-remux and on-demand media work. It owns process running and
// resource admission; callers own planning and artifact semantics.
type Runtime struct {
	runner   executor.Runner
	governor *governor.Governor
}

func New(runner executor.Runner, resourceGovernor *governor.Governor) *Runtime {
	if runner == nil {
		runner = executor.NewProcessRunner()
	}
	if resourceGovernor == nil {
		resourceGovernor = governor.New(governor.DefaultConfig())
	}
	return &Runtime{runner: runner, governor: resourceGovernor}
}

func Default() *Runtime {
	return New(executor.NewProcessRunner(), governor.New(governor.DefaultConfig()))
}

func (r *Runtime) Run(ctx context.Context, kind governor.Kind, command executor.Command, callbacks executor.Callbacks) executor.Result {
	if r == nil || r.governor == nil || r.runner == nil {
		return executor.Result{Err: fmt.Errorf("media execution runtime is unavailable")}
	}
	lease, err := r.governor.Acquire(ctx, kind)
	if err != nil {
		return executor.Result{Err: err, Cancelled: ctx != nil && ctx.Err() == context.Canceled, TimedOut: ctx != nil && ctx.Err() == context.DeadlineExceeded}
	}
	defer lease.Release()
	return r.runner.Run(ctx, command, callbacks)
}

func (r *Runtime) Snapshot() governor.Snapshot {
	if r == nil || r.governor == nil {
		return governor.Snapshot{
			Capacity:  map[governor.Kind]int{},
			InUse:     map[governor.Kind]int{},
			Waiting:   map[governor.Kind]int{},
			PeakInUse: map[governor.Kind]int{},
		}
	}
	return r.governor.Snapshot()
}
