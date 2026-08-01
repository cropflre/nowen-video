package realmediacorpus

import (
	"strings"
	"testing"
)

func TestDefaultSpecIsValidAndCanonical(t *testing.T) {
	spec := DefaultSpec()
	if err := spec.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(spec.Cases) != 6 {
		t.Fatalf("case count = %d, want 6", len(spec.Cases))
	}
	versionA, hashA, canonicalA, err := SpecIdentity(spec)
	if err != nil {
		t.Fatal(err)
	}
	versionB, hashB, canonicalB, err := SpecIdentity(DefaultSpec())
	if err != nil {
		t.Fatal(err)
	}
	if versionA != SpecSchemaVersion || versionA != versionB || hashA != hashB || canonicalA != canonicalB {
		t.Fatal("default corpus spec identity is not deterministic")
	}
	if spec.SeamlessAllowed || !spec.DiscontinuityRequired {
		t.Fatal("default corpus spec is not fail-closed")
	}
}

func TestDefaultSpecCoversContainerAndTimelinePolicies(t *testing.T) {
	spec := DefaultSpec()
	containers := map[string]int{}
	vfr := 0
	editList := 0
	nonZeroOrigin := 0
	opus := 0
	aac44100 := 0
	for _, caseSpec := range spec.Cases {
		containers[caseSpec.Source.Container]++
		if caseSpec.Source.Video.FrameRateMode == FrameRateVFR {
			vfr++
		}
		if caseSpec.Source.Timeline.HasEditList {
			editList++
		}
		if caseSpec.Source.Timeline.OriginMicros != 0 {
			nonZeroOrigin++
		}
		if caseSpec.Source.Audio.Codec == CodecOpus {
			opus++
		}
		if caseSpec.Source.Audio.Codec == CodecAAC && caseSpec.Source.Audio.SampleRate == 44_100 {
			aac44100++
		}
	}
	for _, container := range []string{ContainerMP4, ContainerMatroska, ContainerMPEGTS} {
		if containers[container] == 0 {
			t.Fatalf("container %s is not covered", container)
		}
	}
	if vfr != 1 || editList != 1 || nonZeroOrigin != 2 || opus != 1 || aac44100 != 1 {
		t.Fatalf("unexpected corpus coverage: vfr=%d edit_list=%d non_zero_origin=%d opus=%d aac44100=%d", vfr, editList, nonZeroOrigin, opus, aac44100)
	}
}

func TestManifestBindsSpecAndResolvedAssets(t *testing.T) {
	spec := DefaultSpec()
	manifest := validManifest(t, spec)
	if err := manifest.ValidateFor(spec); err != nil {
		t.Fatal(err)
	}
	version, hash, canonical, err := ManifestIdentity(manifest, spec)
	if err != nil {
		t.Fatal(err)
	}
	if version != ManifestSchemaVersion || len(hash) != 64 || !strings.Contains(canonical, spec.Cases[0].ID) {
		t.Fatal("manifest identity is incomplete")
	}
}

func TestManifestRejectsEscapingAssetPath(t *testing.T) {
	spec := DefaultSpec()
	manifest := validManifest(t, spec)
	manifest.Assets[0].RelativePath = "../outside.mp4"
	if err := manifest.ValidateFor(spec); err == nil || !strings.Contains(err.Error(), "escapes") {
		t.Fatalf("expected escaping path failure, got %v", err)
	}
}

func TestManifestRejectsDifferentSpecIdentity(t *testing.T) {
	spec := DefaultSpec()
	manifest := validManifest(t, spec)
	changed := DefaultSpec()
	changed.Cases[0].Description += " changed"
	if err := manifest.ValidateFor(changed); err == nil || !strings.Contains(err.Error(), "does not bind") {
		t.Fatalf("expected spec binding failure, got %v", err)
	}
}

