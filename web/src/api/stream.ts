import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'
import type {
  MediaPlayInfo,
} from '@/types'
import api from './client'

export type PlaybackMethod = 'direct' | 'remux' | 'transcode'

export interface PlaybackClientCapabilities {
  user_agent?: string
  supports_direct_play: boolean
  supports_remux: boolean
  supports_hevc: boolean
  force_transcode: boolean
  max_bitrate?: number
}

export interface PlaybackPlan {
  media_id: string
  method: PlaybackMethod
  url: string
  reason_code: string
  reason: string
  requires_transcode: boolean
  fallback_method?: PlaybackMethod
  fallback_url?: string
  client_capabilities: PlaybackClientCapabilities
}

type PlannedMediaPlayInfo = MediaPlayInfo & {
  playback_plan?: PlaybackPlan
}

const playbackPlanCache = new Map<string, PlaybackPlan>()

function browserSupportsHEVC(): boolean {
  if (typeof document === 'undefined') return false
  try {
    const video = document.createElement('video')
    return (
      video.canPlayType('video/mp4; codecs="hev1.1.6.L93.B0"') !== '' ||
      video.canPlayType('video/mp4; codecs="hvc1.1.6.L93.B0"') !== '' ||
      video.canPlayType('video/mp4; codecs="hev1"') !== '' ||
      video.canPlayType('video/mp4; codecs="hvc1"') !== ''
    )
  } catch {
    return false
  }
}

function applyPlaybackPlan(info: MediaPlayInfo, plan: PlaybackPlan): MediaPlayInfo {
  const next = { ...info } as PlannedMediaPlayInfo
  next.playback_plan = plan

  if (plan.method === 'direct') {
    next.can_direct_play = true
    next.direct_play_url = plan.url
    next.can_remux = false
    // Lite 的优先级是原始直放优先于历史预处理缓存。
    next.is_preprocessed = false
  } else if (plan.method === 'remux') {
    next.can_direct_play = false
    next.can_remux = true
    next.remux_url = plan.url
    next.is_preprocessed = false
  } else {
    next.can_direct_play = false
    next.can_remux = false
    next.hls_url = plan.url
    if (info.preprocessed_url && plan.url === info.preprocessed_url) {
      next.is_preprocessed = true
    } else {
      next.is_preprocessed = false
    }
  }

  return next
}

// ==================== 流媒体 ====================

function withToken(url: string): string {
  const token = useAuthStore.getState().token
  if (!token) return url
  const sep = url.includes('?') ? '&' : '?'
  return `${url}${sep}token=${encodeURIComponent(token)}`
}

export const streamApi = {
  getPlayInfo: async (mediaId: string) => {
    const supportsHEVC = browserSupportsHEVC()
    const response = await api.get<{ data: PlannedMediaPlayInfo }>(`/stream/${mediaId}/info`, {
      params: {
        supports_direct: true,
        supports_remux: true,
        supports_hevc: supportsHEVC,
      },
    })

    // Current Lite servers embed the plan in /info, so playback initialization
    // needs only one network round trip.
    const embeddedPlan = response.data.data.playback_plan
    if (embeddedPlan) {
      playbackPlanCache.set(mediaId, embeddedPlan)
      response.data.data = applyPlaybackPlan(response.data.data, embeddedPlan) as PlannedMediaPlayInfo
      return response
    }

    try {
      await useServerProfileStore.getState().load()
      if (useServerProfileStore.getState().manifest?.profile !== 'lite') {
        playbackPlanCache.delete(mediaId)
        return response
      }

      // Transitional Lite deployments may expose /plan without embedding it in
      // /info. Keep this fallback until those deployments have upgraded.
      const planResponse = await api.get<{ data: PlaybackPlan }>(`/stream/${mediaId}/plan`, {
        params: {
          supports_direct: true,
          supports_remux: true,
          supports_hevc: supportsHEVC,
        },
      })
      const plan = planResponse.data.data
      playbackPlanCache.set(mediaId, plan)
      response.data.data = applyPlaybackPlan(response.data.data, plan) as PlannedMediaPlayInfo
    } catch {
      // Full and older servers keep the historical play-info behavior.
      playbackPlanCache.delete(mediaId)
    }

    return response
  },

  getPlaybackPlan: (mediaId: string, capabilities?: {
    supportsDirect?: boolean
    supportsRemux?: boolean
    supportsHEVC?: boolean
    forceTranscode?: boolean
    maxBitrate?: number
  }) =>
    api.get<{ data: PlaybackPlan }>(`/stream/${mediaId}/plan`, {
      params: {
        supports_direct: capabilities?.supportsDirect ?? true,
        supports_remux: capabilities?.supportsRemux ?? true,
        supports_hevc: capabilities?.supportsHEVC,
        force_transcode: capabilities?.forceTranscode,
        max_bitrate: capabilities?.maxBitrate,
      },
    }),

  getMasterUrl: (mediaId: string) => {
    const plan = playbackPlanCache.get(mediaId)
    return withToken(plan?.method === 'transcode' ? plan.url : `/api/stream/${mediaId}/master.m3u8`)
  },

  getDirectUrl: (mediaId: string) => {
    const plan = playbackPlanCache.get(mediaId)
    return withToken(plan?.method === 'direct' ? plan.url : `/api/stream/${mediaId}/direct`)
  },

  getRemuxUrl: (mediaId: string) => {
    const plan = playbackPlanCache.get(mediaId)
    return withToken(plan?.method === 'remux' ? plan.url : `/api/stream/${mediaId}/remux`)
  },

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
