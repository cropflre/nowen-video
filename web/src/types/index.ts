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

// ==================== AI 字幕 ====================
export interface ASRTask {
  media_id: string
  status: 'none' | 'pending' | 'extracting' | 'transcribing' | 'converting' | 'translating' | 'completed' | 'failed'
  progress: number
  message: string
  language?: string
  engine?: string  // cloud / local
  vtt_path?: string
  error?: string
  created_at?: string
}

// ==================== AI 字幕翻译 ====================
export interface TranslatedSubtitle {
  language: string
  path: string
}

// ASR 服务状态
export interface ASRServiceStatus {
  enabled: boolean
  cloud_enabled: boolean
  local_enabled: boolean
  prefer_local: boolean
  model: string
  whisper_cpp_path: string
  whisper_model_path: string
  max_concurrent: number
  active_tasks: number
  total_tasks: number
  translate_enabled: boolean
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
  is_strm?: boolean // 是否为 STRM 远程流
  is_preprocessed?: boolean // 是否已预处理
  preprocessed_url?: string // 预处理后的 HLS 地址
  preprocess_status?: string // 预处理状态
  thumbnail_url?: string // 预处理封面缩略图
}

// ==================== 增强详情 ====================
export interface StreamDetail {
  index: number
  codec_type: 'video' | 'audio' | 'subtitle'
  codec_name: string
  codec_long_name: string
  profile?: string
  level?: number
  width?: number
  height?: number
  coded_width?: number
  coded_height?: number
  aspect_ratio?: string
  frame_rate?: string
  bit_rate?: string
  bit_depth?: number
  ref_frames?: number
  is_interlaced: boolean
  sample_rate?: string
  channels?: number
  channel_layout?: string
  language?: string
  title?: string
  is_default: boolean
  is_forced: boolean
  pix_fmt?: string
  color_space?: string
  color_transfer?: string
  color_primaries?: string
  color_range?: string
  bits_per_sample?: number
  duration?: string
  start_time?: string
  nb_frames?: string
  tags?: Record<string, string>
}

export interface FormatDetail {
  format_name: string
  format_long_name: string
  duration: string
  size: string
  bit_rate: string
  stream_count: number
  start_time?: string
  tags?: Record<string, string>
}

export interface FileDetail {
  file_name: string
  file_dir: string
  file_ext: string
  file_size: number
  mime_type: string
  permissions: string
  owner: string
  created_at: string
  modified_at: string
  md5: string
}

export interface LibraryInfo {
  id: string
  name: string
  type: string
  path: string
}

export interface PlaybackStatsInfo {
  total_play_count: number
  total_watch_minutes: number
  unique_viewers: number
  last_played_at: string
}

export interface TechSpecs {
  streams: StreamDetail[]
  format: FormatDetail | null
}

