package repository

import (
	"fmt"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"gorm.io/gorm"
)

// ==================== MediaRepo ====================

type MediaRepo struct {
	db *gorm.DB
}

// DB 返回底层数据库连接（供复杂查询使用）
func (r *MediaRepo) DB() *gorm.DB {
	return r.db
}

func (r *MediaRepo) Create(media *model.Media) error {
	return r.db.Create(media).Error
}

func (r *MediaRepo) FindByID(id string) (*model.Media, error) {
	var media model.Media
	err := r.db.First(&media, "id = ?", id).Error
	return &media, err
}

func (r *MediaRepo) FindByFilePath(filePath string) (*model.Media, error) {
	var media model.Media
	err := r.db.Where("file_path = ?", filePath).First(&media).Error
	return &media, err
}

func (r *MediaRepo) ListByLibraryAndFileSize(libraryID string, fileSize int64) ([]model.Media, error) {
	var media []model.Media
	q := r.db.Where("library_id = ? AND file_size = ?", libraryID, fileSize)
	if fileSize <= 0 {
		q = r.db.Where("library_id = ?", libraryID)
	}
	err := q.Find(&media).Error
	return media, err
}

// ListByFilePaths 批量按 file_path IN (...) 加载。
//
// 用途：SmartRename / LazyIngest 等场景需要一次性把"路径→Media"映射拉齐，
// 避免在循环内逐条 SQL（N+1）。空切片会直接返回空 map，不会发起查询。
//
// 注意：分批限制 GORM/SQLite 的 IN 子句参数数量上限（默认 999），故每批 800。
func (r *MediaRepo) ListByFilePaths(filePaths []string) (map[string]*model.Media, error) {
	out := make(map[string]*model.Media, len(filePaths))
	if len(filePaths) == 0 {
		return out, nil
	}
	const batch = 800
	for i := 0; i < len(filePaths); i += batch {
		end := i + batch
		if end > len(filePaths) {
			end = len(filePaths)
		}
		var rows []model.Media
		if err := r.db.Where("file_path IN ?", filePaths[i:end]).Find(&rows).Error; err != nil {
			return nil, err
		}
		for j := range rows {
			m := rows[j]
			out[m.FilePath] = &m
		}
	}
	return out, nil
}

// excludeDuplicates 在前端列表查询里统一过滤掉"同片副本"，
// 仅保留主版本（duplicate_of 为空）。这是"同片多版本折叠"的核心约束。
//
// 注意：仅用于面向用户的列表/海报墙；管理后台、版本切换、扫描器、清理工具
// 等需要看到全部记录的场景请直接走 r.db。
func (r *MediaRepo) excludeDuplicates(query *gorm.DB) *gorm.DB {
	return query.Where("duplicate_of IS NULL OR duplicate_of = ''")
}

func (r *MediaRepo) List(page, size int, libraryID string) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	query = r.excludeDuplicates(query)

	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

func (r *MediaRepo) Recent(limit int) ([]model.Media, error) {
	var media []model.Media
	err := r.excludeDuplicates(r.db.Model(&model.Media{})).
		Order("created_at DESC").Limit(limit).Find(&media).Error
	return media, err
}

func (r *MediaRepo) Search(keyword string, page, size int) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	// 改进搜索：支持多字段搜索（标题、原始标题、类型），并按相关性排序
	query := r.db.Model(&model.Media{}).Where(
		"title LIKE ? OR orig_title LIKE ? OR genres LIKE ?",
		"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%",
	)
	query = r.excludeDuplicates(query)
	query.Count(&total)
	// 优先显示标题精确匹配的结果，然后按评分降序
	err := query.Order(
		fmt.Sprintf("CASE WHEN title = '%s' THEN 0 WHEN title LIKE '%s%%' THEN 1 ELSE 2 END, rating DESC, created_at DESC",
			keyword, keyword),
	).Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

// SearchAdvancedParams 高级搜索参数
type SearchAdvancedParams struct {
	Keyword   string
	MediaType string
	Genre     string
	YearMin   int
	YearMax   int
	MinRating float64
	SortBy    string
	SortOrder string
	Page      int
	Size      int
}

