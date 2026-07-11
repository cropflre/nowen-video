// ==================== 用户 ====================
export interface User {
  id: string
  username: string
  role: 'admin' | 'user'
  avatar: string
  created_at: string
  // P0~P2 新增字段
  nickname?: string
  email?: string
  disabled?: boolean
  must_change_pwd?: boolean
  last_login_at?: string | null
  last_login_ip?: string
}

// 登录日志
export interface LoginLog {
  id: string
  user_id: string
  username: string
  ip: string
  user_agent: string
  success: boolean
  reason: string
  created_at: string
}

// 审计日志
export interface AuditLog {
  id: string
  operator_id: string
  operator: string
  action: string
  target_type: string
  target_id: string
  detail: string
  ip: string
  created_at: string
}

// 邀请码
export interface InviteCode {
  id: string
  code: string
  max_uses: number
  used_count: number
  expires_at?: string | null
  creator_id?: string
  note?: string
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
  invite_code?: string
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
  /** 主路径（第一个媒体文件夹，保留以兼容旧数据） */
  path: string
  /** 额外媒体文件夹列表（JSON 字符串），使用 paths() 工具函数获取完整数组更方便 */
  extra_paths?: string
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
  auto_scrape_metadata: boolean
  /**
   * AI 自动整理模式（随扫描/重建索引执行：识别 + 归类 + 命名 + 可选硬链接）
   *   - off          关闭：扫描后不做任何后处理
   *   - rule_only    仅规则：跑识别+归类+命名建议，禁用 AI
   *   - ai_assisted  AI 辅助（默认）：每条媒体调用 AI 识别
   */
  auto_organize_mode: AutoOrganizeMode
  /**
   * 硬链接整理输出目录（AI 自动整理后按虚拟路径创建硬链接到此目录）
   * 为空表示不创建硬链接（仅写 DB）；配置路径后自动按分类目录树创建硬链接
   */
  organize_output_dir: string
  enable_file_watch: boolean
}

/** AI 自动整理模式枚举 */
export type AutoOrganizeMode = 'off' | 'rule_only' | 'ai_assisted'

/** 从 Library 中解析出完整的媒体文件夹列表（主路径 + extra_paths） */
export function getLibraryPaths(lib: Pick<Library, 'path' | 'extra_paths'>): string[] {
  const result: string[] = []
  const seen = new Set<string>()
  const add = (p: string) => {
    const trimmed = (p || '').trim()
    if (!trimmed || seen.has(trimmed)) return
    seen.add(trimmed)
    result.push(trimmed)
  }
  add(lib.path)
  if (lib.extra_paths && lib.extra_paths.trim() !== '') {
    try {
      const extras = JSON.parse(lib.extra_paths) as string[]
      if (Array.isArray(extras)) extras.forEach(add)
    } catch {
      /* ignore invalid JSON */
    }
  }
  return result
}

/** 创建媒体库 — 高级设置（媒体库级别） */
export interface LibraryAdvancedSettings {
  prefer_local_nfo: boolean
  enable_file_filter: boolean
  min_file_size: number
  metadata_lang: string
  allow_adult_content: boolean
  auto_download_sub: boolean
  auto_scrape_metadata: boolean
  /** AI 自动整理模式（默认 ai_assisted） */
  auto_organize_mode: AutoOrganizeMode
  /** 硬链接输出目录（为空=不创建硬链接） */
  organize_output_dir: string
  enable_file_watch: boolean
}

