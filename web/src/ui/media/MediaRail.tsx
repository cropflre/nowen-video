import { useCallback, useEffect, useRef, useState, type CSSProperties, type ReactNode } from 'react'
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
   * Keep every visible card fully inside the rail viewport. Homepage rails
   * enable this automatically; other consumers may opt in explicitly.
   */
  fullItemsOnly?: boolean
  /** Minimum card width used when calculating the integer visible count. */
  minItemWidth?: number
}

interface FitLayout {
  visibleCount: number
  itemWidth: number
  gap: number
}

export function MediaRail({
  title,
  ariaLabel,
  itemCount,
  children,
  action,
  className,
  trackClassName,
  fullItemsOnly,
  minItemWidth = 96,
}: MediaRailProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const fitLayoutRef = useRef<FitLayout>({ visibleCount: 1, itemWidth: minItemWidth, gap: 0 })
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)
  const [fitItemWidth, setFitItemWidth] = useState<number | null>(null)
  const [isHomepageRail, setIsHomepageRail] = useState(false)
  const shouldFitFullItems = fullItemsOnly ?? isHomepageRail

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return
    // The homepage product contract explicitly forbids clipped/half cards. This
    // keeps existing HomePage call sites clean while leaving other rails opt-in.
    setIsHomepageRail(Boolean(element.closest('.nv-home-page')))
  }, [])

  const updateScrollState = useCallback(() => {
    const element = scrollRef.current
    if (!element) return
    setCanScrollLeft(element.scrollLeft > 10)
    setCanScrollRight(element.scrollLeft < element.scrollWidth - element.clientWidth - 10)
  }, [])

  const updateFitLayout = useCallback(() => {
    const element = scrollRef.current
    if (!element || !shouldFitFullItems) {
      setFitItemWidth(null)
      return
    }

    const styles = window.getComputedStyle(element)
    const gap = Number.parseFloat(styles.columnGap || styles.gap || '0') || 0
    const paddingLeft = Number.parseFloat(styles.paddingLeft || '0') || 0
    const paddingRight = Number.parseFloat(styles.paddingRight || '0') || 0
    const contentWidth = Math.max(0, element.clientWidth - paddingLeft - paddingRight)
    const cssMinItemWidth = Number.parseFloat(styles.getPropertyValue('--nv-media-rail-min-item-width'))
    const resolvedMinItemWidth = Number.isFinite(cssMinItemWidth) && cssMinItemWidth > 0
      ? cssMinItemWidth
      : minItemWidth

    if (contentWidth <= 0) return

    // Integer-card contract: reduce the visible count before ever clipping a card.
    const visibleCount = Math.max(1, Math.floor((contentWidth + gap) / (resolvedMinItemWidth + gap)))
    const fittedWidth = Math.max(
      resolvedMinItemWidth,
      (contentWidth - gap * Math.max(0, visibleCount - 1)) / visibleCount,
    )

    fitLayoutRef.current = { visibleCount, itemWidth: fittedWidth, gap }
    setFitItemWidth((current) => current !== null && Math.abs(current - fittedWidth) < 0.25 ? current : fittedWidth)
  }, [minItemWidth, shouldFitFullItems])

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return

    const sync = () => {
      updateFitLayout()
      updateScrollState()
    }

    const resizeObserver = typeof ResizeObserver !== 'undefined'
      ? new ResizeObserver(sync)
      : null

    element.addEventListener('scroll', updateScrollState, { passive: true })
    resizeObserver?.observe(element)
    window.addEventListener('resize', sync, { passive: true })
    const frame = window.requestAnimationFrame(sync)

    return () => {
      window.cancelAnimationFrame(frame)
      element.removeEventListener('scroll', updateScrollState)
      window.removeEventListener('resize', sync)
      resizeObserver?.disconnect()
    }
  }, [itemCount, updateFitLayout, updateScrollState])

  useEffect(() => {
    const frame = window.requestAnimationFrame(updateScrollState)
    return () => window.cancelAnimationFrame(frame)
  }, [fitItemWidth, updateScrollState])

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return

    let amount = Math.max(320, element.clientWidth * 0.78)
    if (shouldFitFullItems) {
      const { visibleCount, itemWidth, gap } = fitLayoutRef.current
      amount = Math.max(itemWidth + gap, visibleCount * (itemWidth + gap))
    }

    element.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  const trackStyle = shouldFitFullItems && fitItemWidth !== null
    ? ({ '--nv-media-rail-fit-item-width': `${fitItemWidth}px` } as CSSProperties)
    : undefined

  return (
    <Section title={title} action={action} className={className}>
      <div className="nv-media-rail group/rail relative" data-full-items={shouldFitFullItems ? 'true' : undefined}>
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
            shouldFitFullItems && 'nv-media-rail-track--full-items',
            trackClassName,
          )}
          style={trackStyle}
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
