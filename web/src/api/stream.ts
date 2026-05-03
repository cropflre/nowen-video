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

  // STRM 链路健康检查：返回连通性 + 关键响应头，用于播放器诊断面板
  checkSTRM: (mediaId: string) =>
    api.get<{
      data: {
        media_id: string
        url: string
        status_code: number
        ok: boolean
        content_type?: string
        content_length?: number
        accept_ranges?: string
        response_ms: number
        error?: string
        effective_url?: string
        headers?: Record<string, string>
      }
    }>(`/stream/${mediaId}/strm-check`),

  // version 可选：用于缓存破坏（cache-busting）。
  // 当元数据/海报被替换后，传入一个新的数字即可触发浏览器重新请求图片。
  getPosterUrl: (mediaId: string, version?: number) =>
    withToken(`/api/media/${mediaId}/poster${version ? `?v=${version}` : ''}`),

  getSeriesPosterUrl: (seriesId: string, version?: number) =>
    withToken(`/api/series/${seriesId}/poster${version ? `?v=${version}` : ''}`),

  getSeriesBackdropUrl: (seriesId: string, version?: number) =>
    withToken(`/api/series/${seriesId}/backdrop${version ? `?v=${version}` : ''}`),

  getCollectionPosterUrl: (collectionId: string, version?: number) =>
    withToken(`/api/collections/${collectionId}/poster${version ? `?v=${version}` : ''}`),

  getPersonProfileUrl: (personId: string, version?: number) =>
    withToken(`/api/persons/${personId}/profile${version ? `?v=${version}` : ''}`),

  // 为任意 URL 添加认证 token
  withTokenUrl: (url: string) => withToken(url),
}

// 导出 withToken 供其他模块使用
export { withToken }