// SearchAdvanced 高级搜索 — 支持多条件组合筛选、排序
func (r *MediaRepo) SearchAdvanced(params SearchAdvancedParams) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{})

	if params.Keyword != "" {
		// 改进：多字段搜索（标题、原始标题、标语、类型标签）
		query = query.Where(
			"title LIKE ? OR orig_title LIKE ? OR tagline LIKE ? OR genres LIKE ?",
			"%"+params.Keyword+"%", "%"+params.Keyword+"%", "%"+params.Keyword+"%", "%"+params.Keyword+"%",
		)
	}
	if params.MediaType != "" {
		query = query.Where("media_type = ?", params.MediaType)
	}
	if params.Genre != "" {
		// 改进：支持多类型筛选（逗号分隔）
		genres := strings.Split(params.Genre, ",")
		for _, g := range genres {
			g = strings.TrimSpace(g)
			if g != "" {
				query = query.Where("genres LIKE ?", "%"+g+"%")
			}
		}
	}
	if params.YearMin > 0 {
		query = query.Where("year >= ?", params.YearMin)
	}
	if params.YearMax > 0 {
		query = query.Where("year <= ?", params.YearMax)
	}
	if params.MinRating > 0 {
		query = query.Where("rating >= ?", params.MinRating)
	}
	query = r.excludeDuplicates(query)

	query.Count(&total)

	sortField := "created_at"
	sortDir := "DESC"
	switch params.SortBy {
	case "title":
		sortField = "title"
	case "year":
		sortField = "year"
	case "rating":
		sortField = "rating"
	case "created_at":
		sortField = "created_at"
	}
	if params.SortOrder == "asc" {
		sortDir = "ASC"
	}

	page := params.Page
	size := params.Size
	if page <= 0 {
		page = 1
	}
	if size <= 0 || size > 100 {
		size = 20
	}

	err := query.Order(fmt.Sprintf("%s %s", sortField, sortDir)).
		Offset((page - 1) * size).Limit(size).Find(&media).Error

	return media, total, err
}

func (r *MediaRepo) DeleteByID(id string) error {
	return r.db.Unscoped().Delete(&model.Media{}, "id = ?", id).Error
}

func (r *MediaRepo) DeleteByLibraryID(libraryID string) error {
	return r.db.Unscoped().Where("library_id = ?", libraryID).Delete(&model.Media{}).Error
}

func (r *MediaRepo) CleanOrphanedByLibraryIDs(validLibraryIDs []string) (int64, error) {
	var result *gorm.DB
	if len(validLibraryIDs) == 0 {
		result = r.db.Unscoped().Where("1 = 1").Delete(&model.Media{})
	} else {
		result = r.db.Unscoped().Where("library_id NOT IN ?", validLibraryIDs).Delete(&model.Media{})
	}
	return result.RowsAffected, result.Error
}

func (r *MediaRepo) Update(media *model.Media) error {
	return r.db.Save(media).Error
}

func (r *MediaRepo) UpdateFilePath(id string, filePath string) error {
	return r.db.Model(&model.Media{}).Where("id = ?", id).Update("file_path", filePath).Error
}

func (r *MediaRepo) UpdateOrganizedFields(id string, fields map[string]any) error {
	if id == "" || len(fields) == 0 {
		return nil
	}
	return r.db.Model(&model.Media{}).Where("id = ?", id).Updates(fields).Error
}

func (r *MediaRepo) FindByIDs(ids []string) ([]model.Media, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var media []model.Media
	err := r.db.Where("id IN ?", ids).Find(&media).Error
	return media, err
}

func (r *MediaRepo) ListByGenres(genres []string, excludeIDs []string, limit int) ([]model.Media, error) {
	if len(genres) == 0 {
		return nil, nil
	}
	query := r.db.Model(&model.Media{})
	for i, genre := range genres {
		if i == 0 {
			query = query.Where("genres LIKE ?", "%"+genre+"%")
		} else {
			query = query.Or("genres LIKE ?", "%"+genre+"%")
		}
	}
	if len(excludeIDs) > 0 {
		query = query.Where("id NOT IN ?", excludeIDs)
	}
	var media []model.Media
	err := query.Order("rating DESC").Limit(limit).Find(&media).Error
	return media, err
}

// ListHighRated 获取高评分媒体（用于冷启动推荐的多样化内容）
func (r *MediaRepo) ListHighRated(limit int, minRating float64) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("rating >= ?", minRating).
		Order("rating DESC, created_at DESC").
		Limit(limit).
		Find(&media).Error
	return media, err
}

func (r *MediaRepo) ListByLibraryID(libraryID string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("library_id = ?", libraryID).Find(&media).Error
	return media, err
}

