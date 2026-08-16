import { useRef, useEffect, useCallback, useState } from 'react'
import Hls from 'hls.js'
import { usePlayerStore } from '@/stores/player'
import { useAuthStore } from '@/stores/auth'
import { mediaApi, userApi, subtitleApi, subtitlePreprocessApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { SubtitleTrack, ExternalSubtitle, ASRTask, TranslatedSubtitle, SubtitlePreprocessTask, DanmakuComment } from '@/types'
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
  Sparkles,
  Loader2,
  Languages,
  Search,
  MessageCircle,
} from 'lucide-react'
import clsx from 'clsx'
import CastPanel from './CastPanel'
import SubtitleSearchPanel from './SubtitleSearchPanel'
import SubtitleContentSearch from './SubtitleContentSearch'
import { Tag } from '@/components/design-system'

interface VideoPlayerProps {
  src: string
  mode?: 'direct' | 'hls' | 'remux' | 'smart_remux'
  mediaId: string
  title?: string
  startPosition?: number
  onBack?: () => void
  onNext?: () => void
  nextTitle?: string
  isStrm?: boolean
  knownDuration?: number
  onRemuxFallback?: () => void
  onPreprocessReady?: () => void
  spriteVttUrl?: string
}

const PLAYER_CONTROL_CLASS = 'flex h-9 min-w-9 items-center justify-center rounded-[var(--nv-player-radius-control)] text-[var(--nv-player-text-secondary)] transition-[background-color,color,transform] hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)] active:scale-[0.98]'
const PLAYER_MENU_CLASS = 'absolute bottom-full right-0 mb-2 min-w-[200px] overflow-hidden rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] p-2 shadow-[var(--nv-player-shadow)] backdrop-blur-xl'
const PLAYER_MENU_ITEM = 'flex min-h-10 w-full items-center gap-2 rounded-[var(--nv-player-radius-control)] border border-transparent px-3 py-2 text-left text-sm text-[var(--nv-player-text-secondary)] transition-[background-color,border-color,color] hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]'
const PLAYER_MENU_ACTIVE = 'border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]'
const PLAYER_MENU_LABEL = 'px-3 py-1.5 text-[10px] font-bold uppercase tracking-[0.14em] text-[var(--nv-player-text-faint)]'
const PLAYER_DIVIDER = 'mx-2 my-1.5 border-t border-[var(--nv-player-border-subtle)]'

