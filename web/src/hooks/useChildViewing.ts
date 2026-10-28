import { useEffect, useState } from 'react'
import { userApi } from '@/api/user'
import type { FamilyViewingStatus } from '@/api/user'
import { useAuthStore } from '@/stores/auth'
import { usePlayerStore } from '@/stores/player'
import { getResolvedApiBaseURL } from '@/api/client'

// A route change waits for the previous page's release. This also prevents
// StrictMode's development mount/unmount cycle from opening duplicate grants.
let pendingRelease: Promise<unknown> = Promise.resolve()
const PREPARE_EVENT = 'nowen:prepare-child-viewing'
type PrepareRequest = { mediaID: string; handled: boolean; resolve: (allowed: boolean) => void }

// Seeking can prepare a new source before the video emits `play`. Obtain the
// same grant first, rather than allowing a paused seek to bypass the meter.
export function prepareChildViewing(mediaID: string): Promise<boolean> {
  if (!useAuthStore.getState().user?.child_mode) return Promise.resolve(true)
  return new Promise(resolve => {
    const detail: PrepareRequest = { mediaID, handled: false, resolve }
    window.dispatchEvent(new CustomEvent(PREPARE_EVENT, { detail }))
    if (!detail.handled) resolve(false)
  })
}

export function useChildViewing(mediaID: string | undefined, child: boolean, attempt: number, native = false, enabled = true) {
  const [ready, setReady] = useState(!child)
  const [error, setError] = useState('')
  const [metered, setMetered] = useState(false)
  useEffect(() => {
    if (!child || !mediaID) { setReady(true); setError(''); setMetered(false); return }
    if (!enabled) { setReady(false); setError(''); setMetered(false); return }
    const owner = useAuthStore.getState().user?.id
    const token = useAuthStore.getState().token
    let disposed = false
    let halted = false
    let limited = false
    let lease = ''
    let deadline = 0
    let lastGrant: FamilyViewingStatus | undefined
    let desiredPlaying = true
    let resumeVideo: HTMLVideoElement | undefined
    const suppressedPauses = new WeakSet<HTMLVideoElement>()
    let watchdog = 0
    let queue: Promise<unknown> = Promise.resolve()
    setReady(false); setError('')
    const pause = () => {
      document.querySelectorAll('video').forEach(video => {
        if (!video.paused) suppressedPauses.add(video)
        video.pause()
      })
      usePlayerStore.getState().setPlaying(false)
    }
    const release = async () => {
      const id = lease
      lease = ''; window.clearTimeout(watchdog)
      if (id) await userApi.stopViewing(id)
    }
    const fail = (message: string) => {
      if (disposed || halted) return
      halted = true; desiredPlaying = false; resumeVideo = undefined
      pause(); setError(message)
      window.clearTimeout(watchdog)
    }
    const accept = (grant: FamilyViewingStatus, requestStarted: number) => {
      if (halted || disposed) return
      if (grant.daily_limit_seconds > 0 && grant.remaining_seconds <= 0) {
        fail('今天的观看时间已用完，休息一下吧')
        return
      }
      lease = grant.lease_id || ''
      lastGrant = grant
      // Measure from request dispatch, not response arrival or the user's clock.
      // A slow response shortens the usable grant instead of extending it.
      deadline = requestStarted + (grant.expires_in_ms || 0)
      window.clearTimeout(watchdog)
      if (!lease || deadline <= performance.now()) throw new Error('授权响应已超时，请重试')
      watchdog = window.setTimeout(() => fail(
        grant.remaining_seconds <= Math.ceil((grant.expires_in_ms || 0) / 1000)
          ? '今天的观看时间已用完，休息一下吧'
          : '观看授权已到期，请检查连接后重试',
      ), Math.max(0, deadline - performance.now()))
    }
    const run = (renew = false) => {
      queue = queue.then(async () => {
        if (disposed || !limited || halted) return
        if (!desiredPlaying) { await release(); return }
        if (!lease) {
          const started = performance.now()
          const { data } = await userApi.startViewing(mediaID)
          accept(data.data, started)
        } else if (renew) {
          const started = performance.now()
          const { data } = await userApi.renewViewing(lease)
          accept(data.data, started)
        }
        if (!disposed && desiredPlaying && resumeVideo) {
          const video = resumeVideo; resumeVideo = undefined
          void video.play().catch(() => { desiredPlaying = false; run() })
        }
      }).catch((cause: { message?: string; response?: { data?: { error?: string } } }) => {
        fail(cause.response?.data?.error || '暂时无法确认观看时间，请检查连接后重试')
      })
      return queue
    }
    const onPlay = (event: Event) => {
      if (!limited || !(event.target instanceof HTMLVideoElement)) return
      desiredPlaying = true
      if (!lease || performance.now() >= deadline) {
        resumeVideo = event.target; pause(); void run()
      }
    }
    const prepare = (event: Event) => {
      const request = (event as CustomEvent<PrepareRequest>).detail
      if (request.mediaID !== mediaID) return
      request.handled = true
      if (!limited) { request.resolve(!disposed && !halted); return }
      desiredPlaying = true
      void run().then(() => request.resolve(!disposed && !halted && !!lease && performance.now() < deadline))
    }
    const onPause = (event?: Event) => {
      if (event?.target instanceof HTMLVideoElement && suppressedPauses.has(event.target)) { suppressedPauses.delete(event.target); return }
      if (!limited) return
      desiredPlaying = false; resumeVideo = undefined; void run()
    }
    const focus = () => {
      if (limited && lease && performance.now() >= deadline) fail(
        lastGrant && lastGrant.remaining_seconds <= 15 ? '今天的观看时间已用完，休息一下吧' : '观看授权已到期，请重试',
      )
    }
    const hide = () => { if (document.hidden && limited) { pause(); onPause() } else focus() }
    const pagehide = () => {
      if (!lease || !token) return
      void fetch(`${getResolvedApiBaseURL()}/users/me/viewing/stop`, {
        method: 'POST', keepalive: true, headers: { 'Content-Type': 'application/json', Authorization: `Bearer ${token}` },
        body: JSON.stringify({ lease_id: lease }),
      }).catch(() => {})
    }
    const initialize = window.setTimeout(() => {
      queue = pendingRelease.catch(() => {}).then(async () => {
        if (disposed) return
        const { data } = await userApi.familyControls()
        if (disposed) return
        if (!data.data.child_mode) throw new Error('账号权限已变化，请重新登录')
        const current = useAuthStore.getState().user
        if (!current || current.id !== owner) return
        useAuthStore.getState().updateUser({ ...current, viewing_preferences: { ...current.viewing_preferences, auto_play_next: data.data.allow_auto_play_next } })
        limited = data.data.daily_limit_seconds > 0
        setMetered(limited)
        if (limited && native) throw new Error('儿童限时观看请使用网页端')
        if (limited) {
          const started = performance.now()
          const response = await userApi.startViewing(mediaID)
          accept(response.data.data, started)
        }
        if (!disposed) setReady(true)
      }).catch((cause: { message?: string; response?: { data?: { error?: string } } }) => {
        fail(cause.response?.data?.error || (cause.message === '儿童限时观看请使用网页端' ? cause.message : '暂时无法确认家长设置，请检查连接后重试'))
      })
    }, 0)
    const interval = window.setInterval(() => { if (lease && desiredPlaying) void run(true) }, 5000)
    document.addEventListener('play', onPlay, true)
    document.addEventListener('pause', onPause, true)
    document.addEventListener('ended', onPause, true)
    document.addEventListener('visibilitychange', hide)
    window.addEventListener('focus', focus)
    window.addEventListener('pagehide', pagehide)
    window.addEventListener(PREPARE_EVENT, prepare)
    return () => {
      disposed = true; window.clearTimeout(initialize); window.clearTimeout(watchdog); window.clearInterval(interval)
      document.removeEventListener('play', onPlay, true); document.removeEventListener('pause', onPause, true)
      document.removeEventListener('ended', onPause, true); document.removeEventListener('visibilitychange', hide)
      window.removeEventListener('focus', focus); window.removeEventListener('pagehide', pagehide)
      window.removeEventListener(PREPARE_EVENT, prepare)
      pendingRelease = Promise.allSettled([pendingRelease, queue]).then(release).catch(() => {})
    }
  // The route and retry own the grant, independently of player re-renders.
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [mediaID, child, attempt, native, enabled])
  return { ready, error, metered }
}
