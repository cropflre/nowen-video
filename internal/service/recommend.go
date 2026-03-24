package service

import (
	"math"
	"sort"
	"strings"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// 推荐算法参数常量
const (
	// maxSampleUsers 协同过滤采样的最大用户数量（避免全量加载）
	maxSampleUsers = 100
	// topKUsers 参与推荐计算的Top-K相似用户数
	topKUsers = 20
	// similarityThreshold 余弦相似度阈值，低于此值的用户将被忽略
	similarityThreshold = 0.1
	// cfWeight 协同过滤在混合推荐中的权重
	cfWeight = 0.6
	// cbWeight 基于内容推荐在混合推荐中的权重
	cbWeight = 0.4
	// maxHistoryForContentRec 基于内容推荐时分析的最大历史记录数
	maxHistoryForContentRec = 50
)

// RecommendService 智能推荐服务（基于协同过滤）
type RecommendService struct {
	mediaRepo   *repository.MediaRepo
	historyRepo *repository.WatchHistoryRepo
	favRepo     *repository.FavoriteRepo
	logger      *zap.SugaredLogger
}

func NewRecommendService(
	mediaRepo *repository.MediaRepo,
	historyRepo *repository.WatchHistoryRepo,
	favRepo *repository.FavoriteRepo,
	logger *zap.SugaredLogger,
) *RecommendService {
	return &RecommendService{
		mediaRepo:   mediaRepo,
		historyRepo: historyRepo,
		favRepo:     favRepo,
		logger:      logger,
	}
}

// RecommendedMedia 推荐结果项
type RecommendedMedia struct {
	Media  model.Media `json:"media"`
	Score  float64     `json:"score"`  // 推荐得分
	Reason string      `json:"reason"` // 推荐理由
}

// GetRecommendations 获取个性化推荐列表
// 采用混合推荐策略：协同过滤 + 基于内容的推荐
func (s *RecommendService) GetRecommendations(userID string, limit int) ([]RecommendedMedia, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}

	// 1. 获取当前用户的观看历史
	userHistory, err := s.historyRepo.GetAllByUserID(userID)
	if err != nil {
		return nil, err
	}

	// 如果用户没有观看历史，返回热门推荐
	if len(userHistory) == 0 {
		return s.getPopularRecommendations(limit)
	}

	// 2. 获取最近活跃的用户（采样，最多100个用户，而非全量加载）
	activeUserIDs, err := s.historyRepo.GetActiveUserIDs(maxSampleUsers)
	if err != nil {
		s.logger.Warnf("获取活跃用户失败，降级到内容推荐: %v", err)
		return s.getContentBasedRecommendations(userID, userHistory, limit)
	}

	// 3. 获取这些用户的观看记录（采样而非全量）
	sampledHistory, err := s.historyRepo.GetHistoryByUserIDs(activeUserIDs)
	if err != nil {
		s.logger.Warnf("获取采样观看历史失败，降级到内容推荐: %v", err)
		return s.getContentBasedRecommendations(userID, userHistory, limit)
	}

	// 4. 构建用户-物品评分矩阵（基于采样数据）
	userRatings := s.buildRatingMatrix(sampledHistory)

	// 5. 计算协同过滤推荐
	cfResults := s.collaborativeFilter(userID, userRatings, limit*2)

	// 6. 计算基于内容的推荐
	cbResults, _ := s.getContentBasedRecommendations(userID, userHistory, limit*2)

	// 7. 混合推荐结果（协同过滤权重0.6 + 内容推荐权重0.4）
	merged := s.mergeRecommendations(cfResults, cbResults, cfWeight, cbWeight)

	// 8. 过滤已观看的内容
	watchedSet := make(map[string]bool)
	for _, h := range userHistory {
		watchedSet[h.MediaID] = true
	}

	var results []RecommendedMedia
	for _, item := range merged {
		if watchedSet[item.Media.ID] {
			continue
		}
		results = append(results, item)
		if len(results) >= limit {
			break
		}
	}

	return results, nil
}

// buildRatingMatrix 构建用户-物品评分矩阵
// 评分规则：观看完成=5分, 观看>50%=4分, 观看>20%=3分, 有记录=2分, 收藏=+1分
func (s *RecommendService) buildRatingMatrix(allHistory []model.WatchHistory) map[string]map[string]float64 {
	ratings := make(map[string]map[string]float64)

	for _, h := range allHistory {
		if ratings[h.UserID] == nil {
			ratings[h.UserID] = make(map[string]float64)
		}

		var score float64
		if h.Completed {
			score = 5.0
		} else if h.Duration > 0 {
			progress := h.Position / h.Duration
			if progress > 0.5 {
				score = 4.0
			} else if progress > 0.2 {
				score = 3.0
			} else {
				score = 2.0
			}
		} else {
			score = 2.0
		}

		// 取最高分（同一用户可能多次观看）
		if existing, ok := ratings[h.UserID][h.MediaID]; !ok || score > existing {
			ratings[h.UserID][h.MediaID] = score
		}
	}

	return ratings
}

