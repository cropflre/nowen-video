import { useState } from 'react'
import { motion } from 'framer-motion'
import { Search, FolderOpen, Server, Settings, Play } from 'lucide-react'
import {
  MobileShell,
  MobilePageHeader,
  SegmentedTabs,
  MediaPosterCard,
  MediaRail,
  FloatingTabBar,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'

// 模拟数据
const mockContinueWatching = [
  { id: '1', title: '示例电影 1', year: 2024, progress: 45 },
  { id: '2', title: '示例电影 2', year: 2023, progress: 78 },
  { id: '3', title: '示例剧集 S01E05', year: 2024, progress: 30 },
]

const mockFavorites = [
  { id: '4', title: '收藏电影 1', year: 2024 },
  { id: '5', title: '收藏电影 2', year: 2023 },
  { id: '6', title: '收藏电影 3', year: 2022 },
]

const mockLibraries = [
  { id: 'lib1', name: '电影', count: 128 },
  { id: 'lib2', name: '电视剧', count: 45 },
  { id: 'lib3', name: '动画', count: 67 },
]

// Tab 配置
const aggregateTabs = [
  { key: 'continue', label: '继续观看' },
  { key: 'favorites', label: '收藏' },
  { key: 'libraries', label: '媒体库' },
]

// 底部导航项
const navItems = [
  {
    key: 'servers',
    label: '服务器',
    icon: <Server size={22} />,
  },
  {
    key: 'aggregate',
    label: '聚合视界',
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <rect x="2" y="2" width="8" height="8" rx="2" />
        <rect x="14" y="2" width="8" height="8" rx="2" />
        <rect x="2" y="14" width="8" height="8" rx="2" />
        <rect x="14" y="14" width="8" height="8" rx="2" />
      </svg>
    ),
  },
  {
    key: 'settings',
    label: '设置',
    icon: <Settings size={22} />,
  },
]

/**
 * 移动端聚合视界页面
 * Hills Pro 风格：Tab 切换 + 双列媒体网格
 */
export default function MobileAggregatePage() {
  const [activeTab, setActiveTab] = useState('aggregate')
  const [activeAggregateTab, setActiveAggregateTab] = useState('continue')

  return (
    <MobileShell>
      {/* 页面标题 */}
      <MobilePageHeader
        title="聚合视界"
        actions={[
          {
            icon: <Search size={22} />,
            onClick: () => console.log('Search'),
            label: '搜索',
          },
          {
            icon: <FolderOpen size={22} />,
            onClick: () => console.log('Libraries'),
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
            {mockContinueWatching.length > 0 ? (
              <MediaRail title="继续观看">
                {mockContinueWatching.map((item) => (
                  <div key={item.id} style={{ minWidth: '280px' }}>
                    <MediaPosterCard
                      title={item.title}
                      year={item.year}
                      progress={item.progress}
                      aspect="landscape"
                      onClick={() => console.log('Play', item.id)}
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
            {mockFavorites.length > 0 ? (
              <div className="px-8">
                <h2
                  className="font-semibold mb-4"
                  style={{
                    fontSize: mobileTokens.fontSize.xl,
                    color: mobileTokens.text,
                  }}
                >
                  收藏的电影
                </h2>
                <div
                  className="grid gap-4"
                  style={{
                    gridTemplateColumns: 'repeat(2, 1fr)',
                  }}
                >
                  {mockFavorites.map((item) => (
                    <MediaPosterCard
                      key={item.id}
                      title={item.title}
                      year={item.year}
                      onClick={() => console.log('View', item.id)}
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
            <div className="px-8">
              <h2
                className="font-semibold mb-4"
                style={{
                  fontSize: mobileTokens.fontSize.xl,
                  color: mobileTokens.text,
                }}
              >
                emby
              </h2>
              <div className="space-y-3">
                {mockLibraries.map((lib) => (
                  <motion.div
                    key={lib.id}
                    whileTap={{ scale: 0.98 }}
                    onClick={() => console.log('Open library', lib.id)}
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
                        {lib.count} 部
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
          </div>
        )}
      </motion.div>

      {/* 底部导航 */}
      <FloatingTabBar
        items={navItems}
        activeKey={activeTab}
        onChange={setActiveTab}
      />
    </MobileShell>
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
