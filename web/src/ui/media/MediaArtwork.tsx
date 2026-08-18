import { useState, type HTMLAttributes, type ReactNode } from 'react'
import clsx from 'clsx'
import { Film } from 'lucide-react'

export type MediaArtworkRatio = 'poster' | 'landscape' | 'hero' | 'square'

export interface MediaArtworkProps extends HTMLAttributes<HTMLDivElement> {
  src?: string | null
  alt?: string
  ratio?: MediaArtworkRatio
  loading?: 'eager' | 'lazy'
  fallback?: ReactNode
  imageClassName?: string
  overlay?: ReactNode
}

export function MediaArtwork({
  src,
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
  const [failed, setFailed] = useState(false)
  const showImage = Boolean(src) && !failed

  return (
    <div
      {...props}
      className={clsx('nv-media-artwork', className)}
      data-ratio={ratio}
      data-image-state={showImage ? 'ready' : 'fallback'}
    >
      {showImage ? (
        <img
          src={src!}
          alt={alt}
          loading={loading}
          className={clsx('nv-media-artwork-image', imageClassName)}
          onLoad={() => setFailed(false)}
          onError={() => setFailed(true)}
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
