import api from './client'

// ==================== 番号刮削管理 API ====================

/** 番号刮削数据源信息 */
export interface AdultScraperSource {
  id: string
  name: string
  type: 'go_native' | 'python_service'
  enabled: boolean
  url: string
  priority: number
  desc: string
}

/** 支持的番号格式 */
export interface SupportedFormat {
  type: string
  pattern: string
  example: string
}

/** 番号刮削配置 */
export interface AdultScraperConfig {
  enabled: boolean
  sources: AdultScraperSource[]
  min_request_interval: number
  max_request_interval: number
  supported_formats: SupportedFormat[]
}

/** 番号刮削结果 */
export interface AdultMetadata {
  code: string
  title: string
  original_title?: string
  plot?: string
  original_plot?: string
  cover: string
  thumb?: string
  actresses: string[]
  actor_photos?: Record<string, string>
  studio: string
  label: string
  series: string
  genres: string[]
  release_date: string
  duration: number
  rating: number
  trailer?: string
  extra_fanart?: string[]
  director?: string
  source: string
}

/** 番号识别结果（增强版） */
export interface ParseCodeResult {
  input: string
  code: string
  code_type: string
  is_adult: boolean
  letters?: string
  short_number?: string
  mosaic?: string // 有码 / 无码 / 国产 / 欧美
  cd_part?: string // CD1/PART2 等
  has_chn_sub?: boolean
}

/** Python 微服务健康状态 */
export interface PythonServiceHealth {
  configured: boolean
  status: 'online' | 'offline' | 'error' | 'not_configured'
  message: string
  url?: string
  http_code?: number
}

// ==================== P3~P5 扩展类型 ====================

/** 批量刮削任务 */
export interface AdultBatchTask {
  id: string
  status: 'running' | 'paused' | 'cancelled' | 'completed' | 'failed'
  total: number
  current: number
  success: number
  failed: number
  skipped: number
  started_at: string
  finished_at?: string
  library_id?: string
  dry_run: boolean
  concurrency: number
  aggregated: boolean
  results: AdultBatchItemResult[]
}

/** 批量刮削单条结果 */
export interface AdultBatchItemResult {
  media_id: string
  media_title: string
  code: string
  status: 'success' | 'failed' | 'skipped'
  err_msg?: string
  source?: string
  finished_at: string
}

/** 批量刮削进度事件数据 */
export interface AdultBatchProgressEvent {
  task_id: string
  total: number
  current: number
  success: number
  failed: number
  skipped: number
  media_id: string
  media_title: string
  code: string
  status: string
  err_msg?: string
  elapsed_ms: number
  estimate_left_ms: number
}

/** 镜像状态 */
export interface MirrorStatus {
  url: string
  healthy: boolean
  latency_ms: number
  last_check: string
  fail_count: number
  cooldown_to?: string
}

/** 调度器配置 */
export interface AdultSchedulerConfig {
  Enabled: boolean
  DailyHour: number
  DailyMinute: number
  Concurrency: number
  OnlyUnscraped: boolean
  Aggregated: boolean
}

/** 刮削报表 */
export interface AdultScrapeReport {
  period: string
  total_processed: number
  total_success: number
  total_failed: number
  overall_rate: number
  by_source: Array<{
    source: string
    total: number
    success: number
    failed: number
    success_rate: number
  }>
  by_prefix: Array<{
    prefix: string
    total: number
    success: number
    failed: number
    success_rate: number
  }>
  top_failures: string[]
  best_hours: number[]
  generated_at: string
}