// ListVersionsForMedia 返回与指定 media 同属一个"同片副本组"的全部版本（含主版本本身），
// 用于前端"切换版本/选择清晰度"UI。
//
// 匹配规则（按优先级回退）：
//  1. 给定 media 是主版本（duplicate_of 为空）：返回 duplicate_of = media.ID 的所有副本 + media 本身
//  2. 给定 media 是副本（duplicate_of 非空）：返回主版本 + 同主版本下的全部副本
//  3. 没有 duplicate 关系但有 duplicate_group：按 duplicate_group 聚合
//  4. 都没有：仅返回 media 自己
//
// 排序：分辨率优先级 > 文件大小（高在前），方便前端默认选最高质量。
func (r *MediaRepo) ListVersionsForMedia(media *model.Media) ([]model.Media, error) {
	if media == nil || media.ID == "" {
		return nil, nil
	}

	primaryID := media.ID
	if media.DuplicateOf != "" {
		primaryID = media.DuplicateOf
	}

	var versions []model.Media

	// 主版本 + 同组副本一次查出
	// 用 OR 聚合：自己（主或副本回查主） + 所有以 primaryID 为主的副本
	if err := r.db.Where("id = ? OR duplicate_of = ?", primaryID, primaryID).
		Order("CASE resolution WHEN '4K' THEN 5 WHEN '2K' THEN 4 WHEN '1080p' THEN 3 WHEN '720p' THEN 2 WHEN '480p' THEN 1 ELSE 0 END DESC, file_size DESC").
		Find(&versions).Error; err != nil {
		return nil, err
	}

	// 兜底：以 duplicate_group 再聚合一次（处理历史脏数据：duplicate_of 为空但 duplicate_group 一致）
	if len(versions) <= 1 && media.DuplicateGroup != "" {
		var byGroup []model.Media
		if err := r.db.Where("duplicate_group = ?", media.DuplicateGroup).
			Order("CASE resolution WHEN '4K' THEN 5 WHEN '2K' THEN 4 WHEN '1080p' THEN 3 WHEN '720p' THEN 2 WHEN '480p' THEN 1 ELSE 0 END DESC, file_size DESC").
			Find(&byGroup).Error; err == nil && len(byGroup) > len(versions) {
			versions = byGroup
		}
	}

	if len(versions) == 0 {
		// 至少返回自己
		versions = append(versions, *media)
	}
	return versions, nil
}

func (r *MediaRepo) ListBySeriesID(seriesID string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("series_id = ?", seriesID).
		Order("season_num ASC, episode_num ASC").Find(&media).Error
	return media, err
}

func (r *MediaRepo) ListBySeriesAndSeason(seriesID string, seasonNum int) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("series_id = ? AND season_num = ?", seriesID, seasonNum).
		Order("episode_num ASC").Find(&media).Error
	return media, err
}

func (r *MediaRepo) RecentNonEpisode(limit int) ([]model.Media, error) {
	var media []model.Media
	query := r.db.Where("(series_id = '' OR series_id IS NULL) AND library_id != '' AND (media_type IS NULL OR media_type = '' OR media_type != 'episode')")
	query = r.excludeDuplicates(query)
	err := query.Order("created_at DESC").Limit(limit).Find(&media).Error
	return media, err
}

func (r *MediaRepo) RecentNonEpisodeAll(libraryID string) ([]model.Media, error) {
	var media []model.Media
	query := r.db.Where("(series_id = '' OR series_id IS NULL) AND library_id != '' AND (media_type IS NULL OR media_type = '' OR media_type != 'episode')")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	query = r.excludeDuplicates(query)
	err := query.Order("created_at DESC").Find(&media).Error
	return media, err
}

func (r *MediaRepo) ListNonEpisode(page, size int, libraryID string) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{}).Where("(series_id = '' OR series_id IS NULL) AND library_id != ''")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	query = r.excludeDuplicates(query)

	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

func (r *MediaRepo) CleanGhostMedia() (int64, error) {
	result := r.db.Unscoped().Where("library_id = '' OR library_id IS NULL").Delete(&model.Media{})
	return result.RowsAffected, result.Error
}

func (r *MediaRepo) CountNonEpisodeByLibrary(libraryID string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Media{}).Where("(series_id = '' OR series_id IS NULL) AND library_id != ''")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	query = r.excludeDuplicates(query)
	err := query.Count(&count).Error
	return count, err
}

func (r *MediaRepo) CountNonEpisode(libraryID string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Media{}).Where("(series_id = '' OR series_id IS NULL) AND library_id != '' AND (media_type IS NULL OR media_type = '' OR media_type != 'episode')")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	query = r.excludeDuplicates(query)
	err := query.Count(&count).Error
	return count, err
}

