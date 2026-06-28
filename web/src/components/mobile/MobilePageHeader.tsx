import { ReactNode } from 'react'
import { motion } from 'framer-motion'
import { mobileTokens } from '@/styles/mobile-tokens'

interface MobilePageHeaderAction {
  icon: ReactNode
  onClick: () => void
  label?: string
}

interface MobilePageHeaderProps {
  title: string
  subtitle?: string
  actions?: MobilePageHeaderAction[]
  onBack?: () => void
  backIcon?: ReactNode
  /** 是否使用大标题风格 */
  large?: boolean
}

/**
 * 移动端页面标题
 * Hills Pro 风格：大字号 + 左对齐 + 右侧操作按钮
 */
export default function MobilePageHeader({
  title,
  subtitle,
  actions,
  onBack,
  backIcon,
  large = true,
}: MobilePageHeaderProps) {
  return (
    <div
      className="flex items-center justify-between"
      style={{
        padding: `42px ${mobileTokens.spacing.xl} ${mobileTokens.spacing.xl}`,
      }}
    >
      <div className="flex items-center gap-3">
        {onBack && (
          <motion.button
            whileTap={{ scale: 0.95 }}
            onClick={onBack}
            className="flex items-center justify-center"
            style={{
              width: '40px',
              height: '40px',
              borderRadius: mobileTokens.radius.md,
              color: mobileTokens.text,
            }}
          >
            {backIcon || (
              <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                <path d="M19 12H5M12 19l-7-7 7-7" />
              </svg>
            )}
          </motion.button>
        )}
        <div>
          <motion.h1
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
            style={{
              fontSize: large ? mobileTokens.fontSize['3xl'] : mobileTokens.fontSize.xl,
              fontWeight: large ? 600 : 500,
              color: mobileTokens.text,
              lineHeight: 1.2,
            }}
          >
            {title}
          </motion.h1>
          {subtitle && (
            <motion.p
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.3, delay: 0.1 }}
              style={{
                fontSize: mobileTokens.fontSize.sm,
                color: mobileTokens.textMuted,
                marginTop: '4px',
              }}
            >
              {subtitle}
            </motion.p>
          )}
        </div>
      </div>

      {actions && actions.length > 0 && (
        <div className="flex items-center gap-2">
          {actions.map((action, index) => (
            <motion.button
              key={index}
              whileTap={{ scale: 0.95 }}
              onClick={action.onClick}
              aria-label={action.label}
              className="flex items-center justify-center"
              style={{
                width: '40px',
                height: '40px',
                borderRadius: mobileTokens.radius.md,
                color: mobileTokens.textMuted,
                transition: 'background-color 0.2s ease',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.backgroundColor = 'rgba(0, 0, 0, 0.05)'
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.backgroundColor = 'transparent'
              }}
            >
              {action.icon}
            </motion.button>
          ))}
        </div>
      )}
    </div>
  )
}
