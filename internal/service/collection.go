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
			Rating:     m.Rating,
			PosterPath: m.PosterPath,
			Runtime:    m.Runtime,
			Overview:   m.Overview,
			Genres:     m.Genres,
			IsCurrent:  m.ID == mediaID,
		})
	}

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
			Rating:     m.Rating,
			PosterPath: m.PosterPath,
			Runtime:    m.Runtime,
			Overview:   m.Overview,
			Genres:     m.Genres,
		})
	}

	return result, nil
}

// ListCollections 分页获取合集列表
func (s *CollectionService) ListCollections(page, size int) ([]model.MovieCollection, int64, error) {
	return s.collRepo.List(page, size)
}

// AutoMatchCollections 自动匹配电影系列合集
// 算法：
// 1. 获取所有没有合集的电影
// 2. 提取标题的"基础名"（去掉数字序号、年份等后缀）
// 3. 将相同基础名的电影归入同一合集
// 4. 只有 >= 2 部电影的才创建合集
func (s *CollectionService) AutoMatchCollections() (int, error) {
	movies, err := s.collRepo.ListMoviesWithoutCollection()
	if err != nil {
		return 0, err
	}

	// 按基础名分组
	groups := make(map[string][]model.Media)
	for _, m := range movies {
		baseName := extractSeriesBaseName(m.Title)
		if baseName == "" {
			continue
		}
		groups[baseName] = append(groups[baseName], m)
	}

	created := 0
	for baseName, mediaList := range groups {
		if len(mediaList) < 2 {
			continue
		}

		// 检查是否已存在同名合集
		existing, err := s.collRepo.FindByName(baseName)
		if err == nil && existing != nil {
			// 已存在，将未关联的电影加入
			for _, m := range mediaList {
				if m.CollectionID == "" {
					s.mediaRepo.UpdateFields(m.ID, map[string]interface{}{
						"collection_id": existing.ID,
					})
				}
			}
			s.collRepo.UpdateMediaCount(existing.ID)
			continue
		}

		// 创建新合集
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

	return created, nil
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
			s.collRepo.Update(coll)
		}
	}

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
)

// extractSeriesBaseName 从电影标题中提取系列基础名
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
