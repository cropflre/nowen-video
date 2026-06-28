import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { FolderOpen } from 'lucide-react'
import {
  MobilePageHeader,
  MediaPosterCard,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'
import { mediaApi, libraryApi } from '@/api'

interface MobileLibraryDetailPageProps {
  libraryId: string
  onBack: () => void
  onGoMediaDetail: (id: string) => void
}

/**
 * 移动端媒体库详情页
 */
export default function MobileLibraryDetailPage({
  libraryId,
  onBack,
  onGoMediaDetail,
}: MobileLibraryDetailPageProps) {
  const [library, setLibrary] = useState<any>(null)
  const [mediaList, setMediaList] = useState<any[]>([])
  const [loading, setLoading] = useState(true)
  const [page, setPage] = useState(1)
  const [hasMore, setHasMore] = useState(true)

  // 获取媒体库信息
  useEffect(() => {
    libraryApi.list()
      .then(res => {
        if (res?.data) {
          const libs = Array.isArray(res.data) ? res.data : []
          const found = libs.find((lib: any) => lib.id === libraryId)
          if (found) {
            setLibrary(found)
          }
        }
      })
      .catch(() => {
        setLibrary(null)
      })
  }, [libraryId])

  // 获取媒体列表
  useEffect(() => {
    setLoading(true)
    mediaApi.list({ library_id: libraryId, page, size: 20 })
      .then(res => {
        if (res?.data) {
          const newMedia = Array.isArray(res.data) ? res.data : []
          if (page === 1) {
            setMediaList(newMedia)
          } else {
            setMediaList(prev => [...prev, ...newMedia])
          }
          setHasMore(newMedia.length >= 20)
        }
      })
      .catch(() => {
        if (page === 1) {
          setMediaList([])
        }
      })
      .finally(() => {
        setLoading(false)
      })
  }, [libraryId, page])

  // 加载更多
  const loadMore = () => {
    if (!loading && hasMore) {
      setPage(prev => prev + 1)
    }
  }

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title={library?.name || '媒体库'}
        onBack={onBack}
      />

      {/* 媒体列表 */}
      <div className="px-8">
        {loading && page === 1 ? (
          <div className="py-12 text-center" style={{ color: mobileTokens.textMuted }}>
            加载中...
          </div>
        ) : mediaList.length > 0 ? (
          <>
            <div
              className="grid gap-4"
              style={{
                gridTemplateColumns: 'repeat(2, 1fr)',
              }}
            >
              {mediaList.map((item: any) => (
                <MediaPosterCard
                  key={item.id}
                  title={item.title || '未知'}
                  year={item.year}
                  imageUrl={item.poster_path ? `/api/media/${item.id}/poster` : undefined}
                  onClick={() => onGoMediaDetail(item.id)}
                />
              ))}
            </div>

            {/* 加载更多 */}
            {hasMore && (
              <div className="py-6 text-center">
                <motion.button
                  whileTap={{ scale: 0.95 }}
                  onClick={loadMore}
                  disabled={loading}
                  className="px-6 py-2"
                  style={{
                    borderRadius: mobileTokens.radius.full,
                    background: mobileTokens.card,
                    border: `1px solid ${mobileTokens.cardBorder}`,
                    color: mobileTokens.text,
                    fontSize: mobileTokens.fontSize.sm,
                  }}
                >
                  {loading ? '加载中...' : '加载更多'}
                </motion.button>
              </div>
            )}
          </>
        ) : (
          <div
            className="flex flex-col items-center justify-center py-20"
            style={{ color: mobileTokens.textMuted }}
          >
            <FolderOpen size={48} style={{ opacity: 0.5, marginBottom: '16px' }} />
            <p className="text-center font-medium" style={{ fontSize: mobileTokens.fontSize.lg }}>
              暂无媒体
            </p>
            <p className="text-center mt-2" style={{ fontSize: mobileTokens.fontSize.sm }}>
              该媒体库还没有媒体文件
            </p>
          </div>
        )}
      </div>
    </>
  )
}
