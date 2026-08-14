import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import clsx from 'clsx'
import {
  Activity,
  ChevronLeft,
  ChevronRight,
  Film,
  FolderOpen,
  HardDrive,
  LayoutDashboard,
  Loader2,
  Search,
  Server,
  Settings,
  Sparkles,
  Users,
  Wifi,
  WifiOff,
  Zap,
} from 'lucide-react'
import { adminApi, libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { Library, SystemInfo, SystemSettings, User } from '@/types'
import type { ScanPhaseData, ScanProgressData, ScrapeProgressData, TranscodeProgressData } from '@/hooks/useWebSocket'
import LibraryManager from '@/components/LibraryManager'
import AITab from '@/components/admin/AITab'
import DashboardTab from '@/components/admin/DashboardTab'
import MetadataSettings from '@/components/admin/MetadataSettings'
import StorageTab from '@/components/admin/StorageTab'
import UsersTab from '@/components/admin/UsersTab'
import { AdminPageHeader, AdminStatus } from '@/components/admin/AdminPrimitives'
import { Input } from '@/components/design-system'
import { useTranslation } from '@/i18n'

const TABS = [
  { id: 'dashboard', labelKey: 'admin.tabDashboard', icon: LayoutDashboard, shortLabelKey: 'admin.shortDashboard' },
  { id: 'library', labelKey: 'admin.tabLibrary', icon: FolderOpen, shortLabelKey: 'admin.shortLibrary' },
  { id: 'users', labelKey: 'admin.tabUsers', icon: Users, shortLabelKey: 'admin.shortUsers' },
  { id: 'ai', labelKey: 'admin.tabAI', icon: Sparkles, shortLabelKey: 'admin.shortAI' },
  { id: 'storage', labelKey: 'admin.tabStorage', icon: HardDrive, shortLabelKey: 'admin.shortStorage' },
] as const

type TabId = (typeof TABS)[number]['id']

type QuickNavItem = {
  label: string
  icon: typeof Server
  tab?: TabId
  href?: string
}

function resolveAdminTab(hash: string): TabId {
  const value = hash.replace(/^#/, '')
  return TABS.some((tab) => tab.id === value) ? value as TabId : 'dashboard'
}

function TabScrollNav({
  activeTab,
  switchTab,
  hasActiveProgress,
  t,
}: {
  activeTab: TabId
  switchTab: (tab: TabId) => void
  hasActiveProgress: boolean
  t: (key: string) => string
}) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)

  const checkScroll = useCallback(() => {
    const element = scrollRef.current
    if (!element) return
    const { scrollLeft, scrollWidth, clientWidth } = element
    setCanScrollLeft(scrollLeft > 1)
    setCanScrollRight(scrollLeft + clientWidth < scrollWidth - 1)
  }, [])

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return
    checkScroll()
    element.addEventListener('scroll', checkScroll, { passive: true })
    const resizeObserver = new ResizeObserver(checkScroll)
    resizeObserver.observe(element)
    return () => {
      element.removeEventListener('scroll', checkScroll)
      resizeObserver.disconnect()
    }
  }, [checkScroll])

  useEffect(() => {
    const element = scrollRef.current
    if (!element) return
    const activeButton = element.querySelector(`[data-tab-id="${activeTab}"]`) as HTMLElement | null
    if (!activeButton) return
    const { offsetLeft, offsetWidth } = activeButton
    const { scrollLeft, clientWidth } = element
    if (offsetLeft < scrollLeft) {
      element.scrollTo({ left: offsetLeft - 12, behavior: 'smooth' })
    } else if (offsetLeft + offsetWidth > scrollLeft + clientWidth) {
      element.scrollTo({ left: offsetLeft + offsetWidth - clientWidth + 12, behavior: 'smooth' })
    }
  }, [activeTab])

  const scroll = (direction: 'left' | 'right') => {
    const element = scrollRef.current
    if (!element) return
    const amount = element.clientWidth * 0.6
    element.scrollBy({ left: direction === 'left' ? -amount : amount, behavior: 'smooth' })
  }

  return (
    <div className="relative">
      {canScrollLeft && (
        <button
          type="button"
          onClick={() => scroll('left')}
          className="absolute left-0 top-0 z-10 flex h-full w-9 items-center justify-center bg-[linear-gradient(to_right,var(--nv-bg-canvas)_60%,transparent)] text-[var(--nv-text-tertiary)] transition-colors hover:text-[var(--nv-action-primary)]"
          aria-label="向左滚动"
        >
          <ChevronLeft size={16} />
        </button>
      )}

      <div
        ref={scrollRef}
        className="scrollbar-hide flex gap-1 overflow-x-auto border-b border-[var(--nv-border-subtle)] pb-px scroll-smooth"
        style={{
          paddingLeft: canScrollLeft ? '28px' : undefined,
          paddingRight: canScrollRight ? '28px' : undefined,
          WebkitOverflowScrolling: 'touch',
        }}
      >
        {TABS.map((tab) => {
          const Icon = tab.icon
          const isActive = activeTab === tab.id
          const showActivity = tab.id === 'dashboard' && hasActiveProgress
          return (
            <button
              type="button"
              key={tab.id}
              data-tab-id={tab.id}
              onClick={() => switchTab(tab.id)}
              className={clsx('nv-admin-tab whitespace-nowrap', isActive && 'is-active')}
            >
              <Icon size={16} />
              <span className="hidden sm:inline">{t(tab.labelKey)}</span>
              <span className="sm:hidden">{t(tab.shortLabelKey)}</span>
              {showActivity && <span className="h-2 w-2 rounded-full bg-[var(--nv-action-primary)]" aria-label="有活动任务" />}
            </button>
          )
        })}
      </div>

      {canScrollRight && (
        <button
          type="button"
          onClick={() => scroll('right')}
          className="absolute right-0 top-0 z-10 flex h-full w-9 items-center justify-center bg-[linear-gradient(to_left,var(--nv-bg-canvas)_60%,transparent)] text-[var(--nv-text-tertiary)] transition-colors hover:text-[var(--nv-action-primary)]"
          aria-label="向右滚动"
        >
          <ChevronRight size={16} />
        </button>
      )}
    </div>
  )
}

