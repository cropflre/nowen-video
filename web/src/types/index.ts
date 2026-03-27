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
  // 媒体库级高级设置
  prefer_local_nfo: boolean
  enable_file_filter: boolean
  min_file_size: number
  metadata_lang: string
  allow_adult_content: boolean
  auto_download_sub: boolean
  enable_file_watch: boolean
}

/** 创建媒体库 — 高级设置（媒体库级别） */
export interface LibraryAdvancedSettings {
  prefer_local_nfo: boolean
  enable_file_filter: boolean
  min_file_size: number
  metadata_lang: string
  allow_adult_content: boolean
  auto_download_sub: boolean
  enable_file_watch: boolean
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
  enable_file_watch?: boolean
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
  // V2 扩展字段
  tmdb_id: number
  douban_id: string
  bangumi_id: number
  country: string
  language: string
  tagline: string
  studio: string
  trailer_url: string
  // 剧集字段
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
  // V2 扩展字段
  tmdb_id: number
  douban_id: string
  bangumi_id: number
  country: string
  language: string
  studio: string
  created_at: string
  episodes?: Media[]
}

// ==================== 人物 ====================
export interface Person {
  id: string
  name: string
  orig_name: string
  profile_url: string
  tmdb_id: number
}

export interface MediaPerson {
  id: string
  media_id: string
  series_id: string
  person_id: string
  role: 'director' | 'actor' | 'writer'
  character: string
  sort_order: number
  person: Person
}

// ==================== 播放统计 ====================
export interface UserStatsOverview {
  total_minutes: number
  total_hours: number
  daily_stats: { date: string; total_minutes: number; media_count: number }[]
  top_genres: { genres: string; total_minutes: number }[]
  most_watched: { media_id: string; title: string; poster_path: string; total_minutes: number }[]
}

