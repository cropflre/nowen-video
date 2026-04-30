import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import { useAuthStore } from '@/stores/auth'
import { ToastProvider } from '@/components/Toast'
import { Toaster } from 'react-hot-toast'
import Layout from '@/components/Layout'
import TitleBar from '@/components/TitleBar'
import LoginPage from '@/pages/LoginPage'
import ForceChangePasswordPage from '@/pages/ForceChangePasswordPage'
import { DesktopEventBinder, DesktopServerPicker, UpdateBanner } from '@/desktop'

// 懒加载页面组件 — 按需加载，减少首屏 JS 体积
const HomePage = lazy(() => import('@/pages/HomePage'))
const LibraryPage = lazy(() => import('@/pages/LibraryPage'))
const MediaDetailPage = lazy(() => import('@/pages/MediaDetailPage'))
const PlayerPage = lazy(() => import('@/pages/PlayerPage'))
const SearchPage = lazy(() => import('@/pages/SearchPage'))
const FavoritesPage = lazy(() => import('@/pages/FavoritesPage'))
const HistoryPage = lazy(() => import('@/pages/HistoryPage'))
const PlaylistsPage = lazy(() => import('@/pages/PlaylistsPage'))
const AdminPage = lazy(() => import('@/pages/AdminPage'))
const SeriesDetailPage = lazy(() => import('@/pages/SeriesDetailPage'))
const ProfilePage = lazy(() => import('@/pages/ProfilePage'))
const StatsPage = lazy(() => import('@/pages/StatsPage'))
const FileManagerPage = lazy(() => import('@/pages/FileManagerPage'))
const PulsePage = lazy(() => import('@/pages/PulsePage'))
const PreprocessPage = lazy(() => import('@/pages/PreprocessPage'))
const SubtitlePreprocessPage = lazy(() => import('@/pages/SubtitlePreprocessPage'))
const BrowsePage = lazy(() => import('@/pages/BrowsePage'))
const PersonDetailPage = lazy(() => import('@/pages/PersonDetailPage'))
const CollectionsPage = lazy(() => import('@/pages/CollectionsPage'))
const CollectionDetailPage = lazy(() => import('@/pages/CollectionDetailPage'))

// 页面加载中的占位组件 — 品牌化霓虹脉冲环
function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[60vh] animate-fade-in">
      <div className="flex flex-col items-center gap-4">
        {/* 双层霓虹旋转环 */}
        <div className="relative h-12 w-12">
          <div
            className="absolute inset-0 rounded-full animate-glow-pulse"
            style={{ border: '2px solid var(--neon-blue-20)' }}
          />
          <div
            className="absolute inset-0 rounded-full animate-spin"
            style={{
              border: '2px solid transparent',
              borderTopColor: 'var(--neon-blue)',
              borderRightColor: 'var(--neon-blue-40)',
            }}
          />
          <div
            className="absolute inset-2 rounded-full"
            style={{
              border: '1.5px solid transparent',
              borderBottomColor: 'var(--neon-purple)',
              animation: 'spin 1.5s linear infinite reverse',
            }}
          />
        </div>
        <span
          className="text-sm font-medium animate-neon-breathe"
          style={{ color: 'var(--text-tertiary)' }}
        >
          加载中...
        </span>
      </div>
    </div>
  )
}

// 需要登录的路由守卫
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const user = useAuthStore((s) => s.user)
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  // 强制改密拦截：有 must_change_pwd 标记时强制跳转
  if (user?.must_change_pwd && window.location.pathname !== '/force-change-password') {
    return <Navigate to="/force-change-password" replace />
  }
  return <>{children}</>
}

// 强制改密页路由包装：只要登录过就能访问（绕过强制改密拦截本身）
function ForceChangePasswordRoute() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <ForceChangePasswordPage />
}

export default function App() {
  return (
    <ToastProvider>
      <Toaster position="top-right" />
      <BrowserRouter>
        {/* 桌面端：首次启动"服务器地址"引导（仅在默认端口探活失败时出现） */}
        <DesktopServerPicker />
        {/* 桌面端事件绑定器（仅 Tauri 环境生效） */}
        <DesktopEventBinder />
        {/* 桌面端自动更新横幅 */}
        <UpdateBanner />
        {/* 顶层应用壳：桌面端标题栏 + 全屏主体 */}
        <div className="nv-app-shell">
          <TitleBar />
          <div className="nv-app-body">
            <Suspense fallback={<PageLoader />}>
              <Routes>
            {/* 公开路由 */}
            <Route path="/login" element={<LoginPage />} />
            {/* 强制改密页面（需要登录但绕过强制改密拦截） */}
            <Route path="/force-change-password" element={<ForceChangePasswordRoute />} />

            {/* 播放页面（全屏，不含布局） */}
            <Route
              path="/play/:id"
              element={
                <ProtectedRoute>
                  <PlayerPage />
                </ProtectedRoute>
              }
            />

            {/* 含侧边栏布局的路由 */}
            <Route
              path="/"
              element={
                <ProtectedRoute>
                  <Layout />
                </ProtectedRoute>
              }
            >
              <Route index element={<HomePage />} />
              <Route path="library/:id" element={<LibraryPage />} />
              <Route path="media/:id" element={<MediaDetailPage />} />
              <Route path="series/:id" element={<SeriesDetailPage />} />
              <Route path="search" element={<SearchPage />} />
              <Route path="favorites" element={<FavoritesPage />} />
              <Route path="history" element={<HistoryPage />} />
              <Route path="playlists" element={<PlaylistsPage />} />
              <Route path="admin" element={<AdminPage />} />
              <Route path="scrape" element={<Navigate to="/files?tab=scrape" replace />} />
              <Route path="files" element={<FileManagerPage />} />
              <Route path="profile" element={<ProfilePage />} />
              <Route path="stats" element={<StatsPage />} />
              <Route path="pulse" element={<PulsePage />} />
              <Route path="preprocess" element={<PreprocessPage />} />
              <Route path="subtitle-preprocess" element={<SubtitlePreprocessPage />} />
              <Route path="browse" element={<BrowsePage />} />
              <Route path="collections" element={<CollectionsPage />} />
              <Route path="collections/:id" element={<CollectionDetailPage />} />
              <Route path="person/:id" element={<PersonDetailPage />} />
            </Route>

            {/* 未匹配路由 */}
            <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </Suspense>
          </div>
        </div>
      </BrowserRouter>
    </ToastProvider>
  )
}
