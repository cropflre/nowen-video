package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/nowen-video/nowen-video/internal/matcher"
	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// CollectionService 电影系列合集服务
type CollectionService struct {
	collRepo  *repository.MovieCollectionRepo
	mediaRepo *repository.MediaRepo
	logger    *zap.SugaredLogger
}

// NewCollectionService 创建合集服务
func NewCollectionService(
	collRepo *repository.MovieCollectionRepo,
	mediaRepo *repository.MediaRepo,
	logger *zap.SugaredLogger,
) *CollectionService {
	return &CollectionService{
		collRepo:  collRepo,
		mediaRepo: mediaRepo,
		logger:    logger,
	}
}

// CollectionWithMedia 合集及其包含的电影
type CollectionWithMedia struct {
	Collection *model.MovieCollection `json:"collection"`
	Media      []CollectionMediaItem  `json:"media"`
}

// CollectionMediaItem 合集中的电影项
type CollectionMediaItem struct {
	ID           string  `json:"id"`
	Title        string  `json:"title"`
	OrigTitle    string  `json:"orig_title"`
	Year         int     `json:"year"`
	Premiered    string  `json:"premiered"`
	Rating       float64 `json:"rating"`
	PosterPath   string  `json:"poster_path"`
	Runtime      int     `json:"runtime"`
	Overview     string  `json:"overview"`
	Genres       string  `json:"genres"`
	IsCurrent    bool    `json:"is_current"`    // 是否为当前正在查看的电影
	TMDbID       int     `json:"tmdb_id"`       // TMDB ID（用于前端折叠同片多版本）
	VersionGroup string  `json:"version_group"` // 同一部电影的不同版本共享此 ID
	VersionTag   string  `json:"version_tag"`   // 版本标识（"4K"/"Director's Cut" 等）
	FileSize     int64   `json:"file_size"`     // 文件大小（字节），用于版本选择
	Resolution   string  `json:"resolution"`    // 分辨率（"1080p"/"2160p" 等）
	VideoCodec   string  `json:"video_codec"`   // 视频编码
}

// GetCollectionByMediaID 根据媒体 ID 获取其所属的合集信息
func (s *CollectionService) GetCollectionByMediaID(mediaID string) (*CollectionWithMedia, error) {
	coll, err := s.collRepo.FindCollectionByMediaID(mediaID)
	if err != nil {
		return nil, err
	}

	result := &CollectionWithMedia{
		Collection: coll,
		Media:      make([]CollectionMediaItem, 0, len(coll.Media)),
	}

	for _, m := range coll.Media {
		result.Media = append(result.Media, CollectionMediaItem{
			ID:           m.ID,
			Title:        m.Title,
			OrigTitle:    m.OrigTitle,
			Year:         m.Year,
			Premiered:    m.Premiered,
			Rating:       m.Rating,
			PosterPath:   m.PosterPath,
			Runtime:      m.Runtime,
			Overview:     m.Overview,
			Genres:       m.Genres,
			IsCurrent:    m.ID == mediaID,
			TMDbID:       m.TMDbID,
			VersionGroup: m.VersionGroup,
			VersionTag:   m.VersionTag,
			FileSize:     m.FileSize,
			Resolution:   m.Resolution,
			VideoCodec:   m.VideoCodec,
		})
	}

	// 清空 Preload 填充的 Media 字段，避免 JSON 序列化时 collection 内部重复包含 media 数组
	coll.Media = nil

	return result, nil
}

// GetCollectionDetail 获取合集详情
func (s *CollectionService) GetCollectionDetail(collectionID string) (*CollectionWithMedia, error) {
	coll, err := s.collRepo.FindByIDWithMedia(collectionID)
	if err != nil {
		return nil, err
	}

	result := &CollectionWithMedia{
		Collection: coll,
		Media:      make([]CollectionMediaItem, 0, len(coll.Media)),
	}

	for _, m := range coll.Media {
		result.Media = append(result.Media, CollectionMediaItem{
			ID:           m.ID,
			Title:        m.Title,
			OrigTitle:    m.OrigTitle,
			Year:         m.Year,
			Premiered:    m.Premiered,
			Rating:       m.Rating,
			PosterPath:   m.PosterPath,
			Runtime:      m.Runtime,
			Overview:     m.Overview,
			Genres:       m.Genres,
			TMDbID:       m.TMDbID,
			VersionGroup: m.VersionGroup,
			VersionTag:   m.VersionTag,
			FileSize:     m.FileSize,
			Resolution:   m.Resolution,
			VideoCodec:   m.VideoCodec,
		})
	}

	// 清空 Preload 填充的 Media 字段，避免 JSON 序列化时 collection 内部重复包含 media 数组
	coll.Media = nil

	return result, nil
}

