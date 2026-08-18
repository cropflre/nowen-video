package service

import (
	"errors"
	"testing"
)

func TestMediaComputeClaimUsesRequiredCapability(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	defer mediaComputeDescriptorStates.Delete(analysis)
	input := mustMediaComputeJSON(t, map[string]any{"media_id": "media-1", "bins": 128})

	if err := analysis.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: "wave-task", JobType: "waveform_v1", RequiredCapability: "waveform_v1", Input: input,
	}); err != nil {
		t.Fatalf("register task: %v", err)
	}

	_, err := analysis.ClaimComputeTask(MediaAnalysisWorkerClaimRequest{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-thumbnail", Kind: "desktop", Capabilities: []string{"thumbnail_v1"},
		},
	})
	if !errors.Is(err, ErrMediaAnalysisWorkerNoTask) {
		t.Fatalf("wrong capability should not claim task, got %v", err)
	}

	claim, err := analysis.ClaimComputeTask(MediaAnalysisWorkerClaimRequest{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-wave", Kind: "desktop", Version: "desktop-v2/test",
			Capabilities: []string{"waveform_v1"},
		},
	})
	if err != nil {
		t.Fatalf("matching capability should claim task: %v", err)
	}
	if claim.JobType != "waveform_v1" || claim.RequiredCapability != "waveform_v1" {
		t.Fatalf("unexpected claim: %#v", claim)
	}
	if string(claim.Input) != string(input) {
		t.Fatalf("input = %s, want %s", claim.Input, input)
	}
}

func TestPreviewThumbnailJobClaimsOnlyMatchingNode(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	defer mediaComputeDescriptorStates.Delete(analysis)
	input := mustMediaComputeJSON(t, MediaComputePreviewThumbnailInput{
		MediaID: "media-1", HighlightID: "highlight-1", Fingerprint: "fp-1",
		StreamURL: "/api/stream/media-1/direct", FrameTimes: []float64{10.25, 10.75, 11.25, 11.75, 12.25},
		MaxWidth: 420, FrameRate: 2,
	})
	if err := analysis.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: "preview-task", MediaID: "media-1", Fingerprint: "fp-1",
		JobType: MediaComputeJobPreviewThumbnailV1,
		RequiredCapability: MediaComputeCapabilityPreviewThumbnailV1,
		Input: input,
	}); err != nil {
		t.Fatalf("register preview task: %v", err)
	}

	_, err := analysis.ClaimComputeTask(MediaAnalysisWorkerClaimRequest{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-highlight-only", Kind: "desktop",
			Capabilities: []string{MediaComputeCapabilityHighlightV1}, Version: "desktop-v2/test",
		},
	})
	if !errors.Is(err, ErrMediaAnalysisWorkerNoTask) {
		t.Fatalf("highlight-only node must not claim preview job, got %v", err)
	}

	claim, err := analysis.ClaimComputeTask(MediaAnalysisWorkerClaimRequest{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-preview", Kind: "desktop",
			Capabilities: []string{MediaComputeCapabilityPreviewThumbnailV1}, Version: "desktop-v2/test",
		},
	})
	if err != nil {
		t.Fatalf("preview-capable node should claim job: %v", err)
	}
	if claim.JobType != MediaComputeJobPreviewThumbnailV1 || claim.RequiredCapability != MediaComputeCapabilityPreviewThumbnailV1 {
		t.Fatalf("unexpected preview claim: %#v", claim)
	}
	if string(claim.Input) != string(input) {
		t.Fatalf("preview input = %s, want %s", claim.Input, input)
	}
}

func TestMediaComputeDesktopPreferenceDoesNotBlockDifferentCapability(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	defer mediaComputeDescriptorStates.Delete(analysis)

	if err := analysis.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: "wave-task", JobType: "waveform_v1", RequiredCapability: "waveform_v1",
		Input: mustMediaComputeJSON(t, map[string]any{"bins": 64}),
	}); err != nil {
		t.Fatalf("register task: %v", err)
	}
	analysis.HeartbeatComputeNode(MediaAnalysisWorkerHeartbeat{
		WorkerID: "desktop-thumbnail", Kind: "desktop", Capabilities: []string{"thumbnail_v1"},
	})

	claim, err := analysis.ClaimComputeTask(MediaAnalysisWorkerClaimRequest{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "android-wave", Kind: "android", Network: "wifi", BatteryPercent: 80,
			Capabilities: []string{"waveform_v1"}, Version: "android-v2/test",
		},
	})
	if err != nil {
		t.Fatalf("Android should not be blocked by unrelated desktop capability: %v", err)
	}
	if claim.JobType != "waveform_v1" {
		t.Fatalf("job type = %q", claim.JobType)
	}
}
