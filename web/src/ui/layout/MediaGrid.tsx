import type { HTMLAttributes } from 'react'
import clsx from 'clsx'

export type MediaGridVariant = 'standard' | 'poster'

export interface MediaGridProps extends HTMLAttributes<HTMLDivElement> {
  variant?: MediaGridVariant
}

export function MediaGrid({ variant = 'standard', className, children, ...props }: MediaGridProps) {
  return (
    <div
      {...props}
      className={clsx(
        'nv-ui-media-grid',
        variant === 'standard'
          ? 'nv-media-grid'
          : 'nv-browse-poster-grid grid grid-cols-3 gap-x-2.5 gap-y-4 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6 xl:grid-cols-8',
        className,
      )}
      data-variant={variant}
    >
      {children}
    </div>
  )
}

export default MediaGrid
