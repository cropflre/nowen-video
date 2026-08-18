package service

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestPreviewThumbnailFrameTimesStayInsideHighlight(t *testing.T) {
	highlight := model.VideoHighlight{StartTime: 100, EndTime: 130}
	times := previewThumbnailFrameTimes(highlight)
	if len(times) != previewComputeFrameCount {
		t.Fatalf("frame count = %d, want %d", len(times), previewComputeFrameCount)
	}
	for index, point := range times {
		if point < highlight.StartTime || point > highlight.EndTime {
			t.Fatalf("frame %d outside highlight: %.3f", index, point)
		}
		if index > 0 && point <= times[index-1] {
			t.Fatalf("frame times must increase: %#v", times)
		}
	}
	// 30 秒片段应只采样中间约 2.5 秒，而不是解码完整片段。
	if times[len(times)-1]-times[0] > 2.5 {
		t.Fatalf("preview sample window too wide: %#v", times)
	}
}

func TestPreviewThumbnailFrameTimesHandleShortHighlight(t *testing.T) {
	highlight := model.VideoHighlight{StartTime: 4, EndTime: 4.6}
	times := previewThumbnailFrameTimes(highlight)
	if len(times) != previewComputeFrameCount {
		t.Fatalf("short highlight frame count = %d", len(times))
	}
	for _, point := range times {
		if point < highlight.StartTime || point > highlight.EndTime {
			t.Fatalf("short highlight point outside range: %.3f", point)
		}
	}
}

func TestPreviewFrameMagicValidation(t *testing.T) {
	webp := append([]byte("RIFF1234WEBP"), []byte("payload")...)
	jpeg := []byte{0xff, 0xd8, 0xff, 0x01}
	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	if !validPreviewFrameData("image/webp", webp) {
		t.Fatal("valid WebP magic should pass")
	}
	if !validPreviewFrameData("image/jpeg", jpeg) {
		t.Fatal("valid JPEG magic should pass")
	}
	if !validPreviewFrameData("image/png", png) {
		t.Fatal("valid PNG magic should pass")
	}
	if validPreviewFrameData("image/webp", jpeg) || validPreviewFrameData("text/plain", webp) {
		t.Fatal("mismatched or unsupported MIME must be rejected")
	}
}

func TestPreviewComputeIdleNodeRequiresFreshMatchingCapability(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	state := mediaAnalysisState(analysis)
	now := time.Now()
	state.mu.Lock()
	state.workers["desktop-preview"] = MediaAnalysisWorkerView{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-preview", Kind: "desktop",
			Capabilities: []string{MediaComputeCapabilityPreviewThumbnailV1},
		},
		LastSeen: now,
		State:    "idle",
	}
	state.mu.Unlock()
	if !analysis.HasIdleComputeNode(MediaComputeCapabilityPreviewThumbnailV1) {
		t.Fatal("fresh idle desktop with preview capability should be available")
	}
	if analysis.HasIdleComputeNode("waveform_v1") {
		t.Fatal("node must not match undeclared capability")
	}

	state.mu.Lock()
	worker := state.workers["desktop-preview"]
	worker.LastSeen = now.Add(-previewComputeNodeFreshness - time.Second)
	state.workers["desktop-preview"] = worker
	state.mu.Unlock()
	if analysis.HasIdleComputeNode(MediaComputeCapabilityPreviewThumbnailV1) {
		t.Fatal("stale node must not delay interactive preview")
	}
}

func TestPreviewComputeUsesAnalysisExecutionMode(t *testing.T) {
	if !mediaComputeJobUsesAnalysisMode(MediaComputeJobHighlightV1) {
		t.Fatal("highlight job should use media analysis execution mode")
	}
	if !mediaComputeJobUsesAnalysisMode(MediaComputeJobPreviewThumbnailV1) {
		t.Fatal("preview job should use media analysis execution mode")
	}
	if mediaComputeJobUsesAnalysisMode("waveform_v1") {
		t.Fatal("unregistered future job must not inherit preview/highlight mode accidentally")
	}
}