export interface CreateLibraryRequest {
  name: string
  /** 单一路径（兼容模式），与 paths 二选一。优先使用 paths */
  path?: string
  /** 多路径模式：第一个路径将作为主路径 */
  paths?: string[]
  type: 'movie' | 'tvshow' | 'mixed' | 'other'
  // 高级设置（可选）
  prefer_local_nfo?: boolean
  enable_file_filter?: boolean
  min_file_size?: number
  metadata_lang?: string
  allow_adult_content?: boolean
  auto_download_sub?: boolean
  auto_scrape_metadata?: boolean
  /** AI 自动整理模式（默认由后端取 ai_assisted） */
  auto_organize_mode?: AutoOrganizeMode
  /** 硬链接输出目录（为空=不创建硬链接） */
  organize_output_dir?: string
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
  imdb_id?: string
  douban_id: string
  bangumi_id: number
  country: string
  language: string
  tagline: string
  studio: string
  trailer_url: string
  // 刮削状态追踪（新增）
  scrape_status?: 'pending' | 'scraped' | 'partial' | 'failed' | 'manual' | ''
  scrape_attempts?: number
  last_scrape_at?: string | null
  // NFO 完整建模字段（方案 B）
  num: string
  sort_title: string
  outline: string
  original_plot: string
  mpaa: string
  country_code: string
  maker: string
  publisher: string
  label: string
  tags: string
  website: string
  release_date: string
  premiered: string
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
  // C 方案：刮削状态
  scrape_status?: 'pending' | 'scraped' | 'partial' | 'failed' | 'manual'
  last_scrape_at?: string
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
  most_watched: { media_id: string; title: string; poster_path: string; total_minutes: number; media_type?: string }[]
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
  /** API 反向代理 Base URL，程序自动拼接 /3/... */
  api_proxy?: string
  /** 图片反向代理 Base URL，程序自动拼接 /t/p/... */
  image_proxy?: string
  /** HTTP/HTTPS/SOCKS5 网络出口代理 */
  network_proxy?: string
  network_proxy_configured?: boolean
  api_proxy_base_url?: string
  image_proxy_base_url?: string
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
    // 本进程占用（MB），等价于 sys_mb，供前端展示
    process_used_mb?: number
    // 本进程占主机物理内存的百分比
    process_used_percent?: number
    system_total_mb?: number
    system_used_mb?: number
    system_used_percent?: number
  }
  hw_accel: string
}

export interface TranscodeJob {
  id: string
  media_id: string
  media_title?: string
  quality: string
  status: string
  progress: number
  output_dir?: string
  error?: string
  priority?: number
  retries?: number
  max_retry?: number
  started_at?: string | null
  completed_at?: string | null
  created_at?: string
  updated_at?: string
}

