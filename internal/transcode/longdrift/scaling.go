package longdrift

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

const (
	ScalingShardSchemaVersion     = "long-duration-scaling-shard-evidence-v3"
	ScalingAggregateSchemaVersion = "long-duration-scaling-aggregate-evidence-v3"

	ScalingTierBreadth2H = "multi-hour-breadth-2h-v1"
	ScalingTierDepth6H   = "multi-hour-depth-6h-v1"
)

type ScalingTierSpec struct {
	ID                       string   `json:"id"`
	Purpose                  string   `json:"purpose"`
	DurationMicros           int64    `json:"duration_micros"`
	CheckpointIntervalMicros int64    `json:"checkpoint_interval_micros"`
	RepeatCount              int      `json:"repeat_count"`
	ProfileIDs               []string `json:"profile_ids"`
	CandidateIDs             []string `json:"candidate_ids"`
}

func (s ScalingTierSpec) Policy() Policy {
	return Policy{
		DurationMicros:                s.DurationMicros,
		CheckpointIntervalMicros:      s.CheckpointIntervalMicros,
		RepeatCount:                   s.RepeatCount,
		StartToleranceMicros:          StartToleranceMicros,
		EndToleranceMicros:            EndToleranceMicros,
		CheckpointToleranceMicros:     CheckpointToleranceMicros,
		AVSkewToleranceMicros:         AVSkewToleranceMicros,
		RepeatVarianceToleranceMicros: 0,
		CrossCandidateToleranceMicros: CrossCandidateToleranceMicros,
	}
}

type ScalingShardSpec struct {
	ID          string `json:"id"`
	TierID      string `json:"tier_id"`
	ProfileID   string `json:"profile_id"`
	CandidateID string `json:"candidate_id"`
}

type ScalingShardContract struct {
	SchemaVersion                string            `json:"schema_version"`
	SpecVersion                  string            `json:"spec_version"`
	SpecHash                     string            `json:"spec_hash"`
	ManifestVersion              string            `json:"manifest_version"`
	ManifestHash                 string            `json:"manifest_hash"`
	SourceGeneratorVersion       string            `json:"source_generator_version"`
	SourceFFmpegVersion          string            `json:"source_ffmpeg_version"`
	SourceFFprobeVersion         string            `json:"source_ffprobe_version"`
	CertificationFFmpegVersion   string            `json:"certification_ffmpeg_version"`
	CertificationFFprobeVersion  string            `json:"certification_ffprobe_version"`
	TimestampPlanVersion         string            `json:"timestamp_plan_version"`
	TimestampPlanHash            string            `json:"timestamp_plan_hash"`
	Shard                        ScalingShardSpec  `json:"shard"`
	Tier                         ScalingTierSpec   `json:"tier"`
	Profile                      ProfileSpec       `json:"profile"`
	Source                       SourceIdentity    `json:"source"`
	Candidate                    CandidateEvidence `json:"candidate"`
	SeamlessAllowed              bool              `json:"seamless_allowed"`
	DiscontinuityRequired        bool              `json:"discontinuity_required"`
}

type ScalingShardBinding struct {
	ShardID         string               `json:"shard_id"`
	ContractVersion string               `json:"contract_version"`
	ContractHash    string               `json:"contract_hash"`
	Evidence        ScalingShardContract `json:"evidence"`
}

type ScalingComparisonEvidence struct {
	TierID     string              `json:"tier_id"`
	ProfileID  string              `json:"profile_id"`
	Comparison CandidateComparison `json:"comparison"`
}

type ScalingAggregateContract struct {
	SchemaVersion          string                      `json:"schema_version"`
	SpecVersion            string                      `json:"spec_version"`
	SpecHash               string                      `json:"spec_hash"`
	ManifestVersion        string                      `json:"manifest_version"`
	ManifestHash           string                      `json:"manifest_hash"`
	TimestampPlanVersion   string                      `json:"timestamp_plan_version"`
	TimestampPlanHash      string                      `json:"timestamp_plan_hash"`
	Shards                 []ScalingShardBinding        `json:"shards"`
	Comparisons            []ScalingComparisonEvidence `json:"comparisons"`
	SeamlessAllowed        bool                        `json:"seamless_allowed"`
	DiscontinuityRequired  bool                        `json:"discontinuity_required"`
}

