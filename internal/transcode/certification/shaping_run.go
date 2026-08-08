package certification

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	timestampexecution "github.com/nowen-video/nowen-video/internal/transcode/timestampexecution"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func RunShapingMatrix(ctx context.Context, config Config) (ShapingMatrixReport, error) {
	reports := make([]ShapingCaseReport, 0, len(shapingCaseSpecs))
	for _, spec := range shapingCaseSpecs {
		caseConfig := config
		caseConfig.FixtureID = spec.FixtureID
		if config.WorkDir != "" {
			caseConfig.WorkDir = filepath.Join(config.WorkDir, spec.ID)
		}
		report, err := RunShapingCase(ctx, caseConfig, spec.ID)
		if err != nil {
			return ShapingMatrixReport{}, fmt.Errorf("run shaping case %s: %w", spec.ID, err)
		}
		reports = append(reports, report)
	}
	return BuildShapingMatrixReport(reports)
}

func RunShapingCase(ctx context.Context, config Config, caseID string) (ShapingCaseReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	caseSpec, ok := LookupShapingCase(caseID)
	if !ok {
		return ShapingCaseReport{}, fmt.Errorf("unknown shaping case %q", caseID)
	}
	if err := caseSpec.Validate(); err != nil {
		return ShapingCaseReport{}, err
	}
	fixture, ok := LookupFixture(caseSpec.FixtureID)
	if !ok {
		return ShapingCaseReport{}, fmt.Errorf("unknown fixture %q", caseSpec.FixtureID)
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return ShapingCaseReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return ShapingCaseReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return ShapingCaseReport{}, err
	}
	defer cleanup()

	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return ShapingCaseReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return ShapingCaseReport{}, err
	}
	encodingPlan := fixtureEncodingPlan(fixture)
	encodingVersion, encodingHash, encodingJSON, err := transcodeencoding.Identity(encodingPlan)
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("encoding plan identity: %w", err)
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("timestamp plan identity: %w", err)
	}
	executionPlan, err := timestampexecution.New(caseSpec.VideoPTSShiftMicros, caseSpec.AudioPTSShiftMicros)
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("timestamp execution plan: %w", err)
	}
	executionVersion, executionHash, executionJSON, err := timestampexecution.Identity(executionPlan)
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("timestamp execution identity: %w", err)
	}

	sourcePath := filepath.Join(workDir, "source.mp4")
	if err := runCommand(ctx, ffmpegPath, sourceArgs(sourcePath, fixture)...); err != nil {
		return ShapingCaseReport{}, fmt.Errorf("generate shaping source fixture: %w", err)
	}
	startupManifest, err := produceBoundaryHLS(
		ctx,
		ffmpegPath,
		workDir,
		"startup",
		sourcePath,
		timestampPlan,
		fixture,
		0,
		caseSpec.ExpectedBoundaryMicros,
	)
	if err != nil {
		return ShapingCaseReport{}, err
	}
	continuationManifest, err := produceShapedContinuationHLS(
		ctx,
		ffmpegPath,
		workDir,
		"continuation",
		sourcePath,
		timestampPlan,
		fixture,
		caseSpec.ExpectedBoundaryMicros,
		executionPlan,
	)
	if err != nil {
		return ShapingCaseReport{}, err
	}

	verifier := transcodeattestation.Verifier{FFprobePath: ffprobePath}
	startup, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        startupManifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("verify shaping startup fixture: %w", err)
	}
	continuation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuationManifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("verify shaping continuation fixture: %w", err)
	}
	boundaryMS := int64(math.Round(float64(caseSpec.ExpectedBoundaryMicros) / 1000))
	if err := timestampPlan.VerifyObservedStart(0, startup.First.Timeline.Video.StartMS, startup.First.Timeline.Audio.StartMS); err != nil {
		return ShapingCaseReport{}, fmt.Errorf("shaping startup timestamp certification: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(boundaryMS, continuation.First.Timeline.Video.StartMS, continuation.First.Timeline.Audio.StartMS); err != nil {
		return ShapingCaseReport{}, fmt.Errorf("shaping continuation timestamp certification: %w", err)
	}
	startupVersion, startupHash, _, err := transcodeattestation.Identity(startup)
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("shaping startup attestation identity: %w", err)
	}
	continuationVersion, continuationHash, _, err := transcodeattestation.Identity(continuation)
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("shaping continuation attestation identity: %w", err)
	}

	boundaryCase := BoundaryCaseSpec{
		ID:                      caseSpec.ID,
		Description:             caseSpec.Description,
		FixtureID:               caseSpec.FixtureID,
		OffsetKind:              "timestamp_execution_candidate",
		ReferenceBoundaryMicros: caseSpec.ExpectedBoundaryMicros,
		ExpectedBoundaryMicros:  caseSpec.ExpectedBoundaryMicros,
	}
	evidence, err := probeBoundaryContract(ctx, ffprobePath, boundaryContractRequest{
		Case:                           boundaryCase,
		Fixture:                        fixture,
		StartupManifest:                startupManifest,
		ContinuationManifest:           continuationManifest,
		FFmpegVersion:                  ffmpegVersion,
		FFprobeVersion:                 ffprobeVersion,
		EncodingPlanVersion:            encodingVersion,
		EncodingPlanHash:               encodingHash,
		TimestampPlanVersion:           timestampVersion,
		TimestampPlanHash:              timestampHash,
		StartupAttestationVersion:      startupVersion,
		StartupAttestationHash:         startupHash,
		ContinuationAttestationVersion: continuationVersion,
		ContinuationAttestationHash:    continuationHash,
	})
	if err != nil {
		return ShapingCaseReport{}, err
	}
	evidenceVersion, evidenceHash, _, err := transcodeboundary.Identity(evidence)
	if err != nil {
		return ShapingCaseReport{}, fmt.Errorf("shaping boundary evidence identity: %w", err)
	}
	report := ShapingCaseReport{
		Case:            caseSpec,
		PlanVersion:     executionVersion,
		PlanHash:        executionHash,
		PlanJSON:        executionJSON,
		EvidenceVersion: evidenceVersion,
		EvidenceHash:    evidenceHash,
		Evidence:        evidence,
	}
	if err := report.Validate(); err != nil {
		return ShapingCaseReport{}, err
	}
	return report, nil
}