export const adultScraperApi = {
  /** 获取番号刮削配置 */
  getConfig: () =>
    api.get<{ data: AdultScraperConfig }>('/admin/adult-scraper/config'),

  /** 更新番号刮削配置 */
  updateConfig: (data: {
    enabled?: boolean
    enable_javbus?: boolean
    javbus_url?: string
    enable_javdb?: boolean
    javdb_url?: string
    python_service_url?: string
    python_service_api_key?: string
    min_request_interval?: number
    max_request_interval?: number
  }) =>
    api.put<{ message: string }>('/admin/adult-scraper/config', data),

  /** P2：更新扩展配置（支持 P1/P2 所有字段） */
  updateConfigExtended: (data: Record<string, unknown>) =>
    api.put<{ message: string }>('/admin/adult-scraper/config-ext', data),

  /** 手动刮削指定番号 */
  scrapeByCode: (code: string) =>
    api.post<{ data: AdultMetadata }>('/admin/adult-scraper/scrape', { code }),

  /** P2：多数据源聚合刮削 */
  scrapeAggregated: (code: string) =>
    api.post<{ data: { merged: AdultMetadata; sources: Record<string, AdultMetadata>; count: number } }>(
      '/admin/adult-scraper/aggregate', { code },
    ),

  /** P2：测试所有数据源对同一番号的响应 */
  testAllSources: (code: string) =>
    api.get<{ data: { code: string; success: boolean; sources: Record<string, unknown>; count: number } }>(
      '/admin/adult-scraper/test-sources', { params: { code } },
    ),

  /** P2：映射表信息 */
  getMappings: () => api.get<{ data: Record<string, unknown> }>('/admin/adult-scraper/mappings'),

  /** P2：批量添加映射 */
  addMappings: (data: { type: 'actress' | 'studio' | 'series' | 'genre'; mappings: Record<string, string> }) =>
    api.post<{ message: string; count: number }>('/admin/adult-scraper/mappings', data),

  /** P2：测试规范化效果 */
  testNormalize: (data: { actresses?: string[]; studio?: string; series?: string; genres?: string[] }) =>
    api.post<{ data: Record<string, unknown> }>('/admin/adult-scraper/normalize-test', data),

  /** 测试番号识别 */
  parseCode: (input: string) =>
    api.get<{ data: ParseCodeResult }>('/admin/adult-scraper/parse', { params: { input } }),

  /** 检查 Python 微服务健康状态 */
  pythonServiceHealth: () =>
    api.get<{ data: PythonServiceHealth }>('/admin/adult-scraper/python-health'),

  // ==================== P3~P5：批量/镜像/缓存/调度/报表 ====================

  /** 启动批量刮削任务 */
  startBatch: (data: {
    library_id?: string
    media_ids?: string[]
    only_unscraped?: boolean
    dry_run?: boolean
    concurrency?: number
    aggregated?: boolean
  }) => api.post<{ data: { task_id: string } }>('/admin/adult-scraper/batch/start', data),

  /** 暂停批量任务 */
  pauseBatch: (id: string) => api.post<{ message: string }>(`/admin/adult-scraper/batch/${id}/pause`),

  /** 恢复批量任务 */
  resumeBatch: (id: string) => api.post<{ message: string }>(`/admin/adult-scraper/batch/${id}/resume`),

  /** 取消批量任务 */
  cancelBatch: (id: string) => api.post<{ message: string }>(`/admin/adult-scraper/batch/${id}/cancel`),

  /** 查询单任务 */
  getBatch: (id: string) => api.get<{ data: AdultBatchTask }>(`/admin/adult-scraper/batch/${id}`),

  /** 列出所有批量任务（活跃+历史） */
  listBatchTasks: () =>
    api.get<{ data: { active: AdultBatchTask[]; history: AdultBatchTask[] } }>('/admin/adult-scraper/batch'),

  /** 镜像列表 */
  listMirrors: () =>
    api.get<{ data: { sources: Record<string, { mirrors: MirrorStatus[]; preferred: string }>; last_health_at: string } }>(
      '/admin/adult-scraper/mirrors',
    ),

  /** 健康检查所有镜像 */
  healthCheckMirrors: () =>
    api.post<{ data: { total: number; healthy: number } }>('/admin/adult-scraper/mirrors/health-check'),

  /** 自定义镜像列表 */
  setMirrors: (source: string, urls: string[]) =>
    api.post<{ message: string; count: number }>(`/admin/adult-scraper/mirrors/${source}`, { urls }),

  /** 缓存统计 */
  getCacheStats: () =>
    api.get<{ data: { size: number; max_size: number; total_hit: number; ttl: string } }>(
      '/admin/adult-scraper/cache',
    ),

  /** 清空缓存 */
  clearCache: () => api.delete<{ message: string }>('/admin/adult-scraper/cache'),

  /** 失效单个番号缓存 */
  invalidateCache: (code: string) =>
    api.delete<{ message: string }>(`/admin/adult-scraper/cache/${encodeURIComponent(code)}`),

  /** 定时调度器配置 */
  getScheduler: () =>
    api.get<{ data: { config: AdultSchedulerConfig; last_run_at: string; last_task_id: string } }>(
      '/admin/adult-scraper/scheduler',
    ),

  /** 更新定时调度器配置 */
  updateScheduler: (config: AdultSchedulerConfig) =>
    api.put<{ message: string }>('/admin/adult-scraper/scheduler', config),

  /** 立即触发调度任务 */
  triggerScheduler: () =>
    api.post<{ data: { task_id: string } }>('/admin/adult-scraper/scheduler/run'),

  /** 刮削报表 */
  getReport: (days = 0) =>
    api.get<{ data: AdultScrapeReport }>('/admin/adult-scraper/report', { params: { days } }),

  /** 失败记录 */
  getFailedItems: (days = 7) =>
    api.get<{ data: AdultBatchItemResult[]; count: number }>(
      '/admin/adult-scraper/failed-items', { params: { days } },
    ),

  /** 一键重试失败 */
  retryFailed: (data: { days?: number; concurrency?: number; aggregated?: boolean }) =>
    api.post<{ data: { task_id: string; retry_count: number } }>(
      '/admin/adult-scraper/retry-failed', data,
    ),
}
