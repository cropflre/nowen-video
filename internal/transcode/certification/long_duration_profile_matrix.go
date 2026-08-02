package certification

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodecandidate "github.com/nowen-video/nowen-video/internal/transcode/realmediacandidate"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const LongDurationProfileMatrixReportSchemaVersion = "ffmpeg-long-duration-profile-matrix-v2"

type LongDurationProfileMatrixReport struct {
	SchemaVersion   string                                   `json:"schema_version"`
	Spec            transcodecorpus.Spec                     `json:"spec"`
	Manifest        transcodecorpus.Manifest                 `json:"manifest"`
	ContractVersion string                                   `json:"contract_version"`
	ContractHash    string                                   `json:"contract_hash"`
	Evidence        transcodelongdrift.ProfileMatrixContract `json:"evidence"`
}

func RunLongDurationProfileMatrix(ctx context.Context, config LongDurationDriftConfig) (LongDurationProfileMatrixReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	root, spec, manifest, manifestVersion, manifestHash, err := loadRealMediaCorpus(RealMediaCandidateConfig{
		Config:       config.Config,
		CorpusRoot:   config.CorpusRoot,
		ManifestPath: config.ManifestPath,
	})
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config.Config)
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	defer cleanup()
	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}

	profiles := make([]transcodelongdrift.ProfileEvidence, 0, len(transcodelongdrift.AvailableProfiles()))
	for _, profileSpec := range transcodelongdrift.AvailableProfiles() {
		caseSpec, asset, ok := longDurationProfileSource(spec, manifest, profileSpec.SourceCaseID)
		if !ok {
			return LongDurationProfileMatrixReport{}, fmt.Errorf("long-duration profile source %s is missing", profileSpec.SourceCaseID)
		}
		sourcePath := filepath.Join(root, filepath.FromSlash(asset.RelativePath))
		reorderSpec, err := transcodecandidate.CaseSpecFor(caseSpec, asset)
		if err != nil {
			return LongDurationProfileMatrixReport{}, fmt.Errorf("build long-duration profile %s: %w", profileSpec.ID, err)
		}
		candidateEvidence := make([]transcodelongdrift.CandidateEvidence, 0, 2)
		for _, candidateSpec := range AvailableEncoderTimeBaseCandidates() {
			runs := make([]transcodelongdrift.RunEvidence, 0, transcodelongdrift.RepeatCount)
			for ordinal := 1; ordinal <= transcodelongdrift.RepeatCount; ordinal++ {
				runDir := filepath.Join(workDir, "long-duration-profile-matrix", profileSpec.ID, candidateSpec.ID, fmt.Sprintf("run-%02d", ordinal))
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
					return LongDurationProfileMatrixReport{}, fmt.Errorf("run profile %s candidate %s repeat %d: %w", profileSpec.ID, candidateSpec.ID, ordinal, err)
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
				return LongDurationProfileMatrixReport{}, fmt.Errorf("validate profile %s candidate %s: %w", profileSpec.ID, candidateSpec.ID, err)
			}
			candidateEvidence = append(candidateEvidence, candidate)
		}
		assetHash, err := transcodelongdrift.CanonicalHash(asset)
		if err != nil {
			return LongDurationProfileMatrixReport{}, err
		}
		profile := transcodelongdrift.ProfileEvidence{
			Profile: profileSpec,
			Source: transcodelongdrift.SourceIdentity{
				CaseID:            asset.CaseID,
				RelativePath:      asset.RelativePath,
				SHA256:            asset.SHA256,
				SizeBytes:         asset.SizeBytes,
				AssetEvidenceHash: assetHash,
			},
			Candidates: candidateEvidence,
			Comparison: transcodelongdrift.BuildCandidateComparison(candidateEvidence[0], candidateEvidence[1]),
		}
		if err := profile.ValidateFor(spec, manifest); err != nil {
			return LongDurationProfileMatrixReport{}, err
		}
		profiles = append(profiles, profile)
	}

	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	contract := transcodelongdrift.ProfileMatrixContract{
		SchemaVersion:                 transcodelongdrift.ProfileMatrixSchemaVersion,
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
		DurationMicros:                transcodelongdrift.DurationMicros,
		CheckpointIntervalMicros:      transcodelongdrift.CheckpointMicros,
		RepeatCount:                   transcodelongdrift.RepeatCount,
		StartToleranceMicros:          transcodelongdrift.StartToleranceMicros,
		EndToleranceMicros:            transcodelongdrift.EndToleranceMicros,
		CheckpointToleranceMicros:     transcodelongdrift.CheckpointToleranceMicros,
		AVSkewToleranceMicros:         transcodelongdrift.AVSkewToleranceMicros,
		RepeatVarianceToleranceMicros: transcodelongdrift.RepeatVarianceToleranceMicros,
		CrossCandidateToleranceMicros: transcodelongdrift.CrossCandidateToleranceMicros,
		Profiles:                      profiles,
		DiscontinuityRequired:         true,
	}
	version, hash, _, err := transcodelongdrift.ProfileMatrixIdentity(contract, spec, manifest)
	if err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	report := LongDurationProfileMatrixReport{
		SchemaVersion:   LongDurationProfileMatrixReportSchemaVersion,
		Spec:            spec,
		Manifest:        manifest,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return LongDurationProfileMatrixReport{}, err
	}
	return report, nil
}

func longDurationProfileSource(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest, caseID string) (transcodecorpus.CaseSpec, transcodecorpus.AssetEvidence, bool) {
	assets := make(map[string]transcodecorpus.AssetEvidence, len(manifest.Assets))
	for _, asset := range manifest.Assets {
		assets[asset.CaseID] = asset
	}
	for _, caseSpec := range spec.Cases {
		if caseSpec.ID == caseID {
			asset, ok := assets[caseSpec.ID]
			return caseSpec, asset, ok
		}
	}
	return transcodecorpus.CaseSpec{}, transcodecorpus.AssetEvidence{}, false
}

func (r LongDurationProfileMatrixReport) Validate() error {
	if r.SchemaVersion != LongDurationProfileMatrixReportSchemaVersion {
		return fmt.Errorf("unsupported long-duration profile matrix report schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.ValidateFor(r.Spec, r.Manifest); err != nil {
		return err
	}
	version, hash, _, err := transcodelongdrift.ProfileMatrixIdentity(r.Evidence, r.Spec, r.Manifest)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("long-duration profile matrix contract identity is invalid")
	}
	return nil
}

func MarshalLongDurationProfileMatrixReport(report LongDurationProfileMatrixReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
