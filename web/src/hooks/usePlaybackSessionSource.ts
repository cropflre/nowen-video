import { useCallback, useEffect, useRef, useState } from 'react'
import {
  streamApi,
  type PlaybackSessionResult,
  type RestartPlaybackSessionRequest,
} from '@/api/stream'

const STATUS_POLL_INTERVAL_MS = 250
const SESSION_STARTUP_DEADLINE_MS = 30_000

interface PlaybackSessionHandle {
  sessionId: string
  generationId: number
  heartbeatIntervalSec: number
  profileId: string
}

interface UsePlaybackSessionSourceOptions {
  enabled: boolean
  mediaId: string
  startPosition?: number
}

export interface PlaybackSessionSource {
  source: string
  loading: boolean
  error: string | null
  sessionId: string | null
  generationId: number
  offsetSeconds: number
  heartbeatIntervalSec: number
  restart: (positionSeconds: number, reason?: string, overrides?: Partial<RestartPlaybackSessionRequest>) => Promise<boolean>
  heartbeat: (positionSeconds: number, bufferedEndSeconds: number, paused: boolean) => Promise<void>
  close: (reason?: string, keepalive?: boolean) => Promise<void>
}

function delay(milliseconds: number): Promise<void> {
  return new Promise((resolve) => window.setTimeout(resolve, milliseconds))
}

function sessionFailure(result: PlaybackSessionResult): Error | null {
  const generation = result.session.generation
  if (generation?.error_message) return new Error(generation.error_message)
  if (generation?.error_code) return new Error(generation.error_code)
  if (result.session.close_reason) return new Error(result.session.close_reason)
  if (result.session.state === 'failed') return new Error('播放转码启动失败')
  if (result.session.state === 'expired') return new Error('播放会话已过期')
  if (result.session.state === 'closing' || result.session.state === 'closed') {
    return new Error('播放会话已关闭')
  }
  return null
}

async function waitUntilReady(
  initial: PlaybackSessionResult,
  isCurrent: () => boolean,
): Promise<PlaybackSessionResult> {
  let result = initial
  const deadline = Date.now() + SESSION_STARTUP_DEADLINE_MS

  while (isCurrent()) {
    if (result.first_segment_ready && result.playlist_url && result.session.current_generation_id) {
      return result
    }
    const failure = sessionFailure(result)
    if (failure) throw failure
    if (Date.now() >= deadline) throw new Error('首个转码分片生成超时')

    await delay(STATUS_POLL_INTERVAL_MS)
    if (!isCurrent()) throw new Error('播放会话已被替换')
    const response = await streamApi.getPlaybackSessionStatus(result.session.id)
    result = response.data.data
  }

  throw new Error('播放会话已被替换')
}

