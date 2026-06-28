import { ReactNode } from 'react'
import { motion } from 'framer-motion'
import { mobileTokens } from '@/styles/mobile-tokens'

interface FloatingActionButtonProps {
  icon: ReactNode
  onClick: () => void
  label?: string
  size?: 'sm' | 'md' | 'lg'
  color?: string
  className?: string
}

/**
 * 悬浮操作按钮
 * Hills Pro 风格：圆角 + 阴影 + 弹性动画
 */
export default function FloatingActionButton({
  icon,
  onClick,
  label,
  size = 'md',
  color,
  className = '',
}: FloatingActionButtonProps) {
  const sizeMap = {
    sm: '52px',
    md: '64px',
    lg: '72px',
  }

  const iconSizeMap = {
    sm: 20,
    md: 24,
    lg: 28,
  }

  return (
    <motion.button
      initial={{ scale: 0, opacity: 0 }}
      animate={{ scale: 1, opacity: 1 }}
      whileTap={{ scale: 0.95 }}
      whileHover={{ scale: 1.05 }}
      transition={{ type: 'spring', stiffness: 400, damping: 20 }}
      onClick={onClick}
      aria-label={label}
      className={`fixed z-40 flex items-center justify-center ${className}`}
      style={{
        right: '24px',
        bottom: `calc(100px + env(safe-area-inset-bottom, 0px))`,
        width: sizeMap[size],
        height: sizeMap[size],
        borderRadius: mobileTokens.radius.lg,
        background: color || mobileTokens.primarySoft,
        color: color ? '#fff' : mobileTokens.primary,
        boxShadow: mobileTokens.shadow,
        border: `1px solid ${mobileTokens.glassBorder}`,
      }}
    >
      <div style={{ transform: `scale(${iconSizeMap[size] / 24})` }}>
        {icon}
      </div>
    </motion.button>
  )
}
