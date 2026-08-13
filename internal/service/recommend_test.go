package service

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/nowen-video/nowen-video/internal/model"
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// ==================== 推荐系统测试 ====================

func TestBuildRatingMatrix_TimeDecay(t *testing.T) {
	svc := &RecommendService{}

	now := time.Now()
	history := []model.WatchHistory{
		{
			UserID:    "user1",
			MediaID:   "media1",
			Position:  100,
			Duration:  100,
			Completed: true,
			UpdatedAt: now, // 今天看的
		},
		{
			UserID:    "user1",
			MediaID:   "media2",
			Position:  100,
			Duration:  100,
			Completed: true,
			UpdatedAt: now.AddDate(0, 0, -60), // 60天前看的
		},
	}

	ratings := svc.buildRatingMatrix(history)

	// 今天看的应该接近满分（5.0）
	recentScore := ratings["user1"]["media1"]
	// 60天前看的应该有明显衰减
	oldScore := ratings["user1"]["media2"]

	if recentScore <= oldScore {
		t.Errorf("时间衰减未生效: 最近评分 %.2f 应大于 60天前评分 %.2f", recentScore, oldScore)
	}

	// 60天前的衰减因子约为 exp(-0.023 * 60) ≈ 0.25
	expectedDecay := 0.25
	actualDecay := oldScore / 5.0
	if actualDecay > expectedDecay*1.5 || actualDecay < expectedDecay*0.5 {
		t.Errorf("时间衰减幅度异常: 实际衰减比 %.2f, 期望约 %.2f", actualDecay, expectedDecay)
	}

	t.Logf("时间衰减测试通过: 最近=%.2f, 60天前=%.2f, 衰减比=%.2f", recentScore, oldScore, actualDecay)
}

func TestGetRecommendationsRejectsDeletedMediaCache(t *testing.T) {
	db := setupTestDB(t)
	repos := repository.NewRepositories(db)
	logger := zap.NewNop().Sugar()

	library := model.Library{ID: "library-current", Name: "当前媒体库", Path: "/media/current", Type: "movie"}
	if err := db.Create(&library).Error; err != nil {
		t.Fatal(err)
	}
	current := model.Media{ID: "media-current", LibraryID: library.ID, Title: "当前影片", FilePath: "/media/current/movie.mp4", MediaType: "movie", Rating: 9}
	if err := db.Create(&current).Error; err != nil {
		t.Fatal(err)
	}

	stale := []RecommendedMedia{{Media: model.Media{ID: "media-deleted", Title: "已删除影片"}, Score: 10, Reason: "旧缓存"}}
	payload, err := json.Marshal(stale)
	if err != nil {
		t.Fatal(err)
	}
	if err := repos.RecommendCache.Set("user", string(payload), 60); err != nil {
		t.Fatal(err)
	}

	svc := NewRecommendService(repos.Media, repos.Series, repos.WatchHistory, repos.Favorite, repos.RecommendCache, logger)
	results, err := svc.GetRecommendations("user", 10)
	if err != nil {
		t.Fatalf("获取推荐失败: %v", err)
	}
	if len(results) != 1 || results[0].Media.ID != current.ID {
		t.Fatalf("应重新计算并返回当前媒体，实际=%+v", results)
	}
	if _, ok := repos.RecommendCache.Get("user"); ok {
		t.Fatal("失效的推荐缓存应被删除")
	}
}

func TestBuildRatingMatrix_ScoreRules(t *testing.T) {
	svc := &RecommendService{}
	now := time.Now()

	history := []model.WatchHistory{
		{UserID: "u1", MediaID: "m1", Completed: true, Duration: 100, Position: 100, UpdatedAt: now},
		{UserID: "u1", MediaID: "m2", Completed: false, Duration: 100, Position: 60, UpdatedAt: now},
		{UserID: "u1", MediaID: "m3", Completed: false, Duration: 100, Position: 30, UpdatedAt: now},
		{UserID: "u1", MediaID: "m4", Completed: false, Duration: 100, Position: 10, UpdatedAt: now},
	}

	ratings := svc.buildRatingMatrix(history)

	tests := []struct {
		mediaID  string
		minScore float64
		maxScore float64
		desc     string
	}{
		{"m1", 4.9, 5.1, "完整观看应接近5分"},
		{"m2", 3.9, 4.1, "观看>50%应接近4分"},
		{"m3", 2.9, 3.1, "观看>20%应接近3分"},
		{"m4", 1.9, 2.1, "观看<20%应接近2分"},
	}

	for _, tt := range tests {
		score := ratings["u1"][tt.mediaID]
		if score < tt.minScore || score > tt.maxScore {
			t.Errorf("%s: 评分 %.2f 不在期望范围 [%.1f, %.1f]", tt.desc, score, tt.minScore, tt.maxScore)
		}
	}
}

