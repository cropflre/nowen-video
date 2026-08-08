package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestProcessRunnerParsesMachineProgress(t *testing.T) {
	var events []Progress
	result := NewProcessRunner().Run(context.Background(), helperCommand("progress"), Callbacks{
		OnProgress: func(progress Progress) {
			events = append(events, progress)
		},
	})
	if result.Err != nil {
		t.Fatalf("run failed: %v, stderr=%v", result.Err, result.StderrTail)
	}
	if len(events) != 2 {
		t.Fatalf("expected 2 progress events, got %d: %+v", len(events), events)
	}
	if events[0].OutTimeMS != 1500 || events[0].Speed != "1.25x" || events[0].State != "continue" {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	if events[1].OutTimeMS != 3000 || events[1].State != "end" {
		t.Fatalf("unexpected final event: %+v", events[1])
	}
}

func TestProcessRunnerCancellationKillsProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	done := make(chan Result, 1)
	go func() {
		done <- NewProcessRunner().Run(ctx, helperCommand("wait"), Callbacks{
			OnStarted: func(*os.Process) { close(started) },
		})
	}()

	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("helper process did not start")
	}
	cancel()

	select {
	case result := <-done:
		if !result.Cancelled || result.Err == nil {
			t.Fatalf("expected cancelled result, got %+v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled process did not exit")
	}
}

func TestProcessRunnerFailsClosedOnENOSPCStderrBeyondTail(t *testing.T) {
	result := NewProcessRunner().Run(context.Background(), helperCommand("enospc"), Callbacks{})
	if result.ExitCode != 0 {
		t.Fatalf("helper exit code = %d, want the real exit code 0", result.ExitCode)
	}
	if result.FatalOutputCode != FatalOutputCodeENOSPC {
		t.Fatalf("fatal output code = %q", result.FatalOutputCode)
	}
	if strings.Contains(strings.Join(result.StderrTail, "\n"), "No space left on device") {
		t.Fatal("test did not evict ENOSPC from bounded stderr tail")
	}
	var fatal *FatalOutputError
	if !errors.As(result.Err, &fatal) || fatal.Code != FatalOutputCodeENOSPC {
		t.Fatalf("expected typed fatal output error, got %T %v", result.Err, result.Err)
	}
}

func TestDetectFatalOutputIgnoresOrdinaryProgress(t *testing.T) {
	if code, line, ok := DetectFatalOutput([]string{"progress=end", "video:123kB"}); ok {
		t.Fatalf("ordinary output classified as fatal: code=%q line=%q", code, line)
	}
}

func TestResultErrorTextIncludesStderr(t *testing.T) {
	result := Result{Err: fmt.Errorf("boom"), StderrTail: []string{"line one", "line two"}}
	text := result.ErrorText()
	for _, expected := range []string{"boom", "line one", "line two"} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing %q in %q", expected, text)
		}
	}
}

func helperCommand(mode string) Command {
	return Command{
		Path:       os.Args[0],
		Args:       []string{"-test.run=TestProcessRunnerHelper", "--", mode},
		Env:        []string{"NOWEN_PROCESS_RUNNER_HELPER=1"},
		StderrTail: 20,
	}
}

func TestProcessRunnerHelper(t *testing.T) {
	if os.Getenv("NOWEN_PROCESS_RUNNER_HELPER") != "1" {
		return
	}
	mode := os.Args[len(os.Args)-1]
	switch mode {
	case "progress":
		fmt.Fprintln(os.Stderr, "out_time_us=1500000")
		fmt.Fprintln(os.Stderr, "speed=1.25x")
		fmt.Fprintln(os.Stderr, "progress=continue")
		fmt.Fprintln(os.Stderr, "out_time_us=3000000")
		fmt.Fprintln(os.Stderr, "speed=2.00x")
		fmt.Fprintln(os.Stderr, "progress=end")
		os.Exit(0)
	case "enospc":
		fmt.Fprintln(os.Stderr, "[hls @ 0x1] Failed to open file 'seg0000.ts'")
		fmt.Fprintln(os.Stderr, "av_interleaved_write_frame(): No space left on device")
		for index := 0; index < 50; index++ {
			fmt.Fprintf(os.Stderr, "diagnostic line %d\n", index)
		}
		fmt.Fprintln(os.Stderr, "progress=end")
		os.Exit(0)
	case "wait":
		time.Sleep(30 * time.Second)
		os.Exit(0)
	default:
		os.Exit(2)
	}
}
