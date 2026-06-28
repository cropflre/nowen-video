import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Search, Play } from 'lucide-react'
import {
  MobilePageHeader,
  MediaPosterCard,
  MediaRail,
  MobileHeroCarousel,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'
import { mediaApi, libraryApi } from '@/api'
import { useNavigate } from 'react-router-dom'

interface MobileServerHomePageProps {
  onBack: () => void
  onGoSearch?: () => void
  onGoMediaDetail?: (id: string) => void
  onGoLibraryDetail?: (id: string) => void
}

/**
 * 移动端服务器首页
 * 展示 Hero、继续观看、媒体库、最近添加
 */
export default function MobileServerHomePage({
  onBack,
  onGoSearch,
  onGoMediaDetail,
  onGoLibraryDetail,
}: MobileServerHomePageProps) {
  const navigate = useNavigate()
  const [continueWatching, setContinueWatching] = useState<any[]>([])
  const [libraries, setLibraries] = useState<any[]>([])
  const [recentMedia, setRecentMedia] = useState<any[]>([])
  const [loading, setLoading] = useState(true)

  // 获取数据
  useEffect(() => {
    const fetchData = async () => {
      setLoading(true)
      try {
        // 并行请求
        const [continueRes, libRes, recentRes] = await Promise.allSettled([
          mediaApi.continueWatching(10),
          libraryApi.list(),
          mediaApi.recent(20),
        ])

        if (continueRes.status === 'fulfilled' && continueRes.value?.data) {
          setContinueWatching(Array.isArray(continueRes.value.data) ? continueRes.value.data : [])
        }

        if (libRes.status === 'fulfilled' && libRes.value?.data) {
          setLibraries(Array.isArray(libRes.value.data) ? libRes.value.data : [])
        }

        if (recentRes.status === 'fulfilled' && recentRes.value?.data) {
          setRecentMedia(Array.isArray(recentRes.value.data) ? recentRes.value.data : [])
        }
      } catch (error) {
        console.error('Failed to fetch data:', error)
      } finally {
        setLoading(false)
      }
    }

    fetchData()
  }, [])

  // Hero 数据（使用最近添加的媒体）
  const heroItems = recentMedia.slice(0, 5).map(item => ({
    id: item.id,
    title: item.title || '未知',
    subtitle: item.overview?.substring(0, 100) || '',
    imageUrl: item.poster_path ? `/api/media/${item.id}/poster` : undefined,
    rating: item.rating,
    year: item.year,
    resolution: item.resolution,
  }))

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title="Nowen Video"
        onBack={onBack}
        actions={[
          {
            icon: <Search size={22} />,
            onClick: onGoSearch || (() => {}),
            label: '搜索',
          },
        ]}
      />

      {/* Hero 轮播 */}
      {heroItems.length > 0 && (
        <MobileHeroCarousel
          items={heroItems}
          autoPlay
          interval={6500}
          onItemClick={(item) => {
            if (onGoMediaDetail) {
              onGoMediaDetail(item.id)
            } else {
              navigate(`/media/${item.id}`)
            }
          }}
        />
      )}

      {/* 继续观看 */}
      {continueWatching.length > 0 && (
        <MediaRail title="继续观看">
          {continueWatching.map((item: any) => (
            <div key={item.id} style={{ minWidth: '280px' }}>
              <MediaPosterCard
                title={item.title || item.episode_title || '未知'}
                year={item.year}
                imageUrl={item.thumbnail_path ? `/api/media/${item.id}/thumbnail` : undefined}
                progress={item.duration > 0 ? Math.round((item.position / item.duration) * 100) : 0}
                aspect="landscape"
                onClick={() => {
                  navigate(`/play/${item.id}`)
                }}
              />
            </div>
          ))}
        </MediaRail>
      )}

      {/* 媒体库 */}
      {libraries.length > 0 && (
        <MediaRail title="媒体库">
          {libraries.map((lib: any) => (
            <motion.div
              key={lib.id}
              whileTap={{ scale: 0.98 }}
              onClick={() => {
                if (onGoLibraryDetail) {
                  onGoLibraryDetail(lib.id)
                } else {
                  navigate(`/library/${lib.id}`)
                }
              }}
              className="flex items-center gap-3 p-4"
              style={{
                minWidth: '160px',
                borderRadius: mobileTokens.radius.lg,
                background: mobileTokens.card,
                border: `1px solid ${mobileTokens.cardBorder}`,
              }}
            >
              <div
                className="flex items-center justify-center"
                style={{
                  width: '40px',
                  height: '40px',
                  borderRadius: mobileTokens.radius.md,
                  background: mobileTokens.primarySoft,
                }}
              >
                <svg width="24" height="24" viewBox="0 0 24 24" fill="none" stroke={mobileTokens.primary} strokeWidth="2">
                  <path d="M22 19a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h5l2 3h9a2 2 0 0 1 2 2z" />
                </svg>
              </div>
              <div>
                <div className="font-medium" style={{ fontSize: mobileTokens.fontSize.md, color: mobileTokens.text }}>
                  {lib.name}
                </div>
                <div style={{ fontSize: mobileTokens.fontSize.xs, color: mobileTokens.textMuted }}>
                  {lib.media_count || 0} 部
                </div>
              </div>
            </motion.div>
          ))}
        </MediaRail>
      )}

      {/* 最近添加 */}
      {recentMedia.length > 0 && (
        <MediaRail title="最近添加">
          {recentMedia.map((item: any) => (
            <div key={item.id} style={{ minWidth: '140px' }}>
              <MediaPosterCard
                title={item.title || '未知'}
                year={item.year}
                imageUrl={item.poster_path ? `/api/media/${item.id}/poster` : undefined}
                onClick={() => {
                  if (onGoMediaDetail) {
                    onGoMediaDetail(item.id)
                  } else {
                    navigate(`/media/${item.id}`)
                  }
                }}
              />
            </div>
          ))}
        </MediaRail>
      )}

      {/* 空状态 */}
      {!loading && continueWatching.length === 0 && libraries.length === 0 && recentMedia.length === 0 && (
        <div
          className="flex flex-col items-center justify-center py-20 px-8"
          style={{ color: mobileTokens.textMuted }}
        >
          <Play size={48} style={{ opacity: 0.5, marginBottom: '16px' }} />
          <p className="text-center font-medium" style={{ fontSize: mobileTokens.fontSize.lg }}>
            暂无内容
          </p>
          <p className="text-center mt-2" style={{ fontSize: mobileTokens.fontSize.sm }}>
            请先添加媒体库和媒体文件
          </p>
        </div>
      )}
    </>
  )
}
