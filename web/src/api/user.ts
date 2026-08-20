import api from './client'
import type {
  User,
  WatchHistory,
  Favorite,
  PaginatedResponse,
  LoginLog,
} from '@/types'
import { toAbsolutePlaybackPosition } from '@/playback/sessionRuntime'
import { invalidatePageCachePrefix } from '@/hooks/usePageCache'

function publishFavoriteChanged(mediaId: string, favorited: boolean) {
  // 收藏列表使用 usePageCache。收藏状态变化后必须立即让所有分页缓存失效，
  // 否则从详情页返回/进入“我的收藏”时会继续命中 15 秒旧缓存，看起来像必须刷新页面才生效。
  invalidatePageCachePrefix('favorites:')

  // 同步通知当前窗口内可能仍挂载的收藏相关视图。现在主要用于 FavoritesPage，
  // 后续其它组件接入收藏数量/快捷收藏时也可以直接复用，不需要各自猜测缓存键。
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('nowen:favorites-updated', {
      detail: { mediaId, favorited },
    }))
  }
}

// ==================== 用户 ====================
export const userApi = {
  profile: () =>
    api.get<{ data: User }>('/users/me'),

  updateProfile: (data: { username?: string; nickname?: string; email?: string; avatar?: string }) =>
    api.put<{ data: User; token?: string; expires_at?: number }>('/users/me', data),

  loginLogs: () =>
    api.get<{ data: LoginLog[] }>('/users/me/login-logs'),

  updateProgress: (mediaId: string, position: number, duration: number) =>
    api.put(`/users/me/progress/${mediaId}`, {
      position: toAbsolutePlaybackPosition(mediaId, position),
      duration,
    }),

  favorites: (page = 1, size = 20) =>
    api.get<PaginatedResponse<Favorite>>('/users/me/favorites', { params: { page, size } }),

  addFavorite: async (mediaId: string) => {
    const response = await api.post(`/users/me/favorites/${mediaId}`)
    publishFavoriteChanged(mediaId, true)
    return response
  },

  removeFavorite: async (mediaId: string) => {
    const response = await api.delete(`/users/me/favorites/${mediaId}`)
    publishFavoriteChanged(mediaId, false)
    return response
  },

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
