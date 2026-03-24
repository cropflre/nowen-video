// ==================== 用户 ====================
export interface User {
  id: string
  username: string
  role: 'admin' | 'user'
  avatar: string
  created_at: string
}

// ==================== 认证 ====================
export interface LoginRequest {
  username: string
  password: string
}

export interface RegisterRequest {
  username: string
  password: string
}

export interface TokenResponse {
  token: string
  expires_at: number
  user: User
}

// ==================== 媒体库 ====================
export interface Library {
  id: string
  name: string
  path: string
  type: 'movie' | 'tvshow' | 'mixed' | 'other'
  last_scan: string | null
  created_at: string
  media_count?: number
  // 高级设置
  prefer_local_nfo: boolean
  enable_file_filter: boolean
  min_file_size: number
  metadata_lang: string
  allow_adult_content: boolean
  auto_download_sub: boolean
  // 新增6项功能
  enable_gpu_transcode: boolean
  gpu_fallback_cpu: boolean
  metadata_store_path: string
  play_cache_path: string
  enable_file_watch: boolean
  enable_direct_link: boolean
}

/** 创建媒体库 — 高级设置 */
export interface LibraryAdvancedSettings {
  prefer_local_nfo: boolean
  enable_file_filter: boolean
  min_file_size: number
  metadata_lang: string
  allow_adult_content: boolean
  auto_download_sub: boolean
  // 新增6项功能
  enable_gpu_transcode: boolean
  gpu_fallback_cpu: boolean
  metadata_store_path: string
  play_cache_path: string
  enable_file_watch: boolean
  enable_direct_link: boolean
}

export interface CreateLibraryRequest {
  name: string
  path: string
  type: 'movie' | 'tvshow' | 'mixed' | 'other'
  // 高级设置（可选）
  prefer_local_nfo?: boolean
  enable_file_filter?: boolean
  min_file_size?: number
  metadata_lang?: string
  allow_adult_content?: boolean
  auto_download_sub?: boolean
  // 新增6项功能
  enable_gpu_transcode?: boolean
  gpu_fallback_cpu?: boolean
  metadata_store_path?: string
  play_cache_path?: string
  enable_file_watch?: boolean
  enable_direct_link?: boolean
}

// ==================== 媒体 ====================
export interface Media {
  id: string
  library_id: string
  title: string
  orig_title: string
  year: number
  overview: string
  poster_path: string
  backdrop_path: string
  rating: number
  runtime: number
  genres: string
  file_path: string
  file_size: number
  media_type: 'movie' | 'episode'
  video_codec: string
  audio_codec: string
  resolution: string
  duration: number
  subtitle_paths: string
  series_id: string
  season_num: number
  episode_num: number
  episode_title: string
  created_at: string
  series?: Series
}

// ==================== 剧集合集 ====================
export interface Series {
  id: string
  library_id: string
  title: string
  orig_title: string
  year: number
  overview: string
  poster_path: string
  backdrop_path: string
  rating: number
  genres: string
  folder_path: string
  season_count: number
  episode_count: number
  created_at: string
  episodes?: Media[]
}

export interface SeasonInfo {
  season_num: number
  episode_count: number
  episodes: Media[]
}

// ==================== 观看记录 ====================
export interface WatchHistory {
  id: string
  user_id: string
  media_id: string
  position: number
  duration: number
  completed: boolean
  updated_at: string
  media: Media
}

// ==================== 收藏 ====================
export interface Favorite {
  id: string
  user_id: string
  media_id: string
  created_at: string
  media: Media
}

// ==================== 播放列表 ====================
export interface Playlist {
  id: string
  user_id: string
  name: string
  created_at: string
  updated_at: string
  items: PlaylistItem[]
}

export interface PlaylistItem {
  id: string
  playlist_id: string
  media_id: string
  sort_order: number
  created_at: string
  media: Media
}

export interface CreatePlaylistRequest {
  name: string
}

// ==================== 字幕 ====================
export interface SubtitleTrack {
  index: number
  codec: string
  language: string
  title: string
  default: boolean
  forced: boolean
}

export interface ExternalSubtitle {
  path: string
  filename: string
  format: string
  language: string
}

export interface SubtitleInfo {
  embedded: SubtitleTrack[]
  external: ExternalSubtitle[]
}

// ==================== TMDb 配置 ====================
export interface TMDbConfigStatus {
  configured: boolean
  masked_key: string
}

