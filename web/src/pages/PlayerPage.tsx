import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, streamApi } from '@/api'
import type { Media, MediaPlayInfo } from '@/types'
import VideoPlayer from '@/components/VideoPlayer'

export default function PlayerPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    if (!id) return

    // 并行获取媒体详情和播放信息
    Promise.all([
      mediaApi.detail(id),
      streamApi.getPlayInfo(id),
    ])
      .then(([mediaRes, playInfoRes]) => {
        setMedia(mediaRes.data.data)
        setPlayInfo(playInfoRes.data.data)
      })
      .catch(() => {
        navigate('/')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [id, navigate])

  if (loading || !media || !playInfo || !id) {
    return (
      <div className="flex h-screen items-center justify-center bg-black">
        <div className="flex flex-col items-center gap-3">
          <div className="h-8 w-8 animate-spin rounded-full border-2 border-t-transparent" style={{ borderColor: 'rgba(0,240,255,0.3)', borderTopColor: 'transparent' }} />
          <p className="text-sm text-surface-500">正在加载播放信息...</p>
        </div>
      </div>
    )
  }

  // 智能选择播放模式：MP4优先直接播放，MKV等走HLS转码
  const mode = playInfo.can_direct_play ? 'direct' : 'hls'
  const src = playInfo.can_direct_play
    ? streamApi.getDirectUrl(id)
    : streamApi.getMasterUrl(id)

  return (
    <div className="h-screen w-screen bg-black">
      <VideoPlayer
        src={src}
        mode={mode}
        mediaId={id}
        title={media.title}
        onBack={() => navigate(`/media/${id}`)}
      />
    </div>
  )
}
