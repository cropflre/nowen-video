package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
		timeout      = flag.Duration("timeout", 90*time.Minute, "maximum shard certification runtime")
		list         = flag.Bool("list", false, "list duration tiers and shard identifiers")
		shardID      = flag.String("shard", "", "execute one immutable scaling shard")
		aggregateDir = flag.String("aggregate-dir", "", "aggregate verified shard JSON files from this directory")
	)
	flag.Parse()

	if *list {
		listScalingMatrix()
		return
	}
	if (*shardID == "") == (*aggregateDir == "") {
		fatalf("exactly one of -shard or -aggregate-dir is required")
	}
	if *aggregateDir != "" {
		reports, err := loadShardReports(*aggregateDir)
		if err != nil {
			fatalf("load scaling shard reports: %v", err)
		}
		report, err := transcodecertification.AggregateLongDurationScalingShardReports(reports)
		if err != nil {
			fatalf("aggregate scaling shard reports: %v", err)
		}
		content, err := transcodecertification.MarshalLongDurationScalingAggregateReport(report)
		if err != nil {
			fatalf("encode scaling aggregate report: %v", err)
		}
		for _, comparison := range report.Evidence.Comparisons {
			fmt.Fprintf(os.Stderr,
				"tier=%s profile=%s candidates_equivalent=%t max_checkpoint_difference_us=%d\n",
				comparison.TierID,
				comparison.ProfileID,
				comparison.Comparison.Equivalent,
				comparison.Comparison.MaximumCheckpointDifferenceMicros,
			)
		}
		fmt.Fprintf(os.Stderr, "scaling_shards=%d aggregate_contract=%s\n", len(report.Evidence.Shards), report.ContractHash)
		if err := writeOutput(*outputPath, content); err != nil {
			fatalf("write scaling aggregate report: %v", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := transcodecertification.RunLongDurationScalingShard(ctx, transcodecertification.LongDurationDriftConfig{
		Config: transcodecertification.Config{
			FFmpegPath:  *ffmpegPath,
			FFprobePath: *ffprobePath,
			WorkDir:     *workDir,
			KeepWorkDir: *keepWork,
		},
		CorpusRoot:   *corpusRoot,
		ManifestPath: *manifestPath,
	}, *shardID)
	if err != nil {
		fatalf("long-duration scaling shard failed: %v", err)
	}
	content, err := transcodecertification.MarshalLongDurationScalingShardReport(report)
	if err != nil {
		fatalf("encode scaling shard report: %v", err)
	}
	summary := report.Evidence.Candidate.Summary
	fmt.Fprintf(os.Stderr,
		"shard=%s duration_minutes=%d checkpoints=%d segments=%d max_video_end_error_us=%d max_audio_end_error_us=%d max_av_skew_us=%d max_checkpoint_error_us=%d stable=%t contract=%s\n",
		report.Evidence.Shard.ID,
		report.Evidence.Tier.DurationMicros/60_000_000,
		len(report.Evidence.Candidate.Runs[0].Video.Checkpoints),
		report.Evidence.Candidate.Runs[0].SegmentCount,
		summary.MaximumAbsoluteVideoEndErrorMicros,
		summary.MaximumAbsoluteAudioEndErrorMicros,
		summary.MaximumAbsoluteAVSkewMicros,
		summary.MaximumAbsoluteCheckpointErrorMicros,
		summary.Stable,
		report.ContractHash,
	)
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write scaling shard report: %v", err)
	}
}

func listScalingMatrix() {
	for _, tier := range transcodelongdrift.AvailableScalingTiers() {
		fmt.Printf(
			"tier=%s duration_us=%d checkpoint_interval_us=%d repeats=%d profiles=%s candidates=%s\n",
			tier.ID,
			tier.DurationMicros,
			tier.CheckpointIntervalMicros,
			tier.RepeatCount,
			strings.Join(tier.ProfileIDs, ","),
			strings.Join(tier.CandidateIDs, ","),
		)
	}
	for _, shard := range transcodelongdrift.AvailableScalingShards() {
		fmt.Printf("shard=%s tier=%s profile=%s candidate=%s\n", shard.ID, shard.TierID, shard.ProfileID, shard.CandidateID)
	}
}

func loadShardReports(root string) ([]transcodecertification.LongDurationScalingShardReport, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	var paths []string
	if err := filepath.WalkDir(absolute, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || strings.ToLower(filepath.Ext(entry.Name())) != ".json" {
			return nil
		}
		paths = append(paths, path)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Strings(paths)
	reports := make([]transcodecertification.LongDurationScalingShardReport, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var report transcodecertification.LongDurationScalingShardReport
		if err := json.Unmarshal(content, &report); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		if err := report.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s: %w", path, err)
		}
		reports = append(reports, report)
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("no shard JSON files found under %s", absolute)
	}
	return reports, nil
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
	fmt.Fprintf(os.Stderr, "long-duration scaling report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
