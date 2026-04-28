import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, streamApi, seriesApi } from '@/api'
import type { Media, MediaPlayInfo } from '@/types'
import VideoPlayer from '@/components/VideoPlayer'
import WebCodecsPlayerShell from '@/components/WebCodecsPlayerShell'
import { useToast } from '@/components/Toast'
import { usePlayerStore } from '@/stores/player'
import { Zap, Loader2, Cpu } from 'lucide-react'
import { detectWebCodecs, canUseWebCodecs, type WebCodecsCapability } from '@/utils/webcodecs'

/** 检测浏览器是否支持 HEVC 硬解（Safari / Chrome macOS 116+） */
function canPlayHEVC(): boolean {
  try {
    const video = document.createElement('video')
    return (
      video.canPlayType('video/mp4; codecs="hev1.1.6.L93.B0"') !== '' ||
      video.canPlayType('video/mp4; codecs="hvc1.1.6.L93.B0"') !== '' ||
      video.canPlayType('video/mp4; codecs="hev1"') !== '' ||
      video.canPlayType('video/mp4; codecs="hvc1"') !== ''
    )
  } catch {
    return false
  }
}

/** 检测浏览器是否支持 H.264 直接播放 */
function canPlayH264(): boolean {
  try {
    const video = document.createElement('video')
    return video.canPlayType('video/mp4; codecs="avc1.42E01E"') !== ''
  } catch {
    return false
  }
}

