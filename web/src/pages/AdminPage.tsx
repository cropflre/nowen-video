import { useEffect, useState, useCallback, useMemo, useRef } from 'react'
import { adminApi, libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { SystemInfo, Library, User, TranscodeJob, TMDbConfigStatus, DoubanConfigStatus, DoubanImportTokenInfo, DoubanImportTokenStatus, SystemSettings } from '@/types'
import type { ScanProgressData, ScrapeProgressData, TranscodeProgressData, ScanPhaseData } from '@/hooks/useWebSocket'
import {
  Server,
  Users,
  Zap,
  Film,
  Eye,
  EyeOff,
  Key,
  ExternalLink,
  Check,
  X,
  Loader2,
  Wifi,
  WifiOff,
  LayoutDashboard,
  FolderOpen,
  ListTodo,
  Activity,
  Search,
  ChevronRight,
  ChevronLeft,
  Settings,
  Trash2,
  Sparkles,
  HardDrive,
  Zap as ZapIcon,
  Copy as CopyIcon,
  Bookmark,
  RefreshCw,
  FileText,
} from 'lucide-react'
import clsx from 'clsx'
import LibraryManager from '@/components/LibraryManager'
import LogsTab from '@/components/admin/LogsTab'
import DashboardTab from '@/components/admin/DashboardTab'
import UsersTab from '@/components/admin/UsersTab'
import TasksTab from '@/components/admin/TasksTab'
import AITab from '@/components/admin/AITab'
import StorageTab from '@/components/admin/StorageTab'
import { useTranslation } from '@/i18n'

// ==================== 标签页定义 ====================
const TABS = [
  { id: 'dashboard', labelKey: 'admin.tabDashboard', icon: LayoutDashboard, shortLabelKey: 'admin.shortDashboard' },
  { id: 'library', labelKey: 'admin.tabLibrary', icon: FolderOpen, shortLabelKey: 'admin.shortLibrary' },
  { id: 'users', labelKey: 'admin.tabUsers', icon: Users, shortLabelKey: 'admin.shortUsers' },
  { id: 'tasks', labelKey: 'admin.tabTasks', icon: ListTodo, shortLabelKey: 'admin.shortTasks' },
  { id: 'logs', labelKey: 'admin.tabLogs', icon: FileText, shortLabelKey: 'admin.shortLogs' },
  { id: 'ai', labelKey: 'admin.tabAI', icon: Sparkles, shortLabelKey: 'admin.shortAI' },
  { id: 'storage', labelKey: 'admin.tabStorage', icon: HardDrive, shortLabelKey: 'admin.shortStorage' },
] as const

type TabId = (typeof TABS)[number]['id']

// ==================== 标签页横向滚动导航组件 ====================
function TabScrollNav({
  activeTab,
  switchTab,
  hasActiveProgress,
  transcodeJobs,
  t,
}: {
  activeTab: TabId
  switchTab: (tab: TabId) => void
  hasActiveProgress: boolean
  transcodeJobs: TranscodeJob[]
  t: (key: string) => string
}) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const [canScrollLeft, setCanScrollLeft] = useState(false)
  const [canScrollRight, setCanScrollRight] = useState(false)

  // 检测是否可以向左/右滚动
  const checkScroll = useCallback(() => {
    const el = scrollRef.current
    if (!el) return
    const { scrollLeft, scrollWidth, clientWidth } = el
    setCanScrollLeft(scrollLeft > 1)
    setCanScrollRight(scrollLeft + clientWidth < scrollWidth - 1)
  }, [])

  // 监听滚动和窗口大小变化
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    checkScroll()
    el.addEventListener('scroll', checkScroll, { passive: true })
    const resizeObserver = new ResizeObserver(checkScroll)
    resizeObserver.observe(el)
    return () => {
      el.removeEventListener('scroll', checkScroll)
      resizeObserver.disconnect()
    }
  }, [checkScroll])

  // 选中标签自动滚动到可视区域
  useEffect(() => {
    const el = scrollRef.current
    if (!el) return
    const activeButton = el.querySelector(`[data-tab-id="${activeTab}"]`) as HTMLElement
    if (!activeButton) return
    const { offsetLeft, offsetWidth } = activeButton
    const { scrollLeft, clientWidth } = el
    // 如果选中标签在左侧不可见
    if (offsetLeft < scrollLeft) {
      el.scrollTo({ left: offsetLeft - 12, behavior: 'smooth' })
    }
    // 如果选中标签在右侧不可见
    else if (offsetLeft + offsetWidth > scrollLeft + clientWidth) {
      el.scrollTo({ left: offsetLeft + offsetWidth - clientWidth + 12, behavior: 'smooth' })
    }
  }, [activeTab])

  // 滚动操作
  const scroll = (direction: 'left' | 'right') => {
    const el = scrollRef.current
    if (!el) return
    const scrollAmount = el.clientWidth * 0.6
    el.scrollBy({
      left: direction === 'left' ? -scrollAmount : scrollAmount,
      behavior: 'smooth',
    })
  }

  return (
    <div className="relative group/tabs">
      {/* 左侧滚动按钮 */}
      {canScrollLeft && (
        <button
          onClick={() => scroll('left')}
          className="absolute left-0 top-0 z-10 flex h-full w-8 items-center justify-center transition-opacity"
          style={{
            background: 'linear-gradient(to right, var(--bg-primary) 60%, transparent)',
          }}
          aria-label="向左滚动"
        >
          <ChevronLeft size={16} className="text-surface-400 hover:text-neon transition-colors" />
        </button>
      )}

      {/* 标签页容器 */}
      <div
        ref={scrollRef}
        className="flex gap-1 overflow-x-auto pb-px scrollbar-hide scroll-smooth"
        style={{
          borderBottom: '1px solid var(--border-default)',
          paddingLeft: canScrollLeft ? '24px' : undefined,
          paddingRight: canScrollRight ? '24px' : undefined,
          WebkitOverflowScrolling: 'touch', // iOS 触摸滑动优化
        }}
      >
        {TABS.map((tab) => {
          const Icon = tab.icon
          const isActive = activeTab === tab.id
          // 给「任务」标签添加活动指示器
          const hasIndicator = tab.id === 'tasks' && (hasActiveProgress || transcodeJobs.some((j) => j.status === 'running'))
          // 给「仪表盘」标签在有进度时添加指示器
          const hasDashIndicator = tab.id === 'dashboard' && hasActiveProgress

          return (
            <button
              key={tab.id}
              data-tab-id={tab.id}
              onClick={() => switchTab(tab.id)}
              className={clsx('admin-tab whitespace-nowrap', isActive && 'active')}
            >
              <Icon size={16} />
              <span className="hidden sm:inline">{t(tab.labelKey)}</span>
              <span className="sm:hidden">{t(tab.shortLabelKey)}</span>
              {(hasIndicator || hasDashIndicator) && (
                <span className="relative flex h-2 w-2">
                  <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-neon opacity-75" />
                  <span className="relative inline-flex h-2 w-2 rounded-full bg-neon" />
                </span>
              )}
            </button>
          )
        })}
      </div>

      {/* 右侧滚动按钮 */}
      {canScrollRight && (
        <button
          onClick={() => scroll('right')}
          className="absolute right-0 top-0 z-10 flex h-full w-8 items-center justify-center transition-opacity"
          style={{
            background: 'linear-gradient(to left, var(--bg-primary) 60%, transparent)',
          }}
          aria-label="向右滚动"
        >
          <ChevronRight size={16} className="text-surface-400 hover:text-neon transition-colors" />
        </button>
      )}
    </div>
  )
}

