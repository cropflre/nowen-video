import { useState } from 'react'
import { motion } from 'framer-motion'
import { mobileTokens } from '@/styles/mobile-tokens'

interface MediaPosterCardProps {
  title: string
  year?: string | number
  imageUrl?: string
  progress?: number
  badges?: string[]
  aspect?: 'poster' | 'landscape'
  onClick?: () => void
  className?: string
}

/**
 * 移动端媒体海报卡片
 * Hills Pro 风格：大圆角 + 柔和阴影 + 进度条 + badge
 */
export default function MediaPosterCard({
  title,
  year,
  imageUrl,
  progress,
  badges,
  aspect = 'poster',
  onClick,
  className = '',
}: MediaPosterCardProps) {
  const [imageLoaded, setImageLoaded] = useState(false)
  const [imageError, setImageError] = useState(false)

  const aspectRatio = aspect === 'poster' ? '2/3' : '16/9'

  return (
    <motion.div
      whileTap={{ scale: 0.97 }}
      onClick={onClick}
      className={`relative overflow-hidden ${className}`}
      style={{
        borderRadius: mobileTokens.radius.lg,
        background: mobileTokens.card,
        border: `1px solid ${mobileTokens.cardBorder}`,
        boxShadow: mobileTokens.shadowSm,
        cursor: onClick ? 'pointer' : 'default',
      }}
    >
      {/* 图片容器 */}
      <div
        className="relative w-full overflow-hidden"
        style={{ aspectRatio }}
      >
        {/* 骨架屏 */}
        {!imageLoaded && !imageError && (
          <div
            className="absolute inset-0 animate-pulse"
            style={{
              background: `linear-gradient(90deg, ${mobileTokens.bgAlt} 0%, ${mobileTokens.bg} 50%, ${mobileTokens.bgAlt} 100%)`,
            }}
          />
        )}

        {/* 图片 */}
        {imageUrl && !imageError ? (
          <img
            src={imageUrl}
            alt={title}
            loading="lazy"
            onLoad={() => setImageLoaded(true)}
            onError={() => setImageError(true)}
            className="h-full w-full object-cover"
            style={{
              opacity: imageLoaded ? 1 : 0,
              transition: 'opacity 0.3s ease',
            }}
          />
        ) : (
          /* Fallback */
          <div
            className="flex h-full w-full items-center justify-center"
            style={{
              background: `linear-gradient(135deg, ${mobileTokens.primarySoft}, ${mobileTokens.bgAlt})`,
            }}
          >
            <svg
              width="48"
              height="48"
              viewBox="0 0 24 24"
              fill="none"
              stroke={mobileTokens.primary}
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              style={{ opacity: 0.5 }}
            >
              <rect x="2" y="2" width="20" height="20" rx="2" ry="2" />
              <circle cx="8.5" cy="8.5" r="1.5" />
              <polyline points="21 15 16 10 5 21" />
            </svg>
          </div>
        )}

        {/* 进度条 */}
        {progress !== undefined && progress > 0 && (
          <div
            className="absolute bottom-0 left-0 right-0"
            style={{ height: '3px', background: 'rgba(0, 0, 0, 0.3)' }}
          >
            <div
              className="h-full"
              style={{
                width: `${Math.min(progress, 100)}%`,
                background: mobileTokens.primary,
                borderRadius: '0 2px 2px 0',
              }}
            />
          </div>
        )}

        {/* Badges */}
        {badges && badges.length > 0 && (
          <div className="absolute top-2 left-2 flex flex-wrap gap-1">
            {badges.map((badge, index) => (
              <span
                key={index}
                className="px-2 py-0.5 text-xs font-medium"
                style={{
                  background: 'rgba(0, 0, 0, 0.6)',
                  color: '#fff',
                  borderRadius: mobileTokens.radius.xs,
                  backdropFilter: 'blur(4px)',
                }}
              >
                {badge}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* 标题区域 */}
      <div className="p-3">
        <h3
          className="line-clamp-2 font-medium"
          style={{
            fontSize: mobileTokens.fontSize.md,
            color: mobileTokens.text,
            lineHeight: 1.4,
          }}
        >
          {title}
        </h3>
        {year && (
          <p
            className="mt-1"
            style={{
              fontSize: mobileTokens.fontSize.sm,
              color: mobileTokens.textMuted,
            }}
          >
            {year}
          </p>
        )}
      </div>
    </motion.div>
  )
}
