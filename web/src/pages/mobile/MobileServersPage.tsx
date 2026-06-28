import { useState } from 'react'
import { motion } from 'framer-motion'
import { Download, MoreVertical, Plus, Server } from 'lucide-react'
import {
  MobileShell,
  MobilePageHeader,
  ServerCard,
  FloatingActionButton,
  FloatingTabBar,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'

// 模拟服务器数据
const mockServers = [
  {
    id: '1',
    name: 'emby',
    type: 'emby',
    lastAccess: '27 天前',
    isConnected: true,
  },
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
    icon: (
      <svg width="22" height="22" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
        <circle cx="12" cy="12" r="3" />
        <path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1 0 2.83 2 2 0 0 1-2.83 0l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-2 2 2 2 0 0 1-2-2v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83 0 2 2 0 0 1 0-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1-2-2 2 2 0 0 1 2-2h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 0-2.83 2 2 0 0 1 2.83 0l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 2-2 2 2 0 0 1 2 2v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 0 2 2 0 0 1 0 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 2 2 2 2 0 0 1-2 2h-.09a1.65 1.65 0 0 0-1.51 1z" />
      </svg>
    ),
  },
]

/**
 * 移动端服务器页面
 * Hills Pro 风格
 */
export default function MobileServersPage() {
  const [activeTab, setActiveTab] = useState('servers')

  return (
    <MobileShell>
      {/* 页面标题 */}
      <MobilePageHeader
        title="服务器"
        actions={[
          {
            icon: <Download size={22} />,
            onClick: () => console.log('Import'),
            label: '导入',
          },
          {
            icon: <MoreVertical size={22} />,
            onClick: () => console.log('Menu'),
            label: '更多',
          },
        ]}
      />

      {/* 服务器列表 */}
      <div
        className="flex flex-wrap gap-4 px-8"
        style={{ marginTop: '20px' }}
      >
        {mockServers.map((server, index) => (
          <motion.div
            key={server.id}
            initial={{ opacity: 0, y: 20 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.3, delay: index * 0.1 }}
          >
            <ServerCard
              name={server.name}
              type={server.type}
              lastAccess={server.lastAccess}
              isConnected={server.isConnected}
              onClick={() => console.log('Click server', server.id)}
            />
          </motion.div>
        ))}

        {/* 空状态提示 */}
        {mockServers.length === 0 && (
          <div
            className="flex flex-col items-center justify-center w-full py-20"
            style={{ color: mobileTokens.textMuted }}
          >
            <Server size={48} style={{ opacity: 0.5, marginBottom: '16px' }} />
            <p
              className="text-center"
              style={{ fontSize: mobileTokens.fontSize.lg }}
            >
              还没有服务器
            </p>
            <p
              className="text-center mt-2"
              style={{ fontSize: mobileTokens.fontSize.sm }}
            >
              点击右下角 + 添加你的媒体服务器
            </p>
          </div>
        )}
      </div>

      {/* 悬浮添加按钮 */}
      <FloatingActionButton
        icon={<Plus size={24} />}
        onClick={() => console.log('Add server')}
        label="添加服务器"
      />

      {/* 底部导航 */}
      <FloatingTabBar
        items={navItems}
        activeKey={activeTab}
        onChange={setActiveTab}
      />
    </MobileShell>
  )
}
