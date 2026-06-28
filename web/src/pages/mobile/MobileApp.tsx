import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Server, Settings } from 'lucide-react'
import MobileServersPage from './MobileServersPage'
import MobileAggregatePage from './MobileAggregatePage'
import MobileSettingsPage from './MobileSettingsPage'
import { MobileShell, FloatingTabBar } from '@/components/mobile'
import type { FloatingTabBarItem } from '@/components/mobile'

// 移动端主 Tab 类型
type MobileRootTab = 'servers' | 'aggregate' | 'settings'

// 底部导航项配置
const rootNavItems: FloatingTabBarItem[] = [
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
 * 移动端应用主入口
 * 负责全局底部导航和页面切换
 */
export default function MobileApp() {
  const [activeTab, setActiveTab] = useState<MobileRootTab>('servers')

  return (
    <MobileShell>
      <AnimatePresence mode="wait">
        <motion.div
          key={activeTab}
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -8 }}
          transition={{ duration: 0.2, ease: 'easeOut' }}
          className="min-h-screen"
        >
          {activeTab === 'servers' && <MobileServersPage />}
          {activeTab === 'aggregate' && <MobileAggregatePage />}
          {activeTab === 'settings' && <MobileSettingsPage />}
        </motion.div>
      </AnimatePresence>

      {/* 全局底部导航 */}
      <FloatingTabBar
        items={rootNavItems}
        activeKey={activeTab}
        onChange={(key) => setActiveTab(key as MobileRootTab)}
      />
    </MobileShell>
  )
}
