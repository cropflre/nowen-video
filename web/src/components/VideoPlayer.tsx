import { useRef, useEffect, useCallback, useState } from 'react'
import Hls from 'hls.js'
import { usePlayerStore } from '@/stores/player'
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
}

export default function VideoPlayer({
  src,
  mode = 'hls',
  mediaId,
  title,
  startPosition = 0,
  onBack,
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
    video.addEventListener('play', onPlay)
    video.addEventListener('pause', onPause)
    video.addEventListener('timeupdate', onTimeUpdate)
    video.addEventListener('durationchange', onDurationChange)
    video.addEventListener('volumechange', onVolumeChange)
    return () => {
      video.removeEventListener('play', onPlay)
      video.removeEventListener('pause', onPause)
      video.removeEventListener('timeupdate', onTimeUpdate)
      video.removeEventListener('durationchange', onDurationChange)
      video.removeEventListener('volumechange', onVolumeChange)
    }
  }, [setPlaying, setCurrentTime, setDuration, setVolume, setMuted])

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
      }
    }
    window.addEventListener('keydown', handleKeydown)
    return () => window.removeEventListener('keydown', handleKeydown)
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  const closeAllMenus = () => {
    setShowQuality(false)
    setShowSubtitleMenu(false)
    setShowCastPanel(false)
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
              style={{ border: '1px solid rgba(0, 240, 255, 0.15)' }}
            >
              返回
            </button>
          </div>
        </div>
      )}

      {/* 快进/快退提示气泡 */}
      <div className={clsx(
        'pointer-events-none absolute left-1/2 top-1/2 -translate-x-1/2 -translate-y-1/2 transition-all duration-300',
        seekHint.visible ? 'opacity-100 scale-100' : 'opacity-0 scale-75'
      )}>
        <div className="rounded-2xl px-6 py-3 font-display text-2xl font-bold tracking-wider text-white backdrop-blur-md"
          style={{ background: 'rgba(0, 240, 255, 0.12)', border: '1px solid rgba(0, 240, 255, 0.2)' }}
        >
          {seekHint.text}
        </div>
      </div>

      {/* 中央播放按钮（暂停时显示） */}
      {!isPlaying && !loadError && (
        <div
          className="absolute inset-0 flex items-center justify-center"
          onClick={togglePlay}
        >
          <div className="flex h-20 w-20 items-center justify-center rounded-full transition-all duration-300 hover:scale-110"
            style={{
              background: 'linear-gradient(135deg, rgba(0, 240, 255, 0.2), rgba(138, 43, 226, 0.2))',
              backdropFilter: 'blur(12px)',
              border: '1px solid rgba(0, 240, 255, 0.15)',
              boxShadow: '0 0 40px rgba(0, 240, 255, 0.15), inset 0 0 20px rgba(0, 240, 255, 0.05)',
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
                style={{ background: 'rgba(0, 240, 255, 0.08)', border: '1px solid rgba(0, 240, 255, 0.1)' }}
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
                background: 'rgba(0, 240, 255, 0.15)',
                border: '1px solid rgba(0, 240, 255, 0.2)',
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
                    border: '1px solid rgba(0, 240, 255, 0.1)',
                    backdropFilter: 'blur(20px)',
                  }}
                >
                  <button
                    onClick={() => { loadSubtitle('off', ''); setShowSubtitleMenu(false) }}
                    className={clsx(
                      'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                      !activeSubtitle ? 'text-neon-blue' : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                    )}
                    style={!activeSubtitle ? { background: 'rgba(0, 240, 255, 0.06)' } : {}}
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
                          style={activeSubtitle === `embedded:${sub.index}` ? { background: 'rgba(0, 240, 255, 0.06)' } : {}}
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
                          style={activeSubtitle === `external:${sub.path}` ? { background: 'rgba(0, 240, 255, 0.06)' } : {}}
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
                    border: '1px solid rgba(0, 240, 255, 0.1)',
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
                      style={q.index === currentQuality ? { background: 'rgba(0, 240, 255, 0.06)' } : {}}
                    >
                      {q.label}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {/* 全屏 */}
          <button onClick={toggleFullscreen} className="rounded-lg p-2 text-white/70 transition-all hover:text-white hover:bg-white/5">
            {isFullscreen ? <Minimize size={18} /> : <Maximize size={18} />}
          </button>
        </div>
      </div>

      {/* 点击空白关闭弹出菜单 */}
      {(showQuality || showSubtitleMenu || showCastPanel) && (
        <div className="absolute inset-0 z-[-1]" onClick={closeAllMenus} />
      )}
    </div>
  )
}
