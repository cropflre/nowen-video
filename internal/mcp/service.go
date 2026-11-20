package mcp

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/config"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"github.com/nowen-video/nowen-video/internal/service"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Service 是 MCP 工具的业务层：只包确定性的非 LLM 能力
//（TMDb 刮削、本地分级规则、数据库读写），不调用内置 AI。
type Service struct {
	db          *gorm.DB
	media       *repository.MediaRepo
	series      *repository.SeriesRepo
	person      *repository.PersonRepo
	mediaPerson *repository.MediaPersonRepo
	ratings     *repository.ContentRatingRepo
	audit       *repository.AuditLogRepo
	metadata    *service.MetadataService
	cfg         *config.Config
	logger      *zap.SugaredLogger
}

// NewService 组装 MCP 工具依赖。metadata 用于 rescrape_media，复用现有刮削逻辑。
func NewService(repos *repository.Repositories, metadata *service.MetadataService, cfg *config.Config, logger *zap.SugaredLogger) *Service {
	if logger == nil {
		logger = zap.NewNop().Sugar()
	}
	return &Service{
		db:          repos.Media.DB(),
		media:       repos.Media,
		series:      repos.Series,
		person:      repos.Person,
		mediaPerson: repos.MediaPerson,
		ratings:     repos.ContentRating,
		audit:       repos.AuditLog,
		metadata:    metadata,
		cfg:         cfg,
		logger:      logger,
	}
}

// ==================== 入参校验 ====================

func normalizeKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "movie":
		return "movie", nil
	case "series":
		return "series", nil
	default:
		return "", fmt.Errorf("kind 无效：只支持 movie 或 series")
	}
}

func normalizeSearchKind(kind string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "", "all":
		return "all", nil
	case "movie":
		return "movie", nil
	case "series":
		return "series", nil
	default:
		return "", fmt.Errorf("kind 无效：只支持 movie、series 或 all")
	}
}

func normalizePage(page, size int) (int, int) {
	if page < 1 {
		page = 1
	}
	if size <= 0 {
		size = 20
	}
	if size > 100 {
		size = 100
	}
	return page, size
}

func normalizeRatingStatus(status string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "all":
		return "all", nil
	case "confirmed":
		return "confirmed", nil
	case "unconfirmed":
		return "unconfirmed", nil
	default:
		return "", fmt.Errorf("rating_status 无效：只支持 confirmed、unconfirmed 或 all")
	}
}

// escapeLike 转义 LIKE 通配符，避免用户输入的 % / _ 改变匹配语义。
func escapeLike(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return r.Replace(s)
}

// ==================== 输出结构 ====================

// MediaItem 影片/剧集摘要（搜索与清单用）。
type MediaItem struct {
	ID                    string            `json:"id"`
	Kind                  string            `json:"kind"` // movie | series
	Title                 string            `json:"title"`
	Year                  int               `json:"year,omitempty"`
	Genres                string            `json:"genres,omitempty"`
	Rating                float64           `json:"rating,omitempty"` // TMDb 评分 0-10
	MPAA                  string            `json:"mpaa,omitempty"`
	FamilyRating          string            `json:"family_rating"` // '' 表示未确认
	FamilyRatingConfirmed bool              `json:"family_rating_confirmed"`
	ConfirmedEpisodes     int               `json:"confirmed_episodes,omitempty"` // 仅剧集
	TotalEpisodes         int               `json:"total_episodes,omitempty"`     // 仅剧集
	HasTMDbRatingData     bool              `json:"has_tmdb_rating_data"`
	Suggestion            *RatingSuggestion `json:"suggestion"`
	SuggestionNote        string            `json:"suggestion_note,omitempty"`
}

// MediaDetail 影片详情（get_media，kind=movie）。
type MediaDetail struct {
	ID                    string            `json:"id"`
	Kind                  string            `json:"kind"`
	Title                 string            `json:"title"`
	OrigTitle             string            `json:"orig_title,omitempty"`
	Year                  int               `json:"year,omitempty"`
	Genres                string            `json:"genres,omitempty"`
	Overview              string            `json:"overview,omitempty"`
	Rating                float64           `json:"rating,omitempty"`
	RuntimeMinutes        int               `json:"runtime_minutes,omitempty"`
	DurationSeconds       float64           `json:"duration_seconds,omitempty"`
	MPAA                  string            `json:"mpaa,omitempty"`
	TMDbID                int               `json:"tmdb_id,omitempty"`
	TMDbRatingData        string            `json:"tmdb_rating_data,omitempty"`
	ScrapeStatus          string            `json:"scrape_status,omitempty"`
	FilePath              string            `json:"file_path,omitempty"`
	FamilyRating          string            `json:"family_rating"`
	FamilyRatingConfirmed bool              `json:"family_rating_confirmed"`
	Suggestion            *RatingSuggestion `json:"suggestion"`
	SuggestionNote        string            `json:"suggestion_note,omitempty"`
}

