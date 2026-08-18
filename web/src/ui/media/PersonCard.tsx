import { useEffect, useState, type ReactNode } from 'react'
import clsx from 'clsx'
import { User } from 'lucide-react'
import { MediaArtwork } from './MediaArtwork'

export interface PersonCardProps {
  name: string
  subtitle?: string
  imageSrc?: string | null
  badge?: ReactNode
  onClick?: () => void
  className?: string
  ariaLabel?: string
}

export function PersonCard({
  name,
  subtitle,
  imageSrc,
  badge,
  onClick,
  className,
  ariaLabel,
}: PersonCardProps) {
  const [imageVersion, setImageVersion] = useState(imageSrc)

  useEffect(() => {
    setImageVersion(imageSrc)
  }, [imageSrc])

  const content = (
    <>
      <MediaArtwork
        src={imageVersion}
        alt={name}
        ratio="square"
        fallback={<User size={28} strokeWidth={1.4} aria-hidden="true" />}
        className="nv-person-card-artwork transition-[transform,box-shadow,border-color] duration-200 group-hover:-translate-y-[3px] group-hover:border-[var(--nv-border-hover)] group-hover:shadow-[var(--nv-shadow-card-hover)]"
        imageClassName="transition-[filter] duration-200 group-hover:brightness-[.88]"
      >
        {badge && <div className="absolute left-1.5 top-1.5 z-10 max-w-[calc(100%-12px)]">{badge}</div>}
      </MediaArtwork>
      <p className="mt-1.5 truncate text-xs font-medium text-[var(--nv-text-primary)]">{name}</p>
      {subtitle && (
        <p className="mt-0.5 truncate text-[10px] text-[var(--nv-text-tertiary)]" title={subtitle}>
          {subtitle}
        </p>
      )}
    </>
  )

  if (onClick) {
    return (
      <button
        type="button"
        onClick={onClick}
        className={clsx('nv-person-card group w-[84px] flex-shrink-0 text-left sm:w-[96px]', className)}
        role="listitem"
        aria-label={ariaLabel || name}
      >
        {content}
      </button>
    )
  }

  return (
    <article className={clsx('nv-person-card group w-[84px] flex-shrink-0 sm:w-[96px]', className)} role="listitem">
      {content}
    </article>
  )
}

export default PersonCard
