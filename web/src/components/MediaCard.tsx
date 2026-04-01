import { Link } from 'react-router-dom'
import { Play, Tv } from 'lucide-react'
import { streamApi } from '@/api'
import type { Media, Series } from '@/types'
import { useRef, useCallback } from 'react'
import { motion, useMotionValue, useTransform, useSpring, useMotionTemplate } from 'framer-motion'
import { springDefault } from '@/lib/motion'

interface MediaCardProps {
  media?: Media
  series?: Series
}

export default function MediaCard({ media, series }: MediaCardProps) {
  const cardRef = useRef<HTMLDivElement>(null)

  // 使用 motion value 替代 useState，避免 re-render
  const mouseX = useMotionValue(0.5)
  const mouseY = useMotionValue(0.5)
  const isHovering = useMotionValue(0)

  // 3D 倾斜效果（±4度）
  const rotateX = useSpring(useTransform(mouseY, [0, 1], [4, -4]), { stiffness: 300, damping: 30 })
  const rotateY = useSpring(useTransform(mouseX, [0, 1], [-4, 4]), { stiffness: 300, damping: 30 })

  // 光晕跟随位置
  const glowX = useTransform(mouseX, [0, 1], [0, 100])
  const glowY = useTransform(mouseY, [0, 1], [0, 100])
  const glowOpacity = useSpring(isHovering, { stiffness: 300, damping: 30 })
  const glowBg = useMotionTemplate`radial-gradient(circle 120px at ${glowX}% ${glowY}%, var(--neon-blue-10), transparent)`

  const handleMouseMove = useCallback((e: React.MouseEvent) => {
    if (!cardRef.current) return
    const rect = cardRef.current.getBoundingClientRect()
    mouseX.set((e.clientX - rect.left) / rect.width)
    mouseY.set((e.clientY - rect.top) / rect.height)
  }, [mouseX, mouseY])

  const handleMouseEnter = useCallback(() => {
    isHovering.set(1)
  }, [isHovering])

  const handleMouseLeave = useCallback(() => {
    mouseX.set(0.5)
    mouseY.set(0.5)
    isHovering.set(0)
  }, [mouseX, mouseY, isHovering])

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

  // 确定链接目标和显示数据
  const isSeries = !!series || !!(media?.series_id)
  const seriesData = series || media?.series
  const linkTo = series
    ? `/series/${series.id}`
    : media!.series_id
      ? `/series/${media!.series_id}`
      : `/media/${media!.id}`

  const title = series ? series.title : media!.title
  const year = series ? series.year : media!.year
  const rating = series ? series.rating : media!.rating
  const posterUrl = series
    ? streamApi.getSeriesPosterUrl(series.id)
    : media!.series_id
      ? streamApi.getSeriesPosterUrl(media!.series_id)
      : streamApi.getPosterUrl(media!.id)

  return (
    <motion.div
      ref={cardRef}
      className="media-card group block"
      style={{ perspective: 800 }}
      onMouseMove={handleMouseMove}
      onMouseEnter={handleMouseEnter}
      onMouseLeave={handleMouseLeave}
      whileHover={{ y: -4 }}
      whileTap={{ y: 0 }}
      transition={springDefault}
    >
      <Link to={linkTo}>
        <motion.div style={{ rotateX, rotateY, transformStyle: 'preserve-3d' }}>
          {/* 鼠标追踪光晕 — 使用 motion value，零 re-render */}
          <motion.div
            className="pointer-events-none absolute inset-0 z-10 rounded-xl"
            style={{
              background: glowBg,
              opacity: glowOpacity,
            }}
          />

          {/* 海报区域 */}
          <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl bg-theme-bg-surface">
            <img
              src={posterUrl}
              alt={title}
              className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110 group-hover:brightness-110"
              loading="lazy"
              onError={(e) => {
                (e.target as HTMLImageElement).style.display = 'none'
              }}
            />
            {/* 占位（海报加载失败时可见） */}
            <div className="absolute inset-0 -z-10 flex items-center justify-center text-surface-700">
              {isSeries ? <Tv size={48} /> : <Play size={48} />}
            </div>

            {/* 悬停遮罩 */}
            <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
              <div className="absolute bottom-3 left-3 right-3">
                <div className="flex items-center gap-2">
                  <div
                    className="flex h-10 w-10 items-center justify-center rounded-full transition-all duration-300 group-hover:scale-110"
                    style={{
                      background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                      boxShadow: 'var(--neon-glow-shadow-lg)',
                    }}
                  >
                    <Play size={18} className="ml-0.5 text-white" fill="white" />
                  </div>
                  <span className="text-sm font-semibold text-white">{isSeries ? '查看' : '播放'}</span>
                </div>
              </div>
            </div>

            {/* 分辨率标签（仅电影） */}
            {!isSeries && media!.resolution && (
              <span className="badge-neon absolute right-2 top-2">
                {media!.resolution}
              </span>
            )}

            {/* 剧集合集标签 */}
            {isSeries && seriesData && seriesData.season_count > 0 && (
              <span className="badge-accent absolute left-2 top-2">
                {seriesData.season_count} 季 · {seriesData.episode_count} 集
              </span>
            )}

            {/* 剧集类型标识（右下角） */}
            {isSeries && (
              <div className="absolute bottom-2 right-2 flex h-6 w-6 items-center justify-center rounded-md"
                style={{ background: 'rgba(0,0,0,0.6)', backdropFilter: 'blur(4px)' }}
              >
                <Tv size={12} className="text-neon" />
              </div>
            )}
          </div>

          {/* 信息区域 */}
          <div className="p-3">
            <h3 className="truncate text-sm font-medium transition-colors group-hover:text-neon text-theme-primary">
              {title}
            </h3>
            <div className="mt-1 flex items-center gap-2 text-xs text-theme-secondary">
              {year > 0 && <span>{year}</span>}
              {rating > 0 && (
                <>
                  <span className="text-neon-blue/30">·</span>
                  <span className="text-yellow-400">★ {rating.toFixed(1)}</span>
                </>
              )}
              {!isSeries && media!.duration > 0 && (
                <>
                  <span className="text-neon-blue/30">·</span>
                  <span>{formatDuration(media!.duration)}</span>
                </>
              )}
              {!isSeries && media!.file_size > 0 && (
                <>
                  <span className="text-neon-blue/30">·</span>
                  <span>{formatSize(media!.file_size)}</span>
                </>
              )}
              {isSeries && seriesData && seriesData.episode_count > 0 && (
                <>
                  <span className="text-neon-blue/30">·</span>
                  <span>{seriesData.episode_count} 集</span>
                </>
              )}
            </div>
          </div>
        </motion.div>
      </Link>
    </motion.div>
  )
}
