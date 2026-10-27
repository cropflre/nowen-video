import { useEffect, useMemo, useRef, useState, type ComponentProps } from 'react'
import { mediaAnalysisApi } from '@/api/mediaAnalysis'
import { withToken } from '@/api/stream'
import HeroSection from './HeroSection'
import './hero-section-backdrop.css'

type HeroSectionWithBackdropProps = ComponentProps<typeof HeroSection>

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
  const [highlightFrames, setHighlightFrames] = useState<string[]>([])
  const [failedFrames, setFailedFrames] = useState<string[]>([])
  const [currentFrame, setCurrentFrame] = useState(0)
  const [readyBackdrop, setReadyBackdrop] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setHighlightFrames([])
    setFailedFrames([])
    setCurrentFrame(0)
    setReadyBackdrop(null)

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

  const currentBackdrop = carouselFrames[currentFrame] || ''
  const backdropReady = Boolean(currentBackdrop && readyBackdrop === currentBackdrop)

  const handleBackdropError = () => {
    if (!currentBackdrop) return
    setFailedFrames((current) => current.includes(currentBackdrop) ? current : [...current, currentBackdrop])
    setReadyBackdrop((current) => current === currentBackdrop ? null : current)
  }

  return (
    <div
      ref={shellRef}
      className="nv-detail-hero-backdrop-shell"
      data-has-backdrop={backdropReady ? 'true' : 'false'}
      data-carousel-count={carouselFrames.length}
    >
      {currentBackdrop && (
        <img
          src={currentBackdrop}
          alt=""
          aria-hidden="true"
          className={`nv-detail-hero-local-backdrop${backdropReady ? ' is-ready' : ''}`}
          onLoad={() => setReadyBackdrop(currentBackdrop)}
          onError={handleBackdropError}
        />
      )}

      <HeroSection {...props} />

    </div>
  )
}
