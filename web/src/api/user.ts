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

type FavoriteMutationState = {
  revision: number
  favorited: boolean
}

let favoriteMutationRevision = 0
const favoriteMutationState = new Map<string, FavoriteMutationState>()

function publishFavoriteChanged(mediaId: string, favorited: boolean) {
  // 记录本窗口最近一次已确认的收藏变更。checkFavorite 可能比收藏写请求更早发出、
  // 却更晚返回；通过 revision 可以阻止旧检查结果把刚刚更新的 UI 状态覆盖掉。
  favoriteMutationRevision += 1
  favoriteMutationState.set(mediaId, { revision: favoriteMutationRevision, favorited })

  // 收藏列表使用 usePageCache。收藏状态变化后必须立即让所有分页缓存失效，
  // 否则从详情页返回/进入“我的收藏”时会继续命中 15 秒旧缓存，看起来像必须刷新页面才生效。
  invalidatePageCachePrefix('favorites:')

  // 同步通知当前窗口内可能仍挂载的收藏相关视图。后续其它组件接入收藏数量/
  // 快捷收藏时也可以直接复用，不需要各自猜测缓存键。
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

  checkFavorite: async (mediaId: string) => {
    const startedRevision = favoriteMutationState.get(mediaId)?.revision ?? 0
    const response = await api.get<{ data: boolean }>(`/users/me/favorites/${mediaId}/check`)
    const latestMutation = favoriteMutationState.get(mediaId)

    // 如果这次检查请求发出后用户完成了一次收藏/取消收藏，那么服务端检查响应
    // 可能已经是旧快照。保持最近一次已确认写操作的状态，避免心形按钮“点亮后又熄灭”。
    if (latestMutation && latestMutation.revision !== startedRevision) {
      response.data.data = latestMutation.favorited
    }
    return response
  },

  getProgress: (mediaId: string) =>
    api.get<{ data: import('@/types').WatchHistory | null }>(`/users/me/progress/${mediaId}`),

  history: (page = 1, size = 20) =>
    api.get<PaginatedResponse<WatchHistory>>('/users/me/history', { params: { page, size } }),

  deleteHistory: (mediaId: string) =>
    api.delete(`/users/me/history/${mediaId}`),

  clearHistory: () =>
    api.delete('/users/me/history'),
}
