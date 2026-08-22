import type { ReactNode } from 'react'
import clsx from 'clsx'

type DetailSectionTitleElement = 'h2' | 'h3' | 'span'

interface DetailSectionTitleProps {
  as?: DetailSectionTitleElement
  id?: string
  icon: ReactNode
  children: ReactNode
  className?: string
}

export default function DetailSectionTitle({
  as: Element = 'h2',
  id,
  icon,
  children,
  className,
}: DetailSectionTitleProps) {
  return (
    <Element id={id} className={clsx('nv-detail-section-title', className)}>
      <span className="nv-detail-section-title-icon" aria-hidden="true">{icon}</span>
      <span>{children}</span>
    </Element>
  )
}