func TestManifestRejectsNondeterministicRepeatHash(t *testing.T) {
	spec := DefaultSpec()
	manifest := validManifest(t, spec)
	manifest.Assets[0].RepeatSHA256[1] = strings.Repeat("b", 64)
	if err := manifest.ValidateFor(spec); err == nil || !strings.Contains(err.Error(), "not deterministic") {
		t.Fatalf("expected repeat determinism failure, got %v", err)
	}
}

func TestManifestRejectsObservedEditListDrift(t *testing.T) {
	spec := DefaultSpec()
	manifest := validManifest(t, spec)
	manifest.Assets[0].Probe.HasEditList = true
	if err := manifest.ValidateFor(spec); err == nil || !strings.Contains(err.Error(), "edit-list") {
		t.Fatalf("expected edit-list drift failure, got %v", err)
	}
}

func TestCaseRejectsIncompleteEvidenceSet(t *testing.T) {
	caseSpec := DefaultSpec().Cases[0]
	caseSpec.RequiredEvidence = caseSpec.RequiredEvidence[:len(caseSpec.RequiredEvidence)-1]
	if err := caseSpec.Validate(); err == nil || !strings.Contains(err.Error(), "incomplete") {
		t.Fatalf("expected evidence-set failure, got %v", err)
	}
}

func validManifest(t *testing.T, spec Spec) Manifest {
	t.Helper()
	version, hash, _, err := SpecIdentity(spec)
	if err != nil {
		t.Fatal(err)
	}
	assets := make([]AssetEvidence, 0, len(spec.Cases))
	for _, caseSpec := range spec.Cases {
		plan := caseSpec.Source
		fileHash := strings.Repeat("a", 64)
		assets = append(assets, AssetEvidence{
			CaseID:        caseSpec.ID,
			RelativePath:  "assets/" + caseSpec.ID + ".media",
			CommandSHA256: strings.Repeat("c", 64),
			SHA256:        fileHash,
			RepeatSHA256:  []string{fileHash, fileHash},
			SizeBytes:     1024,
			Probe: ProbeEvidence{
				Container:                   plan.Container,
				DurationMicros:              plan.Timeline.DurationMicros,
				StartMicros:                 plan.Timeline.OriginMicros,
				VideoCodec:                  plan.Video.Codec,
				VideoProfile:                plan.Video.Profile,
				PixelFormat:                 plan.Video.PixelFormat,
				Width:                       plan.Video.Width,
				Height:                      plan.Video.Height,
				ColorPrimaries:              plan.Video.ColorPrimaries,
				ColorTransfer:               plan.Video.ColorTransfer,
				ColorMatrix:                 plan.Video.ColorMatrix,
				FrameRateMode:               plan.Video.FrameRateMode,
				ObservedRates:               append([]Rational(nil), plan.Video.FrameRates...),
				VideoTimeBase:               Rational{Numerator: 1, Denominator: 90_000},
				FrameCount:                  expectedFrameCount(plan),
				KeyFrameCount:               2,
				MaxKeyFrameInterval:         plan.Video.GOPSize,
				MaxPresentationReorderDepth: plan.Video.BFrames,
				MaxCompositionOffsetMicros:  100_000,
				AudioCodec:                  plan.Audio.Codec,
				AudioSampleRate:             plan.Audio.SampleRate,
				AudioChannels:               plan.Audio.Channels,
				AudioTrackCount:             plan.Audio.TrackCount,
				AudioTimeBase:               Rational{Numerator: 1, Denominator: int64(plan.Audio.SampleRate)},
				HasBFrameReorder:            plan.Video.BFrames > 0,
				HasEditList:                 plan.Timeline.HasEditList,
			},
		})
	}
	return Manifest{
		SchemaVersion:         ManifestSchemaVersion,
		SpecVersion:           version,
		SpecHash:              hash,
		GeneratorVersion:      "test-generator-v1",
		GenerationRepeatCount: 2,
		FFmpegVersion:         "ffmpeg test",
		FFprobeVersion:        "ffprobe test",
		Assets:                assets,
		SeamlessAllowed:       false,
		DiscontinuityRequired: true,
	}
}
