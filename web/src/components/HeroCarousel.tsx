import { useState, useEffect, useCallback, useRef, useMemo } from 'react'
import { Link } from 'react-router-dom'
import { AnimatePresence, motion, useReducedMotion, type PanInfo } from 'framer-motion'
import { ChevronLeft, ChevronRight, Info, Pause, Play, Star } from 'lucide-react'
import { streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import type { RecommendedMedia, MixedItem, Media } from '@/types'
import { Button, Tag, buttonClassName } from '@/components/design-system'

const AUTO_PLAY_INTERVAL = 7000
const SWIPE_THRESHOLD = 50
const SWIPE_VELOCITY = 300

function mixedItemToRecommended(item: MixedItem, fallbackReason: string): RecommendedMedia | null {
  if (item.type === 'movie' && item.media) {
    return { media: item.media, score: 0, reason: fallbackReason }
  }
  if (item.type === 'series' && item.series) {
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
      runtime: 0,
      file_path: '',
      file_size: 0,
      video_codec: '',
      audio_codec: '',
      resolution: '',
      duration: 0,
      subtitle_paths: '',
      tmdb_id: s.tmdb_id || 0,
      douban_id: s.douban_id || '',
      bangumi_id: s.bangumi_id || 0,
      country: s.country || '',
      language: s.language || '',
      tagline: '',
      studio: s.studio || '',
      trailer_url: '',
      num: '',
      sort_title: '',
      outline: '',
      original_plot: '',
      mpaa: '',
      country_code: '',
      maker: '',
      publisher: '',
      label: '',
      tags: '',
      website: '',
      release_date: '',
      premiered: '',
      season_num: 0,
      episode_num: 0,
      episode_title: '',
      created_at: s.created_at || '',
    }
    return { media: pseudoMedia, score: 0, reason: fallbackReason }
  }
  return null
}

interface HeroCarouselProps {
  items: RecommendedMedia[]
  fallbackItems?: MixedItem[]
  maxItems?: number
}

