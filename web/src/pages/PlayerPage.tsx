import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, streamApi, seriesApi } from '@/api'
import type { Media, MediaPlayInfo } from '@/types'
import VideoPlayer from '@/components/VideoPlayer'
import { useToast } from '@/components/Toast'

export default function PlayerPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const toast = useToast()
  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)
  const [nextEpisode, setNextEpisode] = useState<Media | null>(null)

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

  // 智能选择播放模式：
  // 1. 预处理完成 → 使用预处理的 HLS 流（秒开）
  // 2. MP4 → 直接播放
  // 3. 其他格式 → 实时 HLS 转码
  const isPreprocessed = playInfo.is_preprocessed && playInfo.preprocessed_url
  const mode = isPreprocessed ? 'hls' : (playInfo.can_direct_play ? 'direct' : 'hls')
  const src = isPreprocessed
    ? streamApi.withTokenUrl(playInfo.preprocessed_url!)
    : playInfo.can_direct_play
      ? streamApi.getDirectUrl(id)
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
    <div className="h-screen w-screen bg-black">
      <VideoPlayer
        src={src}
        mode={mode}
        mediaId={id}
        title={playerTitle}
        isStrm={playInfo.is_strm}
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
