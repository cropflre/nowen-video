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

  // 上报当前播放位置，驱动后端 FFmpeg 节流（Throttling）
  // position: 当前播放时间（秒，绝对位置）
  reportPlayback: (mediaId: string, position: number) =>
    api.post(`/stream/${mediaId}/playback`, null, {
      params: { position: position.toFixed(2) },
    }),

  // 上报客户端 hls.js 的带宽评估（bit/s），驱动服务端 ABR 档位过滤建议
  // 服务端会在响应中返回推荐的 maxBitrate（留 20% 余量）和当前节流状态，
  // 前端可用于：
  //   1. 下次请求 master.m3u8 时带上 ?maxBitrate=xxx
  //   2. 在 Settings 菜单显示节流/转码状态
  reportBandwidth: (mediaId: string, bitrate: number) =>
    api.post<{
      ok: boolean
      reported_bitrate: number
      recommended_max: number
      throttle?: {
        media_id: string
        running: boolean
        active_qualities: string[] | null
        suspended_count: number
        playback_pos: number
        transcoded_pos: number
        ahead_seconds: number
      }
    }>(`/stream/${mediaId}/bandwidth`, null, { params: { bitrate: Math.round(bitrate) } }),

  // 独立查询节流/转码状态（低频，5s 一次即可）
  getThrottleStatus: (mediaId: string) =>
    api.get<{
      data: {
        media_id: string
        running: boolean
        active_qualities: string[] | null
        suspended_count: number
        playback_pos: number
        transcoded_pos: number
        ahead_seconds: number
      }
    }>(`/stream/${mediaId}/throttle`),

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
