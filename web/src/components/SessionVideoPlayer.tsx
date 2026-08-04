import { useCallback, useEffect, useRef } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import VideoPlayer from './VideoPlayer'
import { usePlaybackSessionSource } from '@/hooks/usePlaybackSessionSource'
import { usePlayerStore } from '@/stores/player'
import {
  clearPlaybackSessionRuntime,
  setPlaybackSessionRuntime,
} from '@/playback/sessionRuntime'

interface SessionVideoPlayerProps {
  fallbackSrc: string
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

export default function SessionVideoPlayer({
  fallbackSrc,
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
  const playback = usePlaybackSessionSource({
    enabled: true,
    mediaId,
    fallbackSource: fallbackSrc,
    startPosition,
  })

  useEffect(() => {
    if (!playback.sessionId || playback.generationId <= 0) return
    absolutePositionRef.current = playback.offsetSeconds
    setPlaybackSessionRuntime(mediaId, {
      sessionId: playback.sessionId,
      generationId: playback.generationId,
      offsetSeconds: playback.offsetSeconds,
    })
    usePlayerStore.getState().setCurrentTime(playback.offsetSeconds)

    return () => {
      clearPlaybackSessionRuntime(mediaId, playback.sessionId || undefined)
    }
  }, [mediaId, playback.sessionId, playback.generationId, playback.offsetSeconds])

  useEffect(() => {
    if (!playback.source || !playback.sessionId) return

    let disposed = false
    let video: HTMLVideoElement | null = null
    let heartbeatTimer = 0
    let discoverTimer = 0

    const updateAbsolutePosition = () => {
      if (!video) return
      const relativePosition = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0
      const absolutePosition = playback.offsetSeconds + relativePosition
      absolutePositionRef.current = absolutePosition
      // VideoPlayer owns the visible timeline store. Session playback writes the
      // absolute media position after its relative timeupdate listener runs.
      usePlayerStore.getState().setCurrentTime(absolutePosition)
    }

    const reportHeartbeat = () => {
      if (!video) return
      const relativePosition = Number.isFinite(video.currentTime) ? Math.max(0, video.currentTime) : 0
      let bufferedEnd = relativePosition
      if (video.buffered.length > 0) {
        bufferedEnd = video.buffered.end(video.buffered.length - 1)
      }
      void playback.heartbeat(
        playback.offsetSeconds + relativePosition,
        playback.offsetSeconds + bufferedEnd,
        video.paused,
      )
    }

    const onEnded = () => {
      updateAbsolutePosition()
      reportHeartbeat()
      void playback.close('playback_ended')
    }
    const onPageHide = () => {
      void playback.close('page_hidden', true)
    }

    const attach = () => {
      if (disposed) return
      video = rootRef.current?.querySelector('video') || null
      if (!video) {
        discoverTimer = window.setTimeout(attach, 50)
        return
      }
      video.addEventListener('timeupdate', updateAbsolutePosition)
      video.addEventListener('loadedmetadata', updateAbsolutePosition)
      video.addEventListener('ended', onEnded)
      window.addEventListener('pagehide', onPageHide)
      updateAbsolutePosition()
      reportHeartbeat()
      heartbeatTimer = window.setInterval(
        reportHeartbeat,
        Math.max(5, playback.heartbeatIntervalSec) * 1000,
      )
    }

    attach()
    return () => {
      disposed = true
      clearTimeout(discoverTimer)
      clearInterval(heartbeatTimer)
      video?.removeEventListener('timeupdate', updateAbsolutePosition)
      video?.removeEventListener('loadedmetadata', updateAbsolutePosition)
      video?.removeEventListener('ended', onEnded)
      window.removeEventListener('pagehide', onPageHide)
    }
  }, [
    playback.source,
    playback.sessionId,
    playback.generationId,
    playback.offsetSeconds,
    playback.heartbeatIntervalSec,
    playback.heartbeat,
    playback.close,
  ])

  const requestSeek = useCallback((targetSeconds: number, reason: string) => {
    if (!playback.sessionId || playback.loading) return
    const upperBound = knownDuration && knownDuration > 0 ? knownDuration : Number.MAX_SAFE_INTEGER
    const target = Math.max(0, Math.min(upperBound, targetSeconds))
    absolutePositionRef.current = target
    usePlayerStore.getState().setCurrentTime(target)
    void playback.restart(target, reason)
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
      const duration = knownDuration && knownDuration > 0
        ? knownDuration
        : usePlayerStore.getState().duration
      if (duration <= 0) return
      event.preventDefault()
      event.stopPropagation()
      requestSeek(ratio * duration, 'progress_seek')
      return
    }

    const button = target.closest<HTMLButtonElement>('button')
    if (!button || !rootRef.current?.contains(button)) return
    if (button.querySelector('[class*="lucide-skip-forward"]')) {
      event.preventDefault()
      event.stopPropagation()
      requestSeek(absolutePositionRef.current + 10, 'skip_forward')
      return
    }
    if (button.querySelector('[class*="lucide-skip-back"]')) {
      const parent = button.parentElement
      const isTitleBack = Boolean(
        parent?.classList.contains('absolute') &&
        parent.classList.contains('top-4') &&
        parent.classList.contains('left-4'),
      )
      if (!isTitleBack) {
        event.preventDefault()
        event.stopPropagation()
        requestSeek(absolutePositionRef.current - 10, 'skip_backward')
      }
    }
  }

  const handleBack = () => {
    void playback.close('navigate_back', true)
    onBack?.()
  }
  const handleNext = () => {
    void playback.close('next_media', true)
    onNext?.()
  }
  const handlePreprocessReady = () => {
    void playback.close('switch_to_preprocessed', true)
    onPreprocessReady?.()
  }

  if (playback.error) {
    return (
      <div className="flex h-full w-full flex-col items-center justify-center gap-3 bg-black px-6 text-center text-white">
        <AlertTriangle className="text-red-400" size={32} />
        <p className="text-base font-medium">转码播放启动失败</p>
        <p className="max-w-xl text-sm text-surface-400">{playback.error}</p>
        {onBack && (
          <button
            type="button"
            onClick={handleBack}
            className="mt-2 rounded-lg bg-white/10 px-4 py-2 text-sm transition hover:bg-white/20"
          >
            返回
          </button>
        )}
      </div>
    )
  }

  if (playback.loading || !playback.source) {
    return (
      <div className="flex h-full w-full flex-col items-center justify-center gap-3 bg-black text-white">
        <Loader2 className="animate-spin text-neon-blue" size={32} />
        <p className="text-sm text-surface-400">正在生成首个播放分片...</p>
      </div>
    )
  }

  return (
    <div
      ref={rootRef}
      className="h-full w-full"
      onClickCapture={handlePlayerClickCapture}
    >
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
    </div>
  )
}
