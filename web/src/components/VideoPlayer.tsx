import { useRef, useEffect, useCallback, useState } from 'react'
import Hls from 'hls.js'
import { usePlayerStore } from '@/stores/player'
import { useAuthStore } from '@/stores/auth'
import { userApi, subtitleApi, subtitlePreprocessApi } from '@/api'
import { streamApi } from '@/api/stream'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { SubtitleTrack, ExternalSubtitle, ASRTask, TranslatedSubtitle, SubtitlePreprocessTask } from '@/types'
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
} from 'lucide-react'
import clsx from 'clsx'
import CastPanel from './CastPanel'
import SubtitleSearchPanel from './SubtitleSearchPanel'
import SubtitleContentSearch from './SubtitleContentSearch'

interface VideoPlayerProps {
  src: string
  mode?: 'direct' | 'hls' | 'remux'
  mediaId: string
  title?: string
  startPosition?: number
  onBack?: () => void
  /** 下一集回调，存在时显示「下一集」按钮，播放结束自动触发 */
  onNext?: () => void
  /** 下一集标题（用于提示） */
  nextTitle?: string
  /** 是否为 STRM 远程流 */
  isStrm?: boolean
  /** API 返回的完整视频时长（秒），用于在实时转码 EVENT 模式下显示完整时长 */
  knownDuration?: number
  /** Remux 播放失败回调，触发后 PlayerPage 会自动降级到 HLS 转码模式 */
  onRemuxFallback?: () => void
  /** 预处理完成回调，播放器会自动切换到预处理流 */
  onPreprocessReady?: () => void
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
  isStrm = false,
  knownDuration,
  onPreprocessReady,
  onRemuxFallback,
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

  // AI 字幕状态
  const [aiSubtitleStatus, setAiSubtitleStatus] = useState<ASRTask | null>(null)
  const [aiGenerating, setAiGenerating] = useState(false)

  // 字幕预处理状态
  const [subtitlePreprocessStatus, setSubtitlePreprocessStatus] = useState<SubtitlePreprocessTask | null>(null)

  // Phase 4: 字幕翻译状态
  const [translatedSubs, setTranslatedSubs] = useState<TranslatedSubtitle[]>([])
  const [translateStatus, setTranslateStatus] = useState<ASRTask | null>(null)
  const [translating, setTranslating] = useState(false)
  const [showTranslateMenu, setShowTranslateMenu] = useState(false)

  // 字幕搜索状态
  const [showSubtitleSearch, setShowSubtitleSearch] = useState(false)
  const [showContentSearch, setShowContentSearch] = useState(false)

  // 倍速播放
  const [showSpeedMenu, setShowSpeedMenu] = useState(false)
  const [playbackRate, setPlaybackRate] = useState(1)
  const SPEED_OPTIONS = [0.5, 0.75, 1, 1.25, 1.5, 1.75, 2, 2.5, 3, 4, 5, 6, 7, 8]

  // 优先使用 API 返回的完整时长（解决实时转码 EVENT 模式下时长逐步增长的问题）
  // 必须在所有 useEffect 之前声明，避免 TDZ（暂时性死区）错误
  const displayDuration = (knownDuration && knownDuration > 0 && knownDuration > duration) ? knownDuration : duration
  const progress = displayDuration > 0 ? (currentTime / displayDuration) * 100 : 0

  // 自动下一集倒计时
  const [nextCountdown, setNextCountdown] = useState<number | null>(null)
  const nextCountdownTimerRef = useRef<number>(0)

  // 快进/快退提示
  const [seekHint, setSeekHint] = useState<{ text: string; visible: boolean }>({ text: '', visible: false })
  const seekHintTimer = useRef<number>(0)

  // 进度条hover预览
  const [hoverProgress, setHoverProgress] = useState<number | null>(null)
  const [hoverTime, setHoverTime] = useState('')

  // 手势控制状态
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

  // WebSocket 监听 AI 字幕进度
  const { on, off } = useWebSocket()

