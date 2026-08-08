package certification

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodesourceorigin "github.com/nowen-video/nowen-video/internal/transcode/sourceorigin"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

type encoderTimeBaseProduced struct {
	Manifest string
	Args     []string
}

func RunEncoderTimeBaseMatrix(ctx context.Context, config Config) (EncoderTimeBaseMatrixReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}
	defer cleanup()

	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}

	cases := make([]transcodetimebase.CaseEvidence, 0, len(encoderTimeBaseCaseSpecs))
	for _, caseSpec := range encoderTimeBaseCaseSpecs {
		if err := caseSpec.Validate(); err != nil {
			return EncoderTimeBaseMatrixReport{}, err
		}
		sourceGraph := encoderTimeBaseInputGraph(caseSpec)
		sourceStartup, sourceContinuation, err := probeEncoderTimeBaseSourceTimelines(ctx, ffprobePath, sourceGraph, caseSpec)
		if err != nil {
			return EncoderTimeBaseMatrixReport{}, fmt.Errorf("probe encoder time-base source %s: %w", caseSpec.ID, err)
		}
		candidateEvidence := make([]transcodetimebase.CandidateEvidence, 0, len(encoderTimeBaseCandidateSpecs))
		for _, candidateSpec := range encoderTimeBaseCandidateSpecs {
			runs := make([]transcodetimebase.RunEvidence, 0, transcodetimebase.RepeatCount)
			for ordinal := 1; ordinal <= transcodetimebase.RepeatCount; ordinal++ {
				runDir := filepath.Join(workDir, "encoder-time-base", caseSpec.ID, candidateSpec.ID, fmt.Sprintf("run-%02d", ordinal))
				run, err := runEncoderTimeBaseCandidate(
					ctx,
					ffmpegPath,
					ffprobePath,
					runDir,
					workDir,
					ffmpegVersion,
					ffprobeVersion,
					timestampPlan,
					timestampVersion,
					timestampHash,
					caseSpec,
					candidateSpec,
					ordinal,
					sourceGraph,
					sourceStartup,
					sourceContinuation,
				)
				if err != nil {
					return EncoderTimeBaseMatrixReport{}, fmt.Errorf("run case %s candidate %s repeat %d: %w", caseSpec.ID, candidateSpec.ID, ordinal, err)
				}
				runs = append(runs, run)
			}
			candidate := transcodetimebase.CandidateEvidence{
				Spec:    candidateSpec,
				Runs:    runs,
				Summary: transcodetimebase.BuildCandidateSummary(runs),
			}
			if err := candidate.Validate(caseSpec, sourceStartup, sourceContinuation); err != nil {
				return EncoderTimeBaseMatrixReport{}, err
			}
			candidateEvidence = append(candidateEvidence, candidate)
		}
		caseEvidence := transcodetimebase.CaseEvidence{
			Case:                       caseSpec,
			SourceStartupTimeline:      sourceStartup,
			SourceContinuationTimeline: sourceContinuation,
			Candidates:                 candidateEvidence,
			Comparison:                 transcodetimebase.BuildCandidateComparison(candidateEvidence[0], candidateEvidence[1]),
		}
		if err := caseEvidence.Validate(); err != nil {
			return EncoderTimeBaseMatrixReport{}, err
		}
		cases = append(cases, caseEvidence)
	}

	contract := transcodetimebase.Contract{
		SchemaVersion:                 transcodetimebase.SchemaVersion,
		FFmpegVersion:                 ffmpegVersion,
		FFprobeVersion:                ffprobeVersion,
		RepeatCount:                   transcodetimebase.RepeatCount,
		VarianceToleranceMicros:       transcodetimebase.VarianceToleranceMicros,
		CrossCandidateToleranceMicros: transcodetimebase.CrossCandidateToleranceMicros,
		Cases:                         cases,
		DiscontinuityRequired:         true,
	}
	version, hash, _, err := transcodetimebase.Identity(contract)
	if err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}
	report := EncoderTimeBaseMatrixReport{
		SchemaVersion:   EncoderTimeBaseMatrixSchemaVersion,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return EncoderTimeBaseMatrixReport{}, err
	}
	return report, nil
}

