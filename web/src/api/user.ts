import api from './client'
import type {
  User,
  WatchHistory,
  Favorite,
  PaginatedResponse,
} from '@/types'

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

  checkFavorite: (mediaId: string) =>
    api.get<{ data: boolean }>(`/users/me/favorites/${mediaId}/check`),

  getProgress: (mediaId: string) =>
    api.get<{ data: import('@/types').WatchHistory | null }>(`/users/me/progress/${mediaId}`),

  history: (page = 1, size = 20) =>
    api.get<PaginatedResponse<WatchHistory>>('/users/me/history', { params: { page, size } }),

  deleteHistory: (mediaId: string) =>
    api.delete(`/users/me/history/${mediaId}`),

  clearHistory: () =>
    api.delete('/users/me/history'),
}
