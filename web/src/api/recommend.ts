import api from './client'
import type {
  RecommendedMedia,
  ListResponse,
} from '@/types'

// ==================== 智能推荐 ====================
export const recommendApi = {
  getRecommendations: (limit = 20) =>
    api.get<ListResponse<RecommendedMedia>>('/recommend', { params: { limit } }),

  getSimilarMedia: (mediaId: string, limit = 12) =>
    api.get<ListResponse<RecommendedMedia>>(`/recommend/similar/${mediaId}`, { params: { limit } }),
}
