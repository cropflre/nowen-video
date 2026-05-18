import api from './client'

// =====================================
// 扫描后处理 · 虚拟归类与命名映射 API
// =====================================
//
// 后端：/api/admin/scan-classify/*
// 安全约束：所有接口副作用仅限 media_classifications 表，绝不修改任何磁盘文件。

// ---------- 类型 ----------

export type ClassificationStatus =
  | 'pending'
  | 'running'
  | 'processed'
  | 'partial'
  | 'failed'
  | 'skipped'

export type ClassificationCategory =
  | 'movie'
  | 'tvshow'
  | 'anime'
  | 'documentary'
  | 'variety'
  | 'music'
  | 'adult'
  | 'other'
  | ''

export interface MediaClassification {
  id: string
  media_id: string
  library_id: string

  // 阶段 1：识别
  parsed_title: string
  parsed_title_alt: string
  parsed_year: number
  parsed_tmdb_id: number
  parsed_imdb_id: string
  confidence: number
  ai_invoked: boolean
  ai_provider: string
  ai_model: string
  ai_raw_response: string

  // 阶段 2：归类
  category: ClassificationCategory
  region: string
  decade: string
  genre_tags: string
  language_tag: string
  quality_tier: string
  virtual_path: string

  // 阶段 3：命名映射
  suggested_name: string
  suggested_dir: string
  suggested_full_path: string
  naming_style: 'jellyfin' | 'plex' | ''

  // 状态
  status: ClassificationStatus
  error_msg: string
  processed_at: string | null

  created_at: string
  updated_at: string
}

export interface ClassificationListParams {
  library_id?: string
  status?: ClassificationStatus
  category?: ClassificationCategory
  region?: string
  decade?: string
  keyword?: string
  min_score?: number
  page?: number
  size?: number
}

export interface ClassificationListResp {
  items: MediaClassification[]
  total: number
  page: number
  size: number
}

export interface ClassificationStatsBucket {
  key: string
  count: number
}

export interface ClassificationStats {
  total: number
  by_status: ClassificationStatsBucket[]
  by_category: ClassificationStatsBucket[]
  by_region: ClassificationStatsBucket[]
  by_decade: ClassificationStatsBucket[]
}

export interface ReprocessRequest {
  library_id?: string
  media_ids?: string[]
  async?: boolean
}

export interface CorrectRequest {
  media_id: string
  title?: string
  year?: number
  tmdb_id?: number
  imdb_id?: string
  category?: string
  region?: string
}

// ---------- API ----------

export const scanClassifyApi = {
  list: (params: ClassificationListParams = {}) =>
    api.get<{ data: ClassificationListResp }>('/admin/scan-classify', { params }),

  get: (mediaId: string) =>
    api.get<{ data: MediaClassification }>(`/admin/scan-classify/${mediaId}`),

  stats: (libraryId?: string) =>
    api.get<{ data: ClassificationStats }>('/admin/scan-classify/stats', {
      params: libraryId ? { library_id: libraryId } : {},
    }),

  reprocess: (req: ReprocessRequest) =>
    api.post<{ data: { mode: string; async?: boolean; count?: number; processed?: number; requested?: number } }>(
      '/admin/scan-classify/reprocess',
      req,
    ),

  correct: (req: CorrectRequest) =>
    api.post<{ data: MediaClassification }>('/admin/scan-classify/correct', req),

  clear: (libraryId?: string) =>
    api.delete<{ data: { deleted: number } }>('/admin/scan-classify', {
      params: libraryId ? { library_id: libraryId } : {},
    }),
}

// 显示名映射
export const categoryDisplay: Record<string, string> = {
  movie: '电影',
  tvshow: '剧集',
  anime: '动画',
  documentary: '纪录片',
  variety: '综艺',
  music: '音乐',
  adult: '成人',
  other: '其他',
}

export const regionDisplay: Record<string, string> = {
  CN: '中国大陆',
  HK: '中国香港',
  TW: '中国台湾',
  JP: '日本',
  KR: '韩国',
  US: '美国',
  EU: '欧洲',
  IN: '印度',
  OTHER: '其他',
}

export const statusDisplay: Record<ClassificationStatus, string> = {
  pending: '待处理',
  running: '处理中',
  processed: '已完成',
  partial: '部分完成',
  failed: '失败',
  skipped: '跳过',
}

export const statusColor: Record<ClassificationStatus, string> = {
  pending: 'bg-gray-500/15 text-gray-300',
  running: 'bg-blue-500/15 text-blue-300',
  processed: 'bg-emerald-500/15 text-emerald-300',
  partial: 'bg-amber-500/15 text-amber-300',
  failed: 'bg-red-500/15 text-red-300',
  skipped: 'bg-zinc-500/15 text-zinc-300',
}