export default function HeroCarousel({ items: rawItems, fallbackItems, maxItems = 5 }: HeroCarouselProps) {
  const { t } = useTranslation()
  const prefersReducedMotion = useReducedMotion()
  const containerRef = useRef<HTMLElement>(null)
  const progressRef = useRef<HTMLDivElement>(null)
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const items = useMemo(() => {
    if (rawItems.length > 0) return rawItems.slice(0, maxItems)
    return (fallbackItems || [])
      .slice(0, maxItems)
      .map((item) => mixedItemToRecommended(item, t('home.recentlyAdded')))
      .filter((item): item is RecommendedMedia => item !== null)
  }, [rawItems, fallbackItems, maxItems, t])

  const [current, setCurrent] = useState(0)
  const [direction, setDirection] = useState(1)
  const [isPaused, setIsPaused] = useState(false)
  const [isHovering, setIsHovering] = useState(false)

  useEffect(() => {
    if (current >= items.length && items.length > 0) setCurrent(0)
  }, [current, items.length])

  const goPrev = useCallback(() => {
    if (!items.length) return
    setDirection(-1)
    setCurrent((value) => (value - 1 + items.length) % items.length)
  }, [items.length])

  const goNext = useCallback(() => {
    if (!items.length) return
    setDirection(1)
    setCurrent((value) => (value + 1) % items.length)
  }, [items.length])

  const goTo = useCallback((index: number) => {
    setDirection(index >= current ? 1 : -1)
    setCurrent(index)
  }, [current])

  useEffect(() => {
    if (timerRef.current) clearInterval(timerRef.current)
    if (items.length <= 1 || isPaused || isHovering) return

    timerRef.current = setInterval(goNext, AUTO_PLAY_INTERVAL)
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [goNext, isHovering, isPaused, items.length, current])

  useEffect(() => {
    if (!progressRef.current || isPaused || isHovering || items.length <= 1) return
    const progress = progressRef.current
    progress.style.transition = 'none'
    progress.style.width = '0%'
    void progress.offsetWidth
    progress.style.transition = `width ${AUTO_PLAY_INTERVAL}ms linear`
    progress.style.width = '100%'
  }, [current, isPaused, isHovering, items.length])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      const container = containerRef.current
      if (!container) return
      const rect = container.getBoundingClientRect()
      if (rect.bottom < 0 || rect.top > window.innerHeight) return

      if (event.key === 'ArrowLeft') {
        event.preventDefault()
        goPrev()
      } else if (event.key === 'ArrowRight') {
        event.preventDefault()
        goNext()
      }
    }
    window.addEventListener('keydown', handleKeyDown)
    return () => window.removeEventListener('keydown', handleKeyDown)
  }, [goNext, goPrev])

  const handleDragEnd = useCallback((_: unknown, info: PanInfo) => {
    if (Math.abs(info.offset.x) > SWIPE_THRESHOLD || Math.abs(info.velocity.x) > SWIPE_VELOCITY) {
      if (info.offset.x > 0) goPrev()
      else goNext()
    }
  }, [goNext, goPrev])

  if (!items.length) return null
  const item = items[current]
  if (!item) return null

  const imageUrl = item.media.series_id
    ? streamApi.getSeriesPosterUrl(item.media.series_id)
    : streamApi.getPosterUrl(item.media.id)
  const playLink = item.media.media_type === 'episode' && item.media.series_id
    ? `/series/${item.media.series_id}`
    : `/play/${item.media.id}`
  const detailLink = item.media.series_id
    ? `/series/${item.media.series_id}`
    : `/media/${item.media.id}`

  return (
    <section
      ref={containerRef}
      className="relative isolate min-h-[340px] overflow-hidden rounded-[var(--nv-radius-hero)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] shadow-[var(--nv-shadow-card)] sm:min-h-[400px] lg:min-h-[460px]"
      role="region"
      aria-roledescription="carousel"
      aria-label={t('home.recommended')}
      onMouseEnter={() => setIsHovering(true)}
      onMouseLeave={() => setIsHovering(false)}
    >
      <AnimatePresence initial={false} custom={direction} mode="sync">
        <motion.div
          key={`hero-${item.media.id}`}
          className="absolute inset-0"
          initial={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, scale: 1.025, x: direction > 0 ? '2%' : '-2%' }}
          animate={{ opacity: 1, scale: 1, x: 0 }}
          exit={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, scale: 1.01, x: direction > 0 ? '-1%' : '1%' }}
          transition={{ duration: prefersReducedMotion ? 0.12 : 0.42, ease: [0.2, 0.8, 0.2, 1] }}
          drag={items.length > 1 ? 'x' : false}
          dragConstraints={{ left: 0, right: 0 }}
          dragElastic={0.12}
          onDragEnd={handleDragEnd}
        >
          <img
            src={imageUrl}
            alt=""
            className="h-full w-full select-none object-cover object-center"
            loading="eager"
            draggable={false}
            onError={(event) => { (event.currentTarget as HTMLImageElement).style.display = 'none' }}
          />
        </motion.div>
      </AnimatePresence>

      {items.map((recommendation, index) => index !== current && (
        <img
          key={`hero-preload-${recommendation.media.id}`}
          src={recommendation.media.series_id
            ? streamApi.getSeriesPosterUrl(recommendation.media.series_id)
            : streamApi.getPosterUrl(recommendation.media.id)}
          alt=""
          className="hidden"
          loading="lazy"
        />
      ))}

      <div className="pointer-events-none absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
      <div className="pointer-events-none absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />
      <div
        className="pointer-events-none absolute inset-0 opacity-70"
        style={{
          background: 'radial-gradient(circle at 72% 18%, var(--nv-ambient-purple-soft), transparent 36rem)',
        }}
      />

      <div className="relative z-10 flex min-h-[340px] max-w-[48rem] flex-col justify-end px-[var(--nv-page-gutter)] pb-12 pt-16 sm:min-h-[400px] sm:pb-14 lg:min-h-[460px] lg:pb-16">
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={`hero-content-${item.media.id}`}
            initial={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, y: 14 }}
            animate={{ opacity: 1, y: 0 }}
            exit={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, y: -8 }}
            transition={{ duration: prefersReducedMotion ? 0.12 : 0.28 }}
          >
            {item.reason && (
              <Tag tone="brand" className="mb-3">
                {item.reason}
              </Tag>
            )}

            <h2
              className="max-w-[18ch] font-bold text-[var(--nv-text-primary)]"
              style={{
                fontSize: 'var(--nv-type-display)',
                lineHeight: 'var(--nv-line-tight)',
                letterSpacing: 'var(--nv-tracking-tight)',
              }}
            >
              {item.media.title}
            </h2>

            <div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2 text-sm text-[var(--nv-text-secondary)]">
              {item.media.year > 0 && <span>{item.media.year}</span>}
              {item.media.rating > 0 && (
                <span className="inline-flex items-center gap-1 text-[var(--nv-status-rating)]">
                  <Star size={13} fill="currentColor" aria-hidden="true" />
                  <span className="font-semibold">{item.media.rating.toFixed(1)}</span>
                </span>
              )}
              {item.media.genres && (
                <span className="text-[var(--nv-text-tertiary)]">
                  {item.media.genres.split(',').slice(0, 3).join(' · ')}
                </span>
              )}
            </div>

            {item.media.overview && (
              <p className="mt-3 line-clamp-2 max-w-2xl text-sm leading-6 text-[var(--nv-text-secondary)] sm:text-[var(--nv-type-body)]">
                {item.media.overview}
              </p>
            )}

            <div className="mt-5 flex flex-wrap items-center gap-3">
              <Link
                to={playLink}
                className={buttonClassName({ variant: 'primary', size: 'lg' })}
                data-variant="primary"
                data-size="lg"
              >
                <Play size={18} fill="currentColor" aria-hidden="true" />
                {t('home.playNow')}
              </Link>
              <Link
                to={detailLink}
                className={buttonClassName({ variant: 'secondary', size: 'lg' })}
                data-variant="secondary"
                data-size="lg"
              >
                <Info size={17} aria-hidden="true" />
                {t('home.viewDetail')}
              </Link>
            </div>
          </motion.div>
        </AnimatePresence>
      </div>

      {items.length > 1 && (
        <>
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={goPrev}
            className="absolute left-3 top-1/2 z-20 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] transition-opacity group-hover:opacity-100 sm:left-5"
            style={{ opacity: isHovering ? 1 : undefined }}
            aria-label="上一个"
          >
            <ChevronLeft size={19} aria-hidden="true" />
          </Button>
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={goNext}
            className="absolute right-3 top-1/2 z-20 -translate-y-1/2 opacity-0 shadow-[var(--nv-shadow-card)] transition-opacity sm:right-5"
            style={{ opacity: isHovering ? 1 : undefined }}
            aria-label="下一个"
          >
            <ChevronRight size={19} aria-hidden="true" />
          </Button>
        </>
      )}

      {items.length > 1 && (
        <div className="absolute inset-x-[var(--nv-page-gutter)] bottom-4 z-20 flex items-center gap-3">
          <Button
            variant="ghost"
            size="sm"
            iconOnly
            onClick={() => setIsPaused((value) => !value)}
            aria-label={isPaused ? '继续轮播' : '暂停轮播'}
          >
            {isPaused ? <Play size={12} fill="currentColor" aria-hidden="true" /> : <Pause size={12} aria-hidden="true" />}
          </Button>

          <div className="flex min-w-0 flex-1 items-center gap-1.5">
            {items.map((recommendation, index) => (
              <button
                key={recommendation.media.id}
                onClick={() => goTo(index)}
                className="relative h-6 flex-1 cursor-pointer"
                aria-label={`第 ${index + 1} 张：${recommendation.media.title}`}
                aria-current={index === current ? 'true' : undefined}
              >
                <span className="absolute inset-x-0 top-1/2 h-[3px] -translate-y-1/2 rounded-full bg-black/15 dark:bg-white/10" />
                {index < current && (
                  <span className="absolute inset-x-0 top-1/2 h-[3px] -translate-y-1/2 rounded-full bg-[var(--nv-action-primary)] opacity-55" />
                )}
                {index === current && (
                  <span
                    ref={progressRef}
                    className="absolute left-0 top-1/2 h-[3px] -translate-y-1/2 rounded-full bg-[var(--nv-action-primary)]"
                    style={{ width: '0%' }}
                  />
                )}
              </button>
            ))}
          </div>

          <span className="shrink-0 text-[10px] font-medium tabular-nums text-[var(--nv-text-tertiary)]">
            {String(current + 1).padStart(2, '0')}/{String(items.length).padStart(2, '0')}
          </span>
        </div>
      )}
    </section>
  )
}
