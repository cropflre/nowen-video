import api from './client'
import { useAuthStore } from '@/stores/auth'
import type {
  LoginRequest,
  RegisterRequest,
  TokenResponse,
  User,
  Library,
  CreateLibraryRequest,
  Media,
  Series,
  SeasonInfo,
  WatchHistory,
  Favorite,
  Playlist,
  CreatePlaylistRequest,
  SubtitleInfo,
  PaginatedResponse,
  ListResponse,
  AggregatedRecentResponse,
  SystemInfo,
  TranscodeJob,
  MediaPlayInfo,
  TMDbConfigStatus,
  RecommendedMedia,
  CastDevice,
  CastSession,
  CastRequest,
  CastControlRequest,
  Bookmark,
  CreateBookmarkRequest,
  Comment,
  CreateCommentRequest,
  CommentListResponse,
  SystemMetrics,
  ScheduledTask,
  CreateScheduledTaskRequest,
  UserPermission,
  UpdatePermissionRequest,
  ContentRating,
  AccessLog,
  TMDbSearchResult,
} from '@/types'

// ==================== 认证 ====================
export const authApi = {
  login: (data: LoginRequest) =>
    api.post<TokenResponse>('/auth/login', data),

  register: (data: RegisterRequest) =>
    api.post<TokenResponse>('/auth/register', data),

  refreshToken: () =>
    api.post<TokenResponse>('/auth/refresh'),
}

// ==================== 媒体库 ====================
export const libraryApi = {
  list: () =>
    api.get<ListResponse<Library>>('/libraries'),

  create: (data: CreateLibraryRequest) =>
    api.post<{ data: Library }>('/libraries', data),

  update: (id: string, data: Partial<CreateLibraryRequest>) =>
    api.put<{ data: Library }>(`/libraries/${id}`, data),

  scan: (id: string) =>
    api.post(`/libraries/${id}/scan`),

  reindex: (id: string) =>
    api.post(`/libraries/${id}/reindex`),

  delete: (id: string) =>
    api.delete(`/libraries/${id}`),
}

// ==================== 媒体 ====================
export const mediaApi = {
  list: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<Media>>('/media', { params }),

  listAggregated: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<Media>>('/media/aggregated', { params }),

  detail: (id: string) =>
    api.get<{ data: Media }>(`/media/${id}`),

  recent: (limit = 20) =>
    api.get<ListResponse<Media>>('/media/recent', { params: { limit } }),

  recentAggregated: (limit = 20) =>
    api.get<AggregatedRecentResponse>('/media/recent/aggregated', { params: { limit } }),

  continueWatching: (limit = 10) =>
    api.get<ListResponse<WatchHistory>>('/media/continue', { params: { limit } }),

  search: (q: string, page = 1, size = 20) =>
    api.get<PaginatedResponse<Media>>('/search', { params: { q, page, size } }),

  scrape: (id: string) =>
    api.post(`/media/${id}/scrape`),
}

// ==================== 流媒体 ====================

function withToken(url: string): string {
  const token = useAuthStore.getState().token
  if (!token) return url
  const sep = url.includes('?') ? '&' : '?'
  return `${url}${sep}token=${encodeURIComponent(token)}`
}

export const streamApi = {
  getPlayInfo: (mediaId: string) =>
    api.get<{ data: MediaPlayInfo }>(`/stream/${mediaId}/info`),

  getMasterUrl: (mediaId: string) =>
    withToken(`/api/stream/${mediaId}/master.m3u8`),

  getDirectUrl: (mediaId: string) =>
    withToken(`/api/stream/${mediaId}/direct`),

  getPosterUrl: (mediaId: string) =>
    withToken(`/api/media/${mediaId}/poster`),
}

// ==================== 字幕 ====================
export const subtitleApi = {
  getTracks: (mediaId: string) =>
    api.get<{ data: SubtitleInfo }>(`/subtitle/${mediaId}/tracks`),

  getExtractUrl: (mediaId: string, index: number) =>
    withToken(`/api/subtitle/${mediaId}/extract/${index}`),

  getExternalUrl: (path: string) =>
    withToken(`/api/subtitle/external?path=${encodeURIComponent(path)}`),
}

// ==================== 用户 ====================
export const userApi = {
  profile: () =>
    api.get<{ data: User }>('/users/me'),

  updateProgress: (mediaId: string, position: number, duration: number) =>
    api.put(`/users/me/progress/${mediaId}`, { position, duration }),

  favorites: (page = 1, size = 20) =>
    api.get<PaginatedResponse<Favorite>>('/users/me/favorites', { params: { page, size } }),

  addFavorite: (mediaId: string) =>
    api.post(`/users/me/favorites/${mediaId}`),

  removeFavorite: (mediaId: string) =>
    api.delete(`/users/me/favorites/${mediaId}`),

  history: (page = 1, size = 20) =>
    api.get<PaginatedResponse<WatchHistory>>('/users/me/history', { params: { page, size } }),

  deleteHistory: (mediaId: string) =>
    api.delete(`/users/me/history/${mediaId}`),

  clearHistory: () =>
    api.delete('/users/me/history'),
}