// ==================== MediaRepo 扩展方法（文件管理） ====================

func (r *MediaRepo) ListFilesAdvanced(page, size int, libraryID, mediaType, keyword, sortBy, sortOrder string, scrapedOnly *bool) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{})

	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if mediaType != "" {
		query = query.Where("media_type = ?", mediaType)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR orig_title LIKE ? OR file_path LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if scrapedOnly != nil {
		if *scrapedOnly {
			// 统一口径：scrape_status 为 scraped/partial/manual 即认为已刮削
			query = query.Where("scrape_status IN (?)", []string{"scraped", "partial", "manual"})
		} else {
			query = query.Where("scrape_status IS NULL OR scrape_status = '' OR scrape_status IN (?)", []string{"pending", "failed"})
		}
	}

	query.Count(&total)

	sortField := "created_at"
	sortDir := "DESC"
	switch sortBy {
	case "title":
		sortField = "title"
	case "year":
		sortField = "year"
	case "rating":
		sortField = "rating"
	case "file_size":
		sortField = "file_size"
	case "created_at":
		sortField = "created_at"
	case "updated_at":
		sortField = "updated_at"
	}
	if sortOrder == "asc" {
		sortDir = "ASC"
	}

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}

	err := query.Order(fmt.Sprintf("%s %s", sortField, sortDir)).
		Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

func (r *MediaRepo) CountByMediaType(mediaType string) (int64, error) {
	var count int64
	err := r.db.Model(&model.Media{}).Where("media_type = ?", mediaType).Count(&count).Error
	return count, err
}

// CountScraped 统一以 scrape_status 为准：已刮削 = scraped / partial / manual
func (r *MediaRepo) CountScraped() (int64, error) {
	var count int64
	err := r.db.Model(&model.Media{}).
		Where("scrape_status IN (?)", []string{"scraped", "partial", "manual"}).
		Count(&count).Error
	return count, err
}

