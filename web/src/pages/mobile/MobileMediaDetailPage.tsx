import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Play, Heart } from 'lucide-react'
import {
  MobilePageHeader,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'
import { mediaApi } from '@/api'
import { useNavigate } from 'react-router-dom'

interface MobileMediaDetailPageProps {
  mediaId: string
  onBack: () => void
}

/**
 * 移动端媒体详情页
 */
export default function MobileMediaDetailPage({ mediaId, onBack }: MobileMediaDetailPageProps) {
  const navigate = useNavigate()
  const [media, setMedia] = useState<any>(null)
  const [loading, setLoading] = useState(true)
  const [isFavorite, setIsFavorite] = useState(false)

  // 获取媒体详情
  useEffect(() => {
    setLoading(true)
    mediaApi.detail(mediaId)
      .then(res => {
        if (res?.data) {
          setMedia(res.data)
        }
      })
      .catch(() => {
        setMedia(null)
      })
      .finally(() => {
        setLoading(false)
      })
  }, [mediaId])

  // 切换收藏
  const toggleFavorite = async () => {
    try {
      const { userApi } = await import('@/api')
      if (isFavorite) {
        await userApi.removeFavorite(mediaId)
      } else {
        await userApi.addFavorite(mediaId)
      }
      setIsFavorite(!isFavorite)
    } catch (error) {
      console.error('Failed to toggle favorite:', error)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center min-h-[60vh]">
        <div style={{ color: mobileTokens.textMuted }}>加载中...</div>
      </div>
    )
  }

  if (!media) {
    return (
      <div className="flex flex-col items-center justify-center min-h-[60vh] px-8">
        <MobilePageHeader title="详情" onBack={onBack} />
        <div style={{ color: mobileTokens.textMuted }}>媒体不存在</div>
      </div>
    )
  }

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title={media.title || '详情'}
        onBack={onBack}
      />

      {/* 海报和基本信息 */}
      <div className="px-8 mb-6">
        <div className="flex gap-4">
          {/* 海报 */}
          <div
            className="flex-shrink-0 overflow-hidden"
            style={{
              width: '120px',
              height: '180px',
              borderRadius: mobileTokens.radius.lg,
              background: mobileTokens.bgAlt,
            }}
          >
            {media.poster_path ? (
              <img
                src={`/api/media/${mediaId}/poster`}
                alt={media.title}
                className="w-full h-full object-cover"
              />
            ) : (
              <div
                className="w-full h-full flex items-center justify-center"
                style={{ color: mobileTokens.textMuted }}
              >
                <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                  <rect x="2" y="2" width="20" height="20" rx="2" ry="2" />
                  <circle cx="8.5" cy="8.5" r="1.5" />
                  <polyline points="21 15 16 10 5 21" />
                </svg>
              </div>
            )}
          </div>

          {/* 基本信息 */}
          <div className="flex-1 min-w-0">
            <h1
              className="font-bold mb-2"
              style={{
                fontSize: mobileTokens.fontSize.xl,
                color: mobileTokens.text,
                lineHeight: 1.3,
              }}
            >
              {media.title}
            </h1>

            {/* 信息行 */}
            <div className="flex flex-wrap gap-2 mb-3">
              {media.year && (
                <span
                  className="px-2 py-1 text-xs"
                  style={{
                    borderRadius: mobileTokens.radius.xs,
                    background: mobileTokens.primarySoft,
                    color: mobileTokens.primary,
                  }}
                >
                  {media.year}
                </span>
              )}
              {media.resolution && (
                <span
                  className="px-2 py-1 text-xs"
                  style={{
                    borderRadius: mobileTokens.radius.xs,
                    background: mobileTokens.primarySoft,
                    color: mobileTokens.primary,
                  }}
                >
                  {media.resolution}
                </span>
              )}
              {media.rating && (
                <span
                  className="px-2 py-1 text-xs"
                  style={{
                    borderRadius: mobileTokens.radius.xs,
                    background: mobileTokens.primarySoft,
                    color: mobileTokens.primary,
                  }}
                >
                  ⭐ {media.rating.toFixed(1)}
                </span>
              )}
            </div>

            {/* 时长 */}
            {media.duration > 0 && (
              <p
                style={{
                  fontSize: mobileTokens.fontSize.sm,
                  color: mobileTokens.textMuted,
                }}
              >
                {Math.floor(media.duration / 60)} 分钟
              </p>
            )}
          </div>
        </div>
      </div>

      {/* 操作按钮 */}
      <div className="flex gap-3 px-8 mb-6">
        <motion.button
          whileTap={{ scale: 0.95 }}
          onClick={() => navigate(`/play/${mediaId}`)}
          className="flex-1 flex items-center justify-center gap-2 py-3"
          style={{
            borderRadius: mobileTokens.radius.lg,
            background: mobileTokens.primary,
            color: '#fff',
            fontSize: mobileTokens.fontSize.md,
            fontWeight: 600,
          }}
        >
          <Play size={20} />
          播放
        </motion.button>

        <motion.button
          whileTap={{ scale: 0.95 }}
          onClick={toggleFavorite}
          className="flex items-center justify-center w-12 h-12"
          style={{
            borderRadius: mobileTokens.radius.lg,
            background: isFavorite ? '#FEE2E2' : mobileTokens.card,
            border: `1px solid ${mobileTokens.cardBorder}`,
            color: isFavorite ? '#EF4444' : mobileTokens.textMuted,
          }}
        >
          <Heart size={20} fill={isFavorite ? '#EF4444' : 'none'} />
        </motion.button>
      </div>

      {/* 简介 */}
      {media.overview && (
        <div className="px-8 mb-6">
          <h2
            className="font-semibold mb-2"
            style={{
              fontSize: mobileTokens.fontSize.lg,
              color: mobileTokens.text,
            }}
          >
            简介
          </h2>
          <p
            style={{
              fontSize: mobileTokens.fontSize.md,
              color: mobileTokens.textMuted,
              lineHeight: 1.6,
            }}
          >
            {media.overview}
          </p>
        </div>
      )}

      {/* 详细信息 */}
      <div className="px-8 mb-6">
        <h2
          className="font-semibold mb-3"
          style={{
            fontSize: mobileTokens.fontSize.lg,
            color: mobileTokens.text,
          }}
        >
          详细信息
        </h2>
        <div className="space-y-3">
          {media.studio && (
            <InfoItem label="工作室" value={media.studio} />
          )}
          {media.country && (
            <InfoItem label="国家/地区" value={media.country} />
          )}
          {media.language && (
            <InfoItem label="语言" value={media.language} />
          )}
          {media.genres && (
            <InfoItem label="类型" value={media.genres} />
          )}
          {media.directors && (
            <InfoItem label="导演" value={media.directors} />
          )}
          {media.actors && (
            <InfoItem label="演员" value={media.actors} />
          )}
        </div>
      </div>
    </>
  )
}

// 信息项组件
function InfoItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex">
      <span
        className="flex-shrink-0 w-20"
        style={{
          fontSize: mobileTokens.fontSize.sm,
          color: mobileTokens.textMuted,
        }}
      >
        {label}
      </span>
      <span
        style={{
          fontSize: mobileTokens.fontSize.sm,
          color: mobileTokens.text,
        }}
      >
        {value}
      </span>
    </div>
  )
}
