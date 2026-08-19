import { useState, useEffect, useCallback, useRef, useMemo, type SyntheticEvent } from 'react'
import { Link } from 'react-router-dom'
import { AnimatePresence, motion, useReducedMotion, type PanInfo } from 'framer-motion'
import { ChevronLeft, ChevronRight, Film, Heart, Play, RotateCcw } from 'lucide-react'
import { streamApi } from '@/api'
import { useTranslation } from '@/i18n'
import type { RecommendedMedia, MixedItem, Media } from '@/types'
import { Button, buttonClassName } from '@/components/design-system'
import { MediaArtwork, MediaHeroContent } from '@/ui'

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

interface HeroWatchState {
  position: number
  duration: number
}

function isSeriesProxy(media: Media) {
  return Boolean(media.series_id && media.series_id === media.id)
}

function getHeroPoster(media: Media): string | null {
  if (media.series_id && (media.series?.poster_path || isSeriesProxy(media))) {
    return streamApi.getSeriesPosterUrl(media.series_id)
  }
  if (media.poster_path) return streamApi.getPosterUrl(media.id)
  if (media.series_id) return streamApi.getSeriesPosterUrl(media.series_id)
  return null
}

function getHeroArtwork(media: Media): HeroArtwork {
  const fallback = getHeroPoster(media) || undefined

  if (media.series_id && (media.series?.backdrop_path || (isSeriesProxy(media) && media.backdrop_path))) {
    return {
      primary: streamApi.getSeriesBackdropUrl(media.series_id),
      fallback,
      isBackdrop: true,
    }
  }

  if (media.backdrop_path) {
    return {
      primary: streamApi.getBackdropUrl(media.id),
      fallback,
      isBackdrop: true,
    }
  }

  if (media.series_id) {
    return {
      primary: streamApi.getSeriesBackdropUrl(media.series_id),
      fallback,
      isBackdrop: true,
    }
  }

  return {
    primary: streamApi.getBackdropUrl(media.id),
    fallback,
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

function formatClock(seconds: number) {
  const value = Math.max(0, Math.floor(seconds || 0))
  const hours = Math.floor(value / 3600)
  const minutes = Math.floor((value % 3600) / 60)
  const secs = value % 60
  return hours > 0
    ? `${hours}:${String(minutes).padStart(2, '0')}:${String(secs).padStart(2, '0')}`
    : `${minutes}:${String(secs).padStart(2, '0')}`
}

interface HeroCarouselProps {
  items: RecommendedMedia[]
  fallbackItems?: MixedItem[]
  maxItems?: number
  watchStateByMediaId?: Record<string, HeroWatchState>
}

export default function HeroCarousel({
  items: rawItems,
  fallbackItems,
  maxItems = 5,
  watchStateByMediaId = {},
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
  const poster = getHeroPoster(item.media)
  const playLink = item.media.media_type === 'episode' && item.media.series_id
    ? `/series/${item.media.series_id}`
    : `/play/${item.media.id}`
  const watchState = watchStateByMediaId[item.media.id]
  const progress = watchState?.duration > 0
    ? Math.max(0, Math.min(100, Math.round((watchState.position / watchState.duration) * 100)))
    : 0

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

      <div className="pointer-events-none absolute inset-0" style={{ background: 'var(--nv-hero-scrim)' }} />
      <div className="pointer-events-none absolute inset-0" style={{ background: 'var(--nv-hero-bottom-scrim)' }} />

      <div className="nv-home-hero-foreground relative z-10">
        <MediaArtwork
          src={poster}
          alt=""
          ratio="poster"
          className="nv-home-hero-poster"
          imageClassName="nv-home-hero-poster-image"
          fallback={<Film size={26} aria-hidden="true" />}
        />

        <div className="nv-home-hero-content flex min-w-0 flex-col justify-center">
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
                inlineBadges
                supplemental={watchState?.duration > 0 ? (
                  <div className="nv-home-hero-watch-progress" aria-label={`已观看 ${progress}%`}>
                    <div className="nv-home-hero-watch-progress-label">
                      上次观看至 {formatClock(watchState.position)} / {formatClock(watchState.duration)}
                    </div>
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
                      {watchState ? '继续播放' : t('home.playNow')}
                    </Link>
                    <Link
                      to={`${playLink}?restart=1`}
                      className={buttonClassName({ variant: 'secondary', size: 'lg' })}
                      data-variant="secondary"
                      data-size="lg"
                    >
                      <RotateCcw size={15} aria-hidden="true" />
                      从头播放
                    </Link>
                    <Link
                      to="/favorites"
                      className={buttonClassName({ variant: 'secondary', size: 'lg' })}
                      data-variant="secondary"
                      data-size="lg"
                    >
                      <Heart size={15} aria-hidden="true" />
                      收藏
                    </Link>
                  </>
                )}
              />
            </motion.div>
          </AnimatePresence>
        </div>
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
