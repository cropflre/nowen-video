package service

import (
	"context"
	"time"
)

const playbackPlanningProbeBudget = 5 * time.Second

// PlanPlaybackAuthoritative is the HTTP-facing planner entry point. It keeps
// the fast cached planning path, but verifies an otherwise evidence-free HLS
// decision once before starting an expensive playback session.
func (s *StreamService) PlanPlaybackAuthoritative(mediaID string, caps PlaybackClientCapabilities) (*PlaybackPlan, error) {
	info, err := s.GetMediaPlayInfo(mediaID)
	if err != nil {
		return nil, err
	}
	return s.PlanPlaybackWithInfoAuthoritative(mediaID, info, caps)
}

// PlanPlaybackWithInfoAuthoritative prevents a cold Probe cache from being
// interpreted as proof that a codec/container is unsupported. Direct/remux and
// already-evidenced decisions stay on the zero-extra-I/O fast path. Only an
// otherwise unexplained Session HLS decision gets one bounded authoritative
// Probe before the final plan is returned.
func (s *StreamService) PlanPlaybackWithInfoAuthoritative(mediaID string, info *MediaPlayInfo, caps PlaybackClientCapabilities) (*PlaybackPlan, error) {
	plan, err := s.PlanPlaybackWithInfo(mediaID, info, caps)
	if err != nil || !shouldVerifyPlaybackPlanWithColdProbe(info, plan, caps) {
		return plan, err
	}
	if s == nil || s.mediaRepo == nil || s.execution == nil {
		return markPlaybackProbeUnavailable(plan), nil
	}

	media, findErr := s.mediaRepo.FindByID(mediaID)
	if findErr != nil || media == nil {
		return markPlaybackProbeUnavailable(plan), nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), playbackPlanningProbeBudget)
	defer cancel()
	probe, probeErr := s.execution.ProbeMedia(ctx, media)
	if probeErr != nil || probe == nil {
		return markPlaybackProbeUnavailable(plan), nil
	}

	// Mutate the caller-owned playback info so /info returns the same verified
	// technical decision that the embedded playback plan used.
	applyProbeToPlaybackInfoWithCaps(mediaID, info, probe, caps)
	return s.PlanPlaybackWithInfo(mediaID, info, caps)
}

func shouldVerifyPlaybackPlanWithColdProbe(info *MediaPlayInfo, plan *PlaybackPlan, caps PlaybackClientCapabilities) bool {
	if info == nil || plan == nil {
		return false
	}
	if info.IsSTRM || info.IsPreprocessed || caps.ForceTranscode || caps.MaxBitrate > 0 || !info.PreferDirectPlay {
		return false
	}
	if !plan.SessionRequired || plan.Method != PlaybackMethodTranscode {
		return false
	}
	if plan.SourceTechnical != nil {
		return false
	}
	return plan.ReasonCode == "codec_or_container_unsupported"
}

func markPlaybackProbeUnavailable(plan *PlaybackPlan) *PlaybackPlan {
	if plan == nil || plan.ReasonCode != "codec_or_container_unsupported" {
		return plan
	}
	plan.ReasonCode = "probe_unavailable"
	plan.Reason = "缺少可靠媒体技术信息，保守使用兼容转码"
	return plan
}
