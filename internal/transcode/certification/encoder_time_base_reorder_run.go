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
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func RunEncoderTimeBaseReorderMatrix(ctx context.Context, config Config) (EncoderTimeBaseReorderMatrixReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config)
	if err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}
	defer cleanup()
	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}

	cases := make([]transcodereorder.CaseEvidence, 0, len(encoderTimeBaseReorderCaseSpecs))
	for _, caseSpec := range encoderTimeBaseReorderCaseSpecs {
		if err := caseSpec.Validate(); err != nil {
			return EncoderTimeBaseReorderMatrixReport{}, err
		}
		sourceGraph := encoderTimeBaseInputGraph(caseSpec.Base)
		sourceStartup, sourceContinuation, err := probeEncoderTimeBaseSourceTimelines(ctx, ffprobePath, sourceGraph, caseSpec.Base)
		if err != nil {
			return EncoderTimeBaseReorderMatrixReport{}, fmt.Errorf("probe reorder source %s: %w", caseSpec.Base.ID, err)
		}
		candidates := make([]transcodereorder.CandidateEvidence, 0, len(encoderTimeBaseCandidateSpecs))
		for _, candidateSpec := range encoderTimeBaseCandidateSpecs {
			runs := make([]transcodereorder.RunEvidence, 0, transcodereorder.RepeatCount)
			for ordinal := 1; ordinal <= transcodereorder.RepeatCount; ordinal++ {
				runDir := filepath.Join(workDir, "encoder-time-base-reorder", caseSpec.Base.ID, candidateSpec.ID, fmt.Sprintf("run-%02d", ordinal))
				run, err := runEncoderTimeBaseReorderCandidate(
					ctx, ffmpegPath, ffprobePath, runDir, workDir,
					ffmpegVersion, ffprobeVersion,
					timestampPlan, timestampVersion, timestampHash,
					caseSpec, candidateSpec, ordinal, sourceGraph,
					sourceStartup, sourceContinuation,
				)
				if err != nil {
					return EncoderTimeBaseReorderMatrixReport{}, fmt.Errorf("run reorder case %s candidate %s repeat %d: %w", caseSpec.Base.ID, candidateSpec.ID, ordinal, err)
				}
				runs = append(runs, run)
			}
			candidate := transcodereorder.CandidateEvidence{
				Spec:    candidateSpec,
				Runs:    runs,
				Summary: transcodereorder.BuildCandidateSummary(runs),
			}
			if err := candidate.Validate(caseSpec, sourceStartup, sourceContinuation); err != nil {
				return EncoderTimeBaseReorderMatrixReport{}, err
			}
			candidates = append(candidates, candidate)
		}
		caseEvidence := transcodereorder.CaseEvidence{
			Case:                       caseSpec,
			SourceStartupTimeline:      sourceStartup,
			SourceContinuationTimeline: sourceContinuation,
			Candidates:                 candidates,
			Comparison:                 transcodereorder.BuildCandidateComparison(candidates[0], candidates[1]),
		}
		if err := caseEvidence.Validate(); err != nil {
			return EncoderTimeBaseReorderMatrixReport{}, err
		}
		cases = append(cases, caseEvidence)
	}

	contract := transcodereorder.Contract{
		SchemaVersion:                transcodereorder.SchemaVersion,
		FFmpegVersion:                ffmpegVersion,
		FFprobeVersion:               ffprobeVersion,
		RepeatCount:                  transcodereorder.RepeatCount,
		PacketVarianceTicks:          transcodereorder.PacketVarianceTolerance,
		PerceptualMaxHammingDistance: transcodereorder.PerceptualMaxHammingDistance,
		Cases:                        cases,
		DiscontinuityRequired:        true,
	}
	version, hash, _, err := transcodereorder.Identity(contract)
	if err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}
	report := EncoderTimeBaseReorderMatrixReport{
		SchemaVersion:   EncoderTimeBaseReorderMatrixSchemaVersion,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return EncoderTimeBaseReorderMatrixReport{}, err
	}
	return report, nil
}

