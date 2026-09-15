package service

import (
	"testing"

	"github.com/nowen-video/nowen-video/internal/model"
)

func TestApplySeriesResultPersistsTVDBID(t *testing.T) {
	tests := []struct {
		name   string
		result TheTVDBSeries
		want   int
	}{
		{
			name:   "numeric_id",
			result: TheTVDBSeries{ID: 12345},
			want:   12345,
		},
		{
			name:   "string_tvdb_id",
			result: TheTVDBSeries{TVDbID: "67890"},
			want:   67890,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			series := &model.Series{Title: "Example Series"}
			service := &TheTVDBService{}

			service.applySeriesResult(series, &tt.result)

			if series.TVDbID != tt.want {
				t.Fatalf("TVDbID = %d, want %d", series.TVDbID, tt.want)
			}
		})
	}
}