export default function PlayerPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const toast = useToast()
  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [nextEpisode, setNextEpisode] = useState<Media | null>(null)

  // WebCodecs 能力（异步检测）
  const [webcodecsCap, setWebcodecsCap] = useState<WebCodecsCapability | null>(null)

  // 记录当前播放位置，用于预处理完成后无缝切换时恢复进度
  const currentTimeRef = useRef(0)
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
  const [switchPosition, setSwitchPosition] = useState<number | undefined>(undefined)
  const handlePreprocessReady = useCallback(() => {
    if (!id) return
    streamApi.getPlayInfo(id).then((res) => {
      const newPlayInfo = res.data.data
      if (newPlayInfo.is_preprocessed && newPlayInfo.preprocessed_url) {
        // 记录切换瞬间的播放位置，用于恢复进度
        setSwitchPosition(currentTimeRef.current)
        setPlayInfo(newPlayInfo)
        // 播放信息更新后，VideoPlayer 会自动因 src 变化重新加载
        // startPosition 会恢复到当前播放位置，实现无缝切换
      }
    }).catch(() => {})
  }, [id])

  // Remux 降级状态：当 Remux 播放失败时（如浏览器不支持 HEVC 10-bit），自动降级到 HLS 转码
  const [remuxFailed, setRemuxFailed] = useState(false)
  // WebCodecs 降级状态：WebCodecs 播放失败时降级到原生/Remux/HLS
  const [webcodecsFailed, setWebcodecsFailed] = useState(false)

  // Remux 播放失败回调：自动降级到 HLS 转码模式
  const handleRemuxFallback = useCallback(() => {
    toast.info('当前浏览器不支持该编码格式，已自动切换到转码播放')
    setRemuxFailed(true)
  }, [toast])

  // WebCodecs 播放失败回调：降级到常规模式
  const handleWebCodecsFallback = useCallback(() => {
    toast.info('WebCodecs 播放遇到问题，已切换到兼容模式')
    setWebcodecsFailed(true)
  }, [toast])

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
  //   4. WebCodecs 客户端硬解（源容器兼容 MP4 或走 Remux 输出 fMP4）
  //      —— 适用场景：源是 MKV/AVI 等非 MP4 容器但编码为 H.264/HEVC/VP9/AV1 时，
  //         通过后端 Remux URL 拿到 fMP4 流，前端 WebCodecs 硬解
  //   5. Remux（零转码转封装，走 <video> 原生播放）
  //   6. HLS 实时转码（兜底）
  const isPreprocessed = playInfo.is_preprocessed && playInfo.preprocessed_url
  const preferDirect = playInfo.prefer_direct_play !== false // 默认 true
  const videoCodecLower = (playInfo.video_codec || '').toLowerCase()
  const isHEVCSource = videoCodecLower.includes('hevc') || videoCodecLower.includes('h265') || videoCodecLower === 'h265'
  const browserSupportsHEVC = canPlayHEVC()
  const browserSupportsH264 = canPlayH264()

  // 决策：HEVC 源 + 浏览器支持 HEVC → 直接播放（无需转码）
  const canDirectHEVC = isHEVCSource && browserSupportsHEVC && !isPreprocessed

  // WebCodecs 适用性：
  //   - 未被标记失败
  //   - 未使用预处理（预处理已经是 HLS，不需要 WebCodecs）
  //   - 浏览器原生无法直接播放（否则原生更高效）
  //   - 后端支持 Remux（拿到 fMP4 源）或 源本身是 MP4
  //   - WebCodecs 可解该编码
  const nativeCanPlay = playInfo.can_direct_play || canDirectHEVC
  const canUseWC =
    !webcodecsFailed &&
    !isPreprocessed &&
    !playInfo.is_strm && // STRM 流走服务端代理，不走 WebCodecs
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
          : (playInfo.can_remux && !remuxFailed)
            ? 'remux'
            : preferDirect && !remuxFailed && browserSupportsH264
              ? 'direct'  // 用户强制直接播放（可能不兼容，但尊重用户选择）
              : 'hls'

  // src 选择：WebCodecs 模式使用 remux URL（拿到 fMP4 流）或 direct URL（MP4 源）
  const src = isPreprocessed
    ? streamApi.withTokenUrl(playInfo.preprocessed_url!)
    : mode === 'direct'
      ? streamApi.getDirectUrl(id)
      : mode === 'remux'
        ? streamApi.getRemuxUrl(id)
        : mode === 'webcodecs'
          ? (playInfo.can_remux ? streamApi.getRemuxUrl(id) : streamApi.getDirectUrl(id))
          : streamApi.getMasterUrl(id)

  // 构建播放标题（剧集显示 S01E02 格式）
  const playerTitle = media.media_type === 'episode'
    ? `${media.series?.title || media.title} S${String(media.season_num).padStart(2, '0')}E${String(media.episode_num).padStart(2, '0')}${media.episode_title ? ` - ${media.episode_title}` : ''}`
    : media.title

  // 下一集标题
  const nextTitle = nextEpisode
    ? `S${String(nextEpisode.season_num).padStart(2, '0')}E${String(nextEpisode.episode_num).padStart(2, '0')}${nextEpisode.episode_title ? ` ${nextEpisode.episode_title}` : ''}`
    : undefined

  return (
    <div className="h-screen w-screen bg-black relative">
      {/* 预处理状态提示 */}
      {(playInfo.preprocess_status && playInfo.preprocess_status !== 'none') || canDirectHEVC || mode === 'webcodecs' ? (
        <div className="absolute top-4 right-4 z-50 flex items-center gap-2 rounded-lg px-3 py-1.5 text-xs backdrop-blur-md"
          style={{ background: 'rgba(0,0,0,0.7)', border: '1px solid var(--neon-blue-15)' }}>
          {mode === 'webcodecs' ? (
            <>
              <Cpu size={12} className="text-cyan-400" />
              <span className="text-cyan-400">WebCodecs 硬解播放</span>
            </>
          ) : canDirectHEVC ? (
            <>
              <Zap size={12} className="text-purple-400" />
              <span className="text-purple-400">HEVC 直接播放</span>
            </>
          ) : isPreprocessed ? (
            <>
              <Zap size={12} className="text-emerald-400" />
              <span className="text-emerald-400">秒开播放</span>
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
      ) : null}

      {mode === 'webcodecs' ? (
        <WebCodecsPlayerShell
          src={src}
          mediaId={id}
          title={playerTitle}
          startPosition={switchPosition}
          knownDuration={playInfo.duration}
          onBack={() => {
            if (media.media_type === 'episode' && media.series_id) {
              navigate(`/series/${media.series_id}`)
            } else {
              navigate(`/media/${id}`)
            }
          }}
          onNext={nextEpisode ? handleNext : undefined}
          nextTitle={nextTitle}
          onFallback={handleWebCodecsFallback}
        />
      ) : (
        <VideoPlayer
          src={src}
          mode={mode as 'direct' | 'hls' | 'remux'}
          mediaId={id}
          title={playerTitle}
          isStrm={playInfo.is_strm}
          knownDuration={playInfo.duration}
          startPosition={switchPosition}
          spriteVttUrl={playInfo.sprite_vtt_url ? streamApi.withTokenUrl(playInfo.sprite_vtt_url) : undefined}
          onPreprocessReady={handlePreprocessReady}
          onRemuxFallback={mode === 'remux' ? handleRemuxFallback : undefined}
          onBack={() => {
            if (media.media_type === 'episode' && media.series_id) {
              navigate(`/series/${media.series_id}`)
            } else {
              navigate(`/media/${id}`)
            }
          }}
          onNext={nextEpisode ? handleNext : undefined}
          nextTitle={nextTitle}
        />
      )}
    </div>
  )
}