// ==================== 数据备份 ====================
export interface BackupFile {
  name: string
  size: number
  modified: string
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
  bitmap: boolean  // 是否为图形字幕（PGS/VobSub等，不可提取为文本）
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

// ==================== 混合列表（Emby风格） ====================
export interface MixedItem {
  type: 'movie' | 'series'
  media?: Media
  series?: Series
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

// ==================== TMDb图片信息 ====================
export interface TMDbImageInfo {
  file_path: string
  width: number
  height: number
  aspect_ratio: number
  vote_average: number
  vote_count: number
  iso_639_1: string
}

// ==================== 系统全局设置 ====================
export interface SystemSettings {
  enable_gpu_transcode: boolean
  gpu_fallback_cpu: boolean
  metadata_store_path: string
  play_cache_path: string
  enable_direct_link: boolean
}

// ==================== Bangumi 数据源 ====================
export interface BangumiSubject {
  id: number
  type: number   // 1=书籍 2=动画 3=音乐 4=游戏 6=三次元
  name: string   // 原始名称（日文/英文）
  name_cn: string  // 中文名称
  summary: string
  air_date: string
  url: string
  eps: number
  platform: string
  images: {
    large: string
    common: string
    medium: string
    small: string
    grid: string
  } | null
  rating: {
    total: number
    score: number
    rank: number
  } | null
  tags: { name: string; count: number }[]
}

// ==================== AI 智能搜索 ====================
export interface SearchIntent {
  query: string
  media_type?: string
  genre?: string
  year_min?: number
  year_max?: number
  min_rating?: number
  sort_by?: string
  parsed: boolean
}

// ==================== AI 服务状态 ====================
export interface AIStatus {
  enabled: boolean
  provider: string
  model: string
  api_base: string
  api_configured: boolean
  timeout: number
  enable_smart_search: boolean
  enable_recommend_reason: boolean
  enable_metadata_enhance: boolean
  monthly_calls: number
  monthly_budget: number
  total_prompt_tokens: number
  total_completion_tokens: number
  total_tokens: number
  cache_entries: number
  cache_ttl_hours: number
  max_concurrent: number
  request_interval_ms: number
}

export interface AIErrorLog {
  time: string
  action: string
  error: string
  latency_ms: number
}

export interface AICacheStats {
  total_entries: number
  active_entries: number
  expired_entries: number
  ttl_hours: number
}

export interface AITestResult {
  success: boolean
  error?: string
  response?: string
  latency_ms: number
  provider?: string
  model?: string
  intent?: SearchIntent
  reason?: string
}

export interface BangumiConfigStatus {
  configured: boolean
  masked_token: string
}

// ==================== 刮削数据管理 ====================
export interface ScrapeTask {
  id: string
  url: string
  source: string
  title: string
  media_type: string
  status: 'pending' | 'scraping' | 'scraped' | 'failed' | 'translating' | 'completed'
  progress: number
  media_id: string
  series_id: string
  result_title: string
  result_orig_title: string
  result_year: number
  result_overview: string
  result_genres: string
  result_rating: number
  result_poster: string
  result_country: string
  result_language: string
  translate_status: 'none' | 'pending' | 'translating' | 'done' | 'failed'
  translate_lang: string
  translated_title: string
  translated_overview: string
  translated_genres: string
  translated_tagline: string
  quality_score: number
  error_message: string
  created_by: string
  created_at: string
  updated_at: string
}

export interface ScrapeHistory {
  id: string
  task_id: string
  action: string
  detail: string
  user_id: string
  created_at: string
}

export interface ScrapeStatistics {
  total: number
  pending: number
  scraping: number
  scraped: number
  failed: number
  translating: number
  completed: number
}

// ==================== 影视文件管理 ====================
export interface FileImportRequest {
  file_path: string
  title?: string
  media_type?: string
  library_id?: string
  year?: number
  overview?: string
}

export interface BatchImportResult {
  total: number
  success: number
  failed: number
  skipped: number
  errors: string[]
  media_ids: string[]
}

export interface RenamePreview {
  media_id: string
  old_title: string
  new_title: string
  old_file_path: string
  new_file_path: string
  reason: string
}

export interface RenameTemplate {
  pattern: string
  example: string
}

export interface FileManagerStats {
  total_files: number
  movie_count: number
  episode_count: number
  scraped_count: number
  unscraped_count: number
  total_size_bytes: number
  recent_imports: number
  recent_operations: number
}

export interface FileOperationLog {
  id: string
  action: string
  media_id: string
  detail: string
  old_value: string
  new_value: string
  user_id: string
  created_at: string
}

export interface ScannedFile {
  path: string
  name: string
  size: number
  ext: string
  modified: string
  imported: boolean
  title: string
}

// ==================== AI 助手 ====================
export interface ChatMsg {
  role: 'user' | 'assistant' | 'system'
  content: string
  timestamp: string
  actions?: SuggestedAction[]
  previews?: OperationPreview[]
}

export interface SuggestedAction {
  id: string
  label: string
  description: string
  action: string
  params: string
  dangerous: boolean
}

export interface OperationPreview {
  media_id: string
  title: string
  old_value: string
  new_value: string
  change_type: string
}

export interface ChatSession {
  id: string
  user_id: string
  messages: ChatMsg[]
  context: {
    selected_media_ids?: string[]
    library_id?: string
    last_intent?: Intent
  }
  created_at: string
  updated_at: string
}

export interface Intent {
  action: string
  sub_action: string
  targets: string
  params: Record<string, string>
  confidence: number
  reasoning: string
}

export interface ChatResponse {
  session_id: string
  message: ChatMsg
  intent?: Intent
}

export interface ExecuteResponse {
  success: boolean
  message: string
  results?: OperationPreview[]
  errors?: string[]
  op_id: string
}

export interface AssistantOperation {
  id: string
  session_id: string
  action: string
  previews: OperationPreview[]
  executed_at: string
  user_id: string
  undone: boolean
}

// ==================== 误分类检测 ====================
export interface MisclassifiedItem {
  media_id: string
  title: string
  file_path: string
  current_type: string
  suggested_type: string
  confidence: number
  reasons: string[]
  file_size: number
  dir_path: string
  sibling_count: number
}

export interface MisclassificationReport {
  total_movies: number
  suspected_episodes: number
  high_confidence: number
  medium_confidence: number
  low_confidence: number
  items: MisclassifiedItem[]
  common_patterns: string[]
  suggestions: string[]
}

export interface ReclassifyRequest {
  media_ids: string[]
  new_type: string
  auto_link_series: boolean
}

export interface ReclassifyResult {
  total: number
  success: number
  failed: number
  errors: string[]
  linked_series: number
}