// ListCollections 分页获取合集列表
// 同时确保每个合集都有封面海报（使用第一部电影的海报）
func (s *CollectionService) ListCollections(page, size int) ([]model.MovieCollection, int64, error) {
	return s.ListCollectionsWithOptions(page, size, "created_desc", "")
}

// ListCollectionsWithOptions 支持排序和来源筛选的分页查询
func (s *CollectionService) ListCollectionsWithOptions(page, size int, sort, autoFilter string) ([]model.MovieCollection, int64, error) {
	colls, total, err := s.collRepo.ListWithOptions(page, size, sort, autoFilter)
	if err != nil {
		return colls, total, err
	}

	// 为没有海报的合集自动设置第一部电影的海报
	for i := range colls {
		if colls[i].PosterPath == "" {
			s.ensureCollectionPoster(&colls[i])
		}
	}

	return colls, total, nil
}

// GetCollectionPosterPath 获取合集封面海报的文件路径
// 策略：
// 1. 优先使用合集自身的 PosterPath
// 2. 如果为空，使用合集中第一部电影（按年份排序）的海报
func (s *CollectionService) GetCollectionPosterPath(collectionID string) (string, error) {
	coll, err := s.collRepo.FindByID(collectionID)
	if err != nil {
		return "", err
	}

	// 1. 尝试合集自身的海报路径
	if coll.PosterPath != "" {
		return coll.PosterPath, nil
	}

	// 2. 获取合集中第一部电影的海报
	media, err := s.collRepo.GetMediaByCollectionID(collectionID)
	if err != nil || len(media) == 0 {
		return "", nil
	}

	// 使用第一部电影的海报路径，并同步更新到合集记录中
	firstMedia := media[0]
	if firstMedia.PosterPath != "" {
		// 同步更新合集的 PosterPath
		coll.PosterPath = firstMedia.PosterPath
		s.collRepo.Update(coll)
		return firstMedia.PosterPath, nil
	}

	// 返回第一部电影的 ID，让 handler 层通过 StreamService 获取海报
	return "", nil
}

// GetFirstMediaID 获取合集中第一部电影的 ID（用于海报回退）
func (s *CollectionService) GetFirstMediaID(collectionID string) (string, error) {
	media, err := s.collRepo.GetMediaByCollectionID(collectionID)
	if err != nil || len(media) == 0 {
		return "", err
	}
	return media[0].ID, nil
}

// ensureCollectionPoster 确保合集有封面海报
func (s *CollectionService) ensureCollectionPoster(coll *model.MovieCollection) {
	media, err := s.collRepo.GetMediaByCollectionID(coll.ID)
	if err != nil || len(media) == 0 {
		return
	}

	// 使用第一部电影的海报
	for _, m := range media {
		if m.PosterPath != "" {
			coll.PosterPath = m.PosterPath
			// 异步更新数据库
			go func(id, posterPath string) {
				if c, err := s.collRepo.FindByID(id); err == nil {
					c.PosterPath = posterPath
					s.collRepo.Update(c)
				}
			}(coll.ID, m.PosterPath)
			return
		}
	}
}

