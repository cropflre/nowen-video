import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { mediaApi, recommendApi } from '@/api'
import type { Media, WatchHistory, RecommendedMedia } from '@/types'
import MediaGrid from '@/components/MediaGrid'
import { Play, Clock, Sparkles } from 'lucide-react'
import { streamApi } from '@/api'

export default function HomePage() {
  const [recentMedia, setRecentMedia] = useState<Media[]>([])
  const [continueList, setContinueList] = useState<WatchHistory[]>([])
  const [recommendations, setRecommendations] = useState<RecommendedMedia[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [recentRes, continueRes, recommendRes] = await Promise.all([
          mediaApi.recent(20),
          mediaApi.continueWatching(10),
          recommendApi.getRecommendations(12),
        ])
        setRecentMedia(recentRes.data.data || [])
        setContinueList(continueRes.data.data || [])
        setRecommendations(recommendRes.data.data || [])
      } catch {
        // 静默处理
      } finally {
        setLoading(false)
      }
    }
    fetchData()
  }, [])

  // 格式化进度百分比
  const formatProgress = (position: number, duration: number) => {
    if (!duration) return 0
    return Math.round((position / duration) * 100)
  }

  return (
    <div className="space-y-10">
      {/* 继续观看 */}
      {continueList.length > 0 && (
        <section className="animate-fade-in">
          <h2 className="mb-5 flex items-center gap-2 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Clock size={20} className="text-neon" />
            继续观看
          </h2>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
            {continueList.map((item) => (
              <Link
                key={item.id}
                to={`/play/${item.media_id}`}
                className="glass-panel-subtle group flex gap-3 rounded-xl p-3 transition-all duration-300 hover:border-neon-blue/20 hover:shadow-card-hover"
              >
                {/* 缩略图 */}
                <div className="relative h-20 w-32 flex-shrink-0 overflow-hidden rounded-lg" style={{ background: 'var(--bg-surface)' }}>
                  {item.media.poster_path ? (
                    <img
                      src={item.media.poster_path}
                      alt={item.media.title}
                      className="h-full w-full object-cover"
                    />
                  ) : (
                    <div className="flex h-full w-full items-center justify-center text-surface-700">
                      <Play size={24} />
                    </div>
                  )}
                  {/* 播放图标 */}
                  <div className="absolute inset-0 flex items-center justify-center bg-black/40 opacity-0 transition-opacity group-hover:opacity-100">
                    <Play size={24} className="text-white" fill="white" />
                  </div>
                  {/* 霓虹进度条 */}
                  <div className="absolute bottom-0 left-0 right-0 h-0.5 bg-white/10">
                    <div
                      className="h-full transition-all"
                      style={{
                        width: `${formatProgress(item.position, item.duration)}%`,
                        background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                        boxShadow: '0 0 6px rgba(0, 240, 255, 0.3)',
                      }}
                    />
                  </div>
                </div>

                {/* 信息 */}
                <div className="min-w-0 flex-1">
                  <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
                    {item.media.title}
                  </h3>
                  <p className="mt-1 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                    已观看 {formatProgress(item.position, item.duration)}%
                  </p>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* 为你推荐 */}
      {recommendations.length > 0 && (
        <section className="animate-fade-in">
          <h2 className="mb-5 flex items-center gap-2 font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Sparkles size={20} className="text-yellow-400" />
            为你推荐
          </h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
            {recommendations.map((item) => (
              <Link
                key={item.media.id}
                to={`/media/${item.media.id}`}
                className="media-card group block"
              >
                {/* 海报区域 */}
                <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl" style={{ background: 'var(--bg-surface)' }}>
                  <img
                    src={streamApi.getPosterUrl(item.media.id)}
                    alt={item.media.title}
                    className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110 group-hover:brightness-110"
                    loading="lazy"
                    onError={(e) => {
                      (e.target as HTMLImageElement).style.display = 'none'
                    }}
                  />
                  <div className="absolute inset-0 -z-10 flex items-center justify-center text-surface-700">
                    <Play size={48} />
                  </div>
                  {/* 悬停遮罩 */}
                  <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
                    <div className="absolute bottom-3 left-3 right-3">
                      <div className="flex items-center gap-2">
                        <div
                          className="flex h-10 w-10 items-center justify-center rounded-full transition-all duration-300"
                          style={{
                            background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                            boxShadow: '0 0 20px rgba(0, 240, 255, 0.4)',
                          }}
                        >
                          <Play size={18} className="ml-0.5 text-white" fill="white" />
                        </div>
                        <span className="text-sm font-semibold text-white">播放</span>
                      </div>
                    </div>
                  </div>
                  {/* 推荐理由标签 */}
                  <span className="badge-accent absolute left-2 top-2">
                    {item.reason}
                  </span>
                </div>
                {/* 信息区域 */}
                <div className="p-3">
                  <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
                    {item.media.title}
                  </h3>
                  <div className="mt-1 flex items-center gap-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
                    {item.media.year > 0 && <span>{item.media.year}</span>}
                    {item.media.rating > 0 && (
                      <>
                        <span className="text-neon-blue/30">·</span>
                        <span className="text-yellow-400">★ {item.media.rating.toFixed(1)}</span>
                      </>
                    )}
                  </div>
                </div>
              </Link>
            ))}
          </div>
        </section>
      )}

      {/* 最近添加 */}
      <MediaGrid
        items={recentMedia}
        title="最近添加"
        loading={loading}
      />

      {/* 空状态 */}
      {!loading && recentMedia.length === 0 && continueList.length === 0 && (
        <div className="flex flex-col items-center justify-center py-24 text-center">
          <div
            className="mb-6 flex h-20 w-20 items-center justify-center rounded-2xl animate-float"
            style={{
              background: 'linear-gradient(135deg, rgba(0,240,255,0.1), rgba(138,43,226,0.1))',
              border: '1px solid rgba(0, 240, 255, 0.1)',
            }}
          >
            <Play size={36} className="text-surface-600" />
          </div>
          <h3 className="font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-secondary)' }}>
            暂无媒体内容
          </h3>
          <p className="mt-2 text-sm" style={{ color: 'var(--text-muted)' }}>
            前往管理页面添加媒体库并扫描文件
          </p>
        </div>
      )}
    </div>
  )
}
