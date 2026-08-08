package realmediacorpus

import (
	"encoding/hex"
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"
)

var requiredEvidence = []string{
	EvidenceTimestampPlan,
	EvidenceCadence,
	EvidencePacketOrder,
	EvidenceAttestation,
	EvidenceBoundary,
	EvidenceAVSync,
}

func (s Spec) Validate() error {
	if s.SchemaVersion != SpecSchemaVersion {
		return fmt.Errorf("unsupported real-media corpus spec schema %q", s.SchemaVersion)
	}
	if len(s.Cases) == 0 {
		return fmt.Errorf("real-media corpus has no cases")
	}
	seen := make(map[string]struct{}, len(s.Cases))
	for index, caseSpec := range s.Cases {
		if _, exists := seen[caseSpec.ID]; exists {
			return fmt.Errorf("duplicate real-media corpus case %q", caseSpec.ID)
		}
		seen[caseSpec.ID] = struct{}{}
		if err := caseSpec.Validate(); err != nil {
			return fmt.Errorf("validate real-media corpus case %d: %w", index, err)
		}
	}
	if s.SeamlessAllowed || !s.DiscontinuityRequired {
		return fmt.Errorf("real-media corpus spec cannot authorize seamless playback")
	}
	return nil
}

func (s CaseSpec) Validate() error {
	for label, value := range map[string]string{
		"case ID":     s.ID,
		"description": s.Description,
		"purpose":     s.Purpose,
		"tier":        s.Tier,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if s.Tier != TierDeterministicContainer && s.Tier != TierCuratedExternal {
		return fmt.Errorf("unsupported corpus tier %q", s.Tier)
	}
	if err := s.Source.Validate(); err != nil {
		return err
	}
	if s.BoundaryMicros <= 0 || s.BoundaryMicros >= s.Source.Timeline.DurationMicros {
		return fmt.Errorf("case boundary is outside source duration")
	}
	actual := append([]string(nil), s.RequiredEvidence...)
	expected := append([]string(nil), requiredEvidence...)
	sort.Strings(actual)
	sort.Strings(expected)
	if len(actual) != len(expected) {
		return fmt.Errorf("required evidence set is incomplete")
	}
	for index := range expected {
		if actual[index] != expected[index] {
			return fmt.Errorf("required evidence set is invalid")
		}
		if index > 0 && actual[index] == actual[index-1] {
			return fmt.Errorf("required evidence contains duplicates")
		}
	}
	return nil
}

func (s SourcePlan) Validate() error {
	switch s.Container {
	case ContainerMP4, ContainerMatroska, ContainerMPEGTS:
	default:
		return fmt.Errorf("unsupported source container %q", s.Container)
	}
	if err := s.Video.Validate(); err != nil {
		return err
	}
	if err := s.Audio.Validate(); err != nil {
		return err
	}
	if err := s.Timeline.Validate(); err != nil {
		return err
	}
	if s.Timeline.HasEditList && s.Container != ContainerMP4 {
		return fmt.Errorf("edit-list evidence is only supported for MP4 corpus cases")
	}
	return nil
}

func (v VideoPlan) Validate() error {
	if v.Codec != CodecH264 {
		return fmt.Errorf("unsupported v1 video codec %q", v.Codec)
	}
	for label, value := range map[string]string{
		"video profile":   v.Profile,
		"pixel format":    v.PixelFormat,
		"color primaries": v.ColorPrimaries,
		"color transfer":  v.ColorTransfer,
		"color matrix":    v.ColorMatrix,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if v.Width <= 0 || v.Height <= 0 || v.Width%2 != 0 || v.Height%2 != 0 {
		return fmt.Errorf("video dimensions are invalid")
	}
	if v.FrameRateMode != FrameRateCFR && v.FrameRateMode != FrameRateVFR {
		return fmt.Errorf("unsupported frame-rate mode %q", v.FrameRateMode)
	}
	if len(v.FrameRates) == 0 ||
		(v.FrameRateMode == FrameRateCFR && len(v.FrameRates) != 1) ||
		(v.FrameRateMode == FrameRateVFR && len(v.FrameRates) < 2) {
		return fmt.Errorf("frame-rate registry is inconsistent with mode")
	}
	for _, rate := range v.FrameRates {
		if err := rate.Validate(); err != nil {
			return fmt.Errorf("video frame rate: %w", err)
		}
	}
	if v.GOPSize <= 0 || v.BFrames < 0 || v.ReferenceFrames <= 0 {
		return fmt.Errorf("video GOP and reorder policy is invalid")
	}
	if v.Interlaced || v.HDR {
		return fmt.Errorf("interlaced and HDR sources are outside real-media corpus v1")
	}
	return nil
}

func (a AudioPlan) Validate() error {
	if a.Codec != CodecAAC && a.Codec != CodecOpus {
		return fmt.Errorf("unsupported v1 audio codec %q", a.Codec)
	}
	if a.SampleRate != 44_100 && a.SampleRate != 48_000 {
		return fmt.Errorf("unsupported audio sample rate %d", a.SampleRate)
	}
	if a.Channels <= 0 || strings.TrimSpace(a.Layout) == "" || a.TrackCount <= 0 {
		return fmt.Errorf("audio layout is invalid")
	}
	if a.Codec == CodecOpus && a.SampleRate != 48_000 {
		return fmt.Errorf("Opus corpus sources must use 48 kHz")
	}
	return nil
}

func (t TimelinePlan) Validate() error {
	if t.DurationMicros <= 0 {
		return fmt.Errorf("source duration must be positive")
	}
	if t.Discontinuous {
		return fmt.Errorf("discontinuous sources are outside real-media corpus v1")
	}
	return nil
}

func (r Rational) Validate() error {
	if r.Numerator <= 0 || r.Denominator <= 0 {
		return fmt.Errorf("rational must be positive")
	}
	return nil
}

func (m Manifest) ValidateFor(spec Spec) error {
	if err := spec.Validate(); err != nil {
		return err
	}
	if m.SchemaVersion != ManifestSchemaVersion || m.SpecVersion != SpecSchemaVersion {
		return fmt.Errorf("real-media corpus manifest schema identity is invalid")
	}
	if !isSHA256(m.SpecHash) {
		return fmt.Errorf("real-media corpus spec hash is invalid")
	}
	version, hash, _, err := SpecIdentity(spec)
	if err != nil {
		return err
	}
	if m.SpecVersion != version || m.SpecHash != hash {
		return fmt.Errorf("real-media corpus manifest does not bind the supplied spec")
	}
	if m.GenerationRepeatCount != 2 {
		return fmt.Errorf("deterministic corpus generation requires exactly two repeats")
	}
	for label, value := range map[string]string{
		"generator version": m.GeneratorVersion,
		"FFmpeg version":    m.FFmpegVersion,
		"FFprobe version":   m.FFprobeVersion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", label)
		}
	}
	if len(m.Assets) != len(spec.Cases) {
		return fmt.Errorf("real-media corpus asset count %d differs from case count %d", len(m.Assets), len(spec.Cases))
	}
	for index, asset := range m.Assets {
		caseSpec := spec.Cases[index]
		if asset.CaseID != caseSpec.ID {
			return fmt.Errorf("manifest asset %d is out of canonical case order: got %q want %q", index, asset.CaseID, caseSpec.ID)
		}
		if err := asset.validateFor(caseSpec, m.GenerationRepeatCount); err != nil {
			return fmt.Errorf("validate manifest asset %d: %w", index, err)
		}
	}
	if m.SeamlessAllowed || !m.DiscontinuityRequired {
		return fmt.Errorf("real-media corpus manifest cannot authorize seamless playback")
	}
	return nil
}

func (a AssetEvidence) validateFor(caseSpec CaseSpec, repeatCount int) error {
	if a.CaseID != caseSpec.ID || strings.TrimSpace(a.RelativePath) == "" || filepath.IsAbs(a.RelativePath) {
		return fmt.Errorf("asset identity or relative path is invalid")
	}
	clean := filepath.Clean(a.RelativePath)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("asset path escapes the corpus root")
	}
	if !isSHA256(a.CommandSHA256) || !isSHA256(a.SHA256) || a.SizeBytes <= 0 {
		return fmt.Errorf("asset command or byte identity is invalid")
	}
	if len(a.RepeatSHA256) != repeatCount {
		return fmt.Errorf("asset repeat hash count is invalid")
	}
	for _, hash := range a.RepeatSHA256 {
		if hash != a.SHA256 {
			return fmt.Errorf("asset bytes are not deterministic across repeats")
		}
	}
	return a.Probe.ValidateFor(caseSpec.Source)
}