// AutoMatchCollections 自动匹配电影系列合集
// 双层匹配策略：
// 第一层（精确）：通过数字序号、罗马数字等后缀模式提取基础名
// 第二层（前缀）：通过连接词（之、的、·、—）分割标题提取系列前缀
// 算法流程：
// 1. 先合并已有的同名重复合集
// 2. 获取所有没有合集的电影
// 3. 第一层：精确模式匹配（数字序号等）
// 4. 第二层：对第一层未匹配的电影，使用连接词分割法提取前缀并聚合
// 5. 只有 >= 2 部电影的才创建合集
// 6. 最后清理空壳合集
func (s *CollectionService) AutoMatchCollections() (int, error) {
	// 第一步：先合并已有的同名重复合集，避免后续匹配时出现歧义
	merged, err := s.MergeDuplicateCollections()
	if err != nil {
		s.logger.Warnf("合并同名合集时出错（继续执行）: %v", err)
	} else if merged > 0 {
		s.logger.Infof("已合并 %d 组同名重复合集", merged)
	}

	movies, err := s.collRepo.ListMoviesWithoutCollection()
	if err != nil {
		return 0, err
	}

	// ===== 第一层：精确模式匹配（数字序号、罗马数字等） =====
	// 增强：先尝试组合式深度提取（L2+L1），再退回纯 L1
	//   示例："逃学威龙3之龙过鸡年" -> L2 得 "逃学威龙3" -> L1 得 "逃学威龙"
	//   示例："逃学威龙2"           -> 组合式无命中 -> L1 得 "逃学威龙"
	groups := make(map[string][]model.Media)
	var unmatchedMovies []model.Media // 第一层未匹配到的电影，留给第二层处理

	for _, m := range movies {
		baseName := matcher.ExtractBaseNameDeep(m.Title)
		if baseName == "" {
			baseName = extractSeriesBaseName(m.Title)
		}
		if baseName != "" {
			groups[baseName] = append(groups[baseName], m)
		} else {
			unmatchedMovies = append(unmatchedMovies, m)
		}
	}

	// ===== 第二层：连接词分割法提取前缀并聚合 =====
	prefixGroups := make(map[string][]model.Media)
	matchedIDs := make(map[string]bool) // 记录已被L1或L2匹配的媒体ID
	for id := range groups {
		for _, m := range groups[id] {
			matchedIDs[m.ID] = true
		}
	}
	for _, m := range unmatchedMovies {
		prefix := extractPrefixByDelimiter(m.Title)
		if prefix != "" {
			prefixGroups[prefix] = append(prefixGroups[prefix], m)
			matchedIDs[m.ID] = true
		}
	}

	// 修复点1：不再提前过滤 len < 2 的前缀组，全部合并到主分组，统一在后续处理
	// 这样后续单独入库的系列电影也能被追加到已存在的同名合集中
	for prefix, mediaList := range prefixGroups {
		groups[prefix] = append(groups[prefix], mediaList...)
		s.logger.Infof("[前缀匹配] 发现系列前缀 '%s'，包含 %d 部电影", prefix, len(mediaList))
	}

	// ===== 第三层：通用空格分割 =====
	// 对第一层和第二层均未匹配的电影，只要标题包含空格就提取空格前的基础名
	spaceGroups := make(map[string][]model.Media)
	for _, m := range unmatchedMovies {
		if matchedIDs[m.ID] {
			continue // 已被L1或L2匹配，跳过
		}
		baseName := extractBaseNameBySpaceSplit(m.Title)
		if baseName != "" {
			spaceGroups[baseName] = append(spaceGroups[baseName], m)
		}
	}

	for baseName, mediaList := range spaceGroups {
		groups[baseName] = append(groups[baseName], mediaList...)
		s.logger.Infof("[空格分割] 发现系列基础名 '%s'，包含 %d 部电影", baseName, len(mediaList))
	}

	// ===== 第四层：裸标题吸附 =====
	// 对前三层都未匹配的"裸标题"电影（如 "逃学威龙"、"Toy Story"），
	// 如果其标题（归一化后）恰好等于本次已形成的某个分组 key，或等于数据库中已存在的合集名，
	// 就把它吸附到对应分组。这样"系列首部"不会因为没有任何数字/连接词而游离在合集之外。
	//
	// 构造"归一化 key -> 原始 key"映射，提高匹配鲁棒性（忽略空白/全半角/标点）
	normalizedGroupKeys := make(map[string]string, len(groups))
	for k := range groups {
		nk := matcher.NormalizeForCompare(k)
		if nk != "" {
			normalizedGroupKeys[nk] = k
		}
	}

	absorbed := 0
	for _, m := range unmatchedMovies {
		if matchedIDs[m.ID] {
			continue
		}
		norm := matcher.NormalizeForCompare(m.Title)
		if norm == "" {
			continue
		}
		// 1) 先尝试吸附到本次已形成的分组
		if origKey, ok := normalizedGroupKeys[norm]; ok {
			groups[origKey] = append(groups[origKey], m)
			matchedIDs[m.ID] = true
			absorbed++
			s.logger.Infof("[裸标题吸附] '%s' 归入分组 '%s'", m.Title, origKey)
			continue
		}
		// 2) 再尝试吸附到数据库中已存在的同名合集（例如历史数据已建好"逃学威龙"合集）
		if existing, err := s.collRepo.FindByNameFuzzy(m.Title); err == nil && existing != nil {
			groups[existing.Name] = append(groups[existing.Name], m)
			matchedIDs[m.ID] = true
			absorbed++
			s.logger.Infof("[裸标题吸附] '%s' 归入既有合集 '%s'", m.Title, existing.Name)
		}
	}
	if absorbed > 0 {
		s.logger.Infof("[裸标题吸附] 共吸附 %d 部系列首部电影到既有/新建分组", absorbed)
	}

	created := 0
	for baseName, mediaList := range groups {
		// ★ 方案 A 核心：同片多版本去重
		// 在"建合集/追加合集"之前，先把"同一部电影的不同版本"折叠成一个逻辑项。
		// 判定优先级：
		//   1) 相同 TMDB ID（最可靠）
		//   2) 归一化标题 + 相同年份（兜底）
		// 被折叠的版本会被打上同一个 version_group 标记，方便前端展示版本切换。
		uniqueMovies, versionMap := s.deduplicateVersions(mediaList)
		if len(uniqueMovies) != len(mediaList) {
			s.logger.Infof("[去重] 分组 '%s' 合并了 %d 个版本副本（%d → %d 部电影）",
				baseName, len(mediaList)-len(uniqueMovies), len(mediaList), len(uniqueMovies))
			// 同步把 version_group 写入数据库（幂等）
			s.persistVersionGroups(versionMap)
		}

		// 修复点2：先检查数据库中是否已经存在同名合集（模糊匹配，去除空格/标点/全半角差异）
		existing, err := s.collRepo.FindByNameFuzzy(baseName)

		// 修复点3：只有当合集"不存在"且"待关联电影少于2部（去重后）"时，才跳过（不创建新合集）
		if (err != nil || existing == nil) && len(uniqueMovies) < 2 {
			continue
		}

		// 修复点4：如果合集已存在，即使本次只匹配到 1 部电影，也追加进去
		// 注意：这里用原始 mediaList（包含所有版本副本），它们都应该加入同一个合集，
		// 避免"只有主版本进合集、副版本游离在外"的数据不一致。
		if err == nil && existing != nil {
			added := false
			for _, m := range mediaList {
				if m.CollectionID == "" {
					s.mediaRepo.UpdateFields(m.ID, map[string]interface{}{
						"collection_id": existing.ID,
					})
					added = true
				}
			}
			// 如果有新电影被加入，则更新该合集的媒体总数
			if added {
				s.collRepo.UpdateMediaCount(existing.ID)
				s.logger.Infof("向已有合集 '%s' 自动追加了 %d 部电影（去重后 %d 部）",
					baseName, len(mediaList), len(uniqueMovies))
			}
			continue
		}

		// 创建新合集（只有去重后电影 >= 2 部才会走到这里）
		coll := &model.MovieCollection{
			Name:        baseName,
			PosterPath:  uniqueMovies[0].PosterPath, // 使用第一部电影的海报
			MediaCount:  len(uniqueMovies),          // 计数以"去重后"为准，前端展示更贴合"系列里有几部电影"
			AutoMatched: true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.collRepo.Create(coll); err != nil {
			s.logger.Warnf("创建合集 '%s' 失败: %v", baseName, err)
			continue
		}

		// 关联电影：原始 mediaList 全部都要关联（包括版本副本），
		// 前端展示时再根据 version_group 折叠成单张卡片。
		for _, m := range mediaList {
			s.mediaRepo.UpdateFields(m.ID, map[string]interface{}{
				"collection_id": coll.ID,
			})
		}

		s.logger.Infof("自动创建合集 '%s'，包含 %d 部电影（总文件 %d 个）",
			baseName, len(uniqueMovies), len(mediaList))
		created++
	}

	// 最后清理空壳合集
	cleaned, cleanErr := s.CleanupEmptyCollections()
	if cleanErr != nil {
		s.logger.Warnf("清理空壳合集时出错: %v", cleanErr)
	} else if cleaned > 0 {
		s.logger.Infof("已清理 %d 个空壳合集", cleaned)
	}

	return created, nil
}

// MergeDuplicateCollections 合并所有同名重复合集
// 对于每组同名合集，保留最早创建的那个，将其他合集的电影迁移过来，然后删除重复的空壳
// 返回合并的组数
func (s *CollectionService) MergeDuplicateCollections() (int, error) {
	duplicateNames, err := s.collRepo.FindDuplicateNames()
	if err != nil {
		return 0, err
	}

	mergedGroups := 0
	for _, name := range duplicateNames {
		colls, err := s.collRepo.FindAllByName(name)
		if err != nil || len(colls) < 2 {
			continue
		}

		// 保留第一个（最早创建的）作为目标合集
		target := colls[0]
		sourceIDs := make([]string, 0, len(colls)-1)
		for _, c := range colls[1:] {
			sourceIDs = append(sourceIDs, c.ID)
		}

		// 如果目标合集没有海报，尝试从源合集中获取
		if target.PosterPath == "" {
			for _, c := range colls[1:] {
				if c.PosterPath != "" {
					target.PosterPath = c.PosterPath
					s.collRepo.Update(&target)
					break
				}
			}
		}

		// 合并：将源合集的电影迁移到目标合集，然后删除源合集
		if err := s.collRepo.MergeCollections(target.ID, sourceIDs); err != nil {
			s.logger.Warnf("合并同名合集 '%s' 失败: %v", name, err)
			continue
		}

		s.logger.Infof("已合并同名合集 '%s'：%d 个重复合集合并到 %s", name, len(sourceIDs), target.ID)
		mergedGroups++
	}

	return mergedGroups, nil
}

// CleanupEmptyCollections 清理所有无关联电影的空壳合集
// 返回被清理的合集数量
func (s *CollectionService) CleanupEmptyCollections() (int64, error) {
	cleaned, err := s.collRepo.CleanupEmptyCollections()
	if err != nil {
		return 0, err
	}
	if cleaned > 0 {
		s.logger.Infof("已清理 %d 个空壳合集", cleaned)
	}
	return cleaned, nil
}

// AddMediaToCollection 手动将电影添加到合集
func (s *CollectionService) AddMediaToCollection(collectionID, mediaID string) error {
	// 验证合集存在
	if _, err := s.collRepo.FindByID(collectionID); err != nil {
		return err
	}

	// 更新电影的合集关联
	if err := s.mediaRepo.UpdateFields(mediaID, map[string]interface{}{
		"collection_id": collectionID,
	}); err != nil {
		return err
	}

	// 更新合集计数
	return s.collRepo.UpdateMediaCount(collectionID)
}

// RemoveMediaFromCollection 从合集中移除电影
func (s *CollectionService) RemoveMediaFromCollection(collectionID, mediaID string) error {
	if err := s.mediaRepo.UpdateFields(mediaID, map[string]interface{}{
		"collection_id": "",
	}); err != nil {
		return err
	}
	return s.collRepo.UpdateMediaCount(collectionID)
}

// CreateCollection 手动创建合集
func (s *CollectionService) CreateCollection(name string, mediaIDs []string) (*model.MovieCollection, error) {
	coll := &model.MovieCollection{
		Name:        name,
		MediaCount:  len(mediaIDs),
		AutoMatched: false,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.collRepo.Create(coll); err != nil {
		return nil, err
	}

	// 关联电影
	for _, mid := range mediaIDs {
		s.mediaRepo.UpdateFields(mid, map[string]interface{}{
			"collection_id": coll.ID,
		})
	}

	// 使用第一部电影的海报
	if len(mediaIDs) > 0 {
		if media, err := s.mediaRepo.FindByID(mediaIDs[0]); err == nil {
			coll.PosterPath = media.PosterPath
		}
	}

	// 同步更新电影数量和年份范围
	s.collRepo.UpdateMediaCount(coll.ID)

	return coll, nil
}

// UpdateCollection 更新合集信息
func (s *CollectionService) UpdateCollection(id string, name, overview string) error {
	coll, err := s.collRepo.FindByID(id)
	if err != nil {
		return err
	}
	coll.Name = name
	coll.Overview = overview
	coll.UpdatedAt = time.Now()
	return s.collRepo.Update(coll)
}

// DeleteCollection 删除合集
func (s *CollectionService) DeleteCollection(id string) error {
	return s.collRepo.Delete(id)
}

// SearchCollections 搜索合集
func (s *CollectionService) SearchCollections(keyword string, limit int) ([]model.MovieCollection, error) {
	return s.collRepo.Search(keyword, limit)
}

// GetDuplicateCollectionStats 获取重复合集统计信息
func (s *CollectionService) GetDuplicateCollectionStats() (map[string]int, error) {
	duplicateNames, err := s.collRepo.FindDuplicateNames()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]int)
	for _, name := range duplicateNames {
		colls, err := s.collRepo.FindAllByName(name)
		if err == nil {
			stats[name] = len(colls)
		}
	}
	return stats, nil
}

