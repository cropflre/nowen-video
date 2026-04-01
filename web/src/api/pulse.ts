import api from './client'

// ==================== Pulse 数据中心 ====================

// 仪表盘概览
export interface PulseDashboardOverview {
  total_movies: number
  total_series: number
  total_episodes: number
  total_users: number
  total_storage_bytes: number
  total_play_minutes: number
  today_play_minutes: number
  active_users_7d: number
}

// 播放趋势数据点
export interface PulseTrendPoint {
  date: string
  total_minutes: number
  play_count: number
  unique_users: number
}

// 热门内容项
export interface PulseTopContentItem {
  media_id: string
  title: string
  poster_path: string
  media_type: string
  total_minutes: number
  play_count: number
  unique_users: number
}

// 用户活跃排行项
export interface PulseTopUserItem {
  user_id: string
  username: string
  avatar: string
  total_minutes: number
  play_count: number
  media_count: number
}

// 最近播放记录项
export interface PulseRecentPlayItem {
  user_id: string
  username: string
  media_id: string
  title: string
  poster_path: string
  media_type: string
  watch_minutes: number
  date: string
  created_at: string
}

// 时段分布项
export interface PulseHourlyDistItem {
  hour: number
  play_count: number
  total_minutes: number
}

// 仪表盘完整数据
export interface PulseDashboardData {
  overview: PulseDashboardOverview
  trends: PulseTrendPoint[]
  top_content: PulseTopContentItem[]
  top_users: PulseTopUserItem[]
  recent_plays: PulseRecentPlayItem[]
  hourly_distribution: PulseHourlyDistItem[]
}

// 类型分布项
export interface PulseGenreDistItem {
  genre: string
  count: number
  total_minutes: number
}

// 画质分布项
export interface PulseResolutionDistItem {
  resolution: string
  count: number
}

// 编码分布项
export interface PulseCodecDistItem {
  codec: string
  count: number
}

// 媒体库统计项
export interface PulseLibraryStatItem {
  library_id: string
  library_name: string
  library_type: string
  media_count: number
  series_count: number
  total_size_bytes: number
}

// 增长趋势数据点
export interface PulseGrowthPoint {
  date: string
  count: number
}

// 媒体分析完整数据
export interface PulseAnalyticsData {
  genre_distribution: PulseGenreDistItem[]
  resolution_distribution: PulseResolutionDistItem[]
  codec_distribution: PulseCodecDistItem[]
  library_stats: PulseLibraryStatItem[]
  growth: PulseGrowthPoint[]
}

export const pulseApi = {
  // 仪表盘完整数据（一次性加载）
  getDashboard: () =>
    api.get<{ data: PulseDashboardData }>('/admin/pulse/dashboard'),

  // 播放趋势
  getPlayTrends: (days = 30) =>
    api.get<{ data: PulseTrendPoint[] }>('/admin/pulse/dashboard/trends', { params: { days } }),

  // 热门内容排行
  getTopContent: (days = 30, limit = 10) =>
    api.get<{ data: PulseTopContentItem[] }>('/admin/pulse/dashboard/top-content', { params: { days, limit } }),

  // 用户活跃排行
  getTopUsers: (days = 30, limit = 10) =>
    api.get<{ data: PulseTopUserItem[] }>('/admin/pulse/dashboard/top-users', { params: { days, limit } }),

  // 最近播放记录
  getRecentPlays: (limit = 20) =>
    api.get<{ data: PulseRecentPlayItem[] }>('/admin/pulse/dashboard/recent', { params: { limit } }),

  // 媒体分析完整数据
  getAnalytics: () =>
    api.get<{ data: PulseAnalyticsData }>('/admin/pulse/analytics'),

  // 播放时段分布
  getHourlyDistribution: (days = 30) =>
    api.get<{ data: PulseHourlyDistItem[] }>('/admin/pulse/analytics/hourly', { params: { days } }),

  // 媒体库统计
  getLibraryStats: () =>
    api.get<{ data: PulseLibraryStatItem[] }>('/admin/pulse/analytics/libraries'),

  // 媒体库增长趋势
  getMediaGrowth: (months = 12) =>
    api.get<{ data: PulseGrowthPoint[] }>('/admin/pulse/analytics/growth', { params: { months } }),
}
