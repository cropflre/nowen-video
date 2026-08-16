/**
 * WebCodecsPlayerShell —— 基于 WebCodecsPlayer 的完整播放 UI
 *
 * 和 VideoPlayer.tsx 并行的简化播放器：
 *   - 使用 WebCodecsPlayer 核心做解码/渲染
 *   - 提供必要的控制条（播放/暂停/进度/音量/全屏/倍速）
 *   - 观看历史上报与 VideoPlayer 行为对齐
 *
 * 不支持的功能（相较 VideoPlayer）：
 *   - 字幕（内嵌/外挂/AI 字幕/翻译）—— WebCodecs 不解码字幕流，需未来单独渲染
 *   - 多音轨切换 —— 首版仅使用第一条音轨
 *   - 雪碧图预览 —— 需要预处理，未接入
 *   - 投屏 —— WebCodecs 的 canvas 不支持 remote playback
 *
 * 这些场景会在 PlayerPage 的决策中避免路由到 WebCodecs：
 *   - 已预处理 → 走 HLS（有字幕/雪碧图）
 *   - 原生可播 → 走 direct（有完整 <video> 生态）
 *   - 其余只在 "容器不兼容 + 编码兼容 + 浏览器 WebCodecs 支持" 时启用
 */

import { useCallback, useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import { Play, Pause, Volume2, VolumeX, Maximize, Minimize, SkipBack, SkipForward, Gauge, Cpu } from 'lucide-react'
import WebCodecsPlayer, { type WebCodecsPlayerHandle } from './WebCodecsPlayer'
import { usePlayerStore } from '@/stores/player'
import { userApi } from '@/api'
import { Tag } from '@/components/design-system'

interface WebCodecsPlayerShellProps {
  src: string
  mediaId: string
  title?: string
  startPosition?: number
  knownDuration?: number
  onBack?: () => void
  onNext?: () => void
  nextTitle?: string
  /** WebCodecs 播放失败时触发降级 */
  onFallback?: () => void
}

function formatTime(seconds: number): string {
  if (!isFinite(seconds) || seconds < 0) return '00:00'
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
}

const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 2]
const CONTROL_CLASS = 'flex h-9 min-w-9 items-center justify-center rounded-[var(--nv-player-radius-control)] text-[var(--nv-player-text-secondary)] transition-[background-color,color,transform] hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)] active:scale-[0.98]'

