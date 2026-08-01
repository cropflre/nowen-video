package certification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	timestampexecution "github.com/nowen-video/nowen-video/internal/transcode/timestampexecution"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func produceShapedContinuationHLS(
	ctx context.Context,
	ffmpegPath,
	workDir,
	name,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	fixture FixtureSpec,
	startMicros int64,
	executionPlan timestampexecution.Plan,
) (string, error) {
	directory := filepath.Join(workDir, name)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return "", fmt.Errorf("create shaped continuation directory: %w", err)
	}
	args, err := boundaryHLSArgs(sourcePath, directory, timestampPlan, fixture, startMicros, 0)
	if err != nil {
		return "", fmt.Errorf("build shaped continuation command: %w", err)
	}
	args, err = timestampexecution.ApplyContinuation(args, executionPlan)
	if err != nil {
		return "", fmt.Errorf("apply timestamp execution plan: %w", err)
	}
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return "", fmt.Errorf("produce shaped continuation fixture: %w", err)
	}
	return filepath.Join(directory, "stream.m3u8"), nil
}