func TestCosineSimilarity(t *testing.T) {
	svc := &RecommendService{}

	tests := []struct {
		name     string
		a, b     map[string]float64
		expected float64
		delta    float64
	}{
		{
			name:     "完全相同",
			a:        map[string]float64{"m1": 5, "m2": 3},
			b:        map[string]float64{"m1": 5, "m2": 3},
			expected: 1.0,
			delta:    0.01,
		},
		{
			name:     "完全不同（无交集）",
			a:        map[string]float64{"m1": 5},
			b:        map[string]float64{"m2": 5},
			expected: 0.0,
			delta:    0.01,
		},
		{
			name:     "部分重叠",
			a:        map[string]float64{"m1": 5, "m2": 3, "m3": 1},
			b:        map[string]float64{"m1": 4, "m2": 2, "m4": 5},
			expected: 0.8,
			delta:    0.2,
		},
		{
			name:     "空向量",
			a:        map[string]float64{},
			b:        map[string]float64{"m1": 5},
			expected: 0.0,
			delta:    0.01,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := svc.cosineSimilarity(tt.a, tt.b)
			if result < tt.expected-tt.delta || result > tt.expected+tt.delta {
				t.Errorf("余弦相似度 = %.4f, 期望约 %.4f (±%.2f)", result, tt.expected, tt.delta)
			}
		})
	}
}

func TestCalculateContentScore(t *testing.T) {
	svc := &RecommendService{}

	topGenres := map[string]float64{
		"科幻": 10.0,
		"动作": 5.0,
	}

	tests := []struct {
		name     string
		media    model.Media
		minScore float64
	}{
		{
			name:     "完全匹配",
			media:    model.Media{Genres: "科幻,动作", Rating: 9.0},
			minScore: 10.0,
		},
		{
			name:     "部分匹配",
			media:    model.Media{Genres: "科幻,爱情", Rating: 8.0},
			minScore: 5.0,
		},
		{
			name:     "无匹配",
			media:    model.Media{Genres: "爱情,喜剧", Rating: 9.0},
			minScore: 0.0,
		},
		{
			name:     "空类型",
			media:    model.Media{Genres: "", Rating: 9.0},
			minScore: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score := svc.calculateContentScore(tt.media, topGenres)
			if score < tt.minScore {
				t.Errorf("内容评分 = %.2f, 期望 >= %.2f", score, tt.minScore)
			}
		})
	}
}

func TestMergeRecommendations(t *testing.T) {
	svc := &RecommendService{}

	cfResults := []RecommendedMedia{
		{Media: model.Media{ID: "m1", Title: "电影1"}, Score: 10.0, Reason: "协同过滤"},
		{Media: model.Media{ID: "m2", Title: "电影2"}, Score: 8.0, Reason: "协同过滤"},
	}

	cbResults := []RecommendedMedia{
		{Media: model.Media{ID: "m2", Title: "电影2"}, Score: 9.0, Reason: "内容推荐"},
		{Media: model.Media{ID: "m3", Title: "电影3"}, Score: 7.0, Reason: "内容推荐"},
	}

	merged := svc.mergeRecommendations(cfResults, cbResults, 0.6, 0.4)

	if len(merged) != 3 {
		t.Errorf("合并结果数量 = %d, 期望 3", len(merged))
	}

	// m2 应该在两个列表中都有，分数应该最高
	found := false
	for _, item := range merged {
		if item.Media.ID == "m2" {
			found = true
			if item.Score <= 0 {
				t.Errorf("m2 的合并分数应大于 0, 实际 = %.2f", item.Score)
			}
		}
	}
	if !found {
		t.Error("合并结果中未找到 m2")
	}
}
