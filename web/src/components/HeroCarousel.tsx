// ============================================================
// HeroCarousel �?现代化全屏幻灯片轮播组件
// 深空流体 · 赛博朋克风格
// ============================================================
//
// 特性：
// - framer-motion 驱动的方向感知滑动切�?
// - 触摸手势拖拽支持（移动端左右滑动�?
// - 悬停自动暂停 + 进度条指�?
// - 键盘导航（← �?方向键）
// - 视差滚动背景效果
// - 响应式布局（移动端/桌面端差异化�?
// - reduce-motion 无障碍兼�?
// - 图片预加载策�?
// ============================================================

import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { Link } from 'react-router-dom'
import {
  motion,
  AnimatePresence,
  useMotionValue,
  useReducedMotion,
  type PanInfo,
} from 'framer-motion'
import { Play, Star, ChevronLeft, ChevronRight, Info, Pause } from 'lucide-react'
import { streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import type { RecommendedMedia, MixedItem, Media } from '@/types'
import { easeSmooth, easeExit, springDefault } from '@/lib/motion'
import clsx from 'clsx'

// ==================== 动画变体 ====================

/** 背景图切�?�?方向感知 + 缩放 */
const bgVariants = {
  enter: (dir: number) => ({
    opacity: 0,
    scale: 1.08,
    x: dir > 0 ? '4%' : '-4%',
  }),
  center: {
    opacity: 1,
    scale: 1,
    x: '0%',
    transition: {
      opacity: { duration: 0.8, ease: [0.22, 1, 0.36, 1] as [number, number, number, number] },
      scale: { duration: 1.2, ease: [0.22, 1, 0.36, 1] as [number, number, number, number] },
      x: { duration: 0.8, ease: [0.22, 1, 0.36, 1] as [number, number, number, number] },
    },
  },
  exit: (dir: number) => ({
    opacity: 0,
    scale: 1.04,
    x: dir > 0 ? '-4%' : '4%',
    transition: {
      opacity: { duration: 0.5, ease: [0.36, 0, 0.66, -0.56] as [number, number, number, number] },
      scale: { duration: 0.5 },
      x: { duration: 0.5 },
    },
  }),
}

/** 内容区入�?�?从底部上�?+ 模糊 */
const contentVariants = {
  enter: {
    opacity: 0,
    y: 30,
    filter: 'blur(6px)',
  },
  center: {
    opacity: 1,
    y: 0,
    filter: 'blur(0px)',
    transition: {
      duration: 0.6,
      delay: 0.25,
      ease: easeSmooth as unknown as [number, number, number, number],
      staggerChildren: 0.08,
      delayChildren: 0.3,
    },
  },
  exit: {
    opacity: 0,
    y: -15,
    filter: 'blur(3px)',
    transition: {
      duration: 0.3,
      ease: easeExit as unknown as [number, number, number, number],
    },
  },
}

/** 内容子元素交错入�?*/
const contentChildVariants = {
  enter: { opacity: 0, y: 12 },
  center: {
    opacity: 1,
    y: 0,
    transition: { duration: 0.4, ease: easeSmooth as unknown as [number, number, number, number] },
  },
  exit: { opacity: 0, y: -8, transition: { duration: 0.2 } },
}

// ==================== 常量 ====================
const AUTO_PLAY_INTERVAL = 7000 // 自动轮播间隔 7s
const SWIPE_THRESHOLD = 50 // 触摸滑动阈�?px
const SWIPE_VELOCITY = 300 // 触摸滑动速度阈�?

// ==================== 工具函数 ====================
/** �?MixedItem 转换�?HeroCarousel 可用�?RecommendedMedia 格式 */
function mixedItemToRecommended(item: MixedItem, fallbackReason: string): RecommendedMedia | null {
  if (item.type === 'movie' && item.media) {
    return { media: item.media, score: 0, reason: fallbackReason }
  }
  if (item.type === 'series' && item.series) {
    // �?Series 适配�?Media 接口（只填充轮播展示所需的字段）
    const s = item.series
    const pseudoMedia: Media = {
      id: s.id,
      library_id: s.library_id,
      title: s.title,
      orig_title: s.orig_title || '',
      year: s.year,
      overview: s.overview,
      poster_path: s.poster_path,
      backdrop_path: s.backdrop_path || '',
      rating: s.rating,
      genres: s.genres,
      media_type: 'episode',
      series_id: s.id,
      // 以下字段轮播不使用，填默认�?
      runtime: 0, file_path: '', file_size: 0, video_codec: '', audio_codec: '',
      resolution: '', duration: 0, subtitle_paths: '',
      tmdb_id: s.tmdb_id || 0, douban_id: s.douban_id || '', bangumi_id: s.bangumi_id || 0,
      country: s.country || '', language: s.language || '', tagline: '',
      studio: s.studio || '', trailer_url: '',
      season_num: 0, episode_num: 0, episode_title: '', created_at: s.created_at || '',
    }
    return { media: pseudoMedia, score: 0, reason: fallbackReason }
  }
  return null
}

// ==================== 组件 Props ====================
interface HeroCarouselProps {
  items: RecommendedMedia[]
  /** �?items 为空时的备选数据源（最近添加的混合列表�?*/
  fallbackItems?: MixedItem[]
  /** 最大展示数量，默认 5 */
  maxItems?: number
}

// ==================== 主组�?====================
export default function HeroCarousel({ items: rawItems, fallbackItems, maxItems = 5 }: HeroCarouselProps) {
  const { t } = useTranslation()
  const prefersReducedMotion = useReducedMotion()

  // 优先使用推荐数据；为空时�?fallback 数据
  const items = useMemo(() => {
    if (rawItems.length > 0) return rawItems.slice(0, maxItems)
    if (fallbackItems && fallbackItems.length > 0) {
      const converted = fallbackItems
        .slice(0, maxItems)
        .map(item => mixedItemToRecommended(item, t('home.recentlyAdded')))
        .filter((x): x is RecommendedMedia => x !== null)
      return converted
    }
    return []
  }, [rawItems, fallbackItems, maxItems, t])

  // ---- 状�?----
  const [current, setCurrent] = useState(0)
  const [direction, setDirection] = useState(1)
  const [isPaused, setIsPaused] = useState(false)
  const [isHovering, setIsHovering] = useState(false)
  const [loadedImages, setLoadedImages] = useState<Set<string>>(new Set())

  // ---- Refs ----
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const progressRef = useRef<HTMLDivElement>(null)
  const containerRef = useRef<HTMLDivElement>(null)

  // ---- 拖拽 Motion Value ----
  const dragX = useMotionValue(0)

  // ---- 海报 URL 缓存 ----
  const posterUrls = useMemo(() => {
    const urls = new Map<string, string>()
    items.forEach((rec) => {
      const url = rec.media.series_id
        ? streamApi.getSeriesPosterUrl(rec.media.series_id)
        : streamApi.getPosterUrl(rec.media.id)
      urls.set(rec.media.id, url)
    })
    return urls
  }, [items])

  // ---- 图片加载回调 ----
  const handleImageLoad = useCallback((mediaId: string) => {
    setLoadedImages((prev) => {
      if (prev.has(mediaId)) return prev
      const next = new Set(prev)
      next.add(mediaId)
      return next
    })
  }, [])

  // ---- 自动轮播控制 ----
  const startAutoPlay = useCallback(() => {
    if (timerRef.current) clearInterval(timerRef.current)
    if (items.length <= 1) return
    timerRef.current = setInterval(() => {
      setDirection(1)
      setCurrent((prev) => (prev + 1) % items.length)
    }, AUTO_PLAY_INTERVAL)
  }, [items.length])

  const stopAutoPlay = useCallback(() => {
    if (timerRef.current) {
      clearInterval(timerRef.current)
      timerRef.current = null
    }
  }, [])

  // 自动轮播 �?悬停或手动暂停时停止
  useEffect(() => {
    if (isPaused || isHovering) {
      stopAutoPlay()
    } else {
      startAutoPlay()
    }
    return stopAutoPlay
  }, [isPaused, isHovering, startAutoPlay, stopAutoPlay, current])

  // ---- 导航函数 ----
  const goTo = useCallback((index: number) => {
    setDirection(index > current ? 1 : index < current ? -1 : 1)
    setCurrent(index)
  }, [current])

  const goPrev = useCallback(() => {
    setDirection(-1)
    setCurrent((prev) => (prev - 1 + items.length) % items.length)
  }, [items.length])

  const goNext = useCallback(() => {
    setDirection(1)
    setCurrent((prev) => (prev + 1) % items.length)
  }, [items.length])

  // ---- 键盘导航 ----
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      // 只在轮播区域可见时响�?
      if (!containerRef.current) return
      const rect = containerRef.current.getBoundingClientRect()
      if (rect.bottom < 0 || rect.top > window.innerHeight) return

      if (e.key === 'ArrowLeft') {
        e.preventDefault()
        goPrev()
      } else if (e.key === 'ArrowRight') {
        e.preventDefault()
        goNext()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [goPrev, goNext])

  // ---- 触摸手势处理 ----
  const handleDragEnd = useCallback((_: unknown, info: PanInfo) => {
    const { offset, velocity } = info
    if (Math.abs(offset.x) > SWIPE_THRESHOLD || Math.abs(velocity.x) > SWIPE_VELOCITY) {
      if (offset.x > 0) {
        goPrev()
      } else {
        goNext()
      }
    }
  }, [goPrev, goNext])

  // ---- 进度条动�?----
  useEffect(() => {
    if (!progressRef.current || isPaused || isHovering || items.length <= 1) return
    const el = progressRef.current
    // 重置动画
    el.style.transition = 'none'
    el.style.width = '0%'
    // 强制 reflow
    void el.offsetWidth
    // 启动动画
    el.style.transition = `width ${AUTO_PLAY_INTERVAL}ms linear`
    el.style.width = '100%'
  }, [current, isPaused, isHovering, items.length])

  // ---- 安全检�?----
  if (!items.length) return null
  const item = items[current]
  if (!item) return null

  // ---- 链接计算 ----
  const playLink = item.media.media_type === 'episode' && item.media.series_id
    ? `/series/${item.media.series_id}`
    : `/play/${item.media.id}`
  const detailLink = item.media.series_id
    ? `/series/${item.media.series_id}`
    : `/media/${item.media.id}`

  return (
    <section
      ref={containerRef}
      className="-mx-4 -mt-6 mb-6 sm:-mx-6 lg:-mx-8"
      role="region"
      aria-roledescription="carousel"
      aria-label={t('home.recommended')}
      onMouseEnter={() => setIsHovering(true)}
      onMouseLeave={() => setIsHovering(false)}
    >
      <div className="relative h-[320px] overflow-hidden rounded-b-2xl sm:h-[380px] lg:h-[440px] xl:h-[480px]">

        {/* ==================== 背景图层 ==================== */}
        <AnimatePresence initial={false} custom={direction} mode="popLayout">
          <motion.div
            key={`bg-${current}`}
            custom={direction}
            variants={prefersReducedMotion ? undefined : bgVariants}
            initial={prefersReducedMotion ? { opacity: 0 } : 'enter'}
            animate={prefersReducedMotion ? { opacity: 1 } : 'center'}
            exit={prefersReducedMotion ? { opacity: 0 } : 'exit'}
            className="absolute inset-0"
            // 触摸拖拽
            drag={items.length > 1 ? 'x' : false}
            dragConstraints={{ left: 0, right: 0 }}
            dragElastic={0.15}
            onDragEnd={handleDragEnd}
            style={{ x: dragX, cursor: items.length > 1 ? 'grab' : 'default' }}
          >
            <motion.img
              src={posterUrls.get(item.media.id)}
              alt=""
              className={clsx(
                'h-full w-full object-cover select-none pointer-events-none',
                loadedImages.has(item.media.id) ? 'opacity-100' : 'opacity-0'
              )}
              style={{ transition: 'opacity 0.4s' }}
              loading="eager"
              draggable={false}
              onLoad={() => handleImageLoad(item.media.id)}
              onError={(e) => { (e.target as HTMLImageElement).style.display = 'none' }}
            />
          </motion.div>
        </AnimatePresence>

        {/* 预加载相邻图�?*/}
        {items.map((rec, i) => i !== current && (
          <img
            key={`preload-${rec.media.id}`}
            src={posterUrls.get(rec.media.id)}
            alt=""
            className="hidden"
            loading="lazy"
            onLoad={() => handleImageLoad(rec.media.id)}
          />
        ))}

        {/* ==================== 渐变遮罩�?==================== */}
        {/* 底部渐变 �?确保文字可读 */}
        <div
          className="pointer-events-none absolute inset-0"
          style={{
            background: `
              linear-gradient(to top, var(--bg-base) 0%, var(--bg-base) 5%, rgba(6,10,19,0.7) 35%, transparent 70%),
              linear-gradient(to right, var(--bg-base) 0%, rgba(6,10,19,0.5) 40%, transparent 70%)
            `,
          }}
        />
        {/* 顶部微弱渐变 */}
        <div
          className="pointer-events-none absolute inset-x-0 top-0 h-24"
          style={{
            background: 'linear-gradient(to bottom, var(--bg-base)/40, transparent)',
          }}
        />

        {/* ==================== 内容�?==================== */}
        <div className="absolute bottom-0 left-0 right-0 px-4 pb-10 sm:px-6 lg:px-8">
          <div className="mx-auto max-w-7xl">
            <AnimatePresence mode="wait" initial={false}>
              <motion.div
                key={`content-${current}`}
                variants={prefersReducedMotion ? undefined : contentVariants}
                initial={prefersReducedMotion ? { opacity: 0 } : 'enter'}
                animate={prefersReducedMotion ? { opacity: 1 } : 'center'}
                exit={prefersReducedMotion ? { opacity: 0 } : 'exit'}
                className="max-w-2xl"
              >
                {/* 推荐理由标签 */}
                <motion.div variants={contentChildVariants}>
                  <span className="badge-accent mb-3 inline-flex items-center gap-1.5 text-xs">
                    <Star size={10} fill="currentColor" className="opacity-70" />
                    {item.reason}
                  </span>
                </motion.div>

                {/* 标题 */}
                <motion.h2
                  variants={contentChildVariants}
                  className="mb-2 font-display text-2xl font-bold tracking-wide text-white drop-shadow-lg sm:text-3xl lg:text-4xl xl:text-5xl"
                >
                  {item.media.title}
                </motion.h2>

                {/* 元数据行 */}
                <motion.div
                  variants={contentChildVariants}
                  className="mb-3 flex flex-wrap items-center gap-3 text-sm text-white/70"
                >
                  {item.media.year > 0 && (
                    <span className="rounded-md bg-white/10 px-2 py-0.5 text-xs font-medium backdrop-blur-sm">
                      {item.media.year}
                    </span>
                  )}
                  {item.media.rating > 0 && (
                    <span className="flex items-center gap-1 text-yellow-400">
                      <Star size={13} fill="currentColor" />
                      <span className="font-semibold">{item.media.rating.toFixed(1)}</span>
                    </span>
                  )}
                  {item.media.genres && (
                    <span className="text-white/50">
                      {item.media.genres.split(',').slice(0, 3).join(' / ')}
                    </span>
                  )}
                </motion.div>

                {/* 简�?*/}
                {item.media.overview && (
                  <motion.p
                    variants={contentChildVariants}
                    className="mb-5 line-clamp-2 max-w-xl text-sm leading-relaxed text-white/55 sm:text-base sm:leading-relaxed"
                  >
                    {item.media.overview}
                  </motion.p>
                )}

                {/* 操作按钮 */}
                <motion.div variants={contentChildVariants} className="flex items-center gap-3">
                  {/* 播放按钮 �?�?CTA */}
                  <motion.div
                    whileHover={{ scale: 1.04, y: -2 }}
                    whileTap={{ scale: 0.96 }}
                    transition={springDefault}
                  >
                    <Link
                      to={playLink}
                      className="group inline-flex items-center gap-2.5 rounded-2xl px-7 py-3 text-sm font-bold shadow-lg"
                      style={{
                        background: 'linear-gradient(135deg, var(--neon-blue), rgba(0, 180, 220, 0.95))',
                        boxShadow: 'var(--shadow-neon), 0 4px 15px var(--neon-blue-15)',
                        color: 'var(--text-on-neon)',
                      }}
                    >
                      <Play size={18} fill="currentColor" />
                      {t('home.playNow')}
                    </Link>
                  </motion.div>

                  {/* 详情按钮 */}
                  <motion.div
                    whileHover={{ scale: 1.04 }}
                    whileTap={{ scale: 0.96 }}
                    transition={springDefault}
                  >
                    <Link
                      to={detailLink}
                      className="inline-flex items-center gap-2 rounded-2xl px-5 py-3 text-sm font-medium text-white/80 transition-colors hover:text-white"
                      style={{
                        background: 'rgba(255,255,255,0.08)',
                        border: '1px solid rgba(255,255,255,0.12)',
                        backdropFilter: 'blur(8px)',
                      }}
                    >
                      <Info size={16} />
                      {t('home.viewDetail')}
                    </Link>
                  </motion.div>
                </motion.div>
              </motion.div>
            </AnimatePresence>
          </div>
        </div>

        {/* ==================== 左右切换按钮 ==================== */}
        {items.length > 1 && (
          <>
            <motion.button
              onClick={goPrev}
              className="absolute left-3 top-1/2 z-10 -translate-y-1/2 rounded-full p-2.5 sm:left-5"
              style={{
                background: 'rgba(0,0,0,0.3)',
                backdropFilter: 'blur(8px)',
                border: '1px solid rgba(255,255,255,0.08)',
              }}
              initial={{ opacity: 0 }}
              animate={{ opacity: isHovering ? 1 : 0 }}
              whileHover={{ scale: 1.1, background: 'rgba(0,0,0,0.5)' }}
              whileTap={{ scale: 0.9 }}
              transition={{ duration: 0.2 }}
              aria-label="上一个"
            >
              <ChevronLeft size={22} className="text-white/80" />
            </motion.button>
            <motion.button
              onClick={goNext}
              className="absolute right-3 top-1/2 z-10 -translate-y-1/2 rounded-full p-2.5 sm:right-5"
              style={{
                background: 'rgba(0,0,0,0.3)',
                backdropFilter: 'blur(8px)',
                border: '1px solid rgba(255,255,255,0.08)',
              }}
              initial={{ opacity: 0 }}
              animate={{ opacity: isHovering ? 1 : 0 }}
              whileHover={{ scale: 1.1, background: 'rgba(0,0,0,0.5)' }}
              whileTap={{ scale: 0.9 }}
              transition={{ duration: 0.2 }}
              aria-label="下一个"
            >
              <ChevronRight size={22} className="text-white/80" />
            </motion.button>
          </>
        )}

        {/* ==================== 底部控制�?==================== */}
        {items.length > 1 && (
          <div className="absolute bottom-0 left-0 right-0 z-10">
            {/* 进度�?*/}
            <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
              <div className="flex items-center gap-3">
                {/* 暂停/播放按钮 */}
                <motion.button
                  onClick={() => setIsPaused(!isPaused)}
                  className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full text-white/50 transition-colors hover:text-white/80"
                  whileTap={{ scale: 0.85 }}
                  aria-label={isPaused ? '播放' : '暂停'}
                >
                  {isPaused ? <Play size={10} fill="currentColor" /> : <Pause size={10} />}
                </motion.button>

                {/* 指示�?+ 进度�?*/}
                <div className="flex flex-1 items-center gap-1.5">
                  {items.map((_, i) => (
                    <button
                      key={i}
                      onClick={() => goTo(i)}
                      className="group relative h-6 flex-1 cursor-pointer"
aria-label={`第 ${i + 1} 张`}
                      aria-current={i === current ? 'true' : undefined}
                    >
                      {/* 轨道背景 */}
                      <div
                        className="absolute inset-x-0 top-1/2 h-[3px] -translate-y-1/2 rounded-full transition-all duration-200"
                        style={{
                          background: i === current
                            ? 'rgba(255,255,255,0.15)'
                            : 'rgba(255,255,255,0.08)',
                        }}
                      />
                      {/* 填充进度 */}
                      {i === current && (
                        <div
                          ref={progressRef}
                          className="absolute inset-y-0 left-0 top-1/2 h-[3px] -translate-y-1/2 rounded-full"
                          style={{
                            background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                            boxShadow: '0 0 6px var(--neon-blue-30)',
                            width: '0%',
                          }}
                        />
                      )}
                      {/* 已完成的指示�?*/}
                      {i < current && (
                        <div
                          className="absolute inset-x-0 top-1/2 h-[3px] -translate-y-1/2 rounded-full"
                          style={{
                            background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                            opacity: 0.5,
                          }}
                        />
                      )}
                    </button>
                  ))}
                </div>

                {/* 计数�?*/}
                <span className="shrink-0 text-[10px] font-medium tabular-nums text-white/40">
                  {String(current + 1).padStart(2, '0')}/{String(items.length).padStart(2, '0')}
                </span>
              </div>
            </div>

            {/* 底部间距 */}
            <div className="h-3" />
          </div>
        )}

        {/* ==================== 右侧缩略图预览（仅桌面端�?==================== */}
        {items.length > 1 && (
          <div className="pointer-events-auto absolute bottom-16 right-4 z-10 hidden items-end gap-2 lg:flex lg:right-6 xl:right-8">
            {items.map((rec, i) => (
              <motion.button
                key={rec.media.id}
                onClick={() => goTo(i)}
                className="relative overflow-hidden rounded-lg"
                style={{
                  width: i === current ? 72 : 48,
                  height: i === current ? 48 : 32,
                  border: i === current
                    ? '2px solid var(--neon-blue)'
                    : '1px solid rgba(255,255,255,0.12)',
                  boxShadow: i === current ? 'var(--neon-glow-shadow-md)' : 'none',
                }}
                animate={{
                  width: i === current ? 72 : 48,
                  height: i === current ? 48 : 32,
                  opacity: i === current ? 1 : 0.5,
                }}
                whileHover={{ opacity: 1, scale: 1.08 }}
                transition={{ type: 'spring', stiffness: 300, damping: 25 }}
                aria-label={rec.media.title}
              >
                <img
                  src={posterUrls.get(rec.media.id)}
                  alt=""
                  className="h-full w-full object-cover"
                  loading="lazy"
                  draggable={false}
                />
                {/* 当前项高亮边框光�?*/}
                {i === current && (
                  <motion.div
                    className="absolute inset-0 rounded-lg"
                    style={{
                      boxShadow: 'inset 0 0 12px var(--neon-blue-20)',
                    }}
                    layoutId="thumb-highlight"
                  />
                )}
              </motion.button>
            ))}
          </div>
        )}
      </div>
    </section>
  )
}
