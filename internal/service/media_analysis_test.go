package service

import (
	"math"
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestMediaAnalysisHeuristicHighlights(t *testing.T) {
	svc := &MediaAnalysisService{}
	media := &model.Media{Duration: 7200, Genres: "动作,犯罪"}
	highlights := svc.heuristicHighlights(media)
	if len(highlights) != 3 {
		t.Fatalf("expected 3 fallback highlights, got %d", len(highlights))
	}
	for _, item := range highlights {
		if item.EndTime <= item.StartTime {
			t.Fatalf("invalid highlight interval: %+v", item)
		}
		if item.AnalysisMethod != "heuristic" {
			t.Fatalf("expected heuristic method, got %q", item.AnalysisMethod)
		}
	}
}

func TestAdaptiveSampleCountIsBounded(t *testing.T) {
	cases := []struct {
		duration float64
		want     int
	}{
		{duration: 0, want: 0},
		{duration: 15 * 60, want: 12},
		{duration: 45 * 60, want: 18},
		{duration: 90 * 60, want: 24},
		{duration: 180 * 60, want: 28},
	}
	for _, tc := range cases {
		if got := adaptiveSampleCount(tc.duration); got != tc.want {
			t.Fatalf("duration %.0f: expected %d samples, got %d", tc.duration, tc.want, got)
		}
	}
}

func TestSparseSampleStartsAvoidOpeningAndCredits(t *testing.T) {
	duration := 7200.0
	window := 6.0
	starts := sparseSampleStarts(duration, window, 24)
	if len(starts) != 24 {
		t.Fatalf("expected 24 starts, got %d", len(starts))
	}
	for i, start := range starts {
		center := start + window/2
		if center < duration*0.05-0.001 || center > duration*0.95+0.001 {
			t.Fatalf("sample %d center %.3f outside 5%%-95%% content range", i, center)
		}
		if start < 0 || start+window > duration+0.001 {
			t.Fatalf("sample %d interval invalid: %.3f", i, start)
		}
		if i > 0 && start <= starts[i-1] {
			t.Fatalf("sample starts must be strictly increasing: %v", starts)
		}
	}
}

func TestParseVolumeDetect(t *testing.T) {
	output := "[Parsed_volumedetect_0] mean_volume: -21.4 dB\n[Parsed_volumedetect_0] max_volume: -2.8 dB\n"
	mean, peak, ok := parseVolumeDetect(output)
	if !ok {
		t.Fatal("expected volume output to parse")
	}
	if math.Abs(mean-(-21.4)) > 0.001 || math.Abs(peak-(-2.8)) > 0.001 {
		t.Fatalf("unexpected parsed values mean=%.2f peak=%.2f", mean, peak)
	}
}

func TestSparseRankingKeepsSpacingAndLimit(t *testing.T) {
	svc := &MediaAnalysisService{}
	media := &model.Media{Duration: 7200, Genres: "动作"}
	samples := make([]sparseSample, 0, 16)
	for i := 0; i < 16; i++ {
		center := 420.0 + float64(i)*180
		samples = append(samples, sparseSample{
			Start: center - 3, Center: center, AudioScore: 6 + float64(i%4),
			Score: 6 + float64(i%4), Method: "sparse_audio_scene",
		})
	}
	highlights := svc.rankSparseHighlights(media, samples)
	if len(highlights) != maxHighlightCount {
		t.Fatalf("expected %d highlights, got %d", maxHighlightCount, len(highlights))
	}
	for i := 1; i < len(highlights); i++ {
		if highlights[i].StartTime-highlights[i-1].StartTime < 45 {
			t.Fatalf("highlights are too close: %+v then %+v", highlights[i-1], highlights[i])
		}
	}
	for _, item := range highlights {
		if item.AnalysisMethod != "sparse_audio_scene" {
			t.Fatalf("expected sparse method, got %q", item.AnalysisMethod)
		}
		if item.EndTime <= item.StartTime || item.EndTime > media.Duration {
			t.Fatalf("invalid interval: %+v", item)
		}
	}
}
