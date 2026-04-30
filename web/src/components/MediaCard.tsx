import { Link, useNavigate } from 'react-router-dom'
import { Play, Tv, Film, Info } from 'lucide-react'
import { streamApi } from '@/api'
import type { Media, Series } from '@/types'
import { useRef, useCallback } from 'react'
import { motion, useMotionValue, useTransform, useSpring, useMotionTemplate } from 'framer-motion'
import { springDefault } from '@/lib/motion'
import { usePosterVersion } from '@/stores/mediaRefresh'

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

  // 格式化时长
  const formatDuration = (seconds: number) => {
    if (!seconds) return ''
    const h = Math.floor(seconds / 3600)
    const m = Math.floor((seconds % 3600) / 60)
    if (h > 0) return `${h}h ${m}m`
    return `${m}m`
  }

  const navigate = useNavigate()

  // 确定链接目标和显示数据
  const isSeries = !!series || !!(media?.series_id)
  const seriesData = series || media?.series

  // 详情页链接（点击名字/其他区域）
  const detailTo = series
    ? `/series/${series.id}`
    : media!.series_id
      ? `/series/${media!.series_id}`
      : `/media/${media!.id}`

  // 播放/阅读链接（点击封面中间的播放按钮）
  // 非系列的独立媒体直接进入播放页，系列进入详情页
  const playTo = series
    ? `/series/${series.id}`
    : media!.series_id
      ? `/series/${media!.series_id}`
      : `/play/${media!.id}`

  const title = series ? series.title : media!.title
  const year = series ? series.year : media!.year
  const rating = series ? series.rating : media!.rating
  // 订阅全局海报版本戳：刮削完成/元数据替换后自动刷新图片缓存
  const posterVersion = usePosterVersion()
  const posterUrl = series
    ? streamApi.getSeriesPosterUrl(series.id, posterVersion)
    : media!.series_id
      ? streamApi.getSeriesPosterUrl(media!.series_id, posterVersion)
      : streamApi.getPosterUrl(media!.id, posterVersion)

  // 检查是否有真实海报（poster_path 非空）
  // 对于 episode 或未加载 series 关联的场景，我们也允许尝试加载：
  //   - 后端 Poster handler 会自动 fallback 到 series 海报或 sidecar 文件
  //   - 仅在 media.poster_path 与 series.poster_path 都为空且不是剧集时，才直接显示占位
  const hasPoster = series
    ? !!series.poster_path
    : media!.series_id
      ? !!(media!.series?.poster_path) || !!media!.poster_path
      : !!media!.poster_path

  // 点击播放按钮 — 阻止冒泡，导航到播放页
  const handlePlayClick = useCallback((e: React.MouseEvent) => {
    e.preventDefault()
    e.stopPropagation()
    navigate(playTo)
  }, [navigate, playTo])

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
      <Link to={detailTo}>
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
          <div className="relative aspect-[2/3] overflow-hidden rounded-t-xl bg-theme-bg-surface isolate"
            style={{ transform: 'translateZ(0)' }}
          >
            {hasPoster ? (
              <img
                src={posterUrl}
                alt={title}
                className="h-full w-full object-cover transition-all duration-500 group-hover:scale-110 group-hover:brightness-110"
                loading="lazy"
                onError={(e) => {
                  (e.target as HTMLImageElement).style.display = 'none'
                }}
              />
            ) : null}
            {/* 占位（无海报或海报加载失败时可见） */}
            <div className="absolute inset-0 -z-10 flex flex-col items-center justify-center gap-2"
              style={{
                background: 'linear-gradient(180deg, #1a1b2e 0%, #0f1019 100%)',
              }}
            >
              <div className="flex h-14 w-14 items-center justify-center rounded-2xl"
                style={{
                  background: 'linear-gradient(135deg, rgba(59,130,246,0.15), rgba(139,92,246,0.1))',
                  border: '1px solid rgba(59,130,246,0.1)',
                }}
              >
                {isSeries ? <Tv size={24} style={{ color: '#4a5568' }} /> : <Film size={24} style={{ color: '#4a5568' }} />}
              </div>
              <span className="text-xs font-medium" style={{ color: '#4a5568' }}>暂无海报</span>
            </div>

            {/* 悬停遮罩 */}
            <div className="gradient-overlay opacity-0 transition-opacity duration-300 group-hover:opacity-100">
              <div className="absolute bottom-3 left-3 right-3">
                <div className="flex items-center gap-2">
                  <button
                    onClick={handlePlayClick}
                    className="flex h-10 w-10 items-center justify-center rounded-full transition-all duration-300 hover:scale-125 cursor-pointer"
                    style={{
                      background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                      boxShadow: 'var(--neon-glow-shadow-lg)',
                    }}
                    title={isSeries ? '查看系列' : '立即播放'}
                  >
                    <Play size={18} className="ml-0.5 text-white" fill="white" />
                  </button>
                  <span className="text-sm font-semibold text-white">{isSeries ? '查看' : '播放'}</span>
                </div>
              </div>
              {/* 详情按钮（右下角，更显眼） */}
              <button
                onClick={(e) => { e.preventDefault(); e.stopPropagation(); navigate(detailTo) }}
                className="absolute top-2 right-2 flex h-8 w-8 items-center justify-center rounded-full transition-all duration-300 hover:scale-110 cursor-pointer"
                style={{
                  background: 'rgba(0,0,0,0.7)',
                  backdropFilter: 'blur(8px)',
                  border: '1px solid rgba(255,255,255,0.15)',
                }}
                title="查看详情"
              >
                <Info size={14} className="text-white" />
              </button>
            </div>

            {/* 分辨率标签（仅电影）— 使用 isolate 隔离 3D 变换影响 */}
            {!isSeries && media!.resolution && (
              <span className="badge-neon absolute right-2 top-2 z-20" style={{ transform: 'translateZ(0)' }}>
                {media!.resolution}
              </span>
            )}

            {/* 剧集合集标签 */}
            {isSeries && seriesData && seriesData.season_count > 0 && (
              <span className="absolute left-2 top-2 z-20 rounded-md px-2 py-0.5 text-xs font-medium backdrop-blur-md"
                style={{
                  background: 'rgba(0,0,0,0.65)',
                  color: 'rgba(255,255,255,0.9)',
                  border: '1px solid rgba(255,255,255,0.15)',
                  transform: 'translateZ(0)',
                }}
              >
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
          <div className="px-2 pt-2.5 pb-2">
            <h3
              className="truncate text-sm font-medium leading-snug text-theme-primary transition-colors duration-200 hover:text-neon cursor-pointer"
              onClick={(e) => { e.preventDefault(); e.stopPropagation(); navigate(detailTo) }}
              title={title}
            >
              {title}
            </h3>
            <div className="mt-1 flex items-center gap-1.5 text-xs text-theme-secondary overflow-hidden">
              {year > 0 && <span className="flex-shrink-0">{year}</span>}
              {rating > 0 && (
                <>
                  <span className="text-neon-blue/30 flex-shrink-0">·</span>
                  <span className="text-yellow-400 flex-shrink-0">★ {rating.toFixed(1)}</span>
                </>
              )}
              {!isSeries && media!.duration > 0 && (
                <>
                  <span className="text-neon-blue/30 flex-shrink-0">·</span>
                  <span className="flex-shrink-0">{formatDuration(media!.duration)}</span>
                </>
              )}
              {isSeries && seriesData && seriesData.episode_count > 0 && (
                <>
                  <span className="text-neon-blue/30 flex-shrink-0">·</span>
                  <span className="flex-shrink-0">{seriesData.episode_count} 集</span>
                </>
              )}
            </div>
          </div>
        </motion.div>
      </Link>
    </motion.div>
  )
}
