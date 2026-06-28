import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Server, Settings, Home, Heart, Search } from 'lucide-react'
import MobileServersPage from './MobileServersPage'
import MobileAggregatePage from './MobileAggregatePage'
import MobileSettingsPage from './MobileSettingsPage'
import MobileServerHomePage from './MobileServerHomePage'
import MobileServerFavoritesPage from './MobileServerFavoritesPage'
import MobileServerSearchPage from './MobileServerSearchPage'
import { MobileShell, FloatingTabBar } from '@/components/mobile'
import type { FloatingTabBarItem } from '@/components/mobile'

// 移动端模式
type MobileMode = 'root' | 'server'

// 移动端主 Tab 类型
type MobileRootTab = 'servers' | 'aggregate' | 'settings'
type MobileServerTab = 'home' | 'favorites' | 'search'

// 根底部导航项
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

// 服务器内底部导航项
const serverNavItems: FloatingTabBarItem[] = [
  {
    key: 'home',
    label: '首页',
    icon: <Home size={22} />,
  },
  {
    key: 'favorites',
    label: '收藏',
    icon: <Heart size={22} />,
  },
  {
    key: 'search',
    label: '搜索',
    icon: <Search size={22} />,
  },
]

/**
 * 移动端应用主入口
 * 负责全局底部导航和页面切换
 */
export default function MobileApp() {
  const [mode, setMode] = useState<MobileMode>('root')
  const [rootTab, setRootTab] = useState<MobileRootTab>('servers')
  const [serverTab, setServerTab] = useState<MobileServerTab>('home')

  // 进入服务器详情
  const enterServer = () => {
    setMode('server')
    setServerTab('home')
  }

  // 返回服务器列表
  const exitServer = () => {
    setMode('root')
    setRootTab('servers')
  }

  return (
    <MobileShell>
      <AnimatePresence mode="wait">
        {/* 根模式 */}
        {mode === 'root' && (
          <motion.div
            key="root"
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.2, ease: 'easeOut' }}
            className="min-h-screen"
          >
            {rootTab === 'servers' && <MobileServersPage onEnterServer={enterServer} />}
            {rootTab === 'aggregate' && <MobileAggregatePage />}
            {rootTab === 'settings' && <MobileSettingsPage />}
          </motion.div>
        )}

        {/* 服务器详情模式 */}
        {mode === 'server' && (
          <motion.div
            key="server"
            initial={{ opacity: 0, x: 100 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -100 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
            className="min-h-screen"
          >
            {serverTab === 'home' && <MobileServerHomePage onBack={exitServer} />}
            {serverTab === 'favorites' && <MobileServerFavoritesPage onBack={exitServer} />}
            {serverTab === 'search' && <MobileServerSearchPage onBack={exitServer} />}
          </motion.div>
        )}
      </AnimatePresence>

      {/* 底部导航 */}
      <FloatingTabBar
        items={mode === 'root' ? rootNavItems : serverNavItems}
        activeKey={mode === 'root' ? rootTab : serverTab}
        onChange={(key) => {
          if (mode === 'root') {
            setRootTab(key as MobileRootTab)
          } else {
            setServerTab(key as MobileServerTab)
          }
        }}
      />
    </MobileShell>
  )
}
