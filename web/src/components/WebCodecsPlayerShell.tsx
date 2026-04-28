/**
 * WebCodecsPlayerShell —— 基于 WebCodecsPlayer 的完整播放 UI
 *
 * 和 VideoPlayer.tsx 并行的简化播放器：
 *   - 使用 WebCodecsPlayer 核心做解码/渲染
 *   - 提供必要的控制条（播放/暂停/进度/音量/全屏/倍速）
 *   - 进度上报与 VideoPlayer 行为对齐（reportPlayback / updateProgress）
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
import { streamApi } from '@/api/stream'
import { userApi } from '@/api'

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

  // ===== 播放器事件桥接 =====
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

  // ===== 控制函数 =====
  const togglePlay = useCallback(() => {
    const p = playerRef.current
    if (!p) return
    if (nextCountdown !== null) setNextCountdown(null)
    if (isPlaying) {
      p.pause()
    } else {
      p.play().catch(() => {})
    }
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
    const target = pos * (displayDuration || p.getDuration())
    p.seek(target)
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
    if (!document.fullscreenElement) {
      containerRef.current.requestFullscreen?.()
    } else {
      document.exitFullscreen?.()
    }
  }, [])

  const changePlaybackRate = useCallback((r: number) => {
    setPlaybackRate(r)
    playerRef.current?.setPlaybackRate(r)
    setShowSpeedMenu(false)
  }, [])

  // ===== 副作用 =====
  useEffect(() => {
    const onFs = () => setIsFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFs)
    return () => document.removeEventListener('fullscreenchange', onFs)
  }, [])

  // 初始音量
  useEffect(() => {
    playerRef.current?.setVolume(volume)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 自动隐藏控制栏
  const resetControlsTimer = useCallback(() => {
    setShowControls(true)
    clearTimeout(controlsTimerRef.current)
    controlsTimerRef.current = window.setTimeout(() => {
      if (isPlaying) setShowControls(false)
    }, 3000)
  }, [isPlaying])

  // 进度上报（3 秒一次，每 5 次写一次观看历史）
  useEffect(() => {
    let tick = 0
    const timer = window.setInterval(() => {
      if (!isPlaying || currentTime <= 0) return
      streamApi.reportPlayback(mediaId, currentTime).catch(() => {})
      tick++
      if (tick % 5 === 0) {
        const dur = displayDuration > 0 ? displayDuration : duration
        userApi.updateProgress(mediaId, currentTime, dur).catch(() => {})
      }
    }, 3000)
    return () => clearInterval(timer)
  }, [mediaId, isPlaying, currentTime, displayDuration, duration])

  // 下一集倒计时
  useEffect(() => {
    if (nextCountdown === null) return
    if (nextCountdown <= 0) {
      setNextCountdown(null)
      onNext?.()
      return
    }
    const t = window.setTimeout(() => {
      setNextCountdown(prev => (prev !== null ? prev - 1 : null))
    }, 1000)
    return () => clearTimeout(t)
  }, [nextCountdown, onNext])

  // 键盘快捷键
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
      className="group/player relative h-full w-full bg-black"
      onMouseMove={resetControlsTimer}
      onMouseLeave={() => { if (isPlaying) setShowControls(false) }}
    >
      {/* WebCodecs 核心播放器 */}
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

      {/* 下一集倒计时浮层 */}
      {nextCountdown !== null && nextTitle && (
        <div className="absolute right-4 bottom-24 z-40 rounded-xl p-4 backdrop-blur-md"
          style={{ background: 'rgba(11,17,32,0.85)', border: '1px solid var(--neon-blue-20)' }}>
          <p className="text-xs text-surface-400 mb-1">即将播放</p>
          <p className="text-sm text-white mb-2">{nextTitle}</p>
          <div className="flex items-center gap-2">
            <button
              onClick={() => { setNextCountdown(null); onNext?.() }}
              className="rounded-md px-3 py-1 text-xs text-white"
              style={{ background: 'var(--neon-blue-20)' }}
            >
              立即播放 ({nextCountdown})
            </button>
            <button
              onClick={() => setNextCountdown(null)}
              className="rounded-md px-3 py-1 text-xs text-surface-400 hover:text-white"
            >
              取消
            </button>
          </div>
        </div>
      )}

      {/* 控制栏 */}
      <div className={clsx(
        'absolute inset-0 transition-opacity duration-300',
        showControls ? 'opacity-100' : 'opacity-0 pointer-events-none'
      )}>
        {/* 顶部标题栏 */}
        {title && (
          <div className="absolute left-4 top-4 flex items-center gap-3 z-30">
            {onBack && (
              <button
                onClick={onBack}
                className="rounded-full p-2 text-white backdrop-blur-md transition-all hover:scale-105"
                style={{ background: 'var(--neon-blue-8)', border: '1px solid var(--neon-blue-10)' }}
              >
                <SkipBack size={18} />
              </button>
            )}
            <h2 className="font-display text-base font-medium tracking-wide text-white drop-shadow-lg">
              {title}
            </h2>
            <span
              className="rounded-md px-2 py-0.5 text-[10px] flex items-center gap-1"
              style={{ background: 'rgba(6,182,212,0.15)', border: '1px solid rgba(6,182,212,0.3)', color: '#67e8f9' }}
            >
              <Cpu size={10} /> WebCodecs 硬解
            </span>
            {playbackRate !== 1 && (
              <span className="badge-neon text-[10px]">{playbackRate}x</span>
            )}
          </div>
        )}

        {/* 底部控制 */}
        <div className="absolute inset-x-0 bottom-0 z-30 px-6 pb-4 pt-16"
          style={{ background: 'linear-gradient(to top, rgba(0,0,0,0.85), transparent)' }}
        >
          {/* 进度条 */}
          <div
            className="relative h-1 cursor-pointer rounded-full mb-3 group/progress"
            style={{ background: 'rgba(255,255,255,0.15)' }}
            onClick={handleProgressClick}
          >
            <div
              className="absolute left-0 top-0 h-full rounded-full transition-all"
              style={{ width: `${progress}%`, background: 'var(--neon-blue, #06b6d4)' }}
            />
            <div
              className="absolute top-1/2 -translate-y-1/2 -translate-x-1/2 w-3 h-3 rounded-full opacity-0 group-hover/progress:opacity-100 transition-opacity"
              style={{ left: `${progress}%`, background: 'var(--neon-blue, #06b6d4)', boxShadow: '0 0 8px rgba(6,182,212,0.6)' }}
            />
          </div>

          {/* 控制按钮行 */}
          <div className="flex items-center gap-3 text-white">
            <button onClick={togglePlay} className="p-1 hover:scale-110 transition-transform">
              {isPlaying ? <Pause size={22} /> : <Play size={22} />}
            </button>
            <button onClick={() => seek(-10)} className="p-1 hover:scale-110 transition-transform">
              <SkipBack size={18} />
            </button>
            <button onClick={() => seek(10)} className="p-1 hover:scale-110 transition-transform">
              <SkipForward size={18} />
            </button>

            {/* 时间 */}
            <span className="text-xs font-mono text-surface-300">
              {formatTime(currentTime)} / {formatTime(displayDuration)}
            </span>

            <div className="flex-1" />

            {/* 倍速 */}
            <div className="relative">
              <button
                onClick={() => setShowSpeedMenu(v => !v)}
                className="flex items-center gap-1 p-1 hover:scale-110 transition-transform"
              >
                <Gauge size={18} />
                <span className="text-xs">{playbackRate}x</span>
              </button>
              {showSpeedMenu && (
                <div className="absolute bottom-full right-0 mb-2 rounded-md py-1 min-w-[80px]"
                  style={{ background: 'rgba(11,17,32,0.95)', border: '1px solid var(--neon-blue-15)', backdropFilter: 'blur(8px)' }}
                >
                  {SPEED_OPTIONS.map(r => (
                    <button
                      key={r}
                      onClick={() => changePlaybackRate(r)}
                      className={clsx(
                        'block w-full px-3 py-1 text-xs text-left hover:bg-white/10',
                        playbackRate === r ? 'text-cyan-400' : 'text-white',
                      )}
                    >
                      {r}x
                    </button>
                  ))}
                </div>
              )}
            </div>

            {/* 音量 */}
            <div className="flex items-center gap-2">
              <button onClick={toggleMute} className="p-1 hover:scale-110 transition-transform">
                {muted || volume === 0 ? <VolumeX size={18} /> : <Volume2 size={18} />}
              </button>
              <input
                type="range"
                min="0"
                max="1"
                step="0.01"
                value={muted ? 0 : volume}
                onChange={(e) => handleVolumeChange(Number(e.target.value))}
                className="w-20 accent-cyan-400"
              />
            </div>

            <button onClick={toggleFullscreen} className="p-1 hover:scale-110 transition-transform">
              {isFullscreen ? <Minimize size={18} /> : <Maximize size={18} />}
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