export function usePlaybackSessionSource({
  enabled,
  mediaId,
  startPosition = 0,
}: UsePlaybackSessionSourceOptions): PlaybackSessionSource {
  const [source, setSource] = useState('')
  const [loading, setLoading] = useState(enabled)
  const [error, setError] = useState<string | null>(null)
  const [sessionId, setSessionId] = useState<string | null>(null)
  const [generationId, setGenerationId] = useState(0)
  const [offsetSeconds, setOffsetSeconds] = useState(Math.max(0, startPosition))
  const [heartbeatIntervalSec, setHeartbeatIntervalSec] = useState(15)

  const activeRef = useRef<PlaybackSessionHandle | null>(null)
  const operationRef = useRef(0)
  const restartingRef = useRef(false)

  const close = useCallback(async (reason = 'client_closed', keepalive = false) => {
    const active = activeRef.current
    activeRef.current = null
    setSessionId(null)
    setGenerationId(0)
    if (!active) return

    try {
      if (keepalive) {
        await streamApi.closePlaybackSessionKeepalive(active.sessionId, reason)
      } else {
        await streamApi.closePlaybackSession(active.sessionId, reason)
      }
    } catch {
      // The server-side idle reaper remains the authoritative fallback when a
      // browser closes before the DELETE request is delivered.
    }
  }, [])

  useEffect(() => {
    const operation = ++operationRef.current
    let disposed = false

    if (!enabled) {
      setSource('')
      setLoading(false)
      setError(null)
      setOffsetSeconds(Math.max(0, startPosition))
      void close('playback_mode_changed', true)
      return () => {
        disposed = true
      }
    }

    setSource('')
    setLoading(true)
    setError(null)
    setOffsetSeconds(Math.max(0, startPosition))

    const isCurrent = () => !disposed && operationRef.current === operation

    void (async () => {
      await close('source_changed')
      const plan = streamApi.getCachedPlaybackPlan(mediaId)
      const template = plan?.session_template

      let initial: PlaybackSessionResult | null = null
      try {
        const response = await streamApi.createPlaybackSession({
          media_id: mediaId,
          profile_id: template?.profile_id || 'auto',
          start_position_ms: Math.max(0, Math.round(startPosition * 1000)),
          max_bitrate: template?.max_bitrate,
          audio_track: 0,
          subtitle_track: -1,
          burn_subtitle: false,
        })
        initial = response.data.data

        if (!isCurrent()) {
          await streamApi.closePlaybackSessionKeepalive(initial.session.id, 'superseded_before_ready').catch(() => undefined)
          return
        }

        const ready = await waitUntilReady(initial, isCurrent)
        if (!isCurrent()) {
          await streamApi.closePlaybackSessionKeepalive(ready.session.id, 'superseded_after_ready').catch(() => undefined)
          return
        }

        const currentGeneration = ready.session.current_generation_id || 0
        const profileId = ready.session.generation?.profile_id || template?.profile_id || 'auto'
        activeRef.current = {
          sessionId: ready.session.id,
          generationId: currentGeneration,
          heartbeatIntervalSec: ready.heartbeat_interval_sec || 15,
          profileId,
        }
        setSessionId(ready.session.id)
        setGenerationId(currentGeneration)
        setHeartbeatIntervalSec(ready.heartbeat_interval_sec || 15)
        setSource(ready.playlist_url || '')
        setLoading(false)
      } catch (cause) {
        if (initial && !activeRef.current) {
          await streamApi.closePlaybackSessionKeepalive(initial.session.id, 'startup_failed').catch(() => undefined)
        }
        if (!isCurrent()) return
        setLoading(false)
        setError(cause instanceof Error ? cause.message : '播放会话创建失败')
      }
    })()

    return () => {
      disposed = true
      if (operationRef.current === operation) operationRef.current++
      void close('component_unmounted', true)
    }
  }, [enabled, mediaId, startPosition, close])

  const restart = useCallback(async (
    positionSeconds: number,
    reason = 'seek',
    overrides: Partial<RestartPlaybackSessionRequest> = {},
  ): Promise<boolean> => {
    const active = activeRef.current
    if (!active || restartingRef.current) return false

    restartingRef.current = true
    setLoading(true)
    setError(null)
    const operation = ++operationRef.current
    const isCurrent = () => operationRef.current === operation && activeRef.current?.sessionId === active.sessionId

    try {
      const response = await streamApi.restartPlaybackSession(active.sessionId, {
        profile_id: overrides.profile_id || active.profileId,
        start_position_ms: Math.max(0, Math.round(positionSeconds * 1000)),
        audio_track: overrides.audio_track ?? 0,
        subtitle_track: overrides.subtitle_track ?? -1,
        burn_subtitle: overrides.burn_subtitle ?? false,
        max_bitrate: overrides.max_bitrate,
        reason,
      })
      const ready = await waitUntilReady(response.data.data, isCurrent)
      if (!isCurrent()) return false

      const currentGeneration = ready.session.current_generation_id || 0
      activeRef.current = {
        ...active,
        generationId: currentGeneration,
        heartbeatIntervalSec: ready.heartbeat_interval_sec || active.heartbeatIntervalSec,
        profileId: ready.session.generation?.profile_id || active.profileId,
      }
      setGenerationId(currentGeneration)
      setHeartbeatIntervalSec(ready.heartbeat_interval_sec || active.heartbeatIntervalSec)
      setOffsetSeconds(Math.max(0, positionSeconds))
      setSource(ready.playlist_url || '')
      setLoading(false)
      return true
    } catch (cause) {
      if (isCurrent()) {
        setLoading(false)
        setError(cause instanceof Error ? cause.message : '播放位置切换失败')
      }
      return false
    } finally {
      restartingRef.current = false
    }
  }, [])

  const heartbeat = useCallback(async (
    positionSeconds: number,
    bufferedEndSeconds: number,
    paused: boolean,
  ) => {
    const active = activeRef.current
    if (!active) return
    try {
      await streamApi.heartbeatPlaybackSession(active.sessionId, {
        generation_id: active.generationId,
        position_ms: Math.max(0, Math.round(positionSeconds * 1000)),
        buffered_end_ms: Math.max(0, Math.round(bufferedEndSeconds * 1000)),
        paused,
      })
    } catch {
      // Segment reads also refresh liveness; a single delayed heartbeat is not
      // fatal and the next interval will retry.
    }
  }, [])

  return {
    source,
    loading,
    error,
    sessionId,
    generationId,
    offsetSeconds,
    heartbeatIntervalSec,
    restart,
    heartbeat,
    close,
  }
}
