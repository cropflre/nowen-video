package certification

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodecandidate "github.com/nowen-video/nowen-video/internal/transcode/realmediacandidate"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const LongDurationDriftMatrixSchemaVersion = "ffmpeg-long-duration-drift-matrix-v1"

type LongDurationDriftConfig struct {
	Config
	CorpusRoot   string
	ManifestPath string
}

type LongDurationDriftMatrixReport struct {
	SchemaVersion   string                      `json:"schema_version"`
	Spec            transcodecorpus.Spec        `json:"spec"`
	Manifest        transcodecorpus.Manifest    `json:"manifest"`
	ContractVersion string                      `json:"contract_version"`
	ContractHash    string                      `json:"contract_hash"`
	Evidence        transcodelongdrift.Contract `json:"evidence"`
}

func RunLongDurationDriftMatrix(ctx context.Context, config LongDurationDriftConfig) (LongDurationDriftMatrixReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, spec, manifest, manifestVersion, manifestHash, err := loadRealMediaCorpus(RealMediaCandidateConfig{
		Config:       config.Config,
		CorpusRoot:   config.CorpusRoot,
		ManifestPath: config.ManifestPath,
	})
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	caseSpec, asset, ok := longDurationSource(spec, manifest)
	if !ok {
		return LongDurationDriftMatrixReport{}, fmt.Errorf("long-duration source %s is missing", transcodelongdrift.SourceCaseID)
	}
	sourcePath := filepath.Join(root, filepath.FromSlash(asset.RelativePath))
	reorderSpec, err := transcodecandidate.CaseSpecFor(caseSpec, asset)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config.Config)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	defer cleanup()
	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	candidateEvidence := make([]transcodelongdrift.CandidateEvidence, 0, 2)
	for _, candidateSpec := range AvailableEncoderTimeBaseCandidates() {
		runs := make([]transcodelongdrift.RunEvidence, 0, transcodelongdrift.RepeatCount)
		for ordinal := 1; ordinal <= transcodelongdrift.RepeatCount; ordinal++ {
			runDir := filepath.Join(workDir, "long-duration-drift", candidateSpec.ID, fmt.Sprintf("run-%02d", ordinal))
			run, err := runLongDurationDriftCandidate(
				ctx,
				ffmpegPath,
				ffprobePath,
				runDir,
				workDir,
				sourcePath,
				timestampPlan,
				reorderSpec,
				candidateSpec,
				ordinal,
			)
			if err != nil {
				return LongDurationDriftMatrixReport{}, fmt.Errorf("run long-duration candidate %s repeat %d: %w", candidateSpec.ID, ordinal, err)
			}
			runs = append(runs, run)
		}
		candidate := transcodelongdrift.CandidateEvidence{
			ID:              candidateSpec.ID,
			EncoderTimeBase: candidateSpec.EncoderTimeBase,
			Runs:            runs,
			Summary:         transcodelongdrift.BuildCandidateSummary(runs),
		}
		if err := candidate.Validate(); err != nil {
			return LongDurationDriftMatrixReport{}, err
		}
		candidateEvidence = append(candidateEvidence, candidate)
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	assetHash, err := transcodelongdrift.CanonicalHash(asset)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	contract := transcodelongdrift.Contract{
		SchemaVersion:                 transcodelongdrift.SchemaVersion,
		SpecVersion:                   specVersion,
		SpecHash:                      specHash,
		ManifestVersion:               manifestVersion,
		ManifestHash:                  manifestHash,
		SourceGeneratorVersion:        manifest.GeneratorVersion,
		SourceFFmpegVersion:           manifest.FFmpegVersion,
		SourceFFprobeVersion:          manifest.FFprobeVersion,
		CertificationFFmpegVersion:    ffmpegVersion,
		CertificationFFprobeVersion:   ffprobeVersion,
		TimestampPlanVersion:          timestampVersion,
		TimestampPlanHash:             timestampHash,
		Source: transcodelongdrift.SourceIdentity{
			CaseID:            asset.CaseID,
			RelativePath:      asset.RelativePath,
			SHA256:            asset.SHA256,
			SizeBytes:         asset.SizeBytes,
			AssetEvidenceHash: assetHash,
		},
		DurationMicros:                transcodelongdrift.DurationMicros,
		CheckpointIntervalMicros:      transcodelongdrift.CheckpointMicros,
		RepeatCount:                   transcodelongdrift.RepeatCount,
		StartToleranceMicros:          transcodelongdrift.StartToleranceMicros,
		EndToleranceMicros:            transcodelongdrift.EndToleranceMicros,
		CheckpointToleranceMicros:     transcodelongdrift.CheckpointToleranceMicros,
		AVSkewToleranceMicros:         transcodelongdrift.AVSkewToleranceMicros,
		RepeatVarianceToleranceMicros: transcodelongdrift.RepeatVarianceToleranceMicros,
		CrossCandidateToleranceMicros: transcodelongdrift.CrossCandidateToleranceMicros,
		Candidates:                    candidateEvidence,
		Comparison:                    transcodelongdrift.BuildCandidateComparison(candidateEvidence[0], candidateEvidence[1]),
		DiscontinuityRequired:         true,
	}
	version, hash, _, err := transcodelongdrift.Identity(contract, spec, manifest)
	if err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	report := LongDurationDriftMatrixReport{
		SchemaVersion:   LongDurationDriftMatrixSchemaVersion,
		Spec:            spec,
		Manifest:        manifest,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return LongDurationDriftMatrixReport{}, err
	}
	return report, nil
}

