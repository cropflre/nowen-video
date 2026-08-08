package longdrift

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

const (
	ProfileMatrixSchemaVersion = "long-duration-profile-matrix-evidence-v2"

	ProfileAAC44100CFR = "profile-mp4-aac-44100-cfr-v1"
	ProfileMP4EditList = "profile-mp4-edit-list-v1"
	ProfileMKVVFR      = "profile-mkv-vfr-24-30-v1"
	ProfileMPEGTS      = "profile-mpegts-positive-origin-v1"
	ProfileMKVOpus     = "profile-mkv-opus-v1"
)

// ProfileSpec binds a long-duration certification profile to one immutable
// real-media corpus case and the timing traits that distinguish it from the
// other profiles. The corpus Spec and Manifest remain the authoritative source
// identities; these fields prevent a report from relabelling one case as
// another profile.
type ProfileSpec struct {
	ID                 string `json:"id"`
	SourceCaseID       string `json:"source_case_id"`
	Container          string `json:"container"`
	FrameRateMode      string `json:"frame_rate_mode"`
	AudioCodec         string `json:"audio_codec"`
	AudioSampleRate    int    `json:"audio_sample_rate"`
	BFrames            int    `json:"b_frames"`
	SourceOriginMicros int64  `json:"source_origin_micros"`
	HasEditList        bool   `json:"has_edit_list"`
}

type ProfileEvidence struct {
	Profile    ProfileSpec         `json:"profile"`
	Source     SourceIdentity      `json:"source"`
	Candidates []CandidateEvidence `json:"candidates"`
	Comparison CandidateComparison `json:"comparison"`
}

type ProfileMatrixContract struct {
	SchemaVersion                 string            `json:"schema_version"`
	SpecVersion                   string            `json:"spec_version"`
	SpecHash                      string            `json:"spec_hash"`
	ManifestVersion               string            `json:"manifest_version"`
	ManifestHash                  string            `json:"manifest_hash"`
	SourceGeneratorVersion        string            `json:"source_generator_version"`
	SourceFFmpegVersion           string            `json:"source_ffmpeg_version"`
	SourceFFprobeVersion          string            `json:"source_ffprobe_version"`
	CertificationFFmpegVersion    string            `json:"certification_ffmpeg_version"`
	CertificationFFprobeVersion   string            `json:"certification_ffprobe_version"`
	TimestampPlanVersion          string            `json:"timestamp_plan_version"`
	TimestampPlanHash             string            `json:"timestamp_plan_hash"`
	DurationMicros                int64             `json:"duration_micros"`
	CheckpointIntervalMicros      int64             `json:"checkpoint_interval_micros"`
	RepeatCount                   int               `json:"repeat_count"`
	StartToleranceMicros          int64             `json:"start_tolerance_micros"`
	EndToleranceMicros            int64             `json:"end_tolerance_micros"`
	CheckpointToleranceMicros     int64             `json:"checkpoint_tolerance_micros"`
	AVSkewToleranceMicros         int64             `json:"av_skew_tolerance_micros"`
	RepeatVarianceToleranceMicros int64             `json:"repeat_variance_tolerance_micros"`
	CrossCandidateToleranceMicros int64             `json:"cross_candidate_tolerance_micros"`
	Profiles                      []ProfileEvidence `json:"profiles"`
	SeamlessAllowed               bool              `json:"seamless_allowed"`
	DiscontinuityRequired         bool              `json:"discontinuity_required"`
}

func AvailableProfiles() []ProfileSpec {
	return []ProfileSpec{
		{
			ID:                 ProfileAAC44100CFR,
			SourceCaseID:       transcodecorpus.CaseMP4CFR30AAC44100,
			Container:          transcodecorpus.ContainerMP4,
			FrameRateMode:      transcodecorpus.FrameRateCFR,
			AudioCodec:         transcodecorpus.CodecAAC,
			AudioSampleRate:    44_100,
			BFrames:            3,
			SourceOriginMicros: 0,
			HasEditList:        false,
		},
		{
			ID:                 ProfileMP4EditList,
			SourceCaseID:       transcodecorpus.CaseMP4CFR30000EditList,
			Container:          transcodecorpus.ContainerMP4,
			FrameRateMode:      transcodecorpus.FrameRateCFR,
			AudioCodec:         transcodecorpus.CodecAAC,
			AudioSampleRate:    48_000,
			BFrames:            3,
			SourceOriginMicros: 5_000_000,
			HasEditList:        true,
		},
		{
			ID:                 ProfileMKVVFR,
			SourceCaseID:       transcodecorpus.CaseMKVVFR24To30,
			Container:          transcodecorpus.ContainerMatroska,
			FrameRateMode:      transcodecorpus.FrameRateVFR,
			AudioCodec:         transcodecorpus.CodecAAC,
			AudioSampleRate:    48_000,
			BFrames:            3,
			SourceOriginMicros: 0,
			HasEditList:        false,
		},
		{
			ID:                 ProfileMPEGTS,
			SourceCaseID:       transcodecorpus.CaseTSCFR30B3,
			Container:          transcodecorpus.ContainerMPEGTS,
			FrameRateMode:      transcodecorpus.FrameRateCFR,
			AudioCodec:         transcodecorpus.CodecAAC,
			AudioSampleRate:    48_000,
			BFrames:            3,
			SourceOriginMicros: 1_400_000,
			HasEditList:        false,
		},
		{
			ID:                 ProfileMKVOpus,
			SourceCaseID:       transcodecorpus.CaseMKVCFR25Opus,
			Container:          transcodecorpus.ContainerMatroska,
			FrameRateMode:      transcodecorpus.FrameRateCFR,
			AudioCodec:         transcodecorpus.CodecOpus,
			AudioSampleRate:    48_000,
			BFrames:            2,
			SourceOriginMicros: 0,
			HasEditList:        false,
		},
	}
}

