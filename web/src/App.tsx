import { BrowserRouter, Routes, Route, Navigate } from 'react-router-dom'
import { lazy, Suspense, useEffect } from 'react'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'
import { ToastProvider } from '@/components/Toast'
import { DialogProvider } from '@/components/Dialog'
import { Toaster } from 'react-hot-toast'
import Layout from '@/components/Layout'
import TitleBar from '@/components/TitleBar'
import CapabilityAdminGuard from '@/components/CapabilityAdminGuard'
import LoginPage from '@/pages/LoginPage'
import ForceChangePasswordPage from '@/pages/ForceChangePasswordPage'
import { DesktopEventBinder, DesktopServerPicker, UpdateBanner } from '@/desktop'

// 懒加载页面组件 — 按需加载，减少首屏 JS 体积
const HomePage = lazy(() => import('@/pages/HomePage'))
const LibraryPage = lazy(() => import('@/pages/LibraryPage'))
const MediaDetailPage = lazy(() => import('@/pages/MediaDetailPage'))
const PlayerPage = lazy(() => import('@/pages/PlayerPage'))
const SearchPage = lazy(() => import('@/pages/SearchPage'))
const MyPage = lazy(() => import('@/pages/MyPage'))
const FavoritesPage = lazy(() => import('@/pages/FavoritesPage'))
const HistoryPage = lazy(() => import('@/pages/HistoryPage'))
const PlaylistsPage = lazy(() => import('@/pages/PlaylistsPage'))
const AdminPage = lazy(() => import('@/pages/AdminPage'))
const SeriesDetailPage = lazy(() => import('@/pages/SeriesDetailPage'))
const ProfilePage = lazy(() => import('@/pages/ProfilePage'))
const StatsPage = lazy(() => import('@/pages/StatsPage'))
const FileManagerPage = lazy(() => import('@/pages/FileManagerPage'))
const PreprocessPage = lazy(() => import('@/pages/PreprocessPage'))
const SubtitlePreprocessPage = lazy(() => import('@/pages/SubtitlePreprocessPage'))
const PreprocessLayout = lazy(() => import('@/pages/PreprocessLayout'))
const BrowsePage = lazy(() => import('@/pages/BrowsePage'))
const PersonDetailPage = lazy(() => import('@/pages/PersonDetailPage'))
const CollectionsPage = lazy(() => import('@/pages/CollectionsPage'))
const CollectionDetailPage = lazy(() => import('@/pages/CollectionDetailPage'))

function ServerProfileLoader() {
  const load = useServerProfileStore((state) => state.load)
  useEffect(() => {
    void load()
  }, [load])
  return null
}

function PageLoader() {
  return (
    <div
      className="flex min-h-[60vh] items-center justify-center px-6"
      role="status"
      aria-live="polite"
      aria-label="页面加载中"
    >
      <div className="flex flex-col items-center gap-3">
        <div className="flex h-11 w-11 items-center justify-center rounded-full border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] shadow-[var(--nv-shadow-card)]">
          <span
            className="h-6 w-6 animate-spin rounded-full border-2 border-[var(--nv-border-default)] border-t-[var(--nv-action-primary)] motion-reduce:animate-none"
            aria-hidden="true"
          />
        </div>
        <span className="text-sm font-medium text-[var(--nv-text-tertiary)]">加载中...</span>
      </div>
    </div>
  )
}

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const user = useAuthStore((s) => s.user)
  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }
  if (user?.must_change_pwd && window.location.pathname !== '/force-change-password') {
    return <Navigate to="/force-change-password" replace />
  }
  return <>{children}</>
}

function CapabilityRoute({
  capability,
  fallback,
  children,
}: {
  capability: string
  fallback: string
  children: React.ReactNode
}) {
  const loaded = useServerProfileStore((state) => state.loaded)
  const available = useServerProfileStore(
    (state) => state.manifest?.capabilities[capability]?.available === true,
  )
  if (!loaded) return <PageLoader />
  if (!available) return <Navigate to={fallback} replace />
  return <>{children}</>
}

function ForceChangePasswordRoute() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  if (!isAuthenticated) return <Navigate to="/login" replace />
  return <ForceChangePasswordPage />
}

export default function App() {
  return (
    <ToastProvider>
      <DialogProvider>
        <Toaster position="top-right" />
        <BrowserRouter>
          <ServerProfileLoader />
          <CapabilityAdminGuard />
          <DesktopServerPicker />
          <DesktopEventBinder />
          <UpdateBanner />
          <div className="nv-app-shell flex h-dvh min-h-0 flex-col overflow-hidden">
            <TitleBar />
            <div className="nv-app-body min-h-0 flex-1 overflow-hidden">
              <Suspense fallback={<PageLoader />}>
                <Routes>
                  <Route path="/login" element={<LoginPage />} />
                  <Route path="/force-change-password" element={<ForceChangePasswordRoute />} />

                  <Route
                    path="/play/:id"
                    element={
                      <ProtectedRoute>
                        <PlayerPage />
                      </ProtectedRoute>
                    }
                  />

                  <Route
                    path="/"
                    element={
                      <ProtectedRoute>
                        <Layout />
                      </ProtectedRoute>
                    }
                  >
                    <Route index element={<HomePage />} />
                    <Route path="browse" element={<BrowsePage />} />
                    <Route path="search" element={<SearchPage />} />
                    <Route path="my" element={<MyPage />} />

                    <Route path="library/:id" element={<LibraryPage />} />
                    <Route path="media/:id" element={<MediaDetailPage />} />
                    <Route path="series/:id" element={<SeriesDetailPage />} />
                    <Route path="person/:id" element={<PersonDetailPage />} />
                    <Route path="collections" element={<CollectionsPage />} />
                    <Route path="collections/:id" element={<CollectionDetailPage />} />

                    <Route path="favorites" element={<FavoritesPage />} />
                    <Route path="history" element={<HistoryPage />} />
                    <Route path="playlists" element={<PlaylistsPage />} />
                    <Route path="profile" element={<ProfilePage />} />
                    <Route path="stats" element={<StatsPage />} />

                    <Route path="admin" element={<AdminPage />} />
                    <Route path="files" element={<FileManagerPage />} />
                    <Route path="scrape" element={<Navigate to="/files?tab=scrape" replace />} />

                    <Route
                      path="preprocess"
                      element={
                        <CapabilityRoute capability="preprocess" fallback="/admin#dashboard">
                          <PreprocessLayout />
                        </CapabilityRoute>
                      }
                    >
                      <Route index element={<PreprocessPage />} />
                      <Route path="subtitle" element={<SubtitlePreprocessPage />} />
                    </Route>
                    <Route
                      path="subtitle-preprocess"
                      element={
                        <CapabilityRoute capability="preprocess" fallback="/admin#dashboard">
                          <Navigate to="/preprocess/subtitle" replace />
                        </CapabilityRoute>
                      }
                    />
                  </Route>

                  <Route path="*" element={<Navigate to="/" replace />} />
                </Routes>
              </Suspense>
            </div>
          </div>
        </BrowserRouter>
      </DialogProvider>
    </ToastProvider>
  )
}
