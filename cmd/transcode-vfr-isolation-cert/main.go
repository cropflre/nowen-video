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
		timeout     = flag.Duration("timeout", 60*time.Minute, "maximum certification runtime")
		list        = flag.Bool("list", false, "list registered VFR isolation variants")
	)
	flag.Parse()

	if *list {
		for _, spec := range transcodecertification.AvailableVFRIsolationVariants() {
			fmt.Printf("%s\t%s\tcontainer=%s fps_mode=%s enc_tb=%s copy=%t\n",
				spec.ID, spec.Description, spec.Container, spec.FPSMode, spec.EncoderTimeBase, spec.CopyOnly)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := transcodecertification.RunVFRIsolationMatrix(ctx, transcodecertification.Config{
		FFmpegPath:  *ffmpegPath,
		FFprobePath: *ffprobePath,
		WorkDir:     *workDir,
		KeepWorkDir: *keepWork,
	})
	if err != nil {
		fatalf("VFR isolation certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalVFRIsolationMatrixReport(report)
	if err != nil {
		fatalf("encode VFR isolation matrix: %v", err)
	}
	for _, variant := range report.Evidence.Variants {
		fmt.Fprintf(os.Stderr,
			"variant=%s layer=%s container=%s fps_mode=%s enc_tb=%s frames=%d delta=%d dominant_us=%d near_zero=%d adjacent_duplicates=%d unique_frames=%d cadence=%s sequence_ref=%s sequence_match=%t\n",
			variant.Spec.ID,
			variant.Spec.Layer,
			variant.Spec.Container,
			variant.Spec.FPSMode,
			variant.Spec.EncoderTimeBase,
			variant.Timeline.FrameCount,
			variant.Mapping.FrameCountDelta,
			variant.Timeline.DominantDeltaMicros,
			variant.Timeline.NearZeroDeltaCount,
			variant.Fingerprint.AdjacentDuplicateCount,
			variant.Fingerprint.UniqueFrameCount,
			variant.CadenceClassification,
			variant.SequenceReference,
			variant.SequenceMatchesReference,
		)
	}
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write VFR isolation report: %v", err)
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
	fmt.Fprintf(os.Stderr, "VFR isolation report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
