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
		workDir     = flag.String("work-dir", "", "fixture workspace; temporary by default")
		keepWork    = flag.Bool("keep-work-dir", false, "keep an automatically created fixture workspace")
		ffmpegPath  = flag.String("ffmpeg", "", "ffmpeg executable; resolved from PATH by default")
		ffprobePath = flag.String("ffprobe", "", "ffprobe executable; resolved from PATH by default")
		timeout     = flag.Duration("timeout", 5*time.Minute, "maximum certification runtime")
	)
	flag.Parse()

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := transcodecertification.Run(ctx, transcodecertification.Config{
		FFmpegPath:  *ffmpegPath,
		FFprobePath: *ffprobePath,
		WorkDir:     *workDir,
		KeepWorkDir: *keepWork,
	})
	if err != nil {
		fatalf("fixture certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalReport(report)
	if err != nil {
		fatalf("encode fixture report: %v", err)
	}
	if *outputPath == "-" {
		if _, err := os.Stdout.Write(content); err != nil {
			fatalf("write fixture report: %v", err)
		}
	} else {
		absolute, err := filepath.Abs(*outputPath)
		if err != nil {
			fatalf("resolve output path: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
			fatalf("create output directory: %v", err)
		}
		if err := os.WriteFile(absolute, content, 0o644); err != nil {
			fatalf("write fixture report: %v", err)
		}
		fmt.Fprintf(os.Stderr, "fixture report: %s\n", absolute)
	}
	fmt.Fprintf(
		os.Stderr,
		"fixture=%s status=%s discontinuity_required=%t\n",
		report.FixtureID,
		report.Handoff.Status,
		report.Handoff.DiscontinuityRequired,
	)
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
