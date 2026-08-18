import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { Pause, Play, Volume2, VolumeX, Maximize, Subtitles, Loader2 } from 'lucide-react'
import { desktop, type PlayOptions, type PlayerVideoInfo } from './bridge'
import { usePlayerStore } from '@/stores/player'

interface Props {
  streamUrl: string
  sessionId?: string
  title?: string
  playOptions?: PlayOptions
  initialVolume?: number
  autoDestroy?: boolean
  className?: string
  onReady?: () => void
  onError?: (error: string) => void
  onBack?: () => void
}

export interface DesktopPlayerHandle {
  play(): Promise<void>
  pause(): Promise<void>
  togglePause(): Promise<void>
  seek(seconds: number, absolute?: boolean): Promise<void>
  setVolume(value: number): Promise<void>
  setMute(muted: boolean): Promise<void>
  setSubtitle(sid: number | 'no'): Promise<void>
  setAudioTrack(aid: number | 'no'): Promise<void>
  loadFile(url: string): Promise<void>
}

const CONTROL_CLASS = 'flex h-9 w-9 items-center justify-center rounded-full border border-transparent bg-[var(--nv-player-surface-subtle)] text-[var(--nv-player-text-primary)] transition-[background-color,border-color,color,transform] hover:bg-[var(--nv-player-surface-hover)] active:scale-[0.98]'

