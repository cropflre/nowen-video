package certification

import (
	"context"
	"fmt"
	"path/filepath"

	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func RunOutputCadenceMatrix(ctx context.Context, config Config) (OutputCadenceMatrixReport, error) {
	reports := make([]OutputCadenceCaseReport, 0, len(sourceOriginCaseSpecs))
	for _, spec := range sourceOriginCaseSpecs {
		caseConfig := config
		caseConfig.FixtureID = spec.FixtureID
		if config.WorkDir != "" {
			caseConfig.WorkDir = filepath.Join(config.WorkDir, "output-cadence", spec.ID)
		}
		report, err := RunOutputCadenceCase(ctx, caseConfig, spec.ID)
		if err != nil {
			return OutputCadenceMatrixReport{}, fmt.Errorf("run output cadence case %s: %w", spec.ID, err)
		}
		reports = append(reports, report)
	}
	return BuildOutputCadenceMatrixReport(reports)
}

func RunOutputCadenceCase(ctx context.Context, config Config, caseID string) (OutputCadenceCaseReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	spec, ok := LookupSourceOriginCase(caseID)
	if !ok {
		return OutputCadenceCaseReport{}, fmt.Errorf("unknown output cadence case %q", caseID)
	}
	if err := spec.Validate(); err != nil {
		return OutputCadenceCaseReport{}, err
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}
	defer cleanup()

	sourceConfig := config
	sourceConfig.WorkDir = filepath.Join(workDir, "source-origin-evidence")
	sourceConfig.KeepWorkDir = true
	sourceOrigin, err := RunSourceOriginCase(ctx, sourceConfig, spec.ID)
	if err != nil {
		return OutputCadenceCaseReport{}, fmt.Errorf("build bound source-origin evidence: %w", err)
	}

	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}
	if timestampVersion != sourceOrigin.Evidence.TimestampPlanVersion || timestampHash != sourceOrigin.Evidence.TimestampPlanHash {
		return OutputCadenceCaseReport{}, fmt.Errorf("output cadence timestamp plan identity drifted from source-origin evidence")
	}

	sourceGraph := sourceOriginInputGraph(spec)
	cadenceWorkDir := filepath.Join(workDir, "produced-output")
	startupManifest, err := produceSourceOriginHLS(ctx, ffmpegPath, cadenceWorkDir, "startup", sourceGraph, timestampPlan, spec, 0, spec.ExpectedBoundaryMicros)
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}
	continuationManifest, err := produceSourceOriginHLS(ctx, ffmpegPath, cadenceWorkDir, "continuation", sourceGraph, timestampPlan, spec, spec.ExpectedBoundaryMicros, 0)
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}

	evidence, err := probeOutputCadenceContract(
		ctx,
		ffprobePath,
		sourceGraph,
		startupManifest,
		continuationManifest,
		spec,
		sourceOrigin.Evidence.FFmpegVersion,
		sourceOrigin.Evidence.FFprobeVersion,
		sourceOrigin.ContractVersion,
		sourceOrigin.ContractHash,
		timestampVersion,
		timestampHash,
		sourceOrigin.BoundaryVersion,
		sourceOrigin.BoundaryHash,
		sourceOrigin.AVSyncVersion,
		sourceOrigin.AVSyncHash,
	)
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}
	contractVersion, contractHash, _, err := transcodeoutputcadence.Identity(evidence)
	if err != nil {
		return OutputCadenceCaseReport{}, err
	}
	report := OutputCadenceCaseReport{
		Case:            spec,
		SourceOrigin:    sourceOrigin,
		ContractVersion: contractVersion,
		ContractHash:    contractHash,
		Evidence:        evidence,
	}
	if err := report.Validate(); err != nil {
		return OutputCadenceCaseReport{}, err
	}
	return report, nil
}
