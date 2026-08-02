package certification

import (
	"context"
	"fmt"
	"math"
	"path/filepath"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeavsync "github.com/nowen-video/nowen-video/internal/transcode/avsync"
	transcodeboundary "github.com/nowen-video/nowen-video/internal/transcode/boundaryevidence"
	transcodecandidate "github.com/nowen-video/nowen-video/internal/transcode/realmediacandidate"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodeoutputcadence "github.com/nowen-video/nowen-video/internal/transcode/outputcadence"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

func RunRealMediaCandidateMatrix(ctx context.Context, config RealMediaCandidateConfig) (RealMediaCandidateMatrixReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, spec, manifest, manifestVersion, manifestHash, err := loadRealMediaCorpus(config)
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config.Config)
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	defer cleanup()
	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	candidateSpecs := AvailableEncoderTimeBaseCandidates()
	cases := make([]transcodecandidate.CaseEvidence, 0, len(spec.Cases))
	for index, caseSpec := range spec.Cases {
		asset := manifest.Assets[index]
		sourcePath := filepath.Join(root, filepath.FromSlash(asset.RelativePath))
		reorderSpec, err := transcodecandidate.CaseSpecFor(caseSpec, asset)
		if err != nil {
			return RealMediaCandidateMatrixReport{}, err
		}
		sourceStartup, sourceContinuation, err := probeRealMediaSourceTimelines(ctx, ffprobePath, sourcePath, reorderSpec.Base)
		if err != nil {
			return RealMediaCandidateMatrixReport{}, fmt.Errorf("probe real-media source %s: %w", caseSpec.ID, err)
		}
		candidateEvidence := make([]transcodereorder.CandidateEvidence, 0, len(candidateSpecs))
		for _, candidateSpec := range candidateSpecs {
			runs := make([]transcodereorder.RunEvidence, 0, transcodereorder.RepeatCount)
			for ordinal := 1; ordinal <= transcodereorder.RepeatCount; ordinal++ {
				runDir := filepath.Join(workDir, "real-media-candidate", caseSpec.ID, candidateSpec.ID, fmt.Sprintf("run-%02d", ordinal))
				run, err := runRealMediaCandidate(
					ctx,
					ffmpegPath,
					ffprobePath,
					runDir,
					workDir,
					sourcePath,
					ffmpegVersion,
					ffprobeVersion,
					timestampPlan,
					timestampVersion,
					timestampHash,
					reorderSpec,
					candidateSpec,
					ordinal,
					sourceStartup,
					sourceContinuation,
				)
				if err != nil {
					return RealMediaCandidateMatrixReport{}, fmt.Errorf("run real-media case %s candidate %s repeat %d: %w", caseSpec.ID, candidateSpec.ID, ordinal, err)
				}
				runs = append(runs, run)
			}
			candidate := transcodereorder.CandidateEvidence{
				Spec:    candidateSpec,
				Runs:    runs,
				Summary: transcodereorder.BuildCandidateSummary(runs),
			}
			if err := candidate.Validate(reorderSpec, sourceStartup, sourceContinuation); err != nil {
				return RealMediaCandidateMatrixReport{}, err
			}
			candidateEvidence = append(candidateEvidence, candidate)
		}
		reorderEvidence := transcodereorder.CaseEvidence{
			Case:                       reorderSpec,
			SourceStartupTimeline:      sourceStartup,
			SourceContinuationTimeline: sourceContinuation,
			Candidates:                 candidateEvidence,
			Comparison:                 transcodereorder.BuildCandidateComparison(candidateEvidence[0], candidateEvidence[1]),
		}
		if err := reorderEvidence.Validate(); err != nil {
			return RealMediaCandidateMatrixReport{}, err
		}
		boundCase, err := transcodecandidate.BuildCaseEvidence(index, caseSpec, asset, reorderEvidence)
		if err != nil {
			return RealMediaCandidateMatrixReport{}, err
		}
		cases = append(cases, boundCase)
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	contract := transcodecandidate.Contract{
		SchemaVersion:               transcodecandidate.SchemaVersion,
		SpecVersion:                 specVersion,
		SpecHash:                    specHash,
		ManifestVersion:             manifestVersion,
		ManifestHash:                manifestHash,
		SourceGeneratorVersion:      manifest.GeneratorVersion,
		SourceFFmpegVersion:         manifest.FFmpegVersion,
		SourceFFprobeVersion:        manifest.FFprobeVersion,
		CertificationFFmpegVersion:  ffmpegVersion,
		CertificationFFprobeVersion: ffprobeVersion,
		RepeatCount:                 transcodereorder.RepeatCount,
		Cases:                       cases,
		DiscontinuityRequired:       true,
	}
	contractVersion, contractHash, _, err := transcodecandidate.Identity(contract, spec, manifest)
	if err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	report := RealMediaCandidateMatrixReport{
		SchemaVersion:   RealMediaCandidateMatrixSchemaVersion,
		Spec:            spec,
		Manifest:        manifest,
		ContractVersion: contractVersion,
		ContractHash:    contractHash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return RealMediaCandidateMatrixReport{}, err
	}
	return report, nil
}

func runRealMediaCandidate(
	ctx context.Context,
	ffmpegPath,
	ffprobePath,
	runDir,
	matrixWorkDir,
	sourcePath,
	ffmpegVersion,
	ffprobeVersion string,
	timestampPlan transcodetimestamp.Plan,
	timestampVersion,
	timestampHash string,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	ordinal int,
	sourceStartup,
	sourceContinuation transcodeoutputcadence.TimelineEvidence,
) (transcodereorder.RunEvidence, error) {
	productionSpec := realMediaSourceOriginSpec(caseSpec)
	startup, err := produceRealMediaCandidate(ctx, ffmpegPath, filepath.Join(runDir, "startup"), sourcePath, timestampPlan, caseSpec, candidateSpec, 0, caseSpec.Base.ExpectedBoundaryMicros)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	continuation, err := produceRealMediaCandidate(ctx, ffmpegPath, filepath.Join(runDir, "continuation"), sourcePath, timestampPlan, caseSpec, candidateSpec, caseSpec.Base.ExpectedBoundaryMicros, 0)
	if err != nil {
		return transcodereorder.RunEvidence{}, err
	}
	encodingPlan := encoderTimeBaseEncodingPlan(productionSpec, candidateSpec)
	encodingPlan.ProfileID = "real-media-corpus-reorder-180p-" + candidateSpec.ID
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
		return transcodereorder.RunEvidence{}, fmt.Errorf("verify real-media startup: %w", err)
	}
	continuationAttestation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        continuation.Manifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return transcodereorder.RunEvidence{}, fmt.Errorf("verify real-media continuation: %w", err)
	}
	boundaryMS := int64(math.Round(float64(caseSpec.Base.ExpectedBoundaryMicros) / 1_000))
	if err := timestampPlan.VerifyObservedStart(0, startupAttestation.First.Timeline.Video.StartMS, startupAttestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodereorder.RunEvidence{}, fmt.Errorf("real-media startup normalization: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(boundaryMS, continuationAttestation.First.Timeline.Video.StartMS, continuationAttestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodereorder.RunEvidence{}, fmt.Errorf("real-media continuation normalization: %w", err)
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
			Description:            caseSpec.Base.Description + " / " + candidateSpec.Description + " / real-media reorder",
			FixtureID:              productionSpec.FixtureID,
			ExpectedBoundaryMicros: caseSpec.Base.ExpectedBoundaryMicros,
		},
		Fixture:                        FixtureSpec{ID: productionSpec.FixtureID},
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
		StartupCommandHash:      hashRealMediaArgs(startup.Args, matrixWorkDir, sourcePath),
		ContinuationCommandHash: hashRealMediaArgs(continuation.Args, matrixWorkDir, sourcePath),
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