const SCAN_PROGRESS_STORAGE_KEY = 'nowen:scan-progress:v2'

type PersistedScanState = {
  scanningIds: string[]
  scanProgress: Record<string, ScanProgressData>
  scrapeProgress: Record<string, ScrapeProgressData>
  scanPhase: Record<string, ScanPhaseData>
  updatedAt: number
}

function emptyPersistedScanState(): PersistedScanState {
  return { scanningIds: [], scanProgress: {}, scrapeProgress: {}, scanPhase: {}, updatedAt: 0 }
}

function loadPersistedScanState(): PersistedScanState {
  if (typeof window === 'undefined') return emptyPersistedScanState()
  try {
    const raw = window.localStorage.getItem(SCAN_PROGRESS_STORAGE_KEY)
    if (!raw) return emptyPersistedScanState()
    const parsed = JSON.parse(raw) as PersistedScanState
    if (!parsed || Date.now() - (parsed.updatedAt || 0) > 24 * 60 * 60 * 1000) {
      window.localStorage.removeItem(SCAN_PROGRESS_STORAGE_KEY)
      return emptyPersistedScanState()
    }
    return {
      scanningIds: Array.isArray(parsed.scanningIds) ? parsed.scanningIds : [],
      scanProgress: parsed.scanProgress || {},
      scrapeProgress: parsed.scrapeProgress || {},
      scanPhase: parsed.scanPhase || {},
      updatedAt: parsed.updatedAt || 0,
    }
  } catch {
    return emptyPersistedScanState()
  }
}

