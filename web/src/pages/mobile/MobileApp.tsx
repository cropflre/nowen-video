import { useState, useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { Server, Settings, Home, Heart, Search } from 'lucide-react'
import { useLocation, useNavigate } from 'react-router-dom'
import MobileServersPage from './MobileServersPage'
import MobileAggregatePage from './MobileAggregatePage'
import MobileSettingsPage from './MobileSettingsPage'
import MobileServerHomePage from './MobileServerHomePage'
import MobileServerFavoritesPage from './MobileServerFavoritesPage'
import MobileServerSearchPage from './MobileServerSearchPage'
import MobileMediaDetailPage from './MobileMediaDetailPage'
import MobileLibraryDetailPage from './MobileLibraryDetailPage'
import { MobileShell, FloatingTabBar } from '@/components/mobile'
import type { FloatingTabBarItem } from '@/components/mobile'

// 移动端模式
type MobileMode = 'root' | 'server' | 'detail'

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

interface MobileAppProps {
  initialPath?: string
}

/**
 * 移动端应用主入口
 * 负责全局底部导航和页面切换
 */
export default function MobileApp({ initialPath }: MobileAppProps) {
  const location = useLocation()
  const navigate = useNavigate()
  const [mode, setMode] = useState<MobileMode>('root')
  const [rootTab, setRootTab] = useState<MobileRootTab>('servers')
  const [serverTab, setServerTab] = useState<MobileServerTab>('home')
  const [detailType, setDetailType] = useState<'media' | 'library' | null>(null)
  const [detailId, setDetailId] = useState<string | null>(null)

  // 根据 URL 路径判断初始状态
  useEffect(() => {
    const path = location.pathname

    if (path.startsWith('/media/')) {
      const id = path.split('/media/')[1]
      if (id) {
        setMode('detail')
        setDetailType('media')
        setDetailId(id)
      }
    } else if (path.startsWith('/library/')) {
      const id = path.split('/library/')[1]
      if (id) {
        setMode('detail')
        setDetailType('library')
        setDetailId(id)
      }
    } else if (path === '/search') {
      setMode('server')
      setServerTab('search')
    } else if (path === '/favorites') {
      setMode('server')
      setServerTab('favorites')
    } else if (initialPath) {
      // 处理 initialPath
      if (initialPath === '/media' || initialPath === '/library') {
        // 从 URL 获取 ID
        const id = path.split('/').pop()
        if (id) {
          setMode('detail')
          setDetailType(initialPath === '/media' ? 'media' : 'library')
          setDetailId(id)
        }
      }
    }
  }, [location.pathname, initialPath])

  // 进入服务器详情
  const enterServer = () => {
    setMode('server')
    setServerTab('home')
  }

  // 返回上一页
  const goBack = () => {
    if (mode === 'detail') {
      // 从详情页返回
      navigate(-1)
      // 延迟重置状态，等待动画完成
      setTimeout(() => {
        setMode('root')
        setRootTab('servers')
        setDetailType(null)
        setDetailId(null)
      }, 300)
    } else if (mode === 'server') {
      // 从服务器内页返回
      setMode('root')
      setRootTab('servers')
    }
  }

  // 进入媒体详情
  const goMediaDetail = (id: string) => {
    navigate(`/media/${id}`)
    setMode('detail')
    setDetailType('media')
    setDetailId(id)
  }

  // 进入媒体库详情
  const goLibraryDetail = (id: string) => {
    navigate(`/library/${id}`)
    setMode('detail')
    setDetailType('library')
    setDetailId(id)
  }

  // 切换到搜索 tab
  const goSearch = () => {
    setMode('server')
    setServerTab('search')
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
            {serverTab === 'home' && (
              <MobileServerHomePage
                onBack={goBack}
                onGoSearch={goSearch}
                onGoMediaDetail={goMediaDetail}
                onGoLibraryDetail={goLibraryDetail}
              />
            )}
            {serverTab === 'favorites' && (
              <MobileServerFavoritesPage
                onBack={goBack}
                onGoMediaDetail={goMediaDetail}
              />
            )}
            {serverTab === 'search' && (
              <MobileServerSearchPage
                onBack={goBack}
                onGoMediaDetail={goMediaDetail}
              />
            )}
          </motion.div>
        )}

        {/* 详情模式 */}
        {mode === 'detail' && detailId && (
          <motion.div
            key="detail"
            initial={{ opacity: 0, x: 100 }}
            animate={{ opacity: 1, x: 0 }}
            exit={{ opacity: 0, x: -100 }}
            transition={{ duration: 0.3, ease: 'easeOut' }}
            className="min-h-screen"
          >
            {detailType === 'media' && (
              <MobileMediaDetailPage
                mediaId={detailId}
                onBack={goBack}
              />
            )}
            {detailType === 'library' && (
              <MobileLibraryDetailPage
                libraryId={detailId}
                onBack={goBack}
                onGoMediaDetail={goMediaDetail}
              />
            )}
          </motion.div>
        )}
      </AnimatePresence>

      {/* 底部导航（详情模式隐藏） */}
      {mode !== 'detail' && (
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
      )}
    </MobileShell>
  )
}
