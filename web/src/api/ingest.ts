import api from './client'

// =====================================
// LazyIngest · 一键入库 API
// =====================================
//
// 该模块对接后端 /api/admin/ingest/* 系列接口：
//   - 用户只给 source_path（也可显式指定 target_root），系统自动完成
//     扫描 → AI 分类 → 命名 → 落盘 → 建库 → 扫描全流程；
//   - 默认 hardlink 优先，跨卷自动 copy；不删除源文件；
//   - 进度通过 GET /jobs/:id 轮询，或订阅 WS 事件 ingest_progress。

export type IngestJobStatus =
  | 'pending'
  | 'scanning'
  | 'classifying'
  | 'planning'
  | 'executing'
  | 'completed'
  | 'failed'
  | 'canceled'

export type NamingStyle = 'jellyfin' | 'plex'

export interface IngestStats {
  scanned: number
  classified: number
  planned: number
  executed: number
  skipped: number
  failed: number
  unsorted: number
  /** 视觉占用字节数（含 hardlink） */
  bytes_logical?: number
  /** 真实占用字节数（hardlink 为 0） */
  bytes_physical?: number
}

export interface IngestJob {
  id: string
  source_path: string
  target_root: string
  keep_original: boolean
  naming_style: NamingStyle | string
  /** 落盘模式 hardlink / move */
  mode?: 'hardlink' | 'move' | string
  /** Move 模式下的 journal 文件路径 */
  journal_path?: string
  status: IngestJobStatus
  phase: string
  progress: number
  /** JSON 字符串：本次自动创建/复用的媒体库 ID 数组 */
  library_ids: string
  /** JSON 字符串：本次产出的 RenamePlan ID 数组 */
  plan_ids: string
  /** JSON 字符串：IngestStats */
  stats: string
  error_message: string
  created_by: string
  created_at: string
  updated_at: string
  started_at: string | null
  completed_at: string | null
}

export interface ExistingIngestRef {
  job_id: string
  target_root: string
  mode: string
  completed_at: string
}

/** 409 重复入库响应体 */
export interface AlreadyIngestedError {
  error: string
  code: 'already_ingested'
  existing_jobs: ExistingIngestRef[]
}

export interface RollbackResult {
  total: number
  restored_mv: number
  skipped_mv: number
  removed_dir: number
  kept_dir: number
  errors: string[]
  corrupted: number
}

export interface SubmitRequest {
  source_path: string
  /** 可选：默认 = source_path/_organized */
  target_root?: string
  /** 可选：默认 jellyfin */
  naming_style?: NamingStyle
  /** 可选：落盘模式 hardlink（默认，零风险）/ move（专家模式，原地移动） */
  mode?: 'hardlink' | 'move'
  /** 可选：强制重新入库（跳过同源检测） */
  force?: boolean
}

export const ingestApi = {
  submit: (req: SubmitRequest) =>
    api.post<{ data: IngestJob }>('/admin/ingest/submit', req),

  listJobs: (limit = 50) =>
    api.get<{ data: IngestJob[] }>('/admin/ingest/jobs', { params: { limit } }),

  getJob: (id: string) =>
    api.get<{ data: IngestJob }>(`/admin/ingest/jobs/${id}`),

  cancelJob: (id: string) =>
    api.post<{ message: string }>(`/admin/ingest/jobs/${id}/cancel`),

  /** 取该 Job 关联的所有文件级明细（来自 RenamePlanItem） */
  getJobItems: (id: string) =>
    api.get<{ data: IngestJobItem[]; total: number }>(`/admin/ingest/jobs/${id}/items`),

  /** 回滚 Move 模式任务：倒序还原文件位置 */
  rollback: (id: string) =>
    api.post<{ data: RollbackResult }>(`/admin/ingest/jobs/${id}/rollback`),
}

/**
 * IngestJobItem 单条文件明细
 *
 * 字段对齐后端 model.RenamePlanItem，仅取 UI 需要的部分。
 */
export interface IngestJobItem {
  id: string
  plan_id: string
  source_path: string
  source_name: string
  target_path: string
  target_name: string
  parsed_title: string
  parsed_year: number
  media_type: 'movie' | 'episode' | 'unknown' | string
  season_num: number
  episode_num: number
  confidence: number
  ai_invoked: boolean
  safety_ok: boolean
  safety_note: string
  status: 'pending' | 'skipped' | 'unsafe' | 'executed' | 'failed' | 'reverted' | string
  error_msg: string
  excluded: boolean
  created_at: string
}

/** 文件状态中文映射 */
export function itemStatusLabel(s: string): string {
  switch (s) {
    case 'executed':
      return '已落盘'
    case 'skipped':
      return '已跳过'
    case 'unsafe':
      return '安全检测拦截'
    case 'failed':
      return '失败'
    case 'pending':
      return '待执行'
    case 'reverted':
      return '已回滚'
    default:
      return s
  }
}

/** 给前端按状态分组渲染 */
export function groupItemsByStatus(items: IngestJobItem[]): Record<string, IngestJobItem[]> {
  const out: Record<string, IngestJobItem[]> = {
    failed: [],
    unsafe: [],
    skipped: [],
    executed: [],
    pending: [],
  }
  for (const it of items) {
    if (!out[it.status]) out[it.status] = []
    out[it.status].push(it)
  }
  return out
}

// 解析 stats JSON 字符串为对象（容错处理）
export function parseIngestStats(job: IngestJob | null | undefined): IngestStats {
  const empty: IngestStats = {
    scanned: 0,
    classified: 0,
    planned: 0,
    executed: 0,
    skipped: 0,
    failed: 0,
    unsorted: 0,
    bytes_logical: 0,
    bytes_physical: 0,
  }
  if (!job?.stats) return empty
  try {
    const parsed = JSON.parse(job.stats) as Partial<IngestStats>
    return { ...empty, ...parsed }
  } catch {
    return empty
  }
}

/** 人类可读的字节数格式 */
export function formatBytes(n: number | undefined | null): string {
  if (!n || n <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let v = n
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024
    i++
  }
  return `${v.toFixed(v >= 100 ? 0 : 1)} ${units[i]}`
}

// 解析 library_ids JSON 字符串
export function parseLibraryIds(job: IngestJob | null | undefined): string[] {
  if (!job?.library_ids) return []
  try {
    const arr = JSON.parse(job.library_ids)
    return Array.isArray(arr) ? (arr as string[]) : []
  } catch {
    return []
  }
}

// 任务是否处于"运行中"状态
export function isJobRunning(job: IngestJob | null | undefined): boolean {
  if (!job) return false
  return ['pending', 'scanning', 'classifying', 'planning', 'executing'].includes(job.status)
}

// 中文化任务状态（仅用于展示）
export function statusLabel(status: IngestJobStatus): string {
  switch (status) {
    case 'pending':
      return '排队中'
    case 'scanning':
      return '扫描源目录'
    case 'classifying':
      return 'AI 分类'
    case 'planning':
      return '生成命名计划'
    case 'executing':
      return '整理文件'
    case 'completed':
      return '已完成'
    case 'failed':
      return '失败'
    case 'canceled':
      return '已取消'
    default:
      return status
  }
}