func runEncoderTimeBaseReorderCandidate(
	ctx context.Context,
	ffmpegPath, ffprobePath, runDir, matrixWorkDir,
	ffmpegVersion, ffprobeVersion string,
	timestampPlan transcodetimestamp.Plan,
	timestampVersion, timestampHash string,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	ordinal int,
	sourceGraph string,
	sourceStartup, sourceContinuation transcodeoutputcadence.TimelineEvidence,
) (transcodereorder.RunEvidence, error) {
	if err := os.MkdirAll(runDir, 0o755); err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	productionSpec := encoderTimeBaseSourceOriginSpec(caseSpec.Base)
	startup, err := produceEncoderTimeBaseReorderCandidate(ctx, ffmpegPath, filepath.Join(runDir, "startup"), sourceGraph, timestampPlan, productionSpec, caseSpec, candidateSpec, 0, caseSpec.Base.ExpectedBoundaryMicros)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	continuation, err := produceEncoderTimeBaseReorderCandidate(ctx, ffmpegPath, filepath.Join(runDir, "continuation"), sourceGraph, timestampPlan, productionSpec, caseSpec, candidateSpec, caseSpec.Base.ExpectedBoundaryMicros, 0)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}

	encodingPlan := encoderTimeBaseEncodingPlan(productionSpec, candidateSpec)
	encodingPlan.ProfileID = "encoder-time-base-reorder-180p-" + candidateSpec.ID
	encodingVersion, encodingHash, encodingJSON, err := transcodeencoding.Identity(encodingPlan)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
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
		return transcodereorder.RunEvidence{}, fmt.Errorf("verify reorder startup: %w", err)
	}
	continuationAttestation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuation.Manifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return transcodereorder.RunEvidence{}, fmt.Errorf("verify reorder continuation: %w", err)
	}
	boundaryMS := int64(math.Round(float64(caseSpec.Base.ExpectedBoundaryMicros) / 1_000))
	if err := timestampPlan.VerifyObservedStart(0, startupAttestation.First.Timeline.Video.StartMS, startupAttestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodereorder.RunEvidence{}, fmt.Errorf("reorder startup normalization: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(boundaryMS, continuationAttestation.First.Timeline.Video.StartMS, continuationAttestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodereorder.RunEvidence{}, fmt.Errorf("reorder continuation normalization: %w", err)
	}
	startupAttestationVersion, startupAttestationHash, _, err := transcodeattestation.Identity(startupAttestation)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	continuationAttestationVersion, continuationAttestationHash, _, err := transcodeattestation.Identity(continuationAttestation)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}

	boundary, err := probeBoundaryContract(ctx, ffprobePath, boundaryContractRequest{
		Case: BoundaryCaseSpec{
			ID:                     transcodetimebase.BoundaryCaseID(caseSpec.Base.ID, candidateSpec.ID, ordinal),
			Description:            caseSpec.Base.Description + " / " + candidateSpec.Description + " / B-frame reorder",
			FixtureID:              "fixture-" + caseSpec.Base.ID,
			ExpectedBoundaryMicros: caseSpec.Base.ExpectedBoundaryMicros,
		},
		Fixture:                        FixtureSpec{ID: "fixture-" + caseSpec.Base.ID},
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
		return transcodereorder.RunEvidence{}, err
	}
	boundaryVersion, boundaryHash, _, err := transcodeboundary.Identity(boundary)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	avSync, err := transcodeavsync.FromBoundary(boundary)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	avSyncVersion, avSyncHash, _, err := transcodeavsync.Identity(avSync)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}

	startupTimeline, _, err := probeVideoCadenceTimeline(ctx, ffprobePath, outputCadenceProbeInput{Path: startup.Manifest}, transcodetimebase.TimelineKind(caseSpec.Base.ID, candidateSpec.ID, ordinal, "startup"), 0, caseSpec.Base.ExpectedBoundaryMicros)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	continuationTimeline, _, err := probeVideoCadenceTimeline(ctx, ffprobePath, outputCadenceProbeInput{Path: continuation.Manifest}, transcodetimebase.TimelineKind(caseSpec.Base.ID, candidateSpec.ID, ordinal, "continuation"), caseSpec.Base.ExpectedBoundaryMicros, caseSpec.Base.DurationMicros)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	startupFingerprint, err := probeDecodedFrameFingerprint(ctx, ffmpegPath, startup.Manifest)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	continuationFingerprint, err := probeDecodedFrameFingerprint(ctx, ffmpegPath, continuation.Manifest)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	startupPerceptual, err := probePerceptualFrameSequence(ctx, ffmpegPath, startup.Manifest)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	continuationPerceptual, err := probePerceptualFrameSequence(ctx, ffmpegPath, continuation.Manifest)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	startupOrder, err := probeEncoderTimeBasePacketOrder(ctx, ffprobePath, startup.Manifest, transcodereorder.PacketKind(caseSpec.Base.ID, candidateSpec.ID, ordinal, "startup"))
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	continuationOrder, err := probeEncoderTimeBasePacketOrder(ctx, ffprobePath, continuation.Manifest, transcodereorder.PacketKind(caseSpec.Base.ID, candidateSpec.ID, ordinal, "continuation"))
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}

	base := transcodetimebase.RunEvidence{
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
	run := transcodereorder.RunEvidence{
		Ordinal:                        ordinal,
		Base:                           base,
		StartupPacketOrder:             startupOrder,
		ContinuationPacketOrder:        continuationOrder,
		StartupPerceptualSequence:      startupPerceptual,
		ContinuationPerceptualSequence: continuationPerceptual,
	}
	if err := run.Validate(caseSpec, candidateSpec, sourceStartup, sourceContinuation, ordinal); err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	return run, nil
}

func produceEncoderTimeBaseReorderCandidate(
	ctx context.Context,
	ffmpegPath, outputDir, sourceGraph string,
	timestampPlan transcodetimestamp.Plan,
	productionSpec SourceOriginCaseSpec,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	startMicros, durationMicros int64,
) (encoderTimeBaseProduced, error) {
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	args, err := sourceOriginHLSArgs(sourceGraph, outputDir, timestampPlan, productionSpec, startMicros, durationMicros)
	if err != nil {
		return encoderTimeBaseProduced{}, err
	}
	args = removeReorderOptionPair(args, "-tune", VideoTuneZeroLatency)
	x264Params := fmt.Sprintf("b-adapt=%d:b-pyramid=none:open-gop=0:scenecut=0", caseSpec.BAdapt)
	args = insertIsolationBeforeOutput(args,
		"-enc_time_base:v:0", candidateSpec.EncoderTimeBase,
		"-bf", fmt.Sprint(caseSpec.BFrames),
		"-b_strategy", fmt.Sprint(caseSpec.BAdapt),
		"-refs", fmt.Sprint(caseSpec.ReferenceFrames),
		"-x264-params", x264Params,
	)
	if err := runCommand(ctx, ffmpegPath, args...); err != nil {
		return encoderTimeBaseProduced{}, err
	}
	return encoderTimeBaseProduced{Manifest: filepath.Join(outputDir, "stream.m3u8"), Args: args}, nil
}

func removeReorderOptionPair(args []string, option, value string) []string {
	result := make([]string, 0, len(args))
	for index := 0; index < len(args); index++ {
		if index+1 < len(args) && args[index] == option && args[index+1] == value {
			index++
			continue
		}
		result = append(result, args[index])
	}
	return result
}
