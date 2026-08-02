package recoverystress

import (
	"fmt"
	"strings"

	transcodecorpus "github.com/nowen-video/nowen-video/internal/transcode/realmediacorpus"
)

func (c AggregateContract) ValidateFor(spec transcodecorpus.Spec, manifest transcodecorpus.Manifest) error {
	if c.SchemaVersion != AggregateSchemaVersion {
		return fmt.Errorf("unsupported recovery stress aggregate schema %q", c.SchemaVersion)
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
		return fmt.Errorf("recovery stress aggregate source identity is invalid")
	}
	if c.SourceGeneratorVersion != manifest.GeneratorVersion || c.SourceFFmpegVersion != manifest.FFmpegVersion || c.SourceFFprobeVersion != manifest.FFprobeVersion {
		return fmt.Errorf("recovery stress aggregate source toolchain differs from manifest")
	}
	if strings.TrimSpace(c.CertificationFFmpegVersion) == "" || strings.TrimSpace(c.CertificationFFprobeVersion) == "" {
		return fmt.Errorf("recovery stress aggregate certification toolchain is incomplete")
	}
	if strings.TrimSpace(c.TimestampPlanVersion) == "" || !isSHA256(c.TimestampPlanHash) {
		return fmt.Errorf("recovery stress aggregate timestamp plan identity is invalid")
	}
	asset, ok := findAsset(manifest, c.Source.CaseID)
	if !ok {
		return fmt.Errorf("recovery stress aggregate source asset is missing")
	}
	assetHash, err := CanonicalHash(asset)
	if err != nil {
		return err
	}
	if c.Source.RelativePath != asset.RelativePath || c.Source.SHA256 != asset.SHA256 || c.Source.SizeBytes != asset.SizeBytes || c.Source.AssetEvidenceHash != assetHash {
		return fmt.Errorf("recovery stress aggregate source identity differs from manifest")
	}
	expected := AvailableScenarios()
	if len(c.Scenarios) != len(expected) {
		return fmt.Errorf("recovery stress aggregate has %d scenarios, want %d", len(c.Scenarios), len(expected))
	}
	totalProcesses := 0
	totalSegments := 0
	maximumRSS := int64(0)
	for index, binding := range c.Scenarios {
		if binding.ScenarioID != expected[index].ID || binding.Evidence.Scenario.ID != expected[index].ID {
			return fmt.Errorf("recovery stress scenario order is invalid at index %d", index)
		}
		if err := binding.Evidence.ValidateFor(spec, manifest); err != nil {
			return fmt.Errorf("validate recovery stress scenario %s: %w", binding.ScenarioID, err)
		}
		version, hash, _, err := ScenarioIdentity(binding.Evidence, spec, manifest)
		if err != nil {
			return err
		}
		if binding.ContractVersion != version || binding.ContractHash != hash {
			return fmt.Errorf("recovery stress scenario contract identity is invalid")
		}
		if binding.Evidence.SourceGeneratorVersion != c.SourceGeneratorVersion || binding.Evidence.SourceFFmpegVersion != c.SourceFFmpegVersion || binding.Evidence.SourceFFprobeVersion != c.SourceFFprobeVersion || binding.Evidence.CertificationFFmpegVersion != c.CertificationFFmpegVersion || binding.Evidence.CertificationFFprobeVersion != c.CertificationFFprobeVersion {
			return fmt.Errorf("recovery stress toolchain identity differs across scenarios")
		}
		if binding.Evidence.TimestampPlanVersion != c.TimestampPlanVersion || binding.Evidence.TimestampPlanHash != c.TimestampPlanHash || binding.Evidence.Source != c.Source {
			return fmt.Errorf("recovery stress source or timestamp identity differs across scenarios")
		}
		totalProcesses += len(binding.Evidence.Processes)
		for _, process := range binding.Evidence.Processes {
			totalSegments += process.SegmentCount
			if process.MaxRSSBytes > maximumRSS {
				maximumRSS = process.MaxRSSBytes
			}
		}
	}
	if c.TotalProcesses != totalProcesses || c.TotalSegmentsObserved != totalSegments || c.MaximumRSSBytes != maximumRSS || !c.AllPassed {
		return fmt.Errorf("recovery stress aggregate summary is invalid")
	}
	if c.SeamlessAllowed || !c.DiscontinuityRequired {
		return fmt.Errorf("recovery stress aggregate cannot authorize seamless playback")
	}
	return nil
}