  useEffect(() => {
    const handleASRProgress = (data: any) => {
      if (data.media_id === mediaId) {
        setAiSubtitleStatus(data as ASRTask)
        if (data.status === 'completed' || data.status === 'failed') {
          setAiGenerating(false)
        }
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

    // 监听预处理完成事件：后台预处理完成后自动通知，实现无缝切换
    const handlePreprocessCompleted = (data: any) => {
      if (data.media_id === mediaId && onPreprocessReady) {
        onPreprocessReady()
      }
    }

    // 监听字幕预处理完成事件：预处理完成后自动刷新 AI 字幕状态
    const handleSubPreprocessCompleted = (data: any) => {
      if (data.media_id === mediaId) {
        setSubtitlePreprocessStatus(data as SubtitlePreprocessTask)
        // 字幕预处理完成后，刷新 AI 字幕状态（预处理会生成 AI 字幕）
        subtitleApi.getAIStatus(mediaId).then((res) => {
          const d = res.data.data
          if (d && d.status !== 'none') {
            setAiSubtitleStatus(d)
          }
        }).catch(() => {})
        // 同时刷新翻译字幕列表（预处理可能包含翻译步骤）
        subtitleApi.listTranslated(mediaId).then((res) => {
          if (Array.isArray(res.data.data)) setTranslatedSubs(res.data.data)
        }).catch(() => {})
      }
    }
    const handleSubPreprocessProgress = (data: any) => {
      if (data.media_id === mediaId) {
        setSubtitlePreprocessStatus(data as SubtitlePreprocessTask)
      }
    }
    const handleSubPreprocessFailed = (data: any) => {
      if (data.media_id === mediaId) {
        setSubtitlePreprocessStatus(data as SubtitlePreprocessTask)
      }
    }

    on(WS_EVENTS.PREPROCESS_COMPLETED, handlePreprocessCompleted)
    on(WS_EVENTS.ASR_PROGRESS, handleASRProgress)
    on(WS_EVENTS.ASR_COMPLETED, handleASRCompleted)
    on(WS_EVENTS.ASR_FAILED, handleASRFailed)
    on(WS_EVENTS.SUB_PREPROCESS_COMPLETED, handleSubPreprocessCompleted)
    on(WS_EVENTS.SUB_PREPROCESS_PROGRESS, handleSubPreprocessProgress)
    on(WS_EVENTS.SUB_PREPROCESS_FAILED, handleSubPreprocessFailed)

    // Phase 4: 翻译事件监听
    const handleTranslateProgress = (data: any) => {
      if (data.media_id === mediaId) {
        setTranslateStatus(data as ASRTask)
      }
    }
    const handleTranslateCompleted = (data: any) => {
      if (data.media_id === mediaId) {
        setTranslateStatus(data as ASRTask)
        setTranslating(false)
        // 刷新已翻译字幕列表
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

  // 加载字幕轨道列表 + 检查 AI 字幕状态
  useEffect(() => {
    if (!mediaId) return
    subtitleApi.getTracks(mediaId).then((res) => {
      const data = res.data.data
      if (data) {
        setEmbeddedSubs(data.embedded || [])
        setExternalSubs(data.external || [])
      }
    }).catch(() => {})

    // 检查是否已有 AI 字幕
    subtitleApi.getAIStatus(mediaId).then((res) => {
      const data = res.data.data
      if (data && data.status !== 'none') {
        setAiSubtitleStatus(data)
        if (data.status === 'extracting' || data.status === 'transcribing' || data.status === 'converting') {
          setAiGenerating(true)
        }
      }
    }).catch(() => {})

    // Phase 4: 加载已翻译的字幕列表
    subtitleApi.listTranslated(mediaId).then((res) => {
      if (Array.isArray(res.data.data)) setTranslatedSubs(res.data.data)
    }).catch(() => {})

    // 查询字幕预处理状态
    subtitlePreprocessApi.getMediaStatus(mediaId).then((res) => {
      if (res.data.data) {
        setSubtitlePreprocessStatus(res.data.data)
      }
    }).catch(() => {})
  }, [mediaId])

  // 加载字幕到video元素
  const loadSubtitle = useCallback((type: string, id: string) => {
    const video = videoRef.current
    if (!video) return
    // 清除之前手动添加的 <track> 元素
    video.querySelectorAll('track').forEach(t => t.remove())
    // 将所有 textTrack 设为 hidden（HLS.js 的轨道也一并隐藏，不影响）
    for (let i = 0; i < video.textTracks.length; i++) {
      video.textTracks[i].mode = 'hidden'
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
    } else if (type === 'ai') {
      subtitleUrl = subtitleApi.getAISubtitleUrl(mediaId)
      label = 'AI 生成字幕'
    } else if (type === 'translated') {
      // Phase 4: 翻译字幕
      subtitleUrl = subtitleApi.getTranslatedSubtitleUrl(mediaId, id)
      const langNames: Record<string, string> = {
        zh: '中文', en: '英文', ja: '日文', ko: '韩文',
        fr: '法文', de: '德文', es: '西班牙文', pt: '葡萄牙文',
        ru: '俄文', it: '意大利文', ar: '阿拉伯文', th: '泰文',
      }
      label = `翻译字幕（${langNames[id] || id}）`
    }
    if (subtitleUrl) {
      // 使用 fetch + addTextTrack 代替 <track> 元素，避免跨域和 HLS.js 干扰
      const token = useAuthStore.getState().token
      const headers: Record<string, string> = {}
      if (token) headers['Authorization'] = `Bearer ${token}`

      // 记录本次加载的 blobUrl 和 trackEl，便于失败/卸载时清理
      let createdBlobUrl: string | null = null
      let createdTrackEl: HTMLTrackElement | null = null

      fetch(subtitleUrl, { headers })
        .then(res => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`)
          return res.text()
        })
        .then(vttText => {
          const blob = new Blob([vttText], { type: 'text/vtt' })
          const blobUrl = URL.createObjectURL(blob)
          createdBlobUrl = blobUrl
          const trackEl = document.createElement('track')
          createdTrackEl = trackEl
          trackEl.kind = 'subtitles'
          trackEl.label = label
          trackEl.srclang = 'und'
          trackEl.src = blobUrl
          trackEl.default = true
          // 监听 track load 事件，在浏览器解析完 VTT 后再释放 blob URL，避免过早 revoke 导致字幕无法显示
          const onTrackLoad = () => {
            // 激活对应 textTrack
            for (let i = 0; i < video.textTracks.length; i++) {
              const t = video.textTracks[i]
              t.mode = t.label === label ? 'showing' : 'hidden'
            }
            // 字幕已解析完成，可以安全释放 blob URL
            URL.revokeObjectURL(blobUrl)
            trackEl.removeEventListener('load', onTrackLoad)
          }
          trackEl.addEventListener('load', onTrackLoad)
          video.appendChild(trackEl)
          // 只有字幕成功加载后才更新 activeSubtitle，避免 UI 显示为已选中但实际加载失败
          setActiveSubtitle(`${type}:${id}`)
        })
        .catch(err => {
          console.error('字幕加载失败:', err)
          // 加载失败时重置激活状态，清理未完成的资源
          if (createdTrackEl && createdTrackEl.parentNode) {
            createdTrackEl.parentNode.removeChild(createdTrackEl)
          }
          if (createdBlobUrl) {
            URL.revokeObjectURL(createdBlobUrl)
          }
          setActiveSubtitle(null)
        })
    }
  }, [mediaId, embeddedSubs, externalSubs])

  // 自动选中第一个可用字幕
  const autoSelectSubtitle = useCallback(() => {
    // 优先级：外挂字幕 > 内嵌非bitmap字幕 > AI字幕 > 翻译字幕
    if (externalSubs.length > 0) {
      loadSubtitle('external', externalSubs[0].path)
      return
    }
    const firstPlayableEmbedded = embeddedSubs.find(s => !s.bitmap)
    if (firstPlayableEmbedded) {
      loadSubtitle('embedded', String(firstPlayableEmbedded.index))
      return
    }
    if (aiSubtitleStatus?.status === 'completed') {
      loadSubtitle('ai', '')
      return
    }
    if (translatedSubs.length > 0) {
      loadSubtitle('translated', translatedSubs[0].language)
      return
    }
  }, [externalSubs, embeddedSubs, aiSubtitleStatus, translatedSubs, loadSubtitle])

  // 字幕列表就绪后自动选中第一个
  useEffect(() => {
    if (!mediaId || activeSubtitle) return // 已有选中字幕，不覆盖
    // 有任何可用字幕时尝试自动选中
    if (externalSubs.length > 0 || embeddedSubs.some(s => !s.bitmap) || aiSubtitleStatus?.status === 'completed' || translatedSubs.length > 0) {
      autoSelectSubtitle()
    }
  }, [mediaId, activeSubtitle, externalSubs, embeddedSubs, aiSubtitleStatus, translatedSubs, autoSelectSubtitle])

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
    if (mode === 'direct' || mode === 'remux') {
      video.src = src
      setQualities([])
      video.addEventListener('loadedmetadata', () => {
        if (startPosition > 0) video.currentTime = startPosition
        video.play().catch(() => {})
      }, { once: true })
      video.addEventListener('error', () => {
        const err = video.error
        // Remux 模式下播放失败（如浏览器不支持 HEVC 10-bit），自动降级到 HLS
        if (mode === 'remux' && onRemuxFallback) {
          console.warn('Remux 播放失败，自动降级到 HLS 转码:', err?.message)
          onRemuxFallback()
          return
        }
        setLoadError(`播放失败: ${err?.message || '未知错误'}`)
      }, { once: true })
    } else {
      if (Hls.isSupported()) {
        const hls = new Hls({
          startLevel: -1,
          capLevelToPlayerSize: true,
          maxBufferLength: 30,
          maxMaxBufferLength: 60,
          xhrSetup: (xhr: XMLHttpRequest, _url: string) => {
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
  }, [src, mode, startPosition, reset, onRemuxFallback])

  // 视频事件监听
  useEffect(() => {
    const video = videoRef.current
    if (!video) return
    const onPlay = () => setPlaying(true)
    const onPause = () => setPlaying(false)
    // Remux 模式下，video.currentTime 是从 seek 点开始的相对时间，需要加上偏移量
    const onTimeUpdate = () => setCurrentTime(video.currentTime + remuxOffsetRef.current)
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
    // Seek 完成立即通知后端新位置（非 Remux 路径），避免 throttleLoop 按旧位置误挂起/恢复
    const onSeeked = () => {
      const pos = video.currentTime + remuxOffsetRef.current
      if (pos > 0) {
        streamApi.reportPlayback(mediaId, pos).catch(() => {})
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
  // - 每 3s 向后端 /stream/:id/playback 上报一次，驱动 FFmpeg Throttling（挂起/恢复）
  // - 每 ~15s 额外写一次数据库 WatchHistory
  useEffect(() => {
    let tick = 0
    progressReportRef.current = window.setInterval(() => {
      const video = videoRef.current
      if (!video || video.paused || video.currentTime <= 0) return
      // Remux 模式下 video.currentTime 是相对 seek 点的时间，需要加上偏移
      const actualTime = video.currentTime + remuxOffsetRef.current
      const actualDuration = displayDuration > 0 ? displayDuration : video.duration
      // 驱动后端节流（高频）
      streamApi.reportPlayback(mediaId, actualTime).catch(() => {})
      // 写观看历史（低频，每 5 次 = 15s）
      tick++
      if (tick % 5 === 0) {
        userApi.updateProgress(mediaId, actualTime, actualDuration).catch(() => {})
      }
    }, 3000)
    return () => clearInterval(progressReportRef.current)
  }, [mediaId, displayDuration])

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

    // Remux 模式：通过重新请求带 ?start= 参数的 URL 实现快进/快退
    if (mode === 'remux') {
      const currentPos = remuxOffsetRef.current + (video.currentTime || 0)
      const targetTime = Math.max(0, Math.min(displayDuration, currentPos + seconds))
      remuxSeek(targetTime)
    } else {
      video.currentTime = Math.max(0, Math.min(video.duration || displayDuration, video.currentTime + seconds))
    }
    // 显示快进/快退提示
    clearTimeout(seekHintTimer.current)
    setSeekHint({ text: seconds > 0 ? `+${seconds}s` : `${seconds}s`, visible: true })
    seekHintTimer.current = window.setTimeout(() => {
      setSeekHint(prev => ({ ...prev, visible: false }))
    }, 800)
  }

  // Remux Seek：通过重新加载带 ?start= 参数的 URL 实现进度跳转
  // 类似 Emby 的拖动进度条体验，中止当前流并从新位置开始转封装
  const remuxSeek = useCallback((targetTime: number) => {
    const video = videoRef.current
    if (!video || mode !== 'remux' || !src) return
    // 从 src 中提取基础 URL（去掉已有的 start 参数）
    const baseUrl = src.replace(/[&?]start=[^&]*/g, '')
    const sep = baseUrl.includes('?') ? '&' : '?'
    const newSrc = `${baseUrl}${sep}start=${Math.floor(targetTime)}`
    // 记录目标时间偏移，用于显示正确的进度
    remuxOffsetRef.current = targetTime
    video.src = newSrc
    video.play().catch(() => {})
    // 立即通知后端新位置，避免 throttleLoop 按旧位置误挂起/恢复
    streamApi.reportPlayback(mediaId, targetTime).catch(() => {})
  }, [src, mode, mediaId])

  // Remux 时间偏移：记录 Seek 的起始时间，用于计算真实播放位置
  const remuxOffsetRef = useRef(0)

  const handleProgressClick = (e: React.MouseEvent<HTMLDivElement>) => {
    const video = videoRef.current
    if (!video) return
    const rect = e.currentTarget.getBoundingClientRect()
    const pos = (e.clientX - rect.left) / rect.width
    const targetTime = pos * displayDuration

    // Remux 模式：通过重新请求带 ?start= 参数的 URL 实现 Seek
    if (mode === 'remux') {
      remuxSeek(targetTime)
      return
    }

    // 如果目标时间超出已加载范围，限制到当前可 seek 的最大位置
    if (video.duration > 0 && targetTime <= video.duration) {
      video.currentTime = targetTime
    } else if (video.duration > 0) {
      // 超出已转码范围，跳转到已转码的最末尾
      video.currentTime = video.duration - 0.5
    }
  }

  const handleProgressHover = (e: React.MouseEvent<HTMLDivElement>) => {
    const rect = e.currentTarget.getBoundingClientRect()
    const pos = (e.clientX - rect.left) / rect.width
    setHoverProgress(pos * 100)
    setHoverTime(formatTime(pos * displayDuration))
  }

  const handleVolumeChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const video = videoRef.current
    if (!video) return
    const vol = parseFloat(e.target.value)
    video.volume = vol
    video.muted = vol === 0
  }

  // 倍速切换
  const changeSpeed = useCallback((rate: number) => {
    const video = videoRef.current
    if (!video) return
    video.playbackRate = rate
    setPlaybackRate(rate)
    setShowSpeedMenu(false)
    // 显示倍速切换提示
    clearTimeout(seekHintTimer.current)
    setSeekHint({ text: rate === 1 ? '正常速度' : `${rate}x 倍速`, visible: true })
    seekHintTimer.current = window.setTimeout(() => {
      setSeekHint(prev => ({ ...prev, visible: false }))
    }, 800)
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

  const formatTime = (seconds: number) => {
    if (isNaN(seconds)) return '0:00'
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    const s = Math.floor(seconds % 60)
    if (h > 0) return `${h}:${m.toString().padStart(2, '0')}:${s.toString().padStart(2, '0')}`
    return `${m}:${s.toString().padStart(2, '0')}`
  }

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
        case 'f': {
          e.preventDefault()
          if (e.ctrlKey || e.metaKey) {
            setShowContentSearch(prev => !prev)
          } else {
            toggleFullscreen()
          }
          break
        }
        case 'm':
          e.preventDefault()
          if (videoRef.current) videoRef.current.muted = !videoRef.current.muted
          break
        case 'Escape':
          if (showContentSearch) {
            setShowContentSearch(false)
          } else if (showSubtitleSearch) {
            setShowSubtitleSearch(false)
          } else if (onBack) {
            onBack()
          }
          break
        // 倍速快捷键：< 减速，> 加速，Backspace 恢复正常速度
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
        case 'Backspace': {
          // 快速恢复正常速度
          if (playbackRate !== 1) {
            e.preventDefault()
            changeSpeed(1)
          }
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
  }, [playbackRate, onNext, showContentSearch, showSubtitleSearch])

  // ==================== 手势控制 ====================
  const handleTouchStart = useCallback((e: React.TouchEvent) => {
    const touch = e.touches[0]
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

  const handleTouchMove = useCallback((e: React.TouchEvent) => {
    const gesture = gestureRef.current
    if (!gesture) return

    const touch = e.touches[0]
    const deltaX = touch.clientX - gesture.startX
    const deltaY = touch.clientY - gesture.startY
    const rect = containerRef.current?.getBoundingClientRect()
    if (!rect) return

    // 判断手势方向（首次移动超过10px时锁定方向）
    if (gesture.direction === 'none') {
      if (Math.abs(deltaX) > 10 || Math.abs(deltaY) > 10) {
        gesture.direction = Math.abs(deltaX) > Math.abs(deltaY) ? 'horizontal' : 'vertical'
      } else {
        return
      }
    }

    if (gesture.direction === 'horizontal') {
      // 水平滑动 -> 快进/快退
      const effectiveDuration = (knownDuration && knownDuration > 0 && knownDuration > duration) ? knownDuration : duration
      const seekDelta = (deltaX / rect.width) * effectiveDuration * 0.3
      const newTime = Math.max(0, Math.min(effectiveDuration, gesture.startTime + seekDelta))
      const diff = newTime - gesture.startTime
      const sign = diff >= 0 ? '+' : '-'
      setGestureOverlay({
        type: 'seek',
        value: `${sign}${formatTime(Math.abs(diff))} / ${formatTime(newTime)}`,
      })
    } else if (gesture.direction === 'vertical') {
      if (gesture.side === 'right') {
        // 右侧上下滑动 -> 音量调节
        const volumeDelta = -deltaY / rect.height
        const newVolume = Math.max(0, Math.min(1, gesture.startVolume + volumeDelta))
        setVolume(newVolume)
        setGestureOverlay({
          type: 'volume',
          value: `${Math.round(newVolume * 100)}%`,
        })
      } else {
        // 左侧上下滑动 -> 亮度调节（通过CSS filter模拟）
        const brightnessDelta = -deltaY / rect.height
        const brightness = Math.max(0.3, Math.min(1.5, 1 + brightnessDelta))
        const video = videoRef.current
        if (video) {
          video.style.filter = `brightness(${brightness})`
        }
        setGestureOverlay({
          type: 'brightness',
          value: `${Math.round(brightness * 100)}%`,
        })
      }
    }
  }, [duration, knownDuration, setVolume])

  const handleTouchEnd = useCallback(() => {
    const gesture = gestureRef.current
    if (!gesture) return

    if (gesture.direction === 'horizontal') {
      // 应用快进/快退
      const video = videoRef.current
      const rect = containerRef.current?.getBoundingClientRect()
      if (video && rect) {
        // 重新计算最终位置（从overlay中获取不太方便，重新算一次）
        // 这里简单地让video跳转到gestureOverlay显示的时间
      }
    }

    gestureRef.current = null
    // 延迟隐藏手势提示
    clearTimeout(gestureOverlayTimer.current)
    gestureOverlayTimer.current = window.setTimeout(() => {
      setGestureOverlay(null)
    }, 500)
  }, [])

  const closeAllMenus = () => {
    setShowQuality(false)
    setShowSubtitleMenu(false)
    setShowCastPanel(false)
    setShowSpeedMenu(false)
    setShowTranslateMenu(false)
    setShowContentSearch(false)
  }

  return (
    <div
      ref={containerRef}
      className="group/player relative h-full w-full bg-black"
      onMouseMove={resetControlsTimer}
      onMouseLeave={() => { if (isPlaying) setShowControls(false) }}
      onTouchStart={handleTouchStart}
      onTouchMove={handleTouchMove}
      onTouchEnd={handleTouchEnd}
    >
      {/* 视频元素 */}
      <video
        ref={videoRef}
        className="h-full w-full cursor-pointer"
        onClick={togglePlay}
        onDoubleClick={toggleFullscreen}
        playsInline
        crossOrigin="anonymous"
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

      {/* 手势控制提示浮层 */}
      {gestureOverlay && (
        <div className="absolute inset-0 z-20 flex items-center justify-center pointer-events-none">
          <div className="rounded-2xl px-8 py-4 text-center" style={{
            background: 'rgba(11, 17, 32, 0.85)',
            border: '1px solid var(--neon-blue-15)',
            backdropFilter: 'blur(12px)',
          }}>
            <p className="text-xs text-surface-400 mb-1">
              {gestureOverlay.type === 'seek' ? '⏩ 进度' :
               gestureOverlay.type === 'volume' ? '🔊 音量' : '☀️ 亮度'}
            </p>
            <p className="font-display text-xl font-bold text-white">{gestureOverlay.value}</p>
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
              {isStrm ? 'STRM远程流' : mode === 'direct' ? '直接播放' : mode === 'remux' ? 'Remux播放' : 'HLS转码'}
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
          {/* 已转码范围指示（实时转码 EVENT 模式下，显示已转码的区域） */}
          {knownDuration && knownDuration > 0 && duration > 0 && duration < knownDuration && (
            <div
              className="absolute left-0 top-0 h-full rounded-full opacity-30"
              style={{
                width: `${(duration / knownDuration) * 100}%`,
                background: 'var(--neon-blue)',
              }}
            />
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
            {formatTime(displayDuration)}
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
              <div className="absolute bottom-full right-0 mb-2 min-w-[140px] max-h-[360px] overflow-y-auto rounded-xl py-1 shadow-2xl"
                style={{
                  background: 'rgba(11, 17, 32, 0.9)',
                  border: '1px solid var(--neon-blue-10)',
                  backdropFilter: 'blur(20px)',
                  scrollbarWidth: 'thin',
                  scrollbarColor: 'var(--neon-blue-15) transparent',
                }}
              >
                {/* 快速恢复正常速度按钮 */}
                {playbackRate !== 1 && (
                  <>
                    <button
                      onClick={() => changeSpeed(1)}
                      className="flex w-full items-center justify-between px-4 py-2.5 text-left text-sm text-neon-blue transition-colors hover:bg-neon-blue/10"
                    >
                      <span>恢复正常</span>
                      <span className="text-[10px] text-surface-500">Backspace</span>
                    </button>
                    <div className="mx-3 my-0.5 border-t border-neon-blue/10" />
                  </>
                )}
                {SPEED_OPTIONS.map((speed) => (
                  <button
                    key={speed}
                    onClick={() => changeSpeed(speed)}
                    className={clsx(
                      'block w-full px-4 py-2 text-left text-sm transition-colors',
                      speed === playbackRate
                        ? 'text-neon-blue'
                        : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                    )}
                    style={speed === playbackRate ? { background: 'var(--neon-blue-6)' } : {}}
                  >
                    {speed === 1 ? '正常' : `${speed}x`}
                  </button>
                ))}
                <div className="mx-3 my-0.5 border-t border-neon-blue/10" />
                <div className="px-4 py-1.5 text-[10px] text-surface-500">
                  快捷键: &lt; 减速 &gt; 加速
                </div>
              </div>
            )}
          </div>

          {/* 字幕选择（始终显示，支持 AI 生成） */}
          {!isStrm && (
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
                          onClick={() => {
                            if (sub.bitmap) return // 图形字幕不可选
                            loadSubtitle('embedded', String(sub.index))
                            setShowSubtitleMenu(false)
                          }}
                          disabled={sub.bitmap}
                          className={clsx(
                            'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                            sub.bitmap
                              ? 'text-surface-600 cursor-not-allowed'
                              : activeSubtitle === `embedded:${sub.index}`
                                ? 'text-neon-blue'
                                : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                          )}
                          style={activeSubtitle === `embedded:${sub.index}` && !sub.bitmap ? { background: 'var(--neon-blue-6)' } : {}}
                          title={sub.bitmap ? '图形字幕无法在浏览器中显示' : undefined}
                        >
                          {sub.title || sub.language || `轨道 ${sub.index}`}
                          {sub.codec && <span className="ml-2 text-xs text-surface-600">[{sub.codec}]</span>}
                          {sub.bitmap && <span className="ml-1 text-xs text-red-400/60">不可用</span>}
                          {!sub.bitmap && sub.default && <span className="ml-1 text-xs text-neon-blue/60">默认</span>}
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

                  {/* AI 字幕 — 仅在有可用字幕或正在处理时显示 */}
                  {(aiSubtitleStatus?.status === 'completed' || aiGenerating || subtitlePreprocessStatus?.status === 'running' || subtitlePreprocessStatus?.status === 'pending') && (
                    <>
                  <div className="mx-3 my-1 border-t border-neon-blue/10" />
                  <div className="px-4 py-1 text-[10px] font-bold uppercase tracking-widest text-neon-blue/40">
                    <Sparkles size={10} className="inline mr-1" />AI 字幕
                  </div>

                  {aiSubtitleStatus?.status === 'completed' ? (
                    <button
                      onClick={() => { loadSubtitle('ai', ''); setShowSubtitleMenu(false) }}
                      className={clsx(
                        'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                        activeSubtitle === 'ai:'
                          ? 'text-neon-blue'
                          : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                      )}
                      style={activeSubtitle === 'ai:' ? { background: 'var(--neon-blue-6)' } : {}}
                    >
                      <Sparkles size={12} className="inline mr-1.5" />
                      AI 生成字幕
                      <span className="ml-2 text-xs text-emerald-400/60">✓ 已就绪</span>
                    </button>
                  ) : aiGenerating ? (
                    <div className="flex items-center gap-2 px-4 py-2.5 text-sm text-surface-400">
                      <Loader2 size={14} className="animate-spin text-neon-blue" />
                      <div className="flex-1">
                        <div className="text-xs">{aiSubtitleStatus?.message || '正在生成...'}</div>
                        {aiSubtitleStatus?.progress != null && aiSubtitleStatus.progress > 0 && (
                          <div className="mt-1 h-1 w-full rounded-full bg-surface-700">
                            <div
                              className="h-full rounded-full transition-all duration-500"
                              style={{
                                width: `${aiSubtitleStatus.progress}%`,
                                background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                              }}
                            />
                          </div>
                        )}
                      </div>
                    </div>
                  ) : (subtitlePreprocessStatus?.status === 'running' || subtitlePreprocessStatus?.status === 'pending') ? (
                    <div className="flex items-center gap-2 px-4 py-2.5 text-sm text-surface-400">
                      <Loader2 size={14} className="animate-spin text-yellow-400" />
                      <div className="flex-1">
                        <div className="text-xs">
                          {subtitlePreprocessStatus.status === 'pending' ? '字幕预处理排队中...' : (subtitlePreprocessStatus.message || '字幕预处理中...')}
                        </div>
                        {subtitlePreprocessStatus.progress > 0 && (
                          <div className="mt-1 h-1 w-full rounded-full bg-surface-700">
                            <div
                              className="h-full rounded-full transition-all duration-500"
                              style={{
                                width: `${subtitlePreprocessStatus.progress}%`,
                                background: 'linear-gradient(90deg, var(--yellow-400), var(--neon-blue))',
                              }}
                            />
                          </div>
                        )}
                      </div>
                    </div>
                  ) : null}
                    </>
                  )}

                  {/* Phase 4: 字幕翻译 — 仅在有可用字幕源时显示 */}
                  {(translatedSubs.length > 0 || aiSubtitleStatus?.status === 'completed' || translating) && (
                    <>
                  <div className="mx-3 my-1 border-t border-neon-blue/10" />
                  <div className="px-4 py-1 text-[10px] font-bold uppercase tracking-widest text-neon-blue/40">
                    <Languages size={10} className="inline mr-1" />字幕翻译
                  </div>

                  {/* 已翻译的字幕列表 */}
                  {translatedSubs.map((sub) => {
                    const langNames: Record<string, string> = {
                      zh: '中文', en: '英文', ja: '日文', ko: '韩文',
                      fr: '法文', de: '德文', es: '西班牙文', pt: '葡萄牙文',
                      ru: '俄文', it: '意大利文', ar: '阿拉伯文', th: '泰文',
                    }
                    return (
                      <button
                        key={sub.language}
                        onClick={() => { loadSubtitle('translated', sub.language); setShowSubtitleMenu(false) }}
                        className={clsx(
                          'block w-full px-4 py-2.5 text-left text-sm transition-colors',
                          activeSubtitle === `translated:${sub.language}`
                            ? 'text-neon-blue'
                            : 'text-surface-300 hover:text-white hover:bg-neon-blue/5'
                        )}
                        style={activeSubtitle === `translated:${sub.language}` ? { background: 'var(--neon-blue-6)' } : {}}
                      >
                        <Languages size={12} className="inline mr-1.5" />
                        {langNames[sub.language] || sub.language}
                        <span className="ml-2 text-xs text-emerald-400/60">✓</span>
                      </button>
                    )
                  })}

                  {/* 翻译进度 */}
                  {translating && translateStatus && (
                    <div className="flex items-center gap-2 px-4 py-2.5 text-sm text-surface-400">
                      <Loader2 size={14} className="animate-spin text-neon-blue" />
                      <div className="flex-1">
                        <div className="text-xs">{translateStatus.message || '正在翻译...'}</div>
                        {translateStatus.progress > 0 && (
                          <div className="mt-1 h-1 w-full rounded-full bg-surface-700">
                            <div
                              className="h-full rounded-full transition-all duration-500"
                              style={{
                                width: `${translateStatus.progress}%`,
                                background: 'linear-gradient(90deg, var(--neon-purple), var(--neon-blue))',
                              }}
                            />
                          </div>
                        )}
                      </div>
                    </div>
                  )}

                  {/* 翻译按钮（需要有源字幕才能翻译） */}
                  {!translating && aiSubtitleStatus?.status === 'completed' && (
                    <div className="relative">
                      <button
                        onClick={() => setShowTranslateMenu(!showTranslateMenu)}
                        className="block w-full px-4 py-2.5 text-left text-sm text-surface-300 hover:text-white hover:bg-neon-blue/5 transition-colors"
                      >
                        <Languages size={12} className="inline mr-1.5" />
                        翻译为其他语言...
                      </button>

                      {showTranslateMenu && (
                        <div className="mx-2 mb-1 rounded-lg py-1" style={{
                          background: 'rgba(0, 0, 0, 0.3)',
                          border: '1px solid var(--neon-blue-6)',
                        }}>
                          {[
                            { code: 'zh', name: '中文' },
                            { code: 'en', name: '英文' },
                            { code: 'ja', name: '日文' },
                            { code: 'ko', name: '韩文' },
                            { code: 'fr', name: '法文' },
                            { code: 'de', name: '德文' },
                            { code: 'es', name: '西班牙文' },
                            { code: 'ru', name: '俄文' },
                          ].filter(lang => !translatedSubs.some(s => s.language === lang.code)).map((lang) => (
                            <button
                              key={lang.code}
                              onClick={() => {
                                setTranslating(true)
                                setShowTranslateMenu(false)
                                subtitleApi.translate(mediaId, lang.code).catch(() => setTranslating(false))
                              }}
                              className="block w-full px-3 py-2 text-left text-xs text-surface-300 hover:text-white hover:bg-neon-blue/5 transition-colors"
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

                  {/* 在线字幕搜索入口 */}
                  <div className="mx-3 my-1 border-t border-neon-blue/10" />
                  <button
                    onClick={() => { setShowSubtitleSearch(true); setShowSubtitleMenu(false) }}
                    className="flex w-full items-center gap-2 px-4 py-2.5 text-left text-sm text-surface-300 hover:text-white hover:bg-neon-blue/5 transition-colors"
                  >
                    <Search size={12} className="text-neon-blue/60" />
                    在线搜索字幕...
                  </button>
                </div>
              )}
            </div>
          )}

          {/* 字幕内容搜索按钮 */}
          {!isStrm && (
            <div className="relative">
              <button
                onClick={() => {
                  setShowContentSearch(!showContentSearch)
                  setShowQuality(false)
                  setShowSubtitleMenu(false)
                  setShowCastPanel(false)
                  setShowSpeedMenu(false)
                }}
                className={clsx(
                  'rounded-lg p-2 transition-all hover:bg-white/5',
                  showContentSearch ? 'text-neon-blue' : 'text-white/70 hover:text-white'
                )}
                title="字幕搜索 (Ctrl+F)"
              >
                <Search size={18} />
              </button>

              {showContentSearch && (
                <SubtitleContentSearch
                  videoRef={videoRef}
                  onClose={() => setShowContentSearch(false)}
                  hasActiveSubtitle={!!activeSubtitle}
                />
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
      {(showQuality || showSubtitleMenu || showCastPanel || showSpeedMenu || showContentSearch) && (
        <div className="absolute inset-0 z-[-1]" onClick={closeAllMenus} />
      )}

      {/* 在线字幕搜索弹窗 (P0) */}
      {showSubtitleSearch && (
        <SubtitleSearchPanel
          mediaId={mediaId}
          title={title}
          onClose={() => setShowSubtitleSearch(false)}
          onDownloaded={() => {
            // 下载完成后刷新外挂字幕列表
            subtitleApi.getTracks(mediaId).then((res) => {
              const info = res.data.data
              if (info) {
                setExternalSubs(info.external || [])
              }
            }).catch(() => {})
          }}
        />
      )}
    </div>
  )
}
