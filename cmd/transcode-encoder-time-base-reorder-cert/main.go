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
		list        = flag.Bool("list", false, "list registered reorder cases and candidates")
	)
	flag.Parse()

	if *list {
		fmt.Println("cases:")
		for _, spec := range transcodecertification.AvailableEncoderTimeBaseReorderCases() {
			base := spec.Base
			fmt.Printf("%s\t%s\tmode=%s primary=%d/%d secondary=%d/%d offset_us=%d gop=%d bframes=%d refs=%d\n",
				base.ID, base.Description, base.SourceMode,
				base.PrimaryFrameRateNumerator, base.PrimaryFrameRateDenominator,
				base.SecondaryFrameRateNumerator, base.SecondaryFrameRateDenominator,
				base.SourceOffsetMicros, base.GOPSize, spec.BFrames, spec.ReferenceFrames,
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
	report, err := transcodecertification.RunEncoderTimeBaseReorderMatrix(ctx, transcodecertification.Config{
		FFmpegPath: *ffmpegPath,
		FFprobePath: *ffprobePath,
		WorkDir: *workDir,
		KeepWorkDir: *keepWork,
	})
	if err != nil {
		fatalf("encoder time-base reorder certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalEncoderTimeBaseReorderMatrixReport(report)
	if err != nil {
		fatalf("encode encoder time-base reorder matrix: %v", err)
	}
	for _, caseEvidence := range report.Evidence.Cases {
		for _, candidate := range caseEvidence.Candidates {
			fmt.Fprintf(os.Stderr,
				"case=%s candidate=%s bframes=%d repeats=%d startup_reordered=%d continuation_reordered=%d startup_depth=%d continuation_depth=%d startup_max_cts_us=%d continuation_max_cts_us=%d strict_dts=%t stable=%t\n",
				caseEvidence.Case.Base.ID,
				candidate.Spec.ID,
				caseEvidence.Case.BFrames,
				len(candidate.Runs),
				candidate.Summary.StartupReorderedPacketCount.Min,
				candidate.Summary.ContinuationReorderedPacketCount.Min,
				candidate.Summary.StartupMaxReorderDepth.Min,
				candidate.Summary.ContinuationMaxReorderDepth.Min,
				candidate.Summary.StartupMaxCompositionOffsetMicros.Min,
				candidate.Summary.ContinuationMaxCompositionOffsetMicros.Min,
				candidate.Summary.StrictDTS,
				candidate.Summary.Stable,
			)
		}
		fmt.Fprintf(os.Stderr, "case=%s candidates_equivalent=%t\n", caseEvidence.Case.Base.ID, caseEvidence.Comparison.Equivalent)
	}
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write encoder time-base reorder report: %v", err)
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
	fmt.Fprintf(os.Stderr, "encoder time-base reorder report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
