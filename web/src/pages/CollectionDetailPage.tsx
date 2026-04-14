import { useState, useEffect } from 'react'
import { useParams, useNavigate, Link } from 'react-router-dom'
import { collectionApi } from '@/api'
import { streamApi } from '@/api'
import type { CollectionWithMedia, CollectionMediaItem } from '@/types'
import {
  Layers,
  Film,
  Star,
  Calendar,
  Clock,
  Loader2,
  Play,
  ArrowLeft,
} from 'lucide-react'

export default function CollectionDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [data, setData] = useState<CollectionWithMedia | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!id) return
    setLoading(true)
    setError(null)
    collectionApi.getDetail(id)
      .then(res => {
        setData(res.data.data)
      })
      .catch(() => {
        setError('合集不存在或加载失败')
      })
      .finally(() => {
        setLoading(false)
      })
  }, [id])

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <Loader2 size={32} className="animate-spin text-neon" />
      </div>
    )
  }

  if (error || !data) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] gap-4">
        <Layers size={48} className="text-surface-600" />
        <p className="text-lg font-medium" style={{ color: 'var(--text-secondary)' }}>
          {error || '合集不存在'}
        </p>
        <button onClick={() => navigate('/collections')} className="btn-ghost text-sm">
          返回合集列表
        </button>
      </div>
    )
  }

  const { collection, media } = data

  return (
    <div className="min-h-screen">
      {/* 顶部背景区域 */}
      <div className="relative overflow-hidden" style={{ minHeight: '360px' }}>
        {/* 背景图 */}
        <div className="absolute inset-0">
          <img
            src={streamApi.getCollectionPosterUrl(collection.id)}
            alt=""
            className="h-full w-full object-cover"
            style={{ filter: 'blur(40px) brightness(0.3) saturate(1.5)', transform: 'scale(1.2)' }}
          />
          <div
            className="absolute inset-0"
            style={{
              background: 'linear-gradient(to bottom, transparent 0%, var(--bg-base) 100%)',
            }}
          />
        </div>

        {/* 内容 */}
        <div className="relative z-10 mx-auto max-w-7xl px-4 pt-6 pb-8 sm:px-6 lg:px-8">
          {/* 返回按钮 */}
          <button
            onClick={() => navigate(-1)}
            className="mb-6 flex items-center gap-2 rounded-lg px-3 py-1.5 text-sm font-medium transition-all hover:bg-white/10"
            style={{ color: 'var(--text-secondary)' }}
          >
            <ArrowLeft size={16} />
            返回
          </button>

          <div className="flex flex-col gap-6 sm:flex-row sm:items-end">
            {/* 合集海报 */}
            <div className="flex-shrink-0">
              <div className="relative h-56 w-40 overflow-hidden rounded-xl shadow-2xl sm:h-64 sm:w-44"
                style={{ border: '1px solid var(--border-default)' }}>
                <img
                  src={streamApi.getCollectionPosterUrl(collection.id)}
                  alt={collection.name}
                  className="h-full w-full object-cover"
                  onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
                />
                <div className="absolute inset-0 -z-10 flex h-full w-full items-center justify-center"
                  style={{ background: 'var(--bg-elevated)' }}>
                  <Layers size={40} className="text-surface-600" />
                </div>
              </div>
            </div>

            {/* 合集信息 */}
            <div className="flex-1 min-w-0 pb-2">
              <div className="flex items-center gap-2 mb-2">
                <Layers size={16} className="text-neon/60" />
                <span className="text-xs font-medium uppercase tracking-widest" style={{ color: 'var(--neon-blue)' }}>
                  系列合集
                </span>
              </div>
              <h1 className="font-display text-3xl font-bold tracking-wide sm:text-4xl"
                style={{ color: 'var(--text-primary)' }}>
                {collection.name}
              </h1>
              <div className="mt-3 flex items-center gap-4">
                <span className="rounded-full px-3 py-1 text-sm font-medium"
                  style={{
                    background: 'var(--neon-blue-10)',
                    color: 'var(--neon-blue)',
                    border: '1px solid var(--neon-blue-20)',
                  }}>
                  {media.length} 部电影
                </span>
                {media.length > 0 && (
                  <>
                    {/* 年份范围 */}
                    {(() => {
                      const years = media.filter(m => m.year > 0).map(m => m.year)
                      if (years.length === 0) return null
                      const minYear = Math.min(...years)
                      const maxYear = Math.max(...years)
                      return (
                        <span className="flex items-center gap-1 text-sm" style={{ color: 'var(--text-secondary)' }}>
                          <Calendar size={14} />
                          {minYear === maxYear ? minYear : `${minYear} - ${maxYear}`}
                        </span>
                      )
                    })()}
                    {/* 平均评分 */}
                    {(() => {
                      const ratings = media.filter(m => m.rating > 0).map(m => m.rating)
                      if (ratings.length === 0) return null
                      const avg = ratings.reduce((a, b) => a + b, 0) / ratings.length
                      return (
                        <span className="flex items-center gap-1 text-sm" style={{ color: 'var(--text-secondary)' }}>
                          <Star size={14} className="text-yellow-400" />
                          均分 {avg.toFixed(1)}
                        </span>
                      )
                    })()}
                  </>
                )}
              </div>
              {collection.overview && (
                <p className="mt-4 max-w-2xl text-sm leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>
                  {collection.overview}
                </p>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* 电影列表 */}
      <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
        <h2 className="mb-6 font-display text-lg font-semibold tracking-wide"
          style={{ color: 'var(--text-primary)' }}>
          系列电影
        </h2>

        <div className="grid grid-cols-2 gap-4 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-5 xl:grid-cols-6">
          {media.map((item, index) => (
            <CollectionMovieCard key={item.id} item={item} index={index + 1} />
          ))}
        </div>
      </div>
    </div>
  )
}

