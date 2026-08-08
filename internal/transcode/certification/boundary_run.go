package certification

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func RunBoundaryMatrix(ctx context.Context, config Config) (BoundaryMatrixReport, error) {
	reports := make([]BoundaryCaseReport, 0, len(boundaryCaseSpecs))
	for _, spec := range boundaryCaseSpecs {
		caseConfig := config
		caseConfig.FixtureID = spec.FixtureID
		if config.WorkDir != "" {
			caseConfig.WorkDir = filepath.Join(config.WorkDir, spec.ID)
		}
		report, err := RunBoundaryCase(ctx, caseConfig, spec.ID)
		if err != nil {
			return BoundaryMatrixReport{}, fmt.Errorf("run boundary case %s: %w", spec.ID, err)
		}
		reports = append(reports, report)
	}
	return BuildBoundaryMatrixReport(reports)
}

func RunBoundaryCase(ctx context.Context, config Config, caseID string) (BoundaryCaseReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	caseSpec, ok := LookupBoundaryCase(caseID)
	if !ok {
		return BoundaryCaseReport{}, fmt.Errorf("unknown boundary case %q", caseID)
	}
	if err := caseSpec.Validate(); err != nil {
		return BoundaryCaseReport{}, err
	}
	fixture, ok := LookupFixture(caseSpec.FixtureID)
	if !ok {
		return BoundaryCaseReport{}, fmt.Errorf("unknown fixture %q", caseSpec.FixtureID)
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return BoundaryCaseReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return BoundaryCaseReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return BoundaryCaseReport{}, err
	}
	defer cleanup()

	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return BoundaryCaseReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return BoundaryCaseReport{}, err
	}
	plan := fixtureEncodingPlan(fixture)
	planVersion, planHash, planJSON, err := transcodeencoding.Identity(plan)
	if err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("encoding plan identity: %w", err)
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("timestamp plan identity: %w", err)
	}

	sourcePath := filepath.Join(workDir, "source.mp4")
	if err := runCommand(ctx, ffmpegPath, sourceArgs(sourcePath, fixture)...); err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("generate boundary source fixture: %w", err)
	}
	startupManifest, err := produceBoundaryHLS(ctx, ffmpegPath, workDir, "startup", sourcePath, timestampPlan, fixture, 0, caseSpec.ExpectedBoundaryMicros)
	if err != nil {
		return BoundaryCaseReport{}, err
	}
	continuationManifest, err := produceBoundaryHLS(ctx, ffmpegPath, workDir, "continuation", sourcePath, timestampPlan, fixture, caseSpec.ExpectedBoundaryMicros, 0)
	if err != nil {
		return BoundaryCaseReport{}, err
	}

	verifier := transcodeattestation.Verifier{FFprobePath: ffprobePath}
	startup, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        startupManifest,
		EncodingPlanVersion: planVersion,
		EncodingPlanHash:    planHash,
		EncodingPlanJSON:    planJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("verify boundary startup fixture: %w", err)
	}
	continuation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuationManifest,
		EncodingPlanVersion: planVersion,
		EncodingPlanHash:    planHash,
		EncodingPlanJSON:    planJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("verify boundary continuation fixture: %w", err)
	}
	boundaryMS := int64(math.Round(float64(caseSpec.ExpectedBoundaryMicros) / 1000))
	if err := timestampPlan.VerifyObservedStart(0, startup.First.Timeline.Video.StartMS, startup.First.Timeline.Audio.StartMS); err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("boundary startup timestamp certification: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(boundaryMS, continuation.First.Timeline.Video.StartMS, continuation.First.Timeline.Audio.StartMS); err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("boundary continuation timestamp certification: %w", err)
	}
	startupVersion, startupHash, _, err := transcodeattestation.Identity(startup)
	if err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("boundary startup attestation identity: %w", err)
	}
	continuationVersion, continuationHash, _, err := transcodeattestation.Identity(continuation)
	if err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("boundary continuation attestation identity: %w", err)
	}

	evidence, err := probeBoundaryContract(ctx, ffprobePath, boundaryContractRequest{
		Case:                           caseSpec,
		Fixture:                        fixture,
		StartupManifest:                startupManifest,
		ContinuationManifest:           continuationManifest,
		FFmpegVersion:                  ffmpegVersion,
		FFprobeVersion:                 ffprobeVersion,
		EncodingPlanVersion:            planVersion,
		EncodingPlanHash:               planHash,
		TimestampPlanVersion:           timestampVersion,
		TimestampPlanHash:              timestampHash,
		StartupAttestationVersion:      startupVersion,
		StartupAttestationHash:         startupHash,
		ContinuationAttestationVersion: continuationVersion,
		ContinuationAttestationHash:    continuationHash,
	})
	if err != nil {
		return BoundaryCaseReport{}, err
	}
	contractVersion, contractHash, _, err := transcodeboundary.Identity(evidence)
	if err != nil {
		return BoundaryCaseReport{}, fmt.Errorf("boundary evidence identity: %w", err)
	}
	report := BoundaryCaseReport{
		Case:            caseSpec,
		ContractVersion: contractVersion,
		ContractHash:    contractHash,
		Evidence:        evidence,
	}
	if err := report.Validate(); err != nil {
		return BoundaryCaseReport{}, err
	}
	return report, nil
}