func AvailableScalingTiers() []ScalingTierSpec {
	profiles := AvailableProfiles()
	allProfileIDs := make([]string, 0, len(profiles))
	for _, profile := range profiles {
		allProfileIDs = append(allProfileIDs, profile.ID)
	}
	return []ScalingTierSpec{
		{
			ID:                       ScalingTierBreadth2H,
			Purpose:                  "exercise every certified profile for two continuous hours using the canonical AVTB candidate",
			DurationMicros:           2 * 60 * 60 * 1_000_000,
			CheckpointIntervalMicros: 30 * 60 * 1_000_000,
			RepeatCount:              1,
			ProfileIDs:               allProfileIDs,
			CandidateIDs:             []string{transcodetimebase.CandidateAVTB},
		},
		{
			ID:                       ScalingTierDepth6H,
			Purpose:                  "exercise six-hour clock-grid and VFR sentinels with both encoder time-base candidates",
			DurationMicros:           6 * 60 * 60 * 1_000_000,
			CheckpointIntervalMicros: 60 * 60 * 1_000_000,
			RepeatCount:              1,
			ProfileIDs:               []string{ProfileAAC44100CFR, ProfileMKVVFR},
			CandidateIDs:             []string{transcodetimebase.CandidateAVTB, transcodetimebase.Candidate90K},
		},
	}
}

func LookupScalingTier(id string) (ScalingTierSpec, bool) {
	for _, tier := range AvailableScalingTiers() {
		if tier.ID == id {
			return tier, true
		}
	}
	return ScalingTierSpec{}, false
}

func AvailableScalingShards() []ScalingShardSpec {
	var result []ScalingShardSpec
	for _, tier := range AvailableScalingTiers() {
		for _, profileID := range tier.ProfileIDs {
			for _, candidateID := range tier.CandidateIDs {
				result = append(result, ScalingShardSpec{
					ID:          ScalingShardID(tier.ID, profileID, candidateID),
					TierID:      tier.ID,
					ProfileID:   profileID,
					CandidateID: candidateID,
				})
			}
		}
	}
	return result
}

func ScalingShardID(tierID, profileID, candidateID string) string {
	return tierID + "--" + profileID + "--" + candidateID
}

func LookupScalingShard(id string) (ScalingShardSpec, bool) {
	for _, shard := range AvailableScalingShards() {
		if shard.ID == id {
			return shard, true
		}
	}
	return ScalingShardSpec{}, false
}

func ScalingCandidateTimeBase(id string) (string, bool) {
	switch id {
	case transcodetimebase.CandidateAVTB:
		return "1/1000000", true
	case transcodetimebase.Candidate90K:
		return "1/90000", true
	default:
		return "", false
	}
}

