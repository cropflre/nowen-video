import { ReactNode } from 'react'
import { mobileTokens } from '@/styles/mobile-tokens'

interface MobileShellProps {
  children: ReactNode
  /** 是否显示底部导航预留空间 */
  hasNav?: boolean
  /** 自定义背景色 */
  background?: string
  /** 自定义类名 */
  className?: string
}

/**
 * 移动端外壳组件
 * 负责：背景、safe area、页面 padding、底部导航预留空间
 */
export default function MobileShell({
  children,
  hasNav = true,
  background,
  className = '',
}: MobileShellProps) {
  return (
    <div
      className={`min-h-screen w-full ${className}`}
      style={{
        background: background || mobileTokens.bg,
        paddingTop: 'env(safe-area-inset-top)',
        paddingBottom: hasNav ? `calc(112px + env(safe-area-inset-bottom))` : 'env(safe-area-inset-bottom)',
      }}
    >
      {children}
    </div>
  )
}
