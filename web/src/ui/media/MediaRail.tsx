import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import clsx from 'clsx'
import { ChevronLeft, ChevronRight } from 'lucide-react'
import { Button, Section } from '@/components/design-system'

export interface MediaRailProps {
  title: ReactNode
  ariaLabel: string
  itemCount: number
  children: ReactNode
  action?: ReactNode
  className?: string
  trackClassName?: string
}

export function MediaRail({
  title,
  ariaLabel,
  itemCount,
  children,
  action,
  className,
  trackClassName,
}: MediaRailProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)

  const updateScrollState = useCallback(() => {
    const element = scrollRef.current
    if (!element) return
    setCanScrollLeft(element.scrollLeft > 10)
    setCanScrollRight(element.scrollLeft < element.scrollWidth - element.clientWidth - 10)
  }, [])

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return

    const resizeObserver = typeof ResizeObserver !== 'undefined'
      ? new ResizeObserver(updateScrollState)
      : null

    element.addEventListener('scroll', updateScrollState, { passive: true })
    resizeObserver?.observe(element)
    const frame = window.requestAnimationFrame(updateScrollState)

    return () => {
      window.cancelAnimationFrame(frame)
      element.removeEventListener('scroll', updateScrollState)
      resizeObserver?.disconnect()
    }
  }, [itemCount, updateScrollState])

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return
    const amount = Math.max(320, element.clientWidth * 0.78)
    element.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <Section title={title} action={action} className={className}>
      <div className="nv-media-rail group/rail relative">
        {canScrollLeft && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('left')}
            className="nv-media-rail-arrow nv-media-rail-arrow--left"
            aria-label={`${ariaLabel} 向左滚动`}
          >
            <ChevronLeft size={17} aria-hidden="true" />
          </Button>
        )}

        <div
          ref={scrollRef}
          className={clsx('nv-media-rail-track scrollbar-hide', trackClassName)}
          aria-label={ariaLabel}
        >
          {children}
        </div>

        {canScrollRight && (
          <Button
            variant="secondary"
            size="sm"
            iconOnly
            onClick={() => scroll('right')}
            className="nv-media-rail-arrow nv-media-rail-arrow--right"
            aria-label={`${ariaLabel} 向右滚动`}
          >
            <ChevronRight size={17} aria-hidden="true" />
          </Button>
        )}
      </div>
    </Section>
  )
}

export default MediaRail
