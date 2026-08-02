package certification

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodeencoding "github.com/nowen-video/nowen-video/internal/transcode/encodingplan"
	transcodelongdrift "github.com/nowen-video/nowen-video/internal/transcode/longdrift"
	transcodecandidate "github.com/nowen-video/nowen-video/internal/transcode/realmediacandidate"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
	transcodetimestamp "github.com/nowen-video/nowen-video/internal/transcode/timestampplan"
)

const (
	LongDurationScalingShardReportSchemaVersion     = "ffmpeg-long-duration-scaling-shard-v3"
	LongDurationScalingAggregateReportSchemaVersion = "ffmpeg-long-duration-scaling-aggregate-v3"
)

type LongDurationScalingShardReport struct {
	SchemaVersion   string                                  `json:"schema_version"`
	Spec            transcodecorpus.Spec                    `json:"spec"`
	Manifest        transcodecorpus.Manifest                `json:"manifest"`
	ContractVersion string                                  `json:"contract_version"`
	ContractHash    string                                  `json:"contract_hash"`
	Evidence        transcodelongdrift.ScalingShardContract `json:"evidence"`
}

type LongDurationScalingAggregateReport struct {
	SchemaVersion   string                                      `json:"schema_version"`
	Spec            transcodecorpus.Spec                        `json:"spec"`
	Manifest        transcodecorpus.Manifest                    `json:"manifest"`
	ContractVersion string                                      `json:"contract_version"`
	ContractHash    string                                      `json:"contract_hash"`
	Evidence        transcodelongdrift.ScalingAggregateContract `json:"evidence"`
}

