package service

import (
	"context"
	"errors"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ChildCatalog is a query boundary: visibility is applied before counting,
// ordering or pagination. Adult requests continue through the ordinary services.
type ChildCatalog struct{ db *gorm.DB }
type ChildPolicy struct {
	UserID    string
	ChildMode bool
	Libraries []string
	Ratings   []string
}

func NewChildCatalog(db *gorm.DB) *ChildCatalog { return &ChildCatalog{db: db} }
func (s *ChildCatalog) WithContext(ctx context.Context) *ChildCatalog {
	return NewChildCatalog(s.db.WithContext(ctx))
}
func (s *ChildCatalog) Policy(userID string) (ChildPolicy, error) {
	p := ChildPolicy{UserID: userID}
	var user model.User
	if err := s.db.Select("id", "role", "child_mode").First(&user, "id = ?", userID).Error; err != nil {
		return p, err
	}
	p.ChildMode = user.ChildMode
	if !p.ChildMode {
		return p, nil
	}
	if user.Role == "admin" {
		return p, ErrForbidden
	}
	var permission model.UserPermission
	err := s.db.Where("user_id = ?", userID).First(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	for _, id := range strings.Split(permission.AllowedLibraries, ",") {
		if id = strings.TrimSpace(id); id != "" {
			p.Libraries = append(p.Libraries, id)
		}
	}
	max, ok := ratingLevels[permission.MaxRatingLevel]
	if !ok || max > ratingLevels["PG-13"] {
		return p, nil
	}
	for _, rating := range []string{"G", "PG", "PG-13"} {
		if ratingLevels[rating] <= max {
			p.Ratings = append(p.Ratings, rating)
		}
	}
	return p, nil
}
func (s *ChildCatalog) SeriesQuery(p ChildPolicy) *gorm.DB {
	q := s.db.Model(&model.Series{})
	if len(p.Libraries) == 0 || len(p.Ratings) == 0 {
		return q.Where("1 = 0")
	}
	// A show's artwork and synopsis are shared by every episode. Expose the show
	// only after all current episodes have been reviewed, including library scope.
	return q.Where("series.library_id IN ?", p.Libraries).
		Where("EXISTS (SELECT 1 FROM media m WHERE m.series_id = series.id AND m.deleted_at IS NULL)").
		Where(`NOT EXISTS (SELECT 1 FROM media m WHERE m.series_id = series.id AND m.deleted_at IS NULL AND
   (m.library_id NOT IN ? OR NOT EXISTS (SELECT 1 FROM content_ratings cr WHERE cr.media_id = m.id AND cr.level IN ?)))`, p.Libraries, p.Ratings)
}
func (s *ChildCatalog) MediaQuery(p ChildPolicy) *gorm.DB {
	q := s.db.Model(&model.Media{})
	if len(p.Libraries) == 0 || len(p.Ratings) == 0 {
		return q.Where("1 = 0")
	}
	return q.Where("media.library_id IN ?", p.Libraries).
		Where("media.id IN (?)", s.db.Model(&model.ContentRating{}).Select("media_id").Where("level IN ?", p.Ratings)).
		Where("media.media_type <> ? OR media.series_id IN (?)", "episode", s.SeriesQuery(p).Select("series.id"))
}
func (s *ChildCatalog) CheckMedia(p ChildPolicy, id string) error {
	var count int64
	if err := s.MediaQuery(p).Where("media.id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return ErrContentRestricted
	}
	return nil
}
func (s *ChildCatalog) CheckSeries(p ChildPolicy, id string) error {
	var count int64
	if err := s.SeriesQuery(p).Where("series.id = ?", id).Count(&count).Error; err != nil {
		return err
	}
	if count != 1 {
		return ErrContentRestricted
	}
	return nil
}

type ChildCatalogFilter struct {
	Query, LibraryID, Genre, Type, Sort, Order string
	YearMin, YearMax                           int
	MinRating                                  float64
}

func childFilter(q *gorm.DB, table string, f ChildCatalogFilter) *gorm.DB {
	if f.LibraryID != "" {
		q = q.Where(table+".library_id = ?", f.LibraryID)
	}
	if f.Query != "" {
		q = q.Where("("+table+".title LIKE ? OR "+table+".orig_title LIKE ?)", "%"+f.Query+"%", "%"+f.Query+"%")
	}
	if f.Genre != "" {
		q = q.Where(table+".genres LIKE ?", "%"+f.Genre+"%")
	}
	if f.YearMin > 0 {
		q = q.Where(table+".year >= ?", f.YearMin)
	}
	if f.YearMax > 0 {
		q = q.Where(table+".year <= ?", f.YearMax)
	}
	if f.MinRating > 0 {
		q = q.Where(table+".rating >= ?", f.MinRating)
	}
	return q
}
func childSort(f ChildCatalogFilter, prefix string) string {
	field := "created_at"
	switch f.Sort {
	case "title", "title_asc":
		field = "title"
	case "year", "year_desc", "year_asc":
		field = "year"
	case "rating", "rating_desc":
		field = "rating"
	}
	direction := "DESC"
	if f.Order == "asc" || strings.HasSuffix(f.Sort, "_asc") {
		direction = "ASC"
	}
	return prefix + field + " " + direction + ", " + prefix + "id ASC"
}
func (s *ChildCatalog) Media(p ChildPolicy, f ChildCatalogFilter, page, size int) ([]model.Media, int64, error) {
	q := childFilter(s.MediaQuery(p), "media", f)
	if f.Type == "movie" {
		q = q.Where("media.media_type <> ?", "episode")
	}
	if f.Type == "episode" {
		q = q.Where("media.media_type = ?", "episode")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []model.Media{}
	err := q.Order(childSort(f, "media.")).Limit(size).Offset((page - 1) * size).Preload("Series").Find(&items).Error
	return items, total, err
}
func (s *ChildCatalog) Series(p ChildPolicy, f ChildCatalogFilter, page, size int) ([]model.Series, int64, error) {
	q := childFilter(s.SeriesQuery(p), "series", f)
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []model.Series{}
	err := q.Order(childSort(f, "series.")).Limit(size).Offset((page - 1) * size).Find(&items).Error
	return items, total, err
}
func (s *ChildCatalog) Mixed(p ChildPolicy, f ChildCatalogFilter, page, size int) (*MixedListResult, error) {
	movies := childFilter(s.MediaQuery(p), "media", f).Where("media.media_type <> ?", "episode")
	shows := childFilter(s.SeriesQuery(p), "series", f)
	if f.Type == "series" || f.Type == "episode" {
		movies = movies.Where("1 = 0")
	}
	if f.Type == "movie" {
		shows = shows.Where("1 = 0")
	}
	var movieCount, seriesCount int64
	if err := movies.Count(&movieCount).Error; err != nil {
		return nil, err
	}
	if err := shows.Count(&seriesCount).Error; err != nil {
		return nil, err
	}
	projection := "id,title,year,rating,created_at"
	rows := []struct {
		ID   string
		Kind string
	}{}
	err := s.db.Table("(? UNION ALL ?) AS catalog", movies.Select(projection+", 'movie' AS kind"), shows.Select(projection+", 'series' AS kind")).
		Order(childSort(f, "catalog.")).Limit(size).Offset((page - 1) * size).Find(&rows).Error
	if err != nil {
		return nil, err
	}
	result := &MixedListResult{Items: []MixedItem{}, Total: movieCount + seriesCount, MovieCount: int(movieCount), SeriesCount: int(seriesCount)}
	movieIDs, seriesIDs := []string{}, []string{}
	for _, row := range rows {
		if row.Kind == "movie" {
			movieIDs = append(movieIDs, row.ID)
		} else {
			seriesIDs = append(seriesIDs, row.ID)
		}
	}
	media := []model.Media{}
	series := []model.Series{}
	if len(movieIDs) > 0 {
		if err := s.MediaQuery(p).Where("media.id IN ?", movieIDs).Find(&media).Error; err != nil {
			return nil, err
		}
	}
	if len(seriesIDs) > 0 {
		if err := s.SeriesQuery(p).Where("series.id IN ?", seriesIDs).Find(&series).Error; err != nil {
			return nil, err
		}
	}
	mediaMap := map[string]*model.Media{}
	seriesMap := map[string]*model.Series{}
	for i := range media {
		mediaMap[media[i].ID] = &media[i]
	}
	for i := range series {
		seriesMap[series[i].ID] = &series[i]
	}
	for _, row := range rows {
		if row.Kind == "movie" {
			if m := mediaMap[row.ID]; m != nil {
				result.Items = append(result.Items, MixedItem{Type: "movie", Media: m})
			}
		} else if show := seriesMap[row.ID]; show != nil {
			result.Items = append(result.Items, MixedItem{Type: "series", Series: show})
		}
	}
	return result, nil
}
func (s *ChildCatalog) Libraries(p ChildPolicy) ([]model.Library, error) {
	items := []model.Library{}
	if len(p.Libraries) == 0 {
		return items, nil
	}
	err := s.db.Where("id IN ?", p.Libraries).Find(&items).Error
	return items, err
}
func (s *ChildCatalog) History(p ChildPolicy, continuing bool, page, size int) ([]model.WatchHistory, int64, error) {
	q := s.db.Model(&model.WatchHistory{}).Where("user_id = ? AND media_id IN (?)", p.UserID, s.MediaQuery(p).Select("media.id"))
	if continuing {
		q = q.Where("completed = ? AND position > 0", false)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []model.WatchHistory{}
	err := q.Order("updated_at DESC, id ASC").Limit(size).Offset((page - 1) * size).Preload("Media").Preload("Media.Series").Find(&items).Error
	return items, total, err
}
func (s *ChildCatalog) Favorites(p ChildPolicy, page, size int) ([]model.Favorite, int64, error) {
	q := s.db.Model(&model.Favorite{}).Where("user_id = ? AND media_id IN (?)", p.UserID, s.MediaQuery(p).Select("media.id"))
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	items := []model.Favorite{}
	err := q.Order("created_at DESC, id ASC").Limit(size).Offset((page - 1) * size).Preload("Media").Preload("Media.Series").Find(&items).Error
	return items, total, err
}
