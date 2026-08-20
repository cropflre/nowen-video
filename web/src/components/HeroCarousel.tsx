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

// A stale media record can still advertise a backdrop whose file has already
// disappeared. Remember failed endpoints for this page lifetime so the carousel
// does not issue the same 404 every time that slide becomes active again.
const failedHeroArtworkUrls = new Set<string>()

function artworkCacheKey(url: string) {
  try {
    return new URL(url, window.location.origin).pathname
  } catch {
    return url.split('?')[0]
  }
}

function hasHeroArtworkFailed(url?: string | null) {
  return Boolean(url && failedHeroArtworkUrls.has(artworkCacheKey(url)))
}

function markHeroArtworkFailed(url?: string | null) {
  if (url) failedHeroArtworkUrls.add(artworkCacheKey(url))
}

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
  primary: string | null
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
  if (media.series_id && media.series?.poster_path) return streamApi.getSeriesPosterUrl(media.series_id)
  return null
}

function getHeroArtwork(media: Media): HeroArtwork {
  const poster = getHeroPoster(media)
  let backdrop: string | null = null

  // Only hit backdrop endpoints when metadata says a backdrop exists. The old
  // unconditional fallback path generated guaranteed 404s for poster-only media.
  if (media.series_id && (media.series?.backdrop_path || (isSeriesProxy(media) && media.backdrop_path))) {
    backdrop = streamApi.getSeriesBackdropUrl(media.series_id)
  } else if (media.backdrop_path) {
    backdrop = streamApi.getBackdropUrl(media.id)
  }

  if (backdrop && !hasHeroArtworkFailed(backdrop)) {
    return {
      primary: backdrop,
      fallback: poster && !hasHeroArtworkFailed(poster) ? poster : undefined,
      isBackdrop: true,
    }
  }

  return {
    primary: poster && !hasHeroArtworkFailed(poster) ? poster : null,
    isBackdrop: false,
  }
}

function handleArtworkError(event: SyntheticEvent<HTMLImageElement>, fallback?: string) {
  const image = event.currentTarget
  markHeroArtworkFailed(image.currentSrc || image.src)

  if (fallback && !hasHeroArtworkFailed(fallback) && image.dataset.fallbackApplied !== 'true') {
    image.dataset.fallbackApplied = 'true'
    image.src = fallback
    image.dataset.artworkKind = 'poster'
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
  const [isHovering, setIsHovering] = useState(false)
  const [isInViewport, setIsInViewport] = useState(true)
  const [isPageVisible, setIsPageVisible] = useState(() => document.visibilityState !== 'hidden')

  useEffect(() => {
    if (current >= items.length && items.length > 0) setCurrent(0)
  }, [current, items.length])

  const goPrev = useCallback(() => {
    if (!items.length) return
    setCurrent((value) => (value - 1 + items.length) % items.length)
  }, [items.length])

  const goNext = useCallback(() => {
    if (!items.length) return
    setCurrent((value) => (value + 1) % items.length)
  }, [items.length])

  const goTo = useCallback((index: number) => {
    setCurrent(index)
  }, [])

  useEffect(() => {
    const container = containerRef.current
    if (!container || typeof IntersectionObserver === 'undefined') return

    const observer = new IntersectionObserver(
      ([entry]) => setIsInViewport(entry.isIntersecting),
      { rootMargin: '120px 0px', threshold: 0.01 },
    )
    observer.observe(container)
    return () => observer.disconnect()
  }, [])

  useEffect(() => {
    const handleVisibilityChange = () => setIsPageVisible(document.visibilityState !== 'hidden')
    document.addEventListener('visibilitychange', handleVisibilityChange)
    return () => document.removeEventListener('visibilitychange', handleVisibilityChange)
  }, [])

  useEffect(() => {
    if (timerRef.current) clearInterval(timerRef.current)
    if (items.length <= 1 || isHovering || !isInViewport || !isPageVisible) return

    timerRef.current = setInterval(goNext, AUTO_PLAY_INTERVAL)
    return () => {
      if (timerRef.current) clearInterval(timerRef.current)
    }
  }, [goNext, isHovering, isInViewport, isPageVisible, items.length])

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (!isInViewport || !isPageVisible) return

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
  }, [goNext, goPrev, isInViewport, isPageVisible])

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
      <AnimatePresence initial={false} mode="sync">
        <motion.div
          key={`hero-${item.media.id}`}
          className="absolute inset-0"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: prefersReducedMotion ? 0.1 : 0.28, ease: 'easeOut' }}
          drag={items.length > 1 ? 'x' : false}
          dragConstraints={{ left: 0, right: 0 }}
          dragElastic={0.08}
          onDragEnd={handleDragEnd}
        >
          {artwork.primary && (
            <img
              src={artwork.primary}
              alt=""
              data-artwork-kind={artwork.isBackdrop ? 'backdrop' : 'poster'}
              className={`h-full w-full select-none object-cover object-center${artwork.isBackdrop ? '' : ' scale-110 blur-2xl'}`}
              loading="eager"
              decoding="async"
              draggable={false}
              onError={(event) => handleArtworkError(event, artwork.fallback)}
            />
          )}
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
              initial={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={prefersReducedMotion ? { opacity: 0 } : { opacity: 0, y: -4 }}
              transition={{ duration: prefersReducedMotion ? 0.1 : 0.2 }}
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
