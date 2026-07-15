package service

import (
	"sort"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
)

// MixedListFilter 描述电影与剧集合集混合列表的稳定筛选和排序条件。
// 默认值与历史 ListMixed 行为一致：全部类型、按新增时间倒序。
type MixedListFilter struct {
	LibraryID   string
	ContentType string
	Genre       string
	Query       string
	YearFrom    int
	YearTo      int
	Sort        string
	Order       string
}

func (f MixedListFilter) normalized() MixedListFilter {
	f.LibraryID = strings.TrimSpace(f.LibraryID)
	f.ContentType = strings.ToLower(strings.TrimSpace(f.ContentType))
	switch f.ContentType {
	case "movie", "series":
	default:
		f.ContentType = "all"
	}
	f.Genre = strings.TrimSpace(f.Genre)
	f.Query = strings.TrimSpace(f.Query)
	f.Sort = strings.ToLower(strings.TrimSpace(f.Sort))
	switch f.Sort {
	case "title", "year", "rating":
	default:
		f.Sort = "added"
	}
	f.Order = strings.ToLower(strings.TrimSpace(f.Order))
	if f.Order != "asc" {
		f.Order = "desc"
	}
	if f.YearFrom < 0 {
		f.YearFrom = 0
	}
	if f.YearTo < 0 {
		f.YearTo = 0
	}
	if f.YearFrom > 0 && f.YearTo > 0 && f.YearFrom > f.YearTo {
		f.YearFrom, f.YearTo = f.YearTo, f.YearFrom
	}
	return f
}

// ListMixedFiltered 返回服务端稳定分页的混合媒体列表。
// 筛选和排序必须在分页前执行，否则客户端翻页时会出现重复、漏项或空页。
func (s *MediaService) ListMixedFiltered(page, size int, rawFilter MixedListFilter) (*MixedListResult, error) {
	filter := rawFilter.normalized()
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 2000 {
		size = 20
	}

	movies, err := s.mediaRepo.RecentNonEpisodeAll(filter.LibraryID)
	if err != nil {
		return nil, err
	}
	seriesList, err := s.seriesRepo.ListAll(filter.LibraryID)
	if err != nil {
		return nil, err
	}
	seriesList = deduplicateSeriesByTitle(seriesList)

	items := make([]MixedItem, 0, len(movies)+len(seriesList))
	for i := range movies {
		items = append(items, MixedItem{Type: "movie", Media: &movies[i]})
	}
	for i := range seriesList {
		items = append(items, MixedItem{Type: "series", Series: &seriesList[i]})
	}

	items = applyMixedListFilter(items, filter)
	movieCount, seriesCount := countMixedItemTypes(items)
	total := int64(len(items))
	start := (page - 1) * size
	if start >= len(items) {
		return &MixedListResult{
			Items:       []MixedItem{},
			Total:       total,
			MovieCount:  movieCount,
			SeriesCount: seriesCount,
		}, nil
	}
	end := start + size
	if end > len(items) {
		end = len(items)
	}
	return &MixedListResult{
		Items:       items[start:end],
		Total:       total,
		MovieCount:  movieCount,
		SeriesCount: seriesCount,
	}, nil
}

func applyMixedListFilter(items []MixedItem, rawFilter MixedListFilter) []MixedItem {
	filter := rawFilter.normalized()
	filtered := make([]MixedItem, 0, len(items))
	for _, item := range items {
		if !mixedItemMatches(item, filter) {
			continue
		}
		filtered = append(filtered, item)
	}
	sortMixedItemsWithFilter(filtered, filter)
	return filtered
}

func mixedItemMatches(item MixedItem, filter MixedListFilter) bool {
	if filter.ContentType != "all" && item.Type != filter.ContentType {
		return false
	}
	year := mixedItemYear(item)
	if filter.YearFrom > 0 && year < filter.YearFrom {
		return false
	}
	if filter.YearTo > 0 && year > filter.YearTo {
		return false
	}
	if filter.Genre != "" && !containsFold(mixedItemGenres(item), filter.Genre) {
		return false
	}
	if filter.Query != "" {
		searchable := mixedItemTitle(item) + " " + mixedItemOriginalTitle(item)
		if !containsFold(searchable, filter.Query) {
			return false
		}
	}
	return true
}

func sortMixedItemsWithFilter(items []MixedItem, filter MixedListFilter) {
	descending := filter.Order == "desc"
	sort.SliceStable(items, func(i, j int) bool {
		comparison := compareMixedItems(items[i], items[j], filter.Sort)
		if comparison == 0 {
			comparison = strings.Compare(
				strings.ToLower(mixedItemTitle(items[i])),
				strings.ToLower(mixedItemTitle(items[j])),
			)
		}
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
}

func compareMixedItems(left, right MixedItem, sortBy string) int {
	switch sortBy {
	case "title":
		return strings.Compare(
			strings.ToLower(mixedItemTitle(left)),
			strings.ToLower(mixedItemTitle(right)),
		)
	case "year":
		return compareInt(mixedItemYear(left), mixedItemYear(right))
	case "rating":
		return compareFloat(mixedItemRating(left), mixedItemRating(right))
	default:
		return compareTime(mixedItemCreatedAt(left), mixedItemCreatedAt(right))
	}
}

func countMixedItemTypes(items []MixedItem) (movieCount, seriesCount int) {
	for _, item := range items {
		switch item.Type {
		case "movie":
			movieCount++
		case "series":
			seriesCount++
		}
	}
	return movieCount, seriesCount
}

func mixedItemTitle(item MixedItem) string {
	if item.Media != nil {
		return item.Media.Title
	}
	if item.Series != nil {
		return item.Series.Title
	}
	return ""
}

func mixedItemOriginalTitle(item MixedItem) string {
	if item.Media != nil {
		return item.Media.OrigTitle
	}
	if item.Series != nil {
		return item.Series.OrigTitle
	}
	return ""
}

func mixedItemGenres(item MixedItem) string {
	if item.Media != nil {
		return item.Media.Genres
	}
	if item.Series != nil {
		return item.Series.Genres
	}
	return ""
}

func mixedItemYear(item MixedItem) int {
	if item.Media != nil {
		return item.Media.Year
	}
	if item.Series != nil {
		return item.Series.Year
	}
	return 0
}

func mixedItemRating(item MixedItem) float64 {
	if item.Media != nil {
		return item.Media.Rating
	}
	if item.Series != nil {
		return item.Series.Rating
	}
	return 0
}

func mixedItemCreatedAt(item MixedItem) time.Time {
	if item.Media != nil {
		return item.Media.CreatedAt
	}
	if item.Series != nil {
		return item.Series.CreatedAt
	}
	return time.Time{}
}

func containsFold(value, query string) bool {
	return strings.Contains(strings.ToLower(value), strings.ToLower(query))
}

func compareInt(left, right int) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareFloat(left, right float64) int {
	switch {
	case left < right:
		return -1
	case left > right:
		return 1
	default:
		return 0
	}
}

func compareTime(left, right time.Time) int {
	switch {
	case left.Before(right):
		return -1
	case left.After(right):
		return 1
	default:
		return 0
	}
}

var _ = model.Media{}
