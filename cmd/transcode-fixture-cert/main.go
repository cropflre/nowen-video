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
		outputPath           = flag.String("output", "-", "JSON report path, or - for stdout")
		fixtureID            = flag.String("fixture", transcodecertification.DefaultFixtureID, "fixture ID to certify")
		allFixtures          = flag.Bool("all", false, "run the complete overlap-attribution fixture matrix")
		boundaryMatrix       = flag.Bool("boundary-matrix", false, "run the packet-level boundary placement matrix")
		shapingMatrix        = flag.Bool("shaping-matrix", false, "run timestamp execution v2 shaping candidates")
		avSyncVarianceMatrix = flag.Bool("av-sync-variance-matrix", false, "run repeated A/V boundary sync variance certification")
		listFixtures         = flag.Bool("list", false, "list supported fixture, boundary, shaping, and A/V sync case IDs")
		workDir              = flag.String("work-dir", "", "fixture workspace; temporary by default")
		keepWork             = flag.Bool("keep-work-dir", false, "keep an automatically created fixture workspace")
		ffmpegPath           = flag.String("ffmpeg", "", "ffmpeg executable; resolved from PATH by default")
		ffprobePath          = flag.String("ffprobe", "", "ffprobe executable; resolved from PATH by default")
		timeout              = flag.Duration("timeout", 15*time.Minute, "maximum certification runtime")
	)
	flag.Parse()

	if *listFixtures {
		for _, spec := range transcodecertification.AvailableFixtures() {
			fmt.Printf("fixture\t%s\t%s\n", spec.ID, spec.Description)
		}
		for _, spec := range transcodecertification.AvailableBoundaryCases() {
			fmt.Printf("boundary\t%s\t%s\n", spec.ID, spec.Description)
		}
		for _, spec := range transcodecertification.AvailableShapingCases() {
			fmt.Printf("shaping\t%s\t%s\n", spec.ID, spec.Description)
		}
		for _, spec := range transcodecertification.AvailableAVSyncVarianceCases() {
			fmt.Printf("av-sync-variance\t%s\t%s\n", spec.ID, spec.Description)
		}
		return
	}
	selectedMatrices := 0
	for _, selected := range []bool{*allFixtures, *boundaryMatrix, *shapingMatrix, *avSyncVarianceMatrix} {
		if selected {
			selectedMatrices++
		}
	}
	if selectedMatrices > 1 {
		fatalf("-all, -boundary-matrix, -shaping-matrix, and -av-sync-variance-matrix are mutually exclusive")
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
	switch {
	case *avSyncVarianceMatrix:
		matrix, err := transcodecertification.RunAVSyncVarianceMatrix(ctx, config)
		if err != nil {
			fatalf("A/V sync variance certification failed: %v", err)
		}
		content, err = transcodecertification.MarshalAVSyncVarianceMatrixReport(matrix)
		if err != nil {
			fatalf("encode A/V sync variance matrix: %v", err)
		}
		for _, report := range matrix.Cases {
			fmt.Fprintf(
				os.Stderr,
				"case=%s repeats=%d stable=%t max_span_us=%d startup_skew_us=%d continuation_skew_us=%d boundary_delta_skew_us=%d discontinuity_required=%t\n",
				report.Case.ID,
				report.Summary.RepeatCount,
				report.Summary.Stable,
				report.Summary.MaxObservedSpanMicros,
				report.Summary.StartupEndSkewMicros.MaxMicros,
				report.Summary.ContinuationStartSkewMicros.MaxMicros,
				report.Summary.BoundaryDeltaSkewMicros.MaxMicros,
				report.Runs[0].AVSync.DiscontinuityRequired,
			)
		}
		for _, comparison := range matrix.Comparisons {
			fmt.Fprintf(
				os.Stderr,
				"comparison=%s baseline_abs_delta_skew_us=%d candidate_abs_delta_skew_us=%d improvement_us=%d\n",
				comparison.Name,
				comparison.BaselineMaxAbsDeltaSkewMicros,
				comparison.CandidateMaxAbsDeltaSkewMicros,
				comparison.DeltaSkewImprovementMicros,
			)
		}
	case *shapingMatrix:
		matrix, err := transcodecertification.RunShapingMatrix(ctx, config)
		if err != nil {
			fatalf("shaping matrix certification failed: %v", err)
		}
		content, err = transcodecertification.MarshalShapingMatrixReport(matrix)
		if err != nil {
			fatalf("encode shaping matrix: %v", err)
		}
		for _, report := range matrix.Cases {
			audioDelay := report.Evidence.Audio.AudioDelay
			var audioDeltaSamples int64
			if audioDelay != nil {
				audioDeltaSamples = audioDelay.BoundaryDeltaSamples
			}
			fmt.Fprintf(
				os.Stderr,
				"case=%s video_shift_us=%d audio_shift_us=%d video_status=%s video_delta_us=%d audio_status=%s audio_delta_us=%d audio_delta_samples=%d discontinuity_required=%t\n",
				report.Case.ID,
				report.Case.VideoPTSShiftMicros,
				report.Case.AudioPTSShiftMicros,
				report.Evidence.Video.Status,
				report.Evidence.Video.PresentationDeltaMicros,
				report.Evidence.Audio.Status,
				report.Evidence.Audio.PresentationDeltaMicros,
				audioDeltaSamples,
				report.Evidence.DiscontinuityRequired,
			)
		}
	case *boundaryMatrix:
		matrix, err := transcodecertification.RunBoundaryMatrix(ctx, config)
		if err != nil {
			fatalf("boundary matrix certification failed: %v", err)
		}
		content, err = transcodecertification.MarshalBoundaryMatrixReport(matrix)
		if err != nil {
			fatalf("encode boundary matrix: %v", err)
		}
		for _, report := range matrix.Cases {
			audioDelay := report.Evidence.Audio.AudioDelay
			var audioDeltaSamples int64
			var sideDataObserved bool
			if audioDelay != nil {
				audioDeltaSamples = audioDelay.BoundaryDeltaSamples
				sideDataObserved = audioDelay.SideDataObserved
			}
			fmt.Fprintf(
				os.Stderr,
				"case=%s boundary_us=%d video_status=%s video_delta_us=%d audio_status=%s audio_delta_us=%d audio_delta_samples=%d side_data_observed=%t discontinuity_required=%t\n",
				report.Case.ID,
				report.Case.ExpectedBoundaryMicros,
				report.Evidence.Video.Status,
				report.Evidence.Video.PresentationDeltaMicros,
				report.Evidence.Audio.Status,
				report.Evidence.Audio.PresentationDeltaMicros,
				audioDeltaSamples,
				sideDataObserved,
				report.Evidence.DiscontinuityRequired,
			)
		}
	case *allFixtures:
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
	default:
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
