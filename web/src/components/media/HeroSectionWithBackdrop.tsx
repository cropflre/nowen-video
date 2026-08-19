import { useLayoutEffect, useRef, useState, type ComponentProps } from 'react'
import { createPortal } from 'react-dom'
import { Captions } from 'lucide-react'
import { withToken } from '@/api/stream'
import { Button } from '@/components/design-system'
import SubtitleManager from '@/components/SubtitleManager'
import HeroSection from './HeroSection'
import './hero-section-backdrop.css'

type HeroSectionWithBackdropProps = ComponentProps<typeof HeroSection>

function getMediaBackdropUrl(mediaId: string, version?: number) {
  const query = version ? `?v=${version}` : ''
  return withToken(`/api/media/${mediaId}/backdrop${query}`)
}

export default function HeroSectionWithBackdrop(props: HeroSectionWithBackdropProps) {
  const { media, posterVersion } = props
  const shellRef = useRef<HTMLDivElement>(null)
  const [actionsHost, setActionsHost] = useState<HTMLElement | null>(null)
  const [showSubtitleManager, setShowSubtitleManager] = useState(false)
  const backdropKey = `${media.id}:${media.backdrop_path || ''}:${posterVersion || 0}`
  const [loadedKey, setLoadedKey] = useState<string | null>(null)
  const [failedKey, setFailedKey] = useState<string | null>(null)

  useLayoutEffect(() => {
    setActionsHost(shellRef.current?.querySelector<HTMLElement>('.nv-media-hero-actions') || null)
  }, [media.id])

  // Standalone media always gets one cheap backdrop probe. This lets older
  // database rows benefit from a newly-added `-backdrop.*` file without forcing
  // a rescan first. Episodes keep the existing Series artwork path.
  const shouldProbeStandaloneBackdrop = media.media_type !== 'episode'
  const shouldRequestBackdrop = shouldProbeStandaloneBackdrop && failedKey !== backdropKey
  const isBackdropReady = shouldRequestBackdrop && loadedKey === backdropKey

  return (
    <div
      ref={shellRef}
      className="nv-detail-hero-backdrop-shell"
      data-has-backdrop={isBackdropReady ? 'true' : 'false'}
    >
      {shouldRequestBackdrop && (
        <img
          key={backdropKey}
          src={getMediaBackdropUrl(media.id, posterVersion)}
          alt=""
          aria-hidden="true"
          className={`nv-detail-hero-local-backdrop${isBackdropReady ? ' is-loaded' : ''}`}
          onLoad={() => setLoadedKey(backdropKey)}
          onError={() => setFailedKey(backdropKey)}
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
