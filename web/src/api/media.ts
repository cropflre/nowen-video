import api from './client'
import type {
  Media,
  Series,
  WatchHistory,
  PaginatedResponse,
  ListResponse,
  AggregatedRecentResponse,
  MixedItem,
  MediaDetailEnhanced,
} from '@/types'

// ==================== 媒体 ====================
export const mediaApi = {
  list: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<Media>>('/media', { params }),

  listAggregated: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<Media>>('/media/aggregated', { params }),

  detail: (id: string) =>
    api.get<{ data: Media }>(`/media/${id}`),

  detailEnhanced: (id: string) =>
    api.get<{ data: MediaDetailEnhanced }>(`/media/${id}/enhanced`),

  getPersons: (id: string) =>
    api.get<ListResponse<import('@/types').MediaPerson>>(`/media/${id}/persons`),

  recent: (limit = 20) =>
    api.get<ListResponse<Media>>('/media/recent', { params: { limit } }),

  recentAggregated: (limit = 20) =>
    api.get<AggregatedRecentResponse>('/media/recent/aggregated', { params: { limit } }),

  recentMixed: (limit = 20) =>
    api.get<ListResponse<MixedItem>>('/media/recent/mixed', { params: { limit } }),

  listMixed: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<MixedItem>>('/media/mixed', { params }),

  continueWatching: (limit = 10) =>
    api.get<ListResponse<WatchHistory>>('/media/continue', { params: { limit } }),

  search: (q: string, page = 1, size = 20) =>
    api.get<PaginatedResponse<Media>>('/search', { params: { q, page, size } }),

  searchAdvanced: (params: {
    q?: string
    type?: string
    genre?: string
    year_min?: number
    year_max?: number
    min_rating?: number
    sort_by?: string
    sort_order?: string
    page?: number
    size?: number
  }) =>
    api.get<PaginatedResponse<Media>>('/search/advanced', { params }),

  searchMixed: (q: string, page = 1, size = 20) =>
    api.get<{
      media: Media[]
      series: Series[]
      media_total: number
      series_total: number
      page: number
      size: number
    }>('/search/mixed', { params: { q, page, size } }),

  scrape: (id: string) =>
    api.post(`/media/${id}/scrape`),
}

// ==================== 演员 ====================
export const personApi = {
  /** 获取演员详情 */
  getDetail: (personId: string) =>
    api.get<{ data: import('@/types').Person }>(`/persons/${personId}`),

  /** 获取某个演员参演的所有影视作品 */
  getMedia: (personId: string) =>
    api.get<{ media: Media[]; series: Series[] }>(`/persons/${personId}/media`),
}

// ==================== 电影系列合集 ====================
export const collectionApi = {
  /** 获取电影所属的合集 */
  getMediaCollection: (mediaId: string) =>
    api.get<{ data: import('@/types').CollectionWithMedia | null }>(`/media/${mediaId}/collection`),

  /** 获取合集详情 */
  getDetail: (collectionId: string) =>
    api.get<{ data: import('@/types').CollectionWithMedia }>(`/collections/${collectionId}`),

  /** 获取合集列表（支持排序和来源筛选） */
  list: (params?: { page?: number; size?: number; sort?: string; auto?: string }) =>
    api.get<{ data: import('@/types').MovieCollection[]; total: number }>('/collections', { params }),

  /** 搜索合集 */
  search: (keyword: string, limit = 10) =>
    api.get<{ data: import('@/types').MovieCollection[] }>('/collections/search', { params: { keyword, limit } }),

  /** 合并所有同名重复合集（管理员） */
  mergeDuplicates: () =>
    api.post<{ message: string; merged: number }>('/admin/collections/merge-duplicates'),

  /** 清理所有空壳合集（管理员） */
  cleanupEmpty: () =>
    api.post<{ message: string; cleaned: number }>('/admin/collections/cleanup-empty'),

  /** 重新匹配所有合集（管理员） */
  rematch: () =>
    api.post<{ message: string; created: number }>('/admin/collections/rematch'),

  /** 获取重复合集统计信息（管理员） */
  getDuplicateStats: () =>
    api.get<{ data: Record<string, number>; count: number }>('/admin/collections/duplicate-stats'),
}