export default function VideoPlayer({
  src,
  mode = 'hls',
  mediaId,
  title,
  startPosition = 0,
  onBack,
  onNext,
  nextTitle,
  isStrm = false,
  knownDuration,
  onPreprocessReady,
  onRemuxFallback,
  spriteVttUrl,
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
  const [qualities, setQualities] = useState<{ index: number; label: string; bitrate?: number; height?: number }[]>([])
  const [currentQuality, setCurrentQuality] = useState(-1)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [currentBitrate, setCurrentBitrate] = useState(0)
  const [bandwidthEstimate, setBandwidthEstimate] = useState(0)

  const [showSubtitleMenu, setShowSubtitleMenu] = useState(false)
  const [embeddedSubs, setEmbeddedSubs] = useState<SubtitleTrack[]>([])
  const [externalSubs, setExternalSubs] = useState<ExternalSubtitle[]>([])
  const [activeSubtitle, setActiveSubtitle] = useState<string | null>(null)
  const userDisabledSubtitleRef = useRef(false)
  const [showCastPanel, setShowCastPanel] = useState(false)

  const [aiSubtitleStatus, setAiSubtitleStatus] = useState<ASRTask | null>(null)
  const [aiGenerating, setAiGenerating] = useState(false)
  const [subtitlePreprocessStatus, setSubtitlePreprocessStatus] = useState<SubtitlePreprocessTask | null>(null)

  const [translatedSubs, setTranslatedSubs] = useState<TranslatedSubtitle[]>([])
  const [translateStatus, setTranslateStatus] = useState<ASRTask | null>(null)
  const [translating, setTranslating] = useState(false)
  const [showTranslateMenu, setShowTranslateMenu] = useState(false)

  const [showSubtitleSearch, setShowSubtitleSearch] = useState(false)
  const [showContentSearch, setShowContentSearch] = useState(false)

  const [danmakuItems, setDanmakuItems] = useState<DanmakuComment[]>([])
  const [danmakuEnabled, setDanmakuEnabled] = useState(true)

  const [showSpeedMenu, setShowSpeedMenu] = useState(false)
  const [playbackRate, setPlaybackRate] = useState(1)
  const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 4, 5, 6, 7, 8]

  const displayDuration = (knownDuration && knownDuration > 0 && knownDuration > duration) ? knownDuration : duration
  const progress = displayDuration > 0 ? (currentTime / displayDuration) * 100 : 0

  const [nextCountdown, setNextCountdown] = useState<number | null>(null)
  const nextCountdownTimerRef = useRef<number>(0)
  const activeDanmaku = danmakuEnabled
    ? danmakuItems.filter((item) => currentTime >= item.position && currentTime < item.position + 8).slice(0, 10)
    : []

  const [seekHint, setSeekHint] = useState<{ text: string; visible: boolean }>({ text: '', visible: false })
  const seekHintTimer = useRef<number>(0)
  const [hoverProgress, setHoverProgress] = useState<number | null>(null)
  const [hoverTime, setHoverTime] = useState('')
  const [spriteVttCues, setSpriteVttCues] = useState<Array<{ start: number; end: number; x: number; y: number; w: number; h: number }>>([])
  const [hoverSprite, setHoverSprite] = useState<{ x: number; y: number; w: number; h: number } | null>(null)

  const [audioTracks, setAudioTracks] = useState<Array<{ id: number; name: string; lang: string }>>([])
  const [currentAudioTrack, setCurrentAudioTrack] = useState(-1)
  const [showAudioMenu, setShowAudioMenu] = useState(false)

  const [gestureOverlay, setGestureOverlay] = useState<{ type: string; value: string } | null>(null)
  const gestureRef = useRef<{
    startX: number
    startY: number
    startTime: number
    startVolume: number
    direction: 'none' | 'horizontal' | 'vertical'
    side: 'left' | 'right'
  } | null>(null)
  const gestureOverlayTimer = useRef<number>(0)

  const { on, off } = useWebSocket()

  useEffect(() => {
    const handleASRProgress = (data: any) => {
      if (data.media_id === mediaId) {
        setAiSubtitleStatus(data as ASRTask)
        if (data.status === 'completed' || data.status === 'failed') setAiGenerating(false)
      }
    }
    const handleASRCompleted = (data: any) => {
      if (data.media_id === mediaId) {
        setAiSubtitleStatus(data as ASRTask)
        setAiGenerating(false)
      }
    }
    const handleASRFailed = (data: any) => {
      if (data.media_id === mediaId) {
        setAiSubtitleStatus(data as ASRTask)
        setAiGenerating(false)
      }
    }
    const handlePreprocessCompleted = (data: any) => {
      if (data.media_id === mediaId && onPreprocessReady) onPreprocessReady()
    }
    const handleSubPreprocessCompleted = (data: any) => {
      if (data.media_id === mediaId) {
        setSubtitlePreprocessStatus(data as SubtitlePreprocessTask)
        subtitleApi.getAIStatus(mediaId).then((res) => {
          const data = res.data.data
          if (data && data.status !== 'none') setAiSubtitleStatus(data)
        }).catch(() => {})
        subtitleApi.listTranslated(mediaId).then((res) => {
          if (Array.isArray(res.data.data)) setTranslatedSubs(res.data.data)
        }).catch(() => {})
      }
    }
    const handleSubPreprocessProgress = (data: any) => {
      if (data.media_id === mediaId) setSubtitlePreprocessStatus(data as SubtitlePreprocessTask)
    }
    const handleSubPreprocessFailed = (data: any) => {
      if (data.media_id === mediaId) setSubtitlePreprocessStatus(data as SubtitlePreprocessTask)
    }

    on(WS_EVENTS.PREPROCESS_COMPLETED, handlePreprocessCompleted)
    on(WS_EVENTS.ASR_PROGRESS, handleASRProgress)
    on(WS_EVENTS.ASR_COMPLETED, handleASRCompleted)
    on(WS_EVENTS.ASR_FAILED, handleASRFailed)
    on(WS_EVENTS.SUB_PREPROCESS_COMPLETED, handleSubPreprocessCompleted)
    on(WS_EVENTS.SUB_PREPROCESS_PROGRESS, handleSubPreprocessProgress)
    on(WS_EVENTS.SUB_PREPROCESS_FAILED, handleSubPreprocessFailed)

    const handleTranslateProgress = (data: any) => {
      if (data.media_id === mediaId) setTranslateStatus(data as ASRTask)
    }
    const handleTranslateCompleted = (data: any) => {
      if (data.media_id === mediaId) {
        setTranslateStatus(data as ASRTask)
        setTranslating(false)
        subtitleApi.listTranslated(mediaId).then((res) => {
          if (Array.isArray(res.data.data)) setTranslatedSubs(res.data.data)
        }).catch(() => {})
      }
    }
    const handleTranslateFailed = (data: any) => {
      if (data.media_id === mediaId) {
        setTranslateStatus(data as ASRTask)
        setTranslating(false)
      }
    }

    on(WS_EVENTS.TRANSLATE_PROGRESS, handleTranslateProgress)
    on(WS_EVENTS.TRANSLATE_COMPLETED, handleTranslateCompleted)
    on(WS_EVENTS.TRANSLATE_FAILED, handleTranslateFailed)

    return () => {
      off(WS_EVENTS.PREPROCESS_COMPLETED, handlePreprocessCompleted)
      off(WS_EVENTS.ASR_PROGRESS, handleASRProgress)
      off(WS_EVENTS.ASR_COMPLETED, handleASRCompleted)
      off(WS_EVENTS.ASR_FAILED, handleASRFailed)
      off(WS_EVENTS.SUB_PREPROCESS_COMPLETED, handleSubPreprocessCompleted)
      off(WS_EVENTS.SUB_PREPROCESS_PROGRESS, handleSubPreprocessProgress)
      off(WS_EVENTS.SUB_PREPROCESS_FAILED, handleSubPreprocessFailed)
      off(WS_EVENTS.TRANSLATE_PROGRESS, handleTranslateProgress)
      off(WS_EVENTS.TRANSLATE_COMPLETED, handleTranslateCompleted)
      off(WS_EVENTS.TRANSLATE_FAILED, handleTranslateFailed)
    }
  }, [mediaId, on, off, onPreprocessReady])

  useEffect(() => {
    if (!mediaId) return
    subtitleApi.getTracks(mediaId).then((res) => {
      const data = res.data.data
      if (data) {
        setEmbeddedSubs(data.embedded || [])
        setExternalSubs(data.external || [])
      }
    }).catch(() => {})

    subtitleApi.getAIStatus(mediaId).then((res) => {
      const data = res.data.data
      if (data && data.status !== 'none') {
        setAiSubtitleStatus(data)
        if (data.status === 'extracting' || data.status === 'transcribing' || data.status === 'converting') setAiGenerating(true)
      }
    }).catch(() => {})

    subtitleApi.listTranslated(mediaId).then((res) => {
      if (Array.isArray(res.data.data)) setTranslatedSubs(res.data.data)
    }).catch(() => {})

    subtitlePreprocessApi.getMediaStatus(mediaId).then((res) => {
      if (res.data.data) setSubtitlePreprocessStatus(res.data.data)
    }).catch(() => {})
  }, [mediaId])

  useEffect(() => {
    if (!mediaId) return
    mediaApi.danmaku(mediaId, 200)
      .then((res) => setDanmakuItems((res.data.data || []).filter((item) => item.content)))
      .catch(() => setDanmakuItems([]))
  }, [mediaId])

  const loadSubtitle = useCallback((type: string, id: string) => {
    const video = videoRef.current
    if (!video) return
    video.querySelectorAll('track').forEach(track => track.remove())
    for (let i = 0; i < video.textTracks.length; i++) video.textTracks[i].mode = 'disabled'

    if (type === 'off') {
      userDisabledSubtitleRef.current = true
      try {
        if (hlsRef.current) {
          hlsRef.current.subtitleTrack = -1
          hlsRef.current.subtitleDisplay = false
        }
      } catch { /* ignore */ }
      setActiveSubtitle(null)
      return
    }

    userDisabledSubtitleRef.current = false
    let subtitleUrl = ''
    let label = '字幕'
    if (type === 'embedded') {
      const index = parseInt(id)
      subtitleUrl = subtitleApi.getExtractUrl(mediaId, index)
      const track = embeddedSubs.find(subtitle => subtitle.index === index)
      label = track?.title || track?.language || `轨道 ${index}`
    } else if (type === 'external') {
      subtitleUrl = subtitleApi.getExternalUrl(id)
      const subtitle = externalSubs.find(item => item.path === id)
      label = subtitle?.language || subtitle?.filename || '外挂字幕'
    } else if (type === 'ai') {
      subtitleUrl = subtitleApi.getAISubtitleUrl(mediaId)
      label = 'AI 生成字幕'
    } else if (type === 'translated') {
      subtitleUrl = subtitleApi.getTranslatedSubtitleUrl(mediaId, id)
      const langNames: Record<string, string> = {
        zh: '中文', en: '英文', ja: '日文', ko: '韩文',
        fr: '法文', de: '德文', es: '西班牙文', pt: '葡萄牙文',
        ru: '俄文', it: '意大利文', ar: '阿拉伯文', th: '泰文',
      }
      label = `翻译字幕（${langNames[id] || id}）`
    }

    if (!subtitleUrl) return
    const token = useAuthStore.getState().token
    const headers: Record<string, string> = {}
    if (token) headers.Authorization = `Bearer ${token}`
    let createdBlobUrl: string | null = null
    let createdTrackEl: HTMLTrackElement | null = null

    fetch(subtitleUrl, { headers })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.text()
      })
      .then((vttText) => {
        const blobUrl = URL.createObjectURL(new Blob([vttText], { type: 'text/vtt' }))
        createdBlobUrl = blobUrl
        const trackEl = document.createElement('track')
        createdTrackEl = trackEl
        trackEl.kind = 'subtitles'
        trackEl.label = label
        trackEl.srclang = 'und'
        trackEl.src = blobUrl
        const onTrackLoad = () => {
          for (let i = 0; i < video.textTracks.length; i++) {
            const track = video.textTracks[i]
            track.mode = track.label === label ? 'showing' : 'hidden'
          }
          URL.revokeObjectURL(blobUrl)
          trackEl.removeEventListener('load', onTrackLoad)
        }
        trackEl.addEventListener('load', onTrackLoad)
        video.appendChild(trackEl)
        setActiveSubtitle(`${type}:${id}`)
      })
      .catch((error) => {
        console.error('字幕加载失败:', error)
        if (createdTrackEl?.parentNode) createdTrackEl.parentNode.removeChild(createdTrackEl)
        if (createdBlobUrl) URL.revokeObjectURL(createdBlobUrl)
        setActiveSubtitle(null)
      })
  }, [mediaId, embeddedSubs, externalSubs])

  const autoSelectSubtitle = useCallback(() => {
    if (externalSubs.length > 0) {
      loadSubtitle('external', externalSubs[0].path)
      return
    }
    const firstPlayableEmbedded = embeddedSubs.find(subtitle => !subtitle.bitmap)
    if (firstPlayableEmbedded) {
      loadSubtitle('embedded', String(firstPlayableEmbedded.index))
      return
    }
    if (aiSubtitleStatus?.status === 'completed') {
      loadSubtitle('ai', '')
      return
    }
    if (translatedSubs.length > 0) loadSubtitle('translated', translatedSubs[0].language)
  }, [externalSubs, embeddedSubs, aiSubtitleStatus, translatedSubs, loadSubtitle])

  useEffect(() => {
    if (!mediaId || activeSubtitle || userDisabledSubtitleRef.current) return
    if (externalSubs.length > 0 || embeddedSubs.some(subtitle => !subtitle.bitmap) || aiSubtitleStatus?.status === 'completed' || translatedSubs.length > 0) {
      autoSelectSubtitle()
    }
  }, [mediaId, activeSubtitle, externalSubs, embeddedSubs, aiSubtitleStatus, translatedSubs, autoSelectSubtitle])

  useEffect(() => {
    const video = videoRef.current
    if (!video || !src) return
    reset()
    setLoadError(null)
    if (hlsRef.current) {
      hlsRef.current.destroy()
      hlsRef.current = null
    }

    if (mode === 'direct' || mode === 'remux' || mode === 'smart_remux') {
      video.src = src
      setQualities([])
      video.addEventListener('loadedmetadata', () => {
        if (startPosition > 0) video.currentTime = startPosition
        video.play().catch(() => {})
      }, { once: true })
      video.addEventListener('error', () => {
        const error = video.error
        if ((mode === 'remux' || mode === 'smart_remux') && onRemuxFallback) {
          console.warn('Remux 播放失败，自动降级到 HLS 转码:', error?.message)
          onRemuxFallback()
          return
        }
        setLoadError(`播放失败: ${error?.message || '未知错误'}`)
      }, { once: true })
    } else if (Hls.isSupported()) {
      const hls = new Hls({
        enableWorker: true,
        startLevel: -1,
        capLevelToPlayerSize: true,
        maxBufferLength: 30,
        maxMaxBufferLength: 60,
        maxBufferSize: 60 * 1000 * 1000,
        abrBandWidthFactor: 0.95,
        abrBandWidthUpFactor: 0.7,
        testBandwidth: true,
        xhrSetup: (xhr: XMLHttpRequest) => {
          const token = useAuthStore.getState().token
          if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
        },
      })
      hls.loadSource(src)
      hls.attachMedia(video)
      hls.on(Hls.Events.MANIFEST_PARSED, (_event, data) => {
        const levels = data.levels.map((level, index) => ({
          index,
          label: `${level.height}p`,
          bitrate: level.bitrate,
          height: level.height,
        }))
        setQualities([{ index: -1, label: '自动' }, ...levels])
        if (startPosition > 0) video.currentTime = startPosition
        video.play().catch(() => {})
      })
      hls.on(Hls.Events.LEVEL_SWITCHED, (_event, data) => {
        setCurrentQuality(data.level)
        const level = hls.levels?.[data.level]
        if (level?.bitrate) setCurrentBitrate(level.bitrate)
      })
      hls.on(Hls.Events.AUDIO_TRACKS_UPDATED, (_event, data) => {
        const tracks = data.audioTracks.map((track, index) => ({
          id: index,
          name: track.name || `音轨 ${index + 1}`,
          lang: track.lang || '',
        }))
        setAudioTracks(tracks)
        setCurrentAudioTrack(hls.audioTrack)
      })
      hls.on(Hls.Events.AUDIO_TRACK_SWITCHED, (_event, data) => setCurrentAudioTrack(data.id))
      const updateBandwidthEstimate = () => {
        const estimate = Math.round((hls as unknown as { bandwidthEstimate: number }).bandwidthEstimate || 0)
        if (estimate > 0) setBandwidthEstimate(estimate)
      }
      hls.on(Hls.Events.FRAG_LOADED, updateBandwidthEstimate)
      hls.on(Hls.Events.ERROR, (_event, data) => {
        if (!data.fatal) return
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
      })
      hlsRef.current = hls
    } else if (video.canPlayType('application/vnd.apple.mpegurl')) {
      video.src = src
      if (startPosition > 0) video.currentTime = startPosition
      video.play().catch(() => {})
    } else {
      setLoadError('当前浏览器不支持HLS播放')
    }

    return () => {
      hlsRef.current?.destroy()
      hlsRef.current = null
    }
  }, [src, mode, startPosition, reset, onRemuxFallback])

  const remuxOffsetRef = useRef(0)

  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    const onPlay = () => setPlaying(true)
    const onPause = () => setPlaying(false)
    const onTimeUpdate = () => setCurrentTime(video.currentTime + remuxOffsetRef.current)
    const onDurationChange = () => setDuration(video.duration)
    const onVolumeChange = () => {
      setVolume(video.volume)
      setMuted(video.muted)
    }
    const onEnded = () => {
      setPlaying(false)
      if (onNext) setNextCountdown(5)
    }
    const onSeeked = () => {
      const position = video.currentTime + remuxOffsetRef.current
      if (position > 0) {
        // Playback position is consumed by the reporting interval below.
      }
    }
    video.addEventListener('play', onPlay)
    video.addEventListener('pause', onPause)
    video.addEventListener('timeupdate', onTimeUpdate)
    video.addEventListener('durationchange', onDurationChange)
    video.addEventListener('volumechange', onVolumeChange)
    video.addEventListener('ended', onEnded)
    video.addEventListener('seeked', onSeeked)
    return () => {
      video.removeEventListener('play', onPlay)
      video.removeEventListener('pause', onPause)
      video.removeEventListener('timeupdate', onTimeUpdate)
      video.removeEventListener('durationchange', onDurationChange)
      video.removeEventListener('volumechange', onVolumeChange)
      video.removeEventListener('ended', onEnded)
      video.removeEventListener('seeked', onSeeked)
    }
  }, [setPlaying, setCurrentTime, setDuration, setVolume, setMuted, onNext, mediaId])

  useEffect(() => {
    if (nextCountdown === null) return
    if (nextCountdown <= 0) {
      setNextCountdown(null)
      onNext?.()
      return
    }
    nextCountdownTimerRef.current = window.setTimeout(() => {
      setNextCountdown(prev => (prev !== null ? prev - 1 : null))
    }, 1000)
    return () => clearTimeout(nextCountdownTimerRef.current)
  }, [nextCountdown, onNext])

  useEffect(() => {
    let tick = 0
    progressReportRef.current = window.setInterval(() => {
      const video = videoRef.current
      if (!video || video.paused || video.currentTime <= 0) return
      const actualTime = video.currentTime + remuxOffsetRef.current
      const actualDuration = displayDuration > 0 ? displayDuration : video.duration
      tick++
      if (tick % 5 === 0) userApi.updateProgress(mediaId, actualTime, actualDuration).catch(() => {})
    }, 3000)
    return () => clearInterval(progressReportRef.current)
  }, [mediaId, displayDuration])

  useEffect(() => {
    const onFullscreenChange = () => setFullscreen(!!document.fullscreenElement)
    document.addEventListener('fullscreenchange', onFullscreenChange)
    return () => document.removeEventListener('fullscreenchange', onFullscreenChange)
  }, [setFullscreen])

  const resetControlsTimer = useCallback(() => {
    setShowControls(true)
    clearTimeout(controlsTimerRef.current)
    controlsTimerRef.current = window.setTimeout(() => {
      if (videoRef.current && !videoRef.current.paused) setShowControls(false)
    }, 3000)
  }, [setShowControls])

  const togglePlay = () => {
    const video = videoRef.current
    if (!video) return
    if (nextCountdown !== null) setNextCountdown(null)
    if (video.paused) video.play()
    else video.pause()
  }

  const remuxSeek = useCallback((targetTime: number) => {
    const video = videoRef.current
    if (!video || (mode !== 'remux' && mode !== 'smart_remux') || !src) return
    const baseUrl = src.replace(/[&?]start=[^&]*/g, '')
    const separator = baseUrl.includes('?') ? '&' : '?'
    remuxOffsetRef.current = targetTime
    video.src = `${baseUrl}${separator}start=${Math.floor(targetTime)}`
    video.play().catch(() => {})
  }, [src, mode, mediaId])

  const seek = (seconds: number) => {
    const video = videoRef.current
    if (!video) return
    if (mode === 'remux' || mode === 'smart_remux') {
      const currentPos = remuxOffsetRef.current + (video.currentTime || 0)
      remuxSeek(Math.max(0, Math.min(displayDuration, currentPos + seconds)))
    } else {
      video.currentTime = Math.max(0, Math.min(video.duration || displayDuration, video.currentTime + seconds))
    }
    clearTimeout(seekHintTimer.current)
    setSeekHint({ text: seconds > 0 ? `+${seconds}s` : `${seconds}s`, visible: true })
    seekHintTimer.current = window.setTimeout(() => setSeekHint(prev => ({ ...prev, visible: false })), 800)
  }

  const handleProgressClick = (event: React.MouseEvent<HTMLDivElement>) => {
    const video = videoRef.current
    if (!video) return
    const rect = event.currentTarget.getBoundingClientRect()
    const targetTime = ((event.clientX - rect.left) / rect.width) * displayDuration
    if (mode === 'remux' || mode === 'smart_remux') {
      remuxSeek(targetTime)
      return
    }
    if (video.duration > 0 && targetTime <= video.duration) video.currentTime = targetTime
    else if (video.duration > 0) video.currentTime = video.duration - 0.5
  }

  const handleProgressHover = (event: React.MouseEvent<HTMLDivElement>) => {
    const rect = event.currentTarget.getBoundingClientRect()
    const position = (event.clientX - rect.left) / rect.width
    setHoverProgress(position * 100)
    const hoverSeconds = position * displayDuration
    setHoverTime(formatTime(hoverSeconds))
    if (spriteVttCues.length > 0) {
      const cue = spriteVttCues.find(item => hoverSeconds >= item.start && hoverSeconds < item.end) || spriteVttCues[spriteVttCues.length - 1]
      setHoverSprite(cue ? { x: cue.x, y: cue.y, w: cue.w, h: cue.h } : null)
    }
  }

  const handleVolumeChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const video = videoRef.current
    if (!video) return
    const nextVolume = parseFloat(event.target.value)
    video.volume = nextVolume
    video.muted = nextVolume === 0
  }

  const changeSpeed = useCallback((rate: number) => {
    const video = videoRef.current
    if (!video) return
    video.playbackRate = rate
    setPlaybackRate(rate)
    setShowSpeedMenu(false)
    clearTimeout(seekHintTimer.current)
    setSeekHint({ text: rate === 1 ? '正常速度' : `${rate}x 倍速`, visible: true })
    seekHintTimer.current = window.setTimeout(() => setSeekHint(prev => ({ ...prev, visible: false })), 800)
  }, [])

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

  const switchAudioTrack = (id: number) => {
    if (hlsRef.current) {
      hlsRef.current.audioTrack = id
      setCurrentAudioTrack(id)
    }
    setShowAudioMenu(false)
  }

  useEffect(() => {
    if (!spriteVttUrl) {
      setSpriteVttCues([])
      return
    }
    const token = useAuthStore.getState().token
    const url = token
      ? `${spriteVttUrl}${spriteVttUrl.includes('?') ? '&' : '?'}token=${encodeURIComponent(token)}`
      : spriteVttUrl
    fetch(url)
      .then(response => response.text())
      .then((text) => {
        const cues: Array<{ start: number; end: number; x: number; y: number; w: number; h: number }> = []
        for (const block of text.split(/\n\n+/)) {
          const lines = block.trim().split('\n')
          const timeLine = lines.find(line => line.includes('-->'))
          const coordLine = lines.find(line => line.includes('#xywh='))
          if (!timeLine || !coordLine) continue
          const [startStr, endStr] = timeLine.split('-->').map(value => value.trim())
          const parseVTTTime = (value: string) => {
            const parts = value.split(':').map(Number)
            return parts[0] * 3600 + parts[1] * 60 + parts[2]
          }
          const match = coordLine.match(/#xywh=(\d+),(\d+),(\d+),(\d+)/)
          if (!match) continue
          cues.push({
            start: parseVTTTime(startStr),
            end: parseVTTTime(endStr),
            x: parseInt(match[1]),
            y: parseInt(match[2]),
            w: parseInt(match[3]),
            h: parseInt(match[4]),
          })
        }
        setSpriteVttCues(cues)
      })
      .catch(() => setSpriteVttCues([]))
  }, [spriteVttUrl])

  const formatTime = (seconds: number) => {
    if (isNaN(seconds)) return '0:00'
    const hours = Math.floor(seconds / 3600)
    const minutes = Math.floor((seconds % 3600) / 60)
    const secs = Math.floor(seconds % 60)
    if (hours > 0) return `${hours}:${minutes.toString().padStart(2, '0')}:${secs.toString().padStart(2, '0')}`
    return `${minutes}:${secs.toString().padStart(2, '0')}`
  }

  useEffect(() => {
    const handleKeydown = (event: KeyboardEvent) => {
      switch (event.key) {
        case ' ':
        case 'k':
          event.preventDefault()
          togglePlay()
          break
        case 'ArrowLeft':
          event.preventDefault()
          seek(-10)
          break
        case 'ArrowRight':
          event.preventDefault()
          seek(10)
          break
        case 'ArrowUp':
          event.preventDefault()
          if (videoRef.current) videoRef.current.volume = Math.min(1, videoRef.current.volume + 0.1)
          break
        case 'ArrowDown':
          event.preventDefault()
          if (videoRef.current) videoRef.current.volume = Math.max(0, videoRef.current.volume - 0.1)
          break
        case 'f':
          event.preventDefault()
          if (event.ctrlKey || event.metaKey) setShowContentSearch(prev => !prev)
          else toggleFullscreen()
          break
        case 'm':
          event.preventDefault()
          if (videoRef.current) videoRef.current.muted = !videoRef.current.muted
          break
        case 'Escape':
          if (showContentSearch) setShowContentSearch(false)
          else if (showSubtitleSearch) setShowSubtitleSearch(false)
          else if (onBack) onBack()
          break
        case '<':
        case ',': {
          event.preventDefault()
          const index = SPEED_OPTIONS.indexOf(playbackRate)
          if (index > 0) changeSpeed(SPEED_OPTIONS[index - 1])
          break
        }
        case '>':
        case '.': {
          event.preventDefault()
          const index = SPEED_OPTIONS.indexOf(playbackRate)
          if (index < SPEED_OPTIONS.length - 1) changeSpeed(SPEED_OPTIONS[index + 1])
          break
        }
        case 'Backspace':
          if (playbackRate !== 1) {
            event.preventDefault()
            changeSpeed(1)
          }
          break
        case 'n':
        case 'N':
          if (onNext) {
            event.preventDefault()
            setNextCountdown(null)
            onNext()
          }
          break
      }
    }
    window.addEventListener('keydown', handleKeydown)
    return () => window.removeEventListener('keydown', handleKeydown)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playbackRate, onNext, showContentSearch, showSubtitleSearch])

  const handleTouchStart = useCallback((event: React.TouchEvent) => {
    const touch = event.touches[0]
    const rect = containerRef.current?.getBoundingClientRect()
    if (!rect) return
    gestureRef.current = {
      startX: touch.clientX,
      startY: touch.clientY,
      startTime: currentTime,
      startVolume: volume,
      direction: 'none',
      side: touch.clientX < rect.width / 2 ? 'left' : 'right',
    }
  }, [currentTime, volume])

  const handleTouchMove = useCallback((event: React.TouchEvent) => {
    const gesture = gestureRef.current
    if (!gesture) return
    const touch = event.touches[0]
    const deltaX = touch.clientX - gesture.startX
    const deltaY = touch.clientY - gesture.startY
    const rect = containerRef.current?.getBoundingClientRect()
    if (!rect) return

    if (gesture.direction === 'none') {
      if (Math.abs(deltaX) > 10 || Math.abs(deltaY) > 10) gesture.direction = Math.abs(deltaX) > Math.abs(deltaY) ? 'horizontal' : 'vertical'
      else return
    }

    if (gesture.direction === 'horizontal') {
      const effectiveDuration = (knownDuration && knownDuration > 0 && knownDuration > duration) ? knownDuration : duration
      const seekDelta = (deltaX / rect.width) * effectiveDuration * 0.3
      const newTime = Math.max(0, Math.min(effectiveDuration, gesture.startTime + seekDelta))
      const diff = newTime - gesture.startTime
      setGestureOverlay({
        type: 'seek',
        value: `${diff >= 0 ? '+' : '-'}${formatTime(Math.abs(diff))} / ${formatTime(newTime)}`,
      })
    } else if (gesture.side === 'right') {
      const nextVolume = Math.max(0, Math.min(1, gesture.startVolume - deltaY / rect.height))
      setVolume(nextVolume)
      setGestureOverlay({ type: 'volume', value: `${Math.round(nextVolume * 100)}%` })
    } else {
      const brightness = Math.max(0.3, Math.min(1.5, 1 - deltaY / rect.height))
      if (videoRef.current) videoRef.current.style.filter = `brightness(${brightness})`
      setGestureOverlay({ type: 'brightness', value: `${Math.round(brightness * 100)}%` })
    }
  }, [duration, knownDuration, setVolume])

  const handleTouchEnd = useCallback(() => {
    const gesture = gestureRef.current
    if (!gesture) return
    if (gesture.direction === 'horizontal') {
      const video = videoRef.current
      const rect = containerRef.current?.getBoundingClientRect()
      if (video && rect) {
        // Preserve existing behavior: gesture overlay previews the seek delta.
      }
    }
    gestureRef.current = null
    clearTimeout(gestureOverlayTimer.current)
    gestureOverlayTimer.current = window.setTimeout(() => setGestureOverlay(null), 500)
  }, [])

  const closeAllMenus = () => {
    setShowQuality(false)
    setShowSubtitleMenu(false)
    setShowCastPanel(false)
    setShowSpeedMenu(false)
    setShowTranslateMenu(false)
    setShowContentSearch(false)
    setShowAudioMenu(false)
  }

  const openExclusive = (menu: 'speed' | 'subtitle' | 'cast' | 'audio' | 'quality') => {
    setShowSpeedMenu(menu === 'speed' ? !showSpeedMenu : false)
    setShowSubtitleMenu(menu === 'subtitle' ? !showSubtitleMenu : false)
    setShowCastPanel(menu === 'cast' ? !showCastPanel : false)
    setShowAudioMenu(menu === 'audio' ? !showAudioMenu : false)
    setShowQuality(menu === 'quality' ? !showQuality : false)
    if (menu !== 'subtitle') setShowTranslateMenu(false)
    setShowContentSearch(false)
  }

  const menuItemClass = (active = false, disabled = false) => clsx(
    PLAYER_MENU_ITEM,
    active && PLAYER_MENU_ACTIVE,
    disabled && 'cursor-not-allowed opacity-40 hover:bg-transparent hover:text-[var(--nv-player-text-secondary)]',
  )

  const playbackLabel = isStrm
    ? 'STRM远程流'
    : mode === 'direct'
      ? '直接播放'
      : (mode === 'remux' || mode === 'smart_remux')
        ? 'Remux播放'
        : 'HLS转码'

  return (
    <div
      ref={containerRef}
      className="group/player relative h-full w-full bg-[var(--nv-player-canvas)]"
      onMouseMove={resetControlsTimer}
      onMouseLeave={() => { if (isPlaying) setShowControls(false) }}
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
    >
      <video
        ref={videoRef}
        className="h-full w-full cursor-pointer"
        onClick={togglePlay}
        onDoubleClick={toggleFullscreen}
        playsInline
        crossOrigin="anonymous"
      />

      {activeDanmaku.length > 0 && (
        <div className="pointer-events-none absolute left-0 right-0 top-12 z-10 h-[42%] overflow-hidden">
          {activeDanmaku.map((item, index) => (
            <div
              key={item.id}
              className="danmaku-item absolute left-full whitespace-nowrap rounded-full px-3 py-1 text-sm font-semibold shadow-lg"
              style={{
                top: `${(index % 7) * 14}%`,
                color: item.color || 'var(--nv-player-text-primary)',
                background: 'color-mix(in srgb, var(--nv-player-canvas) 68%, transparent)',
                textShadow: '0 1px 2px rgba(0,0,0,.9)',
                animationDelay: `${(index % 3) * .35}s`,
              }}
            >
              {item.content}
            </div>
          ))}
        </div>
      )}

      {loadError && (
        <div className="absolute inset-0 z-40 flex items-center justify-center bg-[color-mix(in_srgb,var(--nv-player-canvas)_90%,transparent)] p-4 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-surface)] p-7 text-center shadow-[var(--nv-player-shadow)]">
            <p className="text-lg font-medium text-[var(--nv-player-danger)]">{loadError}</p>
            {onBack && (
              <button type="button" onClick={onBack} className="mt-5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-5 py-2.5 text-sm text-[var(--nv-player-text-primary)] transition-[background-color,border-color] hover:border-[var(--nv-player-border-hover)] hover:bg-[var(--nv-player-surface-hover)]">
                返回
              </button>
            )}
          </div>
        </div>
      )}

      {gestureOverlay && (
        <div className="pointer-events-none absolute inset-0 z-30 flex items-center justify-center">
          <div className="rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] px-8 py-4 text-center shadow-[var(--nv-player-shadow)] backdrop-blur-xl">
            <p className="mb-1 text-xs text-[var(--nv-player-text-tertiary)]">
              {gestureOverlay.type === 'seek' ? '⏩ 进度' : gestureOverlay.type === 'volume' ? '🔊 音量' : '☀️ 亮度'}
            </p>
            <p className="font-mono text-xl font-bold tabular-nums text-[var(--nv-player-text-primary)]">{gestureOverlay.value}</p>
          </div>
        </div>
      )}

      {nextCountdown !== null && onNext && (
        <div className="absolute inset-0 z-30 flex items-center justify-center bg-[color-mix(in_srgb,var(--nv-player-canvas)_64%,transparent)] p-4 backdrop-blur-sm">
          <div className="flex w-full max-w-sm flex-col items-center gap-5 rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] p-8 text-center shadow-[var(--nv-player-shadow)]">
            <div className="relative flex h-20 w-20 items-center justify-center">
              <svg className="absolute inset-0 -rotate-90" viewBox="0 0 80 80" aria-hidden="true">
                <circle cx="40" cy="40" r="36" fill="none" stroke="var(--nv-player-border)" strokeWidth="3" />
                <circle
                  cx="40"
                  cy="40"
                  r="36"
                  fill="none"
                  stroke="var(--nv-player-accent)"
                  strokeWidth="3"
                  strokeDasharray={`${2 * Math.PI * 36}`}
                  strokeDashoffset={`${2 * Math.PI * 36 * (1 - nextCountdown / 5)}`}
                  strokeLinecap="round"
                  className="transition-all duration-1000 ease-linear"
                />
              </svg>
              <span className="font-mono text-3xl font-bold tabular-nums text-[var(--nv-player-text-primary)]">{nextCountdown}</span>
            </div>
            <div>
              <p className="text-sm text-[var(--nv-player-text-tertiary)]">即将播放下一集</p>
              {nextTitle && <p className="mt-1 font-display text-base font-medium tracking-tight text-[var(--nv-player-text-primary)]">{nextTitle}</p>}
            </div>
            <div className="flex items-center gap-3">
              <button type="button" onClick={() => setNextCountdown(null)} className="rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-5 py-2.5 text-sm font-medium text-[var(--nv-player-text-secondary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]">
                取消
              </button>
              <button type="button" onClick={() => { setNextCountdown(null); onNext() }} className="rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent)] px-5 py-2.5 text-sm font-bold text-[var(--nv-text-on-brand)] transition-[background-color,transform] hover:bg-[var(--nv-action-primary-hover)] active:scale-[0.98]">
                立即播放
              </button>
            </div>
          </div>
        </div>
      )}

      <div className={clsx(
        'pointer-events-none absolute left-1/2 top-1/2 z-20 -translate-x-1/2 -translate-y-1/2 transition-[opacity,transform] duration-200',
        seekHint.visible ? 'scale-100 opacity-100' : 'scale-[0.96] opacity-0',
      )}>
        <div className="rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-surface)] px-6 py-3 font-mono text-2xl font-bold tabular-nums text-[var(--nv-player-text-primary)] shadow-[var(--nv-player-shadow)] backdrop-blur-xl">
          {seekHint.text}
        </div>
      </div>

      {!isPlaying && !loadError && nextCountdown === null && (
        <button
          type="button"
          className="absolute left-1/2 top-1/2 z-10 flex h-20 w-20 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-full border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-text-primary)] shadow-[var(--nv-player-shadow)] backdrop-blur-xl transition-[background-color,border-color,transform] hover:bg-[var(--nv-player-accent-soft-hover)] active:scale-[0.98]"
          onClick={togglePlay}
          aria-label="播放"
        >
          <Play size={40} className="ml-1" fill="currentColor" aria-hidden="true" />
        </button>
      )}

      <div className={clsx('player-controls transition-opacity duration-300', showControls ? 'opacity-100' : 'pointer-events-none opacity-0')}>
        {title && (
          <div className="absolute left-4 top-4 z-20 flex max-w-[calc(100%-2rem)] items-center gap-3">
            {onBack && (
              <button type="button" onClick={onBack} className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] text-[var(--nv-player-text-primary)] shadow-[var(--nv-shadow-card)] backdrop-blur-md transition-[background-color,border-color,transform] hover:border-[var(--nv-player-border-hover)] hover:bg-[var(--nv-player-surface-hover)] active:scale-[0.98]" aria-label="返回">
                <SkipBack size={18} aria-hidden="true" />
              </button>
            )}
            <h2 className="min-w-0 truncate font-display text-base font-medium tracking-tight text-[var(--nv-player-text-primary)] drop-shadow-lg">{title}</h2>
            <Tag tone="brand" className="shrink-0 text-[10px]">{playbackLabel}</Tag>
            {playbackRate !== 1 && <Tag tone="quality" className="shrink-0 text-[10px]">{playbackRate}x</Tag>}
          </div>
        )}

        <div
          className="progress-bar group/progress mb-4"
          onClick={handleProgressClick}
          onMouseMove={handleProgressHover}
          onMouseLeave={() => { setHoverProgress(null); setHoverSprite(null) }}
          role="slider"
          aria-label="播放进度"
          aria-valuemin={0}
          aria-valuemax={Math.max(displayDuration, 0)}
          aria-valuenow={Math.max(currentTime, 0)}
        >
          {hoverProgress !== null && (
            <div className="pointer-events-none absolute flex -translate-x-1/2 flex-col items-center gap-1" style={{ left: `${hoverProgress}%`, bottom: '100%', marginBottom: 8 }}>
              {hoverSprite && spriteVttUrl && (
                <div
                  className="overflow-hidden rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] shadow-[var(--nv-player-shadow)]"
                  style={{
                    width: hoverSprite.w,
                    height: hoverSprite.h,
                    backgroundImage: `url(${spriteVttUrl.replace('sprite.vtt', 'sprite.jpg')}${spriteVttUrl.includes('?') ? '&' : '?'}token=${useAuthStore.getState().token || ''})`,
                    backgroundPosition: `-${hoverSprite.x}px -${hoverSprite.y}px`,
                    backgroundSize: 'auto',
                    backgroundRepeat: 'no-repeat',
                  }}
                />
              )}
              <div className="rounded-[var(--nv-radius-sm)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] px-2 py-1 font-mono text-xs tabular-nums text-[var(--nv-player-text-primary)] shadow-[var(--nv-shadow-card)] backdrop-blur-md">
                {hoverTime}
              </div>
            </div>
          )}
          {knownDuration && knownDuration > 0 && duration > 0 && duration < knownDuration && (
            <div className="absolute left-0 top-0 h-full rounded-full bg-[var(--nv-player-accent)] opacity-30" style={{ width: `${(duration / knownDuration) * 100}%` }} />
          )}
          <div className="progress-bar-fill" style={{ width: `${progress}%` }} />
          <div className="progress-bar-thumb" style={{ left: `${progress}%` }} />
        </div>

        <div className="flex items-center gap-1 text-[var(--nv-player-text-primary)]">
          <button type="button" onClick={togglePlay} className={PLAYER_CONTROL_CLASS} aria-label={isPlaying ? '暂停' : '播放'}>{isPlaying ? <Pause size={22} aria-hidden="true" /> : <Play size={22} aria-hidden="true" />}</button>
          <button type="button" onClick={() => seek(-10)} className={PLAYER_CONTROL_CLASS} aria-label="后退 10 秒"><SkipBack size={18} aria-hidden="true" /></button>
          <button type="button" onClick={() => seek(10)} className={PLAYER_CONTROL_CLASS} aria-label="前进 10 秒"><SkipForward size={18} aria-hidden="true" /></button>

          {onNext && (
            <button type="button" onClick={() => { setNextCountdown(null); onNext() }} className={clsx(PLAYER_CONTROL_CLASS, 'gap-1 px-2')} title={nextTitle ? `下一集: ${nextTitle}` : '下一集'}>
              <ChevronRight size={18} aria-hidden="true" />
              <span className="hidden text-xs sm:inline">下一集</span>
            </button>
          )}

          <button type="button" onClick={() => { if (videoRef.current) videoRef.current.muted = !videoRef.current.muted }} className={PLAYER_CONTROL_CLASS} aria-label={isMuted ? '取消静音' : '静音'}>
            {isMuted || volume === 0 ? <VolumeX size={18} aria-hidden="true" /> : <Volume2 size={18} aria-hidden="true" />}
          </button>

          <div className="group/vol hidden items-center sm:flex">
            <input
              type="range"
              min="0"
              max="1"
              step="0.05"
              value={isMuted ? 0 : volume}
              onChange={handleVolumeChange}
              className="player-volume-slider w-20 cursor-pointer appearance-none"
              style={{ background: `linear-gradient(to right, var(--nv-player-accent) ${(isMuted ? 0 : volume) * 100}%, color-mix(in srgb, var(--nv-player-text-primary) 15%, transparent) ${(isMuted ? 0 : volume) * 100}%)` }}
              aria-label="音量"
            />
          </div>

          <span className="ml-2 hidden whitespace-nowrap font-mono text-xs tabular-nums text-[var(--nv-player-text-tertiary)] md:inline">
            {formatTime(currentTime)} <span className="mx-1 text-[var(--nv-player-text-faint)]">/</span> {formatTime(displayDuration)}
          </span>

          <div className="flex-1" />

          <div className="relative">
            <button type="button" onClick={() => openExclusive('speed')} className={clsx(PLAYER_CONTROL_CLASS, 'px-2 text-xs font-semibold tabular-nums', playbackRate !== 1 && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')} title="播放速度" aria-expanded={showSpeedMenu}>
              {playbackRate !== 1 ? `${playbackRate}x` : <Gauge size={18} aria-hidden="true" />}
            </button>
            {showSpeedMenu && (
              <div className={clsx(PLAYER_MENU_CLASS, 'grid max-h-[360px] min-w-[244px] grid-cols-2 gap-1.5 overflow-y-auto')} role="menu">
                {playbackRate !== 1 && (
                  <button type="button" onClick={() => changeSpeed(1)} className={clsx(PLAYER_MENU_ITEM, PLAYER_MENU_ACTIVE, 'col-span-2 justify-between')}>
                    <span>恢复正常</span><span className="text-[10px] text-[var(--nv-player-text-faint)]">Backspace</span>
                  </button>
                )}
                {SPEED_OPTIONS.map((speed) => (
                  <button key={speed} type="button" onClick={() => changeSpeed(speed)} className={menuItemClass(speed === playbackRate)} role="menuitemradio" aria-checked={speed === playbackRate}>
                    {speed === 1 ? '正常' : `${speed}x`}
                  </button>
                ))}
                <div className="col-span-2 px-2 pt-1 text-[10px] text-[var(--nv-player-text-faint)]">‹ / › 调速 · Backspace 恢复</div>
              </div>
            )}
          </div>

          {danmakuItems.length > 0 && (
            <button type="button" onClick={() => setDanmakuEnabled(!danmakuEnabled)} className={clsx(PLAYER_CONTROL_CLASS, danmakuEnabled && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')} title={danmakuEnabled ? '关闭短评弹幕' : '开启短评弹幕'} aria-pressed={danmakuEnabled}>
              <MessageCircle size={18} aria-hidden="true" />
            </button>
          )}

          {!isStrm && (
            <div className="relative">
              <button type="button" onClick={() => openExclusive('subtitle')} className={clsx(PLAYER_CONTROL_CLASS, activeSubtitle && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')} title="字幕" aria-expanded={showSubtitleMenu}>
                <Subtitles size={18} aria-hidden="true" />
              </button>
              {showSubtitleMenu && (
                <div className={clsx(PLAYER_MENU_CLASS, 'max-h-[min(480px,calc(100vh-120px))] min-w-[270px] overflow-y-auto')} role="menu">
                  <button type="button" onClick={() => { loadSubtitle('off', ''); setShowSubtitleMenu(false) }} className={menuItemClass(!activeSubtitle)}>关闭字幕</button>

                  {embeddedSubs.length > 0 && (
                    <>
                      <div className={PLAYER_DIVIDER} />
                      <div className={PLAYER_MENU_LABEL}>内嵌字幕</div>
                      {embeddedSubs.map((sub) => (
                        <button
                          key={sub.index}
                          type="button"
                          onClick={() => {
                            if (sub.bitmap) return
                            loadSubtitle('embedded', String(sub.index))
                            setShowSubtitleMenu(false)
                          }}
                          disabled={sub.bitmap}
                          className={menuItemClass(activeSubtitle === `embedded:${sub.index}`, sub.bitmap)}
                        >
                          <span className="min-w-0 flex-1 truncate">{sub.title || sub.language || `轨道 ${sub.index}`}</span>
                          {sub.codec && <span className="text-xs text-[var(--nv-player-text-faint)]">[{sub.codec}]</span>}
                          {sub.bitmap && <span className="text-xs text-[var(--nv-player-danger)]">不可用</span>}
                          {!sub.bitmap && sub.default && <Tag tone="brand">默认</Tag>}
                        </button>
                      ))}
                    </>
                  )}

                  {externalSubs.length > 0 && (
                    <>
                      <div className={PLAYER_DIVIDER} />
                      <div className={PLAYER_MENU_LABEL}>外挂字幕</div>
                      {externalSubs.map((sub) => (
                        <button key={sub.path} type="button" onClick={() => { loadSubtitle('external', sub.path); setShowSubtitleMenu(false) }} className={menuItemClass(activeSubtitle === `external:${sub.path}`)}>
                          <span className="min-w-0 flex-1 truncate">{sub.language || sub.filename}</span>
                          <span className="text-xs text-[var(--nv-player-text-faint)]">[{sub.format}]</span>
                        </button>
                      ))}
                    </>
                  )}

                  {(aiSubtitleStatus?.status === 'completed' || aiGenerating || subtitlePreprocessStatus?.status === 'running' || subtitlePreprocessStatus?.status === 'pending') && (
                    <>
                      <div className={PLAYER_DIVIDER} />
                      <div className={PLAYER_MENU_LABEL}><Sparkles size={10} className="mr-1 inline" aria-hidden="true" />AI 字幕</div>
                      {aiSubtitleStatus?.status === 'completed' ? (
                        <button type="button" onClick={() => { loadSubtitle('ai', ''); setShowSubtitleMenu(false) }} className={menuItemClass(activeSubtitle === 'ai:')}>
                          <Sparkles size={12} aria-hidden="true" /><span className="flex-1">AI 生成字幕</span><span className="text-xs text-[var(--nv-player-success)]">✓ 已就绪</span>
                        </button>
                      ) : aiGenerating ? (
                        <div className="px-3 py-2.5 text-sm text-[var(--nv-player-text-tertiary)]">
                          <div className="flex items-center gap-2"><Loader2 size={14} className="animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" /><span>{aiSubtitleStatus?.message || '正在生成...'}</span></div>
                          {aiSubtitleStatus?.progress != null && aiSubtitleStatus.progress > 0 && (
                            <div className="mt-2 h-1 overflow-hidden rounded-full bg-[var(--nv-player-surface-hover)]"><div className="h-full rounded-full bg-[var(--nv-player-accent)] transition-[width] duration-500" style={{ width: `${aiSubtitleStatus.progress}%` }} /></div>
                          )}
                        </div>
                      ) : (subtitlePreprocessStatus?.status === 'running' || subtitlePreprocessStatus?.status === 'pending') ? (
                        <div className="px-3 py-2.5 text-sm text-[var(--nv-player-text-tertiary)]">
                          <div className="flex items-center gap-2"><Loader2 size={14} className="animate-spin text-[var(--nv-player-warning)]" aria-hidden="true" /><span>{subtitlePreprocessStatus.status === 'pending' ? '字幕预处理排队中...' : (subtitlePreprocessStatus.message || '字幕预处理中...')}</span></div>
                          {subtitlePreprocessStatus.progress > 0 && (
                            <div className="mt-2 h-1 overflow-hidden rounded-full bg-[var(--nv-player-surface-hover)]"><div className="h-full rounded-full bg-[var(--nv-player-warning)] transition-[width] duration-500" style={{ width: `${subtitlePreprocessStatus.progress}%` }} /></div>
                          )}
                        </div>
                      ) : null}
                    </>
                  )}

                  {(translatedSubs.length > 0 || aiSubtitleStatus?.status === 'completed' || translating) && (
                    <>
                      <div className={PLAYER_DIVIDER} />
                      <div className={PLAYER_MENU_LABEL}><Languages size={10} className="mr-1 inline" aria-hidden="true" />字幕翻译</div>
                      {translatedSubs.map((sub) => {
                        const langNames: Record<string, string> = {
                          zh: '中文', en: '英文', ja: '日文', ko: '韩文', fr: '法文', de: '德文', es: '西班牙文', pt: '葡萄牙文', ru: '俄文', it: '意大利文', ar: '阿拉伯文', th: '泰文',
                        }
                        return (
                          <button key={sub.language} type="button" onClick={() => { loadSubtitle('translated', sub.language); setShowSubtitleMenu(false) }} className={menuItemClass(activeSubtitle === `translated:${sub.language}`)}>
                            <Languages size={12} aria-hidden="true" /><span className="flex-1">{langNames[sub.language] || sub.language}</span><span className="text-xs text-[var(--nv-player-success)]">✓</span>
                          </button>
                        )
                      })}

                      {translating && translateStatus && (
                        <div className="px-3 py-2.5 text-sm text-[var(--nv-player-text-tertiary)]">
                          <div className="flex items-center gap-2"><Loader2 size={14} className="animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" /><span>{translateStatus.message || '正在翻译...'}</span></div>
                          {translateStatus.progress > 0 && (
                            <div className="mt-2 h-1 overflow-hidden rounded-full bg-[var(--nv-player-surface-hover)]"><div className="h-full rounded-full bg-[var(--nv-player-accent)] transition-[width] duration-500" style={{ width: `${translateStatus.progress}%` }} /></div>
                          )}
                        </div>
                      )}

                      {!translating && aiSubtitleStatus?.status === 'completed' && (
                        <div className="relative">
                          <button type="button" onClick={() => setShowTranslateMenu(!showTranslateMenu)} className={PLAYER_MENU_ITEM} aria-expanded={showTranslateMenu}><Languages size={12} aria-hidden="true" />翻译为其他语言...</button>
                          {showTranslateMenu && (
                            <div className="mx-2 mb-1 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border-subtle)] bg-[var(--nv-player-surface-soft)] p-1">
                              {[
                                { code: 'zh', name: '中文' }, { code: 'en', name: '英文' }, { code: 'ja', name: '日文' }, { code: 'ko', name: '韩文' },
                                { code: 'fr', name: '法文' }, { code: 'de', name: '德文' }, { code: 'es', name: '西班牙文' }, { code: 'ru', name: '俄文' },
                              ].filter(lang => !translatedSubs.some(sub => sub.language === lang.code)).map((lang) => (
                                <button
                                  key={lang.code}
                                  type="button"
                                  onClick={() => {
                                    setTranslating(true)
                                    setShowTranslateMenu(false)
                                    subtitleApi.translate(mediaId, lang.code).catch(() => setTranslating(false))
                                  }}
                                  className={clsx(PLAYER_MENU_ITEM, 'min-h-9 text-xs')}
                                >
                                  {lang.name}
                                </button>
                              ))}
                            </div>
                          )}
                        </div>
                      )}
                    </>
                  )}

                  <div className={PLAYER_DIVIDER} />
                  <button type="button" onClick={() => { setShowSubtitleSearch(true); setShowContentSearch(false); setShowSubtitleMenu(false) }} className={PLAYER_MENU_ITEM}><Search size={12} className="text-[var(--nv-player-accent)]" aria-hidden="true" />在线搜索字幕...</button>
                  <button type="button" onClick={() => { setShowContentSearch(true); setShowSubtitleSearch(false); setShowSubtitleMenu(false) }} className={PLAYER_MENU_ITEM}><Search size={12} aria-hidden="true" /><span className="flex-1">搜索当前字幕内容...</span><span className="text-[10px] text-[var(--nv-player-text-faint)]">Ctrl+F</span></button>
                </div>
              )}
            </div>
          )}

          {!isStrm && (
            <div className="relative">
              <button
                type="button"
                onClick={() => {
                  setShowSubtitleSearch(true)
                  setShowContentSearch(false)
                  setShowQuality(false)
                  setShowSubtitleMenu(false)
                  setShowCastPanel(false)
                  setShowSpeedMenu(false)
                }}
                className={clsx(PLAYER_CONTROL_CLASS, showSubtitleSearch && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')}
                title="在线字幕搜索"
                aria-pressed={showSubtitleSearch}
              >
                <Search size={18} aria-hidden="true" />
              </button>
              {showContentSearch && <SubtitleContentSearch videoRef={videoRef} onClose={() => setShowContentSearch(false)} hasActiveSubtitle={!!activeSubtitle} />}
            </div>
          )}

          <div className="relative">
            <button type="button" onClick={() => openExclusive('cast')} className={clsx(PLAYER_CONTROL_CLASS, showCastPanel && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')} title="投屏" aria-expanded={showCastPanel}>
              <Monitor size={18} aria-hidden="true" />
            </button>
            {showCastPanel && <CastPanel mediaId={mediaId} mediaTitle={title} onClose={() => setShowCastPanel(false)} />}
          </div>

          {audioTracks.length > 1 && (
            <div className="relative">
              <button type="button" onClick={() => openExclusive('audio')} className={clsx(PLAYER_CONTROL_CLASS, showAudioMenu && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')} title="音轨" aria-expanded={showAudioMenu}><Languages size={18} aria-hidden="true" /></button>
              {showAudioMenu && (
                <div className={clsx(PLAYER_MENU_CLASS, 'min-w-[180px]')} role="menu">
                  <div className={PLAYER_MENU_LABEL}>音轨</div>
                  {audioTracks.map((track) => (
                    <button key={track.id} type="button" onClick={() => switchAudioTrack(track.id)} className={menuItemClass(track.id === currentAudioTrack)} role="menuitemradio" aria-checked={track.id === currentAudioTrack}>
                      {track.name}{track.lang ? ` (${track.lang})` : ''}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}

          {qualities.length > 1 && (
            <div className="relative">
              <button type="button" onClick={() => openExclusive('quality')} className={clsx(PLAYER_CONTROL_CLASS, showQuality && 'border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]')} title="画质" aria-expanded={showQuality}><Settings size={18} aria-hidden="true" /></button>
              {showQuality && (
                <div className={clsx(PLAYER_MENU_CLASS, 'min-w-[240px]')} role="menu">
                  <div className={PLAYER_MENU_LABEL}>画质</div>
                  {qualities.map((quality) => (
                    <button key={quality.index} type="button" onClick={() => switchQuality(quality.index)} className={menuItemClass(quality.index === currentQuality)} role="menuitemradio" aria-checked={quality.index === currentQuality}>
                      <span className="flex-1">{quality.label}</span>
                      {quality.bitrate ? <span className="text-[11px] text-[var(--nv-player-text-faint)]">{(quality.bitrate / 1_000_000).toFixed(1)} Mbps</span> : null}
                    </button>
                  ))}
                  {mode !== 'direct' && mode !== 'remux' && mode !== 'smart_remux' && (
                    <div className="mt-2 border-t border-[var(--nv-player-border-subtle)] px-3 py-2.5 text-[11px] leading-relaxed text-[var(--nv-player-text-tertiary)]">
                      <div className="mb-1 font-medium text-[var(--nv-player-text-secondary)]">实时状态</div>
                      {currentBitrate > 0 && <div className="flex justify-between gap-4"><span>当前码率</span><span className="text-[var(--nv-player-text-primary)]">{(currentBitrate / 1_000_000).toFixed(2)} Mbps</span></div>}
                      {bandwidthEstimate > 0 && <div className="flex justify-between gap-4"><span>带宽评估</span><span className="text-[var(--nv-player-text-primary)]">{(bandwidthEstimate / 1_000_000).toFixed(2)} Mbps</span></div>}
                    </div>
                  )}
                </div>
              )}
            </div>
          )}

          <button
            type="button"
            onClick={() => {
              const video = videoRef.current
              if (!video) return
              if (document.pictureInPictureElement) document.exitPictureInPicture().catch(() => {})
              else video.requestPictureInPicture().catch(() => {})
            }}
            className={PLAYER_CONTROL_CLASS}
            title="画中画"
            aria-label="画中画"
          >
            <PictureInPicture2 size={18} aria-hidden="true" />
          </button>

          <button type="button" onClick={toggleFullscreen} className={PLAYER_CONTROL_CLASS} aria-label={isFullscreen ? '退出全屏' : '进入全屏'}>
            {isFullscreen ? <Minimize size={18} aria-hidden="true" /> : <Maximize size={18} aria-hidden="true" />}
          </button>
        </div>
      </div>

      {(showQuality || showSubtitleMenu || showCastPanel || showSpeedMenu || showContentSearch || showAudioMenu) && (
        <button type="button" className="absolute inset-0 z-[-1]" onClick={closeAllMenus} aria-label="关闭播放器菜单" />
      )}

      {showSubtitleSearch && (
        <SubtitleSearchPanel
          mediaId={mediaId}
          title={title}
          onClose={() => setShowSubtitleSearch(false)}
          onDownloaded={() => {
            subtitleApi.getTracks(mediaId).then((res) => {
              const info = res.data.data
              if (info) setExternalSubs(info.external || [])
            }).catch(() => {})
          }}
        />
      )}
    </div>
  )
}