// 转码任务整体统计（与预处理面板顶部卡片对齐）
export interface TranscodeStatistics {
  status_counts: Record<string, number>
  running_count: number
  active_workers: number
  max_workers: number
  hw_accel: string
  // 转码缓存目录占用（字节）。由后端聚合 cache/transcode 子树计算，30s TTL 缓存
  disk_usage_bytes?: number
  // 缓存目录的绝对路径，仅用于 UI 提示展示
  disk_usage_dir?: string
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
  sprite_url?: string // 进度条雪碧图地址（预处理完成后可用）
  sprite_vtt_url?: string // 进度条雪碧图 WebVTT 索引地址
  prefer_direct_play?: boolean // 系统设置：优先直接播放
  can_remux?: boolean // 是否支持 remux（容器不兼容但编码兼容）
  remux_url?: string // Remux 播放地址（零转码，仅转封装）
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

// ==================== 弹幕 ====================
export interface DanmakuComment {
  id: string
  media_id: string
  source: string
  source_id: string
  author: string
  content: string
  rating: number
  likes: number
  position: number
  color: string
  mode: string
  imported_at: string
  created_at: string
}

// ==================== 定时任务（已下线） ====================
// ScheduledTask / CreateScheduledTaskRequest 已移除，面向用户的定时任务调度不再提供。
// 扫描 / 刮削 / 缓存清理请使用各业务页面的手动入口。

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
  auto_preprocess_on_scan: boolean   // 扫描后自动触发预处理
  auto_transcode_on_play: boolean    // 播放时自动触发转码
  prefer_direct_play: boolean        // 优先直接播放（禁用自动转码）
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
export interface AIProviderProfileView {
  api_base: string
  model: string
  api_key_configured: boolean
  /** 是否参与 AIRouter failover 链路（默认 true） */
  enabled?: boolean
  /** V8：当前实际生效模型（同 provider 内 failover 推进后） */
  current_model?: string
  /** V8：模型级 failover 链（仅对当前激活 provider 生效，其他 provider 的链按需激活） */
  model_chain?: string[]
}

export interface AIStatus {
  enabled: boolean
  // 全自动托管模式（AutoPilot）：开启后由系统自动串联识别/归类/命名/刮削
  auto_pilot?: boolean
  // 是否禁用本地 AI（如 ollama）；开启后强制使用云端服务商
  block_local_ai?: boolean
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
  // 各 provider 的配置档案（key 字段已脱敏，仅返回 api_key_configured 标识）
  profiles?: Record<string, AIProviderProfileView>
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

// ==================== V7：AI 智能调度 / 故障转移 / 用量监控 ====================

export interface AIProviderPreset {
  provider: string
  label: string
  api_base: string
  default_model: string
  available_models: string[]
  description: string
  recommended?: boolean
}

export interface AIProviderTotalView {
  provider: string
  calls: number
  total_tokens: number
  cost_cny: number
  enabled: boolean
  configured: boolean
  /** V8：本 provider 当前生效模型 */
  current_model?: string
  /** V8：本 provider 模型链 */
  model_chain?: string[]
}

export interface AIRouterSnapshot {
  preferred_provider: string
  current_active: string
  /** V8：当前生效 provider 的模型 */
  current_model?: string
  /** V8：当前 provider 的偏好主模型 */
  preferred_model?: string
  /** V8：当前 provider 的模型链 */
  current_model_chain?: string[]
  last_switched_at: number
  failover_enabled: boolean
  chain: string[]
  monthly_token_budget: number
  monthly_token_used: number
  monthly_token_pct: number
  warning_threshold_pct: number
  consecutive_errors: number
  auto_recover_after_min: number
  provider_totals: AIProviderTotalView[]
}

export interface AIFailoverLog {
  id: string
  from_provider: string
  to_provider: string
  reason: string
  detail: string
  operator: string
  occurred_at: string
}

export interface AIUsageBucket {
  bucket: string
  calls: number
  prompt_tokens: number
  completion_tokens: number
  total_tokens: number
  cost_cny: number
}

export interface AIUsageReport {
  buckets: AIUsageBucket[]
  provider_totals: AIProviderTotalView[]
  range: string
  from: string
  to: string
}

export interface BangumiConfigStatus {
  configured: boolean
  masked_token: string
}

// ==================== 豆瓣 Cookie 配置 ====================
export interface DoubanConfigStatus {
  configured: boolean
  masked_cookie: string
}

export interface DoubanValidateResult {
  valid: boolean
  username?: string
  message: string
}

// 懒人版一键导入 token 创建响应
export interface DoubanImportTokenInfo {
  token: string
  expires_in: number
  expires_at: number
  import_url: string
  bookmarklet: string
  script: string
  douban_url: string
}

// 懒人版一键导入 token 状态轮询响应
export interface DoubanImportTokenStatus {
  status: 'pending' | 'success' | 'failed' | 'expired'
  message: string
  username?: string
  expires_at: number
  remaining_secs: number
}

// ==================== 潮汐调度 ====================
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
  partial_count?: number
  failed_count?: number
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
}

// 预处理产物磁盘占用 - 单条目
export interface PreprocessStorageItem {
  media_id: string
  media_title: string
  task_id?: string
  status?: string // pending/running/completed/failed/cancelled/orphan
  output_dir: string
  size: number // bytes
  is_orphan: boolean
}

// 预处理产物磁盘占用 - 总体响应
export interface PreprocessStorageUsage {
  root_dir: string
  total_size: number
  total_count: number
  task_size: number
  orphan_size: number
  orphan_count: number
  items: PreprocessStorageItem[]
  scanned_at: string
  scan_duration_ms: number
}

// 缓存目录分类（cache/ 一级子目录归类）
export interface CacheCategory {
  key: string         // preprocess / transcode / abr / thumbnails / ... / 'other:xxx'
  label: string       // 中文展示名（未登记目录会用原始目录名）
  path: string        // 子目录绝对路径
  size: number        // 字节数
  count: number       // 文件数
  cleanable: boolean  // 是否安全可清空
}

// 整个 cache/ 目录的分类占用统计
export interface CacheUsage {
  root_dir: string
  total_size: number
  total_count: number
  categories: CacheCategory[]
  scanned_at: string
  scan_duration_ms: number
  from_cache: boolean
}

// 自定义筛选预处理 - 筛选条件
export interface PreprocessFilter {
  library_ids?: string[]
  media_types?: string[] // movie / episode
  video_codecs?: string[] // h264 / hevc / av1 / vp9 ...
  audio_codecs?: string[] // aac / ac3 / dts / flac ...
  containers?: string[] // mp4 / mkv / avi / mov / ts / flv / webm
  resolutions?: string[] // 1080p / 720p / 4K / 480p
  keyword?: string
  min_size_bytes?: number
  max_size_bytes?: number
  min_year?: number
  max_year?: number
  min_duration?: number // 秒
  max_duration?: number // 秒
  // 排除策略 - 全部默认 true（在后端默认）；undefined = 后端走默认；显式 false = 不排除
  exclude_already_preprocessed?: boolean
  exclude_directly_playable?: boolean
  exclude_strm?: boolean
}

// 自定义筛选预处理 - 抽样条目
export interface PreprocessSample {
  media_id: string
  title: string
  year: number
  media_type: string
  video_codec: string
  audio_codec: string
  resolution: string
  duration: number
  file_size: number
  file_path: string
}

// 自定义筛选预处理 - 预览结果
export interface PreprocessFilterPreview {
  matched_count: number
  raw_count: number
  excluded_already: number
  excluded_playable: number
  excluded_strm: number
  total_size: number
  sample: PreprocessSample[]
  codec_histogram: Record<string, number>
  resolution_hist: Record<string, number>
}

// 候选影视条目（供用户手动勾选预处理）
export interface PreprocessCandidate {
  media_id: string
  title: string
  orig_title?: string
  year: number
  library_id: string
  media_type: string
  video_codec: string
  audio_codec: string
  resolution: string
  duration: number
  file_size: number
  file_path: string
  poster_path: string
  is_strm: boolean
  can_play_directly: boolean
  preprocess_status: string // none / pending / queued / running / paused / completed / failed / cancelled
  task_id?: string
  // 剧集专属（仅 media_type=episode 时有意义）
  season_num?: number
  episode_num?: number
  episode_title?: string
  // 刮削状态：pending / scraped / partial / failed / manual
  scrape_status?: string
}

// 候选影视列表分页响应
export interface PreprocessCandidateList {
  items: PreprocessCandidate[]
  total: number
  page: number
  size: number
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
  hw_accel: string // 硬件加速模式
  gpu_status?: GPUSafetyStatus // GPU 安全状态
}

// GPU 安全状态
export interface GPUSafetyStatus {
  degraded: boolean
  degrade_reason?: string
  degrade_since?: number
  degraded_task_count: number
  pending_gpu_tasks: number
  metrics: GPUMetrics
  thresholds: GPUThresholdConfig
}

// GPU 实时指标
export interface GPUMetrics {
  utilization: number
  memory_used: number
  memory_total: number
  memory_percent: number
  temperature: number
  power_draw: number
  power_limit: number
  encoder_util: number
  decoder_util: number
  fan_speed: number
  gpu_name: string
  driver_version: string
  available: boolean
  last_update_time: number
}

// GPU 安全阈值配置
export interface GPUThresholdConfig {
  utilization_threshold: number
  temperature_threshold: number
  recovery_threshold: number
  temperature_recovery: number
  monitor_interval: number
  enabled: boolean
}

// ==================== 字幕预处理 ====================
export interface SubtitlePreprocessTask {
  id: string
  media_id: string
  status: 'pending' | 'running' | 'completed' | 'failed' | 'cancelled' | 'skipped'
  phase: 'check' | 'extract' | 'generate' | 'clean' | 'translate' | 'done'
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
  /** 翻译失败的语言列表（逗号分隔） */
  failed_langs?: string
  /** 字幕清洗详细报告（JSON 字符串） */
  clean_report_json?: string
  started_at: string | null
  completed_at: string | null
  elapsed_sec: number
  created_at: string
  updated_at: string
}

/** 字幕清洗报告（后端 CleanReport） */
export interface SubtitleCleanReport {
  source_path: string
  backup_path?: string
  detected_encoding: string
  original_cue_count: number
  processed_cue_count: number
  removed_ads: number
  removed_sdh: number
  removed_empty: number
  merged_cues: number
  split_cues: number
  encoding_converted: boolean
  warnings?: string[]
}

export interface SubtitlePreprocessStatistics {
  status_counts: Record<string, number>
  max_workers: number
  active_workers: number
  queue_size: number
  asr_enabled: boolean
}

// P0: ASR 服务健康状态
export interface ASRHealthStatus {
  configured: boolean
  healthy: boolean
  engine: string
  message: string
}

// ==================== P0~P2: 字幕提取导出 ====================
export interface ExtractedSubtitleFile {
  track_index: number
  language: string
  title: string
  codec: string
  format: string
  path: string
  bitmap: boolean
  error?: string
}

export interface SubExtractProgressData {
  media_id: string
  media_title: string
  format: string
  total: number
  current: number
  progress: number
  message: string
  results?: ExtractedSubtitleFile[]
  error?: string
}

// ==================== 电影系列合集 ====================

/** 电影系列合集 */
export interface MovieCollection {
  id: string
  name: string
  overview: string
  poster_path: string
  tmdb_coll_id: number
  media_count: number
  /** 原始文件总数（每个版本副本各算一个）；老数据可能为 0，此时等同于 media_count */
  file_count?: number
  auto_matched: boolean
  year_range: string
  created_at: string
  updated_at: string
}

/** 合集中的电影项 */
export interface CollectionMediaItem {
  id: string
  title: string
  orig_title: string
  year: number
  premiered: string
  rating: number
  poster_path: string
  runtime: number
  overview: string
  genres: string
  is_current: boolean
  /** TMDB ID（用于前端折叠同片多版本） */
  tmdb_id?: number
  /** 同一部电影的不同版本共享此 ID */
  version_group?: string
  /** 版本标识："4K"/"Director's Cut" 等 */
  version_tag?: string
  /** 文件大小（字节） */
  file_size?: number
  /** 分辨率："1080p"/"2160p" 等 */
  resolution?: string
  /** 视频编码 */
  video_codec?: string
}

/** 合集及其包含的电影 */
export interface CollectionWithMedia {
  collection: MovieCollection
  media: CollectionMediaItem[]
}

// ==================== 系统日志 ====================
export interface SystemLog {
  id: string
  type: 'api' | 'playback' | 'system'
  level: 'debug' | 'info' | 'warn' | 'error'
  message: string
  detail: string
  method: string
  path: string
  status_code: number
  latency_ms: number
  client_ip: string
  user_agent: string
  user_id: string
  username: string
  media_id: string
  media_title: string
  source: string
  created_at: string
}

export interface SystemLogStats {
  total: number
  today_count: number
  today_errors: number
  type_counts: Record<string, number>
  level_counts: Record<string, number>
}

// ==================== 重复媒体检测 ====================
export interface DuplicateItem {
  id: string
  title: string
  file_path: string
  file_size: number
  resolution: string
  video_codec: string
  audio_codec: string
  duration: number
  library_id: string
  is_primary: boolean
}

export interface DuplicateGroup {
  group_key: string
  title: string
  year: number
  media_count: number
  media: DuplicateItem[]
  suggestion: string
}

