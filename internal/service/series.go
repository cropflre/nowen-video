package service

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// SeriesService 剧集合集服务
type SeriesService struct {
	seriesRepo      *repository.SeriesRepo
	mediaRepo       *repository.MediaRepo
	mediaPersonRepo *repository.MediaPersonRepo
	logger          *zap.SugaredLogger
}

func NewSeriesService(seriesRepo *repository.SeriesRepo, mediaRepo *repository.MediaRepo, logger *zap.SugaredLogger) *SeriesService {
	return &SeriesService{
		seriesRepo: seriesRepo,
		mediaRepo:  mediaRepo,
		logger:     logger,
	}
}

// SetMediaPersonRepo 延迟注入 MediaPersonRepo（避免循环依赖）
func (s *SeriesService) SetMediaPersonRepo(repo *repository.MediaPersonRepo) {
	s.mediaPersonRepo = repo
}

// ListSeries 获取剧集合集列表（分页）
func (s *SeriesService) ListSeries(page, size int, libraryID string) ([]model.Series, int64, error) {
	return s.seriesRepo.List(page, size, libraryID)
}

// GetSeriesDetail 获取剧集合集详情（含所有剧集）
func (s *SeriesService) GetSeriesDetail(id string) (*model.Series, error) {
	series, err := s.seriesRepo.FindByID(id)
	if err != nil {
		return nil, ErrMediaNotFound
	}
	return series, nil
}

// GetSeasons 获取剧集合集的季列表
func (s *SeriesService) GetSeasons(seriesID string) ([]SeasonInfo, error) {
	seasons, err := s.seriesRepo.GetSeasonNumbers(seriesID)
	if err != nil {
		return nil, err
	}

	var result []SeasonInfo
	for _, num := range seasons {
		episodes, err := s.mediaRepo.ListBySeriesAndSeason(seriesID, num)
		if err != nil {
			continue
		}
		result = append(result, SeasonInfo{
			SeasonNum:    num,
			EpisodeCount: len(episodes),
			Episodes:     episodes,
		})
	}

	return result, nil
}

// GetSeasonEpisodes 获取指定季的所有剧集
func (s *SeriesService) GetSeasonEpisodes(seriesID string, seasonNum int) ([]model.Media, error) {
	return s.mediaRepo.ListBySeriesAndSeason(seriesID, seasonNum)
}

// GetAllEpisodes 获取合集的所有剧集（按播放顺序排序）
func (s *SeriesService) GetAllEpisodes(seriesID string) ([]model.Media, error) {
	return s.mediaRepo.ListBySeriesID(seriesID)
}

// GetNextEpisode 获取下一集（用于连续播放）
func (s *SeriesService) GetNextEpisode(seriesID string, currentSeason, currentEpisode int) (*model.Media, error) {
	episodes, err := s.mediaRepo.ListBySeriesID(seriesID)
	if err != nil {
		return nil, err
	}

	// 找到当前集的位置，返回下一集
	found := false
	for _, ep := range episodes {
		if found {
			return &ep, nil
		}
		if ep.SeasonNum == currentSeason && ep.EpisodeNum == currentEpisode {
			found = true
		}
	}

	return nil, nil // 已经是最后一集
}

// GetSeriesPosterPath 获取剧集合集海报文件路径
func (s *SeriesService) GetSeriesPosterPath(id string) (string, error) {
	series, err := s.seriesRepo.FindByIDOnly(id)
	if err != nil {
		return "", ErrMediaNotFound
	}

	// 优先使用数据库中存储的海报路径
	if series.PosterPath != "" {
		if _, err := os.Stat(series.PosterPath); err == nil {
			return series.PosterPath, nil
		}
	}

	// 查找剧集根目录下的海报文件
	if series.FolderPath != "" {
		posterExts := []string{".jpg", ".jpeg", ".png", ".webp"}
		posterNames := []string{"poster", "cover", "folder", "show"}
		for _, name := range posterNames {
			for _, ext := range posterExts {
				candidate := filepath.Join(series.FolderPath, name+ext)
				if _, err := os.Stat(candidate); err == nil {
					return candidate, nil
				}
			}
		}
	}

	return "", nil
}