/** 合集中的电影卡片 */
function CollectionMovieCard({ item, index }: { item: CollectionMediaItem; index: number }) {
  return (
    <Link
      to={`/media/${item.id}`}
      className="media-card group cursor-pointer transition-all duration-300 hover:scale-[1.02]"
    >
      {/* 海报 */}
      <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl" style={{ background: 'var(--bg-surface)' }}>
        <img
          src={streamApi.getPosterUrl(item.id)}
          alt={item.title}
          className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110"
          loading="lazy"
          onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
        />
        <div className="absolute inset-0 -z-10 flex items-center justify-center" style={{ color: 'var(--text-muted)' }}>
          <Film size={32} />
        </div>

        {/* 序号标签 */}
        <div className="absolute left-2 top-2">
          <span className="block rounded-md px-1.5 py-0.5 text-[10px] font-bold leading-tight backdrop-blur-md"
            style={{
              background: 'rgba(0,0,0,0.7)',
              color: 'var(--text-primary)',
            }}
          >#{index}</span>
        </div>

        {/* 评分标签 */}
        {item.rating > 0 && (
          <div className="absolute left-2 bottom-2">
            <span className="flex items-center gap-0.5 rounded-md px-1.5 py-0.5 text-[10px] font-bold backdrop-blur-md"
              style={{ background: 'rgba(0,0,0,0.7)' }}>
              <Star size={9} className="text-yellow-400" fill="currentColor" />
              <span className="text-yellow-400">{item.rating.toFixed(1)}</span>
            </span>
          </div>
        )}

        {/* 悬停播放图标 */}
        <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
          <div className="absolute bottom-2 right-2">
            <div className="flex h-9 w-9 items-center justify-center rounded-full"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: '0 0 12px var(--neon-blue-40)',
              }}
            >
              <Play size={14} className="ml-0.5 text-white" fill="white" />
            </div>
          </div>
        </div>
      </div>

      {/* 信息 */}
      <div className="p-3">
        <h4 className="truncate text-sm font-medium transition-colors group-hover:text-neon"
          style={{ color: 'var(--text-primary)' }}>
          {item.title}
        </h4>
        <div className="mt-1 flex items-center gap-2 text-[11px]" style={{ color: 'var(--text-muted)' }}>
          {item.year > 0 && (
            <span className="flex items-center gap-0.5">
              <Calendar size={10} /> {item.year}
            </span>
          )}
          {item.runtime > 0 && (
            <span className="flex items-center gap-0.5">
              <Clock size={10} /> {item.runtime}分钟
            </span>
          )}
        </div>
      </div>
    </Link>
  )
}
