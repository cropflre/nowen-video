package service

import (
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/stretchr/testify/require"
)

func TestMixedListFilterNormalizesDefaultsAndYearRange(t *testing.T) {
	filter := (MixedListFilter{
		ContentType: "invalid",
		YearFrom:    2024,
		YearTo:      1990,
		Sort:        "unknown",
		Order:       "sideways",
	}).normalized()

	require.Equal(t, "all", filter.ContentType)
	require.Equal(t, 1990, filter.YearFrom)
	require.Equal(t, 2024, filter.YearTo)
	require.Equal(t, "added", filter.Sort)
	require.Equal(t, "desc", filter.Order)
}

func TestApplyMixedListFilterBeforePagination(t *testing.T) {
	older := time.Date(2020, time.January, 1, 0, 0, 0, 0, time.UTC)
	newer := time.Date(2025, time.January, 1, 0, 0, 0, 0, time.UTC)
	movie := model.Media{
		ID:        "movie-1",
		Title:     "星际远征",
		OrigTitle: "Interstellar Journey",
		Year:      2024,
		Genres:    "科幻,冒险",
		Rating:    8.7,
		CreatedAt: newer,
	}
	drama := model.Series{
		ID:        "series-1",
		Title:     "城市故事",
		Year:      2018,
		Genres:    "剧情",
		Rating:    9.1,
		CreatedAt: older,
	}
	items := []MixedItem{
		{Type: "series", Series: &drama},
		{Type: "movie", Media: &movie},
	}

	filtered := applyMixedListFilter(items, MixedListFilter{
		ContentType: "movie",
		Genre:       "科幻",
		Query:       "interstellar",
		YearFrom:    2020,
		Sort:        "rating",
		Order:       "desc",
	})

	require.Len(t, filtered, 1)
	require.Equal(t, "movie-1", filtered[0].Media.ID)
}

func TestApplyMixedListFilterSortsAcrossMoviesAndSeries(t *testing.T) {
	movie := model.Media{ID: "movie", Title: "Beta", Year: 2020, Rating: 7.5}
	series := model.Series{ID: "series", Title: "Alpha", Year: 2024, Rating: 9.0}
	items := []MixedItem{
		{Type: "movie", Media: &movie},
		{Type: "series", Series: &series},
	}

	byYear := applyMixedListFilter(items, MixedListFilter{Sort: "year", Order: "desc"})
	require.Equal(t, "series", byYear[0].Type)

	byTitle := applyMixedListFilter(items, MixedListFilter{Sort: "title", Order: "asc"})
	require.Equal(t, "series", byTitle[0].Type)

	byRating := applyMixedListFilter(items, MixedListFilter{Sort: "rating", Order: "desc"})
	require.Equal(t, "series", byRating[0].Type)
}

func TestCountMixedItemTypesUsesFilteredResult(t *testing.T) {
	movie := model.Media{ID: "movie", Title: "Movie"}
	series := model.Series{ID: "series", Title: "Series"}
	movieCount, seriesCount := countMixedItemTypes([]MixedItem{
		{Type: "movie", Media: &movie},
		{Type: "series", Series: &series},
	})

	require.Equal(t, 1, movieCount)
	require.Equal(t, 1, seriesCount)
}
