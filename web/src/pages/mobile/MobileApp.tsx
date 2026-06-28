import { useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import MobileServersPage from './MobileServersPage'
import MobileAggregatePage from './MobileAggregatePage'
import MobileSettingsPage from './MobileSettingsPage'
import { useIsMobile } from '@/hooks/useMobile'

/**
 * 移动端应用主入口
 * 负责全局底部导航和页面切换
 */
export default function MobileApp() {
  const [activeTab] = useState('servers')
  const isMobile = useIsMobile()

  // 桌面端不渲染移动端组件
  if (!isMobile) {
    return null
  }

  // 根据 activeTab 渲染对应页面
  const renderPage = () => {
    switch (activeTab) {
      case 'servers':
        return <MobileServersPage />
      case 'aggregate':
        return <MobileAggregatePage />
      case 'settings':
        return <MobileSettingsPage />
      default:
        return <MobileServersPage />
    }
  }

  return (
    <div className="min-h-screen" style={{ background: '#F8F5FB' }}>
      <AnimatePresence mode="wait">
        <motion.div
          key={activeTab}
          initial={{ opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: -8 }}
          transition={{ duration: 0.2, ease: 'easeOut' }}
        >
          {renderPage()}
        </motion.div>
      </AnimatePresence>
    </div>
  )
}
