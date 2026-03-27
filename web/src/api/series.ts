import api from './client'
import type {
  Series,
  SeasonInfo,
  Media,
  PaginatedResponse,
  ListResponse,
} from '@/types'

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
