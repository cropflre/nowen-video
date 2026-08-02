package realmediacandidate

import (
	"encoding/hex"
	"fmt"
	"reflect"
	"strings"

	transcodeattestation "github.com/nowen-video/nowen-video/internal/transcode/attestation"
	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
	transcodereorder "github.com/nowen-video/nowen-video/internal/transcode/reordercandidate"
	transcodetimebase "github.com/nowen-video/nowen-video/internal/transcode/timebasecandidate"
)

func (c Contract) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	if c.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported real-media candidate schema %q", c.SchemaVersion)
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
		return fmt.Errorf("real-media candidate source contract identity is invalid")
	}
	for label, value := range map[string]string{
		"source generator version":      c.SourceGeneratorVersion,
		"source FFmpeg version":         c.SourceFFmpegVersion,
		"source FFprobe version":        c.SourceFFprobeVersion,
		"certification FFmpeg version":  c.CertificationFFmpegVersion,
		"certification FFprobe version": c.CertificationFFprobeVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if c.SourceGeneratorVersion != manifest.GeneratorVersion || c.SourceFFmpegVersion != manifest.FFmpegVersion || c.SourceFFprobeVersion != manifest.FFprobeVersion {
		return fmt.Errorf("real-media candidate source toolchain differs from manifest")
	}
	if c.RepeatCount != RepeatCount ||
		c.PacketOrderComparisonToleranceTicks != PacketOrderComparisonToleranceTicks ||
		c.DecodedFrameComparisonPolicy.Effective() != DecodedFrameComparisonPolicy {
		return fmt.Errorf("real-media candidate repeat or comparison policy is invalid")
	}
	if len(c.Cases) != len(spec.Cases) || len(c.Cases) != len(manifest.Assets) {
		return fmt.Errorf("real-media candidate matrix is incomplete")
	}
	for index, evidence := range c.Cases {
		if err := evidence.ValidateFor(index, spec.Cases[index], manifest.Assets[index]); err != nil {
			return fmt.Errorf("validate real-media candidate case %d: %w", index, err)
		}
		for _, candidate := range evidence.Evidence.Candidates {
			for _, run := range candidate.Runs {
				boundary := run.Base.Boundary
				if boundary.FFmpegVersion != c.CertificationFFmpegVersion || boundary.FFprobeVersion != c.CertificationFFprobeVersion {
					return fmt.Errorf("real-media candidate certification toolchain is inconsistent")
				}
			}
		}
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("real-media candidate evidence cannot authorize seamless playback")
	}
	return nil
}

func (c CaseEvidence) ValidateFor(index int, caseSpec transcodecorpus.CaseSpec, asset transcodecorpus.AssetEvidence) error {
	if c.Source.AssetIndex != index || c.Source.CaseID != asset.CaseID || c.Source.RelativePath != asset.RelativePath ||
		c.Source.SHA256 != asset.SHA256 || c.Source.SizeBytes != asset.SizeBytes {
		return fmt.Errorf("source identity differs from manifest asset")
	}
	assetHash, err := canonicalHash(asset)
	if err != nil {
		return err
	}
	if c.Source.AssetEvidenceHash != assetHash || !isSHA256(c.Source.AssetEvidenceHash) {
		return fmt.Errorf("manifest asset evidence identity is invalid")
	}
	if !reflect.DeepEqual(c.RequiredEvidence, caseSpec.RequiredEvidence) {
		return fmt.Errorf("required evidence set differs from corpus spec")
	}
	expectedCase, err := CaseSpecFor(caseSpec, asset)
	if err != nil {
		return err
	}
	if c.Evidence.Case != expectedCase {
		return fmt.Errorf("candidate case policy differs from corpus source")
	}
	if err := c.Evidence.ValidateWithPacketTolerance(PacketOrderComparisonToleranceTicks); err != nil {
		return err
	}
	base := BaseEvidence(c.Evidence)
	baseVersion, baseHash, _, err := transcodetimebase.SemanticCaseIdentity(base)
	if err != nil {
		return err
	}
	reorderHash, err := canonicalHash(c.Evidence)
	if err != nil {
		return err
	}
	if c.TimeBaseCandidate.Version != baseVersion || c.TimeBaseCandidate.Hash != baseHash || !isSHA256(c.TimeBaseCandidate.Hash) {
		return fmt.Errorf("semantic time-base candidate case identity is invalid")
	}
	if c.ReorderCandidate.Version != transcodereorder.SchemaVersion || c.ReorderCandidate.Hash != reorderHash || !isSHA256(c.ReorderCandidate.Hash) {
		return fmt.Errorf("reorder candidate case identity is invalid")
	}
	if len(c.Evidence.Candidates) != 2 || c.Evidence.Candidates[0].Spec.ID != transcodetimebase.CandidateAVTB || c.Evidence.Candidates[1].Spec.ID != transcodetimebase.Candidate90K {
		return fmt.Errorf("candidate order is invalid")
	}
	if strings.TrimSpace(c.TimestampPlan.Version) == "" || !isSHA256(c.TimestampPlan.Hash) {
		return fmt.Errorf("timestamp plan identity is invalid")
	}
	for _, candidate := range c.Evidence.Candidates {
		for _, run := range candidate.Runs {
			boundary := run.Base.Boundary
			if boundary.TimestampPlanVersion != c.TimestampPlan.Version || boundary.TimestampPlanHash != c.TimestampPlan.Hash {
				return fmt.Errorf("candidate run does not bind the canonical timestamp plan")
			}
			if boundary.StartupAttestationVersion != transcodeattestation.SchemaVersion || !isSHA256(boundary.StartupAttestationHash) ||
				boundary.ContinuationAttestationVersion != transcodeattestation.SchemaVersion || !isSHA256(boundary.ContinuationAttestationHash) {
				return fmt.Errorf("candidate run produced-media attestation identity is invalid")
			}
		}
	}
	return nil
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