func (c ScalingShardContract) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	if c.SchemaVersion != ScalingShardSchemaVersion {
		return fmt.Errorf("unsupported long-duration scaling shard schema %q", c.SchemaVersion)
	}
	if err := manifest.ValidateFor(spec); err != nil {
		return err
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return err
	}
	manifestVersion, manifestHash, _, err := transcodecorpus.ManifestIdentity(manifest, spec)
	if err != nil {
		return err
	}
	if c.SpecVersion != specVersion || c.SpecHash != specHash || c.ManifestVersion != manifestVersion || c.ManifestHash != manifestHash {
		return fmt.Errorf("long-duration scaling shard source identity is invalid")
	}
	if c.SourceGeneratorVersion != manifest.GeneratorVersion || c.SourceFFmpegVersion != manifest.FFmpegVersion || c.SourceFFprobeVersion != manifest.FFprobeVersion {
		return fmt.Errorf("long-duration scaling shard source toolchain differs from manifest")
	}
	if strings.TrimSpace(c.CertificationFFmpegVersion) == "" || strings.TrimSpace(c.CertificationFFprobeVersion) == "" {
		return fmt.Errorf("long-duration scaling certification toolchain is incomplete")
	}
	if strings.TrimSpace(c.TimestampPlanVersion) == "" || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("long-duration scaling timestamp plan identity is invalid")
	}
	expectedShard, ok := LookupScalingShard(c.Shard.ID)
	if !ok || c.Shard != expectedShard {
		return fmt.Errorf("long-duration scaling shard identity is invalid")
	}
	expectedTier, ok := LookupScalingTier(c.Shard.TierID)
	if !ok || !scalingTierEqual(c.Tier, expectedTier) {
		return fmt.Errorf("long-duration scaling tier identity is invalid")
	}
	expectedProfile, ok := LookupProfile(c.Shard.ProfileID)
	if !ok || c.Profile != expectedProfile {
		return fmt.Errorf("long-duration scaling profile identity is invalid")
	}
	if !slices.Contains(c.Tier.ProfileIDs, c.Profile.ID) || !slices.Contains(c.Tier.CandidateIDs, c.Shard.CandidateID) {
		return fmt.Errorf("long-duration scaling shard is outside its tier")
	}
	caseSpec, asset, ok := findProfileSource(spec, manifest, c.Profile.SourceCaseID)
	if !ok {
		return fmt.Errorf("long-duration scaling source %s is missing", c.Profile.SourceCaseID)
	}
	if c.Source.CaseID != c.Profile.SourceCaseID || c.Source.RelativePath != asset.RelativePath || c.Source.SHA256 != asset.SHA256 || c.Source.SizeBytes != asset.SizeBytes {
		return fmt.Errorf("long-duration scaling source identity differs from manifest")
	}
	assetHash, err := CanonicalHash(asset)
	if err != nil {
		return err
	}
	if c.Source.AssetEvidenceHash != assetHash || !isSHA256(c.Source.AssetEvidenceHash) {
		return fmt.Errorf("long-duration scaling asset identity is invalid")
	}
	if caseSpec.Source.Container != c.Profile.Container ||
		caseSpec.Source.Video.FrameRateMode != c.Profile.FrameRateMode ||
		caseSpec.Source.Video.BFrames != c.Profile.BFrames ||
		caseSpec.Source.Audio.Codec != c.Profile.AudioCodec ||
		caseSpec.Source.Audio.SampleRate != c.Profile.AudioSampleRate ||
		caseSpec.Source.Timeline.OriginMicros != c.Profile.SourceOriginMicros ||
		caseSpec.Source.Timeline.HasEditList != c.Profile.HasEditList {
		return fmt.Errorf("long-duration scaling source traits differ from corpus spec")
	}
	timeBase, ok := ScalingCandidateTimeBase(c.Shard.CandidateID)
	if !ok || c.Candidate.ID != c.Shard.CandidateID || c.Candidate.EncoderTimeBase != timeBase {
		return fmt.Errorf("long-duration scaling candidate identity is invalid")
	}
	if err := c.Candidate.ValidateForPolicy(c.Tier.Policy()); err != nil {
		return fmt.Errorf("validate long-duration scaling candidate: %w", err)
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("long-duration scaling shard cannot authorize seamless playback")
	}
	return nil
}

