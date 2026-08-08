import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, streamApi, seriesApi } from '@/api'
import type { Media, MediaPlayInfo } from '@/types'
import AdaptiveWebVideoPlayer, {
  type BrowserPlaybackMode,
  type PlaybackTransition,
} from '@/components/AdaptiveWebVideoPlayer'
import WebCodecsPlayerShell from '@/components/WebCodecsPlayerShell'
import STRMDiagnostics from '@/components/player/STRMDiagnostics'
import { useToast } from '@/components/Toast'
import { usePlayerStore } from '@/stores/player'
import { Zap, Loader2, Cpu, ArrowLeft } from 'lucide-react'
import { detectWebCodecs, canUseWebCodecs, type WebCodecsCapability } from '@/utils/webcodecs'
import {
  DesktopPlayerBadge,
  MpvEmbedPlayer,
  useDesktop,
  usePlayerEngine,
  type MediaProfile,
} from '@/desktop'

import {
  getMediaCapabilities,
  type BrowserMediaCapability,
} from '@/utils/media-capabilities'

/** 获取浏览器精确能力（延迟初始化 + 缓存） */
function getBrowserCaps(): BrowserMediaCapability {
  return getMediaCapabilities()
}

export default function PlayerPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const toast = useToast()
  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [nextEpisode, setNextEpisode] = useState<Media | null>(null)
  const [switchPosition, setSwitchPosition] = useState<number | undefined>(undefined)
  const [webcodecsFailed, setWebcodecsFailed] = useState(false)
  const [runtimeMode, setRuntimeMode] = useState<BrowserPlaybackMode | null>(null)

  // WebCodecs 能力（异步检测）
  const [webcodecsCap, setWebcodecsCap] = useState<WebCodecsCapability | null>(null)

  // 记录当前播放位置，用于播放内核或播放方式切换时恢复进度
  const currentTimeRef = useRef(0)

  // 记录挂载时的历史栈长度，用于判断是否存在来路（>1 说明从站内进入）
  // 可靠地区分“从列表/详情页进来” vs “外链直接打开播放器”两种场景
  const hasHistoryOnMountRef = useRef<boolean>(window.history.length > 1)
  const { currentTime } = usePlayerStore()
  useEffect(() => {
    currentTimeRef.current = currentTime
  }, [currentTime])

  // 启动时异步检测 WebCodecs 能力（只做一次）
  useEffect(() => {
    detectWebCodecs().then(setWebcodecsCap).catch(() => setWebcodecsCap(null))
  }, [])

  useEffect(() => {
    if (!id) return

    setLoading(true)
    setNextEpisode(null)
    setSwitchPosition(undefined)
    setWebcodecsFailed(false)
    setRuntimeMode(null)

    // 并行获取媒体详情和播放信息
    Promise.all([
      mediaApi.detail(id),
      streamApi.getPlayInfo(id),
    ])
      .then(([mediaRes, playInfoRes]) => {
        const mediaData = mediaRes.data.data
        setMedia(mediaData)
        setPlayInfo(playInfoRes.data.data)

        // 如果是剧集，获取下一集信息
        if (mediaData.media_type === 'episode' && mediaData.series_id) {
          seriesApi
            .nextEpisode(mediaData.series_id, mediaData.season_num, mediaData.episode_num)
            .then((res) => {
              if (res.data.data) {
                setNextEpisode(res.data.data)
              }
            })
            .catch(() => {}) // 获取下一集失败不影响播放
        }
      })
      .catch(() => {
        toast.error('加载播放信息失败')
        navigate('/')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [id, navigate, toast])

  // 下一集回调
  const handleNext = useCallback(() => {
    if (nextEpisode) {
      navigate(`/play/${nextEpisode.id}`, { replace: true })
    }
  }, [nextEpisode, navigate])

  // 预处理完成回调：后台静默刷新播放信息，自动切换到预处理流（无缝、无感知）
  const handlePreprocessReady = useCallback(() => {
    if (!id) return
    streamApi.getPlayInfo(id).then((res) => {
      const newPlayInfo = res.data.data
      if (newPlayInfo.is_preprocessed && newPlayInfo.preprocessed_url) {
        setSwitchPosition(currentTimeRef.current)
        setRuntimeMode(null)
        setPlayInfo(newPlayInfo)
      }
    }).catch(() => {})
  }, [id])

  // WebCodecs 播放失败后，从当前位置进入服务端权威的 Remux/HLS 兼容链路。
  const handleWebCodecsFallback = useCallback(() => {
    setSwitchPosition(currentTimeRef.current)
    setRuntimeMode(null)
    toast.info('WebCodecs 播放遇到问题，已切换到兼容模式')
    setWebcodecsFailed(true)
  }, [toast])

  const handleRuntimeModeChange = useCallback((mode: BrowserPlaybackMode) => {
    setRuntimeMode(mode)
  }, [])

  const handlePlaybackTransition = useCallback((transition: PlaybackTransition) => {
    const target = transition.to === 'remux' ? 'Remux 兼容播放'
      : transition.to === 'smart_remux' ? 'Smart Remux（音频转码）'
      : 'HLS 转码播放'
    toast.info(`当前播放方式不兼容，已自动切换到${target}`)
  }, [toast])

  // ===== 桌面端内核决策 Hook（必须在早返回之前调用以满足 Rules of Hooks）=====
  const desktopCtx = useDesktop()
  const desktopIsDesktop = desktopCtx.isDesktop
  const desktopEmbedAvailable = desktopCtx.embedAvailable
  const profileForEngine: MediaProfile | null = playInfo
    ? {
        container: (playInfo.file_ext || '').replace(/^\./, '').toLowerCase(),
        video_codec: (playInfo.video_codec || '').toLowerCase(),
        audio_codec: (playInfo.audio_codec || '').toLowerCase(),
        height: (playInfo as unknown as { height?: number }).height || 0,
        hdr: (playInfo as unknown as { hdr?: string }).hdr || '',
      }
    : null
  const { engine: desktopEngine } = usePlayerEngine(profileForEngine)

  if (loading || !media || !playInfo || !id) {
    return (
      <div className="flex h-screen items-center justify-center bg-black">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-t-transparent" style={{ borderColor: 'var(--neon-blue-30)', borderTopColor: 'transparent' }} />
          <p className="text-sm text-surface-500">正在加载播放信息...</p>
        </div>
      </div>
    )
  }

  // 智能选择播放模式（优先级从高到低）：
  //   1. 预处理完成 HLS（秒开）
  //   2. HEVC 源 + 浏览器 HEVC 硬解支持 → 原生直接播放
  //   3. 原生可直接播放（MP4/WebM） → 原生直接播放
  //   4. WebCodecs 客户端硬解（失败后进入统一兼容链路）
  //   5. Remux（零转码转封装，走 <video> 原生播放）
  //   6. HLS 实时转码（最终兜底）
  const isPreprocessed = playInfo.is_preprocessed && playInfo.preprocessed_url
  const videoCodecLower = (playInfo.video_codec || '').toLowerCase()
  const isHEVCSource = videoCodecLower.includes('hevc') || videoCodecLower.includes('h265') || videoCodecLower === 'h265'
  const browserCaps = getBrowserCaps()
  const browserSupportsHEVC = browserCaps.video.hevc.main !== 'unsupported'

  // 决策：HEVC 源 + 浏览器支持 HEVC + 后端确认容器/音频均可直放 → 直接播放（无需转码）
  const canDirectHEVC = isHEVCSource && browserSupportsHEVC && !isPreprocessed && playInfo.can_direct_play

  // 桌面端 libmpv 嵌入决策（C 档 Hills 化核心）
  const useMpvEmbed =
    desktopIsDesktop &&
    desktopEmbedAvailable &&
    desktopEngine === 'mpv' &&
    !isPreprocessed

  // WebCodecs 适用性：源容器或 Remux 输出可供客户端解封装，且浏览器可硬解。
  const nativeCanPlay = playInfo.can_direct_play || canDirectHEVC
  const canUseWC =
    !webcodecsFailed &&
    !isPreprocessed &&
    !playInfo.is_strm &&
    !nativeCanPlay &&
    !!webcodecsCap &&
    canUseWebCodecs(playInfo.video_codec, playInfo.audio_codec, webcodecsCap) &&
    (playInfo.can_remux || playInfo.file_ext === '.mp4' || playInfo.file_ext === '.m4v')

  const mode: 'direct' | 'hls' | 'remux' | 'webcodecs' = isPreprocessed
    ? 'hls'
    : canDirectHEVC
      ? 'direct'
      : playInfo.can_direct_play
        ? 'direct'
        : canUseWC
          ? 'webcodecs'
          : playInfo.can_remux
            ? 'remux'
            : 'hls'

  const requiresSessionTranscode =
    mode === 'hls' &&
    !isPreprocessed &&
    streamApi.requiresPlaybackSession(id)

  // src 只能来自直放、Remux、持久预处理或服务端播放计划。
  // Session 模式由 SessionVideoPlayer 创建 Generation 后提供实际 playlist。
  const src = isPreprocessed
    ? streamApi.withTokenUrl(playInfo.preprocessed_url!)
    : mode === 'direct'
      ? streamApi.getDirectUrl(id)
      : mode === 'remux'
        ? streamApi.getRemuxUrl(id)
        : mode === 'webcodecs'
          ? (playInfo.can_remux ? streamApi.getRemuxUrl(id) : streamApi.getDirectUrl(id))
          : requiresSessionTranscode
            ? ''
            : streamApi.getMasterUrl(id)

  const effectiveBrowserMode = runtimeMode || (mode === 'webcodecs' ? null : mode)
  const browserPlaybackResetKey = `${id}:${isPreprocessed ? playInfo.preprocessed_url : 'planned'}:${webcodecsFailed ? 'wc-fallback' : 'initial'}`

  // 构建播放标题（剧集显示 S01E02 格式）
  const playerTitle = media.media_type === 'episode'
    ? `${media.series?.title || media.title} S${String(media.season_num).padStart(2, '0')}E${String(media.episode_num).padStart(2, '0')}${media.episode_title ? ` - ${media.episode_title}` : ''}`
    : media.title

  // 下一集标题
  const nextTitle = nextEpisode
    ? `S${String(nextEpisode.season_num).padStart(2, '0')}E${String(nextEpisode.episode_num).padStart(2, '0')}${nextEpisode.episode_title ? ` ${nextEpisode.episode_title}` : ''}`
    : undefined

  // 返回逻辑：优先回退上一页，保留列表页的分页/筛选/滚动位置
  const handleBack = () => {
    if (hasHistoryOnMountRef.current) {
      navigate(-1)
      return
    }
    if (media.media_type === 'episode' && media.series_id) {
      navigate(`/series/${media.series_id}`, { replace: true })
    } else {
      navigate(`/media/${id}`, { replace: true })
    }
  }

  return (
    <div className="h-screen w-screen bg-black relative">
      <button
        onClick={handleBack}
        aria-label="返回"
        title="返回"
        className="absolute left-4 top-4 z-[60] flex h-9 w-9 items-center justify-center rounded-full text-white backdrop-blur-md transition-all hover:scale-105"
        style={{ background: 'rgba(0,0,0,0.55)', border: '1px solid var(--neon-blue-15)' }}
      >
        <ArrowLeft size={18} />
      </button>

      <div className="absolute top-4 right-4 z-50 flex flex-col items-end gap-2">
        {playInfo.is_strm && (
          <STRMDiagnostics mediaId={id} compact />
        )}
        <DesktopPlayerBadge
          profile={{
            container: (playInfo.file_ext || '').replace(/^\./, '').toLowerCase(),
            video_codec: (playInfo.video_codec || '').toLowerCase(),
            audio_codec: (playInfo.audio_codec || '').toLowerCase(),
            height: (playInfo as unknown as { height?: number }).height || 0,
            hdr: (playInfo as unknown as { hdr?: string }).hdr || '',
          } as MediaProfile}
          streamUrl={streamApi.getDirectUrl(id)}
          playOptions={{ title: playerTitle }}
        />
        {(mode === 'webcodecs' || effectiveBrowserMode === 'remux' || effectiveBrowserMode === 'smart_remux' || effectiveBrowserMode === 'hls' ||
          (effectiveBrowserMode === 'direct' && canDirectHEVC) || isPreprocessed ||
          playInfo.preprocess_status === 'running' ||
          playInfo.preprocess_status === 'pending' ||
          playInfo.preprocess_status === 'queued') && (
          <div className="flex items-center gap-2 rounded-lg px-3 py-1.5 text-xs backdrop-blur-md"
            style={{ background: 'rgba(0,0,0,0.7)', border: '1px solid var(--neon-blue-15)' }}>
            {mode === 'webcodecs' ? (
              <>
                <Cpu size={12} className="text-cyan-400" />
                <span className="text-cyan-400">WebCodecs 硬解播放</span>
              </>
            ) : isPreprocessed ? (
              <>
                <Zap size={12} className="text-emerald-400" />
                <span className="text-emerald-400">秒开播放</span>
              </>
            ) : effectiveBrowserMode === 'direct' && canDirectHEVC ? (
              <>
                <Zap size={12} className="text-purple-400" />
                <span className="text-purple-400">HEVC 直接播放</span>
              </>
            ) : effectiveBrowserMode === 'remux' ? (
              <>
                <Cpu size={12} className="text-cyan-400" />
                <span className="text-cyan-400">Remux 兼容播放</span>
              </>
            ) : effectiveBrowserMode === 'smart_remux' ? (
              <>
                <Cpu size={12} className="text-green-400" />
                <span className="text-green-400">Smart Remux（音频转码）</span>
              </>
            ) : effectiveBrowserMode === 'hls' ? (
              <>
                <Cpu size={12} className="text-orange-400" />
                <span className="text-orange-400">HLS 转码播放</span>
              </>
            ) : playInfo.preprocess_status === 'running' ? (
              <>
                <Loader2 size={12} className="text-neon-blue animate-spin" />
                <span className="text-surface-400">正在预处理中...</span>
              </>
            ) : playInfo.preprocess_status === 'pending' || playInfo.preprocess_status === 'queued' ? (
              <>
                <Loader2 size={12} className="text-yellow-400" />
                <span className="text-surface-400">等待预处理</span>
              </>
            ) : null}
          </div>
        )}
      </div>

      {useMpvEmbed ? (
        <MpvEmbedPlayer
          streamUrl={src}
          sessionId={`media-${id}`}
          title={playerTitle}
          playOptions={{
            title: playerTitle,
            start_time: switchPosition,
          }}
          onBack={handleBack}
          className="h-full w-full"
        />
      ) : mode === 'webcodecs' ? (
        <WebCodecsPlayerShell
          src={src}
          mediaId={id}
          title={playerTitle}
          startPosition={switchPosition}
          knownDuration={playInfo.duration}
          onBack={handleBack}
          onNext={nextEpisode ? handleNext : undefined}
          nextTitle={nextTitle}
          onFallback={handleWebCodecsFallback}
        />
      ) : (
        <AdaptiveWebVideoPlayer
          mediaId={id}
          initialPlan={streamApi.getCachedPlaybackPlan(id)}
          initialMode={mode as BrowserPlaybackMode}
          initialSrc={src}
          initialRequiresSession={requiresSessionTranscode}
          resetKey={browserPlaybackResetKey}
          supportsHEVC={browserSupportsHEVC}
          title={playerTitle}
          isStrm={playInfo.is_strm}
          knownDuration={playInfo.duration}
          startPosition={switchPosition}
          spriteVttUrl={playInfo.sprite_vtt_url ? streamApi.withTokenUrl(playInfo.sprite_vtt_url) : undefined}
          onPreprocessReady={handlePreprocessReady}
          onModeChange={handleRuntimeModeChange}
          onTransition={handlePlaybackTransition}
          onBack={handleBack}
          onNext={nextEpisode ? handleNext : undefined}
          nextTitle={nextTitle}
        />
      )}
    </div>
  )
}
