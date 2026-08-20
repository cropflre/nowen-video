import { useCallback, useEffect, useRef, useState } from 'react'
import { useLocation } from 'react-router-dom'
import { streamApi } from '@/api'
import api, { getResolvedApiBaseURL } from '@/api/client'
import { invalidatePageCachePrefix } from '@/hooks/usePageCache'
import { desktop } from '@/platform/desktop/bridge'
import { useAuthStore } from '@/stores/auth'
import { usePlayerStore } from '@/stores/player'

const FIRST_HISTORY_REPORT_SECONDS = 3
const DESKTOP_PROGRESS_REPORT_INTERVAL_MS = 15_000
const MIN_PROGRESS_SECONDS = 0.25

function mediaIdFromPath(pathname: string): string | null {
  const match = pathname.match(/^\/play\/([^/]+)$/)
  if (!match?.[1]) return null
  try {
    return decodeURIComponent(match[1])
  } catch {
    return match[1]
  }
}

function normalizedProgress(position: number, duration: number) {
  if (!Number.isFinite(position) || !Number.isFinite(duration)) return null
  if (position <= MIN_PROGRESS_SECONDS || duration <= 0) return null
  return {
    position: Math.min(Math.max(position, 0), duration),
    duration,
  }
}

function invalidatePlaybackCaches() {
  invalidatePageCachePrefix('history:')
  invalidatePageCachePrefix('home:')
}

async function reportAbsoluteProgress(mediaId: string, position: number, duration: number) {
  const payload = normalizedProgress(position, duration)
  if (!payload) return false

  try {
    await api.put(`/users/me/progress/${encodeURIComponent(mediaId)}`, payload)
    invalidatePlaybackCaches()
    window.dispatchEvent(new CustomEvent('nowen:watch-progress-updated', {
      detail: { mediaId, ...payload },
    }))
    return true
  } catch {
    return false
  }
}