// SeasonInfo 季信息
type SeasonInfo struct {
	SeasonNum    int           `json:"season_num"`
	EpisodeCount int           `json:"episode_count"`
	Episodes     []model.Media `json:"episodes"`
}

// MergeResult 合并操作结果
type MergeResult struct {
	PrimarySeriesID string   `json:"primary_series_id"`
	PrimaryTitle    string   `json:"primary_title"`
	MergedCount     int      `json:"merged_count"`      // 被合并的 Series 数量
	TotalEpisodes   int      `json:"total_episodes"`    // 合并后总集数
	TotalSeasons    int      `json:"total_seasons"`     // 合并后总季数
	MergedSeriesIDs []string `json:"merged_series_ids"` // 被合并的 Series ID 列表
}

// MergeSeries 将多个 Series 合并为一个
// primaryID: 保留的主 Series ID，其余 Series 的剧集将迁移到主 Series 下
// secondaryIDs: 需要被合并进主 Series 的 Series ID 列表
func (s *SeriesService) MergeSeries(primaryID string, secondaryIDs []string) (*MergeResult, error) {
	if len(secondaryIDs) == 0 {
		return nil, fmt.Errorf("没有需要合并的剧集")
	}

	// 获取主 Series
	primary, err := s.seriesRepo.FindByID(primaryID)
	if err != nil {
		return nil, fmt.Errorf("找不到主剧集: %w", err)
	}

	result := &MergeResult{
		PrimarySeriesID: primaryID,
		PrimaryTitle:    primary.Title,
		MergedSeriesIDs: make([]string, 0),
	}

	db := s.seriesRepo.DB()

	for _, secID := range secondaryIDs {
		if secID == primaryID {
			continue // 跳过自身
		}

		secondary, err := s.seriesRepo.FindByID(secID)
		if err != nil {
			s.logger.Warnf("合并时找不到从属剧集 %s: %v", secID, err)
			continue
		}

		s.logger.Infof("合并剧集: [%s] %s → [%s] %s",
			secondary.ID, secondary.Title, primary.ID, primary.Title)

		// 1. 迁移所有 Media 的 SeriesID
		// 确保 SeasonNum 不冲突：检查从属 Series 的季号是否需要调整
		secondaryEpisodes, _ := s.mediaRepo.ListBySeriesID(secID)
		for _, ep := range secondaryEpisodes {
			// 如果从属系列的 SeasonNum 为 0 或 1，需要检查是否与主系列冲突
			// 通过从属 Series 标题中的季号标识来分配正确的季号
			seasonNum := ep.SeasonNum
			if seasonNum == 0 {
				seasonNum = s.extractSeasonFromTitle(secondary.Title)
				if seasonNum == 0 {
					seasonNum = 1
				}
			}
			// 如果原始季号为 1 且从属标题中有明确的季号标识，使用标题中的季号
			if ep.SeasonNum <= 1 {
				titleSeason := s.extractSeasonFromTitle(secondary.Title)
				if titleSeason > 0 {
					seasonNum = titleSeason
				}
			}
			ep.SeriesID = primaryID
			ep.SeasonNum = seasonNum
			s.mediaRepo.Update(&ep)
		}

		// 2. 迁移 MediaPerson 关联
		if s.mediaPersonRepo != nil {
			db.Model(&model.MediaPerson{}).
				Where("series_id = ?", secID).
				Update("series_id", primaryID)
		}

		// 3. 迁移 ShareLink 关联
		db.Exec("UPDATE share_links SET series_id = ? WHERE series_id = ?", primaryID, secID)

		// 4. 补充主 Series 的元数据（如果主 Series 缺少某些字段，从从属 Series 补充）
		if primary.Overview == "" && secondary.Overview != "" {
			primary.Overview = secondary.Overview
		}
		if primary.PosterPath == "" && secondary.PosterPath != "" {
			primary.PosterPath = secondary.PosterPath
		}
		if primary.BackdropPath == "" && secondary.BackdropPath != "" {
			primary.BackdropPath = secondary.BackdropPath
		}
		if primary.Rating == 0 && secondary.Rating > 0 {
			primary.Rating = secondary.Rating
		}
		if primary.Genres == "" && secondary.Genres != "" {
			primary.Genres = secondary.Genres
		}
		if primary.TMDbID == 0 && secondary.TMDbID > 0 {
			primary.TMDbID = secondary.TMDbID
		}
		if primary.DoubanID == "" && secondary.DoubanID != "" {
			primary.DoubanID = secondary.DoubanID
		}
		if primary.BangumiID == 0 && secondary.BangumiID > 0 {
			primary.BangumiID = secondary.BangumiID
		}
		if primary.Country == "" && secondary.Country != "" {
			primary.Country = secondary.Country
		}
		if primary.Language == "" && secondary.Language != "" {
			primary.Language = secondary.Language
		}
		if primary.Studio == "" && secondary.Studio != "" {
			primary.Studio = secondary.Studio
		}
		if primary.Year == 0 && secondary.Year > 0 {
			primary.Year = secondary.Year
		}

		// 5. 删除从属 Series 记录
		s.seriesRepo.Delete(secID)
		result.MergedCount++
		result.MergedSeriesIDs = append(result.MergedSeriesIDs, secID)

		s.logger.Infof("已合并: %s (%d 集) → %s", secondary.Title, len(secondaryEpisodes), primary.Title)
	}

	// 6. 更新主 Series 的标题（去掉季号标识，使用纯系列名）
	cleanTitle := normalizeSeriesTitleForMerge(primary.Title)
	if cleanTitle != "" && cleanTitle != primary.Title {
		primary.Title = cleanTitle
		result.PrimaryTitle = cleanTitle
	}

	// 7. 更新主 Series 的 FolderPath 为虚拟路径（如果不是已有的虚拟路径）
	if !strings.Contains(primary.FolderPath, "__multi__:") && !strings.Contains(primary.FolderPath, "__loose__:") {
		dir := filepath.Dir(primary.FolderPath)
		primary.FolderPath = filepath.Join(dir, "__multi__:"+primary.Title)
	}

	// 8. 去重演职人员：合并后同一 person_id + role 可能出现多条记录
	if s.mediaPersonRepo != nil {
		removed, err := s.mediaPersonRepo.DeduplicateBySeriesID(primaryID)
		if err != nil {
			s.logger.Warnf("演职人员去重失败: %v", err)
		} else if removed > 0 {
			s.logger.Infof("已清理 %d 条重复的演职人员记录 (series_id=%s)", removed, primaryID)
		}
	}

	// 9. 重新统计季数和集数
	allEpisodes, _ := s.mediaRepo.ListBySeriesID(primaryID)
	seasonSet := make(map[int]bool)
	for _, ep := range allEpisodes {
		seasonSet[ep.SeasonNum] = true
	}
	primary.EpisodeCount = len(allEpisodes)
	primary.SeasonCount = len(seasonSet)
	primary.UpdatedAt = time.Now()
	s.seriesRepo.Update(primary)

	result.TotalEpisodes = primary.EpisodeCount
	result.TotalSeasons = primary.SeasonCount

	s.logger.Infof("合并完成: %s, 合并了 %d 个系列, 共 %d 季 %d 集",
		result.PrimaryTitle, result.MergedCount, result.TotalSeasons, result.TotalEpisodes)

	return result, nil
}