func runEncoderTimeBaseCandidate(
	ctx context.Context,
	ffmpegPath,
	ffprobePath,
	runDir,
	matrixWorkDir,
	ffmpegVersion,
	ffprobeVersion string,
	timestampPlan transcodetimestamp.Plan,
	timestampVersion,
	timestampHash string,
	caseSpec transcodetimebase.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	ordinal int,
	sourceGraph string,
	sourceStartup,
	sourceContinuation transcodeoutputcadence.TimelineEvidence,
) (transcodetimebase.RunEvidence, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	productionSpec := encoderTimeBaseSourceOriginSpec(caseSpec)
	startup, err := produceEncoderTimeBaseCandidate(
		ctx, ffmpegPath, filepath.Join(runDir, "startup"), sourceGraph, timestampPlan, productionSpec,
		candidateSpec, 0, caseSpec.ExpectedBoundaryMicros,
	)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	continuation, err := produceEncoderTimeBaseCandidate(
		ctx, ffmpegPath, filepath.Join(runDir, "continuation"), sourceGraph, timestampPlan, productionSpec,
		candidateSpec, caseSpec.ExpectedBoundaryMicros, 0,
	)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}

	encodingPlan := encoderTimeBaseEncodingPlan(productionSpec, candidateSpec)
	encodingVersion, encodingHash, encodingJSON, err := transcodeencoding.Identity(encodingPlan)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	verifier := transcodeattestation.Verifier{FFprobePath: ffprobePath}
	startupAttestation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        startup.Manifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return transcodetimebase.RunEvidence{}, fmt.Errorf("verify startup candidate: %w", err)
	}
	continuationAttestation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuation.Manifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return transcodetimebase.RunEvidence{}, fmt.Errorf("verify continuation candidate: %w", err)
	}
	boundaryMS := int64(math.Round(float64(caseSpec.ExpectedBoundaryMicros) / 1_000))
	if err := timestampPlan.VerifyObservedStart(0, startupAttestation.First.Timeline.Video.StartMS, startupAttestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodetimebase.RunEvidence{}, fmt.Errorf("startup normalization: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(boundaryMS, continuationAttestation.First.Timeline.Video.StartMS, continuationAttestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodetimebase.RunEvidence{}, fmt.Errorf("continuation normalization: %w", err)
	}
	startupAttestationVersion, startupAttestationHash, _, err := transcodeattestation.Identity(startupAttestation)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	continuationAttestationVersion, continuationAttestationHash, _, err := transcodeattestation.Identity(continuationAttestation)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}

	boundaryCaseID := transcodetimebase.BoundaryCaseID(caseSpec.ID, candidateSpec.ID, ordinal)
	boundary, err := probeBoundaryContract(ctx, ffprobePath, boundaryContractRequest{
		Case: BoundaryCaseSpec{
			ID:                     boundaryCaseID,
			Description:            caseSpec.Description + " / " + candidateSpec.Description,
			FixtureID:              "fixture-" + caseSpec.ID,
			ExpectedBoundaryMicros: caseSpec.ExpectedBoundaryMicros,
		},
		Fixture:                        FixtureSpec{ID: "fixture-" + caseSpec.ID},
		StartupManifest:                startup.Manifest,
		ContinuationManifest:           continuation.Manifest,
		FFmpegVersion:                  ffmpegVersion,
		FFprobeVersion:                 ffprobeVersion,
		EncodingPlanVersion:            encodingVersion,
		EncodingPlanHash:               encodingHash,
		TimestampPlanVersion:           timestampVersion,
		TimestampPlanHash:              timestampHash,
		StartupAttestationVersion:      startupAttestationVersion,
		StartupAttestationHash:         startupAttestationHash,
		ContinuationAttestationVersion: continuationAttestationVersion,
		ContinuationAttestationHash:    continuationAttestationHash,
	})
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	boundaryVersion, boundaryHash, _, err := transcodeboundary.Identity(boundary)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	avSync, err := transcodeavsync.FromBoundary(boundary)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	avSyncVersion, avSyncHash, _, err := transcodeavsync.Identity(avSync)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}

	startupTimeline, _, err := probeVideoCadenceTimeline(
		ctx, ffprobePath, outputCadenceProbeInput{Path: startup.Manifest},
		transcodetimebase.TimelineKind(caseSpec.ID, candidateSpec.ID, ordinal, "startup"),
		0, caseSpec.ExpectedBoundaryMicros,
	)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	continuationTimeline, _, err := probeVideoCadenceTimeline(
		ctx, ffprobePath, outputCadenceProbeInput{Path: continuation.Manifest},
		transcodetimebase.TimelineKind(caseSpec.ID, candidateSpec.ID, ordinal, "continuation"),
		caseSpec.ExpectedBoundaryMicros, caseSpec.DurationMicros,
	)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	startupFingerprint, err := probeDecodedFrameFingerprint(ctx, ffmpegPath, startup.Manifest)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	continuationFingerprint, err := probeDecodedFrameFingerprint(ctx, ffmpegPath, continuation.Manifest)
	if err != nil {
		return transcodetimebase.RunEvidence{}, err
	}

	run := transcodetimebase.RunEvidence{
		Ordinal:                 ordinal,
		StartupCommandHash:      hashNormalizedArgs(startup.Args, matrixWorkDir),
		ContinuationCommandHash: hashNormalizedArgs(continuation.Args, matrixWorkDir),
		StartupTimeline:         startupTimeline,
		ContinuationTimeline:    continuationTimeline,
		StartupMapping:          transcodeoutputcadence.NewFrameMapping(sourceStartup.FrameCount, startupTimeline.FrameCount),
		ContinuationMapping:     transcodeoutputcadence.NewFrameMapping(sourceContinuation.FrameCount, continuationTimeline.FrameCount),
		StartupFingerprint:      startupFingerprint,
		ContinuationFingerprint: continuationFingerprint,
		BoundaryVersion:         boundaryVersion,
		BoundaryHash:            boundaryHash,
		Boundary:                boundary,
		AVSyncVersion:           avSyncVersion,
		AVSyncHash:              avSyncHash,
		AVSync:                  avSync,
	}
	if err := run.Validate(caseSpec, candidateSpec, sourceStartup, sourceContinuation, ordinal); err != nil {
		return transcodetimebase.RunEvidence{}, err
	}
	return run, nil
}