// SeriesDetail 剧集详情（get_media，kind=series）。
type SeriesDetail struct {
	ID                string            `json:"id"`
	Kind              string            `json:"kind"`
	Title             string            `json:"title"`
	OrigTitle         string            `json:"orig_title,omitempty"`
	Year              int               `json:"year,omitempty"`
	Genres            string            `json:"genres,omitempty"`
	Overview          string            `json:"overview,omitempty"`
	Rating            float64           `json:"rating,omitempty"`
	TMDbID            int               `json:"tmdb_id,omitempty"`
	TMDbRatingData    string            `json:"tmdb_rating_data,omitempty"`
	ScrapeStatus      string            `json:"scrape_status,omitempty"`
	FolderPath        string            `json:"folder_path,omitempty"`
	SeasonCount       int               `json:"season_count,omitempty"`
	EpisodeCount      int               `json:"episode_count,omitempty"`
	ConfirmedEpisodes int               `json:"confirmed_episodes"`
	TotalEpisodes     int               `json:"total_episodes"`
	Suggestion        *RatingSuggestion `json:"suggestion"`
	SuggestionNote    string            `json:"suggestion_note,omitempty"`
}

// PageResult 分页结果。
type PageResult struct {
	Items    []MediaItem `json:"items"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// ==================== 内部查询 ====================

func (s *Service) ratingLevels(mediaIDs []string) (map[string]string, error) {
	out := map[string]string{}
	if len(mediaIDs) == 0 {
		return out, nil
	}
	var rows []model.ContentRating
	if err := s.db.Where("media_id IN ?", mediaIDs).Find(&rows).Error; err != nil {
		s.logger.Warnf("读取分级失败: %v", err)
		return nil, fmt.Errorf("读取分级失败")
	}
	for _, r := range rows {
		out[r.MediaID] = r.Level
	}
	return out, nil
}

func (s *Service) searchMovies(query, genre, ratingStatus string, page, size int) ([]MediaItem, int64, error) {
	q := s.db.Model(&model.Media{}).Where("media_type = ?", "movie")
	if query != "" {
		like := "%" + escapeLike(query) + "%"
		q = q.Where("(title LIKE ? ESCAPE '\\' OR orig_title LIKE ? ESCAPE '\\' OR genres LIKE ? ESCAPE '\\')", like, like, like)
	}
	if genre != "" {
		q = q.Where("genres LIKE ? ESCAPE '\\'", "%"+escapeLike(genre)+"%")
	}
	switch ratingStatus {
	case "confirmed":
		q = q.Where("EXISTS (SELECT 1 FROM content_ratings WHERE content_ratings.media_id = media.id)")
	case "unconfirmed":
		q = q.Where("NOT EXISTS (SELECT 1 FROM content_ratings WHERE content_ratings.media_id = media.id)")
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		s.logger.Warnf("搜索影片计数失败: %v", err)
		return nil, 0, fmt.Errorf("搜索影片失败")
	}
	var rows []model.Media
	if err := q.Order("rating DESC, created_at DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		s.logger.Warnf("搜索影片失败: %v", err)
		return nil, 0, fmt.Errorf("搜索影片失败")
	}
	ids := make([]string, 0, len(rows))
	for _, m := range rows {
		ids = append(ids, m.ID)
	}
	levels, err := s.ratingLevels(ids)
	if err != nil {
		return nil, 0, err
	}
	items := make([]MediaItem, 0, len(rows))
	for _, m := range rows {
		level := levels[m.ID]
		sug, note := suggestFamilyRating(m.MPAA, m.TMDbRatingData, false)
		items = append(items, MediaItem{
			ID: m.ID, Kind: "movie", Title: m.Title, Year: m.Year, Genres: m.Genres,
			Rating: m.Rating, MPAA: m.MPAA, FamilyRating: level, FamilyRatingConfirmed: level != "",
			HasTMDbRatingData: strings.TrimSpace(m.TMDbRatingData) != "",
			Suggestion: sug, SuggestionNote: note,
		})
	}
	return items, total, nil
}

// episodeCounts 批量取一批剧集的剧集数与已确认分级的剧集数。
func (s *Service) episodeCounts(seriesIDs []string) (map[string][2]int, error) {
	out := map[string][2]int{}
	if len(seriesIDs) == 0 {
		return out, nil
	}
	type row struct {
		SeriesID  string `gorm:"column:series_id"`
		Total     int    `gorm:"column:total"`
		Confirmed int    `gorm:"column:confirmed"`
	}
	var rows []row
	if err := s.db.Raw(
		"SELECT m.series_id AS series_id, COUNT(*) AS total, "+
			"COALESCE(SUM(CASE WHEN cr.id IS NOT NULL THEN 1 ELSE 0 END), 0) AS confirmed "+
			"FROM media m LEFT JOIN content_ratings cr ON cr.media_id = m.id "+
			"WHERE m.series_id IN ? AND m.media_type = 'episode' GROUP BY m.series_id",
		seriesIDs).Scan(&rows).Error; err != nil {
		s.logger.Warnf("读取剧集统计失败: %v", err)
		return nil, fmt.Errorf("读取剧集统计失败")
	}
	for _, r := range rows {
		out[r.SeriesID] = [2]int{r.Total, r.Confirmed}
	}
	return out, nil
}

func (s *Service) searchSeries(query, genre, ratingStatus string, page, size int) ([]MediaItem, int64, error) {
	q := s.db.Model(&model.Series{})
	if query != "" {
		like := "%" + escapeLike(query) + "%"
		q = q.Where("(title LIKE ? ESCAPE '\\' OR orig_title LIKE ? ESCAPE '\\' OR genres LIKE ? ESCAPE '\\')", like, like, like)
	}
	if genre != "" {
		q = q.Where("genres LIKE ? ESCAPE '\\'", "%"+escapeLike(genre)+"%")
	}
	// 剧集的确认状态按剧集算：confirmed = 全部已有剧集都已确认；
	// unconfirmed = 至少有一集未确认。
	const episodeUnconfirmed = "EXISTS (SELECT 1 FROM media m WHERE m.series_id = series.id AND m.media_type = 'episode' AND NOT EXISTS (SELECT 1 FROM content_ratings cr WHERE cr.media_id = m.id))"
	switch ratingStatus {
	case "confirmed":
		q = q.Where("NOT "+episodeUnconfirmed+" AND EXISTS (SELECT 1 FROM media m WHERE m.series_id = series.id AND m.media_type = 'episode')")
	case "unconfirmed":
		q = q.Where(episodeUnconfirmed)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		s.logger.Warnf("搜索剧集计数失败: %v", err)
		return nil, 0, fmt.Errorf("搜索剧集失败")
	}
	var rows []model.Series
	if err := q.Order("rating DESC, created_at DESC").Offset((page - 1) * size).Limit(size).Find(&rows).Error; err != nil {
		s.logger.Warnf("搜索剧集失败: %v", err)
		return nil, 0, fmt.Errorf("搜索剧集失败")
	}
	ids := make([]string, 0, len(rows))
	for _, sr := range rows {
		ids = append(ids, sr.ID)
	}
	counts, err := s.episodeCounts(ids)
	if err != nil {
		return nil, 0, err
	}
	items := make([]MediaItem, 0, len(rows))
	for _, sr := range rows {
		c := counts[sr.ID]
		sug, note := suggestFamilyRating("", sr.TMDbRatingData, true)
		items = append(items, MediaItem{
			ID: sr.ID, Kind: "series", Title: sr.Title, Year: sr.Year, Genres: sr.Genres,
			Rating: sr.Rating, FamilyRating: "", FamilyRatingConfirmed: c[0] > 0 && c[0] == c[1],
			ConfirmedEpisodes: c[1], TotalEpisodes: c[0],
			HasTMDbRatingData: strings.TrimSpace(sr.TMDbRatingData) != "",
			Suggestion: sug, SuggestionNote: note,
		})
	}
	return items, total, nil
}

// ==================== 读工具 ====================

// SearchMedia 搜索影片/剧集：关键词、类型、分级确认状态过滤、分页。
func (s *Service) SearchMedia(query, genre, ratingStatus, kind string, page, size int) (*PageResult, error) {
	status, err := normalizeRatingStatus(ratingStatus)
	if err != nil {
		return nil, err
	}
	k, err := normalizeSearchKind(kind)
	if err != nil {
		return nil, err
	}
	page, size = normalizePage(page, size)
	items := []MediaItem{}
	var total int64
	switch k {
	case "movie":
		items, total, err = s.searchMovies(query, genre, status, page, size)
	case "series":
		items, total, err = s.searchSeries(query, genre, status, page, size)
	case "all":
		// all 合并两类各自的一页（两类分页口径不同，分开查再合并）。
		var mItems, sItems []MediaItem
		var mTotal, sTotal int64
		mItems, mTotal, err = s.searchMovies(query, genre, status, page, size)
		if err != nil {
			return nil, err
		}
		sItems, sTotal, err = s.searchSeries(query, genre, status, page, size)
		if err != nil {
			return nil, err
		}
		items = append(mItems, sItems...)
		total = mTotal + sTotal
	}
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []MediaItem{}
	}
	return &PageResult{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// GetMedia 取一部影片或剧集的详情（含分级、确认状态、分级建议）。
func (s *Service) GetMedia(kind, id string) (any, error) {
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	if k == "movie" {
		return s.getMovieDetail(id)
	}
	return s.getSeriesDetail(id)
}

func (s *Service) getMovieDetail(id string) (*MediaDetail, error) {
	m, err := s.media.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到影片：%s", id)
		}
		s.logger.Warnf("读取影片失败: %v", err)
		return nil, fmt.Errorf("读取影片失败")
	}
	level := ""
	if r, err := s.ratings.FindByMediaID(id); err == nil {
		level = r.Level
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		s.logger.Warnf("读取分级失败: %v", err)
		return nil, fmt.Errorf("读取分级失败")
	}
	sug, note := suggestFamilyRating(m.MPAA, m.TMDbRatingData, false)
	return &MediaDetail{
		ID: m.ID, Kind: "movie", Title: m.Title, OrigTitle: m.OrigTitle, Year: m.Year,
		Genres: m.Genres, Overview: m.Overview, Rating: m.Rating, RuntimeMinutes: m.Runtime,
		DurationSeconds: m.Duration, MPAA: m.MPAA, TMDbID: m.TMDbID,
		TMDbRatingData: m.TMDbRatingData, ScrapeStatus: m.ScrapeStatus, FilePath: m.FilePath,
		FamilyRating: level, FamilyRatingConfirmed: level != "",
		Suggestion: sug, SuggestionNote: note,
	}, nil
}

func (s *Service) getSeriesDetail(id string) (*SeriesDetail, error) {
	sr, err := s.series.FindByID(id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到剧集：%s", id)
		}
		s.logger.Warnf("读取剧集失败: %v", err)
		return nil, fmt.Errorf("读取剧集失败")
	}
	counts, err := s.episodeCounts([]string{id})
	if err != nil {
		return nil, err
	}
	c := counts[id]
	sug, note := suggestFamilyRating("", sr.TMDbRatingData, true)
	return &SeriesDetail{
		ID: sr.ID, Kind: "series", Title: sr.Title, OrigTitle: sr.OrigTitle, Year: sr.Year,
		Genres: sr.Genres, Overview: sr.Overview, Rating: sr.Rating, TMDbID: sr.TMDbID,
		TMDbRatingData: sr.TMDbRatingData, ScrapeStatus: sr.ScrapeStatus, FolderPath: sr.FolderPath,
		SeasonCount: sr.SeasonCount, EpisodeCount: sr.EpisodeCount,
		ConfirmedEpisodes: c[1], TotalEpisodes: c[0],
		Suggestion: sug, SuggestionNote: note,
	}, nil
}

// ListUnconfirmedRatings 未确认分级的影片/剧集清单。
func (s *Service) ListUnconfirmedRatings(kind string, page, size int) (*PageResult, error) {
	k, err := normalizeSearchKind(kind)
	if err != nil {
		return nil, err
	}
	page, size = normalizePage(page, size)
	items := []MediaItem{}
	var total int64
	switch k {
	case "movie":
		items, total, err = s.searchMovies("", "", "unconfirmed", page, size)
	case "series":
		items, total, err = s.searchSeries("", "", "unconfirmed", page, size)
	case "all":
		var mItems, sItems []MediaItem
		var mTotal, sTotal int64
		mItems, mTotal, err = s.searchMovies("", "", "unconfirmed", page, size)
		if err != nil {
			return nil, err
		}
		sItems, sTotal, err = s.searchSeries("", "", "unconfirmed", page, size)
		if err != nil {
			return nil, err
		}
		items = append(mItems, sItems...)
		total = mTotal + sTotal
	}
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []MediaItem{}
	}
	return &PageResult{Items: items, Total: total, Page: page, PageSize: size}, nil
}

// SuggestFamilyRating 只给分级建议 + 依据，不写库。
func (s *Service) SuggestFamilyRating(kind, id string) (map[string]any, error) {
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	var title, mpaa, tmdbData string
	var isSeries bool
	if k == "movie" {
		m, err := s.media.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到影片：%s", id)
			}
			s.logger.Warnf("读取影片失败: %v", err)
			return nil, fmt.Errorf("读取影片失败")
		}
		title, mpaa, tmdbData = m.Title, m.MPAA, m.TMDbRatingData
	} else {
		sr, err := s.series.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到剧集：%s", id)
			}
			s.logger.Warnf("读取剧集失败: %v", err)
			return nil, fmt.Errorf("读取剧集失败")
		}
		title, tmdbData, isSeries = sr.Title, sr.TMDbRatingData, true
	}
	sug, note := suggestFamilyRating(mpaa, tmdbData, isSeries)
	return map[string]any{
		"id": id, "kind": k, "title": title,
		"suggestion": sug, "note": note,
		"mpaa":                 mpaa,
		"has_tmdb_rating_data": strings.TrimSpace(tmdbData) != "",
	}, nil
}

// ==================== 写工具：审计 ====================

func (s *Service) writeAudit(tx *gorm.DB, action, targetType, targetID string, detail map[string]any) error {
	detailJSON, _ := json.Marshal(detail)
	return tx.Create(&model.AuditLog{
		OperatorID: "mcp",
		Operator:   "mcp",
		Action:     action,
		TargetType: targetType,
		TargetID:   targetID,
		Detail:     string(detailJSON),
		IP:         "stdio",
	}).Error
}

// requireDirectRating 直接写分级的开关门控（默认关闭）。
func (s *Service) requireDirectRating() error {
	if s.cfg != nil && s.cfg.MCP.AllowDirectRating {
		return nil
	}
	return fmt.Errorf("直接写分级未开启：把 mcp.allow_direct_rating 设为 true（config.yaml 或环境变量 NOWEN_MCP_ALLOW_DIRECT_RATING=true）后重启 server-lite 再试；未开启时请用 suggest_family_rating 给建议，由家长在管理端确认")
}

// setFamilyRatingTx 在事务内写确认分级：与管理端 SetFamilyRating 同款语义，
// 先删该目标的全部旧确认，再按 level 写入（level 为 '' 时只删不写，即撤回）。
// 剧集作用于当前全部剧集；新入库的剧集仍需再次确认。
func (s *Service) setFamilyRatingTx(tx *gorm.DB, kind, id, level string, action string) (map[string]any, error) {
	var ids []string
	var targetType string
	var before any
	if kind == "movie" {
		targetType = "media"
		ids = []string{id}
		// 注意：必须用 tx 读。连接池只有 1 个连接时，用仓库的 db 读会等事务
		// 释放连接而死锁。
		var prev model.ContentRating
		prevLevel := ""
		if err := tx.Where("media_id = ?", id).First(&prev).Error; err == nil {
			prevLevel = prev.Level
		}
		before = prevLevel
	} else {
		targetType = "series"
		var episodes []model.Media
		if err := tx.Where("series_id = ? AND media_type = 'episode'", id).Find(&episodes).Error; err != nil {
			s.logger.Warnf("读取剧集失败: %v", err)
			return nil, fmt.Errorf("读取剧集失败")
		}
		if len(episodes) == 0 {
			return nil, fmt.Errorf("该剧集暂无剧集，无法确认分级")
		}
		for _, ep := range episodes {
			ids = append(ids, ep.ID)
		}
		var prevConfirmed int64
		if err := tx.Model(&model.ContentRating{}).Where("media_id IN ?", ids).Count(&prevConfirmed).Error; err != nil {
			return nil, fmt.Errorf("读取分级失败")
		}
		before = map[string]any{"confirmed_episodes": prevConfirmed, "total_episodes": len(ids)}
	}
	if err := tx.Where("media_id IN ?", ids).Delete(&model.ContentRating{}).Error; err != nil {
		return nil, fmt.Errorf("保存分级失败")
	}
	if level != "" {
		ratings := make([]model.ContentRating, 0, len(ids))
		for _, mid := range ids {
			ratings = append(ratings, model.ContentRating{MediaID: mid, Level: level})
		}
		if err := tx.CreateInBatches(ratings, 100).Error; err != nil {
			return nil, fmt.Errorf("保存分级失败")
		}
	}
	detail := map[string]any{
		"kind": kind, "id": id, "level": level,
		"before": before, "affected": len(ids),
	}
	if err := s.writeAudit(tx, action, targetType, id, detail); err != nil {
		return nil, fmt.Errorf("写审计失败")
	}
	return map[string]any{
		"kind": kind, "id": id, "level": level,
		"affected": len(ids), "confirmed": level != "",
	}, nil
}

// SetFamilyRating 写家长确认分级（受 mcp.allow_direct_rating 门控）。
func (s *Service) SetFamilyRating(kind, id, level string) (map[string]any, error) {
	if err := s.requireDirectRating(); err != nil {
		return nil, err
	}
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	if !validFamilyLevels[level] {
		return nil, fmt.Errorf("分级无效：只支持 G、PG、PG-13、R、NC-17（空字符串表示撤回确认）")
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	if k == "movie" {
		if _, err := s.media.FindByID(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到影片：%s", id)
			}
			s.logger.Warnf("读取影片失败: %v", err)
			return nil, fmt.Errorf("读取影片失败")
		}
	} else if _, err := s.series.FindByID(id); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, fmt.Errorf("未找到剧集：%s", id)
		}
		s.logger.Warnf("读取剧集失败: %v", err)
		return nil, fmt.Errorf("读取剧集失败")
	}
	var out map[string]any
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = s.setFamilyRatingTx(tx, k, id, level, "mcp.set_family_rating")
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ClearFamilyRating 把确认分级清回未确认（受 mcp.allow_direct_rating 门控）。
func (s *Service) ClearFamilyRating(kind, id string) (map[string]any, error) {
	if err := s.requireDirectRating(); err != nil {
		return nil, err
	}
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	// 先读“之前是否已确认”，用于 changed 标记。
	changed := false
	if k == "movie" {
		if _, err := s.media.FindByID(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到影片：%s", id)
			}
			s.logger.Warnf("读取影片失败: %v", err)
			return nil, fmt.Errorf("读取影片失败")
		}
		if r, err := s.ratings.FindByMediaID(id); err == nil && r.Level != "" {
			changed = true
		}
	} else {
		if _, err := s.series.FindByID(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到剧集：%s", id)
			}
			s.logger.Warnf("读取剧集失败: %v", err)
			return nil, fmt.Errorf("读取剧集失败")
		}
		counts, err := s.episodeCounts([]string{id})
		if err != nil {
			return nil, err
		}
		changed = counts[id][1] > 0
	}
	var out map[string]any
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		out, err = s.setFamilyRatingTx(tx, k, id, "", "mcp.clear_family_rating")
		return err
	}); err != nil {
		return nil, err
	}
	out["changed"] = changed
	return out, nil
}

// MetadataUpdateInput update_media_metadata 的白名单字段（指针表示“未提供”）。
type MetadataUpdateInput struct {
	Title     *string `json:"title"`
	OrigTitle *string `json:"orig_title"`
	Year      *int    `json:"year"`
	Overview  *string `json:"overview"`
	Genres    *string `json:"genres"`
}

// UpdateMediaMetadata 只改白名单字段：标题、原始标题、年份、简介、类型。
func (s *Service) UpdateMediaMetadata(kind, id string, fields MetadataUpdateInput) (map[string]any, error) {
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	changes := map[string]any{}
	before := map[string]any{}
	after := map[string]any{}
	applyStr := func(name string, v *string, cur string, validate func(string) error) error {
		if v == nil {
			return nil
		}
		nv := strings.TrimSpace(*v)
		if validate != nil {
			if err := validate(nv); err != nil {
				return err
			}
		}
		before[name] = cur
		after[name] = nv
		if nv != cur {
			changes[name] = nv
		}
		return nil
	}
	applyYear := func(v *int, cur int) error {
		if v == nil {
			return nil
		}
		if *v < 1888 || *v > 2100 {
			return fmt.Errorf("年份无效：只支持 1888-2100")
		}
		before["year"] = cur
		after["year"] = *v
		if *v != cur {
			changes["year"] = *v
		}
		return nil
	}
	nonEmpty := func(field string) func(string) error {
		return func(v string) error {
			if v == "" {
				return fmt.Errorf("%s 不能为空", field)
			}
			return nil
		}
	}
	var targetType string
	if k == "movie" {
		targetType = "media"
		m, err := s.media.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到影片：%s", id)
			}
			s.logger.Warnf("读取影片失败: %v", err)
			return nil, fmt.Errorf("读取影片失败")
		}
		if err := applyStr("title", fields.Title, m.Title, nonEmpty("标题")); err != nil {
			return nil, err
		}
		if err := applyStr("orig_title", fields.OrigTitle, m.OrigTitle, nil); err != nil {
			return nil, err
		}
		if err := applyYear(fields.Year, m.Year); err != nil {
			return nil, err
		}
		if err := applyStr("overview", fields.Overview, m.Overview, nil); err != nil {
			return nil, err
		}
		if err := applyStr("genres", fields.Genres, m.Genres, nil); err != nil {
			return nil, err
		}
	} else {
		targetType = "series"
		sr, err := s.series.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到剧集：%s", id)
			}
			s.logger.Warnf("读取剧集失败: %v", err)
			return nil, fmt.Errorf("读取剧集失败")
		}
		if err := applyStr("title", fields.Title, sr.Title, nonEmpty("标题")); err != nil {
			return nil, err
		}
		if err := applyStr("orig_title", fields.OrigTitle, sr.OrigTitle, nil); err != nil {
			return nil, err
		}
		if err := applyYear(fields.Year, sr.Year); err != nil {
			return nil, err
		}
		if err := applyStr("overview", fields.Overview, sr.Overview, nil); err != nil {
			return nil, err
		}
		if err := applyStr("genres", fields.Genres, sr.Genres, nil); err != nil {
			return nil, err
		}
	}
	if len(before) == 0 {
		return nil, fmt.Errorf("没有可更新的字段：只支持 title、orig_title、year、overview、genres")
	}
	changed := len(changes) > 0
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if changed {
			var err error
			if k == "movie" {
				err = tx.Model(&model.Media{}).Where("id = ?", id).Updates(changes).Error
			} else {
				err = tx.Model(&model.Series{}).Where("id = ?", id).Updates(changes).Error
			}
			if err != nil {
				s.logger.Warnf("更新元数据失败: %v", err)
				return fmt.Errorf("更新元数据失败")
			}
		}
		detail := map[string]any{
			"kind": k, "id": id, "before": before, "after": after, "changed": changed,
		}
		if err := s.writeAudit(tx, "mcp.update_media_metadata", targetType, id, detail); err != nil {
			return fmt.Errorf("写审计失败")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{
		"kind": k, "id": id, "changed": changed, "before": before, "after": after,
	}, nil
}

// RescrapeMedia 对一部影片/剧集触发一次 TMDb 重刮，复用现有刮削逻辑。
func (s *Service) RescrapeMedia(kind, id string) (map[string]any, error) {
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	var targetType string
	if k == "movie" {
		targetType = "media"
		if _, err := s.media.FindByID(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到影片：%s", id)
			}
			s.logger.Warnf("读取影片失败: %v", err)
			return nil, fmt.Errorf("读取影片失败")
		}
	} else {
		targetType = "series"
		if _, err := s.series.FindByID(id); err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到剧集：%s", id)
			}
			s.logger.Warnf("读取剧集失败: %v", err)
			return nil, fmt.Errorf("读取剧集失败")
		}
	}
	var scrapeErr error
	if k == "movie" {
		scrapeErr = s.metadata.ScrapeMedia(id)
	} else {
		scrapeErr = s.metadata.ScrapeSeries(id)
	}
	// 审计记录重刮尝试（成功与失败都记；失败原因不回吐给客户端，
	// 避免把上游错误里的敏感信息带出去，详情见服务端日志）。
	detail := map[string]any{"kind": k, "id": id, "success": scrapeErr == nil}
	if scrapeErr != nil {
		s.logger.Warnf("MCP 重刮失败 %s %s: %v", k, id, scrapeErr)
		detail["error"] = "scrape failed"
		if err := s.db.Create(&model.AuditLog{
			OperatorID: "mcp", Operator: "mcp", Action: "mcp.rescrape_media",
			TargetType: targetType, TargetID: id, IP: "stdio",
			Detail: mustJSON(detail),
		}).Error; err != nil {
			s.logger.Warnf("写审计失败: %v", err)
		}
		return nil, fmt.Errorf("重刮失败：请检查 TMDb 配置与网络后重试（详情见服务端日志）")
	}
	// 成功后读回最新状态。
	status := ""
	hasRatingData := false
	if k == "movie" {
		if m, err := s.media.FindByID(id); err == nil {
			status = m.ScrapeStatus
			hasRatingData = strings.TrimSpace(m.TMDbRatingData) != ""
		}
	} else if sr, err := s.series.FindByID(id); err == nil {
		status = sr.ScrapeStatus
		hasRatingData = strings.TrimSpace(sr.TMDbRatingData) != ""
	}
	if err := s.db.Create(&model.AuditLog{
		OperatorID: "mcp", Operator: "mcp", Action: "mcp.rescrape_media",
		TargetType: targetType, TargetID: id, IP: "stdio",
		Detail: mustJSON(detail),
	}).Error; err != nil {
		s.logger.Warnf("写审计失败: %v", err)
	}
	return map[string]any{
		"kind": k, "id": id, "scrape_status": status,
		"has_tmdb_rating_data": hasRatingData,
	}, nil
}

// ClearTMDbRatingData 删某条刮削分级数据（只清 tmdb_rating_data 字段）。
func (s *Service) ClearTMDbRatingData(kind, id string) (map[string]any, error) {
	k, err := normalizeKind(kind)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(id) == "" {
		return nil, fmt.Errorf("id 不能为空")
	}
	var targetType string
	hadData := false
	if k == "movie" {
		targetType = "media"
		m, err := s.media.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到影片：%s", id)
			}
			s.logger.Warnf("读取影片失败: %v", err)
			return nil, fmt.Errorf("读取影片失败")
		}
		hadData = strings.TrimSpace(m.TMDbRatingData) != ""
	} else {
		targetType = "series"
		sr, err := s.series.FindByID(id)
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("未找到剧集：%s", id)
			}
			s.logger.Warnf("读取剧集失败: %v", err)
			return nil, fmt.Errorf("读取剧集失败")
		}
		hadData = strings.TrimSpace(sr.TMDbRatingData) != ""
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		var err error
		// 注意：必须传 Go 字段名 "TMDbRatingData"，不能传 "tmdb_rating_data"。
		// GORM 从字段名派生的真实列名是 tm_db_rating_data；
		// 迁移 DDL 另建的 tmdb_rating_data 列是死列（读写都走 tm_db_rating_data），
		// 写它清不掉数据。UpdateColumn 同样会跳过 UpdatedAt 之外的钩子，
		// 但单列清零正是需要的语义。
		if k == "movie" {
			err = tx.Model(&model.Media{}).Where("id = ?", id).UpdateColumn("TMDbRatingData", "").Error
		} else {
			err = tx.Model(&model.Series{}).Where("id = ?", id).UpdateColumn("TMDbRatingData", "").Error
		}
		if err != nil {
			s.logger.Warnf("清除分级数据失败: %v", err)
			return fmt.Errorf("清除分级数据失败")
		}
		detail := map[string]any{
			"kind": k, "id": id, "had_data": hadData, "changed": hadData,
		}
		if err := s.writeAudit(tx, "mcp.clear_tmdb_rating_data", targetType, id, detail); err != nil {
			return fmt.Errorf("写审计失败")
		}
		return nil
	}); err != nil {
		return nil, err
	}
	return map[string]any{"kind": k, "id": id, "changed": hadData}, nil
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
