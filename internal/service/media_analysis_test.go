package service

import (
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
