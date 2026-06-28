import { ReactNode } from 'react'
import { motion } from 'framer-motion'
import { mobileTokens } from '@/styles/mobile-tokens'

interface MobileSettingGroupProps {
  title?: string
  children: ReactNode
  className?: string
}

/**
 * 移动端设置分组
 * Hills Pro 风格：蓝紫色分组标题 + 干净列表
 */
export default function MobileSettingGroup({
  title,
  children,
  className = '',
}: MobileSettingGroupProps) {
  return (
    <div className={className}>
      {title && (
        <h3
          className="font-semibold"
          style={{
            fontSize: '17px',
            fontWeight: 600,
            color: '#56649A',
            margin: `36px ${mobileTokens.spacing.xl} 12px`,
          }}
        >
          {title}
        </h3>
      )}
      <div>{children}</div>
    </div>
  )
}

interface MobileSettingItemProps {
  icon?: ReactNode
  title: string
  subtitle?: string
  value?: string
  onClick?: () => void
  showArrow?: boolean
  className?: string
}

/**
 * 移动端设置项
 * Hills Pro 风格：左侧图标 + 标题 + 右侧箭头
 */
export function MobileSettingItem({
  icon,
  title,
  subtitle,
  value,
  onClick,
  showArrow = true,
  className = '',
}: MobileSettingItemProps) {
  return (
    <motion.div
      whileTap={onClick ? { backgroundColor: 'rgba(0, 0, 0, 0.05)' } : undefined}
      onClick={onClick}
      className={`flex items-center ${className}`}
      style={{
        padding: `18px ${mobileTokens.spacing.xl}`,
        minHeight: '82px',
        cursor: onClick ? 'pointer' : 'default',
        borderBottom: `1px solid ${mobileTokens.bgAlt}`,
      }}
    >
      {/* 图标 */}
      {icon && (
        <div
          className="flex items-center justify-center"
          style={{
            width: '48px',
            height: '48px',
            borderRadius: mobileTokens.radius.md,
            background: mobileTokens.primarySoft,
            color: mobileTokens.primary,
            marginRight: '16px',
            flexShrink: 0,
          }}
        >
          {icon}
        </div>
      )}

      {/* 内容 */}
      <div className="flex-1 min-w-0">
        <h4
          className="font-medium"
          style={{
            fontSize: mobileTokens.fontSize.lg,
            color: mobileTokens.text,
          }}
        >
          {title}
        </h4>
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

      {/* 右侧 */}
      <div className="flex items-center gap-2 ml-4">
        {value && (
          <span
            style={{
              fontSize: mobileTokens.fontSize.md,
              color: mobileTokens.textMuted,
            }}
          >
            {value}
          </span>
        )}
        {showArrow && (
          <svg
            width="20"
            height="20"
            viewBox="0 0 24 24"
            fill="none"
            stroke={mobileTokens.textMuted}
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          >
            <polyline points="9 18 15 12 9 6" />
          </svg>
        )}
      </div>
    </motion.div>
  )
}
