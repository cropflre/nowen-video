import { motion } from 'framer-motion'
import { Server, Wifi, WifiOff } from 'lucide-react'
import { mobileTokens } from '@/styles/mobile-tokens'

interface ServerCardProps {
  name: string
  type?: string
  lastAccess?: string
  isConnected?: boolean
  icon?: React.ReactNode
  onClick?: () => void
  className?: string
}

/**
 * 移动端服务器卡片
 * Hills Pro 风格：半透明背景 + 大圆角 + 细边框
 */
export default function ServerCard({
  name,
  lastAccess,
  isConnected = true,
  icon,
  onClick,
  className = '',
}: ServerCardProps) {
  // 服务器类型图标
  const getServerIcon = () => {
    if (icon) return icon
    return <Server size={28} style={{ color: mobileTokens.primary }} />
  }

  return (
    <motion.div
      whileTap={{ scale: 0.97 }}
      onClick={onClick}
      className={`relative overflow-hidden ${className}`}
      style={{
        width: '48%',
        minWidth: '160px',
        height: '120px',
        borderRadius: mobileTokens.radius.xl,
        background: mobileTokens.card,
        border: `1px solid ${mobileTokens.cardBorder}`,
        boxShadow: mobileTokens.shadowSm,
        cursor: onClick ? 'pointer' : 'default',
        padding: mobileTokens.spacing.lg,
      }}
    >
      <div className="flex h-full flex-col justify-between">
        {/* 顶部：图标和状态 */}
        <div className="flex items-start justify-between">
          <div
            className="flex items-center justify-center"
            style={{
              width: '48px',
              height: '48px',
              borderRadius: mobileTokens.radius.md,
              background: mobileTokens.primarySoft,
            }}
          >
            {getServerIcon()}
          </div>

          {/* 连接状态 */}
          <div
            style={{
              color: isConnected ? '#22C55E' : mobileTokens.textMuted,
            }}
          >
            {isConnected ? <Wifi size={18} /> : <WifiOff size={18} />}
          </div>
        </div>

        {/* 底部：名称和时间 */}
        <div>
          <h3
            className="font-semibold"
            style={{
              fontSize: mobileTokens.fontSize.lg,
              color: mobileTokens.text,
              lineHeight: 1.2,
            }}
          >
            {name}
          </h3>
          {lastAccess && (
            <p
              style={{
                fontSize: mobileTokens.fontSize.xs,
                color: mobileTokens.textMuted,
                marginTop: '4px',
              }}
            >
              {lastAccess}
            </p>
          )}
        </div>
      </div>
    </motion.div>
  )
}
