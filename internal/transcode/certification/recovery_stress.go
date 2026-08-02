package certification

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodecandidate "github.com/nowen-video/nowen-video/internal/transcode/realmediacandidate"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcoderecovery "github.com/nowen-video/nowen-video/internal/transcode/recoverystress"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const (
	RecoveryStressScenarioReportSchemaVersion  = "ffmpeg-recovery-resource-scenario-v4"
	RecoveryStressAggregateReportSchemaVersion = "ffmpeg-recovery-resource-aggregate-v4"
	recoveryStressProfileID                    = transcodelongdrift.ProfileAAC44100CFR
	recoveryStressPlannerVersion               = "recovery-resource-stress-v4"
	recoveryStressLeaseDuration                = 10 * time.Minute
)

type RecoveryStressConfig struct {
	Config
	CorpusRoot   string
	ManifestPath string
}

type RecoveryStressScenarioReport struct {
	SchemaVersion   string                             `json:"schema_version"`
	Spec            transcodecorpus.Spec               `json:"spec"`
	Manifest        transcodecorpus.Manifest           `json:"manifest"`
	ContractVersion string                             `json:"contract_version"`
	ContractHash    string                             `json:"contract_hash"`
	Evidence        transcoderecovery.ScenarioContract `json:"evidence"`
}

type RecoveryStressAggregateReport struct {
	SchemaVersion   string                              `json:"schema_version"`
	Spec            transcodecorpus.Spec                `json:"spec"`
	Manifest        transcodecorpus.Manifest            `json:"manifest"`
	ContractVersion string                              `json:"contract_version"`
	ContractHash    string                              `json:"contract_hash"`
	Evidence        transcoderecovery.AggregateContract `json:"evidence"`
}

func RunRecoveryStressScenario(ctx context.Context, config RecoveryStressConfig, scenarioID string) (RecoveryStressScenarioReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	scenario, ok := transcoderecovery.LookupScenario(scenarioID)
	if !ok {
		return RecoveryStressScenarioReport{}, fmt.Errorf("unknown recovery stress scenario %q", scenarioID)
	}
	root, spec, manifest, manifestVersion, manifestHash, err := loadRealMediaCorpus(RealMediaCandidateConfig{
		Config:       config.Config,
		CorpusRoot:   config.CorpusRoot,
		ManifestPath: config.ManifestPath,
	})
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	profile, ok := transcodelongdrift.LookupProfile(recoveryStressProfileID)
	if !ok {
		return RecoveryStressScenarioReport{}, fmt.Errorf("recovery stress profile %s is unavailable", recoveryStressProfileID)
	}
	caseSpec, asset, ok := longDurationProfileSource(spec, manifest, profile.SourceCaseID)
	if !ok {
		return RecoveryStressScenarioReport{}, fmt.Errorf("recovery stress source %s is missing", profile.SourceCaseID)
	}
	reorderSpec, err := transcodecandidate.CaseSpecFor(caseSpec, asset)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	candidate, ok := findScalingCandidate(transcodetimebase.CandidateAVTB)
	if !ok {
		return RecoveryStressScenarioReport{}, fmt.Errorf("canonical AVTB candidate is unavailable")
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config.Config)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	defer cleanup()
	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	harness, closeHarness, err := newRecoveryHarness(
		filepath.Join(workDir, "recovery-resource-stress", scenario.ID),
		ffmpegPath,
		filepath.Join(root, filepath.FromSlash(asset.RelativePath)),
		profile,
		reorderSpec,
		candidate,
		timestampPlan,
		scenario,
	)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	defer closeHarness()
	result, err := harness.run(ctx)
	if err != nil {
		return RecoveryStressScenarioReport{}, fmt.Errorf("run recovery stress scenario %s: %w", scenario.ID, err)
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	assetHash, err := transcodelongdrift.CanonicalHash(asset)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	contract := transcoderecovery.ScenarioContract{
		SchemaVersion:               transcoderecovery.ScenarioSchemaVersion,
		SpecVersion:                 specVersion,
		SpecHash:                    specHash,
		ManifestVersion:             manifestVersion,
		ManifestHash:                manifestHash,
		SourceGeneratorVersion:      manifest.GeneratorVersion,
		SourceFFmpegVersion:         manifest.FFmpegVersion,
		SourceFFprobeVersion:        manifest.FFprobeVersion,
		CertificationFFmpegVersion:  ffmpegVersion,
		CertificationFFprobeVersion: ffprobeVersion,
		TimestampPlanVersion:        timestampVersion,
		TimestampPlanHash:           timestampHash,
		Scenario:                    scenario,
		Source: transcodelongdrift.SourceIdentity{
			CaseID:            asset.CaseID,
			RelativePath:      asset.RelativePath,
			SHA256:            asset.SHA256,
			SizeBytes:         asset.SizeBytes,
			AssetEvidenceHash: assetHash,
		},
		Transitions:           result.Transitions,
		Processes:             result.Processes,
		Fence:                 result.Fence,
		Artifact:              result.Artifact,
		ErrorCode:             result.ErrorCode,
		Passed:                true,
		DiscontinuityRequired: true,
	}
	version, hash, _, err := transcoderecovery.ScenarioIdentity(contract, spec, manifest)
	if err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	report := RecoveryStressScenarioReport{
		SchemaVersion:   RecoveryStressScenarioReportSchemaVersion,
		Spec:            spec,
		Manifest:        manifest,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return RecoveryStressScenarioReport{}, err
	}
	return report, nil
}
