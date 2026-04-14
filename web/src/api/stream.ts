import { useAuthStore } from '@/stores/auth'
import type {
  MediaPlayInfo,
} from '@/types'
import api from './client'

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

  getRemuxUrl: (mediaId: string) =>
    withToken(`/api/stream/${mediaId}/remux`),

  getPosterUrl: (mediaId: string) =>
    withToken(`/api/media/${mediaId}/poster`),

  getSeriesPosterUrl: (seriesId: string) =>
    withToken(`/api/series/${seriesId}/poster`),

  getSeriesBackdropUrl: (seriesId: string) =>
    withToken(`/api/series/${seriesId}/backdrop`),

  getCollectionPosterUrl: (collectionId: string) =>
    withToken(`/api/collections/${collectionId}/poster`),

  // 为任意 URL 添加认证 token
  withTokenUrl: (url: string) => withToken(url),
}

// 导出 withToken 供其他模块使用
export { withToken }
