import { useState, useEffect } from 'react'
import { motion } from 'framer-motion'
import { Search, FolderOpen, Play } from 'lucide-react'
import {
  MobilePageHeader,
  SegmentedTabs,
  MediaPosterCard,
  MediaRail,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'
import { mediaApi, libraryApi, userApi } from '@/api'

// Tab 配置
const aggregateTabs = [
  { key: 'continue', label: '继续观看' },
  { key: 'favorites', label: '收藏' },
  { key: 'libraries', label: '媒体库' },
]

/**
 * 移动端聚合视界页面
 * Hills Pro 风格：Tab 切换 + 双列媒体网格
 */
export default function MobileAggregatePage() {
  const [activeAggregateTab, setActiveAggregateTab] = useState('continue')
  const [continueWatching, setContinueWatching] = useState<any[]>([])
  const [favorites, setFavorites] = useState<any[]>([])
  const [libraries, setLibraries] = useState<any[]>([])

  // 类型断言辅助函数
  const extractData = (res: any) => {
    if (res?.data) return Array.isArray(res.data) ? res.data : []
    if (res?.items) return res.items
    return []
  }
  const [loading, setLoading] = useState(false)

  // 获取继续观看数据
  useEffect(() => {
    if (activeAggregateTab === 'continue') {
      setLoading(true)
      mediaApi.continueWatching(20)
        .then(res => {
          setContinueWatching(extractData(res))
        })
        .catch(() => {
          setContinueWatching([])
        })
        .finally(() => {
          setLoading(false)
        })
    }
  }, [activeAggregateTab])

  // 获取收藏数据
  useEffect(() => {
    if (activeAggregateTab === 'favorites') {
      setLoading(true)
      userApi.favorites(1, 50)
        .then(res => {
          setFavorites(extractData(res))
        })
        .catch(() => {
          setFavorites([])
        })
        .finally(() => {
          setLoading(false)
        })
    }
  }, [activeAggregateTab])

  // 获取媒体库数据
  useEffect(() => {
    if (activeAggregateTab === 'libraries') {
      setLoading(true)
      libraryApi.list()
        .then(res => {
          setLibraries(extractData(res))
        })
        .catch(() => {
          setLibraries([])
        })
        .finally(() => {
          setLoading(false)
        })
    }
  }, [activeAggregateTab])

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title="聚合视界"
        actions={[
          {
            icon: <Search size={22} />,
            onClick: () => {
              // TODO: 跳转搜索页
            },
            label: '搜索',
          },
          {
            icon: <FolderOpen size={22} />,
            onClick: () => {
              setActiveAggregateTab('libraries')
            },
            label: '媒体库',
          },
        ]}
      />

      {/* Tab 切换 */}
      <SegmentedTabs
        tabs={aggregateTabs}
        activeKey={activeAggregateTab}
        onChange={setActiveAggregateTab}
        className="mx-8 mb-6"
      />

      {/* 内容区域 */}
      <motion.div
        key={activeAggregateTab}
        initial={{ opacity: 0, x: 20 }}
        animate={{ opacity: 1, x: 0 }}
        transition={{ duration: 0.3 }}
      >
        {/* 继续观看 */}
        {activeAggregateTab === 'continue' && (
          <div>
            {loading ? (
              <div className="px-8 py-12 text-center" style={{ color: mobileTokens.textMuted }}>
                加载中...
              </div>
            ) : continueWatching.length > 0 ? (
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
                        // TODO: 跳转播放
                      }}
                    />
                  </div>
                ))}
              </MediaRail>
            ) : (
              <EmptyState
                icon={<Play size={48} />}
                title="暂无继续观看"
                subtitle="开始播放后会显示在这里"
              />
            )}
          </div>
        )}

        {/* 收藏 */}
        {activeAggregateTab === 'favorites' && (
          <div>
            {loading ? (
              <div className="px-8 py-12 text-center" style={{ color: mobileTokens.textMuted }}>
                加载中...
              </div>
            ) : favorites.length > 0 ? (
              <div className="px-8">
                <h2
                  className="font-semibold mb-4"
                  style={{
                    fontSize: mobileTokens.fontSize.xl,
                    color: mobileTokens.text,
                  }}
                >
                  收藏的媒体
                </h2>
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
                        // TODO: 跳转详情
                      }}
                    />
                  ))}
                </div>
              </div>
            ) : (
              <EmptyState
                icon={
                  <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5">
                    <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
                  </svg>
                }
                title="还没有收藏"
                subtitle="点亮喜欢的影片后会出现在这里"
              />
            )}
          </div>
        )}

        {/* 媒体库 */}
        {activeAggregateTab === 'libraries' && (
          <div>
            {loading ? (
              <div className="px-8 py-12 text-center" style={{ color: mobileTokens.textMuted }}>
                加载中...
              </div>
            ) : libraries.length > 0 ? (
              <div className="px-8">
                <h2
                  className="font-semibold mb-4"
                  style={{
                    fontSize: mobileTokens.fontSize.xl,
                    color: mobileTokens.text,
                  }}
                >
                  媒体库
                </h2>
                <div className="space-y-3">
                  {libraries.map((lib: any) => (
                    <motion.div
                      key={lib.id}
                      whileTap={{ scale: 0.98 }}
                      onClick={() => {
                        // TODO: 跳转媒体库详情
                      }}
                      className="flex items-center justify-between p-4"
                      style={{
                        borderRadius: mobileTokens.radius.lg,
                        background: mobileTokens.card,
                        border: `1px solid ${mobileTokens.cardBorder}`,
                      }}
                    >
                      <div className="flex items-center gap-3">
                        <FolderOpen size={24} style={{ color: mobileTokens.primary }} />
                        <span
                          className="font-medium"
                          style={{
                            fontSize: mobileTokens.fontSize.lg,
                            color: mobileTokens.text,
                          }}
                        >
                          {lib.name}
                        </span>
                      </div>
                      <div className="flex items-center gap-2">
                        <span
                          style={{
                            fontSize: mobileTokens.fontSize.sm,
                            color: mobileTokens.textMuted,
                          }}
                        >
                          {lib.media_count || 0} 部
                        </span>
                        <svg
                          width="20"
                          height="20"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke={mobileTokens.textMuted}
                          strokeWidth="2"
                        >
                          <polyline points="9 18 15 12 9 6" />
                        </svg>
                      </div>
                    </motion.div>
                  ))}
                </div>
              </div>
            ) : (
              <EmptyState
                icon={<FolderOpen size={48} />}
                title="暂无媒体库"
                subtitle="请先创建媒体库"
              />
            )}
          </div>
        )}
      </motion.div>
    </>
  )
}

// 空状态组件
function EmptyState({
  icon,
  title,
  subtitle,
}: {
  icon: React.ReactNode
  title: string
  subtitle: string
}) {
  return (
    <div
      className="flex flex-col items-center justify-center py-20 px-8"
      style={{ color: mobileTokens.textMuted }}
    >
      <div style={{ opacity: 0.5, marginBottom: '16px' }}>{icon}</div>
      <p
        className="text-center font-medium"
        style={{ fontSize: mobileTokens.fontSize.lg }}
      >
        {title}
      </p>
      <p
        className="text-center mt-2"
        style={{ fontSize: mobileTokens.fontSize.sm }}
      >
        {subtitle}
      </p>
    </div>
  )
}
