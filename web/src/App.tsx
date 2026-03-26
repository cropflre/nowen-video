import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { ToastProvider } from '@/components/Toast'
import Layout from '@/components/Layout'
import LoginPage from '@/pages/LoginPage'
import HomePage from '@/pages/HomePage'
import LibraryPage from '@/pages/LibraryPage'
import MediaDetailPage from '@/pages/MediaDetailPage'
import PlayerPage from '@/pages/PlayerPage'
import SearchPage from '@/pages/SearchPage'
import FavoritesPage from '@/pages/FavoritesPage'
import HistoryPage from '@/pages/HistoryPage'
import PlaylistsPage from '@/pages/PlaylistsPage'
import AdminPage from '@/pages/AdminPage'
import SeriesDetailPage from '@/pages/SeriesDetailPage'
import ProfilePage from '@/pages/ProfilePage'
import StatsPage from '@/pages/StatsPage'
import ScrapeManagerPage from '@/pages/ScrapeManagerPage'
import FileManagerPage from '@/pages/FileManagerPage'

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
      <BrowserRouter>
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
            <Route path="scrape" element={<ScrapeManagerPage />} />
            <Route path="files" element={<FileManagerPage />} />
            <Route path="profile" element={<ProfilePage />} />
            <Route path="stats" element={<StatsPage />} />
          </Route>

          {/* 未匹配路由 */}
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </BrowserRouter>
    </ToastProvider>
  )
}
