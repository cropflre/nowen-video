import { useRef, useEffect, useCallback, useState } from 'react'
import Hls from 'hls.js'
import { usePlayerStore } from '@/stores/player'
import { useAuthStore } from '@/stores/auth'
import { userApi, subtitleApi } from '@/api'
import type { SubtitleTrack, ExternalSubtitle } from '@/types'
import {
  Play,
  Pause,
  Volume2,
  VolumeX,
  Maximize,
  Minimize,
  SkipBack,
  SkipForward,
  Settings,
  Subtitles,
  Monitor,
  Gauge,
  ChevronRight,
  PictureInPicture2,
} from 'lucide-react'
import clsx from 'clsx'
import CastPanel from './CastPanel'

interface VideoPlayerProps {
  src: string
  mode?: 'direct' | 'hls'
  mediaId: string
  title?: string
  startPosition?: number
  onBack?: () => void
  /** 下一集回调，存在时显示「下一集」按钮，播放结束自动触发 */
  onNext?: () => void
  /** 下一集标题（用于提示） */
  nextTitle?: string
}

export default function VideoPlayer({
  src,
  mode = 'hls',
  mediaId,
  title,
  startPosition = 0,
  onBack,
  onNext,
  nextTitle,
}: VideoPlayerProps) {
  const videoRef = useRef<HTMLVideoElement>(null)
  const hlsRef = useRef<Hls | null>(null)
  const containerRef = useRef<HTMLDivElement>(null)
  const controlsTimerRef = useRef<number>(0)
  const progressReportRef = useRef<number>(0)

  const {
    isPlaying,
    currentTime,
    duration,
    volume,
    isMuted,
    isFullscreen,
    showControls,
    setPlaying,
    setCurrentTime,
    setDuration,
    setVolume,
    setMuted,
    setFullscreen,
    setShowControls,
    reset,
  } = usePlayerStore()

  const [showQuality, setShowQuality] = useState(false)
  const [qualities, setQualities] = useState<{ index: number; label: string }[]>([])
  const [currentQuality, setCurrentQuality] = useState(-1)
  const [loadError, setLoadError] = useState<string | null>(null)

  // 字幕状态
  const [showSubtitleMenu, setShowSubtitleMenu] = useState(false)
  const [embeddedSubs, setEmbeddedSubs] = useState<SubtitleTrack[]>([])
  const [externalSubs, setExternalSubs] = useState<ExternalSubtitle[]>([])
  const [activeSubtitle, setActiveSubtitle] = useState<string | null>(null)
  const [showCastPanel, setShowCastPanel] = useState(false)

  // 倍速播放
  const [showSpeedMenu, setShowSpeedMenu] = useState(false)
  const [playbackRate, setPlaybackRate] = useState(1)
  const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 2, 3]

  // 自动下一集倒计时
  const [nextCountdown, setNextCountdown] = useState<number | null>(null)
  const nextCountdownTimerRef = useRef<number>(0)

  // 快进/快退提示
  const [seekHint, setSeekHint] = useState<{ text: string; visible: boolean }>({ text: '', visible: false })
  const seekHintTimer = useRef<number>(0)

  // 进度条hover预览
  const [hoverProgress, setHoverProgress] = useState<number | null>(null)
  const [hoverTime, setHoverTime] = useState('')

  // 加载字幕轨道列表
  useEffect(() => {
    if (!mediaId) return
    subtitleApi.getTracks(mediaId).then((res) => {
      const data = res.data.data
      if (data) {
        setEmbeddedSubs(data.embedded || [])
        setExternalSubs(data.external || [])
      }
    }).catch(() => {})
  }, [mediaId])

  // 加载字幕到video元素
  const loadSubtitle = useCallback((type: string, id: string) => {
    const video = videoRef.current
    if (!video) return
    while (video.textTracks.length > 0) {
      const track = video.querySelector('track')
      if (track) track.remove()
      else break
    }
    if (type === 'off') {
      setActiveSubtitle(null)
      return
    }
    let subtitleUrl = ''
    let label = '字幕'
    if (type === 'embedded') {
      const index = parseInt(id)
      subtitleUrl = subtitleApi.getExtractUrl(mediaId, index)
      const track = embeddedSubs.find(s => s.index === index)
      label = track?.title || track?.language || `轨道 ${index}`
    } else if (type === 'external') {
      subtitleUrl = subtitleApi.getExternalUrl(id)
      const sub = externalSubs.find(s => s.path === id)
      label = sub?.language || sub?.filename || '外挂字幕'
    }
    if (subtitleUrl) {
      const trackEl = document.createElement('track')
      trackEl.kind = 'subtitles'
      trackEl.label = label
      trackEl.srclang = 'und'
      trackEl.src = subtitleUrl
      trackEl.default = true
      video.appendChild(trackEl)
      setTimeout(() => {
        if (video.textTracks.length > 0) {
          video.textTracks[0].mode = 'showing'
        }
      }, 100)
      setActiveSubtitle(`${type}:${id}`)
    }
  }, [mediaId, embeddedSubs, externalSubs])

  // 初始化播放器
  useEffect(() => {
    const video = videoRef.current
    if (!video || !src) return
    reset()
    setLoadError(null)
    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }
    if (mode === 'direct') {
      video.src = src
      setQualities([])
      video.addEventListener('loadedmetadata', () => {
        if (startPosition > 0) video.currentTime = startPosition
        video.play().catch(() => {})
      }, { once: true })
      video.addEventListener('error', () => {
        const err = video.error
        setLoadError(`播放失败: ${err?.message || '未知错误'}`)
      }, { once: true })
    } else {
      if (Hls.isSupported()) {
        const hls = new Hls({
          startLevel: -1,
          capLevelToPlayerSize: true,
          maxBufferLength: 30,
          maxMaxBufferLength: 60,
          xhrSetup: (xhr: XMLHttpRequest, url: string) => {
            // 为所有 HLS 请求（子 m3u8、.ts 分片）注入 JWT 认证头
            const token = useAuthStore.getState().token
            if (token) {
              xhr.setRequestHeader('Authorization', `Bearer ${token}`)
            }
          },
        })
        hls.loadSource(src)
        hls.attachMedia(video)
        hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
          const levels = data.levels.map((level, index) => ({
            index,
            label: `${level.height}p`,
          }))
          setQualities([{ index: -1, label: '自动' }, ...levels])
          if (startPosition > 0) video.currentTime = startPosition
          video.play().catch(() => {})
        })
        hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
          setCurrentQuality(data.level)
        })
        hls.on(Hls.Events.ERROR, (_event, data) => {
          if (data.fatal) {
            switch (data.type) {
              case Hls.ErrorTypes.NETWORK_ERROR:
                hls.startLoad()
                break
              case Hls.ErrorTypes.MEDIA_ERROR:
                hls.recoverMediaError()
                break
              default:
                setLoadError('转码播放失败，请稍后重试')
                hls.destroy()
                break
            }
          }
        })
        hlsRef.current = hls
      } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
        video.src = src
        if (startPosition > 0) video.currentTime = startPosition
        video.play().catch(() => {})
      } else {
        setLoadError('当前浏览器不支持HLS播放')
      }
    }
    return () => {
      hlsRef.current?.destroy()
      hlsRef.current = null
    }
  }, [src, mode, startPosition, reset])

  // 视频事件监听
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    const onPlay = () => setPlaying(true)
    const onPause = () => setPlaying(false)
    const onTimeUpdate = () => setCurrentTime(video.currentTime)
    const onDurationChange = () => setDuration(video.duration)
    const onVolumeChange = () => {
      setVolume(video.volume)
      setMuted(video.muted)
    }
    // 播放结束 → 自动下一集
    const onEnded = () => {
      setPlaying(false)
      if (onNext) {
        // 开始 5 秒倒计时
        setNextCountdown(5)
      }
    }
    video.addEventListener('play', onPlay)
    video.addEventListener('pause', onPause)
    video.addEventListener('timeupdate', onTimeUpdate)
    video.addEventListener('durationchange', onDurationChange)
    video.addEventListener('volumechange', onVolumeChange)
    video.addEventListener('ended', onEnded)
    return () => {
      video.removeEventListener('play', onPlay)
      video.removeEventListener('pause', onPause)
      video.removeEventListener('timeupdate', onTimeUpdate)
      video.removeEventListener('durationchange', onDurationChange)
      video.removeEventListener('volumechange', onVolumeChange)
      video.removeEventListener('ended', onEnded)
    }
  }, [setPlaying, setCurrentTime, setDuration, setVolume, setMuted, onNext])

  // 自动下一集倒计时逻辑
  useEffect(() => {
    if (nextCountdown === null) return
    if (nextCountdown <= 0) {
      setNextCountdown(null)
      onNext?.()
      return
    }
    nextCountdownTimerRef.current = window.setTimeout(() => {
      setNextCountdown((prev) => (prev !== null ? prev - 1 : null))
    }, 1000)
    return () => clearTimeout(nextCountdownTimerRef.current)
  }, [nextCountdown, onNext])

  // 定期上报播放进度
  useEffect(() => {
    progressReportRef.current = window.setInterval(() => {
      const video = videoRef.current
      if (video && !video.paused && video.currentTime > 0) {
        userApi.updateProgress(mediaId, video.currentTime, video.duration).catch(() => {})
      }
    }, 15000)
    return () => clearInterval(progressReportRef.current)
  }, [mediaId])

  // 全屏变化监听
  useEffect(() => {
    const onFullscreenChange = () => setFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFullscreenChange)
    return () => document.removeEventListener('fullscreenchange', onFullscreenChange)
  }, [setFullscreen])

  // 自动隐藏控制栏
  const resetControlsTimer = useCallback(() => {
    setShowControls(true)
    clearTimeout(controlsTimerRef.current)
    controlsTimerRef.current = window.setTimeout(() => {
      if (videoRef.current && !videoRef.current.paused) {
        setShowControls(false)
      }
    }, 3000)
  }, [setShowControls])

  const togglePlay = () => {
    const video = videoRef.current
    if (!video) return
    // 如果正在倒计时下一集，取消并重新播放
    if (nextCountdown !== null) {
      setNextCountdown(null)
    }
    if (video.paused) video.play()
    else video.pause()
  }

  const seek = (seconds: number) => {
    const video = videoRef.current
    if (!video) return
    video.currentTime = Math.max(0, Math.min(video.duration, video.currentTime + seconds))
    // 显示快进/快退提示
    clearTimeout(seekHintTimer.current)
    setSeekHint({ text: seconds > 0 ? `+${seconds}s` : `${seconds}s`, visible: true })
    seekHintTimer.current = window.setTimeout(() => {
      setSeekHint(prev => ({ ...prev, visible: false }))
    }, 800)
  }

  const handleProgressClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const video = videoRef.current
    if (!video) return
    const rect = e.currentTarget.getBoundingClientRect()
    const pos = (e.clientX - rect.left) / rect.width
    video.currentTime = pos * video.duration
  }

  const handleProgressHover = (e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const pos = (e.clientX - rect.left) / rect.width
    setHoverProgress(pos * 100)
    setHoverTime(formatTime(pos * duration))
  }

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const video = videoRef.current
    if (!video) return
    const vol = parseFloat(e.target.value)
    video.volume = vol
    video.muted = vol === 0
  }

  // 倍速切换
  const changeSpeed = (rate: number) => {
    const video = videoRef.current
    if (!video) return
    video.playbackRate = rate
    setPlaybackRate(rate)
    setShowSpeedMenu(false)
  }

  const toggleFullscreen = () => {
    if (document.fullscreenElement) document.exitFullscreen()
    else containerRef.current?.requestFullscreen()
  }

  const switchQuality = (index: number) => {
    if (hlsRef.current) {
      hlsRef.current.currentLevel = index
      setCurrentQuality(index)
    }
    setShowQuality(false)
  }

  const formatTime = (seconds: number) => {
    if (isNaN(seconds)) return '0:00'
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = Math.floor(seconds % 60)
    if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
    return `${m}:${s.toString().padStart(2, '0')}`
  }

  const progress = duration > 0 ? (currentTime / duration) * 100 : 0

  // 键盘快捷键
  useEffect(() => {
    const handleKeydown = (e: KeyboardEvent) => {
      switch (e.key) {
        case ' ':
        case 'k':
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
          if (videoRef.current) videoRef.current.volume = Math.min(1, videoRef.current.volume + 0.1)
          break
        case 'ArrowDown':
          e.preventDefault()
          if (videoRef.current) videoRef.current.volume = Math.max(0, videoRef.current.volume - 0.1)
          break
        case 'f':
          e.preventDefault()
          toggleFullscreen()
          break
        case 'm':
          e.preventDefault()
          if (videoRef.current) videoRef.current.muted = !videoRef.current.muted
          break
        case 'Escape':
          if (onBack) onBack()
          break
        // 倍速快捷键
        case '<':
        case ',': {
          e.preventDefault()
          const idx = SPEED_OPTIONS.indexOf(playbackRate)
          if (idx > 0) changeSpeed(SPEED_OPTIONS[idx - 1])
          break
        }
        case '>':
        case '.': {
          e.preventDefault()
          const idx = SPEED_OPTIONS.indexOf(playbackRate)
          if (idx < SPEED_OPTIONS.length - 1) changeSpeed(SPEED_OPTIONS[idx + 1])
          break
        }
        // 下一集快捷键
        case 'n':
        case 'N':
          if (onNext) {
            e.preventDefault()
            setNextCountdown(null)
            onNext()
          }
          break
      }
    }
    window.addEventListener('keydown', handleKeydown)
    return () => window.removeEventListener('keydown', handleKeydown)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playbackRate, onNext])

  const closeAllMenus = () => {
    setShowQuality(false)
    setShowSubtitleMenu(false)
    setShowCastPanel(false)
    setShowSpeedMenu(false)
  }

  return (
    <div
      ref={containerRef}
      className="group/player relative h-full w-full bg-black"
      onMouseMove={resetControlsTimer}
      onMouseLeave={() => { if (isPlaying) setShowControls(false) }}
    >
      {/* 视频元素 */}
      <video
        ref={videoRef}
        className="h-full w-full cursor-pointer"
        onClick={togglePlay}
        onDoubleClick={toggleFullscreen}
        playsInline
      />

      {/* 加载错误提示 */}
      {loadError && (
        <div className="absolute inset-0 flex flex-col items-center justify-center bg-black/90 backdrop-blur-sm">
          <div className="rounded-2xl p-8 text-center" style={{
            background: 'rgba(11, 17, 32, 0.8)',
            border: '1px solid rgba(239, 68, 68, 0.2)',
          }}>
            <p className="text-lg font-medium text-red-400">{loadError}</p>
            <button
              onClick={onBack}
              className="btn-ghost mt-4 rounded-xl px-5 py-2.5 text-sm"
              style={{ border: '1px solid var(--neon-blue-15)' }}
            >
              返回
            </button>
          </div>
        </div>
      )}

      {/* 自动下一集倒计时浮层 */}
      {nextCountdown !== null && onNext && (
        <div className="absolute inset-0 z-10 flex items-center justify-center bg-black/60 backdrop-blur-sm">
          <div className="flex flex-col items-center gap-5 rounded-2xl p-8 text-center"
            style={{
              background: 'rgba(11, 17, 32, 0.85)',
              border: '1px solid var(--neon-blue-15)',
              backdropFilter: 'blur(20px)',
            }}
          >
            {/* 倒计时圆环 */}
            <div className="relative flex h-20 w-20 items-center justify-center">
              <svg className="absolute inset-0 -rotate-90" viewBox="0 0 80 80">
                <circle cx="40" cy="40" r="36" fill="none" stroke="var(--neon-blue-10)" strokeWidth="3" />
                <circle cx="40" cy="40" r="36" fill="none" stroke="url(#neon-grad)" strokeWidth="3"
                  strokeDasharray={`${2 * Math.PI * 36}`}
                  strokeDashoffset={`${2 * Math.PI * 36 * (1 - nextCountdown / 5)}`}
                  strokeLinecap="round"
                  className="transition-all duration-1000 ease-linear"
                />
                <defs>
                  <linearGradient id="neon-grad" x1="0%" y1="0%" x2="100%" y2="100%">
                    <stop offset="0%" stopColor="var(--neon-blue)" />
                    <stop offset="100%" stopColor="var(--neon-purple)" />
                  </linearGradient>
                </defs>
              </svg>
              <span className="font-display text-3xl font-bold text-white">{nextCountdown}</span>
            </div>

            <div>
              <p className="text-sm text-surface-400">即将播放下一集</p>
              {nextTitle && (
                <p className="mt-1 font-display text-base font-medium text-white">{nextTitle}</p>
              )}
            </div>

            <div className="flex items-center gap-3">
              <button
                onClick={() => setNextCountdown(null)}
                className="rounded-xl px-5 py-2.5 text-sm font-medium text-surface-300 transition-all hover:text-white"
                style={{
                  background: 'rgba(255,255,255,0.06)',
                  border: '1px solid rgba(255,255,255,0.1)',
                }}
              >
                取消
              </button>
              <button
                onClick={() => { setNextCountdown(null); onNext() }}
                className="rounded-xl px-5 py-2.5 text-sm font-bold transition-all hover:-translate-y-0.5"
                style={{
                  background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))',
                  boxShadow: 'var(--shadow-neon)',
                  color: 'var(--text-on-neon)',
                }}
              >
                立即播放
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 快进/快退提示气泡 */}
      <div className={clsx(
        'pointer-events-none absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 transition-all duration-300',
        seekHint.visible ? 'opacity-100 scale-100' : 'opacity-0 scale-75'
      )}>
        <div className="rounded-2xl px-6 py-3 font-display text-2xl font-bold tracking-wider text-white backdrop-blur-md"
          style={{ background: 'var(--neon-blue-12)', border: '1px solid var(--neon-blue-20)' }}
        >
          {seekHint.text}
        </div>
      </div>

      {/* 中央播放按钮（暂停时显示） */}
      {!isPlaying && !loadError && nextCountdown === null && (
        <div
          className="absolute inset-0 flex items-center justify-center"
          onClick={togglePlay}
        >
          <div className="flex h-20 w-20 items-center justify-center rounded-full transition-all duration-300 hover:scale-110"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue-20), var(--neon-purple-20))',
              backdropFilter: 'blur(12px)',
              border: '1px solid var(--neon-blue-15)',
              boxShadow: '0 0 40px var(--neon-blue-15), inset 0 0 20px var(--neon-blue-5)',
            }}
          >
            <Play size={40} className="ml-1 text-white drop-shadow-lg" fill="white" />
          </div>
        </div>
      )}

      {/* 控制栏 */}
      <div
        className={clsx(
          'player-controls transition-all duration-500',
          showControls ? 'opacity-100' : 'opacity-0 pointer-events-none'
        )}
      >
        {/* 标题栏 */}
        {title && (
          <div className="absolute left-4 top-4 flex items-center gap-3">
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
            <span className="badge-neon text-[10px]">
              {mode === 'direct' ? '直接播放' : 'HLS转码'}
            </span>
            {playbackRate !== 1 && (
              <span className="badge-neon text-[10px]">{playbackRate}x</span>
            )}
          </div>
        )}

        {/* 进度条 */}
        <div
          className="progress-bar group/progress mb-4"
          onClick={handleProgressClick}
          onMouseMove={handleProgressHover}
          onMouseLeave={() => setHoverProgress(null)}
        >
          {/* 预览时间提示 */}
          {hoverProgress !== null && (
            <div
              className="absolute -top-8 -translate-x-1/2 rounded-md px-2 py-1 text-xs font-display text-white tracking-wide pointer-events-none"
              style={{
                left: `${hoverProgress}%`,
                background: 'var(--neon-blue-15)',
                border: '1px solid var(--neon-blue-20)',
                backdropFilter: 'blur(8px)',
              }}
            >
              {hoverTime}
            </div>
          )}
          <div className="progress-bar-fill" style={{ width: `${progress}%` }} />
          <div className="progress-bar-thumb" style={{ left: `${progress}%` }} />
        </div>

        {/* 控制按钮 */}
        <div className="flex items-center gap-1">
          <button onClick={togglePlay} className="rounded-lg p-2 text-white/90 transition-all hover:text-white hover:bg-white/5">
            {isPlaying ? <Pause size={22} /> : <Play size={22} />}
          </button>

          <button onClick={() => seek(-10)} className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5">
            <SkipBack size={18} />
          </button>

          <button onClick={() => seek(10)} className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5">
            <SkipForward size={18} />
          </button>

          {/* 下一集按钮 */}
          {onNext && (
            <button
              onClick={() => { setNextCountdown(null); onNext() }}
              className="flex items-center gap-1 rounded-lg px-2 py-2 text-white/70 transition-all hover:text-white hover:bg-white/5"
              title={nextTitle ? `下一集: ${nextTitle}` : '下一集'}
            >
              <ChevronRight size={18} />
              <span className="hidden text-xs sm:inline">下一集</span>
            </button>
          )}

          {/* 音量 */}
          <button
            onClick={() => { if (videoRef.current) videoRef.current.muted = !videoRef.current.muted }}
            className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5"
          >
            {isMuted || volume === 0 ? <VolumeX size={18} /> : <Volume2 size={18} />}
          </button>

          <div className="group/vol relative flex items-center">
            <input
              type="range"
              min="0"
              max="1"
              step="0.05"
              value={isMuted ? 0 : volume}
              onChange={handleVolumeChange}
              className="h-1 w-20 cursor-pointer appearance-none rounded-full"
              style={{
                background: `linear-gradient(to right, var(--neon-blue) ${(isMuted ? 0 : volume) * 100}%, rgba(255,255,255,0.15) ${(isMuted ? 0 : volume) * 100}%)`,
              }}
            />
          </div>

          {/* 时间 */}
          <span className="ml-3 font-display text-xs tracking-wide text-white/60">
            {formatTime(currentTime)}
            <span className="mx-1 text-neon-blue/30">/</span>
            {formatTime(duration)}
          </span>

          <div className="flex-1" />

          {/* 倍速选择 */}
          <div className="relative">
            <button
              onClick={() => {
                setShowSpeedMenu(!showSpeedMenu)
                setShowQuality(false)
                setShowSubtitleMenu(false)
                setShowCastPanel(false)
              }}
              className={clsx(
                'rounded-lg px-2 py-2 text-xs font-semibold transition-all hover:bg-white/5',
                playbackRate !== 1 ? 'text-neon-blue' : 'text-white/70 hover:text-white'
              )}
              title="播放速度"
            >
              {playbackRate !== 1 ? `${playbackRate}x` : <Gauge size={18} />}
            </button>

            {showSpeedMenu && (
              <div className="absolute bottom-full right-0 mb-2 min-w-[120px] rounded-xl py-1 shadow-2xl"
                style={{
                  background: 'rgba(11, 17, 32, 0.9)',
                  border: '1px solid var(--neon-blue-10)',
                  backdropFilter: 'blur(20px)',
                }}
              >
                {SPEED_OPTIONS.map((speed) => (
                  <button
                    key={speed}
                    onClick={() => changeSpeed(speed)}
                    className={clsx(
                      'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                      speed === playbackRate
                        ? 'text-neon-blue'
                        : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                    )}
                    style={speed === playbackRate ? { background: 'var(--neon-blue-6)' } : {}}
                  >
                    {speed === 1 ? '正常' : `${speed}x`}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* 字幕选择 */}
          {(embeddedSubs.length > 0 || externalSubs.length > 0) && (
            <div className="relative">
              <button
                onClick={() => {
                  setShowSubtitleMenu(!showSubtitleMenu)
                  setShowQuality(false)
                  setShowCastPanel(false)
                }}
                className={clsx(
                  'rounded-lg p-2 transition-all hover:bg-white/5',
                  activeSubtitle ? 'text-neon-blue' : 'text-white/70 hover:text-white'
                )}
                title="字幕"
              >
                <Subtitles size={18} />
              </button>

              {showSubtitleMenu && (
                <div className="absolute bottom-full right-0 mb-2 min-w-[200px] rounded-xl py-1 shadow-2xl"
                  style={{
                    background: 'rgba(11, 17, 32, 0.9)',
                    border: '1px solid var(--neon-blue-10)',
                    backdropFilter: 'blur(20px)',
                  }}
                >
                  <button
                    onClick={() => { loadSubtitle('off', ''); setShowSubtitleMenu(false) }}
                    className={clsx(
                      'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                      !activeSubtitle ? 'text-neon-blue' : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                    )}
                    style={!activeSubtitle ? { background: 'var(--neon-blue-6)' } : {}}
                  >
                    关闭字幕
                  </button>

                  {embeddedSubs.length > 0 && (
                    <>
                      <div className="mx-3 my-1 border-t border-neon-blue/10" />
                      <div className="px-4 py-1 text-[10px] font-bold uppercase tracking-widest text-neon-blue/40">内嵌字幕</div>
                      {embeddedSubs.map((sub) => (
                        <button
                          key={sub.index}
                          onClick={() => { loadSubtitle('embedded', String(sub.index)); setShowSubtitleMenu(false) }}
                          className={clsx(
                            'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                            activeSubtitle === `embedded:${sub.index}`
                              ? 'text-neon-blue'
                              : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                          )}
                          style={activeSubtitle === `embedded:${sub.index}` ? { background: 'var(--neon-blue-6)' } : {}}
                        >
                          {sub.title || sub.language || `轨道 ${sub.index}`}
                          {sub.codec && <span className="ml-2 text-xs text-surface-600">[{sub.codec}]</span>}
                          {sub.default && <span className="ml-1 text-xs text-neon-blue/60">默认</span>}
                        </button>
                      ))}
                    </>
                  )}

                  {externalSubs.length > 0 && (
                    <>
                      <div className="mx-3 my-1 border-t border-neon-blue/10" />
                      <div className="px-4 py-1 text-[10px] font-bold uppercase tracking-widest text-neon-blue/40">外挂字幕</div>
                      {externalSubs.map((sub) => (
                        <button
                          key={sub.path}
                          onClick={() => { loadSubtitle('external', sub.path); setShowSubtitleMenu(false) }}
                          className={clsx(
                            'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                            activeSubtitle === `external:${sub.path}`
                              ? 'text-neon-blue'
                              : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                          )}
                          style={activeSubtitle === `external:${sub.path}` ? { background: 'var(--neon-blue-6)' } : {}}
                        >
                          {sub.language || sub.filename}
                          <span className="ml-2 text-xs text-surface-600">[{sub.format}]</span>
                        </button>
                      ))}
                    </>
                  )}
                </div>
              )}
            </div>
          )}

          {/* 投屏按钮 */}
          <div className="relative">
            <button
              onClick={() => {
                setShowCastPanel(!showCastPanel)
                setShowQuality(false)
                setShowSubtitleMenu(false)
              }}
              className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5"
              title="投屏"
            >
              <Monitor size={18} />
            </button>
            {showCastPanel && (
              <CastPanel
                mediaId={mediaId}
                mediaTitle={title}
                onClose={() => setShowCastPanel(false)}
              />
            )}
          </div>

          {/* 画质选择 */}
          {qualities.length > 1 && (
            <div className="relative">
              <button
                onClick={() => {
                  setShowQuality(!showQuality)
                  setShowSubtitleMenu(false)
                  setShowCastPanel(false)
                }}
                className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5"
              >
                <Settings size={18} />
              </button>

              {showQuality && (
                <div className="absolute bottom-full right-0 mb-2 min-w-[140px] rounded-xl py-1 shadow-2xl"
                  style={{
                    background: 'rgba(11, 17, 32, 0.9)',
                    border: '1px solid var(--neon-blue-10)',
                    backdropFilter: 'blur(20px)',
                  }}
                >
                  {qualities.map((q) => (
                    <button
                      key={q.index}
                      onClick={() => switchQuality(q.index)}
                      className={clsx(
                        'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                        q.index === currentQuality
                          ? 'text-neon-blue'
                          : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                      )}
                      style={q.index === currentQuality ? { background: 'var(--neon-blue-6)' } : {}}
                    >
                      {q.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* 画中画 */}
          <button
            onClick={() => {
              const video = videoRef.current
              if (!video) return
              if (document.pictureInPictureElement) {
                document.exitPictureInPicture().catch(() => {})
              } else {
                video.requestPictureInPicture().catch(() => {})
              }
            }}
            className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5"
            title="画中画"
          >
            <PictureInPicture2 size={18} />
          </button>

          {/* 全屏 */}
          <button onClick={toggleFullscreen} className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5">
            {isFullscreen ? <Minimize size={18} /> : <Maximize size={18} />}
          </button>
        </div>
      </div>

      {/* 点击空白关闭弹出菜单 */}
      {(showQuality || showSubtitleMenu || showCastPanel || showSpeedMenu) && (
        <div className="absolute inset-0 z-[-1]" onClick={closeAllMenus} />
      )}
    </div>
  )
}
