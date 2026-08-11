/**
 * MpvEmbedPlayer —— 嵌入式 libmpv 播放器。
 * Web 层只负责控制条；解码与渲染仍由原生 libmpv 子窗口完成。
 */

import { forwardRef, useCallback, useEffect, useImperativeHandle, useRef, useState } from 'react'
import { Pause, Play, Volume2, VolumeX, Maximize, Subtitles, Sparkles, Loader2, PictureInPicture2, Pin } from 'lucide-react'
import { desktop, PlayOptions, MpvVideoInfo } from './bridge'
import Anime4KPanel, { Anime4KLevel } from './Anime4KPanel'

interface Props {
  streamUrl: string
  sessionId?: string
  title?: string
  playOptions?: PlayOptions
  initialVolume?: number
  autoDestroy?: boolean
  className?: string
  onReady?: () => void
  onError?: (e: string) => void
  onBack?: () => void
}

export interface MpvEmbedHandle {
  play(): Promise<void>
  pause(): Promise<void>
  togglePause(): Promise<void>
  seek(seconds: number, absolute?: boolean): Promise<void>
  setVolume(v: number): Promise<void>
  setMute(m: boolean): Promise<void>
  setSubtitle(sid: number | 'no'): Promise<void>
  setAudioTrack(aid: number | 'no'): Promise<void>
  setAnime4K(level: Anime4KLevel): Promise<void>
  loadFile(url: string): Promise<void>
}

const MPV_CONTROL_CLASS = 'flex h-9 w-9 items-center justify-center rounded-full border border-transparent bg-[var(--nv-player-surface-subtle)] text-[var(--nv-player-text-primary)] transition-[background-color,border-color,color,transform] hover:bg-[var(--nv-player-surface-hover)] active:scale-[0.98]'
const MPV_ACTIVE_CLASS = 'border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]'

