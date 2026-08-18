import { useEffect, useState, type HTMLAttributes, type ReactNode } from 'react'
import clsx from 'clsx'
import { Film } from 'lucide-react'

export type MediaArtworkRatio = 'poster' | 'landscape' | 'hero' | 'square'

export interface MediaArtworkProps extends HTMLAttributes<HTMLDivElement> {
  src?: string | null
  fallbackSrc?: string | null
  alt?: string
  ratio?: MediaArtworkRatio
  loading?: 'eager' | 'lazy'
  fallback?: ReactNode
  imageClassName?: string
  overlay?: ReactNode
}

export function MediaArtwork({
  src,
  fallbackSrc,
  alt = '',
  ratio = 'poster',
  loading = 'lazy',
  fallback,
  imageClassName,
  overlay,
  className,
  children,
  ...props
}: MediaArtworkProps) {
  const [usingFallbackSrc, setUsingFallbackSrc] = useState(false)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    setUsingFallbackSrc(false)
    setFailed(false)
  }, [fallbackSrc, src])

  const activeSrc = usingFallbackSrc ? fallbackSrc : src
  const showImage = Boolean(activeSrc) && !failed

  const handleImageError = () => {
    if (!usingFallbackSrc && fallbackSrc && fallbackSrc !== src) {
      setUsingFallbackSrc(true)
      setFailed(false)
      return
    }
    setFailed(true)
  }

  return (
    <div
      {...props}
      className={clsx('nv-media-artwork', className)}
      data-ratio={ratio}
      data-image-state={showImage ? (usingFallbackSrc ? 'fallback-image' : 'ready') : 'fallback'}
    >
      {showImage ? (
        <img
          src={activeSrc!}
          alt={alt}
          loading={loading}
          className={clsx('nv-media-artwork-image', imageClassName)}
          onLoad={() => setFailed(false)}
          onError={handleImageError}
        />
      ) : (
        <div className="nv-media-artwork-fallback" aria-hidden={alt ? undefined : true}>
          {fallback ?? <Film size={24} aria-hidden="true" />}
        </div>
      )}
      {overlay && <div className="nv-media-artwork-overlay">{overlay}</div>}
      {children}
    </div>
  )
}

export default MediaArtwork
