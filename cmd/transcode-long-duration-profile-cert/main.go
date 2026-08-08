package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	transcodecertification "github.com/nowen-video/nowen-video/internal/transcode/certification"
	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
)

func main() {
	var (
		corpusRoot   = flag.String("corpus-root", "", "real-media corpus root containing assets and manifest")
		manifestPath = flag.String("manifest", "", "manifest path; defaults to <corpus-root>/real-media-corpus-manifest-v1.json")
		outputPath   = flag.String("output", "-", "JSON report path, or - for stdout")
		workDir      = flag.String("work-dir", "", "certification workspace; temporary by default")
		keepWork     = flag.Bool("keep-work-dir", false, "keep an automatically created workspace")
		ffmpegPath   = flag.String("ffmpeg", "", "ffmpeg executable; resolved from PATH by default")
		ffprobePath  = flag.String("ffprobe", "", "ffprobe executable; resolved from PATH by default")
		timeout      = flag.Duration("timeout", 4*time.Hour, "maximum certification runtime")
		list         = flag.Bool("list", false, "list long-duration profiles, checkpoints, and candidates")
	)
	flag.Parse()

	if *list {
		fmt.Printf("profiles=%d duration_us=%d checkpoint_interval_us=%d repeats=%d\n",
			len(transcodelongdrift.AvailableProfiles()),
			transcodelongdrift.DurationMicros,
			transcodelongdrift.CheckpointMicros,
			transcodelongdrift.RepeatCount,
		)
		for _, profile := range transcodelongdrift.AvailableProfiles() {
			fmt.Printf("profile=%s source_case=%s container=%s frame_rate_mode=%s audio_codec=%s audio_sample_rate=%d source_origin_us=%d edit_list=%t\n",
				profile.ID,
				profile.SourceCaseID,
				profile.Container,
				profile.FrameRateMode,
				profile.AudioCodec,
				profile.AudioSampleRate,
				profile.SourceOriginMicros,
				profile.HasEditList,
			)
		}
		for _, target := range transcodelongdrift.CheckpointTargets() {
			fmt.Printf("checkpoint_us=%d\n", target)
		}
		for _, candidate := range transcodecertification.AvailableEncoderTimeBaseCandidates() {
			fmt.Printf("candidate=%s enc_tb=%s\n", candidate.ID, candidate.EncoderTimeBase)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := transcodecertification.RunLongDurationProfileMatrix(ctx, transcodecertification.LongDurationDriftConfig{
		Config: transcodecertification.Config{
			FFmpegPath:  *ffmpegPath,
			FFprobePath: *ffprobePath,
			WorkDir:     *workDir,
			KeepWorkDir: *keepWork,
		},
		CorpusRoot:   *corpusRoot,
		ManifestPath: *manifestPath,
	})
	if err != nil {
		fatalf("long-duration profile certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalLongDurationProfileMatrixReport(report)
	if err != nil {
		fatalf("encode long-duration profile report: %v", err)
	}
	for _, profile := range report.Evidence.Profiles {
		for _, candidate := range profile.Candidates {
			fmt.Fprintf(os.Stderr,
				"profile=%s candidate=%s repeats=%d max_video_end_error_us=%d max_audio_end_error_us=%d max_av_skew_us=%d max_checkpoint_error_us=%d stable=%t\n",
				profile.Profile.ID,
				candidate.ID,
				candidate.Summary.RepeatCount,
				candidate.Summary.MaximumAbsoluteVideoEndErrorMicros,
				candidate.Summary.MaximumAbsoluteAudioEndErrorMicros,
				candidate.Summary.MaximumAbsoluteAVSkewMicros,
				candidate.Summary.MaximumAbsoluteCheckpointErrorMicros,
				candidate.Summary.Stable,
			)
		}
		fmt.Fprintf(os.Stderr,
			"profile=%s candidates_equivalent=%t max_checkpoint_difference_us=%d\n",
			profile.Profile.ID,
			profile.Comparison.Equivalent,
			profile.Comparison.MaximumCheckpointDifferenceMicros,
		)
	}
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write long-duration profile report: %v", err)
	}
}

func writeOutput(outputPath string, content []byte) error {
	if outputPath == "-" {
		_, err := os.Stdout.Write(content)
		return err
	}
	absolute, err := filepath.Abs(outputPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(absolute), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(absolute, content, 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "long-duration profile report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
