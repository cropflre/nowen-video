import { useAuthStore } from '@/stores/auth'
import type {
  MediaPlayInfo,
} from '@/types'
import api, { getResolvedApiBaseURL } from './client'
import {
  getMediaCapabilities,
  buildClientCapabilities,
  type BrowserMediaCapability,
} from '@/utils/media-capabilities'

export type PlaybackMethod = 'direct' | 'remux' | 'smart_remux' | 'startup_stream' | 'transcode'

export interface PlaybackClientCapabilities {
  user_agent?: string
  supports_direct_play: boolean
  supports_remux: boolean
  supports_hevc: boolean
  force_transcode: boolean
  max_bitrate?: number
  /** 扩展：浏览器精确能力参数 */
  hevc_hardware?: boolean
  audio_supports_ac3?: boolean
  audio_supports_eac3?: boolean
  audio_supports_flac?: boolean
  audio_supports_opus?: boolean
  container_supports_mp4?: boolean
  container_supports_webm?: boolean
  mse_h264?: boolean
  mse_hevc?: boolean
  platform?: string
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
  source_technical?: {
    probe_version?: string
    video_codec?: string
    audio_codecs?: string[]
    width?: number
    height?: number
    frame_rate?: number
    pixel_format?: string
    bit_depth?: number
    hdr?: boolean
  }
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
  ahead_ms: number
  suspended: boolean
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

/** 获取当前浏览器精确能力（延迟初始化 + 缓存） */
function getCaps(): BrowserMediaCapability {
  return getMediaCapabilities()
}

/** 构建发送给后端的完整能力参数 */
function planParams(caps: BrowserMediaCapability, overrides?: {
  supportsDirect?: boolean
  supportsRemux?: boolean
  forceTranscode?: boolean
  maxBitrate?: number
}) {
  const base = buildClientCapabilities(caps)
  return {
    supports_direct: overrides?.supportsDirect ?? true,
    supports_remux: overrides?.supportsRemux ?? true,
    supports_hevc: base.supports_hevc,
    force_transcode: overrides?.forceTranscode ?? false,
    max_bitrate: overrides?.maxBitrate,
    hevc_hardware: base.hevc_hardware,
    audio_supports_ac3: base.audio_supports_ac3,
    audio_supports_eac3: base.audio_supports_eac3,
    audio_supports_flac: base.audio_supports_flac,
    audio_supports_opus: base.audio_supports_opus,
    container_supports_mp4: base.container_supports_mp4,
    container_supports_webm: base.container_supports_webm,
    mse_h264: base.mse_h264,
    mse_hevc: base.mse_hevc,
    platform: base.platform,
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

/**
 * 把 /api/... 资源地址映射到 Desktop 2.0 当前真实服务器。
 * Web 端的 getResolvedApiBaseURL() 仍是 /api，因此保持同源相对路径。
 */
function resolveRuntimeUrl(url: string): string {
  if (!url || /^(https?:|blob:|data:)/i.test(url)) return url

  const apiBase = getResolvedApiBaseURL().replace(/\/+$/, '')
  if (!/^https?:\/\//i.test(apiBase)) return url

  const serverBase = apiBase.replace(/\/api$/, '')
  if (url.startsWith('/api/')) return `${serverBase}${url}`
  if (url === '/api') return apiBase
  if (url.startsWith('/')) return `${serverBase}${url}`
  return `${apiBase}/${url}`
}

function withToken(url: string): string {
  const resolvedUrl = resolveRuntimeUrl(url)
  const token = useAuthStore.getState().token
  if (!token) return resolvedUrl
  const sep = resolvedUrl.includes('?') ? '&' : '?'
  return `${resolvedUrl}${sep}token=${encodeURIComponent(token)}`
}

function playbackSessionEndpoint(sessionId: string, suffix = ''): string {
  return `/playback/sessions/${encodeURIComponent(sessionId)}${suffix}`
}

export const streamApi = {
  getPlayInfo: async (mediaId: string) => {
    const caps = getCaps()
    const params = planParams(caps)
    const response = await api.get<{ data: PlannedMediaPlayInfo }>(`/stream/${mediaId}/info`, { params })

    const embeddedPlan = response.data.data.playback_plan
    if (embeddedPlan) {
      playbackPlanCache.set(mediaId, embeddedPlan)
      response.data.data = applyPlaybackPlan(response.data.data, embeddedPlan) as PlannedMediaPlayInfo
      return response
    }

    try {
      const planResponse = await api.get<{ data: PlaybackPlan }>(`/stream/${mediaId}/plan`, { params })
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
    const caps = getCaps()
    const params = planParams(caps, {
      supportsDirect: capabilities?.supportsDirect,
      supportsRemux: capabilities?.supportsRemux,
      forceTranscode: capabilities?.forceTranscode,
      maxBitrate: capabilities?.maxBitrate,
    })
    const response = await api.get<{ data: PlaybackPlan }>(`/stream/${mediaId}/plan`, { params })
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

    const apiBase = getResolvedApiBaseURL().replace(/\/+$/, '')
    const relative = `/api${playbackSessionEndpoint(sessionId)}?reason=${encodeURIComponent(reason)}`
    const url = /^https?:\/\//i.test(apiBase)
      ? `${apiBase}${playbackSessionEndpoint(sessionId)}?reason=${encodeURIComponent(reason)}`
      : relative

    return fetch(url, {
      method: 'DELETE',
      headers,
      keepalive: true,
      credentials: /^https?:\/\//i.test(apiBase) ? 'omit' : 'same-origin',
    })
  },

  getMasterUrl: (mediaId: string) => {
    const plan = playbackPlanCache.get(mediaId)
    const plannedHls = plan?.method === 'transcode' || plan?.method === 'startup_stream'
    if (!plannedHls || !plan.url) {
      throw new Error('播放计划未提供可用的 HLS 地址')
    }
    return withToken(plan.url)
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

  getBackdropUrl: (mediaId: string, version?: number) =>
    withToken(`/api/media/${mediaId}/backdrop${version ? `?v=${version}` : ''}`),

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
