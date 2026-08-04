import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'
import type {
  MediaPlayInfo,
} from '@/types'
import api from './client'

export type PlaybackMethod = 'direct' | 'remux' | 'smart_remux' | 'startup_stream' | 'transcode'

export interface PlaybackClientCapabilities {
  user_agent?: string
  supports_direct_play: boolean
  supports_remux: boolean
  supports_hevc: boolean
  force_transcode: boolean
  max_bitrate?: number
}

export interface PlaybackStartupStream {
  profile_id: string
  duration_ms: number
  playlist_url: string
  continuation_mode: 'event_bridge_v1' | string
  discontinuity_at_handoff: boolean
  encoding_plan_version: string
  encoding_plan_hash: string
}

export interface PlaybackSessionTemplate {
  create_url: string
  profile_id: string
  max_bitrate?: number
}

export interface PlaybackPlan {
  media_id: string
  method: PlaybackMethod
  url: string
  reason_code: string
  reason: string
  requires_transcode: boolean
  session_required: boolean
  session_template?: PlaybackSessionTemplate
  fallback_method?: PlaybackMethod
  fallback_url?: string
  client_capabilities: PlaybackClientCapabilities
  startup_stream?: PlaybackStartupStream
}

export type PlaybackSessionState =
  | 'creating'
  | 'starting'
  | 'ready'
  | 'active'
  | 'closing'
  | 'closed'
  | 'failed'
  | 'expired'

export type PlaybackGenerationState =
  | 'preparing'
  | 'running'
  | 'completed'
  | 'draining'
  | 'retired'
  | 'failed'

export interface PlaybackGenerationSnapshot {
  id: number
  session_id: string
  state: PlaybackGenerationState
  profile_id: string
  start_position_ms: number
  audio_track: number
  subtitle_track: number
  burn_subtitle: boolean
  max_bitrate: number
  reason?: string
  backend?: string
  process_pid?: number
  transcoded_ms: number
  speed?: string
  error_code?: string
  error_message?: string
  created_at: string
  updated_at: string
  started_at?: string
  first_segment_at?: string
  completed_at?: string
}

export interface PlaybackSessionSnapshot {
  id: string
  user_id: string
  media_id: string
  state: PlaybackSessionState
  created_at: string
  updated_at: string
  last_seen: string
  paused: boolean
  position_ms: number
  buffered_end_ms: number
  current_generation_id?: number
  pending_generation_id?: number
  close_reason?: string
  generation?: PlaybackGenerationSnapshot
}

export interface PlaybackSessionResult {
  session: PlaybackSessionSnapshot
  playlist_url?: string
  status_url: string
  heartbeat_interval_sec: number
  first_segment_ready: boolean
  startup_ms?: number
}

export interface CreatePlaybackSessionRequest {
  media_id: string
  profile_id?: string
  start_position_ms?: number
  audio_track?: number
  subtitle_track?: number
  burn_subtitle?: boolean
  max_bitrate?: number
}

export interface RestartPlaybackSessionRequest {
  profile_id?: string
  start_position_ms: number
  audio_track?: number
  subtitle_track?: number
  burn_subtitle?: boolean
  max_bitrate?: number
  reason?: string
}

export interface PlaybackSessionHeartbeatRequest {
  generation_id: number
  position_ms: number
  buffered_end_ms: number
  paused: boolean
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

function playbackSessionEndpoint(sessionId: string, suffix = ''): string {
  return `/playback/sessions/${encodeURIComponent(sessionId)}${suffix}`
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

  getPlaybackPlan: async (mediaId: string, capabilities?: {
    supportsDirect?: boolean
    supportsRemux?: boolean
    supportsHEVC?: boolean
    forceTranscode?: boolean
    maxBitrate?: number
  }) => {
    const response = await api.get<{ data: PlaybackPlan }>(`/stream/${mediaId}/plan`, {
      params: {
        supports_direct: capabilities?.supportsDirect ?? true,
        supports_remux: capabilities?.supportsRemux ?? true,
        supports_hevc: capabilities?.supportsHEVC,
        force_transcode: capabilities?.forceTranscode,
        max_bitrate: capabilities?.maxBitrate,
      },
    })
    playbackPlanCache.set(mediaId, response.data.data)
    return response
  },

  getCachedPlaybackPlan: (mediaId: string) => playbackPlanCache.get(mediaId),

  requiresPlaybackSession: (mediaId: string) => {
    const plan = playbackPlanCache.get(mediaId)
    return Boolean(plan?.method === 'transcode' && plan.session_required && plan.session_template)
  },

  createPlaybackSession: (request: CreatePlaybackSessionRequest) =>
    api.post<{ data: PlaybackSessionResult }>('/playback/sessions', request),

  getPlaybackSessionStatus: (sessionId: string) =>
    api.get<{ data: PlaybackSessionResult }>(playbackSessionEndpoint(sessionId, '/status')),

  restartPlaybackSession: (sessionId: string, request: RestartPlaybackSessionRequest) =>
    api.post<{ data: PlaybackSessionResult }>(playbackSessionEndpoint(sessionId, '/restart'), request),

  heartbeatPlaybackSession: (sessionId: string, request: PlaybackSessionHeartbeatRequest) =>
    api.post<{ data: PlaybackSessionResult }>(playbackSessionEndpoint(sessionId, '/heartbeat'), request),

  closePlaybackSession: (sessionId: string, reason = 'client_closed') =>
    api.delete(playbackSessionEndpoint(sessionId), { params: { reason } }),

  closePlaybackSessionKeepalive: (sessionId: string, reason = 'component_unmounted') => {
    const token = useAuthStore.getState().token
    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    return fetch(`/api${playbackSessionEndpoint(sessionId)}?reason=${encodeURIComponent(reason)}`, {
      method: 'DELETE',
      headers,
      keepalive: true,
      credentials: 'same-origin',
    })
  },

  getMasterUrl: (mediaId: string) => {
    const plan = playbackPlanCache.get(mediaId)
    const plannedHls = plan?.method === 'transcode' || plan?.method === 'startup_stream'
    return withToken(plannedHls ? plan.url : `/api/stream/${mediaId}/master.m3u8`)
  },

  getPlaybackFallbackUrl: (mediaId: string) => {
    const fallback = playbackPlanCache.get(mediaId)?.fallback_url
    return fallback ? withToken(fallback) : ''
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