// CountByScrapeStatus 按刮削状态分别计数（传入 libraryID 和 folderPath 实现作用域化）
// folderPath 为空时不限制目录；libraryID 为空时不限制媒体库
func (r *MediaRepo) CountByScrapeStatus(libraryID, folderPath string) (map[string]int64, error) {
	result := make(map[string]int64)
	query := r.db.Model(&model.Media{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if folderPath != "" {
		normalized := strings.ReplaceAll(folderPath, "\\", "/")
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		query = query.Where("REPLACE(file_path, '\\', '/') LIKE ?", normalized+"%")
	}

	rows, err := query.Select("COALESCE(NULLIF(scrape_status, ''), 'pending') as st, COUNT(*) as cnt").
		Group("st").Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var st string
		var cnt int64
		if err := rows.Scan(&st, &cnt); err != nil {
			continue
		}
		result[st] = cnt
	}
	return result, nil
}

// RepairScrapeStatusForCompleted 幂等修复存量数据：
// 对那些 overview 与 poster_path 均已齐全、但 scrape_status 仍处于 pending / 空 的 media 行，
// 直接置为 scraped。常见于剧集（episode）通过 syncSeriesToEpisodes 补齐元数据但
// 历史版本未回写状态字段的场景。
//
// 不影响 manual / failed / partial / scraped 行；不影响仍缺字段的行；可重复调用。
// 返回修复的行数（用于日志）。
func (r *MediaRepo) RepairScrapeStatusForCompleted() (int64, error) {
	res := r.db.Model(&model.Media{}).
		Where("(scrape_status IS NULL OR scrape_status = '' OR scrape_status = 'pending')").
		Where("overview IS NOT NULL AND TRIM(overview) <> ''").
		Where("poster_path IS NOT NULL AND TRIM(poster_path) <> ''").
		Updates(map[string]interface{}{
			"scrape_status": "scraped",
		})
	return res.RowsAffected, res.Error
}

// CountByScopeAndType 在指定作用域下按媒体类型计数（movie/episode 等）
func (r *MediaRepo) CountByScopeAndType(libraryID, folderPath, mediaType string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Media{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if folderPath != "" {
		normalized := strings.ReplaceAll(folderPath, "\\", "/")
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		query = query.Where("REPLACE(file_path, '\\', '/') LIKE ?", normalized+"%")
	}
	if mediaType != "" {
		query = query.Where("media_type = ?", mediaType)
	}
	err := query.Count(&count).Error
	return count, err
}

// CountByScope 在指定作用域下统计总数
func (r *MediaRepo) CountByScope(libraryID, folderPath string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Media{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if folderPath != "" {
		normalized := strings.ReplaceAll(folderPath, "\\", "/")
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		query = query.Where("REPLACE(file_path, '\\', '/') LIKE ?", normalized+"%")
	}
	err := query.Count(&count).Error
	return count, err
}

// SumFileSizeByScope 作用域化的总文件大小
func (r *MediaRepo) SumFileSizeByScope(libraryID, folderPath string) (int64, error) {
	var total int64
	query := r.db.Model(&model.Media{})
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if folderPath != "" {
		normalized := strings.ReplaceAll(folderPath, "\\", "/")
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		query = query.Where("REPLACE(file_path, '\\', '/') LIKE ?", normalized+"%")
	}
	err := query.Select("COALESCE(SUM(file_size), 0)").Scan(&total).Error
	return total, err
}

// CountRecentImportsByScope 作用域化的近 N 天导入数
func (r *MediaRepo) CountRecentImportsByScope(days int, libraryID, folderPath string) (int64, error) {
	var count int64
	query := r.db.Model(&model.Media{}).
		Where("created_at >= datetime('now', ?)", fmt.Sprintf("-%d days", days))
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if folderPath != "" {
		normalized := strings.ReplaceAll(folderPath, "\\", "/")
		if !strings.HasSuffix(normalized, "/") {
			normalized += "/"
		}
		query = query.Where("REPLACE(file_path, '\\', '/') LIKE ?", normalized+"%")
	}
	err := query.Count(&count).Error
	return count, err
}

func (r *MediaRepo) SumFileSize() (int64, error) {
	var total int64
	err := r.db.Model(&model.Media{}).Select("COALESCE(SUM(file_size), 0)").Scan(&total).Error
	return total, err
}

func (r *MediaRepo) CountRecentImports(days int) (int64, error) {
	var count int64
	err := r.db.Model(&model.Media{}).
		Where("created_at >= datetime('now', ?)", fmt.Sprintf("-%d days", days)).
		Count(&count).Error
	return count, err
}

func (r *MediaRepo) ListByMediaType(mediaType string) ([]model.Media, error) {
	var media []model.Media
	err := r.db.Where("media_type = ?", mediaType).Find(&media).Error
	return media, err
}

func (r *MediaRepo) BatchUpdateMediaType(ids []string, mediaType string) (int64, error) {
	result := r.db.Model(&model.Media{}).Where("id IN ?", ids).Update("media_type", mediaType)
	return result.RowsAffected, result.Error
}

// GetAllFilePaths 获取所有媒体文件路径（用于构建文件夹树）
func (r *MediaRepo) GetAllFilePaths(libraryID string) ([]string, error) {
	var paths []string
	query := r.db.Model(&model.Media{}).Select("file_path")
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	err := query.Pluck("file_path", &paths).Error
	return paths, err
}

// ListByFolderPath 按文件夹路径查询文件（精确匹配目录，不递归子目录）
func (r *MediaRepo) ListByFolderPath(folderPath string, page, size int, libraryID, mediaType, keyword, sortBy, sortOrder string, scrapedOnly *bool) ([]model.Media, int64, error) {
	var media []model.Media
	var total int64

	query := r.db.Model(&model.Media{})

	// 使用 LIKE 匹配指定目录下的直接子文件（不含子目录中的文件）
	// folderPath 末尾需要加分隔符
	// SQLite 中使用 file_path LIKE 'folder/%' AND file_path NOT LIKE 'folder/%/%'
	if folderPath != "" {
		// 标准化路径分隔符
		normalizedPath := strings.ReplaceAll(folderPath, "\\", "/")
		if !strings.HasSuffix(normalizedPath, "/") {
			normalizedPath += "/"
		}
		query = query.Where(
			"(REPLACE(file_path, '\\', '/') LIKE ? AND REPLACE(file_path, '\\', '/') NOT LIKE ?)",
			normalizedPath+"%",
			normalizedPath+"%/%",
		)
	}

	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	if mediaType != "" {
		query = query.Where("media_type = ?", mediaType)
	}
	if keyword != "" {
		query = query.Where("title LIKE ? OR orig_title LIKE ? OR file_path LIKE ?",
			"%"+keyword+"%", "%"+keyword+"%", "%"+keyword+"%")
	}
	if scrapedOnly != nil {
		if *scrapedOnly {
			query = query.Where("scrape_status IN (?)", []string{"scraped", "partial", "manual"})
		} else {
			query = query.Where("scrape_status IS NULL OR scrape_status = '' OR scrape_status IN (?)", []string{"pending", "failed"})
		}
	}

	query.Count(&total)

	sortField := "created_at"
	sortDir := "DESC"
	switch sortBy {
	case "title":
		sortField = "title"
	case "year":
		sortField = "year"
	case "rating":
		sortField = "rating"
	case "file_size":
		sortField = "file_size"
	case "created_at":
		sortField = "created_at"
	case "updated_at":
		sortField = "updated_at"
	}
	if sortOrder == "asc" {
		sortDir = "ASC"
	}

	if page < 1 {
		page = 1
	}
	if size < 1 || size > 200 {
		size = 20
	}

	err := query.Order(fmt.Sprintf("%s %s", sortField, sortDir)).
		Offset((page - 1) * size).Limit(size).Find(&media).Error
	return media, total, err
}

// UpdateFilePathPrefix 批量更新文件路径前缀（用于文件夹重命名）
func (r *MediaRepo) UpdateFilePathPrefix(oldPrefix, newPrefix string) error {
	return r.db.Exec(
		"UPDATE media SET file_path = ? || SUBSTR(REPLACE(file_path, '\\', '/'), LENGTH(?) + 1) WHERE REPLACE(file_path, '\\', '/') LIKE ?",
		newPrefix, oldPrefix, oldPrefix+"%",
	).Error
}

// DeleteByPathPrefix 删除指定路径前缀下的所有文件记录
func (r *MediaRepo) DeleteByPathPrefix(pathPrefix string) error {
	return r.db.Where("REPLACE(file_path, '\\', '/') LIKE ?", pathPrefix+"%").Delete(&model.Media{}).Error
}

// ==================== P2/P3: 性能优化方法 ====================

// GetAllFilePathsByLibrary 获取指定媒体库的所有文件路径集合（用于内存查重，避免 N+1 查询）
func (r *MediaRepo) GetAllFilePathsByLibrary(libraryID string) (map[string]bool, error) {
	var paths []string
	err := r.db.Model(&model.Media{}).Where("library_id = ?", libraryID).Pluck("file_path", &paths).Error
	if err != nil {
		return nil, err
	}
	pathSet := make(map[string]bool, len(paths))
	for _, p := range paths {
		pathSet[p] = true
	}
	return pathSet, nil
}

// BatchCreate 批量创建媒体记录（减少 SQLite 写锁竞争，每批 100 条）
func (r *MediaRepo) BatchCreate(mediaList []*model.Media) error {
	if len(mediaList) == 0 {
		return nil
	}
	return r.db.CreateInBatches(mediaList, 100).Error
}

// UpdateFields 仅更新指定字段（减少写锁争用，提高 SQLite 并发性能）
func (r *MediaRepo) UpdateFields(id string, fields map[string]interface{}) error {
	return r.db.Model(&model.Media{}).Where("id = ?", id).Updates(fields).Error
}

// ListNeedScrape 获取需要刮削的媒体列表
// 规则：
//   - 排除 manual（用户手动锁定）
//   - 排除 scraped（已完整刮削）
//   - partial（部分成功，海报/overview 缺失）允许重试，但 1 小时内不重复
//   - failed 按 skipRecentFailedDays 节流
//   - pending 或无状态：直接需要刮削
func (r *MediaRepo) ListNeedScrape(libraryID string, skipRecentFailedDays int) ([]model.Media, error) {
	var media []model.Media
	query := r.db.Model(&model.Media{}).Where(
		"scrape_status IS NULL OR scrape_status = '' OR scrape_status IN (?)",
		[]string{"pending", "failed", "partial"},
	)
	if libraryID != "" {
		query = query.Where("library_id = ?", libraryID)
	}
	// 跳过最近 N 天内已失败的记录（避免重复无效请求）
	if skipRecentFailedDays > 0 {
		query = query.Where(
			"NOT (scrape_status = 'failed' AND last_scrape_at IS NOT NULL AND last_scrape_at >= datetime('now', ?))",
			fmt.Sprintf("-%d days", skipRecentFailedDays),
		)
	}
	// partial 节流：最近 1 小时不重试（避免图片 CDN 短时抖动反复请求）
	query = query.Where(
		"NOT (scrape_status = 'partial' AND last_scrape_at IS NOT NULL AND last_scrape_at >= datetime('now', '-1 hours'))",
	)
	err := query.Order("created_at DESC").Find(&media).Error
	return media, err
}
