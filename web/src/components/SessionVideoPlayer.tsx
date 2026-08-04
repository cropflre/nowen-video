import { useEffect, useRef } from 'react'
import { AlertTriangle, Loader2 } from 'lucide-react'
import VideoPlayer from './VideoPlayer'
import { usePlaybackSessionSource } from '@/hooks/usePlaybackSessionSource'

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
  const playback = usePlaybackSessionSource({
    enabled: true,
    mediaId,
    fallbackSource: fallbackSrc,
    startPosition,
  })

  useEffect(() => {
    if (!playback.source || !playback.sessionId) return

    let disposed = false
    let video: HTMLVideoElement | null = null
    let heartbeatTimer = 0
    let discoverTimer = 0

    const reportHeartbeat = () => {
      if (!video) return
      const relativePosition = Number.isFinite(video.currentTime) ? video.currentTime : 0
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
      video.addEventListener('ended', onEnded)
      window.addEventListener('pagehide', onPageHide)
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
    <div ref={rootRef} className="h-full w-full">
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
