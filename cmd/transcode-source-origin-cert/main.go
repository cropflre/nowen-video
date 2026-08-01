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
		timeout     = flag.Duration("timeout", 30*time.Minute, "maximum certification runtime")
		listCases   = flag.Bool("list", false, "list registered source-origin cases")
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
	matrix, err := transcodecertification.RunSourceOriginMatrix(ctx, transcodecertification.Config{
		FFmpegPath:  *ffmpegPath,
		FFprobePath: *ffprobePath,
		WorkDir:     *workDir,
		KeepWorkDir: *keepWork,
	})
	if err != nil {
		fatalf("source-origin certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalSourceOriginMatrixReport(matrix)
	if err != nil {
		fatalf("encode source-origin matrix: %v", err)
	}
	for _, report := range matrix.Cases {
		fmt.Fprintf(
			os.Stderr,
			"case=%s mode=%s declared_fps_milli=%d source_offset_us=%d video_first_pts_us=%d audio_first_pts_us=%d video_duration_spread_us=%d startup_video_ms=%d continuation_video_ms=%d av_delta_skew_us=%d discontinuity_required=%t\n",
			report.Case.ID,
			report.Case.SourceMode,
			report.Case.DeclaredFrameRateMilli(),
			report.Case.SourceOffsetMicros,
			report.Evidence.SourceVideo.FirstPTSMicros,
			report.Evidence.SourceAudio.FirstPTSMicros,
			report.Evidence.SourceVideo.DurationSpreadMicros,
			report.Evidence.NormalizedStartupVideoStartMS,
			report.Evidence.NormalizedContinuationVideoMS,
			report.AVSync.BoundaryDeltaSkewMicros,
			report.Evidence.DiscontinuityRequired,
		)
	}
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write source-origin report: %v", err)
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
	fmt.Fprintf(os.Stderr, "source-origin report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
