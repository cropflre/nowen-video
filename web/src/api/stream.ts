import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'
import type {
  MediaPlayInfo,
} from '@/types'
import api from './client'

export type PlaybackMethod = 'direct' | 'remux' | 'smart_remux' | 'transcode'

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
    next.is_preprocessed = false
  } else if (plan.method === 'remux' || plan.method === 'smart_remux') {
    next.can_direct_play = false
    next.can_remux = true
    next.remux_url = plan.url
    next.is_preprocessed = false
  } else {
    next.can_direct_play = false
    next.can_remux = false
    next.hls_url = plan.url
    next.is_preprocessed = Boolean(info.preprocessed_url && plan.url === info.preprocessed_url)
  }

  return next
}

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
    const plannedRemux = plan?.method === 'remux' || plan?.method === 'smart_remux'
    return withToken(plannedRemux ? plan.url : `/api/stream/${mediaId}/remux`)
  },

  reportPlayback: (mediaId: string, position: number) =>
    api.post(`/stream/${mediaId}/playback`, null, {
      params: { position: position.toFixed(2) },
    }),

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

  withTokenUrl: (url: string) => withToken(url),
}

export { withToken }