// ReMatchCollections 重新匹配合集
// 清除所有自动匹配的合集关联和记录，然后重新执行自动匹配
// 手动创建的合集（auto_matched = false）及其关联会被保留
func (s *CollectionService) ReMatchCollections() (int, error) {
	// 第一步：清除所有自动匹配合集的电影关联
	cleared, err := s.collRepo.ClearAutoMatchedAssociations()
	if err != nil {
		s.logger.Warnf("清除自动匹配关联时出错: %v", err)
	} else if cleared > 0 {
		s.logger.Infof("已清除 %d 条自动匹配的电影关联", cleared)
	}

	// 第二步：删除所有自动匹配的合集记录
	deleted, err := s.collRepo.DeleteAutoMatchedCollections()
	if err != nil {
		s.logger.Warnf("删除自动匹配合集记录时出错: %v", err)
	} else if deleted > 0 {
		s.logger.Infof("已删除 %d 个自动匹配合集", deleted)
	}

	// 第三步：重新执行自动匹配
	created, err := s.AutoMatchCollections()
	if err != nil {
		return 0, err
	}

	s.logger.Infof("重新匹配完成：删除 %d 个旧合集，新建 %d 个合集", deleted, created)
	return created, nil
}

// ==================== 同片多版本去重 ====================

// deduplicateVersions 对一个合集候选分组内的电影做"同片多版本"折叠。
//
// 判定"是否为同一部电影"的优先级：
//  1. 两部都有 TMDB ID 且相等 → 同一部
//  2. 归一化标题相同 且 年份相同（且年份非 0）→ 同一部
//  3. 归一化标题相同 且 一方年份为 0 → 同一部（兜底，避免年份缺失导致漏合）
//
// 返回：
//   - uniqueMovies：每"部"电影只保留 1 条代表（选择有 TMDB ID / 有海报 / 文件更大的版本）
//   - versionMap：  被折叠的副本 → 主版本 ID 的映射（用于写入 version_group 字段）
//
// 注意：本函数不会修改数据库，只做内存计算。
func (s *CollectionService) deduplicateVersions(mediaList []model.Media) ([]model.Media, map[string]string) {
	if len(mediaList) <= 1 {
		return mediaList, nil
	}

	// 为了让"代表版本"的选择稳定，先按质量排序：
	//   - 有 TMDB ID 的优先
	//   - 有海报的优先
	//   - 文件体积大的优先（通常质量更高）
	//   - 最后按 ID 字典序兜底
	sorted := make([]model.Media, len(mediaList))
	copy(sorted, mediaList)
	sort.SliceStable(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		if (a.TMDbID > 0) != (b.TMDbID > 0) {
			return a.TMDbID > 0
		}
		if (a.PosterPath != "") != (b.PosterPath != "") {
			return a.PosterPath != ""
		}
		if a.FileSize != b.FileSize {
			return a.FileSize > b.FileSize
		}
		return a.ID < b.ID
	})

	// 两级 key：
	//   - tmdbKey：   "tmdb:<id>"
	//   - titleKey： "title:<归一化标题>|<year>"
	tmdbPrimary := make(map[string]string)  // tmdbKey  -> 主版本 ID
	titlePrimary := make(map[string]string) // titleKey -> 主版本 ID

	var uniqueMovies []model.Media
	versionMap := make(map[string]string) // 副本 media ID -> 主版本 media ID

	for _, m := range sorted {
		var primaryID string

		// 1) TMDB 强匹配
		if m.TMDbID > 0 {
			key := fmt.Sprintf("tmdb:%d", m.TMDbID)
			if pid, ok := tmdbPrimary[key]; ok {
				primaryID = pid
			} else {
				tmdbPrimary[key] = m.ID
			}
		}

		// 2) 标题 + 年份兜底
		if primaryID == "" {
			norm := matcher.NormalizeForCompare(m.Title)
			if norm != "" {
				// 先用精确年份 key
				if m.Year > 0 {
					key := fmt.Sprintf("title:%s|%d", norm, m.Year)
					if pid, ok := titlePrimary[key]; ok {
						primaryID = pid
					} else {
						titlePrimary[key] = m.ID
					}
				}
				// 再尝试"无年份" key（兜底，供年份缺失的副本匹配）
				if primaryID == "" {
					key := fmt.Sprintf("title:%s|0", norm)
					if pid, ok := titlePrimary[key]; ok {
						primaryID = pid
					} else {
						titlePrimary[key] = m.ID
					}
				}
			}
		}

		if primaryID != "" && primaryID != m.ID {
			// 被识别为已有主版本的副本
			versionMap[m.ID] = primaryID
		} else {
			// 自己就是主版本
			uniqueMovies = append(uniqueMovies, m)
		}
	}

	return uniqueMovies, versionMap
}

