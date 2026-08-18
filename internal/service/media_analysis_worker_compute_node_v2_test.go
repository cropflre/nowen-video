package service

import (
	"testing"
	"time"
)

func TestMediaComputeClaimV2KeepsLegacyCompatibility(t *testing.T) {
	lease := time.Now().Add(time.Minute).Round(time.Millisecond)
	claim := mediaComputeHighlightClaim(&MediaAnalysisWorkerClaim{
		TaskID:         "task-1",
		ClaimToken:     "claim-1",
		MediaID:        "media-1",
		Fingerprint:    "fingerprint-1",
		Duration:       3600,
		StreamURL:      "/api/stream/media-1/direct",
		SampleTimes:    []float64{120, 360},
		MaxHighlights:  8,
		EngineVersion:  3,
		LeaseExpiresAt: lease,
	})
	if claim == nil {
		t.Fatal("V2 claim must not be nil")
	}
	if claim.ProtocolVersion != MediaComputeProtocolVersion {
		t.Fatalf("protocol version = %d", claim.ProtocolVersion)
	}
	if claim.JobType != MediaComputeJobHighlightV1 || claim.RequiredCapability != MediaComputeCapabilityHighlightV1 {
		t.Fatalf("unexpected job envelope: job=%q capability=%q", claim.JobType, claim.RequiredCapability)
	}
	if claim.Input.MediaID != claim.MediaID || claim.Input.StreamURL != claim.StreamURL {
		t.Fatal("V2 input and V1 flattened compatibility fields must stay equivalent")
	}
	if len(claim.Input.SampleTimes) != 2 || len(claim.SampleTimes) != 2 {
		t.Fatal("sampling plan must be preserved in both V2 and compatibility fields")
	}
	if !claim.LeaseExpiresAt.Equal(lease) {
		t.Fatalf("lease = %v, want %v", claim.LeaseExpiresAt, lease)
	}
}

func TestMediaComputeClientProtocolVersion(t *testing.T) {
	cases := map[string]int{
		"desktop-v2/dev": MediaComputeProtocolVersion,
		"android-v2/16":  MediaComputeProtocolVersion,
		"v2/custom":      MediaComputeProtocolVersion,
		"desktop-v1/dev": 1,
		"android-v1/15":  1,
		"":               1,
	}
	for input, want := range cases {
		if got := mediaComputeClientProtocolVersion(input); got != want {
			t.Fatalf("mediaComputeClientProtocolVersion(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestMediaComputeNodeSupportsCapability(t *testing.T) {
	heartbeat := MediaAnalysisWorkerHeartbeat{Capabilities: []string{" highlight_v1 ", "waveform_v1"}}
	if !mediaComputeNodeSupportsCapability(heartbeat, "HIGHLIGHT_V1") {
		t.Fatal("capability matching should be case-insensitive and trim whitespace")
	}
	if !mediaComputeNodeSupportsCapability(heartbeat, "waveform_v1") {
		t.Fatal("declared future capabilities should be discoverable independently")
	}
	if mediaComputeNodeSupportsCapability(heartbeat, "subtitle_v1") {
		t.Fatal("undeclared capability must not match")
	}
	if mediaComputeNodeSupportsCapability(heartbeat, "") {
		t.Fatal("empty capability must never match")
	}
}

func TestMediaComputeNodeViewReportsCurrentAdapter(t *testing.T) {
	now := time.Now()
	view := mediaComputeNodeView(MediaAnalysisWorkerView{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID:     "desktop-1",
			Kind:         "desktop",
			Version:      "desktop-v2/dev",
			Capabilities: []string{MediaComputeCapabilityHighlightV1},
		},
		LastSeen: now,
		State:    "busy",
		TaskID:   "task-1",
	})
	if view.ClientProtocolVersion != MediaComputeProtocolVersion {
		t.Fatalf("client protocol = %d", view.ClientProtocolVersion)
	}
	if view.CurrentJobType != MediaComputeJobHighlightV1 {
		t.Fatalf("current job type = %q", view.CurrentJobType)
	}
	if !view.LastSeen.Equal(now) {
		t.Fatal("node last_seen must be preserved")
	}
}
