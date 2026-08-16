import { useState, type ComponentProps } from 'react'
import { withToken } from '@/api/stream'
import HeroSection from './HeroSection'
import './hero-section-backdrop.css'

type HeroSectionWithBackdropProps = ComponentProps<typeof HeroSection>

function getMediaBackdropUrl(mediaId: string, version?: number) {
  const query = version ? `?v=${version}` : ''
  return withToken(`/api/media/${mediaId}/backdrop${query}`)
}

export default function HeroSectionWithBackdrop(props: HeroSectionWithBackdropProps) {
  const { media, posterVersion } = props
  const backdropKey = `${media.id}:${media.backdrop_path || ''}:${posterVersion || 0}`
  const [loadedKey, setLoadedKey] = useState<string | null>(null)
  const [failedKey, setFailedKey] = useState<string | null>(null)

  // Standalone media always gets one cheap backdrop probe. This lets older
  // database rows benefit from a newly-added `-backdrop.*` file without forcing
  // a rescan first. Episodes keep the existing Series artwork path.
  const shouldProbeStandaloneBackdrop = media.media_type !== 'episode'
  const shouldRequestBackdrop = shouldProbeStandaloneBackdrop && failedKey !== backdropKey
  const isBackdropReady = shouldRequestBackdrop && loadedKey === backdropKey

  return (
    <div
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
    </div>
  )
}