// ==================== 播放列表 ====================
export const playlistApi = {
  list: () =>
    api.get<ListResponse<Playlist>>('/playlists'),

  create: (data: CreatePlaylistRequest) =>
    api.post<{ data: Playlist }>('/playlists', data),

  detail: (id: string) =>
    api.get<{ data: Playlist }>(`/playlists/${id}`),

  delete: (id: string) =>
    api.delete(`/playlists/${id}`),

  addItem: (playlistId: string, mediaId: string) =>
    api.post(`/playlists/${playlistId}/items/${mediaId}`),

  removeItem: (playlistId: string, mediaId: string) =>
    api.delete(`/playlists/${playlistId}/items/${mediaId}`),
}

// ==================== 剧集合集 ====================
export const seriesApi = {
  list: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<Series>>('/series', { params }),

  detail: (id: string) =>
    api.get<{ data: Series }>(`/series/${id}`),

  seasons: (id: string) =>
    api.get<ListResponse<SeasonInfo>>(`/series/${id}/seasons`),

  seasonEpisodes: (id: string, season: number) =>
    api.get<ListResponse<Media>>(`/series/${id}/seasons/${season}`),

  nextEpisode: (id: string, season: number, episode: number) =>
    api.get<{ data: Media | null; message?: string }>(`/series/${id}/next`, {
      params: { season, episode },
    }),
}

// ==================== 管理 ====================
export const adminApi = {
  listUsers: () =>
    api.get<ListResponse<User>>('/admin/users'),

  deleteUser: (id: string) =>
    api.delete(`/admin/users/${id}`),

  systemInfo: () =>
    api.get<{ data: SystemInfo }>('/admin/system'),

  transcodeStatus: () =>
    api.get<ListResponse<TranscodeJob>>('/admin/transcode/status'),

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

  // 访问日志
  listAccessLogs: (params: { page?: number; size?: number; user_id?: string; action?: string }) =>
    api.get<PaginatedResponse<AccessLog>>('/admin/logs', { params }),

  // 手动元数据匹配
  searchMetadata: (q: string, type_: string = 'movie', year?: number) =>
    api.get<ListResponse<TMDbSearchResult>>('/admin/metadata/search', {
      params: { q, type: type_, year },
    }),

  matchMetadata: (mediaId: string, tmdbId: number) =>
    api.post(`/admin/media/${mediaId}/match`, { tmdb_id: tmdbId }),
}

// ==================== 智能推荐 ====================
export const recommendApi = {
  getRecommendations: (limit = 20) =>
    api.get<ListResponse<RecommendedMedia>>('/recommend', { params: { limit } }),
}

// ==================== 投屏 ====================
export const castApi = {
  listDevices: () =>
    api.get<ListResponse<CastDevice>>('/cast/devices'),

  refreshDevices: () =>
    api.post('/cast/devices/refresh'),

  startCast: (data: CastRequest) =>
    api.post<{ data: CastSession; message: string }>('/cast/start', data),

  listSessions: () =>
    api.get<ListResponse<CastSession>>('/cast/sessions'),

  getSession: (sessionId: string) =>
    api.get<{ data: CastSession }>(`/cast/sessions/${sessionId}`),

  control: (sessionId: string, data: CastControlRequest) =>
    api.post(`/cast/sessions/${sessionId}/control`, data),

  stopSession: (sessionId: string) =>
    api.delete(`/cast/sessions/${sessionId}`),
}

// ==================== 视频书签 ====================
export const bookmarkApi = {
  create: (data: CreateBookmarkRequest) =>
    api.post<{ data: Bookmark }>('/bookmarks', data),

  listByUser: (page = 1, size = 20) =>
    api.get<PaginatedResponse<Bookmark>>('/bookmarks', { params: { page, size } }),

  listByMedia: (mediaId: string) =>
    api.get<ListResponse<Bookmark>>(`/bookmarks/media/${mediaId}`),

  update: (id: string, title: string, note: string) =>
    api.put(`/bookmarks/${id}`, { title, note }),

  delete: (id: string) =>
    api.delete(`/bookmarks/${id}`),
}

// ==================== 评论 ====================
export const commentApi = {
  listByMedia: (mediaId: string, page = 1, size = 20) =>
    api.get<CommentListResponse>(`/media/${mediaId}/comments`, { params: { page, size } }),

  create: (mediaId: string, data: CreateCommentRequest) =>
    api.post<{ data: Comment }>(`/media/${mediaId}/comments`, data),

  delete: (id: string) =>
    api.delete(`/comments/${id}`),
}
