import { Link } from 'react-router-dom'
import { Play } from 'lucide-react'
import { streamApi } from '@/api'
import type { Media } from '@/types'
import { useState, useRef } from 'react'

interface MediaCardProps {
  media: Media
}

export default function MediaCard({ media }: MediaCardProps) {
  const cardRef = useRef<HTMLAnchorElement>(null)
  const [mousePos, setMousePos] = useState({ x: 0, y: 0 })
  const [isHovering, setIsHovering] = useState(false)

  // 格式化文件大小
  const formatSize = (bytes: number) => {
    if (!bytes) return ''
    const gb = bytes / (1024 * 1024 * 1024)
    if (gb >= 1) return `${gb.toFixed(1)} GB`
    const mb = bytes / (1024 * 1024)
    return `${mb.toFixed(0)} MB`
  }

  // 格式化时长
  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    if (h > 0) return `${h}h ${m}m`
    return `${m}m`
  }

  // 鼠标追踪实现拟物理光照
  const handleMouseMove = (e: React.MouseEvent) => {
    if (!cardRef.current) return
    const rect = cardRef.current.getBoundingClientRect()
    setMousePos({
      x: ((e.clientX - rect.left) / rect.width) * 100,
      y: ((e.clientY - rect.top) / rect.height) * 100,
    })
  }

  // 剧集类型: 点击跳转到剧集合集页面
  const linkTo = media.series_id
    ? `/series/${media.series_id}`
    : `/media/${media.id}`

  return (
    <Link
      ref={cardRef}
      to={linkTo}
      className="media-card group block"
      onMouseMove={handleMouseMove}
      onMouseEnter={() => setIsHovering(true)}
      onMouseLeave={() => setIsHovering(false)}
    >
      {/* 鼠标追踪光晕 */}
      {isHovering && (
        <div
          className="pointer-events-none absolute inset-0 z-10 rounded-xl opacity-60 transition-opacity duration-300"
          style={{
            background: `radial-gradient(circle 120px at ${mousePos.x}% ${mousePos.y}%, rgba(0, 240, 255, 0.1), transparent)`,
          }}
        />
      )}

      {/* 海报区域 */}
      <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl" style={{ background: 'var(--bg-surface)' }}>
        <img
          src={streamApi.getPosterUrl(media.id)}
          alt={media.title}
          className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110 group-hover:brightness-110"
          loading="lazy"
          onError={(e) => {
            (e.target as HTMLImageElement).style.display = 'none'
          }}
        />
        {/* 占位（海报加载失败时可见） */}
        <div className="absolute inset-0 -z-10 flex items-center justify-center text-surface-700">
          <Play size={48} />
        </div>

        {/* 悬停遮罩 */}
        <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
          <div className="absolute bottom-3 left-3 right-3">
            <div className="flex items-center gap-2">
              <div
                className="flex h-10 w-10 items-center justify-center rounded-full transition-all duration-300 group-hover:scale-110"
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

        {/* 分辨率标签 */}
        {media.resolution && (
          <span className="badge-neon absolute right-2 top-2">
            {media.resolution}
          </span>
        )}

        {/* 剧集集数标签 */}
        {media.media_type === 'episode' && media.episode_num > 0 && (
          <span className="badge-accent absolute left-2 top-2">
            S{String(media.season_num || 1).padStart(2, '0')}E{String(media.episode_num).padStart(2, '0')}
          </span>
        )}
      </div>

      {/* 信息区域 */}
      <div className="p-3">
        <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-primary)' }}>
          {media.title}
        </h3>
        <div className="mt-1 flex items-center gap-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
          {media.year > 0 && <span>{media.year}</span>}
          {media.duration > 0 && (
            <>
              <span className="text-neon-blue/30">·</span>
              <span>{formatDuration(media.duration)}</span>
            </>
          )}
          {media.file_size > 0 && (
            <>
              <span className="text-neon-blue/30">·</span>
              <span>{formatSize(media.file_size)}</span>
            </>
          )}
        </div>
      </div>
    </Link>
  )
}
