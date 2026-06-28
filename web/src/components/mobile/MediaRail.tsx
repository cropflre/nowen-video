import { ReactNode, useRef } from 'react'
import { motion } from 'framer-motion'
import { ChevronRight } from 'lucide-react'
import { mobileTokens } from '@/styles/mobile-tokens'

interface MediaRailProps {
  title: string
  subtitle?: string
  onSeeAll?: () => void
  children: ReactNode
  /** 是否显示渐变遮罩 */
  showFade?: boolean
}

/**
 * 移动端媒体横向滚动轨道
 * Hills Pro 风格：标题 + 横向滚动 + 渐变遮罩
 */
export default function MediaRail({
  title,
  subtitle,
  onSeeAll,
  children,
  showFade = true,
}: MediaRailProps) {
  const scrollRef = useRef<HTMLDivElement>(null)

  return (
    <div className="mb-8">
      {/* 标题栏 */}
      <div
        className="flex items-center justify-between px-8 mb-4"
      >
        <div>
          <h2
            className="font-semibold"
            style={{
              fontSize: mobileTokens.fontSize.xl,
              color: mobileTokens.text,
            }}
          >
            {title}
          </h2>
          {subtitle && (
            <p
              style={{
                fontSize: mobileTokens.fontSize.sm,
                color: mobileTokens.textMuted,
                marginTop: '2px',
              }}
            >
              {subtitle}
            </p>
          )}
        </div>

        {onSeeAll && (
          <motion.button
            whileTap={{ scale: 0.95 }}
            onClick={onSeeAll}
            className="flex items-center gap-1"
            style={{
              color: mobileTokens.primary,
              fontSize: mobileTokens.fontSize.sm,
              fontWeight: 500,
            }}
          >
            <span>查看全部</span>
            <ChevronRight size={16} />
          </motion.button>
        )}
      </div>

      {/* 横向滚动容器 */}
      <div className="relative">
        <div
          ref={scrollRef}
          className="flex gap-4 overflow-x-auto px-8 pb-2"
          style={{
            scrollbarWidth: 'none',
            msOverflowStyle: 'none',
            WebkitOverflowScrolling: 'touch',
          }}
        >
          {children}
        </div>

        {/* 右侧渐变遮罩 */}
        {showFade && (
          <div
            className="pointer-events-none absolute right-0 top-0 bottom-0 w-8"
            style={{
              background: `linear-gradient(to right, transparent, ${mobileTokens.bg})`,
            }}
          />
        )}
      </div>
    </div>
  )
}
