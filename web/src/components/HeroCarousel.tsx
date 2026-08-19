import { useState, useEffect, useCallback, useRef, useMemo, type SyntheticEvent } from 'react'
import { Link } from 'react-router-dom'
import { AnimatePresence, motion, useReducedMotion, type PanInfo } from 'framer-motion'
import { ChevronLeft, ChevronRight, Info, Play } from 'lucide-react'
import { streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import type { RecommendedMedia, MixedItem, Media } from '@/types'
import { Button, buttonClassName } from '@/components/design-system'
import { MediaHeroContent } from '@/ui'

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

interface HeroArtwork {
  primary: string
  fallback?: string
  isBackdrop: boolean
}

function getHeroArtwork(media: Media): HeroArtwork {
  if (media.series_id) {
    if (media.backdrop_path) {
      return {
        primary: streamApi.getSeriesBackdropUrl(media.series_id),
        fallback: streamApi.getSeriesPosterUrl(media.series_id),
        isBackdrop: true,
      }
    }
    return {
      primary: streamApi.getSeriesPosterUrl(media.series_id),
      isBackdrop: false,
    }
  }

  return {
    primary: streamApi.withTokenUrl(`/api/media/${media.id}/backdrop`),
    fallback: streamApi.getPosterUrl(media.id),
    isBackdrop: true,
  }
}

function handleArtworkError(event: SyntheticEvent<HTMLImageElement>, fallback?: string) {
  const image = event.currentTarget
  if (fallback && image.dataset.fallbackApplied !== 'true') {
    image.dataset.fallbackApplied = 'true'
    image.src = fallback
    image.classList.add('scale-110', 'blur-2xl')
    return
  }
  image.style.display = 'none'
}

interface HeroCarouselProps {
  items: RecommendedMedia[]
  fallbackItems?: MixedItem[]
  maxItems?: number
  progressByMediaId?: Record<string, number>
}

export default function HeroCarousel({
  items: rawItems,
  fallbackItems,
  maxItems = 5,
  progressByMediaId = {},
}: HeroCarouselProps) {
  const { t } = useTranslation()
  const prefersReducedMotion = useReducedMotion()
  const containerRef = useRef<HTMLElement>(null)
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
    if (items.length <= 1 || isHovering) return

    timerRef.current = setInterval(goNext, AUTO_PLAY_INTERVAL)
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [goNext, isHovering, items.length, current])

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

  const artwork = getHeroArtwork(item.media)
  const playLink = item.media.media_type === 'episode' && item.media.series_id
    ? `/series/${item.media.series_id}`
    : `/play/${item.media.id}`
  const detailLink = item.media.series_id
    ? `/series/${item.media.series_id}`
    : `/media/${item.media.id}`
  const progress = Math.max(0, Math.min(100, progressByMediaId[item.media.id] || 0))
  const eyebrow = item.media.orig_title && item.media.orig_title !== item.media.title
    ? item.media.orig_title
    : item.reason

  return (
    <section
      ref={containerRef}
      className="nv-hero-carousel relative isolate overflow-hidden rounded-[var(--nv-radius-hero)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] shadow-[var(--nv-shadow-card)]"
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
            src={artwork.primary}
            alt=""
            data-artwork-kind={artwork.isBackdrop ? 'backdrop' : 'poster'}
            className={`h-full w-full select-none object-cover object-center${artwork.isBackdrop ? '' : ' scale-110 blur-2xl'}`}
            loading="eager"
            draggable={false}
            onError={(event) => handleArtworkError(event, artwork.fallback)}
          />
        </motion.div>
      </AnimatePresence>

      {items.map((recommendation, index) => {
        if (index === current) return null
        const preloadArtwork = getHeroArtwork(recommendation.media)
        return (
          <img
            key={`hero-preload-${recommendation.media.id}`}
            src={preloadArtwork.primary}
            alt=""
            className="hidden"
            loading="lazy"
            onError={(event) => handleArtworkError(event, preloadArtwork.fallback)}
          />
        )
      })}

      <div className="pointer-events-none absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
      <div className="pointer-events-none absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />

      <div className="nv-home-hero-content relative z-10 flex flex-col justify-center">
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={`hero-content-${item.media.id}`}
            initial={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, y: -6 }}
            transition={{ duration: prefersReducedMotion ? 0.12 : 0.25 }}
          >
            <MediaHeroContent
              media={item.media}
              eyebrow={eyebrow}
              subtitle={false}
              supplemental={progress > 0 ? (
                <div className="nv-home-hero-watch-progress" aria-label={`已观看 ${progress}%`}>
                  <div className="nv-home-hero-watch-progress-label">已观看 {progress}%</div>
                  <div className="nv-home-hero-watch-progress-track">
                    <span style={{ width: `${progress}%` }} />
                  </div>
                </div>
              ) : undefined}
              actions={(
                <>
                  <Link
                    to={playLink}
                    className={buttonClassName({ variant: 'primary', size: 'lg' })}
                    data-variant="primary"
                    data-size="lg"
                  >
                    <Play size={16} fill="currentColor" aria-hidden="true" />
                    {progress > 0 ? '继续播放' : t('home.playNow')}
                  </Link>
                  <Link
                    to={detailLink}
                    className={buttonClassName({ variant: 'secondary', size: 'lg' })}
                    data-variant="secondary"
                    data-size="lg"
                  >
                    <Info size={16} aria-hidden="true" />
                    {t('home.viewDetail')}
                  </Link>
                </>
              )}
            />
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
            className="nv-home-hero-arrow nv-home-hero-arrow--left"
            aria-label="上一个"
          >
            <ChevronLeft size={18} aria-hidden="true" />
          </Button>
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={goNext}
            className="nv-home-hero-arrow nv-home-hero-arrow--right"
            aria-label="下一个"
          >
            <ChevronRight size={18} aria-hidden="true" />
          </Button>
        </>
      )}

      {items.length > 1 && (
        <div className="nv-home-hero-dots" role="tablist" aria-label="精选内容">
          {items.map((recommendation, index) => (
            <button
              key={recommendation.media.id}
              type="button"
              onClick={() => goTo(index)}
              className="nv-home-hero-dot"
              aria-label={`第 ${index + 1} 张：${recommendation.media.title}`}
              aria-selected={index === current}
              role="tab"
            />
          ))}
        </div>
      )}
    </section>
  )
}