func RunLongDurationScalingShard(
	ctx context.Context,
	config LongDurationDriftConfig,
	shardID string,
) (LongDurationScalingShardReport, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	shard, ok := transcodelongdrift.LookupScalingShard(shardID)
	if !ok {
		return LongDurationScalingShardReport{}, fmt.Errorf("unknown long-duration scaling shard %q", shardID)
	}
	tier, _ := transcodelongdrift.LookupScalingTier(shard.TierID)
	profile, _ := transcodelongdrift.LookupProfile(shard.ProfileID)
	candidateSpec, ok := findScalingCandidate(shard.CandidateID)
	if !ok {
		return LongDurationScalingShardReport{}, fmt.Errorf("unknown encoder time-base candidate %q", shard.CandidateID)
	}

	root, spec, manifest, manifestVersion, manifestHash, err := loadRealMediaCorpus(RealMediaCandidateConfig{
		Config:       config.Config,
		CorpusRoot:   config.CorpusRoot,
		ManifestPath: config.ManifestPath,
	})
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	caseSpec, asset, ok := longDurationProfileSource(spec, manifest, profile.SourceCaseID)
	if !ok {
		return LongDurationScalingShardReport{}, fmt.Errorf("long-duration scaling source %s is missing", profile.SourceCaseID)
	}
	reorderSpec, err := transcodecandidate.CaseSpecFor(caseSpec, asset)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	ffmpegPath, err := resolveExecutable(config.FFmpegPath, "ffmpeg")
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	ffprobePath, err := resolveExecutable(config.FFprobePath, "ffprobe")
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	workDir, cleanup, err := prepareWorkDir(config.Config)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	defer cleanup()
	ffmpegVersion, err := commandVersion(ctx, ffmpegPath)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	ffprobeVersion, err := commandVersion(ctx, ffprobePath)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	timestampPlan := transcodetimestamp.Default()
	timestampVersion, timestampHash, _, err := transcodetimestamp.Identity(timestampPlan)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	policy := tier.Policy()
	runDir := filepath.Join(workDir, "long-duration-scaling", shard.ID)
	run, err := runLongDurationCandidateForPolicy(
		ctx,
		ffmpegPath,
		ffprobePath,
		runDir,
		workDir,
		filepath.Join(root, filepath.FromSlash(asset.RelativePath)),
		timestampPlan,
		reorderSpec,
		candidateSpec,
		policy,
		1,
		"long-duration-scaling-"+shard.ID,
	)
	if err != nil {
		return LongDurationScalingShardReport{}, fmt.Errorf("run long-duration scaling shard %s: %w", shard.ID, err)
	}
	candidate := transcodelongdrift.CandidateEvidence{
		ID:              candidateSpec.ID,
		EncoderTimeBase: candidateSpec.EncoderTimeBase,
		Runs:            []transcodelongdrift.RunEvidence{run},
	}
	candidate.Summary = transcodelongdrift.BuildCandidateSummaryForPolicy(candidate.Runs, policy)
	if err := candidate.ValidateForPolicy(policy); err != nil {
		return LongDurationScalingShardReport{}, err
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	assetHash, err := transcodelongdrift.CanonicalHash(asset)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	contract := transcodelongdrift.ScalingShardContract{
		SchemaVersion:                transcodelongdrift.ScalingShardSchemaVersion,
		SpecVersion:                  specVersion,
		SpecHash:                     specHash,
		ManifestVersion:              manifestVersion,
		ManifestHash:                 manifestHash,
		SourceGeneratorVersion:       manifest.GeneratorVersion,
		SourceFFmpegVersion:          manifest.FFmpegVersion,
		SourceFFprobeVersion:         manifest.FFprobeVersion,
		CertificationFFmpegVersion:   ffmpegVersion,
		CertificationFFprobeVersion:  ffprobeVersion,
		TimestampPlanVersion:         timestampVersion,
		TimestampPlanHash:            timestampHash,
		Shard:                        shard,
		Tier:                         tier,
		Profile:                      profile,
		Source: transcodelongdrift.SourceIdentity{
			CaseID:            asset.CaseID,
			RelativePath:      asset.RelativePath,
			SHA256:            asset.SHA256,
			SizeBytes:         asset.SizeBytes,
			AssetEvidenceHash: assetHash,
		},
		Candidate:             candidate,
		DiscontinuityRequired: true,
	}
	version, hash, _, err := transcodelongdrift.ScalingShardIdentity(contract, spec, manifest)
	if err != nil {
		return LongDurationScalingShardReport{}, err
	}
	report := LongDurationScalingShardReport{
		SchemaVersion:   LongDurationScalingShardReportSchemaVersion,
		Spec:            spec,
		Manifest:        manifest,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return LongDurationScalingShardReport{}, err
	}
	return report, nil
}

func runLongDurationCandidateForPolicy(
	ctx context.Context,
	ffmpegPath,
	ffprobePath,
	runDir,
	workDir,
	sourcePath string,
	timestampPlan transcodetimestamp.Plan,
	caseSpec transcodereorder.CaseSpec,
	candidateSpec transcodetimebase.CandidateSpec,
	policy transcodelongdrift.Policy,
	ordinal int,
	encodingProfileID string,
) (transcodelongdrift.RunEvidence, error) {
	produced, err := produceLongDurationCandidateForPolicy(ctx, ffmpegPath, runDir, sourcePath, timestampPlan, caseSpec, candidateSpec, policy)
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	productionSpec := realMediaSourceOriginSpec(caseSpec)
	encodingPlan := encoderTimeBaseEncodingPlan(productionSpec, candidateSpec)
	encodingPlan.ProfileID = encodingProfileID
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
		return transcodelongdrift.RunEvidence{}, fmt.Errorf("verify long-duration scaling output: %w", err)
	}
	if err := timestampPlan.VerifyObservedStart(0, attestation.First.Timeline.Video.StartMS, attestation.First.Timeline.Audio.StartMS); err != nil {
		return transcodelongdrift.RunEvidence{}, fmt.Errorf("long-duration scaling start normalization: %w", err)
	}
	attestationVersion, attestationHash, _, err := transcodeattestation.Identity(attestation)
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	video, err := probeLongDriftStreamForPolicy(ctx, ffprobePath, produced.Manifest, "v:0", "video", policy)
	if err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	audio, err := probeLongDriftStreamForPolicy(ctx, ffprobePath, produced.Manifest, "a:0", "audio", policy)
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
	if err := run.ValidateForPolicy(ordinal, policy); err != nil {
		return transcodelongdrift.RunEvidence{}, err
	}
	return run, nil
}

