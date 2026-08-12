import api from './client'
import type {
  CastDevice,
  CastSession,
  CastRequest,
  CastControlRequest,
  ListResponse,
} from '@/types'

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
import type {
  Bookmark,
  CreateBookmarkRequest,
  PaginatedResponse,
} from '@/types'

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
import type {
  Comment,
  CreateCommentRequest,
  CommentListResponse,
} from '@/types'

export const commentApi = {
  listByMedia: (mediaId: string, page = 1, size = 20) =>
    api.get<CommentListResponse>(`/media/${mediaId}/comments`, { params: { page, size } }),

  create: (mediaId: string, data: CreateCommentRequest) =>
    api.post<{ data: Comment }>(`/media/${mediaId}/comments`, data),

  delete: (id: string) =>
    api.delete(`/comments/${id}`),
}

// ==================== 播放统计 ====================
export const statsApi = {
  recordPlayback: (mediaId: string, watchMinutes: number) =>
    api.post('/stats/playback', { media_id: mediaId, watch_minutes: watchMinutes }),

  getMyStats: () =>
    api.get<{ data: import('@/types').UserStatsOverview }>('/stats/me'),
}