func produceEncoderTimeBaseCandidate(
	ctx context.Context,
	ffmpegPath,
	outputDir,
	sourceGraph string,
	timestampPlan transcodetimestamp.Plan,
	productionSpec SourceOriginCaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	startMicros,
	durationMicros int64,
) (encoderTimeBaseProduced, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	args, err := sourceOriginHLSArgs(sourceGraph, outputDir, timestampPlan, productionSpec, startMicros, durationMicros)
	if err != nil {
		return encoderTimeBaseProduced{}, err
	}
	args = insertIsolationBeforeOutput(args, "-enc_time_base:v:0", candidateSpec.EncoderTimeBase)
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	return encoderTimeBaseProduced{Manifest: filepath.Join(outputDir, "stream.m3u8"), Args: args}, nil
}

func probeEncoderTimeBaseSourceTimelines(
	ctx context.Context,
	ffprobePath,
	sourceGraph string,
	caseSpec transcodetimebase.CaseSpec,
) (transcodeoutputcadence.TimelineEvidence, transcodeoutputcadence.TimelineEvidence, error) {
	full, points, err := probeVideoCadenceTimeline(
		ctx,
		ffprobePath,
		outputCadenceProbeInput{Path: sourceGraph, Lavfi: true},
		"encoder_time_base_source_full_"+caseSpec.ID,
		caseSpec.SourceOffsetMicros,
		caseSpec.SourceOffsetMicros+caseSpec.DurationMicros,
	)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, transcodeoutputcadence.TimelineEvidence{}, err
	}
	startupTicks := make([]int64, 0, len(points))
	continuationTicks := make([]int64, 0, len(points))
	for _, point := range points {
		relative := point.Micros - caseSpec.SourceOffsetMicros
		switch {
		case relative >= 0 && relative < caseSpec.ExpectedBoundaryMicros:
			startupTicks = append(startupTicks, point.Ticks)
		case relative >= caseSpec.ExpectedBoundaryMicros && relative < caseSpec.DurationMicros:
			continuationTicks = append(continuationTicks, point.Ticks)
		}
	}
	timeBase := full.TimeBase
	startup, err := transcodeoutputcadence.NewTimelineEvidence(
		transcodeoutputcadence.TimelineSourceStartup,
		timeBase,
		caseSpec.SourceOffsetMicros,
		caseSpec.SourceOffsetMicros+caseSpec.ExpectedBoundaryMicros,
		startupTicks,
	)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, transcodeoutputcadence.TimelineEvidence{}, err
	}
	continuation, err := transcodeoutputcadence.NewTimelineEvidence(
		transcodeoutputcadence.TimelineSourceContinuation,
		timeBase,
		caseSpec.SourceOffsetMicros+caseSpec.ExpectedBoundaryMicros,
		caseSpec.SourceOffsetMicros+caseSpec.DurationMicros,
		continuationTicks,
	)
	if err != nil {
		return transcodeoutputcadence.TimelineEvidence{}, transcodeoutputcadence.TimelineEvidence{}, err
	}
	return startup, continuation, nil
}

