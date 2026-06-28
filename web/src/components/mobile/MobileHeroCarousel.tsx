import { useState, useEffect, useCallback } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { mobileTokens } from '@/styles/mobile-tokens'

interface HeroItem {
  id: string
  title: string
  subtitle?: string
  imageUrl?: string
  rating?: number
  year?: number
  resolution?: string
  ratingTag?: string
}

interface MobileHeroCarouselProps {
  items: HeroItem[]
  autoPlay?: boolean
  interval?: number
  onItemClick?: (item: HeroItem) => void
  className?: string
}

/**
 * 移动端 Hero 轮播
 * Hills Pro 风格：大背景图 + 渐变叠加 + 文字信息 + 分页点
 */
export default function MobileHeroCarousel({
  items,
  autoPlay = true,
  interval = 6500,
  onItemClick,
  className = '',
}: MobileHeroCarouselProps) {
  const [currentIndex, setCurrentIndex] = useState(0)
  const [direction, setDirection] = useState(0)

  // 自动轮播
  useEffect(() => {
    if (!autoPlay || items.length <= 1) return

    const timer = setInterval(() => {
      setDirection(1)
      setCurrentIndex((prev) => (prev + 1) % items.length)
    }, interval)

    return () => clearInterval(timer)
  }, [autoPlay, interval, items.length])

  // 手动切换
  const goTo = useCallback((index: number) => {
    setDirection(index > currentIndex ? 1 : -1)
    setCurrentIndex(index)
  }, [currentIndex])

  // 滑动手势
  const handleDragEnd = useCallback((_: any, info: { offset: { x: number }; velocity: { x: number } }) => {
    const swipeThreshold = 50
    const velocityThreshold = 300

    if (info.offset.x < -swipeThreshold || info.velocity.x < -velocityThreshold) {
      // 向左滑动 -> 下一张
      setDirection(1)
      setCurrentIndex((prev) => (prev + 1) % items.length)
    } else if (info.offset.x > swipeThreshold || info.velocity.x > velocityThreshold) {
      // 向右滑动 -> 上一张
      setDirection(-1)
      setCurrentIndex((prev) => (prev - 1 + items.length) % items.length)
    }
  }, [])

  if (items.length === 0) return null

  const currentItem = items[currentIndex]

  // 动画变体
  const variants = {
    enter: (direction: number) => ({
      x: direction > 0 ? '100%' : '-100%',
      opacity: 0,
    }),
    center: {
      x: 0,
      opacity: 1,
    },
    exit: (direction: number) => ({
      x: direction > 0 ? '-100%' : '100%',
      opacity: 0,
    }),
  }

  return (
    <div
      className={`relative overflow-hidden ${className}`}
      style={{
        height: '400px',
        borderRadius: mobileTokens.radius['2xl'],
        margin: `0 ${mobileTokens.spacing.xl}`,
      }}
    >
      {/* 背景图 */}
      <AnimatePresence initial={false} custom={direction} mode="popLayout">
        <motion.div
          key={currentIndex}
          custom={direction}
          variants={variants}
          initial="enter"
          animate="center"
          exit="exit"
          transition={{ duration: 0.4, ease: 'easeInOut' }}
          drag="x"
          dragConstraints={{ left: 0, right: 0 }}
          dragElastic={1}
          onDragEnd={handleDragEnd}
          className="absolute inset-0"
          onClick={() => onItemClick?.(currentItem)}
        >
          {/* 图片 */}
          {currentItem.imageUrl ? (
            <div
              className="h-full w-full bg-cover bg-center"
              style={{ backgroundImage: `url(${currentItem.imageUrl})` }}
            />
          ) : (
            <div
              className="h-full w-full"
              style={{
                background: `linear-gradient(135deg, ${mobileTokens.primarySoft}, ${mobileTokens.bgWarm})`,
              }}
            />
          )}

          {/* 渐变叠加 */}
          <div
            className="absolute inset-0"
            style={{
              background: `
                linear-gradient(180deg, rgba(0,0,0,0.1) 0%, rgba(0,0,0,0.3) 50%, rgba(0,0,0,0.7) 100%)
              `,
            }}
          />

          {/* 文字信息 */}
          <div
            className="absolute bottom-0 left-0 right-0 p-6"
            style={{ paddingBottom: '40px' }}
          >
            {/* 信息行 */}
            <div className="flex items-center gap-3 mb-2">
              {currentItem.rating && (
                <span
                  className="px-2 py-0.5 text-xs font-medium"
                  style={{
                    background: 'rgba(255, 255, 255, 0.2)',
                    color: '#fff',
                    borderRadius: mobileTokens.radius.xs,
                    backdropFilter: 'blur(4px)',
                  }}
                >
                  ⭐ {currentItem.rating.toFixed(1)}
                </span>
              )}
              {currentItem.year && (
                <span
                  className="text-sm"
                  style={{ color: 'rgba(255, 255, 255, 0.8)' }}
                >
                  {currentItem.year}
                </span>
              )}
              {currentItem.resolution && (
                <span
                  className="px-2 py-0.5 text-xs font-medium"
                  style={{
                    background: 'rgba(255, 255, 255, 0.2)',
                    color: '#fff',
                    borderRadius: mobileTokens.radius.xs,
                  }}
                >
                  {currentItem.resolution}
                </span>
              )}
              {currentItem.ratingTag && (
                <span
                  className="px-2 py-0.5 text-xs font-medium"
                  style={{
                    background: 'rgba(255, 255, 255, 0.2)',
                    color: '#fff',
                    borderRadius: mobileTokens.radius.xs,
                  }}
                >
                  {currentItem.ratingTag}
                </span>
              )}
            </div>

            {/* 标题 */}
            <h2
              className="font-bold"
              style={{
                fontSize: '28px',
                color: '#fff',
                lineHeight: 1.2,
                textShadow: '0 2px 4px rgba(0, 0, 0, 0.3)',
              }}
            >
              {currentItem.title}
            </h2>

            {/* 副标题 */}
            {currentItem.subtitle && (
              <p
                className="mt-2 line-clamp-2"
                style={{
                  fontSize: mobileTokens.fontSize.md,
                  color: 'rgba(255, 255, 255, 0.8)',
                  lineHeight: 1.5,
                }}
              >
                {currentItem.subtitle}
              </p>
            )}
          </div>
        </motion.div>
      </AnimatePresence>

      {/* 分页点 */}
      {items.length > 1 && (
        <div
          className="absolute bottom-4 left-1/2 -translate-x-1/2 flex items-center gap-2"
          style={{ zIndex: 10 }}
        >
          {items.map((_, index) => (
            <motion.button
              key={index}
              onClick={() => goTo(index)}
              animate={{
                scale: index === currentIndex ? 1.2 : 1,
                opacity: index === currentIndex ? 1 : 0.5,
              }}
              transition={{ duration: 0.2 }}
              style={{
                width: '8px',
                height: '8px',
                borderRadius: '50%',
                background: '#fff',
              }}
            />
          ))}
        </div>
      )}
    </div>
  )
}
