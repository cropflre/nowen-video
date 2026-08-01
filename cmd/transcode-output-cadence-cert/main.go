package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	transcodecertification "github.com/nowen-video/nowen-video/internal/transcode/certification"
)

func main() {
	var (
		outputPath  = flag.String("output", "-", "JSON report path, or - for stdout")
		workDir     = flag.String("work-dir", "", "certification workspace; temporary by default")
		keepWork    = flag.Bool("keep-work-dir", false, "keep an automatically created workspace")
		ffmpegPath  = flag.String("ffmpeg", "", "ffmpeg executable; resolved from PATH by default")
		ffprobePath = flag.String("ffprobe", "", "ffprobe executable; resolved from PATH by default")
		timeout     = flag.Duration("timeout", 45*time.Minute, "maximum certification runtime")
		listCases   = flag.Bool("list", false, "list registered output-cadence cases")
	)
	flag.Parse()

	if *listCases {
		for _, spec := range transcodecertification.AvailableSourceOriginCases() {
			fmt.Printf("%s\t%s\n", spec.ID, spec.Description)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	matrix, err := transcodecertification.RunOutputCadenceMatrix(ctx, transcodecertification.Config{
		FFmpegPath:  *ffmpegPath,
		FFprobePath: *ffprobePath,
		WorkDir:     *workDir,
		KeepWorkDir: *keepWork,
	})
	if err != nil {
		fatalf("output-cadence certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalOutputCadenceMatrixReport(matrix)
	if err != nil {
		fatalf("encode output-cadence matrix: %v", err)
	}
	for _, report := range matrix.Cases {
		evidence := report.Evidence
		fmt.Fprintf(
			os.Stderr,
			"case=%s mode=%s source_frames=%d startup_in=%d startup_out=%d startup_delta=%d startup_min_us=%d startup_max_us=%d startup_variable=%t continuation_in=%d continuation_out=%d continuation_delta=%d continuation_min_us=%d continuation_max_us=%d continuation_variable=%t duplicate_pts=%d non_monotonic_pts=%d preservation=%s discontinuity_required=%t\n",
			report.Case.ID,
			report.Case.SourceMode,
			evidence.SourceTimeline.FrameCount,
			evidence.StartupMapping.InputFrames,
			evidence.StartupMapping.OutputFrames,
			evidence.StartupMapping.FrameCountDelta,
			evidence.StartupTimeline.MinDeltaMicros,
			evidence.StartupTimeline.MaxDeltaMicros,
			evidence.StartupTimeline.VariableDuration,
			evidence.ContinuationMapping.InputFrames,
			evidence.ContinuationMapping.OutputFrames,
			evidence.ContinuationMapping.FrameCountDelta,
			evidence.ContinuationTimeline.MinDeltaMicros,
			evidence.ContinuationTimeline.MaxDeltaMicros,
			evidence.ContinuationTimeline.VariableDuration,
			evidence.StartupTimeline.DuplicatePTSCount+evidence.ContinuationTimeline.DuplicatePTSCount,
			evidence.StartupTimeline.NonMonotonicPTSCount+evidence.ContinuationTimeline.NonMonotonicPTSCount,
			evidence.PreservationStatus,
			evidence.DiscontinuityRequired,
		)
	}
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write output-cadence report: %v", err)
	}
}

func writeOutput(outputPath string, content []byte) error {
	if outputPath == "-" {
		_, err := os.Stdout.Write(content)
		return err
	}
	absolute, err := filepath.Abs(outputPath)
	if err != nil {
		return fmt.Errorf("resolve output path: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return fmt.Errorf("create output directory: %w", err)
	}
	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "output-cadence report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
