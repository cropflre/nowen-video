import { useEffect, useLayoutEffect, useMemo, useRef, useState, type ComponentProps } from 'react'
import { createPortal } from 'react-dom'
import { Captions } from 'lucide-react'
import { mediaAnalysisApi } from '@/api/mediaAnalysis'
import { withToken } from '@/api/stream'
import { Button } from '@/components/design-system'
import SubtitleManager from '@/components/SubtitleManager'
import HeroSection from './HeroSection'
import './hero-section-backdrop.css'

type HeroSectionWithBackdropProps = ComponentProps<typeof HeroSection>

const BACKDROP_ROTATE_INTERVAL = 6500
const MAX_HERO_FRAMES = 6

function getMediaBackdropUrl(mediaId: string, version?: number) {
  const query = version ? `?v=${version}` : ''
  return withToken(`/api/media/${mediaId}/backdrop${query}`)
}

function uniqueUrls(urls: string[]) {
  return urls.filter((url, index) => url && urls.indexOf(url) === index)
}

export default function HeroSectionWithBackdrop(props: HeroSectionWithBackdropProps) {
  const { media, posterVersion } = props
  const shellRef = useRef<HTMLDivElement>(null)
  const [actionsHost, setActionsHost] = useState<HTMLElement | null>(null)
  const [showSubtitleManager, setShowSubtitleManager] = useState(false)
  const [highlightFrames, setHighlightFrames] = useState<string[]>([])
  const [failedFrames, setFailedFrames] = useState<string[]>([])
  const [currentFrame, setCurrentFrame] = useState(0)

  useLayoutEffect(() => {
    setActionsHost(shellRef.current?.querySelector<HTMLElement>('.nv-media-hero-actions') || null)
  }, [media.id])

  useEffect(() => {
    let cancelled = false
    setHighlightFrames([])
    setFailedFrames([])
    setCurrentFrame(0)

    mediaAnalysisApi.getHighlights(media.id)
      .then((response) => {
        if (cancelled) return
        const frames = (response.data.data?.highlights || [])
          .filter((highlight) => Boolean(highlight.thumbnail_url))
          .sort((a, b) => a.start_time - b.start_time)
          .slice(0, MAX_HERO_FRAMES)
          .map((highlight) => withToken(highlight.thumbnail_url!))
        setHighlightFrames(uniqueUrls(frames))
      })
      .catch(() => {
        if (!cancelled) setHighlightFrames([])
      })

    return () => {
      cancelled = true
    }
  }, [media.id])

  const backdropUrl = useMemo(
    () => getMediaBackdropUrl(media.id, posterVersion),
    [media.id, posterVersion],
  )

  const carouselFrames = useMemo(() => {
    // Highlight thumbnails are true single video frames. When they exist, use
    // only those frames for the detail slideshow so a legacy local backdrop
    // that is itself a storyboard/contact sheet can never re-enter the cycle.
    const source = highlightFrames.length > 0 ? highlightFrames : [backdropUrl]
    return uniqueUrls(source).filter((url) => !failedFrames.includes(url))
  }, [backdropUrl, failedFrames, highlightFrames])

  useEffect(() => {
    if (currentFrame < carouselFrames.length) return
    setCurrentFrame(0)
  }, [carouselFrames.length, currentFrame])

  useEffect(() => {
    if (carouselFrames.length <= 1) return
    if (typeof window !== 'undefined' && window.matchMedia?.('(prefers-reduced-motion: reduce)').matches) return

    const timer = window.setInterval(() => {
      setCurrentFrame((index) => (index + 1) % carouselFrames.length)
    }, BACKDROP_ROTATE_INTERVAL)

    return () => window.clearInterval(timer)
  }, [carouselFrames.length])

  const currentBackdrop = carouselFrames[currentFrame] || ''
  const hasCarouselBackdrop = Boolean(currentBackdrop)

  const handleBackdropError = () => {
    if (!currentBackdrop) return
    setFailedFrames((current) => current.includes(currentBackdrop) ? current : [...current, currentBackdrop])
  }

  return (
    <div
      ref={shellRef}
      className="nv-detail-hero-backdrop-shell"
      data-has-backdrop={hasCarouselBackdrop ? 'true' : 'false'}
      data-carousel-count={carouselFrames.length}
    >
      {currentBackdrop && (
        <img
          key={currentBackdrop}
          src={currentBackdrop}
          alt=""
          aria-hidden="true"
          className="nv-detail-hero-local-backdrop is-loaded"
          onError={handleBackdropError}
        />
      )}

      <HeroSection {...props} />

      {actionsHost && createPortal(
        <span className="nv-detail-subtitle-action-slot">
          <Button
            type="button"
            variant="secondary"
            size="lg"
            className="nv-detail-subtitle-action"
            onClick={() => setShowSubtitleManager(true)}
            title="字幕"
            aria-label="打开字幕菜单"
          >
            <Captions size={18} aria-hidden="true" />
            <span>字幕</span>
          </Button>
        </span>,
        actionsHost,
      )}

      {showSubtitleManager && (
        <SubtitleManager
          mediaId={media.id}
          mediaTitle={media.title}
          onClose={() => setShowSubtitleManager(false)}
        />
      )}
    </div>
  )
}
