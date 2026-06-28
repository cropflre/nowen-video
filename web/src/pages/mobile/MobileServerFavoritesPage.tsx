import { useState, useEffect } from 'react'
import { Heart } from 'lucide-react'
import {
  MobilePageHeader,
  MediaPosterCard,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'
import { userApi } from '@/api'
import { useNavigate } from 'react-router-dom'

interface MobileServerFavoritesPageProps {
  onBack: () => void
}

/**
 * 移动端服务器收藏页
 */
export default function MobileServerFavoritesPage({ onBack }: MobileServerFavoritesPageProps) {
  const navigate = useNavigate()
  const [favorites, setFavorites] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  // 获取收藏数据
  useEffect(() => {
    setLoading(true)
    userApi.favorites(1, 50)
      .then(res => {
        if (res?.data) {
          setFavorites(Array.isArray(res.data) ? res.data : [])
        }
      })
      .catch(() => {
        setFavorites([])
      })
      .finally(() => {
        setLoading(false)
      })
  }, [])

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title="收藏"
        onBack={onBack}
      />

      {/* 收藏列表 */}
      <div className="px-8">
        {loading ? (
          <div className="py-12 text-center" style={{ color: mobileTokens.textMuted }}>
            加载中...
          </div>
        ) : favorites.length > 0 ? (
          <div
            className="grid gap-4"
            style={{
              gridTemplateColumns: 'repeat(2, 1fr)',
            }}
          >
            {favorites.map((item: any) => (
              <MediaPosterCard
                key={item.id}
                title={item.title || '未知'}
                year={item.year}
                imageUrl={item.poster_path ? `/api/media/${item.id}/poster` : undefined}
                onClick={() => {
                  navigate(`/media/${item.id}`)
                }}
              />
            ))}
          </div>
        ) : (
          <div
            className="flex flex-col items-center justify-center py-20"
            style={{ color: mobileTokens.textMuted }}
          >
            <Heart size={48} style={{ opacity: 0.5, marginBottom: '16px' }} />
            <p className="text-center font-medium" style={{ fontSize: mobileTokens.fontSize.lg }}>
              还没有收藏
            </p>
            <p className="text-center mt-2" style={{ fontSize: mobileTokens.fontSize.sm }}>
              点亮喜欢的影片后会出现在这里
            </p>
          </div>
        )}
      </div>
    </>
  )
}
