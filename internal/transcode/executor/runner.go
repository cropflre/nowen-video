package executor

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Progress is emitted from FFmpeg's machine-readable -progress protocol.
type Progress struct {
	OutTimeMS  int64
	CurrentSec float64
	Speed      string
	State      string
}

// Command describes one process attempt. Prepare is called immediately before
// Start and is used by platform adapters to set process priority or groups.
type Command struct {
	Path       string
	Args       []string
	Dir        string
	Env        []string
	Stdin      io.Reader
	Stdout     io.Writer
	StderrTail int
	Prepare    func(*exec.Cmd)
}

// Callbacks expose process lifecycle without leaking ownership of cmd.Wait.
type Callbacks struct {
	OnStarted  func(*os.Process)
	OnProgress func(Progress)
}

// Result is the complete outcome of one process attempt.
type Result struct {
	StartedAt       time.Time
	CompletedAt     time.Time
	ExitCode        int
	Cancelled       bool
	TimedOut        bool
	Err             error
	StderrTail      []string
	FatalOutputCode string
	FatalOutputLine string
}

func (r Result) ErrorText() string {
	parts := make([]string, 0, 2)
	if r.Err != nil {
		parts = append(parts, r.Err.Error())
	}
	if len(r.StderrTail) > 0 {
		parts = append(parts, strings.Join(r.StderrTail, "\n"))
	}
	return strings.TrimSpace(strings.Join(parts, "\n"))
}

// Runner executes one command attempt.
type Runner interface {
	Run(context.Context, Command, Callbacks) Result
}

// ProcessRunner is the production Runner. It uses exec.CommandContext so queued
// and running cancellation share the same durable context signal.
type ProcessRunner struct{}

func NewProcessRunner() *ProcessRunner { return &ProcessRunner{} }

func (r *ProcessRunner) Run(ctx context.Context, command Command, callbacks Callbacks) Result {
	result := Result{ExitCode: -1}
	if ctx == nil {
		ctx = context.Background()
	}
	if strings.TrimSpace(command.Path) == "" {
		result.Err = errors.New("empty process path")
		return result
	}

	cmd := exec.CommandContext(ctx, command.Path, command.Args...)
	cmd.Dir = command.Dir
	if command.Env != nil {
		cmd.Env = append(os.Environ(), command.Env...)
	}
	cmd.Stdin = command.Stdin
	cmd.Stdout = command.Stdout
	if command.Prepare != nil {
		command.Prepare(cmd)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		result.Err = fmt.Errorf("create stderr pipe: %w", err)
		return result
	}

	result.StartedAt = time.Now()
	if err := cmd.Start(); err != nil {
		result.CompletedAt = time.Now()
		result.Err = fmt.Errorf("start process: %w", err)
		return result
	}
	if callbacks.OnStarted != nil {
		callbacks.OnStarted(cmd.Process)
	}

	tailLimit := command.StderrTail
	if tailLimit <= 0 {
		tailLimit = 40
	}
	parser := newProgressParser(tailLimit, callbacks.OnProgress)
	var parseWG sync.WaitGroup
	parseWG.Add(1)
	go func() {
		defer parseWG.Done()
		parser.consume(stderr)
	}()

	// StderrPipe requires the reader to drain to EOF before Wait closes the
	// descriptor. Calling Wait first can truncate early stderr under scheduler
	// pressure, losing fatal evidence such as ENOSPC that has already scrolled
	// beyond the bounded tail. The child closing stderr on exit releases the
	// parser, after which Wait safely reaps the process and records its state.
	parseWG.Wait()
	waitErr := cmd.Wait()
	result.CompletedAt = time.Now()
	result.StderrTail = parser.tail()
	result.FatalOutputCode, result.FatalOutputLine = parser.fatalOutput()
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if ctxErr := ctx.Err(); ctxErr != nil {
		result.Cancelled = errors.Is(ctxErr, context.Canceled)
		result.TimedOut = errors.Is(ctxErr, context.DeadlineExceeded)
		result.Err = ctxErr
		return result
	}
	if waitErr != nil {
		result.Err = waitErr
		return result
	}
	if result.FatalOutputCode != "" {
		result.Err = &FatalOutputError{Code: result.FatalOutputCode, Line: result.FatalOutputLine}
		return result
	}
	return result
}

type progressParser struct {
	mu              sync.Mutex
	values          map[string]string
	stderrTail      []string
	tailLimit       int
	onProgress      func(Progress)
	fatalOutputCode string
	fatalOutputLine string
}

func newProgressParser(tailLimit int, onProgress func(Progress)) *progressParser {
	return &progressParser{
		values:     make(map[string]string),
		stderrTail: make([]string, 0, tailLimit),
		tailLimit:  tailLimit,
		onProgress: onProgress,
	}
}

func (p *progressParser) consume(reader io.Reader) {
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 2*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		p.observeLine(line)
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		p.values[key] = value
		if key == "progress" {
			p.emit(value)
			p.values = make(map[string]string)
		}
	}
}

func (p *progressParser) observeLine(line string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.fatalOutputCode == "" {
		if code, fatalLine, ok := DetectFatalOutput([]string{line}); ok {
			p.fatalOutputCode = code
			p.fatalOutputLine = fatalLine
		}
	}
	if len(p.stderrTail) >= p.tailLimit {
		copy(p.stderrTail, p.stderrTail[1:])
		p.stderrTail[len(p.stderrTail)-1] = line
		return
	}
	p.stderrTail = append(p.stderrTail, line)
}

func (p *progressParser) emit(state string) {
	if p.onProgress == nil {
		return
	}
	outTimeMS := parseOutTimeMS(p.values)
	p.onProgress(Progress{
		OutTimeMS:  outTimeMS,
		CurrentSec: float64(outTimeMS) / 1000,
		Speed:      p.values["speed"],
		State:      state,
	})
}

func (p *progressParser) tail() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := make([]string, len(p.stderrTail))
	copy(result, p.stderrTail)
	return result
}

func (p *progressParser) fatalOutput() (string, string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.fatalOutputCode, p.fatalOutputLine
}

func parseOutTimeMS(values map[string]string) int64 {
	// FFmpeg currently emits out_time_us. Older builds and wrappers may emit
	// out_time_ms with microsecond units, so both are normalized to milliseconds.
	for _, key := range []string{"out_time_us", "out_time_ms"} {
		if raw := values[key]; raw != "" {
			value, err := strconv.ParseInt(raw, 10, 64)
			if err == nil && value >= 0 {
				return value / 1000
			}
		}
	}
	if raw := values["out_time"]; raw != "" {
		parts := strings.Split(raw, ":")
		if len(parts) == 3 {
			hours, _ := strconv.ParseFloat(parts[0], 64)
			minutes, _ := strconv.ParseFloat(parts[1], 64)
			seconds, _ := strconv.ParseFloat(parts[2], 64)
			return int64((hours*3600 + minutes*60 + seconds) * 1000)
		}
	}
	return 0
}
