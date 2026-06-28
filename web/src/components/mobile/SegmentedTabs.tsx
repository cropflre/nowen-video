import { useRef, useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { mobileTokens } from '@/styles/mobile-tokens'

interface SegmentedTab {
  key: string
  label: string
}

interface SegmentedTabsProps {
  tabs: SegmentedTab[]
  activeKey: string
  onChange: (key: string) => void
  className?: string
}

/**
 * 分段式 Tab 切换器
 * Hills Pro 风格：文字 Tab + 底部短横线指示器 + 滑动动画
 */
export default function SegmentedTabs({
  tabs,
  activeKey,
  onChange,
  className = '',
}: SegmentedTabsProps) {
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
    <div className={`relative ${className}`}>
      <div className="flex">
        {tabs.map((tab) => {
          const isActive = tab.key === activeKey
          return (
            <button
              key={tab.key}
              ref={(el) => {
                if (el) tabRefs.current.set(tab.key, el)
              }}
              onClick={() => onChange(tab.key)}
              className="relative flex-1 py-3 text-center"
              style={{
                fontSize: mobileTokens.fontSize.md,
                fontWeight: isActive ? 600 : 400,
                color: isActive ? mobileTokens.primary : mobileTokens.textMuted,
                transition: 'color 0.2s ease, font-weight 0.2s ease',
              }}
            >
              {tab.label}
            </button>
          )
        })}
      </div>

      {/* 底部指示器 */}
      <div
        className="absolute bottom-0 left-0 right-0"
        style={{ height: '2px', background: mobileTokens.bgAlt }}
      >
        <motion.div
          className="absolute bottom-0 h-full"
          style={{
            background: mobileTokens.primary,
            borderRadius: '1px',
          }}
          initial={false}
          animate={{
            left: indicatorStyle.left,
            width: indicatorStyle.width,
          }}
          transition={{
            type: 'spring',
            stiffness: 400,
            damping: 30,
          }}
        />
      </div>
    </div>
  )
}
