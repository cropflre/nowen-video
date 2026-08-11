import { useCallback, useEffect, useRef } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import VideoPlayer from './VideoPlayer'
import { usePlaybackSessionSource } from '@/hooks/usePlaybackSessionSource'
import { usePlayerStore } from '@/stores/player'
import { clearPlaybackSessionRuntime, setPlaybackSessionRuntime } from '@/playback/sessionRuntime'

interface SessionVideoPlayerProps {
  mediaId: string
  title?: string
  startPosition?: number
  onBack?: () => void
  onNext?: () => void
  nextTitle?: string
  knownDuration?: number
  onPreprocessReady?: () => void
  spriteVttUrl?: string
}

function formatTimestamp(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds <= 0) return '0:00'
  const total = Math.floor(seconds)
  const hours = Math.floor(total / 3600)
  const minutes = Math.floor((total % 3600) / 60)
  const remainingSeconds = total % 60
  if (hours > 0) return `${hours}:${minutes.toString().padStart(2, '0')}:${remainingSeconds.toString().padStart(2, '0')}`
  return `${minutes}:${remainingSeconds.toString().padStart(2, '0')}`
}

export default function SessionVideoPlayer({
  mediaId,
  title,
  startPosition = 0,
  onBack,
  onNext,
  nextTitle,
  knownDuration,
  onPreprocessReady,
  spriteVttUrl,
}: SessionVideoPlayerProps) {
  const rootRef = useRef<HTMLDivElement>(null)
  const absolutePositionRef = useRef(Math.max(0, startPosition))
  const seekTargetRef = useRef<number | null>(null)
  const seekInFlightRef = useRef(false)
  const resumeAfterSeekRef = useRef(false)
  const playback = usePlaybackSessionSource({ enabled: true, mediaId, startPosition })
  const isSeeking = playback.loading && Boolean(playback.source)

  useEffect(() => {
    if (!playback.sessionId || playback.generationId <= 0) return
    const pendingTarget = seekTargetRef.current
    const timelinePosition = isSeeking && pendingTarget !== null ? pendingTarget : playback.offsetSeconds
    absolutePositionRef.current = timelinePosition
    setPlaybackSessionRuntime(mediaId, {
      sessionId: playback.sessionId,
      generationId: playback.generationId,
      offsetSeconds: playback.offsetSeconds,
    })
    const frame = window.requestAnimationFrame(() => usePlayerStore.getState().setCurrentTime(timelinePosition))
    if (!isSeeking && pendingTarget !== null && Math.abs(playback.offsetSeconds - pendingTarget) < 1) seekTargetRef.current = null
    return () => {
      window.cancelAnimationFrame(frame)
      clearPlaybackSessionRuntime(mediaId, playback.sessionId || undefined)
    }
  }, [mediaId, playback.sessionId, playback.generationId, playback.offsetSeconds, isSeeking])

  useEffect(() => {
    if (isSeeking || !resumeAfterSeekRef.current || !playback.source) return
    let disposed = false
    let timer = 0
    let attempts = 0
    const resume = () => {
      if (disposed || !resumeAfterSeekRef.current) return
      const video = rootRef.current?.querySelector('video') || null
      if (!video) {
        timer = window.setTimeout(resume, 50)
        return
      }
      if (!video.paused) {
        resumeAfterSeekRef.current = false
        return
      }
      attempts += 1
      void video.play().then(() => { resumeAfterSeekRef.current = false }).catch(() => {
        if (!disposed && attempts < 20) timer = window.setTimeout(resume, 100)
      })
    }
    timer = window.setTimeout(resume, 0)
    return () => {
      disposed = true
      window.clearTimeout(timer)
    }
  }, [isSeeking, playback.source, playback.generationId])

  useEffect(() => {
    if (!playback.source || !playback.sessionId) return
    let disposed = false
    let video: HTMLVideoElement | null = null
    let heartbeatTimer = 0
    let discoverTimer = 0

    const writeAbsolutePosition = (absolutePosition: number) => {
      absolutePositionRef.current = absolutePosition
      queueMicrotask(() => {
        if (!disposed) usePlayerStore.getState().setCurrentTime(absolutePosition)
      })
    }

    const updateAbsolutePosition = () => {
      if (!video) return
      const pendingTarget = seekTargetRef.current
      if (playback.loading && pendingTarget !== null) {
        writeAbsolutePosition(pendingTarget)
        return
      }
      const relativePosition = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0
      writeAbsolutePosition(playback.offsetSeconds + relativePosition)
    }

    const reportHeartbeat = () => {
      if (!video || playback.loading) return
      const relativePosition = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0
      let bufferedEnd = relativePosition
      if (video.buffered.length > 0) bufferedEnd = video.buffered.end(video.buffered.length - 1)
      void playback.heartbeat(playback.offsetSeconds + relativePosition, playback.offsetSeconds + bufferedEnd, video.paused)
    }

    const onEnded = () => {
      updateAbsolutePosition()
      reportHeartbeat()
      void playback.close('playback_ended')
    }
    const onPageHide = () => { void playback.close('page_hidden', true) }

    const attach = () => {
      if (disposed) return
      video = rootRef.current?.querySelector('video') || null
      if (!video) {
        discoverTimer = window.setTimeout(attach, 50)
        return
      }
      video.addEventListener('timeupdate', updateAbsolutePosition)
      video.addEventListener('loadedmetadata', updateAbsolutePosition)
      video.addEventListener('durationchange', updateAbsolutePosition)
      video.addEventListener('ended', onEnded)
      window.addEventListener('pagehide', onPageHide)
      updateAbsolutePosition()
      reportHeartbeat()
      heartbeatTimer = window.setInterval(reportHeartbeat, Math.max(5, playback.heartbeatIntervalSec) * 1000)
    }

    attach()
    return () => {
      disposed = true
      clearTimeout(discoverTimer)
      clearInterval(heartbeatTimer)
      video?.removeEventListener('timeupdate', updateAbsolutePosition)
      video?.removeEventListener('loadedmetadata', updateAbsolutePosition)
      video?.removeEventListener('durationchange', updateAbsolutePosition)
      video?.removeEventListener('ended', onEnded)
      window.removeEventListener('pagehide', onPageHide)
    }
  }, [
    playback.source,
    playback.sessionId,
    playback.generationId,
    playback.offsetSeconds,
    playback.heartbeatIntervalSec,
    playback.loading,
    playback.heartbeat,
    playback.close,
  ])

  const requestSeek = useCallback((targetSeconds: number, reason: string) => {
    if (!playback.sessionId || playback.loading || seekInFlightRef.current) return
    const upperBound = knownDuration && knownDuration > 0 ? knownDuration : Number.MAX_SAFE_INTEGER
    const target = Math.max(0, Math.min(upperBound, targetSeconds))
    const previousPosition = absolutePositionRef.current
    const video = rootRef.current?.querySelector('video') || null
    const wasPlaying = Boolean(video && !video.paused)
    seekInFlightRef.current = true
    resumeAfterSeekRef.current = wasPlaying
    seekTargetRef.current = target
    absolutePositionRef.current = target
    usePlayerStore.getState().setCurrentTime(target)

    void playback.restart(target, reason).then((success) => {
      if (success) return
      resumeAfterSeekRef.current = false
      seekTargetRef.current = null
      absolutePositionRef.current = previousPosition
      usePlayerStore.getState().setCurrentTime(previousPosition)
    }).finally(() => { seekInFlightRef.current = false })
  }, [knownDuration, playback.sessionId, playback.loading, playback.restart])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!playback.sessionId || playback.loading) return
      if (event.altKey || event.ctrlKey || event.metaKey) return
      const target = event.target as HTMLElement | null
      if (target?.closest('input, textarea, select, [contenteditable="true"]')) return
      let delta = 0
      if (event.key === 'ArrowLeft' || event.key.toLowerCase() === 'j') delta = -10
      if (event.key === 'ArrowRight' || event.key.toLowerCase() === 'l') delta = 10
      if (delta === 0) return
      event.preventDefault()
      event.stopImmediatePropagation()
      requestSeek(absolutePositionRef.current + delta, 'keyboard_seek')
    }
    window.addEventListener('keydown', handleKeyDown, true)
    return () => window.removeEventListener('keydown', handleKeyDown, true)
  }, [playback.sessionId, playback.loading, requestSeek])

  const handlePlayerClickCapture = (event: React.MouseEvent<HTMLDivElement>) => {
    const target = event.target as HTMLElement | null
    if (!target) return
    const progressBar = target.closest<HTMLElement>('.progress-bar')
    if (progressBar && rootRef.current?.contains(progressBar)) {
      const rect = progressBar.getBoundingClientRect()
      if (rect.width <= 0) return
      const ratio = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width))
      const duration = knownDuration && knownDuration > 0 ? knownDuration : usePlayerStore.getState().duration
      if (duration <= 0) return
      event.preventDefault()
      event.stopPropagation()
      event.nativeEvent.stopImmediatePropagation()
      requestSeek(ratio * duration, 'progress_seek')
      return
    }
    const button = target.closest<HTMLButtonElement>('button')
    if (!button || !rootRef.current?.contains(button)) return
    if (button.querySelector('[class*="lucide-skip-forward"]')) {
      event.preventDefault()
      event.stopPropagation()
      event.nativeEvent.stopImmediatePropagation()
      requestSeek(absolutePositionRef.current + 10, 'skip_forward')
      return
    }
    if (button.querySelector('[class*="lucide-skip-back"]')) {
      const parent = button.parentElement
      const isTitleBack = Boolean(parent?.classList.contains('absolute') && parent.classList.contains('top-4') && parent.classList.contains('left-4'))
      if (!isTitleBack) {
        event.preventDefault()
        event.stopPropagation()
        event.nativeEvent.stopImmediatePropagation()
        requestSeek(absolutePositionRef.current - 10, 'skip_backward')
      }
    }
  }

  const handleBack = () => { void playback.close('navigate_back', true); onBack?.() }
  const handleNext = () => { void playback.close('next_media', true); onNext?.() }
  const handlePreprocessReady = () => { void playback.close('switch_to_preprocessed', true); onPreprocessReady?.() }

  if (!playback.source) {
    if (playback.error) {
      return (
        <div className="group/player flex h-full w-full flex-col items-center justify-center gap-3 bg-[var(--nv-player-canvas)] px-6 text-center text-[var(--nv-player-text-primary)]">
          <AlertTriangle className="text-[var(--nv-player-danger)]" size={32} aria-hidden="true" />
          <p className="text-base font-medium">转码播放启动失败</p>
          <p className="max-w-xl text-sm text-[var(--nv-player-text-tertiary)]">{playback.error}</p>
          {onBack && <button type="button" onClick={handleBack} className="mt-2 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-4 py-2 text-sm transition-colors hover:bg-[var(--nv-player-surface-hover)]">返回</button>}
        </div>
      )
    }
    return (
      <div className="group/player flex h-full w-full flex-col items-center justify-center gap-3 bg-[var(--nv-player-canvas)] text-[var(--nv-player-text-primary)]">
        <Loader2 className="animate-spin text-[var(--nv-player-accent)]" size={32} aria-hidden="true" />
        <p className="text-sm text-[var(--nv-player-text-tertiary)]">正在生成首个播放分片...</p>
      </div>
    )
  }

  const seekTarget = seekTargetRef.current ?? playback.offsetSeconds

  return (
    <div ref={rootRef} className="group/player relative h-full w-full overflow-hidden bg-[var(--nv-player-canvas)]" onClickCapture={handlePlayerClickCapture} aria-busy={isSeeking}>
      <VideoPlayer
        src={playback.source}
        mode="hls"
        mediaId={mediaId}
        title={title}
        startPosition={0}
        knownDuration={knownDuration}
        onBack={handleBack}
        onNext={onNext ? handleNext : undefined}
        nextTitle={nextTitle}
        onPreprocessReady={onPreprocessReady ? handlePreprocessReady : undefined}
        spriteVttUrl={spriteVttUrl}
      />

      {isSeeking && (
        <div className="pointer-events-none absolute left-1/2 top-16 z-50 flex -translate-x-1/2 items-center gap-3 rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] px-4 py-2.5 text-[var(--nv-player-text-primary)] shadow-[var(--nv-player-shadow)] backdrop-blur-xl" role="status" aria-live="polite">
          <Loader2 className="animate-spin text-[var(--nv-player-accent)]" size={17} aria-hidden="true" />
          <div className="leading-tight"><p className="text-xs font-medium">正在跳转</p><p className="mt-0.5 font-mono text-[11px] text-[var(--nv-player-text-tertiary)]">{formatTimestamp(seekTarget)}</p></div>
        </div>
      )}

      {playback.error && (
        <div className="pointer-events-none absolute left-1/2 top-16 z-50 flex max-w-[min(90vw,520px)] -translate-x-1/2 items-center gap-2 rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-danger-soft)] px-4 py-2.5 text-sm text-[var(--nv-player-danger)] shadow-[var(--nv-player-shadow)] backdrop-blur-xl">
          <AlertTriangle size={16} className="shrink-0" aria-hidden="true" /><span className="truncate">跳转失败：{playback.error}</span>
        </div>
      )}
    </div>
  )
}