export default function WebCodecsPlayerShell({
  src,
  mediaId,
  title,
  startPosition,
  knownDuration,
  onBack,
  onNext,
  nextTitle,
  onFallback,
}: WebCodecsPlayerShellProps) {
  const playerRef = useRef<WebCodecsPlayerHandle>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const controlsTimerRef = useRef<number>(0)

  const [isPlaying, setIsPlaying] = useState(false)
  const [currentTime, setCurrentTime] = useState(0)
  const [duration, setDuration] = useState(knownDuration || 0)
  const [volume, setVolume] = useState(1)
  const [muted, setMuted] = useState(false)
  const [isFullscreen, setIsFullscreen] = useState(false)
  const [showControls, setShowControls] = useState(true)
  const [playbackRate, setPlaybackRate] = useState(1)
  const [showSpeedMenu, setShowSpeedMenu] = useState(false)
  const [nextCountdown, setNextCountdown] = useState<number | null>(null)

  const setStoreTime = usePlayerStore(s => s.setCurrentTime)
  const setStoreDuration = usePlayerStore(s => s.setDuration)

  const displayDuration = (knownDuration && knownDuration > duration) ? knownDuration : duration
  const progress = displayDuration > 0 ? (currentTime / displayDuration) * 100 : 0

  const handleTimeUpdate = useCallback((t: number) => {
    setCurrentTime(t)
    setStoreTime(t)
  }, [setStoreTime])

  const handleDurationChange = useCallback((d: number) => {
    setDuration(d)
    setStoreDuration(d)
  }, [setStoreDuration])

  const handlePlay = useCallback(() => setIsPlaying(true), [])
  const handlePause = useCallback(() => setIsPlaying(false), [])
  const handleEnded = useCallback(() => {
    setIsPlaying(false)
    if (onNext) setNextCountdown(5)
  }, [onNext])

  const handleError = useCallback((msg: string) => {
    console.warn('[WebCodecs] 播放失败:', msg)
    onFallback?.()
  }, [onFallback])

  const togglePlay = useCallback(() => {
    const p = playerRef.current
    if (!p) return
    if (nextCountdown !== null) setNextCountdown(null)
    if (isPlaying) p.pause()
    else p.play().catch(() => {})
  }, [isPlaying, nextCountdown])

  const seek = useCallback((seconds: number) => {
    const p = playerRef.current
    if (!p) return
    const target = Math.max(0, Math.min(displayDuration || p.getDuration(), currentTime + seconds))
    p.seek(target)
  }, [currentTime, displayDuration])

  const handleProgressClick = useCallback((e: React.MouseEvent<HTMLDivElement>) => {
    const p = playerRef.current
    if (!p) return
    const rect = e.currentTarget.getBoundingClientRect()
    const pos = (e.clientX - rect.left) / rect.width
    p.seek(pos * (displayDuration || p.getDuration()))
  }, [displayDuration])

  const handleVolumeChange = useCallback((v: number) => {
    setVolume(v)
    playerRef.current?.setVolume(v)
    if (v > 0 && muted) {
      setMuted(false)
      playerRef.current?.setMuted(false)
    }
  }, [muted])

  const toggleMute = useCallback(() => {
    const next = !muted
    setMuted(next)
    playerRef.current?.setMuted(next)
  }, [muted])

  const toggleFullscreen = useCallback(() => {
    if (!containerRef.current) return
    if (!document.fullscreenElement) containerRef.current.requestFullscreen?.()
    else document.exitFullscreen?.()
  }, [])

  const changePlaybackRate = useCallback((r: number) => {
    setPlaybackRate(r)
    playerRef.current?.setPlaybackRate(r)
    setShowSpeedMenu(false)
  }, [])

  useEffect(() => {
    const onFs = () => setIsFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFs)
    return () => document.removeEventListener('fullscreenchange', onFs)
  }, [])

  useEffect(() => {
    playerRef.current?.setVolume(volume)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const resetControlsTimer = useCallback(() => {
    setShowControls(true)
    clearTimeout(controlsTimerRef.current)
    controlsTimerRef.current = window.setTimeout(() => {
      if (isPlaying) setShowControls(false)
    }, 3000)
  }, [isPlaying])

  useEffect(() => {
    let tick = 0
    const timer = window.setInterval(() => {
      if (!isPlaying || currentTime <= 0) return
      tick++
      if (tick % 5 === 0) {
        const dur = displayDuration > 0 ? displayDuration : duration
        userApi.updateProgress(mediaId, currentTime, dur).catch(() => {})
      }
    }, 3000)
    return () => clearInterval(timer)
  }, [mediaId, isPlaying, currentTime, displayDuration, duration])

  useEffect(() => {
    if (nextCountdown === null) return
    if (nextCountdown <= 0) {
      setNextCountdown(null)
      onNext?.()
      return
    }
    const timer = window.setTimeout(() => {
      setNextCountdown(prev => (prev !== null ? prev - 1 : null))
    }, 1000)
    return () => clearTimeout(timer)
  }, [nextCountdown, onNext])

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.target instanceof HTMLInputElement) return
      switch (e.code) {
        case 'Space':
          e.preventDefault()
          togglePlay()
          break
        case 'ArrowLeft':
          e.preventDefault()
          seek(-10)
          break
        case 'ArrowRight':
          e.preventDefault()
          seek(10)
          break
        case 'ArrowUp':
          e.preventDefault()
          handleVolumeChange(Math.min(1, volume + 0.1))
          break
        case 'ArrowDown':
          e.preventDefault()
          handleVolumeChange(Math.max(0, volume - 0.1))
          break
        case 'KeyM':
          e.preventDefault()
          toggleMute()
          break
        case 'KeyF':
          e.preventDefault()
          toggleFullscreen()
          break
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [togglePlay, seek, handleVolumeChange, toggleMute, toggleFullscreen, volume])

  return (
    <div
      ref={containerRef}
      className="group/player relative h-full w-full bg-[var(--nv-player-canvas)]"
      onMouseMove={resetControlsTimer}
      onMouseLeave={() => { if (isPlaying) setShowControls(false) }}
    >
      <div className="absolute inset-0 cursor-pointer" onClick={togglePlay} onDoubleClick={toggleFullscreen}>
        <WebCodecsPlayer
          ref={playerRef}
          src={src}
          startPosition={startPosition}
          onTimeUpdate={handleTimeUpdate}
          onDurationChange={handleDurationChange}
          onPlay={handlePlay}
          onPause={handlePause}
          onEnded={handleEnded}
          onError={handleError}
          className="h-full w-full"
        />
      </div>

      {nextCountdown !== null && nextTitle && (
        <div className="absolute bottom-24 right-4 z-40 w-[min(22rem,calc(100vw-2rem))] rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] p-4 shadow-[var(--nv-player-shadow)] backdrop-blur-xl">
          <p className="text-xs text-[var(--nv-player-text-tertiary)]">即将播放</p>
          <p className="mt-1 truncate text-sm font-medium text-[var(--nv-player-text-primary)]">{nextTitle}</p>
          <div className="mt-3 flex items-center gap-2">
            <button
              type="button"
              onClick={() => { setNextCountdown(null); onNext?.() }}
              className="rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent)] px-3 py-2 text-xs font-semibold text-[var(--nv-text-on-brand)] transition-[background-color,transform] hover:bg-[var(--nv-action-primary-hover)] active:scale-[0.98]"
            >
              立即播放 ({nextCountdown})
            </button>
            <button
              type="button"
              onClick={() => setNextCountdown(null)}
              className="rounded-[var(--nv-player-radius-control)] px-3 py-2 text-xs text-[var(--nv-player-text-secondary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]"
            >
              取消
            </button>
          </div>
        </div>
      )}

      <div className={clsx(
        'absolute inset-0 transition-opacity duration-300',
        showControls ? 'opacity-100' : 'pointer-events-none opacity-0',
      )}>
        {title && (
          <div className="absolute left-4 top-4 z-30 flex max-w-[calc(100%-2rem)] items-center gap-3">
            {onBack && (
              <button
                type="button"
                onClick={onBack}
                className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] text-[var(--nv-player-text-primary)] shadow-[var(--nv-shadow-card)] backdrop-blur-md transition-[background-color,border-color,transform] hover:border-[var(--nv-player-border-hover)] hover:bg-[var(--nv-player-surface-hover)] active:scale-[0.98]"
                aria-label="返回"
              >
                <SkipBack size={18} aria-hidden="true" />
              </button>
            )}
            <h2 className="min-w-0 truncate font-display text-base font-medium tracking-tight text-[var(--nv-player-text-primary)] drop-shadow-lg">
              {title}
            </h2>
            <Tag tone="brand" className="shrink-0 text-[10px]">
              <Cpu size={10} aria-hidden="true" /> WebCodecs 硬解
            </Tag>
            {playbackRate !== 1 && (
              <Tag tone="quality" className="shrink-0 text-[10px]">{playbackRate}x</Tag>
            )}
          </div>
        )}

        <div
          className="absolute inset-x-0 bottom-0 z-30 px-4 pb-4 pt-16 sm:px-6"
          style={{ background: 'linear-gradient(to top, color-mix(in srgb, var(--nv-player-canvas) 92%, transparent), transparent)' }}
        >
          <div
            className="group/progress relative mb-3 h-1.5 cursor-pointer overflow-visible rounded-full bg-[color-mix(in_srgb,var(--nv-player-text-primary)_16%,transparent)]"
            onClick={handleProgressClick}
            role="slider"
            aria-label="播放进度"
            aria-valuemin={0}
            aria-valuemax={Math.max(displayDuration, 0)}
            aria-valuenow={Math.max(currentTime, 0)}
          >
            <div
              className="absolute left-0 top-0 h-full rounded-full bg-[var(--nv-player-accent)] transition-[width] duration-150"
              style={{ width: `${progress}%` }}
            />
            <div
              className="absolute top-1/2 h-3 w-3 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-[var(--nv-player-text-primary)] bg-[var(--nv-player-accent)] opacity-0 shadow-[0_2px_8px_rgba(0,0,0,.45)] transition-opacity group-hover/progress:opacity-100"
              style={{ left: `${progress}%` }}
            />
          </div>

          <div className="flex items-center gap-1 text-[var(--nv-player-text-primary)] sm:gap-2">
            <button type="button" onClick={togglePlay} className={CONTROL_CLASS} aria-label={isPlaying ? '暂停' : '播放'}>
              {isPlaying ? <Pause size={22} aria-hidden="true" /> : <Play size={22} aria-hidden="true" />}
            </button>
            <button type="button" onClick={() => seek(-10)} className={CONTROL_CLASS} aria-label="后退 10 秒">
              <SkipBack size={18} aria-hidden="true" />
            </button>
            <button type="button" onClick={() => seek(10)} className={CONTROL_CLASS} aria-label="前进 10 秒">
              <SkipForward size={18} aria-hidden="true" />
            </button>

            <span className="ml-1 whitespace-nowrap font-mono text-xs text-[var(--nv-player-text-tertiary)]">
              {formatTime(currentTime)} <span className="text-[var(--nv-player-text-faint)]">/</span> {formatTime(displayDuration)}
            </span>

            <div className="flex-1" />

            <div className="relative">
              <button
                type="button"
                onClick={() => setShowSpeedMenu(v => !v)}
                className={clsx(CONTROL_CLASS, 'gap-1 px-2', playbackRate !== 1 && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')}
                title="播放速度"
                aria-expanded={showSpeedMenu}
              >
                <Gauge size={18} aria-hidden="true" />
                <span className="text-xs font-semibold tabular-nums">{playbackRate}x</span>
              </button>
              {showSpeedMenu && (
                <div className="absolute bottom-full right-0 mb-2 grid min-w-[176px] grid-cols-2 gap-1.5 rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] p-2 shadow-[var(--nv-player-shadow)] backdrop-blur-xl" role="menu">
                  {SPEED_OPTIONS.map(rate => (
                    <button
                      key={rate}
                      type="button"
                      onClick={() => changePlaybackRate(rate)}
                      className={clsx(
                        'rounded-[var(--nv-player-radius-control)] border px-3 py-2 text-left text-xs font-medium tabular-nums transition-[background-color,border-color,color]',
                        playbackRate === rate
                          ? 'border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]'
                          : 'border-[var(--nv-player-border-subtle)] bg-[var(--nv-player-surface-subtle)] text-[var(--nv-player-text-secondary)] hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]',
                      )}
                      role="menuitemradio"
                      aria-checked={playbackRate === rate}
                    >
                      {rate}x
                    </button>
                  ))}
                </div>
              )}
            </div>

            <div className="hidden items-center gap-2 sm:flex">
              <button type="button" onClick={toggleMute} className={CONTROL_CLASS} aria-label={muted ? '取消静音' : '静音'}>
                {muted || volume === 0 ? <VolumeX size={18} aria-hidden="true" /> : <Volume2 size={18} aria-hidden="true" />}
              </button>
              <input
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={muted ? 0 : volume}
                onChange={(e) => handleVolumeChange(Number(e.target.value))}
                className="player-volume-slider w-20 cursor-pointer appearance-none"
                style={{
                  background: `linear-gradient(to right, var(--nv-player-accent) ${(muted ? 0 : volume) * 100}%, color-mix(in srgb, var(--nv-player-text-primary) 16%, transparent) ${(muted ? 0 : volume) * 100}%)`,
                }}
                aria-label="音量"
              />
            </div>

            <button type="button" onClick={toggleFullscreen} className={CONTROL_CLASS} aria-label={isFullscreen ? '退出全屏' : '进入全屏'}>
              {isFullscreen ? <Minimize size={18} aria-hidden="true" /> : <Maximize size={18} aria-hidden="true" />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
