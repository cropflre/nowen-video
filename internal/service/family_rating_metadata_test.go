package service

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"go.uber.org/zap"
)

func TestMovieRatingMetadataStaysBoundToMatchedTitle(t *testing.T) {
	var detail TMDbMovieDetail
	err := json.Unmarshal([]byte(`{"id":42,"title":"Film","release_dates":{"results":[{"iso_3166_1":"US","release_dates":[{"certification":"PG-13"}]}]}}`), &detail)
	if err != nil {
		t.Fatal(err)
	}
	media := &model.Media{TMDbID: 42, TMDbRatingData: "old", MPAA: "R"}
	(&MetadataService{}).applyMovieDetail(media, &detail)
	if media.TMDbID != 42 || !strings.Contains(media.TMDbRatingData, "PG-13") || media.MPAA != "R" {
		t.Fatalf("unexpected rating data: %+v", media)
	}
	// A temporarily absent appended field does not erase data for the same film.
	(&MetadataService{}).applyMovieDetail(media, &TMDbMovieDetail{ID: 42})
	if !strings.Contains(media.TMDbRatingData, "PG-13") {
		t.Fatal("same-title refresh removed the previous classification")
	}
	// A different identity must never inherit the old film's suggestion.
	(&MetadataService{}).applyMovieDetail(media, &TMDbMovieDetail{ID: 43})
	if media.TMDbRatingData != "" || media.TMDbID != 43 {
		t.Fatal("old classification survived a change of film")
	}
}

func TestSeriesRatingFetchPreservesIdentityOnFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3/tv/12/content_ratings" {
			_, _ = w.Write([]byte(`{"results":[{"iso_3166_1":"US","rating":"TV-14"}]}`))
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()
	cfg := &config.Config{}
	cfg.Secrets.TMDbAPIKey = "test"
	cfg.Secrets.TMDbAPIProxy = server.URL
	svc := &MetadataService{cfg: cfg, client: server.Client(), logger: zap.NewNop().Sugar()}
	series := &model.Series{TMDbID: 12}
	svc.refreshSeriesRating(series, 12)
	if !strings.Contains(series.TMDbRatingData, "TV-14") {
		t.Fatalf("missing source classification: %q", series.TMDbRatingData)
	}
	series.TMDbID = 13
	svc.refreshSeriesRating(series, 12)
	if series.TMDbRatingData != "" {
		t.Fatal("old series classification survived a failed fetch for a different series")
	}
}