func (p ProbeEvidence) ValidateFor(plan SourcePlan) error {
	if p.Container != plan.Container || p.VideoCodec != plan.Video.Codec || p.AudioCodec != plan.Audio.Codec {
		return fmt.Errorf("observed container or codec differs from source plan")
	}
	if strings.ToLower(p.VideoProfile) != strings.ToLower(plan.Video.Profile) ||
		p.PixelFormat != plan.Video.PixelFormat ||
		p.Width != plan.Video.Width ||
		p.Height != plan.Video.Height {
		return fmt.Errorf("observed video format differs from source plan")
	}
	if p.ColorPrimaries != plan.Video.ColorPrimaries ||
		p.ColorTransfer != plan.Video.ColorTransfer ||
		p.ColorMatrix != plan.Video.ColorMatrix {
		return fmt.Errorf("observed color metadata differs from source plan")
	}
	if p.FrameRateMode != plan.Video.FrameRateMode || len(p.ObservedRates) != len(plan.Video.FrameRates) {
		return fmt.Errorf("observed frame-rate policy differs from source plan")
	}
	for index := range plan.Video.FrameRates {
		if p.ObservedRates[index] != plan.Video.FrameRates[index] {
			return fmt.Errorf("observed frame rate %d differs from source plan", index)
		}
	}
	if err := p.VideoTimeBase.Validate(); err != nil {
		return fmt.Errorf("observed video time base: %w", err)
	}
	if err := p.AudioTimeBase.Validate(); err != nil {
		return fmt.Errorf("observed audio time base: %w", err)
	}
	if p.AudioSampleRate != plan.Audio.SampleRate ||
		p.AudioChannels != plan.Audio.Channels ||
		p.AudioTrackCount != plan.Audio.TrackCount {
		return fmt.Errorf("observed audio format differs from source plan")
	}
	if p.DurationMicros <= 0 {
		return fmt.Errorf("observed source duration is invalid")
	}
	if delta := abs64(p.DurationMicros - plan.Timeline.DurationMicros); delta > 100_000 {
		return fmt.Errorf("observed duration differs from source plan by %d microseconds", delta)
	}
	startTolerance := frameDurationMicros(plan.Video.FrameRates[0]) + 1_000
	if delta := abs64(p.StartMicros - plan.Timeline.OriginMicros); delta > startTolerance {
		return fmt.Errorf("observed source origin differs from source plan by %d microseconds", delta)
	}
	wantFrameCount := expectedFrameCount(plan)
	if p.FrameCount != wantFrameCount {
		return fmt.Errorf("observed frame count %d differs from planned count %d", p.FrameCount, wantFrameCount)
	}
	if p.KeyFrameCount <= 0 || p.MaxKeyFrameInterval != plan.Video.GOPSize {
		return fmt.Errorf("observed key-frame policy differs from source plan")
	}
	if p.HasBFrameReorder != (plan.Video.BFrames > 0) ||
		p.MaxPresentationReorderDepth != plan.Video.BFrames ||
		p.MaxCompositionOffsetMicros <= 0 {
		return fmt.Errorf("observed B-frame reorder policy differs from source plan")
	}
	if p.HasEditList != plan.Timeline.HasEditList {
		return fmt.Errorf("observed edit-list policy differs from source plan")
	}
	return nil
}

func expectedFrameCount(plan SourcePlan) int {
	if plan.Video.FrameRateMode == FrameRateCFR {
		rate := plan.Video.FrameRates[0]
		return int(math.Ceil(
			float64(plan.Timeline.DurationMicros) * float64(rate.Numerator) /
				(1_000_000 * float64(rate.Denominator)),
		))
	}
	segmentMicros := float64(plan.Timeline.DurationMicros) / float64(len(plan.Video.FrameRates))
	total := 0
	for _, rate := range plan.Video.FrameRates {
		total += int(math.Ceil(
			segmentMicros * float64(rate.Numerator) /
				(1_000_000 * float64(rate.Denominator)),
		))
	}
	return total
}

func frameDurationMicros(rate Rational) int64 {
	return int64(math.Round(1_000_000 * float64(rate.Denominator) / float64(rate.Numerator)))
}

func abs64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

func isSHA256(value string) bool {
	if len(value) != 64 {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}
