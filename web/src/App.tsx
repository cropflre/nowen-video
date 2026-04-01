import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense } from 'react'
import { useAuthStore } from '@/stores/auth'
import { ToastProvider } from '@/components/Toast'
import { Toaster } from 'react-hot-toast'
import Layout from '@/components/Layout'
import LoginPage from '@/pages/LoginPage'

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
const FamilyPage = lazy(() => import('@/pages/FamilyPage'))
const LivePage = lazy(() => import('@/pages/LivePage'))
const SyncPage = lazy(() => import('@/pages/SyncPage'))
const PulsePage = lazy(() => import('@/pages/PulsePage'))

// 页面加载中的占位组件
function PageLoader() {
  return (
    <div className="flex items-center justify-center min-h-[60vh]">
      <div className="flex flex-col items-center gap-3">
        <div className="w-8 h-8 border-2 border-t-transparent rounded-full animate-spin" style={{ borderColor: 'var(--neon-blue)', borderTopColor: 'transparent' }} />
        <span className="text-sm" style={{ color: 'var(--text-tertiary)' }}>加载中...</span>
      </div>
    </div>
  )
}

// 需要登录的路由守卫
function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  return <>{children}</>
}

export default function App() {
  return (
    <ToastProvider>
      <Toaster position="top-right" />
      <BrowserRouter>
        <Suspense fallback={<PageLoader />}>
          <Routes>
            {/* 公开路由 */}
            <Route path="/login" element={<LoginPage />} />

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
              <Route path="family" element={<FamilyPage />} />
              <Route path="live" element={<LivePage />} />
              <Route path="sync" element={<SyncPage />} />
              <Route path="pulse" element={<PulsePage />} />
            </Route>

            {/* 未匹配路由 */}
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </Suspense>
      </BrowserRouter>
    </ToastProvider>
  )
}