func LookupProfile(id string) (ProfileSpec, bool) {
	for _, profile := range AvailableProfiles() {
		if profile.ID == id {
			return profile, true
		}
	}
	return ProfileSpec{}, false
}

func (c ProfileMatrixContract) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	if c.SchemaVersion != ProfileMatrixSchemaVersion {
		return fmt.Errorf("unsupported long-duration profile matrix schema %q", c.SchemaVersion)
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
		return fmt.Errorf("long-duration profile matrix source identity is invalid")
	}
	if c.SourceGeneratorVersion != manifest.GeneratorVersion || c.SourceFFmpegVersion != manifest.FFmpegVersion || c.SourceFFprobeVersion != manifest.FFprobeVersion {
		return fmt.Errorf("long-duration profile matrix source toolchain differs from manifest")
	}
	if strings.TrimSpace(c.CertificationFFmpegVersion) == "" || strings.TrimSpace(c.CertificationFFprobeVersion) == "" {
		return fmt.Errorf("long-duration profile matrix certification toolchain is incomplete")
	}
	if strings.TrimSpace(c.TimestampPlanVersion) == "" || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("long-duration profile matrix timestamp plan identity is invalid")
	}
	if c.DurationMicros != DurationMicros || c.CheckpointIntervalMicros != CheckpointMicros || c.RepeatCount != RepeatCount ||
		c.StartToleranceMicros != StartToleranceMicros || c.EndToleranceMicros != EndToleranceMicros || c.CheckpointToleranceMicros != CheckpointToleranceMicros ||
		c.AVSkewToleranceMicros != AVSkewToleranceMicros || c.RepeatVarianceToleranceMicros != RepeatVarianceToleranceMicros || c.CrossCandidateToleranceMicros != CrossCandidateToleranceMicros {
		return fmt.Errorf("long-duration profile matrix policy is invalid")
	}
	expectedProfiles := AvailableProfiles()
	if len(c.Profiles) != len(expectedProfiles) {
		return fmt.Errorf("long-duration profile matrix has %d profiles, want %d", len(c.Profiles), len(expectedProfiles))
	}
	for index, profile := range c.Profiles {
		if profile.Profile != expectedProfiles[index] {
			return fmt.Errorf("long-duration profile order or identity is invalid at index %d", index)
		}
		if err := profile.ValidateFor(spec, manifest); err != nil {
			return fmt.Errorf("validate long-duration profile %s: %w", profile.Profile.ID, err)
		}
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("long-duration profile matrix cannot authorize seamless playback")
	}
	return nil
}

func (p ProfileEvidence) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	expected, ok := LookupProfile(p.Profile.ID)
	if !ok || p.Profile != expected {
		return fmt.Errorf("unknown or altered long-duration profile %q", p.Profile.ID)
	}
	caseSpec, asset, ok := findProfileSource(spec, manifest, p.Profile.SourceCaseID)
	if !ok {
		return fmt.Errorf("long-duration profile source %s is missing", p.Profile.SourceCaseID)
	}
	if p.Source.CaseID != p.Profile.SourceCaseID || p.Source.RelativePath != asset.RelativePath || p.Source.SHA256 != asset.SHA256 || p.Source.SizeBytes != asset.SizeBytes {
		return fmt.Errorf("long-duration profile source identity differs from manifest")
	}
	assetHash, err := CanonicalHash(asset)
	if err != nil {
		return err
	}
	if p.Source.AssetEvidenceHash != assetHash || !isSHA256(p.Source.AssetEvidenceHash) {
		return fmt.Errorf("long-duration profile asset identity is invalid")
	}
	if caseSpec.Source.Container != p.Profile.Container ||
		caseSpec.Source.Video.FrameRateMode != p.Profile.FrameRateMode ||
		caseSpec.Source.Video.BFrames != p.Profile.BFrames ||
		caseSpec.Source.Audio.Codec != p.Profile.AudioCodec ||
		caseSpec.Source.Audio.SampleRate != p.Profile.AudioSampleRate ||
		caseSpec.Source.Timeline.OriginMicros != p.Profile.SourceOriginMicros ||
		caseSpec.Source.Timeline.HasEditList != p.Profile.HasEditList {
		return fmt.Errorf("long-duration profile traits differ from corpus spec")
	}
	if len(p.Candidates) != 2 {
		return fmt.Errorf("long-duration profile must contain two candidates")
	}
	wants := []struct {
		id string
		tb string
	}{
		{transcodetimebase.CandidateAVTB, "1/1000000"},
		{transcodetimebase.Candidate90K, "1/90000"},
	}
	for index, candidate := range p.Candidates {
		want := wants[index]
		if candidate.ID != want.id || candidate.EncoderTimeBase != want.tb {
			return fmt.Errorf("long-duration profile candidate order is invalid")
		}
		if err := candidate.Validate(); err != nil {
			return fmt.Errorf("validate candidate %s: %w", candidate.ID, err)
		}
	}
	if p.Comparison != BuildCandidateComparison(p.Candidates[0], p.Candidates[1]) || !p.Comparison.Equivalent {
		return fmt.Errorf("long-duration profile candidates diverged")
	}
	return nil
}

func ProfileMatrixIdentity(contract ProfileMatrixContract, spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) (version, hash, canonical string, err error) {
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

func findProfileSource(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest, caseID string) (transcodecorpus.CaseSpec, transcodecorpus.AssetEvidence, bool) {
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