func (c ScalingAggregateContract) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	if c.SchemaVersion != ScalingAggregateSchemaVersion {
		return fmt.Errorf("unsupported long-duration scaling aggregate schema %q", c.SchemaVersion)
	}
	if err := manifest.ValidateFor(spec); err != nil {
		return err
	}
	specVersion, specHash, _, err := transcodecorpus.SpecIdentity(spec)
	if err != nil {
		return err
	}
	manifestVersion, manifestHash, _, err := transcodecorpus.ManifestIdentity(manifest, spec)
	if err != nil {
		return err
	}
	if c.SpecVersion != specVersion || c.SpecHash != specHash || c.ManifestVersion != manifestVersion || c.ManifestHash != manifestHash {
		return fmt.Errorf("long-duration scaling aggregate source identity is invalid")
	}
	if strings.TrimSpace(c.TimestampPlanVersion) == "" || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("long-duration scaling aggregate timestamp identity is invalid")
	}
	expectedShards := AvailableScalingShards()
	if len(c.Shards) != len(expectedShards) {
		return fmt.Errorf("long-duration scaling aggregate has %d shards, want %d", len(c.Shards), len(expectedShards))
	}
	candidates := make(map[string]CandidateEvidence, len(c.Shards))
	for index, binding := range c.Shards {
		expected := expectedShards[index]
		if binding.ShardID != expected.ID || binding.Evidence.Shard != expected {
			return fmt.Errorf("long-duration scaling shard order is invalid at index %d", index)
		}
		if err := binding.Evidence.ValidateFor(spec, manifest); err != nil {
			return fmt.Errorf("validate scaling shard %s: %w", binding.ShardID, err)
		}
		version, hash, _, err := ScalingShardIdentity(binding.Evidence, spec, manifest)
		if err != nil {
			return err
		}
		if binding.ContractVersion != version || binding.ContractHash != hash {
			return fmt.Errorf("long-duration scaling shard contract identity is invalid")
		}
		if binding.Evidence.TimestampPlanVersion != c.TimestampPlanVersion || binding.Evidence.TimestampPlanHash != c.TimestampPlanHash {
			return fmt.Errorf("long-duration scaling shard timestamp identity differs from aggregate")
		}
		candidates[binding.ShardID] = binding.Evidence.Candidate
	}
	expectedComparisons := buildScalingComparisons(candidates)
	if !scalingComparisonsEqual(c.Comparisons, expectedComparisons) {
		return fmt.Errorf("long-duration scaling candidate comparisons are invalid")
	}
	for _, comparison := range c.Comparisons {
		if !comparison.Comparison.Equivalent {
			return fmt.Errorf("long-duration scaling candidates diverged for %s/%s", comparison.TierID, comparison.ProfileID)
		}
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("long-duration scaling aggregate cannot authorize seamless playback")
	}
	return nil
}

func ScalingShardIdentity(contract ScalingShardContract, spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (version, hash, canonical string, err error) {
	if err := contract.ValidateFor(spec, manifest); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(contract)
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256(content)
	return contract.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}

func ScalingAggregateIdentity(contract ScalingAggregateContract, spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (version, hash, canonical string, err error) {
	if err := contract.ValidateFor(spec, manifest); err != nil {
		return "", "", "", err
	}
	content, err := json.Marshal(contract)
	if err != nil {
		return "", "", "", err
	}
	digest := sha256.Sum256(content)
	return contract.SchemaVersion, hex.EncodeToString(digest[:]), string(content), nil
}

func BuildScalingComparisons(bindings []ScalingShardBinding) []ScalingComparisonEvidence {
	candidates := make(map[string]CandidateEvidence, len(bindings))
	for _, binding := range bindings {
		candidates[binding.ShardID] = binding.Evidence.Candidate
	}
	return buildScalingComparisons(candidates)
}

func buildScalingComparisons(candidates map[string]CandidateEvidence) []ScalingComparisonEvidence {
	var result []ScalingComparisonEvidence
	for _, tier := range AvailableScalingTiers() {
		if len(tier.CandidateIDs) != 2 {
			continue
		}
		for _, profileID := range tier.ProfileIDs {
			leftID := ScalingShardID(tier.ID, profileID, tier.CandidateIDs[0])
			rightID := ScalingShardID(tier.ID, profileID, tier.CandidateIDs[1])
			left, leftOK := candidates[leftID]
			right, rightOK := candidates[rightID]
			comparison := CandidateComparison{
				CandidateAID: tier.CandidateIDs[0],
				CandidateBID: tier.CandidateIDs[1],
			}
			if leftOK && rightOK {
				comparison = BuildCandidateComparisonForPolicy(left, right, tier.Policy())
			}
			result = append(result, ScalingComparisonEvidence{
				TierID:     tier.ID,
				ProfileID:  profileID,
				Comparison: comparison,
			})
		}
	}
	return result
}

func scalingTierEqual(left, right ScalingTierSpec) bool {
	return left.ID == right.ID &&
		left.Purpose == right.Purpose &&
		left.DurationMicros == right.DurationMicros &&
		left.CheckpointIntervalMicros == right.CheckpointIntervalMicros &&
		left.RepeatCount == right.RepeatCount &&
		slices.Equal(left.ProfileIDs, right.ProfileIDs) &&
		slices.Equal(left.CandidateIDs, right.CandidateIDs)
}

func scalingComparisonsEqual(left, right []ScalingComparisonEvidence) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}
