package service

import (
	"github.com/nowen-video/nowen-video/internal/repository"
	"go.uber.org/zap"
)

// PulseService Pulse 数据中心服务（全景仪表盘 + 媒体分析）
type PulseService struct {
	pulseRepo *repository.PulseRepo
	logger    *zap.SugaredLogger
	wsHub     *WSHub
}

func NewPulseService(pulseRepo *repository.PulseRepo, logger *zap.SugaredLogger) *PulseService {
	return &PulseService{
		pulseRepo: pulseRepo,
		logger:    logger,
	}
}

// SetWSHub 注入 WebSocket Hub
func (s *PulseService) SetWSHub(hub *WSHub) {
	s.wsHub = hub
}

// ==================== 全景仪表盘 ====================

// DashboardData 仪表盘完整数据
type DashboardData struct {
	Overview    *repository.DashboardOverview `json:"overview"`
	Trends      []repository.TrendPoint       `json:"trends"`
	TopContent  []repository.TopContentItem   `json:"top_content"`
	TopUsers    []repository.TopUserItem      `json:"top_users"`
	RecentPlays []repository.RecentPlayItem   `json:"recent_plays"`
	HourlyDist  []repository.HourlyDistItem   `json:"hourly_distribution"`
}

// GetDashboard 获取仪表盘完整数据（一次性加载）
func (s *PulseService) GetDashboard() (*DashboardData, error) {
	data := &DashboardData{}

	overview, err := s.pulseRepo.GetDashboardOverview()
	if err != nil {
		s.logger.Warnf("获取仪表盘概览失败: %v", err)
	} else {
		data.Overview = overview
	}

	trends, err := s.pulseRepo.GetPlayTrends(30)
	if err != nil {
		s.logger.Warnf("获取播放趋势失败: %v", err)
	} else {
		data.Trends = trends
	}

	topContent, err := s.pulseRepo.GetTopContent(30, 10)
	if err != nil {
		s.logger.Warnf("获取热门内容失败: %v", err)
	} else {
		data.TopContent = topContent
	}

	topUsers, err := s.pulseRepo.GetTopUsers(30, 10)
	if err != nil {
		s.logger.Warnf("获取用户排行失败: %v", err)
	} else {
		data.TopUsers = topUsers
	}

	recentPlays, err := s.pulseRepo.GetRecentPlays(20)
	if err != nil {
		s.logger.Warnf("获取最近播放失败: %v", err)
	} else {
		data.RecentPlays = recentPlays
	}

	hourlyDist, err := s.pulseRepo.GetHourlyDistribution(30)
	if err != nil {
		s.logger.Warnf("获取时段分布失败: %v", err)
	} else {
		data.HourlyDist = hourlyDist
	}

	return data, nil
}

// GetPlayTrends 获取播放趋势
func (s *PulseService) GetPlayTrends(days int) ([]repository.TrendPoint, error) {
	if days <= 0 {
		days = 30
	}
	if days > 365 {
		days = 365
	}
	return s.pulseRepo.GetPlayTrends(days)
}

// GetTopContent 获取热门内容排行
func (s *PulseService) GetTopContent(days, limit int) ([]repository.TopContentItem, error) {
	if days <= 0 {
		days = 30
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.pulseRepo.GetTopContent(days, limit)
}

// GetTopUsers 获取用户活跃排行
func (s *PulseService) GetTopUsers(days, limit int) ([]repository.TopUserItem, error) {
	if days <= 0 {
		days = 30
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	return s.pulseRepo.GetTopUsers(days, limit)
}

// GetRecentPlays 获取最近播放记录
func (s *PulseService) GetRecentPlays(limit int) ([]repository.RecentPlayItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	return s.pulseRepo.GetRecentPlays(limit)
}

// ==================== 媒体分析 ====================

// AnalyticsData 媒体分析完整数据
type AnalyticsData struct {
	GenreDist      []repository.GenreDistItem      `json:"genre_distribution"`
	ResolutionDist []repository.ResolutionDistItem `json:"resolution_distribution"`
	CodecDist      []repository.CodecDistItem      `json:"codec_distribution"`
	LibraryStats   []repository.LibraryStatItem    `json:"library_stats"`
	Growth         []repository.GrowthPoint        `json:"growth"`
}

// GetAnalytics 获取媒体分析完整数据
func (s *PulseService) GetAnalytics() (*AnalyticsData, error) {
	data := &AnalyticsData{}

	genreDist, err := s.pulseRepo.GetGenreDistribution()
	if err != nil {
		s.logger.Warnf("获取类型分布失败: %v", err)
	} else {
		data.GenreDist = genreDist
	}

	resDist, err := s.pulseRepo.GetResolutionDistribution()
	if err != nil {
		s.logger.Warnf("获取画质分布失败: %v", err)
	} else {
		data.ResolutionDist = resDist
	}

	codecDist, err := s.pulseRepo.GetCodecDistribution()
	if err != nil {
		s.logger.Warnf("获取编码分布失败: %v", err)
	} else {
		data.CodecDist = codecDist
	}

	libStats, err := s.pulseRepo.GetLibraryStats()
	if err != nil {
		s.logger.Warnf("获取媒体库统计失败: %v", err)
	} else {
		data.LibraryStats = libStats
	}

	growth, err := s.pulseRepo.GetMediaGrowth(12)
	if err != nil {
		s.logger.Warnf("获取增长趋势失败: %v", err)
	} else {
		data.Growth = growth
	}

	return data, nil
}

// GetHourlyDistribution 获取播放时段分布
func (s *PulseService) GetHourlyDistribution(days int) ([]repository.HourlyDistItem, error) {
	if days <= 0 {
		days = 30
	}
	return s.pulseRepo.GetHourlyDistribution(days)
}

// GetLibraryStats 获取媒体库统计
func (s *PulseService) GetLibraryStats() ([]repository.LibraryStatItem, error) {
	return s.pulseRepo.GetLibraryStats()
}

// GetMediaGrowth 获取媒体库增长趋势
func (s *PulseService) GetMediaGrowth(months int) ([]repository.GrowthPoint, error) {
	if months <= 0 {
		months = 12
	}
	return s.pulseRepo.GetMediaGrowth(months)
}
