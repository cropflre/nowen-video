import api from './client'
import type {
  Media,
  Series,
  WatchHistory,
  PaginatedResponse,
  ListResponse,
  AggregatedRecentResponse,
  MixedItem,
} from '@/types'

// ==================== 媒体 ====================
export const mediaApi = {
  list: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<Media>>('/media', { params }),

  listAggregated: (params: { page?: number; size?: number; library_id?: string }) =>
    api.get<PaginatedResponse<Media>>('/media/aggregated', { params }),

  detail: (id: string) =>
    api.get<{ data: Media }>(`/media/${id}`),

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
