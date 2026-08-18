import { forwardRef, useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from 'react'
import {
  AudioLines,
  Check,
  ChevronUp,
  ListVideo,
  Loader2,
  Maximize,
  Pause,
  Play,
  Subtitles,
  Volume2,
  VolumeX,
} from 'lucide-react'
import {
  desktop,
  type PlayOptions,
  type PlayerMediaInfo,
  type PlayerTrack,
  type PlayerVideoInfo,
} from './bridge'
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

type PickerKind = 'audio' | 'subtitle' | 'chapter'

const EMPTY_MEDIA_INFO: PlayerMediaInfo = {
  tracks: [],
  chapters: [],
  current_chapter: -1,
}

const CONTROL_CLASS = 'flex h-9 w-9 items-center justify-center rounded-full border border-transparent bg-[var(--nv-player-surface-subtle)] text-[var(--nv-player-text-primary)] transition-[background-color,border-color,color,transform] hover:bg-[var(--nv-player-surface-hover)] active:scale-[0.98]'
const PICKER_PANEL_CLASS = 'absolute bottom-[4.75rem] right-4 z-20 max-h-[min(58vh,30rem)] w-[min(23rem,calc(100vw-2rem))] overflow-hidden rounded-[var(--nv-radius-lg)] border border-[var(--nv-player-border)] bg-[color-mix(in_srgb,var(--nv-player-canvas)_92%,transparent)] shadow-[var(--nv-shadow-elevated)] backdrop-blur-xl'

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
  const mediaInfoTimer = useRef<number | null>(null)
  const [ready, setReady] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [paused, setPaused] = useState(false)
  const [muted, setMuted] = useState(false)
  const [volume, setVolume] = useState(initialVolume)
  const [controlsVisible, setControlsVisible] = useState(true)
  const [videoInfo, setVideoInfo] = useState<PlayerVideoInfo | null>(null)
  const [mediaInfo, setMediaInfo] = useState<PlayerMediaInfo>(EMPTY_MEDIA_INFO)
  const [picker, setPicker] = useState<PickerKind | null>(null)
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

  const applyVideoInfo = useCallback((info: PlayerVideoInfo, event = 'state') => {
    setVideoInfo(info)
    setPaused(info.paused)
    setMuted(info.mute)
    setVolume(info.volume)

    const store = usePlayerStore.getState()
    store.setCurrentTime(info.position)
    store.setDuration(info.duration)
    store.setPlaying(event === 'end-file' ? false : !info.paused)
    store.setMuted(info.mute)
    store.setVolume(Math.max(0, Math.min(1, info.volume / 100)))
  }, [])

  const refreshMediaInfo = useCallback(async () => {
    const info = await desktop.playerMediaInfo(sessionId)
    if (info) setMediaInfo(info)
  }, [sessionId])

  const scheduleMediaInfoRefresh = useCallback(() => {
    if (mediaInfoTimer.current) window.clearTimeout(mediaInfoTimer.current)
    mediaInfoTimer.current = window.setTimeout(() => {
      void refreshMediaInfo()
    }, 60)
  }, [refreshMediaInfo])

  const audioTracks = useMemo(
    () => mediaInfo.tracks.filter((track) => track.kind === 'audio'),
    [mediaInfo.tracks],
  )
  const subtitleTracks = useMemo(
    () => mediaInfo.tracks.filter((track) => track.kind === 'sub'),
    [mediaInfo.tracks],
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
      scheduleMediaInfoRefresh()
    },
    async setAudioTrack(aid) {
      await setProperty('aid', String(aid))
      scheduleMediaInfoRefresh()
    },
    async loadFile(url) {
      await command('loadfile', [url, 'replace'])
    },
  }), [command, scheduleMediaInfoRefresh, setProperty])

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

  // Player Core 事件驱动状态同步：启动时读取一次缓存快照，之后只消费 player-state。
  useEffect(() => {
    if (!ready || !desktop.isDesktop) return
    let active = true
    let unlisten: (() => void) | undefined

    ;(async () => {
      unlisten = await desktop.onPlayerState((event) => {
        if (!active || event.session_id !== sessionId) return
        applyVideoInfo(event.state, event.event)
        if (
          event.event === 'file-loaded'
          || event.event === 'media-info-change'
          || event.event === 'queue-overflow'
        ) {
          scheduleMediaInfoRefresh()
        }
      })

      if (!active) {
        unlisten()
        return
      }

      // 监听建立后读取一次快照，补齐订阅建立前可能已发生的初始属性/轨道事件。
      const [bootstrap, navigation] = await Promise.all([
        desktop.playerVideoInfo(sessionId),
        desktop.playerMediaInfo(sessionId),
      ])
      if (!active) return
      if (bootstrap) applyVideoInfo(bootstrap, 'bootstrap')
      if (navigation) setMediaInfo(navigation)
    })().catch((cause) => {
      console.warn('[desktop] Player Core 状态订阅失败:', cause)
    })

    return () => {
      active = false
      unlisten?.()
    }
  }, [applyVideoInfo, ready, scheduleMediaInfoRefresh, sessionId])

  useEffect(() => () => {
    if (mediaInfoTimer.current) window.clearTimeout(mediaInfoTimer.current)
  }, [])

  const resetHideTimer = useCallback(() => {
    setControlsVisible(true)
    if (hideTimer.current) clearTimeout(hideTimer.current)
    hideTimer.current = window.setTimeout(() => {
      setControlsVisible(false)
      setPicker(null)
    }, 3500)
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

  const selectTrack = useCallback(async (kind: 'audio' | 'subtitle', trackId: number | 'no') => {
    await setProperty(kind === 'audio' ? 'aid' : 'sid', String(trackId))
    scheduleMediaInfoRefresh()
    setPicker(null)
    resetHideTimer()
  }, [resetHideTimer, scheduleMediaInfoRefresh, setProperty])

  const selectChapter = useCallback(async (chapterIndex: number) => {
    await setProperty('chapter', String(chapterIndex))
    scheduleMediaInfoRefresh()
    setPicker(null)
    resetHideTimer()
  }, [resetHideTimer, scheduleMediaInfoRefresh, setProperty])

  const togglePicker = useCallback((kind: PickerKind) => {
    setPicker((current) => current === kind ? null : kind)
    resetHideTimer()
  }, [resetHideTimer])

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

      {ready && picker && (
        <PlayerPicker
          kind={picker}
          audioTracks={audioTracks}
          subtitleTracks={subtitleTracks}
          mediaInfo={mediaInfo}
          onAudio={(id) => void selectTrack('audio', id)}
          onSubtitle={(id) => void selectTrack('subtitle', id)}
          onChapter={(index) => void selectChapter(index)}
        />
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

            {audioTracks.length > 0 && (
              <button
                type="button"
                onClick={() => togglePicker('audio')}
                className={`${CONTROL_CLASS} ${picker === 'audio' ? 'border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]' : ''}`}
                title="音轨"
                aria-label="选择音轨"
                aria-expanded={picker === 'audio'}
              >
                <AudioLines className="h-4 w-4" aria-hidden="true" />
              </button>
            )}

            <button
              type="button"
              onClick={() => togglePicker('subtitle')}
              className={`${CONTROL_CLASS} ${picker === 'subtitle' ? 'border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]' : ''}`}
              title="字幕"
              aria-label="选择字幕"
              aria-expanded={picker === 'subtitle'}
            >
              <Subtitles className="h-4 w-4" aria-hidden="true" />
            </button>

            {mediaInfo.chapters.length > 0 && (
              <button
                type="button"
                onClick={() => togglePicker('chapter')}
                className={`${CONTROL_CLASS} ${picker === 'chapter' ? 'border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]' : ''}`}
                title="章节"
                aria-label="选择章节"
                aria-expanded={picker === 'chapter'}
              >
                <ListVideo className="h-4 w-4" aria-hidden="true" />
              </button>
            )}

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

function PlayerPicker({
  kind,
  audioTracks,
  subtitleTracks,
  mediaInfo,
  onAudio,
  onSubtitle,
  onChapter,
}: {
  kind: PickerKind
  audioTracks: PlayerTrack[]
  subtitleTracks: PlayerTrack[]
  mediaInfo: PlayerMediaInfo
  onAudio: (id: number) => void
  onSubtitle: (id: number | 'no') => void
  onChapter: (index: number) => void
}) {
  const title = kind === 'audio' ? '音轨' : kind === 'subtitle' ? '字幕' : '章节'

  return (
    <div className={PICKER_PANEL_CLASS} role="dialog" aria-label={`选择${title}`} onDoubleClick={(event) => event.stopPropagation()}>
      <div className="flex items-center justify-between border-b border-[var(--nv-player-border)] px-4 py-3">
        <div className="flex items-center gap-2 text-sm font-medium text-[var(--nv-player-text-primary)]">
          {kind === 'audio' ? <AudioLines className="h-4 w-4" aria-hidden="true" /> : kind === 'subtitle' ? <Subtitles className="h-4 w-4" aria-hidden="true" /> : <ListVideo className="h-4 w-4" aria-hidden="true" />}
          {title}
        </div>
        <ChevronUp className="h-4 w-4 text-[var(--nv-player-text-tertiary)]" aria-hidden="true" />
      </div>

      <div className="max-h-[min(50vh,25rem)] overflow-y-auto p-2">
        {kind === 'audio' && audioTracks.map((track, index) => (
          <TrackOption key={`audio-${track.id}-${index}`} track={track} index={index} fallback="音轨" onClick={() => onAudio(track.id)} />
        ))}

        {kind === 'subtitle' && (
          <>
            <PickerOption
              selected={!subtitleTracks.some((track) => track.selected)}
              title="关闭字幕"
              subtitle="不显示字幕"
              onClick={() => onSubtitle('no')}
            />
            {subtitleTracks.map((track, index) => (
              <TrackOption key={`sub-${track.id}-${index}`} track={track} index={index} fallback="字幕" onClick={() => onSubtitle(track.id)} />
            ))}
          </>
        )}

        {kind === 'chapter' && mediaInfo.chapters.map((chapter) => (
          <PickerOption
            key={`chapter-${chapter.index}`}
            selected={chapter.index === mediaInfo.current_chapter}
            title={chapter.title}
            subtitle={formatTime(chapter.time)}
            onClick={() => onChapter(chapter.index)}
          />
        ))}

        {kind === 'audio' && audioTracks.length === 0 && <EmptyPicker text="没有可用音轨" />}
        {kind === 'chapter' && mediaInfo.chapters.length === 0 && <EmptyPicker text="当前视频没有章节" />}
      </div>
    </div>
  )
}

function TrackOption({
  track,
  index,
  fallback,
  onClick,
}: {
  track: PlayerTrack
  index: number
  fallback: string
  onClick: () => void
}) {
  const details = [
    formatLanguage(track.language),
    track.codec_desc || track.codec?.toUpperCase(),
    track.external ? '外挂' : '',
    track.forced ? '强制' : '',
    track.is_default ? '默认' : '',
  ].filter(Boolean)

  return (
    <PickerOption
      selected={track.selected}
      title={track.title || `${fallback} ${index + 1}`}
      subtitle={details.join(' · ') || track.codec || undefined}
      onClick={onClick}
    />
  )
}

function PickerOption({
  selected,
  title,
  subtitle,
  onClick,
}: {
  selected: boolean
  title: string
  subtitle?: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`flex w-full items-center gap-3 rounded-[var(--nv-radius-control)] px-3 py-2.5 text-left transition-colors ${selected ? 'bg-[var(--nv-player-accent-soft)]' : 'hover:bg-[var(--nv-player-surface-hover)]'}`}
      aria-pressed={selected}
    >
      <span className={`grid h-5 w-5 shrink-0 place-items-center rounded-full border ${selected ? 'border-[var(--nv-player-accent-border)] text-[var(--nv-player-accent)]' : 'border-[var(--nv-player-border)] text-transparent'}`}>
        <Check className="h-3.5 w-3.5" aria-hidden="true" />
      </span>
      <span className="min-w-0 flex-1">
        <span className={`block truncate text-sm ${selected ? 'font-medium text-[var(--nv-player-text-primary)]' : 'text-[var(--nv-player-text-secondary)]'}`}>{title}</span>
        {subtitle && <span className="mt-0.5 block truncate text-xs text-[var(--nv-player-text-tertiary)]">{subtitle}</span>}
      </span>
    </button>
  )
}

function EmptyPicker({ text }: { text: string }) {
  return <div className="px-3 py-8 text-center text-sm text-[var(--nv-player-text-tertiary)]">{text}</div>
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

function formatLanguage(language: string): string {
  const normalized = language.trim().toLowerCase()
  if (!normalized) return ''
  const known: Record<string, string> = {
    chi: '中文',
    zho: '中文',
    cmn: '中文',
    eng: '英语',
    en: '英语',
    jpn: '日语',
    ja: '日语',
    kor: '韩语',
    ko: '韩语',
    yue: '粤语',
    fra: '法语',
    fre: '法语',
    deu: '德语',
    ger: '德语',
    spa: '西班牙语',
  }
  return known[normalized] || language.toUpperCase()
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
  setChapter(sessionId: string, chapter: number) {
    return desktop.playerSetProperty({ sessionId, name: 'chapter', value: String(chapter) })
  },
}