// AutoMergeDuplicates 自动扫描并合并所有重复/可合并的 Series
// 返回所有合并操作的结果列表
func (s *SeriesService) AutoMergeDuplicates() ([]MergeResult, error) {
	allSeries, err := s.seriesRepo.ListAllWithEpisodes()
	if err != nil {
		return nil, fmt.Errorf("获取剧集列表失败: %w", err)
	}

	// 按 libraryID + 标准化标题 分组
	type groupKey struct {
		LibraryID      string
		NormalizedName string
	}
	groups := make(map[groupKey][]model.Series)

	for _, ser := range allSeries {
		normalized := normalizeSeriesTitleForMerge(ser.Title)
		if normalized == "" {
			normalized = ser.Title
		}
		key := groupKey{LibraryID: ser.LibraryID, NormalizedName: normalized}
		groups[key] = append(groups[key], ser)
	}

	var results []MergeResult

	for key, group := range groups {
		if len(group) <= 1 {
			continue // 不需要合并
		}

		s.logger.Infof("发现可合并的系列组: %s (%d 个)", key.NormalizedName, len(group))

		// 选择主 Series：优先选择元数据最丰富的，其次选择创建时间最早的
		primaryIdx := 0
		bestScore := s.metadataScore(&group[0])
		for i := 1; i < len(group); i++ {
			score := s.metadataScore(&group[i])
			if score > bestScore {
				bestScore = score
				primaryIdx = i
			}
		}

		primaryID := group[primaryIdx].ID
		var secondaryIDs []string
		for i, ser := range group {
			if i != primaryIdx {
				secondaryIDs = append(secondaryIDs, ser.ID)
			}
		}

		result, err := s.MergeSeries(primaryID, secondaryIDs)
		if err != nil {
			s.logger.Warnf("自动合并失败 [%s]: %v", key.NormalizedName, err)
			continue
		}
		results = append(results, *result)
	}

	s.logger.Infof("自动合并完成: 共处理 %d 个系列组", len(results))
	return results, nil
}