func AggregateLongDurationScalingShardReports(
	reports []LongDurationScalingShardReport,
) (LongDurationScalingAggregateReport, error) {
	if len(reports) == 0 {
		return LongDurationScalingAggregateReport{}, fmt.Errorf("no long-duration scaling shard reports were supplied")
	}
	first := reports[0]
	if err := first.Validate(); err != nil {
		return LongDurationScalingAggregateReport{}, err
	}
	byShard := make(map[string]LongDurationScalingShardReport, len(reports))
	for _, report := range reports {
		if err := report.Validate(); err != nil {
			return LongDurationScalingAggregateReport{}, err
		}
		if report.Evidence.SpecHash != first.Evidence.SpecHash || report.Evidence.ManifestHash != first.Evidence.ManifestHash {
			return LongDurationScalingAggregateReport{}, fmt.Errorf("long-duration scaling shard corpus identities differ")
		}
		if _, exists := byShard[report.Evidence.Shard.ID]; exists {
			return LongDurationScalingAggregateReport{}, fmt.Errorf("duplicate long-duration scaling shard %s", report.Evidence.Shard.ID)
		}
		byShard[report.Evidence.Shard.ID] = report
	}
	bindings := make([]transcodelongdrift.ScalingShardBinding, 0, len(transcodelongdrift.AvailableScalingShards()))
	for _, shard := range transcodelongdrift.AvailableScalingShards() {
		report, ok := byShard[shard.ID]
		if !ok {
			return LongDurationScalingAggregateReport{}, fmt.Errorf("missing long-duration scaling shard %s", shard.ID)
		}
		bindings = append(bindings, transcodelongdrift.ScalingShardBinding{
			ShardID:         shard.ID,
			ContractVersion: report.ContractVersion,
			ContractHash:    report.ContractHash,
			Evidence:        report.Evidence,
		})
	}
	contract := transcodelongdrift.ScalingAggregateContract{
		SchemaVersion:         transcodelongdrift.ScalingAggregateSchemaVersion,
		SpecVersion:           first.Evidence.SpecVersion,
		SpecHash:              first.Evidence.SpecHash,
		ManifestVersion:       first.Evidence.ManifestVersion,
		ManifestHash:          first.Evidence.ManifestHash,
		TimestampPlanVersion:  first.Evidence.TimestampPlanVersion,
		TimestampPlanHash:     first.Evidence.TimestampPlanHash,
		Shards:                bindings,
		Comparisons:           transcodelongdrift.BuildScalingComparisons(bindings),
		DiscontinuityRequired: true,
	}
	version, hash, _, err := transcodelongdrift.ScalingAggregateIdentity(contract, first.Spec, first.Manifest)
	if err != nil {
		return LongDurationScalingAggregateReport{}, err
	}
	report := LongDurationScalingAggregateReport{
		SchemaVersion:   LongDurationScalingAggregateReportSchemaVersion,
		Spec:            first.Spec,
		Manifest:        first.Manifest,
		ContractVersion: version,
		ContractHash:    hash,
		Evidence:        contract,
	}
	if err := report.Validate(); err != nil {
		return LongDurationScalingAggregateReport{}, err
	}
	return report, nil
}

func findScalingCandidate(id string) (transcodetimebase.CandidateSpec, bool) {
	for _, candidate := range AvailableEncoderTimeBaseCandidates() {
		if candidate.ID == id {
			return candidate, true
		}
	}
	return transcodetimebase.CandidateSpec{}, false
}

func (r LongDurationScalingShardReport) Validate() error {
	if r.SchemaVersion != LongDurationScalingShardReportSchemaVersion {
		return fmt.Errorf("unsupported long-duration scaling shard report schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.ValidateFor(r.Spec, r.Manifest); err != nil {
		return err
	}
	version, hash, _, err := transcodelongdrift.ScalingShardIdentity(r.Evidence, r.Spec, r.Manifest)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("long-duration scaling shard contract identity is invalid")
	}
	return nil
}

func (r LongDurationScalingAggregateReport) Validate() error {
	if r.SchemaVersion != LongDurationScalingAggregateReportSchemaVersion {
		return fmt.Errorf("unsupported long-duration scaling aggregate report schema %q", r.SchemaVersion)
	}
	if err := r.Evidence.ValidateFor(r.Spec, r.Manifest); err != nil {
		return err
	}
	version, hash, _, err := transcodelongdrift.ScalingAggregateIdentity(r.Evidence, r.Spec, r.Manifest)
	if err != nil {
		return err
	}
	if r.ContractVersion != version || r.ContractHash != hash {
		return fmt.Errorf("long-duration scaling aggregate contract identity is invalid")
	}
	return nil
}

func MarshalLongDurationScalingShardReport(report LongDurationScalingShardReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}

func MarshalLongDurationScalingAggregateReport(report LongDurationScalingAggregateReport) ([]byte, error) {
	if err := report.Validate(); err != nil {
		return nil, err
	}
	content, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(content, '\n'), nil
}