// ==================== 智能推荐 ====================
export interface RecommendedMedia {
  media: Media
  score: number
  reason: string
}

// ==================== 投屏 ====================
export interface CastDevice {
  id: string
  name: string
  type: 'dlna' | 'chromecast'
  location: string
  manufacturer: string
  model_name: string
  last_seen: number
}

export interface CastSession {
  id: string
  device_id: string
  media_id: string
  status: 'idle' | 'playing' | 'paused' | 'stopped'
  position: number
  duration: number
  volume: number
  device?: CastDevice
}

export interface CastRequest {
  device_id: string
  media_id: string
}

export interface CastControlRequest {
  action: 'play' | 'pause' | 'stop' | 'seek' | 'volume'
  value?: number
}

// ==================== 分页 ====================
export interface PaginatedResponse<T> {
  data: T[]
  total: number
  page: number
  size: number
}

// 聚合模式的最近添加响应
export interface AggregatedRecentResponse {
  media: Media[]
  series: Series[]
}

export interface ListResponse<T> {
  data: T[]
}

// ==================== 系统 ====================
export interface SystemInfo {
  version: string
  go_version: string
  os: string
  arch: string
  cpus: number
  goroutines: number
  memory: {
    alloc_mb: number
    total_alloc_mb: number
    sys_mb: number
  }
  hw_accel: string
}

export interface TranscodeJob {
  id: string
  media_id: string
  quality: string
  status: string
  progress: number
}

// ==================== 播放信息 ====================
export interface MediaPlayInfo {
  media_id: string
  direct_play_url: string
  hls_url: string
  can_direct_play: boolean
  file_ext: string
  video_codec: string
  audio_codec: string
  duration: number
}

// ==================== 视频书签 ====================
export interface Bookmark {
  id: string
  user_id: string
  media_id: string
  position: number
  title: string
  note: string
  created_at: string
  media?: Media
}

export interface CreateBookmarkRequest {
  media_id: string
  position: number
  title: string
  note?: string
}

// ==================== 评论 ====================
export interface Comment {
  id: string
  user_id: string
  media_id: string
  content: string
  rating: number
  created_at: string
  updated_at: string
  user?: User
}

export interface CreateCommentRequest {
  content: string
  rating?: number
}

export interface CommentListResponse {
  data: Comment[]
  total: number
  page: number
  size: number
  avg_rating: number
  rating_count: number
}

// ==================== 系统监控 ====================
export interface SystemMetrics {
  timestamp: number
  cpu: {
    usage_percent: number
    cores: number
    goroutines: number
  }
  memory: {
    total_mb: number
    used_mb: number
    free_mb: number
    used_percent: number
    go_alloc_mb: number
    go_sys_mb: number
    go_total_alloc_mb: number
  }
  disk: {
    total_gb: number
    used_gb: number
    free_gb: number
    used_percent: number
    cache_size_mb: number
  }
  transcode: {
    active_jobs: number
    queue_size: number
    hw_accel: string
  }
  connections: number
}

// ==================== 定时任务 ====================
export interface ScheduledTask {
  id: string
  name: string
  type: 'scan' | 'scrape' | 'cleanup'
  schedule: string
  target_id: string
  enabled: boolean
  last_run: string | null
  next_run: string | null
  status: 'idle' | 'running' | 'error'
  last_error: string
  created_at: string
}

export interface CreateScheduledTaskRequest {
  name: string
  type: 'scan' | 'scrape' | 'cleanup'
  schedule: string
  target_id?: string
}

// ==================== 权限管理 ====================
export interface UserPermission {
  id: string
  user_id: string
  allowed_libraries: string
  max_rating_level: string
  daily_time_limit: number
}

export interface UpdatePermissionRequest {
  allowed_libraries: string
  max_rating_level: string
  daily_time_limit: number
}

// ==================== 内容分级 ====================
export interface ContentRating {
  media_id: string
  level: '' | 'G' | 'PG' | 'PG-13' | 'R' | 'NC-17'
}

// ==================== 访问日志 ====================
export interface AccessLog {
  id: string
  user_id: string
  username: string
  action: string
  resource: string
  detail: string
  ip: string
  user_agent: string
  created_at: string
}

// ==================== TMDb搜索结果（手动匹配） ====================
export interface TMDbSearchResult {
  id: number
  title: string
  name: string
  original_title: string
  overview: string
  poster_path: string
  backdrop_path: string
  release_date: string
  first_air_date: string
  vote_average: number
  genre_ids: number[]
}