// FindMergeCandidates 查找可合并的 Series 候选分组（预览，不执行合并）
func (s *SeriesService) FindMergeCandidates() ([][]model.Series, error) {
	allSeries, err := s.seriesRepo.ListAllWithEpisodes()
	if err != nil {
		return nil, fmt.Errorf("获取剧集列表失败: %w", err)
	}

	type groupKey struct {
		LibraryID      string
		NormalizedName string
	}
	groups := make(map[groupKey][]model.Series)

	for _, ser := range allSeries {
		normalized := normalizeSeriesTitleForMerge(ser.Title)
		if normalized == "" {
			normalized = ser.Title
		}
		key := groupKey{LibraryID: ser.LibraryID, NormalizedName: normalized}
		groups[key] = append(groups[key], ser)
	}

	var candidates [][]model.Series
	for _, group := range groups {
		if len(group) > 1 {
			candidates = append(candidates, group)
		}
	}

	return candidates, nil
}

// metadataScore 评估一个 Series 的元数据丰富程度（分数越高越适合作为主 Series）
func (s *SeriesService) metadataScore(ser *model.Series) int {
	score := 0
	if ser.Overview != "" {
		score += 3
	}
	if ser.PosterPath != "" {
		score += 3
	}
	if ser.BackdropPath != "" {
		score += 2
	}
	if ser.Rating > 0 {
		score += 2
	}
	if ser.Genres != "" {
		score += 1
	}
	if ser.TMDbID > 0 {
		score += 2
	}
	if ser.DoubanID != "" {
		score += 1
	}
	if ser.BangumiID > 0 {
		score += 1
	}
	if ser.Year > 0 {
		score += 1
	}
	if ser.Country != "" {
		score += 1
	}
	// 集数越多越好（更完整的数据）
	score += ser.EpisodeCount
	return score
}

