import { useEffect, useState, useCallback } from 'react'
import { adminApi, libraryApi } from '@/api'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import type { SystemInfo, Library, User, TranscodeJob, TMDbConfigStatus, ScheduledTask } from '@/types'
import type { ScanProgressData, ScrapeProgressData, TranscodeProgressData } from '@/hooks/useWebSocket'
import {
  Server,
  Cpu,
  HardDrive,
  Users,
  FolderPlus,
  RefreshCw,
  Trash2,
  Zap,
  AlertCircle,
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
  Timer,
  Play,
  Pause,
  Plus,
  Shield,
  ScrollText,
} from 'lucide-react'
import clsx from 'clsx'
import SystemMonitor from '@/components/SystemMonitor'
import LibraryManager from '@/components/LibraryManager'

export default function AdminPage() {
  const [systemInfo, setSystemInfo] = useState<SystemInfo | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [users, setUsers] = useState<User[]>([])
  const [transcodeJobs, setTranscodeJobs] = useState<TranscodeJob[]>([])
  const [showAddLib, setShowAddLib] = useState(false)
  const [scanning, setScanning] = useState<Set<string>>(new Set())

  // TMDb 配置状态
  const [tmdbConfig, setTmdbConfig] = useState<TMDbConfigStatus | null>(null)
  const [tmdbKeyInput, setTmdbKeyInput] = useState('')
  const [tmdbEditing, setTmdbEditing] = useState(false)
  const [tmdbShowKey, setTmdbShowKey] = useState(false)
  const [tmdbSaving, setTmdbSaving] = useState(false)
  const [tmdbMessage, setTmdbMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  // WebSocket 实时进度
  const { connected, on, off } = useWebSocket()
  const [scanProgress, setScanProgress] = useState<Record<string, ScanProgressData>>({})
  const [scrapeProgress, setScrapeProgress] = useState<Record<string, ScrapeProgressData>>({})
  const [transcodeProgress, setTranscodeProgress] = useState<Record<string, TranscodeProgressData>>({})
  const [realtimeMessages, setRealtimeMessages] = useState<string[]>([])

  // 添加实时消息（保留最近10条）
  const addMessage = useCallback((msg: string) => {
    setRealtimeMessages((prev) => [msg, ...prev].slice(0, 10))
  }, [])

  // WebSocket 事件监听
  useEffect(() => {
    // 扫描事件
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
      // 刷新媒体库列表
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

    // 刮削事件
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
      // 扫描+刮削全部完成后取消动画
      setScanning((s) => {
        const ns = new Set(s)
        if (data.library_id) ns.delete(data.library_id)
        return ns
      })
      addMessage(`✨ ${data.message}`)
    }

    // 转码事件
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
    }
  }, [on, off, addMessage])

  // 加载数据
  useEffect(() => {
    const loadAll = async () => {
      try {
        const [sysRes, libRes, userRes, transRes, tmdbRes] = await Promise.all([
          adminApi.systemInfo(),
          libraryApi.list(),
          adminApi.listUsers(),
          adminApi.transcodeStatus(),
          adminApi.getTMDbConfig(),
        ])
        setSystemInfo(sysRes.data.data)
        setLibraries(libRes.data.data || [])
        setUsers(userRes.data.data || [])
        setTranscodeJobs(transRes.data.data || [])
        setTmdbConfig(tmdbRes.data.data)
      } catch {
        // 静默处理
      }
    }
    loadAll()
  }, [])

  // 删除用户
  const handleDeleteUser = async (id: string) => {
    if (!confirm('确定删除此用户？')) return
    try {
      await adminApi.deleteUser(id)
      setUsers((u) => u.filter((user) => user.id !== id))
    } catch {
      alert('删除失败')
    }
  }

  const hwAccelLabel = (hw: string) => {
    switch (hw) {
      case 'qsv': return 'Intel QSV'
      case 'vaapi': return 'VAAPI'
      case 'nvenc': return 'NVIDIA NVENC'
      case 'none': return '软件编码'
      default: return hw
    }
  }

  // ==================== TMDb 配置操作 ====================

  const showTmdbMessage = (type: 'success' | 'error', text: string) => {
    setTmdbMessage({ type, text })
    setTimeout(() => setTmdbMessage(null), 4000)
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
      showTmdbMessage('success', 'TMDb API Key 已保存成功')
    } catch (err: any) {
      const msg = err?.response?.data?.error || '保存失败，请稍后重试'
      showTmdbMessage('error', msg)
    } finally {
      setTmdbSaving(false)
    }
  }

  const handleClearTMDbKey = async () => {
    if (!confirm('确定清除 TMDb API Key？清除后元数据刮削功能将不可用。')) return

    try {
      const res = await adminApi.clearTMDbConfig()
      setTmdbConfig(res.data.data)
      setTmdbKeyInput('')
      setTmdbEditing(false)
      showTmdbMessage('success', 'TMDb API Key 已清除')
    } catch {
      showTmdbMessage('error', '清除失败，请稍后重试')
    }
  }

  return (
    <div className="space-y-8">
      <div className="flex items-center justify-between">
        <h1 className="font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>系统管理</h1>
        <div className="flex items-center gap-2 text-xs">
          {connected ? (
            <span className="flex items-center gap-1.5 text-neon">
              <Wifi size={14} />
              实时连接
            </span>
          ) : (
            <span className="flex items-center gap-1.5 text-surface-500">
              <WifiOff size={14} />
              未连接
            </span>
          )}
        </div>
      </div>

      {/* ==================== 实时进度面板 ==================== */}
      {(Object.keys(scanProgress).length > 0 || Object.keys(scrapeProgress).length > 0 || Object.keys(transcodeProgress).length > 0) && (
        <section className="animate-slide-up">
          <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <Loader2 size={20} className="animate-spin text-neon" />
            实时进度
          </h2>
          <div className="space-y-3">
            {/* 扫描进度 */}
            {Object.entries(scanProgress).map(([libId, data]) => (
              <div key={`scan-${libId}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'rgba(0,240,255,0.15)' }}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    📂 扫描: {data.library_name}
                  </span>
                  <span className="text-xs text-neon">新增 {data.new_found} 个文件</span>
                </div>
                <p className="text-xs text-surface-400">{data.message}</p>
              </div>
            ))}

            {/* 刮削进度 */}
            {Object.entries(scrapeProgress).map(([key, data]) => (
              <div key={`scrape-${key}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'rgba(138,43,226,0.15)' }}>
                  <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    🎨 元数据刮削
                  </span>
                  <span className="text-xs text-purple-400">
                    {data.current}/{data.total} (成功:{data.success} 失败:{data.failed})
                  </span>
                </div>
                <div className="mb-2 h-2 overflow-hidden rounded-full" style={{ background: 'rgba(0,240,255,0.06)' }}>
                  <div
                    className="h-full rounded-full transition-all duration-300"
                    style={{ background: 'linear-gradient(90deg, var(--neon-purple), var(--neon-pink))', width: `${data.total > 0 ? (data.current / data.total) * 100 : 0}%` }}
                  />
                </div>
                <p className="text-xs text-surface-400">{data.message}</p>
              </div>
            ))}

            {/* 转码进度 */}
            {Object.entries(transcodeProgress).map(([taskId, data]) => (
              <div key={`transcode-${taskId}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'rgba(245,158,11,0.15)' }}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    🎥 转码: {data.title} ({data.quality})
                  </span>
                  <span className="text-xs text-amber-400">
                    {data.progress.toFixed(1)}% {data.speed && `| ${data.speed}`}
                  </span>
                </div>
                <div className="h-2 overflow-hidden rounded-full" style={{ background: 'rgba(0,240,255,0.06)' }}>
                  <div
                    className="h-full rounded-full bg-amber-500 transition-all duration-300"
                    style={{ width: `${data.progress}%` }}
                  />
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* ==================== 实时消息流 ==================== */}
      {realtimeMessages.length > 0 && (
        <section>
          <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-surface-400">
            活动日志
          </h2>
          <div className="glass-panel-subtle max-h-32 overflow-y-auto rounded-lg p-3 space-y-1">
            {realtimeMessages.map((msg, i) => (
              <p key={i} className={clsx('text-xs', i === 0 ? 'text-surface-300' : 'text-surface-500')}>
                {msg}
              </p>
            ))}
          </div>
        </section>
      )}

      {/* ==================== 系统信息 ==================== */}
      {systemInfo && (
        <section>
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Server size={20} className="text-neon/60" />
            系统状态
          </h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <Cpu size={16} className="text-neon/60" />
                <span className="text-xs">CPU / 协程</span>
              </div>
              <p className="mt-2 font-display text-lg font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                {systemInfo.cpus} 核 / {systemInfo.goroutines}
              </p>
            </div>

            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <HardDrive size={16} className="text-neon/60" />
                <span className="text-xs">内存使用</span>
              </div>
              <p className="mt-2 font-display text-lg font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                {systemInfo.memory.alloc_mb} MB
              </p>
            </div>

            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <Zap size={16} className="text-neon/60" />
                <span className="text-xs">硬件加速</span>
              </div>
              <p className={clsx(
                'mt-2 text-lg font-bold',
                systemInfo.hw_accel !== 'none' ? 'text-green-400' : 'text-yellow-400'
              )}>
                {hwAccelLabel(systemInfo.hw_accel)}
              </p>
            </div>

            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <Server size={16} className="text-neon/60" />
                <span className="text-xs">版本</span>
              </div>
              <p className="mt-2 font-display text-lg font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                v{systemInfo.version}
              </p>
              <p className="text-xs text-surface-500">
                {systemInfo.go_version} / {systemInfo.os}_{systemInfo.arch}
              </p>
            </div>
          </div>
        </section>
      )}

      {/* ==================== 媒体库管理（飞牛风格） ==================== */}
      <LibraryManager
        libraries={libraries}
        setLibraries={setLibraries}
        scanning={scanning}
        setScanning={setScanning}
        scanProgress={scanProgress}
        scrapeProgress={scrapeProgress}
      />

      {/* ==================== 转码任务 ==================== */}
      {transcodeJobs.length > 0 && (
        <section>
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Zap size={20} className="text-neon/60" />
            转码任务
          </h2>
          <div className="space-y-2">
            {transcodeJobs.map((job) => (
              <div
                key={job.id}
                className="glass-panel-subtle flex items-center justify-between rounded-xl p-3"
              >
                <div className="text-sm" style={{ color: 'var(--text-primary)' }}>
                  <span className="text-surface-400">媒体ID:</span> {job.media_id.slice(0, 8)}...
                  <span className="ml-3 text-surface-400">质量:</span> {job.quality}
                </div>
                <div className="flex items-center gap-3">
                  <div className="h-2 w-32 overflow-hidden rounded-full" style={{ background: 'rgba(0,240,255,0.06)' }}>
                    <div
                      className="h-full rounded-full transition-all"
                      style={{ width: `${job.progress}%`, background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))' }}
                    />
                  </div>
                  <span
                    className={clsx(
                      'text-xs font-medium',
                      job.status === 'running' && 'text-neon',
                      job.status === 'pending' && 'text-yellow-400',
                      job.status === 'done' && 'text-green-400',
                      job.status === 'failed' && 'text-red-400'
                    )}
                  >
                    {job.status}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* ==================== TMDb 元数据刮削配置 ==================== */}
      <section>
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Film size={20} className="text-neon/60" />
            元数据刮削配置
        </h2>

        <div className="glass-panel rounded-xl p-5">
          {/* 说明信息 */}
          <div className="mb-5 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
            <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
              通过配置{' '}
              <span className="font-medium text-neon">TMDb（The Movie Database）</span>{' '}
              API 密钥，系统将自动获取视频的海报、简介、评分、类型等元数据信息，让您的媒体库内容更加丰富完整。
            </p>
            <a
              href="https://www.themoviedb.org/settings/api"
              target="_blank"
              rel="noopener noreferrer"
              className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-neon hover:text-neon-blue transition-colors"
            >
              <ExternalLink size={14} />
              前往 TMDb 官网免费申请 API Key
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
                {tmdbConfig?.configured ? 'API Key 已配置' : 'API Key 未配置'}
              </p>
              {tmdbConfig?.configured && tmdbConfig.masked_key && (
                <p className="mt-0.5 flex items-center gap-2 text-xs text-surface-400 font-mono">
                  {tmdbShowKey ? tmdbConfig.masked_key : '••••••••••••••••••••'}
                  <button
                    onClick={() => setTmdbShowKey(!tmdbShowKey)}
                    className="text-surface-500 hover:text-surface-300 transition-colors"
                    title={tmdbShowKey ? '隐藏密钥' : '显示掩码密钥'}
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
              tmdbMessage.type === 'error' && 'bg-red-500/10 text-red-400'
            )}>
              {tmdbMessage.type === 'success' ? <Check size={16} /> : <X size={16} />}
              {tmdbMessage.text}
            </div>
          )}

          {/* 编辑表单 */}
          {tmdbEditing ? (
            <div className="space-y-3">
              <div>
                <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                  输入 TMDb API Key
                </label>
                <input
                  type="text"
                  value={tmdbKeyInput}
                  onChange={(e) => setTmdbKeyInput(e.target.value)}
                  className="input font-mono"
                  placeholder="例如: a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6"
                  autoFocus
                  onKeyDown={(e) => e.key === 'Enter' && handleSaveTMDbKey()}
                />
                <p className="mt-1.5 text-xs text-surface-500">
                  TMDb API Key 通常是一个32位的十六进制字符串，请从{' '}
                  <a
                    href="https://www.themoviedb.org/settings/api"
                    target="_blank"
                    rel="noopener noreferrer"
                    className="text-neon hover:underline"
                  >
                    TMDb 账户设置页
                  </a>
                  {' '}中获取。
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  onClick={handleSaveTMDbKey}
                  disabled={!tmdbKeyInput.trim() || tmdbSaving}
                  className="btn-primary gap-1.5 px-4 py-2 text-sm disabled:opacity-50"
                >
                  {tmdbSaving ? (
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
                    setTmdbEditing(false)
                    setTmdbKeyInput('')
                  }}
                  className="btn-ghost px-4 py-2 text-sm"
                >
                  取消
                </button>
              </div>
            </div>
          ) : (
            <div className="flex items-center gap-2">
              <button
                onClick={() => setTmdbEditing(true)}
                className="btn-primary gap-1.5 px-4 py-2 text-sm"
              >
                <Key size={14} />
                {tmdbConfig?.configured ? '修改 API Key' : '配置 API Key'}
              </button>
              {tmdbConfig?.configured && (
                <button
                  onClick={handleClearTMDbKey}
                  className="btn-ghost gap-1.5 px-4 py-2 text-sm text-red-400 hover:text-red-300"
                >
                  <Trash2 size={14} />
                  清除密钥
                </button>
              )}
            </div>
          )}

          {/* 功能说明 */}
          <div className="mt-5 pt-4" style={{ borderTop: '1px solid var(--border-default)' }}>
            <p className="text-xs font-medium text-surface-400 mb-2">配置后可使用以下功能：</p>
            <ul className="space-y-1.5 text-xs text-surface-500">
              <li className="flex items-center gap-2">
                <span className={clsx(
                  'inline-block h-1.5 w-1.5 rounded-full',
                  tmdbConfig?.configured ? 'bg-green-400' : 'bg-surface-600'
                )} />
                扫描媒体库时自动获取海报、简介、评分等信息
              </li>
              <li className="flex items-center gap-2">
                <span className={clsx(
                  'inline-block h-1.5 w-1.5 rounded-full',
                  tmdbConfig?.configured ? 'bg-green-400' : 'bg-surface-600'
                )} />
                在媒体详情页手动刮削指定视频的元数据
              </li>
              <li className="flex items-center gap-2">
                <span className={clsx(
                  'inline-block h-1.5 w-1.5 rounded-full',
                  tmdbConfig?.configured ? 'bg-green-400' : 'bg-surface-600'
                )} />
                自动匹配电影/剧集的中文类型标签
              </li>
            </ul>
          </div>
        </div>
      </section>

      {/* ==================== 用户管理 ==================== */}
      <section>
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Users size={20} className="text-neon/60" />
            用户管理
        </h2>
        <div className="space-y-2">
          {users.map((user) => (
            <div
              key={user.id}
              className="glass-panel-subtle flex items-center justify-between rounded-xl p-4"
            >
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-full text-sm font-bold" style={{ background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))', boxShadow: 'var(--shadow-neon)', color: 'var(--text-on-neon)' }}>
                  {user.username.charAt(0).toUpperCase()}
                </div>
                <div>
                  <p className="font-medium" style={{ color: 'var(--text-primary)' }}>{user.username}</p>
                  <p className="text-xs text-surface-500">
                    {user.role === 'admin' ? '管理员' : '普通用户'}
                    <span className="ml-2">
                      注册于 {new Date(user.created_at).toLocaleDateString('zh-CN')}
                    </span>
                  </p>
                </div>
              </div>
              {user.role !== 'admin' && (
                <button
                  onClick={() => handleDeleteUser(user.id)}
                  className="btn-ghost p-2 text-red-400 hover:text-red-300"
                  title="删除用户"
                >
                  <Trash2 size={16} />
                </button>
              )}
            </div>
          ))}
        </div>

        <div className="mt-3 flex items-start gap-2 rounded-xl p-3 text-xs text-yellow-400/80" style={{ background: 'rgba(234, 179, 8, 0.03)', border: '1px solid rgba(234, 179, 8, 0.08)' }}>
          <AlertCircle size={14} className="mt-0.5 flex-shrink-0" />
          <span>新用户可以通过登录页面的"创建账号"自行注册。第一个注册的用户将自动成为管理员。</span>
        </div>
      </section>
    </div>
  )
}