function DesktopPlayerInner(
  {
    streamUrl,
    sessionId = 'main-player',
    title,
    playOptions,
    initialVolume = 80,
    autoDestroy = true,
    className = '',
    onReady,
    onError,
    onBack,
  }: Props,
  ref: React.Ref<DesktopPlayerHandle>,
) {
  const surfaceRef = useRef<HTMLDivElement>(null)
  const hideTimer = useRef<number | null>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [paused, setPaused] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(initialVolume)
  const [controlsVisible, setControlsVisible] = useState(true)
  const [videoInfo, setVideoInfo] = useState<PlayerVideoInfo | null>(null)
  const [seeking, setSeeking] = useState(false)
  const [seekPreview, setSeekPreview] = useState<number | null>(null)

  const command = useCallback(
    (name: string, args?: string[]) => desktop.playerCommand({ sessionId, command: name, args }),
    [sessionId],
  )
  const setProperty = useCallback(
    (name: string, value: string) => desktop.playerSetProperty({ sessionId, name, value }),
    [sessionId],
  )

  useImperativeHandle(ref, (): DesktopPlayerHandle => ({
    async play() {
      await setProperty('pause', 'no')
      setPaused(false)
    },
    async pause() {
      await setProperty('pause', 'yes')
      setPaused(true)
    },
    async togglePause() {
      await command('cycle', ['pause'])
      setPaused((value) => !value)
    },
    async seek(seconds, absolute = false) {
      await command('seek', [String(seconds), absolute ? 'absolute' : 'relative'])
    },
    async setVolume(value) {
      await setProperty('volume', String(value))
      setVolume(value)
    },
    async setMute(value) {
      await setProperty('mute', value ? 'yes' : 'no')
      setMuted(value)
    },
    async setSubtitle(sid) {
      await setProperty('sid', String(sid))
    },
    async setAudioTrack(aid) {
      await setProperty('aid', String(aid))
    },
    async loadFile(url) {
      await command('loadfile', [url, 'replace'])
    },
  }), [command, setProperty])

  useEffect(() => {
    if (!desktop.isDesktop || !streamUrl) return
    let canceled = false

    ;(async () => {
      const result = await desktop.playerStart({ sessionId, url: streamUrl, options: playOptions })
      if (canceled) return

      if (!result) {
        const message = '原生播放器启动失败，请检查 Desktop Player Core 运行资源。'
        setError(message)
        onError?.(message)
        return
      }

      await setProperty('volume', String(initialVolume))
      setReady(true)
      usePlayerStore.getState().setPlaying(true)
      onReady?.()
    })().catch((cause: unknown) => {
      if (canceled) return
      const message = cause instanceof Error ? cause.message : String(cause)
      setError(message)
      onError?.(message)
    })

    return () => {
      canceled = true
      usePlayerStore.getState().setPlaying(false)
      if (autoDestroy) {
        desktop.playerStop(sessionId).catch(() => {})
        desktop.playerDestroy().catch(() => {})
      }
    }
    // 播放会话只由媒体 URL / sessionId 决定；父级进度更新不得触发 Player Core 重启。
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [streamUrl, sessionId])

  useEffect(() => {
    if (!ready || !surfaceRef.current || !desktop.isDesktop) return
    const element = surfaceRef.current
    let rafId = 0

    const sync = () => {
      const rect = element.getBoundingClientRect()
      const dpr = window.devicePixelRatio || 1
      desktop.playerSyncSurface({
        x: Math.round(rect.left * dpr),
        y: Math.round(rect.top * dpr),
        width: Math.max(1, Math.round(rect.width * dpr)),
        height: Math.max(1, Math.round(rect.height * dpr)),
        visible: rect.width > 0 && rect.height > 0,
      }).catch(() => {})
    }

    const scheduleSync = () => {
      cancelAnimationFrame(rafId)
      rafId = requestAnimationFrame(sync)
    }

    const observer = new ResizeObserver(scheduleSync)
    observer.observe(element)
    window.addEventListener('resize', scheduleSync)
    window.addEventListener('scroll', scheduleSync, true)
    sync()

    return () => {
      cancelAnimationFrame(rafId)
      observer.disconnect()
      window.removeEventListener('resize', scheduleSync)
      window.removeEventListener('scroll', scheduleSync, true)
      desktop.playerSyncSurface({ x: 0, y: 0, width: 1, height: 1, visible: false }).catch(() => {})
    }
  }, [ready])

  // Render/事件泵接入前的单点兼容同步。业务状态只在这一处读取 Player Core 快照。
  useEffect(() => {
    if (!ready || !desktop.isDesktop) return
    let canceled = false

    const tick = async () => {
      const info = await desktop.playerVideoInfo(sessionId)
      if (!info || canceled) return

      setVideoInfo(info)
      setPaused(info.paused)
      setMuted(info.mute)
      setVolume(info.volume)

      const store = usePlayerStore.getState()
      store.setCurrentTime(info.position)
      store.setDuration(info.duration)
      store.setPlaying(!info.paused)
      store.setMuted(info.mute)
      store.setVolume(Math.max(0, Math.min(1, info.volume / 100)))
    }

    tick()
    const timer = window.setInterval(tick, 750)
    return () => {
      canceled = true
      window.clearInterval(timer)
    }
  }, [ready, sessionId])

  const resetHideTimer = useCallback(() => {
    setControlsVisible(true)
    if (hideTimer.current) clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => setControlsVisible(false), 3500)
  }, [])

  useEffect(() => {
    if (!ready) return
    resetHideTimer()
    return () => {
      if (hideTimer.current) clearTimeout(hideTimer.current)
    }
  }, [ready, resetHideTimer])

  const togglePause = useCallback(async () => {
    await command('cycle', ['pause'])
    setPaused((value) => !value)
    resetHideTimer()
  }, [command, resetHideTimer])

  const toggleMute = useCallback(async () => {
    const next = !muted
    await setProperty('mute', next ? 'yes' : 'no')
    setMuted(next)
    resetHideTimer()
  }, [muted, resetHideTimer, setProperty])

  const onVolumeChange = useCallback(async (value: number) => {
    setVolume(value)
    await setProperty('volume', String(value))
    if (value > 0 && muted) {
      await setProperty('mute', 'no')
      setMuted(false)
    }
    resetHideTimer()
  }, [muted, resetHideTimer, setProperty])

  const toggleFullscreen = useCallback(async () => {
    const fullscreen = await desktop.windowToggleFullscreen()
    usePlayerStore.getState().setFullscreen(fullscreen)
    resetHideTimer()
  }, [resetHideTimer])

  const seekAbsolute = useCallback(async (seconds: number) => {
    await command('seek', [String(seconds), 'absolute'])
    resetHideTimer()
  }, [command, resetHideTimer])

  if (!desktop.isDesktop) return null

  if (error) {
    return (
      <div className={`group/player flex flex-col items-center justify-center gap-3 bg-[var(--nv-player-canvas)] text-[var(--nv-player-danger)] ${className}`}>
        <span className="text-sm">播放器启动失败</span>
        <code className="max-w-[80%] rounded-[var(--nv-radius-sm)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-danger-soft)] px-3 py-1 text-center text-xs">{error}</code>
        {onBack && (
          <button type="button" onClick={onBack} className="mt-2 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-3 py-1.5 text-xs text-[var(--nv-player-text-secondary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]">
            返回
          </button>
        )}
      </div>
    )
  }

  const currentPosition = seeking && seekPreview !== null ? seekPreview : videoInfo?.position || 0
  const progressPercent = videoInfo && videoInfo.duration > 0 ? (currentPosition / videoInfo.duration) * 100 : 0

  return (
    <div
      className={`group/player relative overflow-hidden bg-[var(--nv-player-canvas)] ${className}`}
      onMouseMove={resetHideTimer}
      onDoubleClick={toggleFullscreen}
    >
      <div ref={surfaceRef} className="absolute inset-0" data-desktop-player-surface />

      {!ready && (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3 text-[var(--nv-player-text-tertiary)]">
          <Loader2 className="h-10 w-10 animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" />
          <span className="text-sm">正在启动原生播放器...</span>
        </div>
      )}

      {ready && (
        <div
          className={`absolute left-0 right-0 top-0 flex items-center gap-3 p-4 transition-opacity duration-300 ${controlsVisible ? 'opacity-100' : 'pointer-events-none opacity-0'}`}
          style={{ background: 'linear-gradient(to bottom, color-mix(in srgb, var(--nv-player-canvas) 68%, transparent), transparent)' }}
        >
          {onBack && (
            <button type="button" onClick={onBack} className={CONTROL_CLASS} title="返回" aria-label="返回">←</button>
          )}
          {title && <div className="min-w-0 flex-1 truncate text-sm font-medium text-[var(--nv-player-text-primary)] drop-shadow">{title}</div>}
          {videoInfo && videoInfo.width > 0 && (
            <div className="flex items-center gap-2">
              {videoInfo.hdr && videoInfo.hdr !== 'SDR' && (
                <span className="rounded-full border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] px-2 py-1 text-xs text-[var(--nv-player-accent)]">{videoInfo.hdr}</span>
              )}
              <span className="rounded-full border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-subtle)] px-2 py-1 font-mono text-xs text-[var(--nv-player-text-secondary)]">
                {videoInfo.height >= 2160 ? '4K' : videoInfo.height >= 1440 ? '2K' : videoInfo.height >= 1080 ? '1080p' : `${videoInfo.height}p`}
              </span>
            </div>
          )}
        </div>
      )}

      {ready && (
        <div
          className={`absolute bottom-0 left-0 right-0 p-4 transition-opacity duration-300 ${controlsVisible ? 'opacity-100' : 'pointer-events-none opacity-0'}`}
          style={{ background: 'linear-gradient(to top, color-mix(in srgb, var(--nv-player-canvas) 72%, transparent), transparent)' }}
        >
          {videoInfo && videoInfo.duration > 0 && (
            <div className="mb-3 flex select-none items-center gap-3 font-mono text-xs text-[var(--nv-player-text-secondary)]">
              <span className="w-14 text-right tabular-nums">{formatTime(currentPosition)}</span>
              <input
                type="range"
                min={0}
                max={Math.max(1, videoInfo.duration)}
                step={0.1}
                value={currentPosition}
                onChange={(event) => {
                  setSeeking(true)
                  setSeekPreview(Number(event.target.value))
                }}
                onMouseUp={async (event) => {
                  await seekAbsolute(Number((event.target as HTMLInputElement).value))
                  setSeeking(false)
                  setSeekPreview(null)
                }}
                onTouchEnd={async (event) => {
                  await seekAbsolute(Number((event.target as HTMLInputElement).value))
                  setSeeking(false)
                  setSeekPreview(null)
                }}
                className="player-volume-slider h-1.5 flex-1 cursor-pointer appearance-none"
                style={{ background: `linear-gradient(to right, var(--nv-player-accent) 0%, var(--nv-player-accent) ${progressPercent}%, color-mix(in srgb, var(--nv-player-text-primary) 15%, transparent) ${progressPercent}%, color-mix(in srgb, var(--nv-player-text-primary) 15%, transparent) 100%)` }}
                aria-label="播放进度"
              />
              <span className="w-14 tabular-nums text-[var(--nv-player-text-tertiary)]">{formatTime(videoInfo.duration)}</span>
            </div>
          )}

          <div className="flex items-center gap-3">
            <button type="button" onClick={togglePause} className={`${CONTROL_CLASS} h-10 w-10`} title={paused ? '播放' : '暂停'} aria-label={paused ? '播放' : '暂停'}>
              {paused ? <Play className="h-5 w-5" aria-hidden="true" /> : <Pause className="h-5 w-5" aria-hidden="true" />}
            </button>
            <button type="button" onClick={toggleMute} className={CONTROL_CLASS} title={muted ? '取消静音' : '静音'} aria-label={muted ? '取消静音' : '静音'}>
              {muted ? <VolumeX className="h-4 w-4" aria-hidden="true" /> : <Volume2 className="h-4 w-4" aria-hidden="true" />}
            </button>
            <input
              type="range"
              min={0}
              max={100}
              value={muted ? 0 : volume}
              onChange={(event) => onVolumeChange(Number(event.target.value))}
              className="player-volume-slider w-28 cursor-pointer appearance-none"
              style={{ background: `linear-gradient(to right, var(--nv-player-accent) ${muted ? 0 : volume}%, color-mix(in srgb, var(--nv-player-text-primary) 15%, transparent) ${muted ? 0 : volume}%)` }}
              aria-label="音量"
            />
            <button type="button" onClick={async () => { await command('cycle', ['sid']); resetHideTimer() }} className={CONTROL_CLASS} title="切换字幕" aria-label="切换字幕">
              <Subtitles className="h-4 w-4" aria-hidden="true" />
            </button>
            <div className="flex-1" />
            <button type="button" onClick={toggleFullscreen} className={CONTROL_CLASS} title="全屏" aria-label="全屏">
              <Maximize className="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

const DesktopPlayer = forwardRef<DesktopPlayerHandle, Props>(DesktopPlayerInner)
DesktopPlayer.displayName = 'DesktopPlayer'

export default DesktopPlayer

function formatTime(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return '00:00'
  const s = Math.floor(seconds % 60)
  const m = Math.floor((seconds / 60) % 60)
  const h = Math.floor(seconds / 3600)
  const pad = (value: number) => String(value).padStart(2, '0')
  return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
}

export const desktopPlayerControl = {
  togglePause(sessionId = 'main-player') {
    return desktop.playerCommand({ sessionId, command: 'cycle', args: ['pause'] })
  },
  seek(sessionId: string, seconds: number, absolute = false) {
    return desktop.playerCommand({ sessionId, command: 'seek', args: [String(seconds), absolute ? 'absolute' : 'relative'] })
  },
  setVolume(sessionId: string, value: number) {
    return desktop.playerSetProperty({ sessionId, name: 'volume', value: String(value) })
  },
  setMute(sessionId: string, mute: boolean) {
    return desktop.playerSetProperty({ sessionId, name: 'mute', value: mute ? 'yes' : 'no' })
  },
  setSubtitle(sessionId: string, sid: number | 'no') {
    return desktop.playerSetProperty({ sessionId, name: 'sid', value: String(sid) })
  },
  setAudioTrack(sessionId: string, aid: number | 'no') {
    return desktop.playerSetProperty({ sessionId, name: 'aid', value: String(aid) })
  },
}
