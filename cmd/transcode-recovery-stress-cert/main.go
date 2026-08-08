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
	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
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
		timeout      = flag.Duration("timeout", 25*time.Minute, "maximum scenario certification runtime")
		list         = flag.Bool("list", false, "list immutable recovery/resource scenarios")
		scenarioID   = flag.String("scenario", "", "execute one immutable recovery/resource scenario")
		aggregateDir = flag.String("aggregate-dir", "", "aggregate verified scenario JSON files from this directory")
	)
	flag.Parse()

	if *list {
		for _, scenario := range transcoderecovery.AvailableScenarios() {
			fmt.Printf(
				"scenario=%s fault=%s duration_us=%d trigger_us=%d processes=%d final_job=%s final_artifact=%s cpu=%d address_space_bytes=%d enospc_after_bytes=%d\n",
				scenario.ID,
				scenario.FaultKind,
				scenario.LogicalDurationMicros,
				scenario.TriggerMicros,
				scenario.ExpectedProcessCount,
				scenario.ExpectedFinalJobStatus,
				scenario.ExpectedFinalArtifactStatus,
				scenario.Limits.CPUCount,
				scenario.Limits.AddressSpaceBytes,
				scenario.Limits.ENOSPCAfterBytes,
			)
		}
		return
	}
	if (*scenarioID == "") == (*aggregateDir == "") {
		fatalf("exactly one of -scenario or -aggregate-dir is required")
	}
	if *aggregateDir != "" {
		reports, err := loadScenarioReports(*aggregateDir)
		if err != nil {
			fatalf("load recovery stress reports: %v", err)
		}
		report, err := transcodecertification.AggregateRecoveryStressScenarioReports(reports)
		if err != nil {
			fatalf("aggregate recovery stress reports: %v", err)
		}
		content, err := transcodecertification.MarshalRecoveryStressAggregateReport(report)
		if err != nil {
			fatalf("encode recovery stress aggregate report: %v", err)
		}
		fmt.Fprintf(os.Stderr,
			"recovery_scenarios=%d processes=%d segments=%d max_rss_bytes=%d aggregate_contract=%s\n",
			len(report.Evidence.Scenarios),
			report.Evidence.TotalProcesses,
			report.Evidence.TotalSegmentsObserved,
			report.Evidence.MaximumRSSBytes,
			report.ContractHash,
		)
		if err := writeOutput(*outputPath, content); err != nil {
			fatalf("write recovery stress aggregate report: %v", err)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	report, err := transcodecertification.RunRecoveryStressScenario(ctx, transcodecertification.RecoveryStressConfig{
		Config: transcodecertification.Config{
			FFmpegPath:  *ffmpegPath,
			FFprobePath: *ffprobePath,
			WorkDir:     *workDir,
			KeepWorkDir: *keepWork,
		},
		CorpusRoot:   *corpusRoot,
		ManifestPath: *manifestPath,
	}, *scenarioID)
	if err != nil {
		fatalf("recovery stress scenario failed: %v", err)
	}
	content, err := transcodecertification.MarshalRecoveryStressScenarioReport(report)
	if err != nil {
		fatalf("encode recovery stress scenario report: %v", err)
	}
	segments := 0
	maximumRSS := int64(0)
	for _, process := range report.Evidence.Processes {
		segments += process.SegmentCount
		if process.MaxRSSBytes > maximumRSS {
			maximumRSS = process.MaxRSSBytes
		}
	}
	fmt.Fprintf(os.Stderr,
		"scenario=%s processes=%d segments=%d max_rss_bytes=%d final_job=%s final_artifact=%s readable_artifact=%s contract=%s\n",
		report.Evidence.Scenario.ID,
		len(report.Evidence.Processes),
		segments,
		maximumRSS,
		report.Evidence.Artifact.FinalJobStatus,
		report.Evidence.Artifact.FinalArtifactStatus,
		report.Evidence.Artifact.ReadableArtifactID,
		report.ContractHash,
	)
	if err := writeOutput(*outputPath, content); err != nil {
		fatalf("write recovery stress scenario report: %v", err)
	}
}

func loadScenarioReports(root string) ([]transcodecertification.RecoveryStressScenarioReport, error) {
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
	reports := make([]transcodecertification.RecoveryStressScenarioReport, 0, len(paths))
	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var report transcodecertification.RecoveryStressScenarioReport
		if err := json.Unmarshal(content, &report); err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		if err := report.Validate(); err != nil {
			return nil, fmt.Errorf("validate %s: %w", path, err)
		}
		reports = append(reports, report)
	}
	if len(reports) == 0 {
		return nil, fmt.Errorf("no scenario JSON files found under %s", absolute)
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
	fmt.Fprintf(os.Stderr, "recovery stress report: %s\n", absolute)
	return nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