export default function AdminPage() {
  const persistedScanStateRef = useRef(loadPersistedScanState())
  const { t } = useTranslation()

  const [activeTab, setActiveTab] = useState<TabId>(() => resolveAdminTab(window.location.hash))
  const [searchQuery, setSearchQuery] = useState('')
  const [systemInfo, setSystemInfo] = useState<SystemInfo | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [scanning, setScanning] = useState<Set<string>>(() => new Set(persistedScanStateRef.current.scanningIds))
  const [sysSettings, setSysSettings] = useState<SystemSettings>({
    enable_gpu_transcode: true,
    gpu_fallback_cpu: true,
    metadata_store_path: '',
    play_cache_path: '',
    enable_direct_link: false,
    auto_preprocess_on_scan: false,
    auto_transcode_on_play: false,
    prefer_direct_play: true,
  })

  const { connected, on, off } = useWebSocket()
  const [scanProgress, setScanProgress] = useState<Record<string, ScanProgressData>>(() => persistedScanStateRef.current.scanProgress)
  const [scrapeProgress, setScrapeProgress] = useState<Record<string, ScrapeProgressData>>(() => persistedScanStateRef.current.scrapeProgress)
  const [transcodeProgress, setTranscodeProgress] = useState<Record<string, TranscodeProgressData>>({})
  const [scanPhase, setScanPhase] = useState<Record<string, ScanPhaseData>>(() => persistedScanStateRef.current.scanPhase)
  const [realtimeMessages, setRealtimeMessages] = useState<string[]>([])

  const switchTab = useCallback((tab: TabId) => {
    setActiveTab(tab)
    setSearchQuery('')
    const nextHash = `#${tab}`
    if (window.location.hash !== nextHash) window.location.hash = tab
  }, [])

  useEffect(() => {
    const handleHashChange = () => {
      setActiveTab(resolveAdminTab(window.location.hash))
      setSearchQuery('')
    }
    window.addEventListener('hashchange', handleHashChange)
    return () => window.removeEventListener('hashchange', handleHashChange)
  }, [])

  const addMessage = useCallback((message: string) => {
    const time = new Date().toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    setRealtimeMessages((previous) => [`[${time}] ${message}`, ...previous].slice(0, 20))
  }, [])

  useEffect(() => {
    if (typeof window === 'undefined') return
    if (scanning.size === 0 && Object.keys(scanProgress).length === 0 && Object.keys(scrapeProgress).length === 0 && Object.keys(scanPhase).length === 0) {
      window.localStorage.removeItem(SCAN_PROGRESS_STORAGE_KEY)
      return
    }
    const payload: PersistedScanState = {
      scanningIds: Array.from(scanning),
      scanProgress,
      scrapeProgress,
      scanPhase,
      updatedAt: Date.now(),
    }
    window.localStorage.setItem(SCAN_PROGRESS_STORAGE_KEY, JSON.stringify(payload))
  }, [scanPhase, scanProgress, scanning, scrapeProgress])

  useEffect(() => {
    const handleScanStarted = (data: ScanProgressData) => {
      setScanning((current) => new Set(current).add(data.library_id))
      setScanProgress((previous) => ({ ...previous, [data.library_id]: data }))
      addMessage(`📂 ${data.message}`)
    }
    const handleScanProgress = (data: ScanProgressData) => setScanProgress((previous) => ({ ...previous, [data.library_id]: data }))
    const clearLibraryProgress = (libraryId: string) => {
      setScanning((current) => {
        const next = new Set(current)
        next.delete(libraryId)
        return next
      })
      setScanProgress((previous) => {
        const next = { ...previous }
        delete next[libraryId]
        return next
      })
      setScrapeProgress((previous) => {
        const next = { ...previous }
        delete next[libraryId]
        return next
      })
      setScanPhase((previous) => {
        const next = { ...previous }
        delete next[libraryId]
        return next
      })
    }
    const handleScanCompleted = (data: ScanProgressData) => {
      clearLibraryProgress(data.library_id)
      addMessage(`✅ ${data.message}`)
      void libraryApi.list().then((response) => setLibraries(response.data.data || [])).catch(() => {})
    }
    const handleScanFailed = (data: ScanProgressData) => {
      clearLibraryProgress(data.library_id)
      addMessage(`❌ ${data.message}`)
    }
    const handleScrapeStarted = (data: ScrapeProgressData) => {
      setScrapeProgress((previous) => ({ ...previous, [data.library_id || 'default']: data }))
      addMessage(`🎨 ${data.message}`)
    }
    const handleScrapeProgress = (data: ScrapeProgressData) => setScrapeProgress((previous) => ({ ...previous, [data.library_id || 'default']: data }))
    const handleScrapeCompleted = (data: ScrapeProgressData) => {
      setScrapeProgress((previous) => ({ ...previous, [data.library_id || 'default']: data }))
      addMessage(`✨ ${data.message}`)
    }
    const handleTranscodeStarted = (data: TranscodeProgressData) => {
      setTranscodeProgress((previous) => ({ ...previous, [data.task_id]: data }))
      addMessage(`🎥 ${data.message}`)
    }
    const handleTranscodeProgress = (data: TranscodeProgressData) => setTranscodeProgress((previous) => ({ ...previous, [data.task_id]: data }))
    const handleTranscodeCompleted = (data: TranscodeProgressData) => {
      setTranscodeProgress((previous) => {
        const next = { ...previous }
        delete next[data.task_id]
        return next
      })
      addMessage(`✅ ${data.message}`)
    }
    const handleTranscodeFailed = (data: TranscodeProgressData) => {
      setTranscodeProgress((previous) => {
        const next = { ...previous }
        delete next[data.task_id]
        return next
      })
      addMessage(`❌ ${data.message}`)
    }
    const handleScanPhase = (data: ScanPhaseData) => {
      if (data.phase === 'completed') {
        setScanPhase((previous) => {
          const next = { ...previous }
          delete next[data.library_id]
          return next
        })
        setScanning((current) => {
          const next = new Set(current)
          next.delete(data.library_id)
          return next
        })
        addMessage(`✨ ${data.message}`)
        void libraryApi.list().then((response) => setLibraries(response.data.data || [])).catch(() => {})
      } else {
        setScanPhase((previous) => ({ ...previous, [data.library_id]: data }))
        addMessage(`📦 ${data.message}`)
      }
    }

    on(WS_EVENTS.SCAN_STARTED, handleScanStarted)
    on(WS_EVENTS.SCAN_PROGRESS, handleScanProgress)
    on(WS_EVENTS.SCAN_COMPLETED, handleScanCompleted)
    on(WS_EVENTS.SCAN_FAILED, handleScanFailed)
    on(WS_EVENTS.SCRAPE_STARTED, handleScrapeStarted)
    on(WS_EVENTS.SCRAPE_PROGRESS, handleScrapeProgress)
    on(WS_EVENTS.SCRAPE_COMPLETED, handleScrapeCompleted)
    on(WS_EVENTS.TRANSCODE_STARTED, handleTranscodeStarted)
    on(WS_EVENTS.TRANSCODE_PROGRESS, handleTranscodeProgress)
    on(WS_EVENTS.TRANSCODE_COMPLETED, handleTranscodeCompleted)
    on(WS_EVENTS.TRANSCODE_FAILED, handleTranscodeFailed)
    on(WS_EVENTS.SCAN_PHASE, handleScanPhase)

    return () => {
      off(WS_EVENTS.SCAN_STARTED, handleScanStarted)
      off(WS_EVENTS.SCAN_PROGRESS, handleScanProgress)
      off(WS_EVENTS.SCAN_COMPLETED, handleScanCompleted)
      off(WS_EVENTS.SCAN_FAILED, handleScanFailed)
      off(WS_EVENTS.SCRAPE_STARTED, handleScrapeStarted)
      off(WS_EVENTS.SCRAPE_PROGRESS, handleScrapeProgress)
      off(WS_EVENTS.SCRAPE_COMPLETED, handleScrapeCompleted)
      off(WS_EVENTS.TRANSCODE_STARTED, handleTranscodeStarted)
      off(WS_EVENTS.TRANSCODE_PROGRESS, handleTranscodeProgress)
      off(WS_EVENTS.TRANSCODE_COMPLETED, handleTranscodeCompleted)
      off(WS_EVENTS.TRANSCODE_FAILED, handleTranscodeFailed)
      off(WS_EVENTS.SCAN_PHASE, handleScanPhase)
    }
  }, [addMessage, off, on])

  useEffect(() => {
    let active = true

    const loadAll = async () => {
      const [systemResult, libraryResult, userResult, settingsResult, scanStatusResult] = await Promise.allSettled([
        adminApi.systemInfo(),
        libraryApi.list(),
        adminApi.listUsers(),
        adminApi.getSystemSettings(),
        libraryApi.scanStatus(),
      ])
      if (!active) return

      if (systemResult.status === 'fulfilled') setSystemInfo(systemResult.value.data.data)
      if (libraryResult.status === 'fulfilled') setLibraries(libraryResult.value.data.data || [])
      if (userResult.status === 'fulfilled') setUsers(userResult.value.data.data || [])
      if (settingsResult.status === 'fulfilled' && settingsResult.value.data.data) {
        setSysSettings(settingsResult.value.data.data)
      }

      // Only reconcile persisted progress when the server status endpoint
      // actually answered. A transient scan-status failure must not erase a
      // valid persisted running state from the previous page session.
      if (scanStatusResult.status === 'fulfilled') {
        const activeScanPhases = scanStatusResult.value.data.data || []
        if (activeScanPhases.length > 0) {
          const activeIds = new Set(activeScanPhases.map((phase) => phase.library_id))
          setScanning(activeIds)
          setScanPhase(Object.fromEntries(activeScanPhases.map((phase) => [phase.library_id, phase])))
          setScanProgress((previous) => Object.fromEntries(Object.entries(previous).filter(([id]) => activeIds.has(id))))
          setScrapeProgress((previous) => Object.fromEntries(Object.entries(previous).filter(([id]) => activeIds.has(id))))
        } else {
          setScanning(new Set())
          setScanPhase({})
          setScanProgress({})
          setScrapeProgress({})
        }
      }
    }

    void loadAll()
    return () => { active = false }
  }, [])

  const quickNavItems = useMemo<QuickNavItem[]>(() => {
    if (!searchQuery.trim()) return []
    const items: QuickNavItem[] = [
      { label: t('admin.quickNavSystemStatus'), tab: 'dashboard', icon: Server },
      { label: t('admin.quickNavSystemSettings'), tab: 'dashboard', icon: Settings },
      { label: t('admin.quickNavRealtimeProgress'), tab: 'dashboard', icon: Loader2 },
      { label: t('admin.quickNavActivityLog'), tab: 'dashboard', icon: Activity },
      { label: t('admin.quickNavLibrary'), tab: 'library', icon: FolderOpen },
      { label: t('admin.quickNavTMDb'), tab: 'library', icon: Film },
      { label: t('admin.quickNavUsers'), tab: 'users', icon: Users },
      { label: t('admin.quickNavTranscode'), href: '/preprocess#transcode', icon: Zap },
      { label: t('admin.quickNavAI'), tab: 'ai', icon: Sparkles },
    ]
    const query = searchQuery.toLowerCase()
    return items.filter((item) => item.label.toLowerCase().includes(query))
  }, [searchQuery, t])

  const hasActiveProgress = Object.keys(scanProgress).length > 0
    || Object.keys(scrapeProgress).length > 0
    || Object.keys(transcodeProgress).length > 0
    || Object.keys(scanPhase).length > 0

  return (
    <div className="nv-admin-page space-y-6">
      <div className="relative">
        <AdminPageHeader
          title={t('admin.title')}
          description="管理媒体库、用户、服务状态、元数据与存储设置。"
          actions={(
            <div className="flex flex-wrap items-center justify-end gap-2">
              <div className="relative w-full min-w-0 sm:w-64">
                <Search size={14} className="pointer-events-none absolute left-3 top-1/2 z-10 -translate-y-1/2 text-[var(--nv-text-tertiary)]" />
                <Input
                  type="search"
                  value={searchQuery}
                  onChange={(event) => setSearchQuery(event.target.value)}
                  className="pl-9"
                  placeholder={t('admin.searchPlaceholder')}
                  aria-label={t('admin.searchPlaceholder')}
                />
                {quickNavItems.length > 0 && (
                  <div className="absolute left-0 right-0 top-full z-[var(--nv-z-dropdown)] mt-2 overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] py-1 shadow-[var(--nv-shadow-elevated)]">
                    {quickNavItems.map((item) => {
                      const Icon = item.icon
                      return (
                        <button
                          type="button"
                          key={`${item.label}-${item.href || item.tab}`}
                          onClick={() => {
                            if (item.href) window.location.href = item.href
                            else if (item.tab) switchTab(item.tab)
                            setSearchQuery('')
                          }}
                          className="flex w-full items-center gap-2.5 px-3 py-2.5 text-left text-sm text-[var(--nv-text-secondary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]"
                        >
                          <Icon size={14} className="text-[var(--nv-action-primary)]" />
                          <span>{item.label}</span>
                          <ChevronRight size={12} className="ml-auto text-[var(--nv-text-tertiary)]" />
                        </button>
                      )
                    })}
                  </div>
                )}
              </div>
              <AdminStatus tone={connected ? 'connected' : 'neutral'}>
                {connected ? <Wifi size={13} /> : <WifiOff size={13} />}
                {connected ? t('admin.connected') : t('admin.disconnected')}
              </AdminStatus>
            </div>
          )}
        />

        <TabScrollNav activeTab={activeTab} switchTab={switchTab} hasActiveProgress={hasActiveProgress} t={t} />
      </div>

      <div className="nv-admin-content-enter" key={activeTab}>
        {activeTab === 'dashboard' && (
          <DashboardTab
            systemInfo={systemInfo}
            sysSettings={sysSettings}
            setSysSettings={setSysSettings}
            scanProgress={scanProgress}
            scrapeProgress={scrapeProgress}
            transcodeProgress={transcodeProgress}
            scanPhase={scanPhase}
            realtimeMessages={realtimeMessages}
            switchTab={(tab) => switchTab(tab as TabId)}
          />
        )}

        {activeTab === 'library' && (
          <div className="space-y-8">
            <LibraryManager
              libraries={libraries}
              setLibraries={setLibraries}
              scanning={scanning}
              setScanning={setScanning}
              scanProgress={scanProgress}
              scrapeProgress={scrapeProgress}
              scanPhase={scanPhase}
            />
            <MetadataSettings />
          </div>
        )}

        {activeTab === 'users' && <UsersTab users={users} setUsers={setUsers} />}
        {activeTab === 'ai' && <AITab />}
        {activeTab === 'storage' && <StorageTab />}
      </div>

      {searchQuery && quickNavItems.length > 0 && <div className="fixed inset-0 z-[calc(var(--nv-z-dropdown)-1)]" onClick={() => setSearchQuery('')} />}
    </div>
  )
}