export default function AdminPage() {
  // 从 URL hash 读取初始标签
  const getInitialTab = (): TabId => {
    const hash = window.location.hash.replace('#', '')
    if (TABS.some((t) => t.id === hash)) return hash as TabId
    return 'dashboard'
  }

  const [activeTab, setActiveTab] = useState<TabId>(getInitialTab)
  const [searchQuery, setSearchQuery] = useState('')
  const { t } = useTranslation()

  const [systemInfo, setSystemInfo] = useState<SystemInfo | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [transcodeJobs, setTranscodeJobs] = useState<TranscodeJob[]>([])
  const [scanning, setScanning] = useState<Set<string>>(new Set())

  // 系统全局设置
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

  // TMDb 配置状态
  const [tmdbConfig, setTmdbConfig] = useState<TMDbConfigStatus | null>(null)
  const [tmdbKeyInput, setTmdbKeyInput] = useState('')
  const [tmdbEditing, setTmdbEditing] = useState(false)
  const [tmdbShowKey, setTmdbShowKey] = useState(false)
  const [tmdbSaving, setTmdbSaving] = useState(false)
  const [tmdbTesting, setTmdbTesting] = useState(false)
  const [tmdbMessage, setTmdbMessage] = useState<{ type: 'success' | 'error' | 'info'; text: string } | null>(null)

  // 豆瓣 Cookie 配置状态
  const [doubanConfig, setDoubanConfig] = useState<DoubanConfigStatus | null>(null)
  const [doubanCookieInput, setDoubanCookieInput] = useState('')
  const [doubanEditing, setDoubanEditing] = useState(false)
  const [doubanShowCookie, setDoubanShowCookie] = useState(false)
  const [doubanSaving, setDoubanSaving] = useState(false)
  const [doubanValidating, setDoubanValidating] = useState(false)
  const [doubanMessage, setDoubanMessage] = useState<{ type: 'success' | 'error' | 'info'; text: string } | null>(null)

  // 豆瓣 Cookie 懒人版一键导入
  const [doubanImportOpen, setDoubanImportOpen] = useState(false)
  const [doubanImportInfo, setDoubanImportInfo] = useState<DoubanImportTokenInfo | null>(null)
  const [doubanImportStatus, setDoubanImportStatus] = useState<DoubanImportTokenStatus | null>(null)
  const [doubanImportLoading, setDoubanImportLoading] = useState(false)
  const [doubanImportCopied, setDoubanImportCopied] = useState<'script' | 'bookmarklet' | null>(null)
  const [doubanImportRemaining, setDoubanImportRemaining] = useState(0)

  // WebSocket 实时进度
  const { connected, on, off } = useWebSocket()
  const [scanProgress, setScanProgress] = useState<Record<string, ScanProgressData>>({})
  const [scrapeProgress, setScrapeProgress] = useState<Record<string, ScrapeProgressData>>({})
  const [transcodeProgress, setTranscodeProgress] = useState<Record<string, TranscodeProgressData>>({})
  const [scanPhase, setScanPhase] = useState<Record<string, ScanPhaseData>>({})
  const [realtimeMessages, setRealtimeMessages] = useState<string[]>([])

  // 标签页切换 — 同步到 URL hash
  const switchTab = useCallback((tab: TabId) => {
    setActiveTab(tab)
    window.location.hash = tab
    setSearchQuery('')
  }, [])

  // 添加实时消息（保留最近20条）
  const addMessage = useCallback((msg: string) => {
    const time = new Date().toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit', second: '2-digit' })
    setRealtimeMessages((prev) => [`[${time}] ${msg}`, ...prev].slice(0, 20))
  }, [])

  // ==================== WebSocket 事件监听 ====================
  useEffect(() => {
    const handleScanStarted = (data: ScanProgressData) => {
      setScanning((s) => new Set(s).add(data.library_id))
      setScanProgress((prev) => ({ ...prev, [data.library_id]: data }))
      addMessage(`📂 ${data.message}`)
    }
    const handleScanProgress = (data: ScanProgressData) => {
      setScanProgress((prev) => ({ ...prev, [data.library_id]: data }))
    }
    const handleScanCompleted = (data: ScanProgressData) => {
      setScanProgress((prev) => {
        const next = { ...prev }
        delete next[data.library_id]
        return next
      })
      addMessage(`✅ ${data.message}`)
      libraryApi.list().then((res) => setLibraries(res.data.data || []))
    }
    const handleScanFailed = (data: ScanProgressData) => {
      setScanning((s) => {
        const ns = new Set(s)
        ns.delete(data.library_id)
        return ns
      })
      setScanProgress((prev) => {
        const next = { ...prev }
        delete next[data.library_id]
        return next
      })
      addMessage(`❌ ${data.message}`)
    }

    const handleScrapeStarted = (data: ScrapeProgressData) => {
      setScrapeProgress((prev) => ({ ...prev, [data.library_id || 'default']: data }))
      addMessage(`🎨 ${data.message}`)
    }
    const handleScrapeProgress = (data: ScrapeProgressData) => {
      setScrapeProgress((prev) => ({ ...prev, [data.library_id || 'default']: data }))
    }
    const handleScrapeCompleted = (data: ScrapeProgressData) => {
      setScrapeProgress((prev) => {
        const next = { ...prev }
        delete next[data.library_id || 'default']
        return next
      })
      setScanning((s) => {
        const ns = new Set(s)
        if (data.library_id) ns.delete(data.library_id)
        return ns
      })
      addMessage(`✨ ${data.message}`)
    }

    const handleTranscodeStarted = (data: TranscodeProgressData) => {
      setTranscodeProgress((prev) => ({ ...prev, [data.task_id]: data }))
      addMessage(`🎥 ${data.message}`)
    }
    const handleTranscodeProgress = (data: TranscodeProgressData) => {
      setTranscodeProgress((prev) => ({ ...prev, [data.task_id]: data }))
    }
    const handleTranscodeCompleted = (data: TranscodeProgressData) => {
      setTranscodeProgress((prev) => {
        const next = { ...prev }
        delete next[data.task_id]
        return next
      })
      addMessage(`✅ ${data.message}`)
    }
    const handleTranscodeFailed = (data: TranscodeProgressData) => {
      setTranscodeProgress((prev) => {
        const next = { ...prev }
        delete next[data.task_id]
        return next
      })
      addMessage(`❌ ${data.message}`)
    }

    const handleScanPhase = (data: ScanPhaseData) => {
      if (data.phase === 'completed') {
        setScanPhase((prev) => {
          const next = { ...prev }
          delete next[data.library_id]
          return next
        })
        setScanning((s) => {
          const ns = new Set(s)
          ns.delete(data.library_id)
          return ns
        })
        addMessage(`✨ ${data.message}`)
        libraryApi.list().then((res) => setLibraries(res.data.data || []))
      } else {
        setScanPhase((prev) => ({ ...prev, [data.library_id]: data }))
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
  }, [on, off, addMessage])

  // ==================== 加载数据 ====================
  useEffect(() => {
    const loadAll = async () => {
      try {
        const [sysRes, libRes, userRes, transRes, tmdbRes, doubanRes, settingsRes] = await Promise.all([
          adminApi.systemInfo(),
          libraryApi.list(),
          adminApi.listUsers(),
          adminApi.transcodeStatus(),
          adminApi.getTMDbConfig(),
          adminApi.getDoubanConfig(),
          adminApi.getSystemSettings(),
        ])
        setSystemInfo(sysRes.data.data)
        setLibraries(libRes.data.data || [])
        setUsers(userRes.data.data || [])
        setTranscodeJobs(transRes.data.data || [])
        setTmdbConfig(tmdbRes.data.data)
        setDoubanConfig(doubanRes.data.data)
        if (settingsRes.data.data) setSysSettings(settingsRes.data.data)
      } catch {
        // 静默处理
      }
    }
    loadAll()
  }, [])

  // ==================== TMDb 配置操作 ====================
  const showTmdbMessage = (type: 'success' | 'error' | 'info', text: string) => {
    setTmdbMessage({ type, text })
    setTimeout(() => setTmdbMessage(null), 5000)
  }

  const handleSaveTMDbKey = async () => {
    const key = tmdbKeyInput.trim()
    if (!key) return
    setTmdbSaving(true)
    try {
      const res = await adminApi.updateTMDbConfig(key)
      setTmdbConfig(res.data.data)
      setTmdbKeyInput('')
      setTmdbEditing(false)
      setTmdbShowKey(false)
      showTmdbMessage('success', t('admin.tmdbSaveSuccess'))
    } catch (err: any) {
      const msg = err?.response?.data?.error || t('admin.tmdbSaveFailed')
      showTmdbMessage('error', msg)
    } finally {
      setTmdbSaving(false)
    }
  }

  const handleClearTMDbKey = async () => {
    if (!confirm(t('admin.tmdbClearConfirm'))) return
    try {
      const res = await adminApi.clearTMDbConfig()
      setTmdbConfig(res.data.data)
      setTmdbKeyInput('')
      setTmdbEditing(false)
      showTmdbMessage('success', t('admin.tmdbClearSuccess'))
    } catch {
      showTmdbMessage('error', t('admin.tmdbClearFailed'))
    }
  }

  // 测试 TMDb 连接是否可用
  // - 编辑状态下：优先使用输入框中尚未保存的 key（保存前预检）
  // - 非编辑状态下：测试当前已保存的 key
  const handleTestTMDbKey = async () => {
    const inputKey = tmdbKeyInput.trim()
    const useInput = tmdbEditing && inputKey.length > 0

    if (!useInput && !tmdbConfig?.configured) {
      showTmdbMessage('error', t('admin.tmdbTestNoKey'))
      return
    }

    setTmdbTesting(true)
    showTmdbMessage('info', t('admin.tmdbTesting'))
    try {
      const res = useInput
        ? await adminApi.testTMDbKey(inputKey)
        : await adminApi.validateTMDbConfig()
      const { valid, message } = res.data.data
      showTmdbMessage(valid ? 'success' : 'error', message || (valid ? t('admin.tmdbTestOK') : t('admin.tmdbTestFailed')))
    } catch (err: any) {
      const msg = err?.response?.data?.error || t('admin.tmdbTestFailed')
      showTmdbMessage('error', msg)
    } finally {
      setTmdbTesting(false)
    }
  }

  // ==================== 豆瓣 Cookie 配置操作 ====================
  const showDoubanMessage = (type: 'success' | 'error' | 'info', text: string) => {
    setDoubanMessage({ type, text })
    setTimeout(() => setDoubanMessage(null), 5000)
  }

  const handleSaveDoubanCookie = async () => {
    const cookie = doubanCookieInput.trim()
    if (!cookie) return
    setDoubanSaving(true)
    try {
      const res = await adminApi.updateDoubanConfig(cookie)
      setDoubanConfig(res.data.data)
      setDoubanCookieInput('')
      setDoubanEditing(false)
      setDoubanShowCookie(false)
      showDoubanMessage('success', '豆瓣 Cookie 已保存')
    } catch (err: any) {
      const msg = err?.response?.data?.error || '保存失败，请稍后重试'
      showDoubanMessage('error', msg)
    } finally {
      setDoubanSaving(false)
    }
  }

  const handleClearDoubanCookie = async () => {
    if (!confirm('确定要清除豆瓣 Cookie 吗？清除后豆瓣刮削将回退到匿名模式（成功率较低）。')) return
    try {
      const res = await adminApi.clearDoubanConfig()
      setDoubanConfig(res.data.data)
      setDoubanCookieInput('')
      setDoubanEditing(false)
      showDoubanMessage('success', '豆瓣 Cookie 已清除')
    } catch {
      showDoubanMessage('error', '清除失败，请稍后重试')
    }
  }

  const handleValidateDoubanCookie = async () => {
    setDoubanValidating(true)
    try {
      const res = await adminApi.validateDoubanConfig()
      const { valid, message } = res.data.data
      showDoubanMessage(valid ? 'success' : 'error', message)
    } catch (err: any) {
      const msg = err?.response?.data?.error || '校验失败'
      showDoubanMessage('error', msg)
    } finally {
      setDoubanValidating(false)
    }
  }

  // ==================== 豆瓣 Cookie 懒人版一键导入 ====================
  const openDoubanImport = async () => {
    setDoubanImportOpen(true)
    setDoubanImportInfo(null)
    setDoubanImportStatus(null)
    setDoubanImportCopied(null)
    setDoubanImportLoading(true)
    try {
      const res = await adminApi.createDoubanImportToken()
      setDoubanImportInfo(res.data.data)
      setDoubanImportRemaining(res.data.data.expires_in)
    } catch (err: any) {
      const msg = err?.response?.data?.error || '生成导入链接失败，请稍后重试'
      showDoubanMessage('error', msg)
      setDoubanImportOpen(false)
    } finally {
      setDoubanImportLoading(false)
    }
  }

  const closeDoubanImport = () => {
    setDoubanImportOpen(false)
    setDoubanImportInfo(null)
    setDoubanImportStatus(null)
  }

  const copyDoubanImportText = async (text: string, kind: 'script' | 'bookmarklet') => {
    try {
      await navigator.clipboard.writeText(text)
      setDoubanImportCopied(kind)
      setTimeout(() => setDoubanImportCopied(null), 2000)
    } catch {
      // 回退：创建临时 textarea
      const ta = document.createElement('textarea')
      ta.value = text
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
      setDoubanImportCopied(kind)
      setTimeout(() => setDoubanImportCopied(null), 2000)
    }
  }

  // 轮询导入状态
  useEffect(() => {
    if (!doubanImportOpen || !doubanImportInfo) return
    if (doubanImportStatus?.status === 'success' || doubanImportStatus?.status === 'expired') return

    const timer = setInterval(async () => {
      try {
        const res = await adminApi.getDoubanImportTokenStatus(doubanImportInfo.token)
        setDoubanImportStatus(res.data.data)
        setDoubanImportRemaining(res.data.data.remaining_secs)
        if (res.data.data.status === 'success') {
          // 导入成功，刷新豆瓣配置状态
          const cfgRes = await adminApi.getDoubanConfig()
          setDoubanConfig(cfgRes.data.data)
          showDoubanMessage('success', res.data.data.message || '豆瓣 Cookie 已导入')
        }
      } catch {
        // 静默失败，下一轮再试
      }
    }, 2000)
    return () => clearInterval(timer)
  }, [doubanImportOpen, doubanImportInfo, doubanImportStatus?.status])

  // ==================== 搜索匹配 ====================
  // 快捷导航条目
  const quickNavItems = useMemo(() => {
    const items = [
      { label: t('admin.quickNavSystemStatus'), tab: 'dashboard' as TabId, icon: Server },
      { label: t('admin.quickNavSystemSettings'), tab: 'dashboard' as TabId, icon: Settings },
      { label: t('admin.quickNavRealtimeProgress'), tab: 'dashboard' as TabId, icon: Loader2 },
      { label: t('admin.quickNavActivityLog'), tab: 'dashboard' as TabId, icon: Activity },
      { label: t('admin.quickNavLibrary'), tab: 'library' as TabId, icon: FolderOpen },
      { label: t('admin.quickNavTMDb'), tab: 'library' as TabId, icon: Film },
      { label: t('admin.quickNavUsers'), tab: 'users' as TabId, icon: Users },
      { label: t('admin.quickNavTranscode'), tab: 'tasks' as TabId, icon: Zap },
      { label: t('admin.quickNavLogs'), tab: 'logs' as TabId, icon: FileText },
      { label: t('admin.quickNavAI'), tab: 'ai' as TabId, icon: Sparkles },
    ]
    if (!searchQuery.trim()) return []
    const q = searchQuery.toLowerCase()
    return items.filter((item) => item.label.toLowerCase().includes(q))
  }, [searchQuery, t])

  // 实时进度是否有活动
  const hasActiveProgress = Object.keys(scanProgress).length > 0 || Object.keys(scrapeProgress).length > 0 || Object.keys(transcodeProgress).length > 0

  return (
    <div className="space-y-0">
      {/* ==================== 顶部标题栏 ==================== */}
      <div className="mb-6">
        <div className="flex items-center justify-between mb-4">
          <h1 className="font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            {t('admin.title')}
          </h1>
          <div className="flex items-center gap-3">
            {/* 搜索框 */}
            <div className="relative">
              <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-surface-500" />
              <input
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
                className="input pl-9 pr-3 py-1.5 text-sm w-48 lg:w-64"
                placeholder={t('admin.searchPlaceholder')}
              />
              {/* 搜索结果下拉 */}
              {quickNavItems.length > 0 && (
                <div
                  className="absolute left-0 right-0 top-full z-50 mt-1 overflow-hidden rounded-xl py-1 animate-slide-up"
                  style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border-strong)',
                    boxShadow: 'var(--shadow-elevated)',
                  }}
                >
                  {quickNavItems.map((item) => {
                    const Icon = item.icon
                    return (
                      <button
                        key={item.label}
                        onClick={() => {
                          switchTab(item.tab)
                          setSearchQuery('')
                        }}
                        className="flex w-full items-center gap-2.5 px-4 py-2.5 text-left text-sm transition-colors hover:bg-[var(--nav-hover-bg)]"
                        style={{ color: 'var(--text-secondary)' }}
                      >
                        <Icon size={14} className="text-neon/60" />
                        <span>{item.label}</span>
                        <ChevronRight size={12} className="ml-auto text-surface-600" />
                      </button>
                    )
                  })}
                </div>
              )}
            </div>
            {/* WebSocket 状态 */}
            <div className="flex items-center gap-2 text-xs">
              {connected ? (
                <span className="flex items-center gap-1.5 text-neon">
                  <Wifi size={14} />
                  <span className="hidden sm:inline">{t('admin.connected')}</span>
                </span>
              ) : (
                <span className="flex items-center gap-1.5 text-surface-500">
                  <WifiOff size={14} />
                  <span className="hidden sm:inline">{t('admin.disconnected')}</span>
                </span>
              )}
            </div>
          </div>
        </div>

        {/* ==================== 标签页导航（支持横向滚动） ==================== */}
        <TabScrollNav
          activeTab={activeTab}
          switchTab={switchTab}
          hasActiveProgress={hasActiveProgress}
          transcodeJobs={transcodeJobs}
          t={t}
        />
      </div>

      {/* ==================== 标签页内容区 ==================== */}
      <div className="tab-content-enter" key={activeTab}>
        {/* ===== 仪表盘标签页 ===== */}
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

        {/* ===== 媒体库管理标签页 ===== */}
        {activeTab === 'library' && (
          <div className="space-y-8">
            {/* 媒体库管理器 */}
            <LibraryManager
              libraries={libraries}
              setLibraries={setLibraries}
              scanning={scanning}
              setScanning={setScanning}
              scanProgress={scanProgress}
              scrapeProgress={scrapeProgress}
              scanPhase={scanPhase}
            />

            {/* TMDb 元数据刮削配置 */}
            <section>
              <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                <Film size={20} className="text-neon/60" />
                {t('admin.metadataConfig')}
              </h2>
              <div className="glass-panel rounded-xl p-5">
                {/* 说明信息 */}
                <div className="mb-5 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                  <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                    {t('admin.metadataConfigDesc').split('TMDb')[0]}
                    <span className="font-medium text-neon">TMDb（The Movie Database）</span>
                    {t('admin.metadataConfigDesc').split('TMDb（The Movie Database）')[1] || t('admin.metadataConfigDesc').split('TMDb (The Movie Database)')[1]}
                  </p>
                  <a
                    href="https://www.themoviedb.org/settings/api"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-neon hover:text-neon-blue transition-colors"
                  >
                    <ExternalLink size={14} />
                    {t('admin.applyTMDbKey')}
                  </a>
                </div>

                {/* 当前状态 */}
                <div className="mb-4 flex items-center gap-3">
                  <div className={clsx(
                    'flex h-10 w-10 items-center justify-center rounded-lg',
                    tmdbConfig?.configured ? 'bg-green-500/10' : ''
                  )}
                    style={!tmdbConfig?.configured ? { background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' } : undefined}
                  >
                    <Key size={18} className={tmdbConfig?.configured ? 'text-green-400' : 'text-surface-500'} />
                  </div>
                  <div>
                    <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                      {tmdbConfig?.configured ? t('admin.tmdbConfigured') : t('admin.tmdbNotConfigured')}
                    </p>
                    {tmdbConfig?.configured && tmdbConfig.masked_key && (
                      <p className="mt-0.5 flex items-center gap-2 text-xs text-surface-400 font-mono">
                        {tmdbShowKey ? tmdbConfig.masked_key : '••••••••••••••••••••'}
                        <button
                          onClick={() => setTmdbShowKey(!tmdbShowKey)}
                          className="text-surface-500 hover:text-surface-300 transition-colors"
                          title={tmdbShowKey ? t('admin.tmdbHideKey') : t('admin.tmdbShowKey')}
                        >
                          {tmdbShowKey ? <EyeOff size={12} /> : <Eye size={12} />}
                        </button>
                      </p>
                    )}
                  </div>
                </div>

                {/* 操作提示消息 */}
                {tmdbMessage && (
                  <div className={clsx(
                    'mb-4 flex items-center gap-2 rounded-lg px-4 py-3 text-sm',
                    tmdbMessage.type === 'success' && 'bg-green-500/10 text-green-400',
                    tmdbMessage.type === 'error' && 'bg-red-500/10 text-red-400',
                    tmdbMessage.type === 'info' && 'bg-blue-500/10 text-blue-400'
                  )}>
                    {tmdbMessage.type === 'success' ? <Check size={16} /> : tmdbMessage.type === 'error' ? <X size={16} /> : <Loader2 size={16} className="animate-spin" />}
                    {tmdbMessage.text}
                  </div>
                )}

                {/* 编辑表单 */}
                {tmdbEditing ? (
                  <div className="space-y-3">
                    <div>
                      <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                        {t('admin.tmdbInputLabel')}
                      </label>
                      <input
                        type="text"
                        value={tmdbKeyInput}
                        onChange={(e) => setTmdbKeyInput(e.target.value)}
                        className="input font-mono"
                        placeholder={t('admin.tmdbInputPlaceholder')}
                        autoFocus
                        onKeyDown={(e) => e.key === 'Enter' && handleSaveTMDbKey()}
                      />
                      <p className="mt-1.5 text-xs text-surface-500">
                        {t('admin.tmdbInputHint')}
                      </p>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <button
                        onClick={handleSaveTMDbKey}
                        disabled={!tmdbKeyInput.trim() || tmdbSaving || tmdbTesting}
                        className="btn-primary gap-1.5 px-4 py-2 text-sm disabled:opacity-50"
                      >
                        {tmdbSaving ? (
                          <>
                            <Loader2 size={14} className="animate-spin" />
                            {t('admin.saving')}
                          </>
                        ) : (
                          <>
                            <Check size={14} />
                            {t('common.save')}
                          </>
                        )}
                      </button>
                      <button
                        onClick={handleTestTMDbKey}
                        disabled={!tmdbKeyInput.trim() || tmdbTesting || tmdbSaving}
                        className="btn-ghost gap-1.5 px-4 py-2 text-sm disabled:opacity-50"
                        title={t('admin.tmdbTestInputHint')}
                      >
                        {tmdbTesting ? (
                          <>
                            <Loader2 size={14} className="animate-spin" />
                            {t('admin.tmdbTesting')}
                          </>
                        ) : (
                          <>
                            <Wifi size={14} />
                            {t('admin.tmdbTestBtn')}
                          </>
                        )}
                      </button>
                      <button
                        onClick={() => {
                          setTmdbEditing(false)
                          setTmdbKeyInput('')
                        }}
                        className="btn-ghost px-4 py-2 text-sm"
                      >
                        {t('common.cancel')}
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex flex-wrap items-center gap-2">
                    <button
                      onClick={() => setTmdbEditing(true)}
                      className="btn-primary gap-1.5 px-4 py-2 text-sm"
                    >
                      <Key size={14} />
                      {tmdbConfig?.configured ? t('admin.modifyApiKey') : t('admin.configApiKey')}
                    </button>
                    {tmdbConfig?.configured && (
                      <button
                        onClick={handleTestTMDbKey}
                        disabled={tmdbTesting}
                        className="btn-ghost gap-1.5 px-4 py-2 text-sm disabled:opacity-50"
                        title={t('admin.tmdbTestSavedHint')}
                      >
                        {tmdbTesting ? (
                          <>
                            <Loader2 size={14} className="animate-spin" />
                            {t('admin.tmdbTesting')}
                          </>
                        ) : (
                          <>
                            <Wifi size={14} />
                            {t('admin.tmdbTestConnection')}
                          </>
                        )}
                      </button>
                    )}
                    {tmdbConfig?.configured && (
                      <button
                        onClick={handleClearTMDbKey}
                        className="btn-ghost gap-1.5 px-4 py-2 text-sm text-red-400 hover:text-red-300"
                      >
                        <Trash2 size={14} />
                        {t('admin.clearKey')}
                      </button>
                    )}
                  </div>
                )}

                {/* 功能说明 */}
                <div className="mt-5 pt-4" style={{ borderTop: '1px solid var(--border-default)' }}>
                  <p className="text-xs font-medium text-surface-400 mb-2">{t('admin.configFeatures')}</p>
                  <ul className="space-y-1.5 text-xs text-surface-500">
                    <li className="flex items-center gap-2">
                      <span className={clsx(
                        'inline-block h-1.5 w-1.5 rounded-full',
                        tmdbConfig?.configured ? 'bg-green-400' : ''
                      )}
                      style={!tmdbConfig?.configured ? { background: 'var(--text-muted)' } : undefined}
                      />
                      {t('admin.feature1')}
                    </li>
                    <li className="flex items-center gap-2">
                      <span className={clsx(
                        'inline-block h-1.5 w-1.5 rounded-full',
                        tmdbConfig?.configured ? 'bg-green-400' : ''
                      )}
                      style={!tmdbConfig?.configured ? { background: 'var(--text-muted)' } : undefined}
                      />
                      {t('admin.feature2')}
                    </li>
                    <li className="flex items-center gap-2">
                      <span className={clsx(
                        'inline-block h-1.5 w-1.5 rounded-full',
                        tmdbConfig?.configured ? 'bg-green-400' : ''
                      )}
                      style={!tmdbConfig?.configured ? { background: 'var(--text-muted)' } : undefined}
                      />
                      {t('admin.feature3')}
                    </li>
                  </ul>
                </div>
              </div>
            </section>

            {/* ===== 豆瓣 Cookie 配置卡片 ===== */}
            <section>
              <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                <Film size={20} className="text-neon/60" />
                豆瓣刮削登录配置
              </h2>
              <div className="glass-panel rounded-xl p-5">
                {/* 说明信息 */}
                <div className="mb-5 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                  <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
                    配置豆瓣登录 <span className="font-medium text-neon">Cookie</span> 后可提升豆瓣刮削成功率、降低风控概率，并获取更完整的元数据。
                    未配置时将以匿名模式访问（成功率较低）。
                  </p>
                  <p className="mt-2 text-xs text-surface-400">
                    获取方式：浏览器登录豆瓣 → F12 打开开发者工具 → Network 标签 → 刷新页面 → 任意请求的 Request Headers → 复制完整 <code className="text-neon font-mono">Cookie</code> 值
                  </p>
                  <a
                    href="https://www.douban.com/"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-neon hover:text-neon-blue transition-colors"
                  >
                    <ExternalLink size={14} />
                    打开豆瓣登录
                  </a>
                </div>

                {/* 当前状态 */}
                <div className="mb-4 flex items-center gap-3">
                  <div className={clsx(
                    'flex h-10 w-10 items-center justify-center rounded-lg',
                    doubanConfig?.configured ? 'bg-green-500/10' : ''
                  )}
                    style={!doubanConfig?.configured ? { background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' } : undefined}
                  >
                    <Key size={18} className={doubanConfig?.configured ? 'text-green-400' : 'text-surface-500'} />
                  </div>
                  <div className="min-w-0 flex-1">
                    <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                      {doubanConfig?.configured ? 'Cookie 已配置' : 'Cookie 未配置（匿名模式）'}
                    </p>
                    {doubanConfig?.configured && doubanConfig.masked_cookie && (
                      <p className="mt-0.5 flex items-center gap-2 text-xs text-surface-400 font-mono truncate">
                        {doubanShowCookie ? doubanConfig.masked_cookie : '••••••••••••••••••••'}
                        <button
                          onClick={() => setDoubanShowCookie(!doubanShowCookie)}
                          className="text-surface-500 hover:text-surface-300 transition-colors flex-shrink-0"
                          title={doubanShowCookie ? '隐藏' : '显示掩码'}
                        >
                          {doubanShowCookie ? <EyeOff size={12} /> : <Eye size={12} />}
                        </button>
                      </p>
                    )}
                  </div>
                </div>

                {/* 操作提示消息 */}
                {doubanMessage && (
                  <div className={clsx(
                    'mb-4 flex items-center gap-2 rounded-lg px-4 py-3 text-sm',
                    doubanMessage.type === 'success' && 'bg-green-500/10 text-green-400',
                    doubanMessage.type === 'error' && 'bg-red-500/10 text-red-400',
                    doubanMessage.type === 'info' && 'bg-blue-500/10 text-blue-400'
                  )}>
                    {doubanMessage.type === 'success' ? <Check size={16} /> : <X size={16} />}
                    {doubanMessage.text}
                  </div>
                )}

                {/* 编辑表单 */}
                {doubanEditing ? (
                  <div className="space-y-3">
                    <div>
                      <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                        豆瓣 Cookie 字符串
                      </label>
                      <textarea
                        value={doubanCookieInput}
                        onChange={(e) => setDoubanCookieInput(e.target.value)}
                        className="input font-mono text-xs min-h-[120px] resize-y"
                        placeholder='示例：bid=xxxxxxxxxxxx; ll="108288"; dbcl2="xxxxxxx:xxxxxxx"; ck=xxxx; ...'
                        autoFocus
                      />
                      <p className="mt-1.5 text-xs text-surface-500">
                        应当包含 <code className="text-neon font-mono">bid</code> / <code className="text-neon font-mono">dbcl2</code> 等关键字段。Cookie 有效期约 1 个月，失效后需重新获取。
                      </p>
                    </div>
                    <div className="flex items-center gap-2 flex-wrap">
                      <button
                        onClick={handleSaveDoubanCookie}
                        disabled={!doubanCookieInput.trim() || doubanSaving}
                        className="btn-primary gap-1.5 px-4 py-2 text-sm disabled:opacity-50"
                      >
                        {doubanSaving ? (
                          <>
                            <Loader2 size={14} className="animate-spin" />
                            保存中...
                          </>
                        ) : (
                          <>
                            <Check size={14} />
                            保存
                          </>
                        )}
                      </button>
                      <button
                        onClick={() => {
                          setDoubanEditing(false)
                          setDoubanCookieInput('')
                        }}
                        className="btn-ghost px-4 py-2 text-sm"
                      >
                        取消
                      </button>
                    </div>
                  </div>
                ) : (
                  <div className="flex items-center gap-2 flex-wrap">
                    <button
                      onClick={openDoubanImport}
                      className="btn-primary gap-1.5 px-4 py-2 text-sm"
                      style={{ background: 'linear-gradient(135deg, #10b981 0%, #059669 100%)' }}
                      title="无需 F12，通过浏览器书签一键导入 Cookie"
                    >
                      <ZapIcon size={14} />
                      懒人版登录
                    </button>
                    <button
                      onClick={() => setDoubanEditing(true)}
                      className="btn-ghost gap-1.5 px-4 py-2 text-sm"
                    >
                      <Key size={14} />
                      {doubanConfig?.configured ? '手动修改 Cookie' : '手动配置 Cookie'}
                    </button>
                    {doubanConfig?.configured && (
                      <>
                        <button
                          onClick={handleValidateDoubanCookie}
                          disabled={doubanValidating}
                          className="btn-ghost gap-1.5 px-4 py-2 text-sm disabled:opacity-50"
                        >
                          {doubanValidating ? (
                            <Loader2 size={14} className="animate-spin" />
                          ) : (
                            <Check size={14} />
                          )}
                          测试连接
                        </button>
                        <button
                          onClick={handleClearDoubanCookie}
                          className="btn-ghost gap-1.5 px-4 py-2 text-sm text-red-400 hover:text-red-300"
                        >
                          <Trash2 size={14} />
                          清除 Cookie
                        </button>
                      </>
                    )}
                  </div>
                )}

                {/* 安全提示 */}
                <div className="mt-5 pt-4" style={{ borderTop: '1px solid var(--border-default)' }}>
                  <p className="text-xs text-surface-500 leading-relaxed">
                    ⚠️ <span className="font-medium text-surface-400">安全提示</span>：Cookie 等同于您的豆瓣登录凭证，请妥善保管。仅供个人刮削使用，请勿分享或用于商业/公共服务。如账号被豆瓣风控，请先清除 Cookie 使用匿名模式。
                  </p>
                </div>
              </div>
            </section>
          </div>
        )}

        {/* ===== 用户管理标签页 ===== */}
        {activeTab === 'users' && (
          <UsersTab users={users} setUsers={setUsers} />
        )}

        {/* ===== 任务与转码标签页 ===== */}
        {activeTab === 'tasks' && (
          <TasksTab
            transcodeJobs={transcodeJobs}
            transcodeProgress={transcodeProgress}
          />
        )}

        {/* ===== 日志标签页 ===== */}
        {activeTab === 'logs' && (
          <LogsTab />
        )}

        {/* ===== AI 配置标签页 ===== */}
        {activeTab === 'ai' && (
          <AITab />
        )}

        {/* ===== 存储管理标签页 ===== */}
        {activeTab === 'storage' && (
          <StorageTab />
        )}
      </div>

      {/* 搜索遮罩 */}
      {searchQuery && quickNavItems.length > 0 && (
        <div className="fixed inset-0 z-40" onClick={() => setSearchQuery('')} />
      )}

      {/* ===== 豆瓣懒人版登录 模态框 ===== */}
      {doubanImportOpen && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center p-4"
          style={{ background: 'rgba(0, 0, 0, 0.6)', backdropFilter: 'blur(8px)' }}
          onClick={closeDoubanImport}
        >
          <div
            className="glass-panel w-full max-w-2xl rounded-2xl p-6 max-h-[90vh] overflow-y-auto"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="mb-5 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-lg bg-green-500/10">
                  <ZapIcon size={18} className="text-green-400" />
                </div>
                <div>
                  <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>豆瓣懒人版登录</h3>
                  <p className="text-xs text-surface-400">无需 F12，一键导入豆瓣 Cookie</p>
                </div>
              </div>
              <button onClick={closeDoubanImport} className="btn-ghost p-1.5 rounded-lg">
                <X size={16} />
              </button>
            </div>

            {doubanImportLoading && (
              <div className="flex items-center justify-center py-12">
                <Loader2 size={24} className="animate-spin text-neon" />
                <span className="ml-3 text-sm text-surface-400">正在生成一次性导入链接...</span>
              </div>
            )}

            {doubanImportInfo && !doubanImportLoading && (
              <>
                {/* 状态条 */}
                {doubanImportStatus?.status === 'success' ? (
                  <div className="mb-5 rounded-lg bg-green-500/10 border border-green-500/30 p-4">
                    <div className="flex items-center gap-2 text-green-400 font-medium text-sm">
                      <Check size={18} /> 导入成功！
                    </div>
                    <p className="mt-1.5 text-xs text-surface-300">{doubanImportStatus.message}</p>
                    <button
                      onClick={closeDoubanImport}
                      className="mt-3 btn-primary px-4 py-1.5 text-sm"
                    >
                      完成
                    </button>
                  </div>
                ) : doubanImportStatus?.status === 'expired' ? (
                  <div className="mb-5 rounded-lg bg-red-500/10 border border-red-500/30 p-4">
                    <div className="flex items-center gap-2 text-red-400 font-medium text-sm">
                      <X size={18} /> 链接已过期
                    </div>
                    <button
                      onClick={openDoubanImport}
                      className="mt-3 btn-primary gap-1.5 px-4 py-1.5 text-sm"
                    >
                      <RefreshCw size={13} />
                      重新生成
                    </button>
                  </div>
                ) : doubanImportStatus?.status === 'failed' ? (
                  <div className="mb-5 rounded-lg bg-red-500/10 border border-red-500/30 p-4">
                    <div className="flex items-center gap-2 text-red-400 font-medium text-sm">
                      <X size={18} /> 导入失败
                    </div>
                    <p className="mt-1.5 text-xs text-surface-300">{doubanImportStatus.message}</p>
                    <button
                      onClick={openDoubanImport}
                      className="mt-3 btn-primary gap-1.5 px-4 py-1.5 text-sm"
                    >
                      <RefreshCw size={13} />
                      重新生成
                    </button>
                  </div>
                ) : (
                  <div className="mb-5 rounded-lg bg-blue-500/10 border border-blue-500/30 p-4">
                    <div className="flex items-center gap-2 text-blue-400 font-medium text-sm">
                      <Loader2 size={16} className="animate-spin" />
                      等待您在豆瓣页面执行脚本...
                    </div>
                    <p className="mt-1.5 text-xs text-surface-400">
                      链接 {doubanImportRemaining > 0 ? `将在 ${Math.floor(doubanImportRemaining / 60)}分${doubanImportRemaining % 60}秒 后过期` : '即将过期'}
                    </p>
                  </div>
                )}

                {/* 使用方法 Tab 区 */}
                <div className="mb-4">
                  <h4 className="mb-3 text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    三种方式任选其一
                  </h4>

                  {/* 方式一：Bookmarklet 书签 */}
                  <div className="mb-4 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                    <div className="flex items-center gap-2 mb-2">
                      <Bookmark size={14} className="text-neon" />
                      <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>方式 1：浏览器书签（推荐）</span>
                      <span className="ml-auto text-xs px-2 py-0.5 rounded bg-green-500/10 text-green-400">最简单</span>
                    </div>
                    <ol className="text-xs text-surface-400 space-y-1 ml-5 list-decimal mb-3">
                      <li>把下面的链接 <span className="text-neon">拖动</span> 到浏览器 <span className="text-neon">书签栏</span>（或右键加入书签）</li>
                      <li>打开 <a href={doubanImportInfo.douban_url} target="_blank" rel="noopener noreferrer" className="text-neon underline">豆瓣首页</a> 并确保已登录</li>
                      <li>点击刚刚创建的书签 → 自动提交 Cookie</li>
                    </ol>
                    <div className="flex items-center gap-2">
                      <a
                        href={doubanImportInfo.bookmarklet}
                        onClick={(e) => e.preventDefault()}
                        className="btn-primary gap-1.5 px-3 py-1.5 text-xs cursor-grab active:cursor-grabbing select-none"
                        draggable
                        title="拖到书签栏即可"
                      >
                        <Bookmark size={12} />
                        📌 豆瓣一键登录
                      </a>
                      <button
                        onClick={() => copyDoubanImportText(doubanImportInfo.bookmarklet, 'bookmarklet')}
                        className="btn-ghost gap-1.5 px-3 py-1.5 text-xs"
                      >
                        {doubanImportCopied === 'bookmarklet' ? <Check size={12} className="text-green-400" /> : <CopyIcon size={12} />}
                        复制书签地址
                      </button>
                    </div>
                  </div>

                  {/* 方式二：控制台脚本 */}
                  <div className="mb-4 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                    <div className="flex items-center gap-2 mb-2">
                      <Key size={14} className="text-neon" />
                      <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>方式 2：浏览器控制台</span>
                    </div>
                    <ol className="text-xs text-surface-400 space-y-1 ml-5 list-decimal mb-3">
                      <li>打开 <a href={doubanImportInfo.douban_url} target="_blank" rel="noopener noreferrer" className="text-neon underline">豆瓣首页</a> 并登录</li>
                      <li>按 <kbd className="px-1.5 py-0.5 rounded bg-black/30 text-xs">F12</kbd> 打开开发者工具 → Console 标签</li>
                      <li>粘贴下面脚本并回车</li>
                    </ol>
                    <div className="relative">
                      <pre className="bg-black/40 rounded p-2 text-[11px] font-mono text-surface-300 max-h-32 overflow-auto whitespace-pre-wrap break-all">
                        {doubanImportInfo.script.trim()}
                      </pre>
                      <button
                        onClick={() => copyDoubanImportText(doubanImportInfo.script, 'script')}
                        className="absolute top-2 right-2 btn-ghost gap-1 px-2 py-1 text-xs"
                      >
                        {doubanImportCopied === 'script' ? <Check size={11} className="text-green-400" /> : <CopyIcon size={11} />}
                        {doubanImportCopied === 'script' ? '已复制' : '复制'}
                      </button>
                    </div>
                  </div>

                  {/* 方式三：浏览器插件（Cookie Editor） */}
                  <div className="rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                    <div className="flex items-center gap-2 mb-2">
                      <ExternalLink size={14} className="text-neon" />
                      <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>方式 3：Cookie 浏览器插件</span>
                    </div>
                    <ol className="text-xs text-surface-400 space-y-1 ml-5 list-decimal">
                      <li>安装 Chrome 扩展 <span className="text-neon">"Cookie-Editor"</span> 或 <span className="text-neon">"EditThisCookie"</span></li>
                      <li>访问豆瓣并登录 → 点开插件 → 导出 Cookie 字符串</li>
                      <li>关闭本弹窗 → 使用"手动配置 Cookie"粘贴即可</li>
                    </ol>
                  </div>
                </div>

                <div className="pt-4 border-t" style={{ borderColor: 'var(--border-default)' }}>
                  <p className="text-xs text-surface-500">
                    🔒 本链接 <span className="text-neon">仅本次有效</span>，5 分钟后自动失效。Cookie 只会保存在您的服务器上，不会发送给第三方。
                  </p>
                </div>
              </>
            )}
          </div>
        </div>
      )}
    </div>
  )
}
