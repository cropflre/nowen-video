import api from './client'
import type {
  User,
  SystemInfo,
  TranscodeJob,
  TMDbConfigStatus,
  ListResponse,
  SystemMetrics,
  ScheduledTask,
  CreateScheduledTaskRequest,
  UserPermission,
  UpdatePermissionRequest,
  ContentRating,
  TMDbSearchResult,
  TMDbImageInfo,
  BangumiSubject,
  BangumiConfigStatus,
  DoubanConfigStatus,
  DoubanValidateResult,
  SystemSettings,
  DoubanSearchResult,
  TheTVDBSearchResult,
  LoginLog,
  AuditLog,
  InviteCode,
} from '@/types'

// ==================== 管理 ====================
export const adminApi = {
  listUsers: () =>
    api.get<ListResponse<User>>('/admin/users'),

  createUser: (data: { username: string; password: string; role?: 'admin' | 'user'; nickname?: string; email?: string }) =>
    api.post<{ data: User }>('/admin/users', data),

  updateUser: (id: string, data: { role?: 'admin' | 'user'; nickname?: string; email?: string; avatar?: string }) =>
    api.put<{ data: User }>(`/admin/users/${id}`, data),

  setUserDisabled: (id: string, disabled: boolean) =>
    api.post<{ message: string }>(`/admin/users/${id}/disabled`, { disabled }),

  deleteUser: (id: string) =>
    api.delete(`/admin/users/${id}`),

  resetUserPassword: (id: string, newPassword: string, forceChangeOnNextLogin: boolean = true) =>
    api.put<{ message: string }>(`/admin/users/${id}/password`, {
      new_password: newPassword,
      force_change_on_next_login: forceChangeOnNextLogin,
    }),

  // 登录日志 & 审计日志
  listLoginLogs: (params?: { page?: number; size?: number; only_failed?: boolean }) =>
    api.get<ListResponse<LoginLog>>('/admin/login-logs', { params }),

  listAuditLogs: (params?: { page?: number; size?: number; action?: string }) =>
    api.get<ListResponse<AuditLog>>('/admin/audit-logs', { params }),

  // 邀请码
  listInviteCodes: () =>
    api.get<ListResponse<InviteCode>>('/admin/invite-codes'),

  createInviteCode: (data: { code?: string; max_uses?: number; expires_in_hours?: number; note?: string }) =>
    api.post<{ data: InviteCode }>('/admin/invite-codes', data),

  deleteInviteCode: (id: string) =>
    api.delete(`/admin/invite-codes/${id}`),

  systemInfo: () =>
    api.get<{ data: SystemInfo }>('/admin/system'),

  transcodeStatus: () =>
    api.get<ListResponse<TranscodeJob>>('/admin/transcode/status'),

  cancelTranscode: (taskId: string) =>
    api.post(`/admin/transcode/${taskId}/cancel`),

  // TMDb 配置管理
  getTMDbConfig: () =>
    api.get<{ data: TMDbConfigStatus }>('/admin/settings/tmdb'),

  updateTMDbConfig: (apiKey: string) =>
    api.put<{ message: string; data: TMDbConfigStatus }>('/admin/settings/tmdb', { api_key: apiKey }),

  clearTMDbConfig: () =>
    api.delete<{ message: string; data: TMDbConfigStatus }>('/admin/settings/tmdb'),

  // 系统监控
  getMetrics: () =>
    api.get<{ data: SystemMetrics }>('/admin/metrics'),

  // 定时任务
  listTasks: () =>
    api.get<ListResponse<ScheduledTask>>('/admin/tasks'),

  createTask: (data: CreateScheduledTaskRequest) =>
    api.post<{ data: ScheduledTask }>('/admin/tasks', data),

  updateTask: (id: string, data: { name: string; schedule: string; enabled: boolean }) =>
    api.put(`/admin/tasks/${id}`, data),

  deleteTask: (id: string) =>
    api.delete(`/admin/tasks/${id}`),

  runTaskNow: (id: string) =>
    api.post(`/admin/tasks/${id}/run`),

  // 批量操作
  batchScan: (libraryIds: string[]) =>
    api.post('/admin/batch/scan', { library_ids: libraryIds }),

  batchScrape: (mediaIds: string[]) =>
    api.post('/admin/batch/scrape', { media_ids: mediaIds }),

  // 权限管理
  getUserPermission: (userId: string) =>
    api.get<{ data: UserPermission }>(`/admin/permissions/${userId}`),

  updateUserPermission: (userId: string, data: UpdatePermissionRequest) =>
    api.put(`/admin/permissions/${userId}`, data),

  // 内容分级
  getContentRating: (mediaId: string) =>
    api.get<{ data: ContentRating }>(`/admin/rating/${mediaId}`),

  setContentRating: (mediaId: string, level: string) =>
    api.put(`/admin/rating/${mediaId}`, { level }),

  // 手动元数据匹配
  searchMetadata: (q: string, type_: string = 'movie', year?: number) =>
    api.get<ListResponse<TMDbSearchResult>>('/admin/metadata/search', {
      params: { q, type: type_, year },
    }),

  matchMetadata: (mediaId: string, tmdbId: number) =>
    api.post(`/admin/media/${mediaId}/match`, { tmdb_id: tmdbId }),

  unmatchMetadata: (mediaId: string) =>
    api.post(`/admin/media/${mediaId}/unmatch`),

  deleteMedia: (mediaId: string) =>
    api.delete(`/admin/media/${mediaId}`),

  updateMediaMetadata: (mediaId: string, data: {
    title?: string
    orig_title?: string
    year?: number
    overview?: string
    rating?: number
    genres?: string
    country?: string
    language?: string
    tagline?: string
    studio?: string
  }) =>
    api.put<{ message: string; data: import('@/types').Media }>(`/admin/media/${mediaId}/metadata`, data),

  // 剧集合集管理
  matchSeriesMetadata: (seriesId: string, tmdbId: number) =>
    api.post(`/admin/series/${seriesId}/match`, { tmdb_id: tmdbId }),

  unmatchSeriesMetadata: (seriesId: string) =>
    api.post(`/admin/series/${seriesId}/unmatch`),

  scrapeSeriesMetadata: (seriesId: string) =>
    api.post(`/admin/series/${seriesId}/scrape`),

  deleteSeries: (seriesId: string) =>
    api.delete(`/admin/series/${seriesId}`),

  updateSeriesMetadata: (seriesId: string, data: {
    title?: string
    orig_title?: string
    year?: number
    overview?: string
    rating?: number
    genres?: string
    country?: string
    language?: string
    studio?: string
  }) =>
    api.put<{ message: string; data: import('@/types').Series }>(`/admin/series/${seriesId}/metadata`, data),

  // 系统全局设置
  getSystemSettings: () =>
    api.get<{ data: SystemSettings }>('/admin/settings/system'),

  updateSystemSettings: (data: Partial<SystemSettings>) =>
    api.put<{ data: SystemSettings }>('/admin/settings/system', data),

  // 图片管理
  searchTMDbImages: (tmdbId: number, type_: string = 'movie') =>
    api.get<{ data: { posters: TMDbImageInfo[]; backdrops: TMDbImageInfo[] } }>('/admin/images/tmdb', {
      params: { tmdb_id: tmdbId, type: type_ },
    }),

  uploadMediaImage: (mediaId: string, file: File, imageType: 'poster' | 'backdrop' = 'poster') => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`/admin/media/${mediaId}/image/upload?type=${imageType}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  uploadSeriesImage: (seriesId: string, file: File, imageType: 'poster' | 'backdrop' = 'poster') => {
    const formData = new FormData()
    formData.append('file', file)
    return api.post(`/admin/series/${seriesId}/image/upload?type=${imageType}`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
    })
  },

  setMediaImageByURL: (mediaId: string, url: string, imageType: 'poster' | 'backdrop' = 'poster') =>
    api.post(`/admin/media/${mediaId}/image/url`, { url, image_type: imageType }),

  setSeriesImageByURL: (seriesId: string, url: string, imageType: 'poster' | 'backdrop' = 'poster') =>
    api.post(`/admin/series/${seriesId}/image/url`, { url, image_type: imageType }),

  setMediaImageFromTMDb: (mediaId: string, tmdbPath: string, imageType: 'poster' | 'backdrop' = 'poster') =>
    api.post(`/admin/media/${mediaId}/image/tmdb`, { tmdb_path: tmdbPath, image_type: imageType }),

  setSeriesImageFromTMDb: (seriesId: string, tmdbPath: string, imageType: 'poster' | 'backdrop' = 'poster') =>
    api.post(`/admin/series/${seriesId}/image/tmdb`, { tmdb_path: tmdbPath, image_type: imageType }),

  // 豆瓣数据源
  searchDouban: (q: string, year?: number) =>
    api.get<ListResponse<DoubanSearchResult>>('/admin/metadata/douban/search', {
      params: { q, year },
    }),

  matchMediaDouban: (mediaId: string, doubanId: string) =>
    api.post(`/admin/media/${mediaId}/match/douban`, { douban_id: doubanId }),

  matchSeriesDouban: (seriesId: string, doubanId: string) =>
    api.post(`/admin/series/${seriesId}/match/douban`, { douban_id: doubanId }),

  // TheTVDB 数据源
  searchTheTVDB: (q: string, year?: number) =>
    api.get<ListResponse<TheTVDBSearchResult>>('/admin/metadata/thetvdb/search', {
      params: { q, year },
    }),

  matchSeriesTheTVDB: (seriesId: string, tvdbId: number) =>
    api.post(`/admin/series/${seriesId}/match/thetvdb`, { tvdb_id: tvdbId }),

  // Bangumi 数据源
  searchBangumi: (q: string, type_: number = 2, year?: number) =>
    api.get<ListResponse<BangumiSubject>>('/admin/metadata/bangumi/search', {
      params: { q, type: type_, year },
    }),

  getBangumiSubject: (subjectId: number) =>
    api.get<{ data: BangumiSubject }>(`/admin/metadata/bangumi/subject/${subjectId}`),

  matchMediaBangumi: (mediaId: string, bangumiId: number) =>
    api.post(`/admin/media/${mediaId}/match/bangumi`, { bangumi_id: bangumiId }),

  matchSeriesBangumi: (seriesId: string, bangumiId: number) =>
    api.post(`/admin/series/${seriesId}/match/bangumi`, { bangumi_id: bangumiId }),

  getBangumiConfig: () =>
    api.get<{ data: BangumiConfigStatus }>('/admin/settings/bangumi'),

  updateBangumiConfig: (accessToken: string) =>
    api.put<{ message: string; data: BangumiConfigStatus }>('/admin/settings/bangumi', { access_token: accessToken }),

  clearBangumiConfig: () =>
    api.delete<{ message: string; data: BangumiConfigStatus }>('/admin/settings/bangumi'),

  // 豆瓣 Cookie 配置管理
  getDoubanConfig: () =>
    api.get<{ data: DoubanConfigStatus }>('/admin/settings/douban'),

  updateDoubanConfig: (cookie: string) =>
    api.put<{ message: string; data: DoubanConfigStatus }>('/admin/settings/douban', { cookie }),

  clearDoubanConfig: () =>
    api.delete<{ message: string; data: DoubanConfigStatus }>('/admin/settings/douban'),

  validateDoubanConfig: () =>
    api.post<{ data: DoubanValidateResult }>('/admin/settings/douban/validate'),

  // 文件系统浏览
  browseFS: (path: string) =>
    api.get<{ data: { current: string; parent: string; items: { name: string; path: string; is_dir: boolean }[] } }>('/admin/fs/browse', {
      params: { path },
    }),

  // 一键清空数据（保留影视文件）
  clearAllData: (confirm: string) =>
    api.post<{ data: {
      status: string
      message: string
      total_cleared: number
      success_count: number
      error_count: number
      details: { table: string; cleared: number; status: string; message?: string }[]
    } }>('/admin/system/clear-data', { confirm }),

  // 剧集合并（多季自动合并为一个整体）
  mergeSeries: (primaryId: string, secondaryIds: string[]) =>
    api.post<{ message: string; data: {
      primary_series_id: string
      primary_title: string
      merged_count: number
      total_episodes: number
      total_seasons: number
      merged_series_ids: string[]
    } }>('/admin/series/merge', { primary_id: primaryId, secondary_ids: secondaryIds }),

  autoMergeSeries: () =>
    api.post<{ message: string; data: {
      groups_processed: number
      total_merged: number
      details: {
        primary_series_id: string
        primary_title: string
        merged_count: number
        total_episodes: number
        total_seasons: number
        merged_series_ids: string[]
      }[]
    } }>('/admin/series/auto-merge'),

  mergeCandidates: () =>
    api.get<{ data: {
      normalized_title: string
      count: number
      series: { id: string; title: string; season_count: number; episode_count: number; poster_path: string }[]
    }[]; total: number }>('/admin/series/merge-candidates'),

  // 重复媒体检测
  detectDuplicates: (libraryId?: string) =>
    libraryId
      ? api.get<{ data: import('@/types').DuplicateGroup[]; total: number }>(`/admin/libraries/${libraryId}/duplicates`)
      : api.get<{ data: import('@/types').DuplicateGroup[]; total: number }>('/admin/duplicates'),

  markDuplicates: (libraryId: string) =>
    api.post<{ message: string; marked: number }>(`/admin/libraries/${libraryId}/mark-duplicates`),

  // 手动预处理单个媒体
  submitPreprocess: (mediaId: string) =>
    api.post<{ message: string }>('/admin/preprocess/submit', { media_id: mediaId }),

  // 手动转码单个媒体（通过预处理提交）
  submitTranscode: (mediaId: string) =>
    api.post<{ message: string }>('/admin/preprocess/submit', { media_id: mediaId, force: true }),
}
