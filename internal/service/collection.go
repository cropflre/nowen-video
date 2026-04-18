package service

import (
	"regexp"
	"strings"
	"time"
	"unicode"

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
	ID         string  `json:"id"`
	Title      string  `json:"title"`
	OrigTitle  string  `json:"orig_title"`
	Year       int     `json:"year"`
	Premiered  string  `json:"premiered"`
	Rating     float64 `json:"rating"`
	PosterPath string  `json:"poster_path"`
	Runtime    int     `json:"runtime"`
	Overview   string  `json:"overview"`
	Genres     string  `json:"genres"`
	IsCurrent  bool    `json:"is_current"` // 是否为当前正在查看的电影
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
			ID:         m.ID,
			Title:      m.Title,
			OrigTitle:  m.OrigTitle,
			Year:       m.Year,
			Premiered:  m.Premiered,
			Rating:     m.Rating,
			PosterPath: m.PosterPath,
			Runtime:    m.Runtime,
			Overview:   m.Overview,
			Genres:     m.Genres,
			IsCurrent:  m.ID == mediaID,
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
			ID:         m.ID,
			Title:      m.Title,
			OrigTitle:  m.OrigTitle,
			Year:       m.Year,
			Premiered:  m.Premiered,
			Rating:     m.Rating,
			PosterPath: m.PosterPath,
			Runtime:    m.Runtime,
			Overview:   m.Overview,
			Genres:     m.Genres,
		})
	}

	// 清空 Preload 填充的 Media 字段，避免 JSON 序列化时 collection 内部重复包含 media 数组
	coll.Media = nil

	return result, nil
}

