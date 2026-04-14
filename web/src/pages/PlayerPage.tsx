import { useEffect, useState, useCallback, useRef } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, streamApi, seriesApi } from '@/api'
import type { Media, MediaPlayInfo } from '@/types'
import VideoPlayer from '@/components/VideoPlayer'
import { useToast } from '@/components/Toast'
import { usePlayerStore } from '@/stores/player'
import { Zap, Loader2 } from 'lucide-react'

export default function PlayerPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const toast = useToast()
  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [nextEpisode, setNextEpisode] = useState<Media | null>(null)

  // 记录当前播放位置，用于预处理完成后无缝切换时恢复进度
  const currentTimeRef = useRef(0)
  const { currentTime } = usePlayerStore()
  useEffect(() => {
    currentTimeRef.current = currentTime
  }, [currentTime])

  useEffect(() => {
    if (!id) return

    setLoading(true)
    setNextEpisode(null)

    // 并行获取媒体详情和播放信�?
    Promise.all([
      mediaApi.detail(id),
      streamApi.getPlayInfo(id),
    ])
      .then(([mediaRes, playInfoRes]) => {
        const mediaData = mediaRes.data.data
        setMedia(mediaData)
        setPlayInfo(playInfoRes.data.data)

        // 如果是剧集，获取下一集信�?
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

  // 下一集回�?
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

  // Remux 降级状态：当 Remux 播放失败时（如浏览器不支持 HEVC 10-bit），自动降级到 HLS 转码
  const [remuxFailed, setRemuxFailed] = useState(false)

  // Remux 播放失败回调：自动降级到 HLS 转码模式
  const handleRemuxFallback = useCallback(() => {
    toast.info('当前浏览器不支持该编码格式，已自动切换到转码播放')
    setRemuxFailed(true)
  }, [toast])

  // 智能选择播放模式：
  // 优先级：预处理 HLS > 直接播放（MP4/WebM） > Remux（MKV等零转码转封装） > HLS 实时转码
  //
  // 1. 预处理完成 → 使用预处理的 HLS 流（秒开）
  // 2. 浏览器兼容格式（MP4/WebM/M4V） → 直接播放
  // 3. 容器不兼容但编码兼容（MKV+H.264+AAC） → Remux（FFmpeg -c copy 转封装为 fMP4，零转码）
  //    → 如果 Remux 播放失败（如 HEVC 10-bit 浏览器不支持） → 自动降级到 HLS 转码
  // 4. 其他格式 → HLS 实时转码
  const isPreprocessed = playInfo.is_preprocessed && playInfo.preprocessed_url
  const preferDirect = playInfo.prefer_direct_play !== false // 默认 true
  const mode: 'direct' | 'hls' | 'remux' = isPreprocessed
    ? 'hls'
    : playInfo.can_direct_play
      ? 'direct'
      : (playInfo.can_remux && !remuxFailed)
        ? 'remux'
        : preferDirect && !remuxFailed
          ? 'direct'  // 用户强制直接播放（可能不兼容，但尊重用户选择）
          : 'hls'
  const src = isPreprocessed
    ? streamApi.withTokenUrl(playInfo.preprocessed_url!)
    : mode === 'direct'
      ? streamApi.getDirectUrl(id)
      : mode === 'remux'
        ? streamApi.getRemuxUrl(id)
        : streamApi.getMasterUrl(id)

  // 构建播放标题（剧集显�?S01E02 格式�?
  const playerTitle = media.media_type === 'episode'
    ? `${media.series?.title || media.title} S${String(media.season_num).padStart(2, '0')}E${String(media.episode_num).padStart(2, '0')}${media.episode_title ? ` - ${media.episode_title}` : ''}`
    : media.title

  // 下一集标�?
  const nextTitle = nextEpisode
    ? `S${String(nextEpisode.season_num).padStart(2, '0')}E${String(nextEpisode.episode_num).padStart(2, '0')}${nextEpisode.episode_title ? ` ${nextEpisode.episode_title}` : ''}`
    : undefined

  return (
    <div className="h-screen w-screen bg-black relative">
      {/* 预处理状态提示 */}
      {playInfo.preprocess_status && playInfo.preprocess_status !== 'none' && (
        <div className="absolute top-4 right-4 z-50 flex items-center gap-2 rounded-lg px-3 py-1.5 text-xs backdrop-blur-md"
          style={{ background: 'rgba(0,0,0,0.7)', border: '1px solid var(--neon-blue-15)' }}>
          {isPreprocessed ? (
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
      )}

      <VideoPlayer
        src={src}
        mode={mode}
        mediaId={id}
        title={playerTitle}
        isStrm={playInfo.is_strm}
        knownDuration={playInfo.duration}
        startPosition={switchPosition}
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
    </div>
  )
}