function MpvEmbedPlayerInner(
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
  ref: React.Ref<MpvEmbedHandle>,
) {
  const wrapperRef = useRef<HTMLDivElement>(null)
  const placeholderRef = useRef<HTMLDivElement>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [paused, setPaused] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(initialVolume)
  const [anime4kLevel, setAnime4kLevel] = useState<Anime4KLevel>('off')
  const [controlsVisible, setControlsVisible] = useState(true)
  const [showAnime4KPanel, setShowAnime4KPanel] = useState(false)
  const hideTimer = useRef<number | null>(null)
  const [videoInfo, setVideoInfo] = useState<MpvVideoInfo | null>(null)
  const [seeking, setSeeking] = useState(false)
  const [seekPreview, setSeekPreview] = useState<number | null>(null)
  const [pipActive, setPipActive] = useState(false)
  const [pinned, setPinned] = useState(false)

  const cmd = useCallback(
    (command: string, args?: string[]) => desktop.mpvEmbedCommand({ sessionId, command, args }),
    [sessionId],
  )
  const setProp = useCallback(
    (name: string, value: string) => desktop.mpvEmbedSetProperty({ sessionId, name, value }),
    [sessionId],
  )

  const applyAnime4K = useCallback(async (level: Anime4KLevel) => {
    const ok = await desktop.mpvEmbedSetAnime4K({ sessionId, level })
    if (ok) setAnime4kLevel(level)
    else console.warn('[mpv] 切换 Anime4K 档位失败:', level)
  }, [sessionId])

  useImperativeHandle(ref, (): MpvEmbedHandle => ({
    async play() { await setProp('pause', 'no'); setPaused(false) },
    async pause() { await setProp('pause', 'yes'); setPaused(true) },
    async togglePause() { await cmd('cycle', ['pause']); setPaused((value) => !value) },
    async seek(seconds, absolute = false) { await cmd('seek', [String(seconds), absolute ? 'absolute' : 'relative']) },
    async setVolume(value) { await setProp('volume', String(value)); setVolume(value) },
    async setMute(value) { await setProp('mute', value ? 'yes' : 'no'); setMuted(value) },
    async setSubtitle(sid) { await setProp('sid', String(sid)) },
    async setAudioTrack(aid) { await setProp('aid', String(aid)) },
    async setAnime4K(level) { await applyAnime4K(level) },
    async loadFile(url) { await cmd('loadfile', [url, 'replace']) },
  }), [cmd, setProp, applyAnime4K])

  useEffect(() => {
    if (!desktop.isDesktop || !streamUrl) return
    let canceled = false
    ;(async () => {
      try {
        const result = await desktop.mpvEmbedStart({ sessionId, url: streamUrl, options: playOptions })
        if (canceled) return
        if (result) {
          await setProp('volume', String(initialVolume))
          setReady(true)
          onReady?.()
        } else {
          const message = '无法启动嵌入式 mpv（请确认已启用 embed-mpv 且 libmpv-2.dll 存在）'
          setError(message)
          onError?.(message)
        }
      } catch (cause: unknown) {
        if (canceled) return
        const message = (cause as Error)?.message || String(cause)
        setError(message)
        onError?.(message)
      }
    })()
    return () => {
      canceled = true
      if (autoDestroy) desktop.mpvEmbedDestroy().catch(() => {})
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [streamUrl, sessionId])

  useEffect(() => {
    if (!ready || !placeholderRef.current || !desktop.isDesktop) return
    const element = placeholderRef.current
    let rafId = 0
    const sync = () => {
      const rect = element.getBoundingClientRect()
      const dpr = window.devicePixelRatio || 1
      desktop.mpvEmbedSync({
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
      desktop.mpvEmbedSync({ x: 0, y: 0, width: 1, height: 1, visible: false }).catch(() => {})
    }
  }, [ready])

  useEffect(() => {
    if (!ready || !desktop.isDesktop) return
    let canceled = false
    const tick = async () => {
      if (canceled) return
      try {
        const info = await desktop.mpvEmbedVideoInfo(sessionId)
        if (info && !canceled) {
          setVideoInfo(info)
          if (info.paused !== paused) setPaused(info.paused)
          if (info.mute !== muted) setMuted(info.mute)
        }
      } catch { /* ignore */ }
    }
    tick()
    const timer = window.setInterval(tick, 1000)
    return () => {
      canceled = true
      window.clearInterval(timer)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [ready, sessionId])

  const resetHideTimer = useCallback(() => {
    setControlsVisible(true)
    if (hideTimer.current) clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => {
      setControlsVisible(false)
      setShowAnime4KPanel(false)
    }, 3500)
  }, [])

  useEffect(() => {
    if (!ready) return
    resetHideTimer()
    return () => { if (hideTimer.current) clearTimeout(hideTimer.current) }
  }, [ready, resetHideTimer])

  const togglePause = useCallback(async () => {
    await cmd('cycle', ['pause'])
    setPaused((value) => !value)
    resetHideTimer()
  }, [cmd, resetHideTimer])

  const toggleMute = useCallback(async () => {
    const next = !muted
    await setProp('mute', next ? 'yes' : 'no')
    setMuted(next)
    resetHideTimer()
  }, [muted, setProp, resetHideTimer])

  const onVolumeChange = useCallback(async (value: number) => {
    setVolume(value)
    await setProp('volume', String(value))
    if (value > 0 && muted) {
      await setProp('mute', 'no')
      setMuted(false)
    }
    resetHideTimer()
  }, [setProp, muted, resetHideTimer])

  const toggleFullscreen = useCallback(async () => {
    await desktop.windowToggleFullscreen()
    resetHideTimer()
  }, [resetHideTimer])

  const seekAbsolute = useCallback(async (seconds: number) => {
    await cmd('seek', [String(seconds), 'absolute'])
    resetHideTimer()
  }, [cmd, resetHideTimer])

  const togglePip = useCallback(async () => {
    try {
      if (pipActive) {
        await desktop.windowPipExit()
        setPipActive(false)
      } else {
        await desktop.windowPipEnter()
        setPipActive(true)
      }
    } catch (cause) {
      console.warn('[mpv] PiP 切换失败:', cause)
    }
    resetHideTimer()
  }, [pipActive, resetHideTimer])

  const togglePin = useCallback(async () => {
    try {
      const next = !pinned
      await desktop.windowSetAlwaysOnTop(next)
      setPinned(next)
    } catch (cause) {
      console.warn('[mpv] 始终置顶切换失败:', cause)
    }
    resetHideTimer()
  }, [pinned, resetHideTimer])

  if (!desktop.isDesktop) {
    return (
      <div className={`group/player flex items-center justify-center bg-[var(--nv-player-canvas)] text-[var(--nv-player-text-tertiary)] ${className}`}>
        <span className="text-sm">嵌入式 mpv 仅在桌面端可用</span>
      </div>
    )
  }

  if (error) {
    return (
      <div className={`group/player flex flex-col items-center justify-center gap-3 bg-[var(--nv-player-canvas)] text-[var(--nv-player-danger)] ${className}`}>
        <span className="text-sm">mpv 启动失败</span>
        <code className="rounded-[var(--nv-radius-sm)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-danger-soft)] px-3 py-1 text-xs">{error}</code>
        {onBack && (
          <button type="button" onClick={onBack} className="mt-2 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-3 py-1.5 text-xs text-[var(--nv-player-text-secondary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]">返回</button>
        )}
      </div>
    )
  }

  const currentPosition = seeking && seekPreview !== null ? seekPreview : videoInfo?.position || 0
  const progressPercent = videoInfo && videoInfo.duration > 0 ? (currentPosition / videoInfo.duration) * 100 : 0

  return (
    <div
      ref={wrapperRef}
      className={`group/player relative overflow-hidden bg-[var(--nv-player-canvas)] ${className}`}
      onMouseMove={resetHideTimer}
      onDoubleClick={toggleFullscreen}
    >
      <div ref={placeholderRef} className="absolute inset-0" data-mpv-embed-placeholder />

      {!ready && (
        <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center gap-3 text-[var(--nv-player-text-tertiary)]">
          <Loader2 className="h-10 w-10 animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" />
          <span className="text-sm">正在加载 libmpv 内核...</span>
        </div>
      )}

      {ready && (
        <div
          className={`absolute left-0 right-0 top-0 flex items-center gap-3 p-4 transition-opacity duration-300 ${controlsVisible ? 'opacity-100' : 'pointer-events-none opacity-0'}`}
          style={{ background: 'linear-gradient(to bottom, color-mix(in srgb, var(--nv-player-canvas) 68%, transparent), transparent)' }}
        >
          {onBack && (
            <button type="button" onClick={onBack} className={MPV_CONTROL_CLASS} title="返回" aria-label="返回">←</button>
          )}
          {title && <div className="min-w-0 flex-1 truncate text-sm font-medium text-[var(--nv-player-text-primary)] drop-shadow">{title}</div>}
          <div className="flex items-center gap-1.5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-subtle)] px-2 py-1 text-xs text-[var(--nv-player-text-tertiary)] backdrop-blur">
            <Sparkles className="h-3.5 w-3.5 text-[var(--nv-player-accent)]" aria-hidden="true" />libmpv · gpu-next
          </div>

          {videoInfo && videoInfo.width > 0 && (
            <>
              {videoInfo.hdr !== 'SDR' && videoInfo.hdr !== '' && (
                <div className="rounded-[var(--nv-player-radius-control)] border px-2 py-1 text-xs font-semibold tracking-wider backdrop-blur" style={{ borderColor: 'color-mix(in srgb, var(--nv-player-warning) 38%, transparent)', background: 'color-mix(in srgb, var(--nv-player-warning) 12%, transparent)', color: 'var(--nv-player-warning)' }} title={`色域 ${videoInfo.primaries} · Gamma ${videoInfo.gamma}`}>
                  {videoInfo.hdr}
                </div>
              )}
              <div className="rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-subtle)] px-2 py-1 font-mono text-xs text-[var(--nv-player-text-secondary)] backdrop-blur" title={videoInfo.codec}>
                {videoInfo.height >= 2160 ? '4K' : videoInfo.height >= 1440 ? '2K' : videoInfo.height >= 1080 ? '1080p' : `${videoInfo.height}p`}
              </div>
            </>
          )}
        </div>
      )}

      {ready && (
        <div
          className={`absolute bottom-0 left-0 right-0 p-4 transition-opacity duration-300 ${controlsVisible ? 'opacity-100' : 'pointer-events-none opacity-0'}`}
          style={{ background: 'linear-gradient(to top, color-mix(in srgb, var(--nv-player-canvas) 72%, transparent), transparent)' }}
        >
          {showAnime4KPanel && <div className="mb-3 flex justify-end"><Anime4KPanel value={anime4kLevel} onChange={applyAnime4K} /></div>}

          {videoInfo && videoInfo.duration > 0 && (
            <div className="mb-3 flex select-none items-center gap-3 font-mono text-xs text-[var(--nv-player-text-secondary)]">
              <span className="w-14 text-right tabular-nums">{formatTime(currentPosition)}</span>
              <input
                type="range"
                min={0}
                max={Math.max(1, videoInfo.duration)}
                step={0.1}
                value={currentPosition}
                onChange={(event) => { setSeeking(true); setSeekPreview(Number(event.target.value)) }}
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
            <button type="button" onClick={togglePause} className={`${MPV_CONTROL_CLASS} h-10 w-10`} title={paused ? '播放' : '暂停'} aria-label={paused ? '播放' : '暂停'}>
              {paused ? <Play className="h-5 w-5" aria-hidden="true" /> : <Pause className="h-5 w-5" aria-hidden="true" />}
            </button>
            <button type="button" onClick={toggleMute} className={MPV_CONTROL_CLASS} title={muted ? '取消静音' : '静音'} aria-label={muted ? '取消静音' : '静音'}>
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
            <button type="button" onClick={async () => { await cmd('cycle', ['sid']); resetHideTimer() }} className={MPV_CONTROL_CLASS} title="切换字幕" aria-label="切换字幕">
              <Subtitles className="h-4 w-4" aria-hidden="true" />
            </button>

            <div className="flex-1" />

            <button
              type="button"
              onClick={() => setShowAnime4KPanel((value) => !value)}
              className={`flex h-9 items-center gap-1.5 rounded-full border px-3 text-xs font-medium transition-[background-color,border-color,color] ${anime4kLevel !== 'off' ? MPV_ACTIVE_CLASS : 'border-transparent bg-[var(--nv-player-surface-subtle)] text-[var(--nv-player-text-primary)] hover:bg-[var(--nv-player-surface-hover)]'}`}
              title="Anime4K 超分"
              aria-pressed={anime4kLevel !== 'off'}
            >
              <Sparkles className="h-4 w-4" aria-hidden="true" />
              {anime4kLevel === 'off' ? 'Anime4K' : `Anime4K · ${anime4kLevel.toUpperCase()}`}
            </button>
            <button type="button" onClick={togglePin} className={`${MPV_CONTROL_CLASS} ${pinned ? MPV_ACTIVE_CLASS : ''}`} title={pinned ? '取消置顶' : '始终置顶'} aria-pressed={pinned}>
              <Pin className="h-4 w-4" aria-hidden="true" />
            </button>
            <button type="button" onClick={togglePip} className={`${MPV_CONTROL_CLASS} ${pipActive ? MPV_ACTIVE_CLASS : ''}`} title={pipActive ? '退出画中画' : '画中画'} aria-pressed={pipActive}>
              <PictureInPicture2 className="h-4 w-4" aria-hidden="true" />
            </button>
            <button type="button" onClick={toggleFullscreen} className={MPV_CONTROL_CLASS} title="全屏" aria-label="全屏">
              <Maximize className="h-4 w-4" aria-hidden="true" />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

const MpvEmbedPlayer = forwardRef<MpvEmbedHandle, Props>(MpvEmbedPlayerInner)
MpvEmbedPlayer.displayName = 'MpvEmbedPlayer'

export default MpvEmbedPlayer

function formatTime(sec: number): string {
  if (!Number.isFinite(sec) || sec < 0) return '00:00'
  const s = Math.floor(sec % 60)
  const m = Math.floor((sec / 60) % 60)
  const h = Math.floor(sec / 3600)
  const pad = (n: number) => String(n).padStart(2, '0')
  return h > 0 ? `${pad(h)}:${pad(m)}:${pad(s)}` : `${pad(m)}:${pad(s)}`
}

export const mpvControl = {
  togglePause(sessionId = 'main-player') {
    return desktop.mpvEmbedCommand({ sessionId, command: 'cycle', args: ['pause'] })
  },
  seek(sessionId: string, seconds: number, absolute = false) {
    return desktop.mpvEmbedCommand({ sessionId, command: 'seek', args: [String(seconds), absolute ? 'absolute' : 'relative'] })
  },
  setVolume(sessionId: string, value: number) {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'volume', value: String(value) })
  },
  setMute(sessionId: string, mute: boolean) {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'mute', value: mute ? 'yes' : 'no' })
  },
  setSubtitle(sessionId: string, sid: number | 'no') {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'sid', value: String(sid) })
  },
  setAudioTrack(sessionId: string, aid: number | 'no') {
    return desktop.mpvEmbedSetProperty({ sessionId, name: 'aid', value: String(aid) })
  },
}