function reportAbsoluteProgressKeepalive(mediaId: string, position: number, duration: number) {
  const payload = normalizedProgress(position, duration)
  if (!payload) return

  const token = useAuthStore.getState().token
  const baseURL = getResolvedApiBaseURL().replace(/\/$/, '')
  void fetch(`${baseURL}/users/me/progress/${encodeURIComponent(mediaId)}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: JSON.stringify(payload),
    keepalive: true,
  }).catch(() => undefined)
}

/**
 * PlaybackHistoryBridge is the route-level safety net for watch history.
 *
 * Browser <video> playback already performs periodic reporting internally, but
 * short sessions could leave the route before the first 15-second report. The
 * Desktop 2.0 native player has no browser video element at all, so it also needs
 * a route-level reporter. Keeping this bridge above every playback engine gives
 * all modes the same product contract:
 *   - once playback has genuinely advanced, create history promptly;
 *   - Desktop keeps reporting while it plays;
 *   - navigation/page-hide flushes the latest absolute position;
 *   - highlights never pollute normal watch history.
 */
export default function PlaybackHistoryBridge() {
  const location = useLocation()
  const mediaId = mediaIdFromPath(location.pathname)
  const highlightMode = new URLSearchParams(location.search).get('mode') === 'highlight'
  const currentTime = usePlayerStore((state) => state.currentTime)
  const storeDuration = usePlayerStore((state) => state.duration)
  const isPlaying = usePlayerStore((state) => state.isPlaying)

  const [knownDuration, setKnownDuration] = useState(0)
  const knownDurationRef = useRef(0)
  const firstReportedRef = useRef(false)
  const initialPositionRef = useRef(0)
  const lastReportedPositionRef = useRef(0)
  const reportInFlightRef = useRef(false)

  useEffect(() => {
    knownDurationRef.current = knownDuration
  }, [knownDuration])

  useEffect(() => {
    firstReportedRef.current = false
    lastReportedPositionRef.current = 0
    reportInFlightRef.current = false
    initialPositionRef.current = usePlayerStore.getState().currentTime
    knownDurationRef.current = 0
    setKnownDuration(0)

    if (!mediaId || highlightMode) return

    let cancelled = false
    streamApi.getPlayInfo(mediaId)
      .then((response) => {
        if (cancelled) return
        const duration = Number(response.data.data?.duration || 0)
        if (Number.isFinite(duration) && duration > 0) {
          knownDurationRef.current = duration
          setKnownDuration(duration)
        }
      })
      .catch(() => undefined)

    return () => {
      cancelled = true
    }
  }, [highlightMode, mediaId])

  const reportSnapshot = useCallback(async (force = false) => {
    if (!mediaId || highlightMode || reportInFlightRef.current) return false

    const state = usePlayerStore.getState()
    const position = Number(state.currentTime || 0)
    const duration = Number(knownDurationRef.current || state.duration || 0)
    const payload = normalizedProgress(position, duration)
    if (!payload) return false

    if (!force && Math.abs(payload.position - lastReportedPositionRef.current) < 1) return false

    reportInFlightRef.current = true
    const success = await reportAbsoluteProgress(mediaId, payload.position, payload.duration)
    reportInFlightRef.current = false
    if (success) lastReportedPositionRef.current = payload.position
    return success
  }, [highlightMode, mediaId])

  // Product rule: opening the player alone is not a watch. Once playback has
  // advanced for a few seconds, however, history should already exist before the
  // user navigates back to Home/History.
  useEffect(() => {
    if (!mediaId || highlightMode || firstReportedRef.current) return
    const duration = knownDuration || storeDuration
    if (duration <= 0 || currentTime < FIRST_HISTORY_REPORT_SECONDS) return

    const advancedFromEntry = Math.abs(currentTime - initialPositionRef.current) >= 1
    if (!isPlaying && !advancedFromEntry) return

    firstReportedRef.current = true
    void reportSnapshot(true).then((success) => {
      if (!success) firstReportedRef.current = false
    })
  }, [currentTime, highlightMode, isPlaying, knownDuration, mediaId, reportSnapshot, storeDuration])

  // Desktop 2.0 renders through libmpv and therefore bypasses VideoPlayer's
  // existing browser interval. Keep its long-running progress current here.
  useEffect(() => {
    if (!mediaId || highlightMode || !desktop.isDesktop) return
    const timer = window.setInterval(() => {
      const state = usePlayerStore.getState()
      if (!state.isPlaying || state.currentTime <= MIN_PROGRESS_SECONDS) return
      void reportSnapshot()
    }, DESKTOP_PROGRESS_REPORT_INTERVAL_MS)
    return () => window.clearInterval(timer)
  }, [highlightMode, mediaId, reportSnapshot])

  // Normal SPA navigation gets an async flush. This closes the old gap where a
  // user could watch for <15s, press Back, and never create a history row.
  useEffect(() => {
    if (!mediaId || highlightMode) return
    const routeMediaId = mediaId
    return () => {
      const state = usePlayerStore.getState()
      const duration = Number(knownDurationRef.current || state.duration || 0)
      const payload = normalizedProgress(Number(state.currentTime || 0), duration)
      if (!payload) return
      void reportAbsoluteProgress(routeMediaId, payload.position, payload.duration)
    }
  }, [highlightMode, mediaId])

  // Browser/tab/app shutdown cannot wait for axios. Use keepalive with the last
  // resolved API base so Web and Desktop both get a best-effort final flush.
  useEffect(() => {
    if (!mediaId || highlightMode) return
    const handlePageHide = () => {
      const state = usePlayerStore.getState()
      const duration = Number(knownDurationRef.current || state.duration || 0)
      reportAbsoluteProgressKeepalive(mediaId, Number(state.currentTime || 0), duration)
    }
    window.addEventListener('pagehide', handlePageHide)
    return () => window.removeEventListener('pagehide', handlePageHide)
  }, [highlightMode, mediaId])

  return null
}
