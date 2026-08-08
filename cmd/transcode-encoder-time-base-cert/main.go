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
		timeout     = flag.Duration("timeout", 90*time.Minute, "maximum certification runtime")
		list        = flag.Bool("list", false, "list registered cases and candidates")
	)
	flag.Parse()

	if *list {
		fmt.Println("cases:")
		for _, spec := range transcodecertification.AvailableEncoderTimeBaseCases() {
			fmt.Printf("%s\t%s\tmode=%s primary=%d/%d secondary=%d/%d offset_us=%d gop=%d\n",
				spec.ID,
				spec.Description,
				spec.SourceMode,
				spec.PrimaryFrameRateNumerator,
				spec.PrimaryFrameRateDenominator,
				spec.SecondaryFrameRateNumerator,
				spec.SecondaryFrameRateDenominator,
				spec.SourceOffsetMicros,
				spec.GOPSize,
			)
		}
		fmt.Println("candidates:")
		for _, spec := range transcodecertification.AvailableEncoderTimeBaseCandidates() {
			fmt.Printf("%s\t%s\tenc_tb=%s\n", spec.ID, spec.Description, spec.EncoderTimeBase)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := transcodecertification.RunEncoderTimeBaseMatrix(ctx, transcodecertification.Config{
		FFmpegPath:  *ffmpegPath,
		FFprobePath: *ffprobePath,
		WorkDir:     *workDir,
		KeepWorkDir: *keepWork,
	})
	if err != nil {
		fatalf("encoder time-base certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalEncoderTimeBaseMatrixReport(report)
	if err != nil {
		fatalf("encode encoder time-base matrix: %v", err)
	}
	for _, caseEvidence := range report.Evidence.Cases {
		for _, candidate := range caseEvidence.Candidates {
			fmt.Fprintf(os.Stderr,
				"case=%s candidate=%s repeats=%d startup_frames=%d continuation_frames=%d startup_dominant_us=%d continuation_dominant_us=%d boundary_skew_us=%d stable=%t preserved=%t\n",
				caseEvidence.Case.ID,
				candidate.Spec.ID,
				candidate.Summary.RepeatCount,
				candidate.Summary.StartupFrameCount.Min,
				candidate.Summary.ContinuationFrameCount.Min,
				candidate.Summary.StartupDominantDeltaMicros.Min,
				candidate.Summary.ContinuationDominantDeltaMicros.Min,
				candidate.Summary.BoundaryDeltaSkewMicros.Min,
				candidate.Summary.Stable,
				candidate.Summary.AllPreserved,
			)
		}
		fmt.Fprintf(os.Stderr,
			"case=%s candidates_equivalent=%t max_av_sync_difference_us=%d\n",
			caseEvidence.Case.ID,
			caseEvidence.Comparison.Equivalent,
			caseEvidence.Comparison.MaxAVSyncMetricDifferenceMicros,
		)
	}
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write encoder time-base report: %v", err)
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
	fmt.Fprintf(os.Stderr, "encoder time-base candidate report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