func runLongDurationDriftCandidate(
	ctx context.Context,
	ffmpegPath,
	ffprobePath,
	runDir,
	workDir,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	ordinal int,
) (transcodelongdrift.RunEvidence, error) {
	produced, err := produceLongDurationDriftCandidate(ctx, ffmpegPath, runDir, sourcePath, timestampPlan, caseSpec, candidateSpec)
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	productionSpec := realMediaSourceOriginSpec(caseSpec)
	encodingPlan := encoderTimeBaseEncodingPlan(productionSpec, candidateSpec)
	encodingPlan.ProfileID = "long-duration-drift-180p-" + candidateSpec.ID
	encodingVersion, encodingHash, encodingJSON, err := transcodeencoding.Identity(encodingPlan)
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	verifier := transcodeattestation.Verifier{FFprobePath: ffprobePath}
	attestation, err := verifier.Verify(ctx, transcodeattestation.VerifyRequest{
		ManifestPath:        produced.Manifest,
		EncodingPlanVersion: encodingVersion,
		EncodingPlanHash:    encodingHash,
		EncodingPlanJSON:    encodingJSON,
		Scope:               transcodeattestation.ScopeComplete,
	})
	if err != nil {
		return transcodelongdrift.RunEvidence{}, fmt.Errorf("verify long-duration output: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(0, attestation.First.Timeline.Video.StartMS, attestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodelongdrift.RunEvidence{}, fmt.Errorf("long-duration start normalization: %w", err)
	}
	attestationVersion, attestationHash, _, err := transcodeattestation.Identity(attestation)
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	video, err := probeLongDriftStream(ctx, ffprobePath, produced.Manifest, "v:0", "video")
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	audio, err := probeLongDriftStream(ctx, ffprobePath, produced.Manifest, "a:0", "audio")
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	manifestHash, err := realMediaFileSHA256(produced.Manifest)
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	run := transcodelongdrift.RunEvidence{
		Ordinal:            ordinal,
		CommandHash:        hashRealMediaArgs(produced.Args, workDir, sourcePath),
		ManifestSHA256:     manifestHash,
		AttestationVersion: attestationVersion,
		AttestationHash:    attestationHash,
		SegmentCount:       attestation.SegmentCount,
		Video:              video,
		Audio:              audio,
		FinalAVSkewMicros:  video.EndMicros - audio.EndMicros,
	}
	if err := run.Validate(ordinal); err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	return run, nil
}

func longDurationSource(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (transcodecorpus.CaseSpec, transcodecorpus.AssetEvidence, bool) {
	assets := make(map[string]transcodecorpus.AssetEvidence, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		assets[asset.CaseID] = asset
	}
	for _, caseSpec := range spec.Cases {
		if caseSpec.ID == transcodelongdrift.SourceCaseID {
			asset, ok := assets[caseSpec.ID]
			return caseSpec, asset, ok
		}
	}
	return transcodecorpus.CaseSpec{}, transcodecorpus.AssetEvidence{}, false
}

func (r LongDurationDriftMatrixReport) Validate() error {
	if r.SchemaVersion != LongDurationDriftMatrixSchemaVersion {
		return fmt.Errorf("unsupported long-duration matrix schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.ValidateFor(r.Spec, r.Manifest); err != nil {
		return err
	}
	version, hash, _, err := transcodelongdrift.Identity(r.Evidence, r.Spec, r.Manifest)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("long-duration contract identity is invalid")
	}
	return nil
}

func MarshalLongDurationDriftMatrixReport(report LongDurationDriftMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