// ListCollections 分页获取合集列表
// 同时确保每个合集都有封面海报（使用第一部电影的海报）
func (s *CollectionService) ListCollections(page, size int) ([]model.MovieCollection, int64, error) {
	colls, total, err := s.collRepo.List(page, size)
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
	groups := make(map[string][]model.Media)
	var unmatchedMovies []model.Media // 第一层未匹配到的电影，留给第二层处理

	for _, m := range movies {
		baseName := extractSeriesBaseName(m.Title)
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

	created := 0
	for baseName, mediaList := range groups {
		// 修复点2：先检查数据库中是否已经存在同名合集（模糊匹配，去除空格/标点/全半角差异）
		existing, err := s.collRepo.FindByNameFuzzy(baseName)

		// 修复点3：只有当合集"不存在"且"待关联电影少于2部"时，才跳过（不创建新合集）
		if (err != nil || existing == nil) && len(mediaList) < 2 {
			continue
		}

		// 修复点4：如果合集已存在，即使本次只匹配到 1 部电影，也追加进去
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
				s.logger.Infof("向已有合集 '%s' 自动追加了 %d 部电影", baseName, len(mediaList))
			}
			continue
		}

		// 创建新合集（只有待关联电影 >= 2 部才会走到这里）
		coll := &model.MovieCollection{
			Name:        baseName,
			PosterPath:  mediaList[0].PosterPath, // 使用第一部电影的海报
			MediaCount:  len(mediaList),
			AutoMatched: true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		if err := s.collRepo.Create(coll); err != nil {
			s.logger.Warnf("创建合集 '%s' 失败: %v", baseName, err)
			continue
		}

		// 关联电影
		for _, m := range mediaList {
			s.mediaRepo.UpdateFields(m.ID, map[string]interface{}{
				"collection_id": coll.ID,
			})
		}

		s.logger.Infof("自动创建合集 '%s'，包含 %d 部电影", baseName, len(mediaList))
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

// ==================== 标题匹配算法 ====================

// 用于匹配标题中的数字序号模式
var (
	// 匹配中文数字序号：逃学威龙1、逃学威龙2、速度与激情3
	reChineseSequel = regexp.MustCompile(`^(.{2,})\s*[0-9０-９一二三四五六七八九十百]+\s*$`)
	// 匹配英文续集模式：Toy Story 2, Iron Man 3, Fast & Furious 7
	reEnglishSequel = regexp.MustCompile(`(?i)^(.{2,})\s+(\d+|[IVX]+|Part\s+\d+|Chapter\s+\d+)\s*$`)
	// 匹配带冒号的续集：Alien: Resurrection, Batman: The Dark Knight
	reColonSequel = regexp.MustCompile(`^(.{2,})\s*[:：]\s*.+$`)
	// 匹配括号中的年份或编号：电影名 (2020)
	reParenSuffix = regexp.MustCompile(`^(.{2,})\s*[（(]\s*(?:\d{4}|\d+)\s*[）)]\s*$`)
	// 匹配罗马数字后缀
	reRomanSuffix = regexp.MustCompile(`(?i)^(.{2,})\s+(?:II|III|IV|V|VI|VII|VIII|IX|X|XI|XII)\s*$`)

	// ===== 第二层：连接词分割模式 =====
	// 匹配中文连接词模式："哈哈哈之我真是醉了"、"名侦探柯南之xxx"、"熊出没·原始时代"
	// 支持的连接词：之、的、·、—、-
	// "的"作为连接词时，要求后半部分至少3字（避免把"我的家"拆成"我"+"家"）
	reChineseDelimiter = regexp.MustCompile(`^(.{2,}?)(之|[·•]|\s*[—–-]\s*)(.{2,})$`)
	reChineseDelimiterDe = regexp.MustCompile(`^(.{2,}?)的(.{3,})$`)
	// 匹配英文分隔符模式："Harry Potter - The Chamber of Secrets"
	reEnglishDelimiter = regexp.MustCompile(`(?i)^(.{2,}?)\s*[-–—:：]\s+(.{2,})$`)

	// ===== 第二层补充：人名/副标题后缀模式 =====
	// 匹配"基础名 + 空格 + XX编/篇/版/章/辑/卷/期"模式
	// 例如："少女教育 稻垣纱衣编" -> "少女教育"
	//       "少女教育 雏田麻未编" -> "少女教育"
	//       "世界遗产 第X卷"      -> "世界遗产"
	//       "名作剧场 第X期"      -> "名作剧场"
	// 后缀词至少2个字符（人名），避免误拆
	rePersonSuffix = regexp.MustCompile(`^(.{2,}?)\s+(.{2,}(?:编|篇|版|章|辑|卷|期|作|風|style|edition))\s*$`)

	// ===== 第三层：通用空格分割模式 =====
	// 匹配"基础名 + 空格 + 任意后缀"的通用模式
	// 这是最后一道防线，只要标题中包含空格且基础名合法，就提取空格前的基础名
	// 例如："少女教育 稻垣纱衣" -> "少女教育"
	//       "少女教育 雏田麻未" -> "少女教育"
	//       "Super Hero Max"    -> "Super Hero"
	// 要求：基础名 >= 2字符，后缀 >= 2字符
	reSpaceSplit = regexp.MustCompile(`^(.{2,}?)\s+(.{2,})\s*$`)
)

// extractSeriesBaseName 从电影标题中提取系列基础名（第一层：精确模式匹配）
// 例如：
//   - "逃学威龙1" -> "逃学威龙"
//   - "逃学威龙2" -> "逃学威龙"
//   - "速度与激情7" -> "速度与激情"
//   - "Toy Story 2" -> "Toy Story"
//   - "Iron Man 3" -> "Iron Man"
//   - "The Godfather Part II" -> "The Godfather"
func extractSeriesBaseName(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	// 尝试各种模式匹配
	patterns := []*regexp.Regexp{
		reChineseSequel,
		reEnglishSequel,
		reRomanSuffix,
		reParenSuffix,
		reColonSequel,
	}

	for _, pattern := range patterns {
		if matches := pattern.FindStringSubmatch(title); len(matches) >= 2 {
			baseName := strings.TrimSpace(matches[1])
			// 基础名至少 2 个字符
			if len([]rune(baseName)) >= 2 {
				return normalizeBaseName(baseName)
			}
		}
	}

	return ""
}

// extractPrefixByDelimiter 从电影标题中提取前缀（第二层：连接词分割法）
// 通过识别中文连接词（之、的、·、—）来分割标题，提取系列前缀
// 例如：
//   - "哈哈哈之我真是醉了" -> "哈哈哈"
//   - "哈哈哈之我也无奈"   -> "哈哈哈"
//   - "名侦探柯南之xxx"   -> "名侦探柯南"
//   - "熊出没·原始时代"   -> "熊出没"
//   - "新大头儿子和小头爸爸之xxx" -> "新大头儿子和小头爸爸"
func extractPrefixByDelimiter(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	// 尝试中文连接词分割（之、·、—、-）
	if matches := reChineseDelimiter.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix)
		}
	}

	// 尝试"的"连接词分割（要求后半部分至少3字，避免把"我的家"拆成"我"+"家"）
	if matches := reChineseDelimiterDe.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix)
		}
	}

	// 尝试英文分隔符分割
	if matches := reEnglishDelimiter.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix)
		}
	}

	// 尝试人名/副标题后缀分割
	// 例如："少女教育 稻垣纱衣编" -> "少女教育"
	if matches := rePersonSuffix.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix)
		}
	}

	return ""
}

// extractBaseNameBySpaceSplit 第三层：通用空格分割
// 只要标题中包含空格，就提取空格前的基础名
// 这是最后防线，在第一层（序号）和第二层（连接词/后缀）都无法匹配时使用
// 例如："少女教育 稻垣纱衣" -> "少女教育"
//       "Super Hero Max"    -> "Super Hero"
func extractBaseNameBySpaceSplit(title string) string {
	title = strings.TrimSpace(title)
	if title == "" {
		return ""
	}

	if matches := reSpaceSplit.FindStringSubmatch(title); len(matches) >= 2 {
		prefix := strings.TrimSpace(matches[1])
		if len([]rune(prefix)) >= 2 {
			return normalizeBaseName(prefix)
		}
	}

	return ""
}

// normalizeBaseName 标准化基础名（去除尾部标点、空格等）
func normalizeBaseName(name string) string {
	name = strings.TrimSpace(name)
	// 去除尾部的常见分隔符
	name = strings.TrimRight(name, " -_·.、，,")
	// 去除尾部的冒号
	name = strings.TrimRight(name, ":：")
	name = strings.TrimSpace(name)

	// 如果全是标点或空白，返回空
	allPunct := true
	for _, r := range name {
		if !unicode.IsPunct(r) && !unicode.IsSpace(r) {
			allPunct = false
			break
		}
	}
	if allPunct {
		return ""
	}

	return name
}