// persistVersionGroups 把 deduplicateVersions 识别出的"主-副版本关系"
// 写回到 media.version_group 字段（同一部电影的所有版本共享同一个 group ID）。
// group ID 直接使用主版本的 media.ID，无需额外生成 UUID。
func (s *CollectionService) persistVersionGroups(versionMap map[string]string) {
	if len(versionMap) == 0 {
		return
	}
	// 按主版本 ID 聚合所有成员
	groupMembers := make(map[string][]string)
	for copyID, primaryID := range versionMap {
		groupMembers[primaryID] = append(groupMembers[primaryID], copyID)
	}
	for primaryID, copies := range groupMembers {
		// 主版本自己也要写入同样的 group ID
		if err := s.mediaRepo.UpdateFields(primaryID, map[string]interface{}{
			"version_group": primaryID,
		}); err != nil {
			s.logger.Warnf("写入主版本 version_group 失败 id=%s: %v", primaryID, err)
		}
		for _, cid := range copies {
			if err := s.mediaRepo.UpdateFields(cid, map[string]interface{}{
				"version_group": primaryID,
			}); err != nil {
				s.logger.Warnf("写入副版本 version_group 失败 id=%s: %v", cid, err)
			}
		}
	}
}

// ==================== 标题匹配算法（薄包装） ====================
// 核心算法已抽取到 internal/matcher 包，供本服务和诊断脚本共同使用。
// 这里保留包级别的包装函数名称，以最小化对现有调用点的影响。

// extractSeriesBaseName 第一层：精确续集模式匹配。详见 matcher.ExtractSeriesBaseName。
func extractSeriesBaseName(title string) string {
	return matcher.ExtractSeriesBaseName(title)
}

// extractPrefixByDelimiter 第二层：连接词/人名后缀分割。详见 matcher.ExtractPrefixByDelimiter。
func extractPrefixByDelimiter(title string) string {
	return matcher.ExtractPrefixByDelimiter(title)
}

// extractBaseNameBySpaceSplit 第三层：通用空格分割兜底。详见 matcher.ExtractBaseNameBySpaceSplit。
func extractBaseNameBySpaceSplit(title string) string {
	return matcher.ExtractBaseNameBySpaceSplit(title)
}