// normalizeSeriesTitleForMerge 标准化系列标题：去掉季号标识，提取纯系列名
// 例如: "女神咖啡厅 第一季" → "女神咖啡厅", "一拳超人 S2" → "一拳超人"
// [C 方案] 叠加 NormalizeSeriesTitle 的广告/发行组/编码噪声清洗，让"报春鸟【傲仔压制】"能和"报春鸟"合并
func normalizeSeriesTitleForMerge(title string) string {
	// 先跑一次统一清洗（去【xxx压制】、去[站点]、去编码、剥离末尾季号等）
	if normalized := NormalizeSeriesTitle(title); normalized != "" {
		title = normalized
	}

	// 移除季号标识的正则模式（再保险一次，兼容历史命名）
	seasonPatterns := []*regexp.Regexp{
		regexp.MustCompile(`(?i)\s*S\d{1,2}\s*$`),                          // 末尾 S1, S02
		regexp.MustCompile(`(?i)\s*Season\s*\d{1,2}\s*$`),                  // 末尾 Season 1
		regexp.MustCompile(`\s*第\s*[一二三四五六七八九十\d]+\s*季\s*$`),               // 末尾 第一季, 第2季
		regexp.MustCompile(`(?i)\s*\(\s*Season\s*\d{1,2}\s*\)\s*$`),        // 末尾 (Season 1)
		regexp.MustCompile(`(?i)\s*【\s*第?\s*[一二三四五六七八九十\d]+\s*季?\s*】\s*$`), // 末尾 【第一季】
		regexp.MustCompile(`\s*第\s*[一二三四五六七八九十\d]+\s*部\s*$`),               // 末尾 第一部, 第2部
	}

	result := title
	for _, p := range seasonPatterns {
		result = p.ReplaceAllString(result, "")
	}

	result = strings.TrimSpace(result)
	if result == "" {
		return title // 如果标准化后为空，回退使用原始标题
	}
	return result
}

// extractSeasonFromTitle 从 Series 标题中提取季号
// 例如: "女神咖啡厅 第二季" → 2, "一拳超人 S2" → 2
func (s *SeriesService) extractSeasonFromTitle(title string) int {
	// S1, S02 格式
	if m := regexp.MustCompile(`(?i)\bS(\d{1,2})\b`).FindStringSubmatch(title); len(m) >= 2 {
		num := 0
		fmt.Sscanf(m[1], "%d", &num)
		if num > 0 && num <= 30 {
			return num
		}
	}
	// Season 1 格式
	if m := regexp.MustCompile(`(?i)\bSeason\s*(\d{1,2})\b`).FindStringSubmatch(title); len(m) >= 2 {
		num := 0
		fmt.Sscanf(m[1], "%d", &num)
		if num > 0 && num <= 30 {
			return num
		}
	}
	// 第1季 格式
	if m := regexp.MustCompile(`第\s*(\d{1,2})\s*季`).FindStringSubmatch(title); len(m) >= 2 {
		num := 0
		fmt.Sscanf(m[1], "%d", &num)
		if num > 0 && num <= 30 {
			return num
		}
	}
	// 中文数字季号
	chineseNums := map[string]int{
		"一": 1, "二": 2, "三": 3, "四": 4, "五": 5,
		"六": 6, "七": 7, "八": 8, "九": 9, "十": 10,
	}
	if m := regexp.MustCompile(`第\s*([一二三四五六七八九十]+)\s*季`).FindStringSubmatch(title); len(m) >= 2 {
		if num, ok := chineseNums[m[1]]; ok {
			return num
		}
	}
	// 第N部 格式（也视为季号）
	if m := regexp.MustCompile(`第\s*(\d{1,2})\s*部`).FindStringSubmatch(title); len(m) >= 2 {
		num := 0
		fmt.Sscanf(m[1], "%d", &num)
		if num > 0 && num <= 30 {
			return num
		}
	}
	if m := regexp.MustCompile(`第\s*([一二三四五六七八九十]+)\s*部`).FindStringSubmatch(title); len(m) >= 2 {
		if num, ok := chineseNums[m[1]]; ok {
			return num
		}
	}
	return 0
}