func encoderTimeBaseInputGraph(caseSpec transcodetimebase.CaseSpec) string {
	offset := sourceOriginOffsetExpression(caseSpec.SourceOffsetMicros)
	if caseSpec.SourceMode == transcodesourceorigin.ModeVFR {
		return fmt.Sprintf(
			"testsrc2=size=%dx%d:rate=%s:duration=20,settb=AVTB[v0];"+
				"testsrc2=size=%dx%d:rate=%s:duration=20,settb=AVTB[v1];"+
				"[v0][v1]concat=n=2:v=1:a=0,setpts=%s[out0];"+
				"sine=frequency=1000:sample_rate=%d:duration=40,asettb=1/%d,asetpts=%s[out1]",
			fixtureWidth, fixtureHeight, encoderTimeBaseRateExpression(caseSpec.PrimaryFrameRateNumerator, caseSpec.PrimaryFrameRateDenominator),
			fixtureWidth, fixtureHeight, encoderTimeBaseRateExpression(caseSpec.SecondaryFrameRateNumerator, caseSpec.SecondaryFrameRateDenominator),
			offset,
			caseSpec.AudioSampleRate, caseSpec.AudioSampleRate, offset,
		)
	}
	return fmt.Sprintf(
		"testsrc2=size=%dx%d:rate=%s:duration=40,settb=AVTB,setpts=%s[out0];"+
			"sine=frequency=1000:sample_rate=%d:duration=40,asettb=1/%d,asetpts=%s[out1]",
		fixtureWidth, fixtureHeight,
		encoderTimeBaseRateExpression(caseSpec.PrimaryFrameRateNumerator, caseSpec.PrimaryFrameRateDenominator),
		offset,
		caseSpec.AudioSampleRate, caseSpec.AudioSampleRate, offset,
	)
}

func encoderTimeBaseRateExpression(numerator, denominator int64) string {
	if denominator == 1 {
		return fmt.Sprint(numerator)
	}
	return fmt.Sprintf("%d/%d", numerator, denominator)
}

func encoderTimeBaseSourceOriginSpec(caseSpec transcodetimebase.CaseSpec) SourceOriginCaseSpec {
	numerator := caseSpec.PrimaryFrameRateNumerator
	denominator := caseSpec.PrimaryFrameRateDenominator
	if caseSpec.SourceMode == transcodesourceorigin.ModeVFR {
		numerator = int64(caseSpec.DeclaredFrameRateMilli())
		denominator = 1_000
	}
	return SourceOriginCaseSpec{
		ID:                           caseSpec.ID,
		Description:                  caseSpec.Description,
		FixtureID:                    "fixture-" + caseSpec.ID,
		SourceMode:                   caseSpec.SourceMode,
		DeclaredFrameRateNumerator:   numerator,
		DeclaredFrameRateDenominator: denominator,
		SourceOffsetMicros:           caseSpec.SourceOffsetMicros,
		AudioSampleRate:              caseSpec.AudioSampleRate,
		GOPSize:                      caseSpec.GOPSize,
		ExpectedBoundaryMicros:       caseSpec.ExpectedBoundaryMicros,
	}
}

func encoderTimeBaseEncodingPlan(spec SourceOriginCaseSpec, candidate transcodetimebase.CandidateSpec) transcodeencoding.Plan {
	plan := sourceOriginEncodingPlan(spec)
	plan.ProfileID = "encoder-time-base-180p-" + candidate.ID
	return plan
}