// collaborativeFilter 基于用户的协同过滤
func (s *RecommendService) collaborativeFilter(
	targetUserID string,
	userRatings map[string]map[string]float64,
	limit int,
) []RecommendedMedia {
	targetRatings, exists := userRatings[targetUserID]
	if !exists {
		return nil
	}

	// 计算目标用户与其他用户的相似度（余弦相似度）
	type userSim struct {
		userID     string
		similarity float64
	}

	var similarities []userSim
	for otherUserID, otherRatings := range userRatings {
		if otherUserID == targetUserID {
			continue
		}
		sim := s.cosineSimilarity(targetRatings, otherRatings)
		if sim > similarityThreshold { // 只考虑相似度大于阈值的用户
			similarities = append(similarities, userSim{otherUserID, sim})
		}
	}

	// 按相似度降序排序
	sort.Slice(similarities, func(i, j int) bool {
		return similarities[i].similarity > similarities[j].similarity
	})

	// 取Top-K相似用户
	topK := topKUsers
	if len(similarities) < topK {
		topK = len(similarities)
	}
	similarities = similarities[:topK]

	// 计算推荐分数：加权求和（相似度 × 评分）
	mediaScores := make(map[string]float64)
	mediaSimSum := make(map[string]float64)

	for _, sim := range similarities {
		otherRatings := userRatings[sim.userID]
		for mediaID, rating := range otherRatings {
			if _, watched := targetRatings[mediaID]; watched {
				continue // 跳过已看过的
			}
			mediaScores[mediaID] += sim.similarity * rating
			mediaSimSum[mediaID] += math.Abs(sim.similarity)
		}
	}

	// 归一化分数
	type mediaScore struct {
		mediaID string
		score   float64
	}
	var scored []mediaScore
	for mediaID, score := range mediaScores {
		if mediaSimSum[mediaID] > 0 {
			scored = append(scored, mediaScore{mediaID, score / mediaSimSum[mediaID]})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	// 批量查询媒体详情（避免 N+1 查询）
	mediaIDs := make([]string, 0, len(scored))
	for _, item := range scored {
		mediaIDs = append(mediaIDs, item.mediaID)
	}

	mediaList, err := s.mediaRepo.FindByIDs(mediaIDs)
	if err != nil {
		s.logger.Warnf("批量查询媒体失败: %v", err)
		return nil
	}

	// 构建 ID 到媒体的映射
	mediaMap := make(map[string]model.Media, len(mediaList))
	for _, m := range mediaList {
		mediaMap[m.ID] = m
	}

	var results []RecommendedMedia
	for _, item := range scored {
		if media, ok := mediaMap[item.mediaID]; ok {
			results = append(results, RecommendedMedia{
				Media:  media,
				Score:  item.score,
				Reason: "与你口味相似的用户也在看",
			})
		}
	}

	return results
}

// cosineSimilarity 计算两个用户评分向量的余弦相似度
func (s *RecommendService) cosineSimilarity(a, b map[string]float64) float64 {
	var dotProduct, normA, normB float64

	// 只计算共同评分的物品
	for mediaID, ratingA := range a {
		if ratingB, ok := b[mediaID]; ok {
			dotProduct += ratingA * ratingB
		}
		normA += ratingA * ratingA
	}

	for _, ratingB := range b {
		normB += ratingB * ratingB
	}

	if normA == 0 || normB == 0 {
		return 0
	}

	return dotProduct / (math.Sqrt(normA) * math.Sqrt(normB))
}

// getContentBasedRecommendations 基于内容的推荐（利用类型/年份相似度）
func (s *RecommendService) getContentBasedRecommendations(
	userID string,
	userHistory []model.WatchHistory,
	limit int,
) ([]RecommendedMedia, error) {
	// 统计用户偏好的类型
	genreCount := make(map[string]int)
	var watchedIDs []string

	// 只取最近的观看记录进行分析（避免大量历史无谓查询）
	historyLimit := len(userHistory)
	if historyLimit > maxHistoryForContentRec {
		historyLimit = maxHistoryForContentRec
	}

	// 收集需要查询的媒体ID
	mediaIDsToQuery := make([]string, 0, historyLimit)
	for i := 0; i < historyLimit; i++ {
		watchedIDs = append(watchedIDs, userHistory[i].MediaID)
		mediaIDsToQuery = append(mediaIDsToQuery, userHistory[i].MediaID)
	}

	// 批量查询媒体信息（避免 N+1 查询）
	watchedMedia, err := s.mediaRepo.FindByIDs(mediaIDsToQuery)
	if err != nil {
		return nil, err
	}

	// 构建 completed 集合用于权重计算
	completedSet := make(map[string]bool)
	for _, h := range userHistory {
		if h.Completed {
			completedSet[h.MediaID] = true
		}
	}

	for _, media := range watchedMedia {
		if media.Genres != "" {
			for _, genre := range strings.Split(media.Genres, ",") {
				genre = strings.TrimSpace(genre)
				if genre != "" {
					weight := 1
					if completedSet[media.ID] {
						weight = 3 // 完整看完的权重更高
					}
					genreCount[genre] += weight
				}
			}
		}
	}

	// 找出最喜欢的类型（取前3）
	type genreWeight struct {
		genre  string
		weight int
	}
	var sortedGenres []genreWeight
	for g, w := range genreCount {
		sortedGenres = append(sortedGenres, genreWeight{g, w})
	}
	sort.Slice(sortedGenres, func(i, j int) bool {
		return sortedGenres[i].weight > sortedGenres[j].weight
	})

	topGenres := make([]string, 0, 3)
	topGenreWeights := make(map[string]float64)
	for i, gw := range sortedGenres {
		if i >= 3 {
			break
		}
		topGenres = append(topGenres, gw.genre)
		topGenreWeights[gw.genre] = float64(gw.weight)
	}

	if len(topGenres) == 0 {
		return nil, nil
	}

	// 使用数据库查询按类型检索，而非全量加载 500 个媒体
	candidates, err := s.mediaRepo.ListByGenres(topGenres, watchedIDs, limit*3)
	if err != nil {
		return nil, err
	}

	var results []RecommendedMedia
	for _, media := range candidates {
		score := s.calculateContentScore(media, topGenreWeights)
		if score > 0 {
			reason := "基于你喜欢的类型推荐"
			if len(sortedGenres) > 0 {
				reason = "因为你喜欢「" + sortedGenres[0].genre + "」类影片"
			}
			results = append(results, RecommendedMedia{
				Media:  media,
				Score:  score,
				Reason: reason,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// calculateContentScore 计算单个媒体的内容匹配得分
func (s *RecommendService) calculateContentScore(media model.Media, topGenres map[string]float64) float64 {
	if media.Genres == "" {
		return 0
	}

	var score float64
	genres := strings.Split(media.Genres, ",")
	for _, genre := range genres {
		genre = strings.TrimSpace(genre)
		if weight, ok := topGenres[genre]; ok {
			score += weight
		}
	}

	// 评分加成：高评分媒体优先
	if media.Rating > 0 {
		score *= (1 + media.Rating/20) // 评分越高加成越大
	}

	return score
}

// mergeRecommendations 合并协同过滤和内容推荐结果
func (s *RecommendService) mergeRecommendations(
	cfResults []RecommendedMedia,
	cbResults []RecommendedMedia,
	cfWeight, cbWeight float64,
) []RecommendedMedia {
	scoreMap := make(map[string]*RecommendedMedia)

	// 归一化协同过滤分数
	cfMax := 0.0
	for _, r := range cfResults {
		if r.Score > cfMax {
			cfMax = r.Score
		}
	}
	for _, r := range cfResults {
		normalizedScore := 0.0
		if cfMax > 0 {
			normalizedScore = (r.Score / cfMax) * cfWeight
		}
		item := r
		item.Score = normalizedScore
		scoreMap[r.Media.ID] = &item
	}

	// 归一化内容推荐分数
	cbMax := 0.0
	for _, r := range cbResults {
		if r.Score > cbMax {
			cbMax = r.Score
		}
	}
	for _, r := range cbResults {
		normalizedScore := 0.0
		if cbMax > 0 {
			normalizedScore = (r.Score / cbMax) * cbWeight
		}
		if existing, ok := scoreMap[r.Media.ID]; ok {
			existing.Score += normalizedScore
			// 保留协同过滤的推荐理由（优先级更高）
		} else {
			item := r
			item.Score = normalizedScore
			scoreMap[r.Media.ID] = &item
		}
	}

	// 转换为切片并排序
	var merged []RecommendedMedia
	for _, item := range scoreMap {
		merged = append(merged, *item)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	return merged
}

// getPopularRecommendations 热门推荐（无观看历史时使用）
func (s *RecommendService) getPopularRecommendations(limit int) ([]RecommendedMedia, error) {
	// 获取被最多用户观看/收藏的媒体
	popularMedia, err := s.historyRepo.GetMostWatched(limit)
	if err != nil {
		// 降级为最新媒体
		media, err := s.mediaRepo.Recent(limit)
		if err != nil {
			return nil, err
		}
		var results []RecommendedMedia
		for _, m := range media {
			results = append(results, RecommendedMedia{
				Media:  m,
				Score:  0,
				Reason: "最新上架",
			})
		}
		return results, nil
	}

	// 批量查询媒体详情（避免 N+1 查询）
	mediaIDs := make([]string, 0, len(popularMedia))
	for _, pm := range popularMedia {
		mediaIDs = append(mediaIDs, pm.MediaID)
	}

	mediaList, err := s.mediaRepo.FindByIDs(mediaIDs)
	if err != nil {
		return nil, err
	}

	mediaMap := make(map[string]model.Media, len(mediaList))
	for _, m := range mediaList {
		mediaMap[m.ID] = m
	}

	var results []RecommendedMedia
	for _, pm := range popularMedia {
		if media, ok := mediaMap[pm.MediaID]; ok {
			results = append(results, RecommendedMedia{
				Media:  media,
				Score:  float64(pm.WatchCount),
				Reason: "热门推荐",
			})
		}
	}

	return results, nil
}
