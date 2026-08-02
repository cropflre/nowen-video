package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	transcodecertification "github.com/nowen-video/nowen-video/internal/transcode/certification"
	transcodecandidate "github.com/nowen-video/nowen-video/internal/transcode/realmediacandidate"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
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
		timeout      = flag.Duration("timeout", 3*time.Hour, "maximum certification runtime")
		list         = flag.Bool("list", false, "list registered corpus cases and candidates")
	)
	flag.Parse()

	if *list {
		for _, caseSpec := range transcodecorpus.DefaultSpec().Cases {
			fmt.Printf("%s\t%s\tcontainer=%s mode=%s audio=%s/%d boundary_us=%d\n",
				caseSpec.ID,
				caseSpec.Description,
				caseSpec.Source.Container,
				caseSpec.Source.Video.FrameRateMode,
				caseSpec.Source.Audio.Codec,
				caseSpec.Source.Audio.SampleRate,
				caseSpec.BoundaryMicros,
			)
		}
		fmt.Printf("repeat_count=%d\n", transcodecandidateRepeatCount())
		for _, candidate := range transcodecertification.AvailableEncoderTimeBaseCandidates() {
			fmt.Printf("%s\t%s\tenc_tb=%s\n", candidate.ID, candidate.Description, candidate.EncoderTimeBase)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := transcodecertification.RunRealMediaCandidateMatrix(ctx, transcodecertification.RealMediaCandidateConfig{
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
		fatalf("real-media candidate certification failed: %v", err)
	}
	content, err := transcodecertification.MarshalRealMediaCandidateMatrixReport(report)
	if err != nil {
		fatalf("encode real-media candidate report: %v", err)
	}
	for _, caseEvidence := range report.Evidence.Cases {
		for _, candidate := range caseEvidence.Evidence.Candidates {
			fmt.Fprintf(os.Stderr,
				"case=%s source_sha256=%s candidate=%s repeats=%d startup_depth=%d continuation_depth=%d stable=%t\n",
				caseEvidence.Source.CaseID,
				caseEvidence.Source.SHA256,
				candidate.Spec.ID,
				len(candidate.Runs),
				candidate.Summary.StartupMaxReorderDepth.Min,
				candidate.Summary.ContinuationMaxReorderDepth.Min,
				candidate.Summary.Stable,
			)
		}
		fmt.Fprintf(os.Stderr, "case=%s candidates_equivalent=%t\n", caseEvidence.Source.CaseID, caseEvidence.Evidence.Comparison.Equivalent)
	}
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write real-media candidate report: %v", err)
	}
}

func transcodecandidateRepeatCount() int {
	return 3
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
	fmt.Fprintf(os.Stderr, "real-media candidate report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