export interface MediaDetailEnhanced {
  media: Media
  tech_specs: TechSpecs | null
  library: LibraryInfo | null
  playback_stats: PlaybackStatsInfo | null
  file_info: FileDetail | null
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

// ==================== TMDb搜索结果（手动匹配） ====================export interface TMDbSearchResult {
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

// ==================== 豆瓣数据源 ====================
export interface DoubanSearchResult {
  id: string
  title: string
  year: number
  rating: number
  cover: string
  overview: string
  genres: string
}

// ==================== TheTVDB 数据源 ====================
export interface TheTVDBSearchResult {
  id: number
  name: string
  originalName: string
  image: string
  overview: string
  firstAired: string
  year: string
  status: string
  network: string
  genre: string[]
  country: string
  originalCountry: string
  originalLanguage: string
  primaryLanguage: string
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

export interface FolderNode {
  name: string
  path: string
  children: FolderNode[]
  file_count: number
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

// ==================== 字幕在线搜索 ====================
export interface SubtitleSearchResult {
  id: string
  title: string
  file_name: string
  language: string
  language_name: string
  format: string
  rating: number
  download_count: number
  source: string
  download_url: string
  match_type: 'hash' | 'title' | 'imdb'
}

export interface SubtitleDownloadResult {
  file_path: string
  file_name: string
  language: string
  format: string
}

// ==================== 智能通知系统 ====================
export interface NotificationConfig {
  enabled: boolean
  webhooks: WebhookNotifyConfig[]
  email: EmailConfig
  telegram: TelegramConfig
  events: NotificationEvents
}

export interface WebhookNotifyConfig {
  id: string
  name: string
  url: string
  secret?: string
  enabled: boolean
}

export interface EmailConfig {
  enabled: boolean
  smtp_host: string
  smtp_port: number
  username: string
  password: string
  from_addr: string
  from_name: string
  recipients: string[]
  use_tls: boolean
}

export interface TelegramConfig {
  enabled: boolean
  bot_token: string
  chat_id: string
}

export interface NotificationEvents {
  media_added: boolean
  scan_complete: boolean
  scrape_complete: boolean
  transcode_complete: boolean
  user_login: boolean
  system_error: boolean
}

// ==================== 批量元数据编辑 ====================
export interface BatchUpdateRequest {
  media_ids: string[]
  updates: Record<string, string>
}

export interface BatchUpdateResult {
  total: number
  success: number
  failed: number
  errors: string[]
}

// ==================== 媒体库导入/导出 ====================
export interface ImportSource {
  type: 'emby' | 'jellyfin' | 'nfo'
  server_url: string
  api_key: string
  user_id?: string
}

export interface ImportResult {
  total: number
  imported: number
  skipped: number
  failed: number
  errors: string[]
}

export interface ExportData {
  version: string
  export_at: string
  source: string
  libraries: { name: string; path: string; type: string }[]
  media: ExportMedia[]
  series: ExportSeries[]
}

export interface ExportMedia {
  title: string
  orig_title: string
  year: number
  overview: string
  rating: number
  genres: string
  file_path: string
  media_type: string
  tmdb_id?: number
  country?: string
  language?: string
  studio?: string
}

export interface ExportSeries {
  title: string
  orig_title: string
  year: number
  overview: string
  rating: number
  genres: string
  folder_path: string
  tmdb_id?: number
}

export interface EmbyLibrary {
  Id: string
  Name: string
  CollectionType: string
}

// ==================== V3: AI 场景识别与内容理解 ====================
export interface VideoChapter {
  id: string
  media_id: string
  title: string
  start_time: number
  end_time: number
  description: string
  scene_type: string
  confidence: number
  source: 'ai' | 'manual'
  thumbnail: string
  created_at: string
}

export interface VideoHighlight {
  id: string
  media_id: string
  title: string
  start_time: number
  end_time: number
  score: number
  tags: string
  thumbnail: string
  gif_path: string
  source: 'ai' | 'manual'
  created_at: string
}

export interface AIAnalysisTask {
  id: string
  media_id: string
  task_type: 'scene_detect' | 'highlight' | 'cover_select' | 'chapter_gen'
  status: 'pending' | 'running' | 'completed' | 'failed'
  progress: number
  result: string
  error: string
  started_at: string | null
  completed_at: string | null
  created_at: string
}

// ==================== V3: AI 封面优化 ====================
export interface CoverCandidate {
  id: string
  media_id: string
  frame_time: number
  image_path: string
  score: number
  brightness: number
  sharpness: number
  composition: number
  face_count: number
  is_selected: boolean
  created_at: string
}

// ==================== V2: 多用户配置文件 ====================
export interface UserProfile {
  id: string
  user_id: string
  name: string
  avatar: string
  type: 'standard' | 'kids' | 'restricted'
  pin: string
  is_default: boolean
  kids_settings?: KidsProfileSettings
  parental_control?: ParentalControlSettings
  created_at: string
  updated_at: string
}

export interface KidsProfileSettings {
  max_content_rating: string
  allowed_genres: string[]
  blocked_genres: string[]
  daily_time_limit_min: number
  bedtime_start: string
  bedtime_end: string
  require_approval: boolean
}

export interface ParentalControlSettings {
  enabled: boolean
  monitor_watch_history: boolean
  remote_management: boolean
  content_filter_level: 'strict' | 'moderate' | 'relaxed'
  blocked_media_ids: string[]
  notification_email: string
}

export interface ProfileWatchLog {
  id: string
  profile_id: string
  media_id: string
  media_title: string
  duration_min: number
  started_at: string
  ended_at: string
}

export interface ProfileDailyUsage {
  id: string
  profile_id: string
  date: string
  total_minutes: number
  media_count: number
}

// ==================== V2: 离线下载 ====================
export interface DownloadTask {
  id: string
  user_id: string
  media_id: string
  title: string
  status: 'pending' | 'downloading' | 'paused' | 'completed' | 'failed' | 'cancelled'
  progress: number
  file_size: number
  downloaded_size: number
  file_path: string
  output_path: string
  quality: string
  speed: number
  eta_seconds: number
  error: string
  expires_at: string | null
  created_at: string
  updated_at: string
}

export interface DownloadQueueInfo {
  total: number
  pending: number
  downloading: number
  completed: number
  failed: number
  total_size: number
  downloaded_size: number
}

// ==================== V2: 插件系统 ====================
export interface PluginInfo {
  id: string
  name: string
  version: string
  author: string
  description: string
  type: 'media_source' | 'theme' | 'player' | 'metadata' | 'notification'
  entry_point: string
  config_json: string
  enabled: boolean
  installed: boolean
  homepage: string
  license: string
  min_version: string
  created_at: string
  updated_at: string
}

export interface PluginManifest {
  id: string
  name: string
  version: string
  author: string
  description: string
  type: string
  entry_point: string
  homepage: string
  license: string
  min_version: string
  config: PluginConfigDef[]
  hooks: string[]
  permissions: string[]
}

export interface PluginConfigDef {
  key: string
  label: string
  type: 'string' | 'number' | 'boolean' | 'select'
  default: unknown
  required: boolean
  options?: string[]
  description: string
}

// ==================== V2: 音乐库 ====================
export interface MusicTrack {
  id: string
  library_id: string
  album_id: string
  title: string
  artist: string
  album_artist: string
  album: string
  genre: string
  year: number
  track_num: number
  disc_num: number
  duration: number
  file_path: string
  file_size: number
  format: string
  bitrate: number
  sample_rate: number
  channels: number
  cover_path: string
  lyrics_path: string
  play_count: number
  loved: boolean
  created_at: string
}

export interface MusicAlbum {
  id: string
  library_id: string
  title: string
  artist: string
  year: number
  genre: string
  cover_path: string
  track_count: number
  total_duration: number
  tracks?: MusicTrack[]
}

export interface MusicPlaylist {
  id: string
  user_id: string
  name: string
  cover_path: string
  is_public: boolean
  created_at: string
  updated_at: string
  items?: MusicPlaylistItem[]
}

export interface MusicPlaylistItem {
  id: string
  playlist_id: string
  track_id: string
  sort_order: number
  track?: MusicTrack
}

// ==================== V2: 图片库 ====================
export interface Photo {
  id: string
  library_id: string
  album_id: string
  file_name: string
  file_path: string
  file_size: number
  format: string
  width: number
  height: number
  thumb_path: string
  camera_make: string
  camera_model: string
  lens_model: string
  focal_length: string
  aperture: string
  shutter_speed: string
  iso: number
  taken_at: string | null
  latitude: number
  longitude: number
  tags: string
  face_ids: string
  scene_type: string
  color_tone: string
  is_favorite: boolean
  is_hidden: boolean
  rating: number
  created_at: string
}

export interface PhotoAlbum {
  id: string
  user_id: string
  name: string
  description: string
  cover_photo_id: string
  type: 'manual' | 'auto' | 'smart' | 'face'
  photo_count: number
  is_public: boolean
  created_at: string
  photos?: Photo[]
}

export interface FaceCluster {
  id: string
  name: string
  sample_path: string
  photo_count: number
}

// ==================== V2: 联邦架构 ====================
export interface ServerNode {
  id: string
  name: string
  url: string
  api_key: string
  status: 'online' | 'offline' | 'syncing' | 'error'
  role: 'primary' | 'peer' | 'mirror'
  version: string
  media_count: number
  storage_used: number
  storage_total: number
  cpu_usage: number
  mem_usage: number
  last_sync: string | null
  sync_status: string
  latency: number
  is_local: boolean
  created_at: string
}

export interface SharedMedia {
  id: string
  node_id: string
  remote_id: string
  title: string
  orig_title: string
  year: number
  overview: string
  poster_path: string
  rating: number
  genres: string
  media_type: string
  duration: number
  resolution: string
  stream_url: string
}

export interface FederationStats {
  total_nodes: number
  online_nodes: number
  total_media: number
  shared_media: number
  total_storage: number
  used_storage: number
}

export interface SyncTask {
  id: string
  node_id: string
  type: 'full' | 'incremental' | 'metadata_only'
  status: 'pending' | 'running' | 'completed' | 'failed'
  progress: number
  total: number
  synced: number
  failed: number
  error: string
  started_at: string | null
  completed_at: string | null
}

// ==================== V2: ABR 自适应码率 ====================
export interface ABRStatus {
  enabled: boolean
  gpu: GPUInfo
  active_streams: number
  max_streams: number
  profiles: string[]
}

export interface GPUInfo {
  available: boolean
  type: string
  name: string
  encoders: string[]
  max_streams: number
  memory_mb: number
  utilization: number
}

// ==================== P1: 批量移动媒体 ====================
export interface BatchMoveRequest {
  media_ids: string[]
  target_library_id: string
}

export interface BatchMoveResult {
  total: number
  success: number
  failed: number
  errors: string[]
}

// ==================== 视频预处理 ====================
export interface PreprocessTask {
  id: string
  media_id: string
  status: 'pending' | 'queued' | 'running' | 'paused' | 'completed' | 'failed' | 'cancelled'
  phase: string
  progress: number
  priority: number
  message: string
  error: string
  retries: number
  max_retry: number
  input_path: string
  output_dir: string
  media_title: string
  thumbnail_path: string
  keyframes_dir: string
  hls_master_path: string
  variants: string
  source_height: number
  source_width: number
  source_codec: string
  source_duration: number
  source_size: number
  started_at: string | null
  completed_at: string | null
  elapsed_sec: number
  speed_ratio: number
  created_at: string
  updated_at: string
}

export interface PreprocessStatistics {
  status_counts: Record<string, number>
  running_count: number
  max_workers: number
  active_workers: number
  queue_size: number
  hw_accel: string
  mode: string
  resource_limit: number
}

export interface SystemLoadInfo {
  cpu_count: number
  cpu_percent: number
  goroutines: number
  mem_alloc_mb: number
  mem_sys_mb: number
  active_workers: number
  max_workers: number
  cur_workers: number // 动态调整后的当前并发数
  queue_size: number
  resource_limit: number // CPU 资源使用率上限
  ffmpeg_threads: number // FFmpeg 线程数
  hw_accel: string // 硬件加速模式
  suggestions: string[] // 性能优化建议
}

// 性能配置
export interface PerformanceConfig {
  resource_limit: number
  max_transcode_jobs: number
  transcode_preset: string
  hw_accel: string
  detected_hw_accel: string // 实际检测到的硬件加速模式
  vaapi_device: string
  cpu_count: number
  ffmpeg_threads: number
  max_workers: number
}

// ==================== 字幕预处理 ====================
export interface SubtitlePreprocessTask {
  id: string
  media_id: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | 'skipped'
  phase: 'check' | 'extract' | 'generate' | 'translate' | 'done'
  progress: number
  message: string
  error: string
  media_title: string
  source_lang: string
  target_langs: string
  force_regenerate: boolean
  original_vtt_path: string
  translated_paths: string
  subtitle_source: string
  detected_language: string
  cue_count: number
  started_at: string | null
  completed_at: string | null
  elapsed_sec: number
  created_at: string
  updated_at: string
}

export interface SubtitlePreprocessStatistics {
  status_counts: Record<string, number>
  max_workers: number
  active_workers: number
  queue_size: number
  asr_enabled: boolean
}

