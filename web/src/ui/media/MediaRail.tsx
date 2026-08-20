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
  /**
   * Keep every resting card fully inside the rail viewport. Geometry is handled
   * by CSS container queries so resizing never needs synchronous JS layout reads.
   */
  fullItemsOnly?: boolean
}

export function MediaRail({
  title,
  ariaLabel,
  itemCount,
  children,
  action,
  className,
  trackClassName,
  fullItemsOnly = false,
}: MediaRailProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const frameRef = useRef<number | null>(null)
  const scrollStateRef = useRef({ left: false, right: itemCount > 1 })
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(itemCount > 1)

  const updateScrollState = useCallback(() => {
    const element = scrollRef.current
    if (!element) return

    const nextLeft = element.scrollLeft > 8
    const nextRight = element.scrollLeft < element.scrollWidth - element.clientWidth - 8
    const previous = scrollStateRef.current

    if (nextLeft !== previous.left) setCanScrollLeft(nextLeft)
    if (nextRight !== previous.right) setCanScrollRight(nextRight)
    scrollStateRef.current = { left: nextLeft, right: nextRight }
  }, [])

  const scheduleScrollState = useCallback(() => {
    if (frameRef.current !== null) return
    frameRef.current = window.requestAnimationFrame(() => {
      frameRef.current = null
      updateScrollState()
    })
  }, [updateScrollState])

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return

    element.addEventListener('scroll', scheduleScrollState, { passive: true })
    window.addEventListener('resize', scheduleScrollState, { passive: true })
    scheduleScrollState()

    return () => {
      element.removeEventListener('scroll', scheduleScrollState)
      window.removeEventListener('resize', scheduleScrollState)
      if (frameRef.current !== null) {
        window.cancelAnimationFrame(frameRef.current)
        frameRef.current = null
      }
    }
  }, [itemCount, scheduleScrollState])

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return

    // CSS guarantees an integer number of cards per viewport. Paging by the
    // viewport width therefore always lands on a full-card boundary; scroll snap
    // resolves the final sub-pixel difference without another measurement pass.
    const amount = Math.max(240, element.clientWidth)
    element.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <Section title={title} action={action} className={className}>
      <div
        className="nv-media-rail group/rail relative"
        data-full-items={fullItemsOnly ? 'true' : undefined}
      >
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
          className={clsx(
            'nv-media-rail-track scrollbar-hide',
            fullItemsOnly && 'nv-media-rail-track--full-items',
            trackClassName,
          )}
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
