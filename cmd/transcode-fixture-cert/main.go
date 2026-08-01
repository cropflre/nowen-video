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
		outputPath   = flag.String("output", "-", "JSON report path, or - for stdout")
		fixtureID    = flag.String("fixture", transcodecertification.DefaultFixtureID, "fixture ID to certify")
		allFixtures  = flag.Bool("all", false, "run the complete overlap-attribution fixture matrix")
		listFixtures = flag.Bool("list", false, "list supported fixture IDs")
		workDir      = flag.String("work-dir", "", "fixture workspace; temporary by default")
		keepWork     = flag.Bool("keep-work-dir", false, "keep an automatically created fixture workspace")
		ffmpegPath   = flag.String("ffmpeg", "", "ffmpeg executable; resolved from PATH by default")
		ffprobePath  = flag.String("ffprobe", "", "ffprobe executable; resolved from PATH by default")
		timeout      = flag.Duration("timeout", 10*time.Minute, "maximum certification runtime")
	)
	flag.Parse()

	if *listFixtures {
		for _, spec := range transcodecertification.AvailableFixtures() {
			fmt.Printf("%s\t%s\n", spec.ID, spec.Description)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	config := transcodecertification.Config{
		FFmpegPath:  *ffmpegPath,
		FFprobePath: *ffprobePath,
		WorkDir:     *workDir,
		KeepWorkDir: *keepWork,
		FixtureID:   *fixtureID,
	}

	var content []byte
	if *allFixtures {
		matrix, err := transcodecertification.RunMatrix(ctx, config)
		if err != nil {
			fatalf("fixture matrix certification failed: %v", err)
		}
		content, err = transcodecertification.MarshalMatrixReport(matrix)
		if err != nil {
			fatalf("encode fixture matrix: %v", err)
		}
		for _, report := range matrix.Reports {
			fmt.Fprintf(
				os.Stderr,
				"fixture=%s status=%s video_pts_delta_us=%d audio_pts_delta_us=%d discontinuity_required=%t\n",
				report.FixtureID,
				report.Handoff.Status,
				report.Handoff.VideoPresentationDeltaMicros,
				report.Handoff.AudioPresentationDeltaMicros,
				report.Handoff.DiscontinuityRequired,
			)
		}
		for _, comparison := range matrix.Comparisons {
			fmt.Fprintf(
				os.Stderr,
				"comparison=%s video_pts_change_us=%d audio_pts_change_us=%d\n",
				comparison.Name,
				comparison.VideoPresentationDeltaChangeMicros,
				comparison.AudioPresentationDeltaChangeMicros,
			)
		}
	} else {
		report, err := transcodecertification.Run(ctx, config)
		if err != nil {
			fatalf("fixture certification failed: %v", err)
		}
		content, err = transcodecertification.MarshalCertifiedReport(report)
		if err != nil {
			fatalf("encode fixture report: %v", err)
		}
		fmt.Fprintf(
			os.Stderr,
			"fixture=%s status=%s video_pts_delta_us=%d audio_pts_delta_us=%d discontinuity_required=%t\n",
			report.FixtureID,
			report.Handoff.Status,
			report.Handoff.VideoPresentationDeltaMicros,
			report.Handoff.AudioPresentationDeltaMicros,
			report.Handoff.DiscontinuityRequired,
		)
	}

	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write certification report: %v", err)
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
	fmt.Fprintf(os.Stderr, "fixture report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
