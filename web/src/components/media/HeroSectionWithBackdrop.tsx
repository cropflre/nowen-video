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

  const hasDeclaredBackdrop = Boolean(media.backdrop_path)
  const shouldRequestBackdrop = hasDeclaredBackdrop && failedKey !== backdropKey
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
