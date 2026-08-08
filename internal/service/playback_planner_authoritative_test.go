package service

import "testing"

func TestShouldVerifyPlaybackPlanWithColdProbe(t *testing.T) {
	baseInfo := MediaPlayInfo{PreferDirectPlay: true, FileExt: ".mkv"}
	basePlan := PlaybackPlan{
		Method:          PlaybackMethodTranscode,
		ReasonCode:      "codec_or_container_unsupported",
		SessionRequired: true,
	}
	baseCaps := PlaybackClientCapabilities{SupportsDirectPlay: true, SupportsRemux: true}

	if !shouldVerifyPlaybackPlanWithColdProbe(&baseInfo, &basePlan, baseCaps) {
		t.Fatal("an evidence-free compatibility HLS decision must be verified once")
	}

	withEvidence := basePlan
	withEvidence.SourceTechnical = &PlaybackSourceTechnical{VideoCodec: "vc1"}
	if shouldVerifyPlaybackPlanWithColdProbe(&baseInfo, &withEvidence, baseCaps) {
		t.Fatal("a plan backed by a fresh probe must not trigger another cold probe")
	}

	forcedCaps := baseCaps
	forcedCaps.ForceTranscode = true
	if shouldVerifyPlaybackPlanWithColdProbe(&baseInfo, &basePlan, forcedCaps) {
		t.Fatal("explicit client transcode must not trigger planner probing")
	}

	bitrateCaps := baseCaps
	bitrateCaps.MaxBitrate = 2_000_000
	if shouldVerifyPlaybackPlanWithColdProbe(&baseInfo, &basePlan, bitrateCaps) {
		t.Fatal("an explicit bitrate-bounded session must not trigger planner probing")
	}

	systemTranscode := baseInfo
	systemTranscode.PreferDirectPlay = false
	if shouldVerifyPlaybackPlanWithColdProbe(&systemTranscode, &basePlan, baseCaps) {
		t.Fatal("system-preferred transcode must not trigger planner probing")
	}

	strm := baseInfo
	strm.IsSTRM = true
	if shouldVerifyPlaybackPlanWithColdProbe(&strm, &basePlan, baseCaps) {
		t.Fatal("STRM proxy planning must not trigger local media probing")
	}

	directPlan := basePlan
	directPlan.Method = PlaybackMethodDirect
	directPlan.SessionRequired = false
	if shouldVerifyPlaybackPlanWithColdProbe(&baseInfo, &directPlan, baseCaps) {
		t.Fatal("non-session plans must stay on the fast path")
	}

	knownHEVCFailure := basePlan
	knownHEVCFailure.ReasonCode = "client_hevc_unsupported"
	if shouldVerifyPlaybackPlanWithColdProbe(&baseInfo, &knownHEVCFailure, baseCaps) {
		t.Fatal("a known client capability rejection must not trigger probing")
	}
}

func TestMarkPlaybackProbeUnavailableUsesHonestReason(t *testing.T) {
	plan := &PlaybackPlan{
		Method:          PlaybackMethodTranscode,
		ReasonCode:      "codec_or_container_unsupported",
		SessionRequired: true,
	}
	markPlaybackProbeUnavailable(plan)
	if plan.ReasonCode != "probe_unavailable" {
		t.Fatalf("expected probe_unavailable, got %q", plan.ReasonCode)
	}
	if plan.Reason == "" {
		t.Fatal("probe-unavailable fallback must remain explainable")
	}
}
