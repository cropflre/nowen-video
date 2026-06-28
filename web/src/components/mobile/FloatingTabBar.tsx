import { ReactNode, useState, useEffect, useRef } from 'react'
import { motion } from 'framer-motion'
import { mobileTokens } from '@/styles/mobile-tokens'

export interface FloatingTabBarItem {
  key: string
  label: string
  icon: ReactNode
  activeIcon?: ReactNode
}

interface FloatingTabBarProps {
  items: FloatingTabBarItem[]
  activeKey: string
  onChange: (key: string) => void
}

/**
 * 悬浮毛玻璃底部导航栏
 * Hills Pro 风格：半透明背景 + 大圆角 + 胶囊高亮
 */
export default function FloatingTabBar({ items, activeKey, onChange }: FloatingTabBarProps) {
  const [indicatorStyle, setIndicatorStyle] = useState({ left: 0, width: 0 })
  const tabRefs = useRef<Map<string, HTMLButtonElement>>(new Map())

  // 计算指示器位置
  useEffect(() => {
    const activeTab = tabRefs.current.get(activeKey)
    if (activeTab) {
      const parent = activeTab.parentElement
      if (parent) {
        const parentRect = parent.getBoundingClientRect()
        const tabRect = activeTab.getBoundingClientRect()
        setIndicatorStyle({
          left: tabRect.left - parentRect.left,
          width: tabRect.width,
        })
      }
    }
  }, [activeKey])

  return (
    <div
      className="fixed left-1/2 -translate-x-1/2 z-50"
      style={{
        bottom: `calc(${mobileTokens.nav.bottom} + env(safe-area-inset-bottom, 0px))`,
        width: mobileTokens.nav.width,
        maxWidth: mobileTokens.nav.maxWidth,
      }}
    >
      <div
        className="relative flex items-center justify-around"
        style={{
          height: mobileTokens.nav.height,
          background: mobileTokens.nav.background,
          backdropFilter: 'blur(22px)',
          WebkitBackdropFilter: 'blur(22px)',
          border: mobileTokens.nav.border,
          borderRadius: mobileTokens.nav.borderRadius,
          boxShadow: mobileTokens.nav.boxShadow,
        }}
      >
        {/* 滑动指示器 */}
        <motion.div
          className="absolute top-1/2 -translate-y-1/2"
          style={{
            height: 'calc(100% - 16px)',
            borderRadius: mobileTokens.radius.full,
            background: mobileTokens.nav.activeBackground,
          }}
          initial={false}
          animate={{
            left: indicatorStyle.left + 4,
            width: indicatorStyle.width - 8,
          }}
          transition={{
            type: 'spring',
            stiffness: 400,
            damping: 30,
          }}
        />

        {/* Tab 按钮 */}
        {items.map((item) => {
          const isActive = item.key === activeKey
          return (
            <button
              key={item.key}
              ref={(el) => {
                if (el) tabRefs.current.set(item.key, el)
              }}
              onClick={() => onChange(item.key)}
              className="relative z-10 flex flex-1 flex-col items-center justify-center gap-1"
              style={{
                height: '100%',
                color: isActive ? mobileTokens.nav.activeColor : mobileTokens.textMuted,
                transition: 'color 0.2s ease',
              }}
            >
              <motion.div
                animate={{
                  scale: isActive ? 1.05 : 1,
                  y: isActive ? -1 : 0,
                }}
                transition={{ type: 'spring', stiffness: 400, damping: 25 }}
              >
                {isActive && item.activeIcon ? item.activeIcon : item.icon}
              </motion.div>
              <span
                className="text-xs font-medium"
                style={{
                  fontSize: mobileTokens.fontSize.xs,
                  opacity: isActive ? 1 : 0.7,
                }}
              >
                {item.label}
              </span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
