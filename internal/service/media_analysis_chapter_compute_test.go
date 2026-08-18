package service

import (
	"errors"
	"math"
	"testing"
)

func TestChapterSampleTimesStayInsideMedia(t *testing.T) {
	for _, duration := range []float64{90, 600, 1800, 3600, 7200, 14400} {
		times := chapterSampleTimes(duration)
		if len(times) == 0 || len(times) > chapterMaxSupportedSamples {
			t.Fatalf("duration %.0f sample count = %d", duration, len(times))
		}
		last := -1.0
		for _, point := range times {
			if point <= 0 || point >= duration || point <= last {
				t.Fatalf("duration %.0f invalid sample plan: %#v", duration, times)
			}
			last = point
		}
	}
}

func TestSelectChapterPointsRespectsScoreSpacingAndLimit(t *testing.T) {
	candidates := []MediaComputeChapterCandidate{
		{Time: 100, Score: 0.20},
		{Time: 140, Score: 0.95},
		{Time: 320, Score: 0.70},
		{Time: 510, Score: 0.80},
		{Time: 700, Score: 0.65},
		{Time: 880, Score: 0.90},
	}
	selected := selectChapterPoints(candidates, 1000, 150, 4)
	if len(selected) != 3 {
		t.Fatalf("selected = %#v", selected)
	}
	for index := 1; index < len(selected); index++ {
		if selected[index].Time-selected[index-1].Time < 150 {
			t.Fatalf("spacing violation: %#v", selected)
		}
	}
	if selected[0].Time != 140 || selected[1].Time != 510 || selected[2].Time != 880 {
		t.Fatalf("unexpected ranking/spacing selection: %#v", selected)
	}
}

func TestBuildVideoChaptersCoversTimelineWithoutGaps(t *testing.T) {
	points := uniformChapterPoints(1200, 6)
	chapters := buildVideoChapters("media-1", 1200, points)
	if len(chapters) < 2 || chapters[0].StartTime != 0 || chapters[len(chapters)-1].EndTime != 1200 {
		t.Fatalf("unexpected chapters: %#v", chapters)
	}
	for index, chapter := range chapters {
		if chapter.MediaID != "media-1" || chapter.Source != "analysis" || chapter.EndTime <= chapter.StartTime {
			t.Fatalf("invalid chapter %d: %#v", index, chapter)
		}
		if index > 0 && math.Abs(chapters[index-1].EndTime-chapter.StartTime) > 0.001 {
			t.Fatalf("chapter timeline gap: %#v", chapters)
		}
	}
}

func TestChapterDetectUsesAnalysisMode(t *testing.T) {
	if !mediaComputeJobUsesAnalysisMode(MediaComputeJobChapterDetectV1) {
		t.Fatal("chapter_detect_v1 must obey media analysis execution mode")
	}
}

func TestChapterDetectClaimRequiresChapterCapability(t *testing.T) {
	analysis := &MediaAnalysisService{}
	defer mediaAnalysisWorkerStates.Delete(analysis)
	defer mediaComputeDescriptorStates.Delete(analysis)
	input := mustMediaComputeJSON(t, MediaComputeChapterDetectInput{
		MediaID: "media-1", Fingerprint: "fp-1", Duration: 1200,
		StreamURL: "/api/stream/media-1/direct", SampleTimes: []float64{120, 240},
		ProbeGapSeconds: 3, MinChapterSeconds: 90, MaxChapters: 5, CaptureWidth: 240, EngineVersion: 1,
	})
	if err := analysis.RegisterComputeTask(MediaComputeTaskRegistration{
		TaskID: "chapter-task", MediaID: "media-1", Fingerprint: "fp-1",
		JobType: MediaComputeJobChapterDetectV1, RequiredCapability: MediaComputeCapabilityChapterDetectV1,
		Input: input,
	}); err != nil {
		t.Fatalf("register chapter task: %v", err)
	}

	_, err := analysis.ClaimComputeTask(MediaAnalysisWorkerClaimRequest{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-preview", Kind: "desktop", Capabilities: []string{MediaComputeCapabilityPreviewThumbnailV1},
		},
	})
	if !errors.Is(err, ErrMediaAnalysisWorkerNoTask) {
		t.Fatalf("wrong capability should not claim chapter task, got %v", err)
	}

	claim, err := analysis.ClaimComputeTask(MediaAnalysisWorkerClaimRequest{
		MediaAnalysisWorkerHeartbeat: MediaAnalysisWorkerHeartbeat{
			WorkerID: "desktop-chapter", Kind: "desktop", Version: "desktop-v2/test",
			Capabilities: []string{MediaComputeCapabilityChapterDetectV1},
		},
	})
	if err != nil {
		t.Fatalf("matching chapter capability should claim: %v", err)
	}
	if claim.JobType != MediaComputeJobChapterDetectV1 || claim.RequiredCapability != MediaComputeCapabilityChapterDetectV1 {
		t.Fatalf("unexpected chapter claim: %#v", claim)
	}
}
