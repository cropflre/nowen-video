import { useEffect, useState, useCallback } from 'react'
import { adultScraperApi } from '@/api'
import type { AdultScraperConfig, AdultMetadata, ParseCodeResult, PythonServiceHealth } from '@/api/adultScraper'
import {
  Shield,
  Zap,
  Globe,
  Search,
  Check,
  X,
  Loader2,
  RefreshCw,
  Save,
  Eye,
  EyeOff,
  ArrowDown,
  Clock,
  Film,
  Users,
  Tag,
  Star,
  Calendar,
  Building2,
  AlertTriangle,
  CheckCircle2,
  XCircle,
  Wifi,
  WifiOff,
  TestTube,
  Image,
  ChevronDown,
} from 'lucide-react'
import clsx from 'clsx'

// ==================== 番号刮削配置 Section ====================
// 嵌入到影视文件管理（library）Tab 中，与 TMDb / 豆瓣配置并列

export default function AdultScraperSection() {
  const [config, setConfig] = useState<AdultScraperConfig | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<{ type: 'success' | 'error' | 'info'; text: string } | null>(null)

  // 配置编辑状态
  const [editEnabled, setEditEnabled] = useState(false)
  const [editJavBus, setEditJavBus] = useState(true)
  const [editJavDB, setEditJavDB] = useState(true)
  const [editJavBusURL, setEditJavBusURL] = useState('')
  const [editJavDBURL, setEditJavDBURL] = useState('')
  const [editPythonURL, setEditPythonURL] = useState('http://localhost:5000')
  const [editPythonKey, setEditPythonKey] = useState('')
  const [editMinInterval, setEditMinInterval] = useState(1500)
  const [editMaxInterval, setEditMaxInterval] = useState(3000)
  const [showPythonKey, setShowPythonKey] = useState(false)

  // Python 微服务健康状态
  const [pythonHealth, setPythonHealth] = useState<PythonServiceHealth | null>(null)
  const [checkingHealth, setCheckingHealth] = useState(false)

  // 手动刮削测试
  const [testCode, setTestCode] = useState('')
  const [testResult, setTestResult] = useState<AdultMetadata | null>(null)
  const [testLoading, setTestLoading] = useState(false)
  const [testError, setTestError] = useState('')

  // 番号识别测试
  const [parseInput, setParseInput] = useState('')
  const [parseResult, setParseResult] = useState<ParseCodeResult | null>(null)
  const [parseLoading, setParseLoading] = useState(false)

  // 折叠面板状态
  const [showArchitecture, setShowArchitecture] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)
  const [showTest, setShowTest] = useState(false)

  // 加载配置
  const loadConfig = useCallback(async () => {
    setLoading(true)
    try {
      const res = await adultScraperApi.getConfig()
      const cfg = res.data.data
      setConfig(cfg)
      setEditEnabled(cfg.enabled)
      setEditMinInterval(cfg.min_request_interval)
      setEditMaxInterval(cfg.max_request_interval)
      cfg.sources.forEach(s => {
        if (s.id === 'javbus') {
          setEditJavBus(s.enabled)
          setEditJavBusURL(s.url === 'https://www.javbus.com' ? '' : s.url)
        }
        if (s.id === 'javdb') {
          setEditJavDB(s.enabled)
          setEditJavDBURL(s.url === 'https://javdb.com' ? '' : s.url)
        }
        if (s.id === 'python') {
          // 若后端未配置，默认填充本地地址，方便用户直接启用
          setEditPythonURL(s.url || 'http://localhost:5000')
        }
      })
    } catch {
      showMessage('error', '加载配置失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => { loadConfig() }, [loadConfig])

  const showMessage = (type: 'success' | 'error' | 'info', text: string) => {
    setMessage({ type, text })
    setTimeout(() => setMessage(null), 4000)
  }

  // 保存配置
  const handleSave = async () => {
    setSaving(true)
    try {
      await adultScraperApi.updateConfig({
        enabled: editEnabled,
        enable_javbus: editJavBus,
        javbus_url: editJavBusURL || undefined,
        enable_javdb: editJavDB,
        javdb_url: editJavDBURL || undefined,
        python_service_url: editPythonURL,
        python_service_api_key: editPythonKey,
        min_request_interval: editMinInterval,
        max_request_interval: editMaxInterval,
      })
      showMessage('success', '配置已保存')
      loadConfig()
    } catch {
      showMessage('error', '保存失败')
    } finally {
      setSaving(false)
    }
  }

  // 检查 Python 微服务健康
  const checkPythonHealth = async () => {
    setCheckingHealth(true)
    try {
      const res = await adultScraperApi.pythonServiceHealth()
      setPythonHealth(res.data.data)
    } catch {
      setPythonHealth({ configured: false, status: 'error', message: '检查失败' })
    } finally {
      setCheckingHealth(false)
    }
  }

  // 手动刮削测试
  const handleTestScrape = async () => {
    if (!testCode.trim()) return
    setTestLoading(true)
    setTestResult(null)
    setTestError('')
    try {
      const res = await adultScraperApi.scrapeByCode(testCode.trim())
      setTestResult(res.data.data)
    } catch (err: any) {
      setTestError(err?.response?.data?.error || '刮削失败')
    } finally {
      setTestLoading(false)
    }
  }

  // 番号识别测试
  const handleParseCode = async () => {
    if (!parseInput.trim()) return
    setParseLoading(true)
    setParseResult(null)
    try {
      const res = await adultScraperApi.parseCode(parseInput.trim())
      setParseResult(res.data.data)
    } catch {
      setParseResult(null)
    } finally {
      setParseLoading(false)
    }
  }

  return (
    <section>
      <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <Shield size={20} className="text-neon/60" />
        番号刮削配置
      </h2>
      <div className="glass-panel rounded-xl p-5">
        {/* 说明信息 */}
        <div className="mb-5 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
          <p className="text-sm leading-relaxed" style={{ color: 'var(--text-secondary)' }}>
            配置 <span className="font-medium text-neon">番号刮削引擎</span> 后，媒体库扫描时将自动识别成人内容番号（如 SSIS-001、FC2-PPV-1234567 等），
            并通过 <span className="font-medium text-teal-400">Go 原生爬虫</span> 和 <span className="font-medium text-yellow-400">Python 微服务</span> 混合调度获取元数据。
          </p>
        </div>

        {/* 加载状态 */}
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 size={20} className="animate-spin text-neon" />
            <span className="ml-2 text-sm text-surface-400">加载配置中...</span>
          </div>
        ) : (
          <div className="space-y-5">
            {/* 提示消息 */}
            {message && (
              <div className={clsx(
                'flex items-center gap-2 rounded-lg px-4 py-3 text-sm animate-slide-up',
                message.type === 'success' && 'bg-green-500/10 text-green-400',
                message.type === 'error' && 'bg-red-500/10 text-red-400',
                message.type === 'info' && 'bg-blue-500/10 text-blue-400'
              )}>
                {message.type === 'success' ? <Check size={16} /> : message.type === 'error' ? <X size={16} /> : <Loader2 size={16} className="animate-spin" />}
                {message.text}
              </div>
            )}

            {/* ===== 总开关 + 数据源状态概览 ===== */}
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className={clsx(
                  'flex h-10 w-10 items-center justify-center rounded-lg',
                  editEnabled ? 'bg-green-500/10' : ''
                )}
                  style={!editEnabled ? { background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' } : undefined}
                >
                  <Shield size={18} className={editEnabled ? 'text-green-400' : 'text-surface-500'} />
                </div>
                <div>
                  <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    {editEnabled ? '番号刮削已启用' : '番号刮削未启用'}
                  </p>
                  <p className="text-xs text-surface-500 mt-0.5">
                    {editEnabled
                      ? `${[editJavBus && 'JavBus', editJavDB && 'JavDB', editPythonURL && 'Python 微服务'].filter(Boolean).join(' / ')} 已激活`
                      : '开启后将自动识别番号内容并刮削元数据'}
                  </p>
                </div>
              </div>
              <button
                onClick={() => setEditEnabled(!editEnabled)}
                className="relative inline-flex h-6 w-11 items-center rounded-full transition-colors shrink-0"
                style={{
                  background: editEnabled ? 'var(--neon-blue)' : 'var(--border-default)',
                  border: '1px solid var(--border-default)',
                  boxShadow: editEnabled ? '0 0 0 2px var(--neon-blue-15)' : 'none',
                }}
                aria-label={editEnabled ? '关闭番号刮削' : '开启番号刮削'}
              >
                <span
                  className={clsx(
                    'inline-block h-4 w-4 transform rounded-full transition-transform',
                    editEnabled ? 'translate-x-6' : 'translate-x-1'
                  )}
                  style={{
                    background: '#ffffff',
                    boxShadow: '0 1px 3px rgba(0,0,0,0.3)',
                  }}
                />
              </button>
            </div>

            {/* ===== 数据源状态卡片（始终可见） ===== */}
            {config && (
              <div className="grid grid-cols-3 gap-3">
                {config.sources.map((source, idx) => (
                  <div
                    key={source.id}
                    className={clsx(
                      'rounded-lg p-3 text-center transition-all',
                      source.enabled
                        ? source.type === 'python_service'
                          ? 'bg-yellow-500/10 border border-yellow-500/30'
                          : 'bg-teal-500/10 border border-teal-500/30'
                        : 'opacity-50'
                    )}
                    style={!source.enabled ? { background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' } : undefined}
                  >
                    <div className="flex items-center justify-center gap-1.5 mb-1">
                      {source.type === 'go_native' ? (
                        <Zap size={14} className="text-teal-400" />
                      ) : (
                        <Globe size={14} className="text-yellow-400" />
                      )}
                      <span className={clsx(
                        'text-xs font-medium',
                        source.type === 'python_service' ? 'text-yellow-300' : 'text-teal-300'
                      )}>
                        {source.name}
                      </span>
                    </div>
                    <div className="text-[10px] text-surface-500">
                      第 {idx + 1} 层{idx === 2 ? ' (Fallback)' : ''}
                    </div>
                    <div className="mt-1.5 flex items-center justify-center gap-1">
                      {source.enabled ? (
                        <span className="flex items-center gap-1 text-[10px] text-green-400">
                          <span className="h-1.5 w-1.5 rounded-full bg-green-400" />
                          启用
                        </span>
                      ) : (
                        <span className="flex items-center gap-1 text-[10px] text-surface-500">
                          <span className="h-1.5 w-1.5 rounded-full bg-surface-600" />
                          禁用
                        </span>
                      )}
                    </div>
                  </div>
                ))}
              </div>
            )}

            {/* ===== 可折叠：架构总览 ===== */}
            <div style={{ borderTop: '1px solid var(--border-default)' }} className="pt-4">
              <button
                onClick={() => setShowArchitecture(!showArchitecture)}
                className="flex items-center gap-2 text-sm font-medium w-full text-left group"
                style={{ color: 'var(--text-secondary)' }}
              >
                <ChevronDown size={14} className={clsx('transition-transform text-surface-500', showArchitecture && 'rotate-180')} />
                混合调度引擎架构
                <span className="text-[10px] text-surface-500 font-normal">（点击展开）</span>
              </button>
              {showArchitecture && (
                <div className="mt-4 flex flex-col items-center gap-4">
                  {/* 入口：媒体文件 */}
                  <div className="flex items-center gap-3 rounded-lg px-4 py-2.5" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                    <Film size={18} className="text-neon" />
                    <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>媒体文件</span>
                  </div>
                  <ArrowDown size={16} className="text-surface-500" />
                  {/* 番号识别引擎 */}
                  <div className="flex items-center gap-3 rounded-lg px-4 py-2.5 bg-purple-500/10 border border-purple-500/30">
                    <Search size={18} className="text-purple-400" />
                    <span className="text-sm font-medium text-purple-300">番号识别引擎 (ParseCode)</span>
                  </div>
                  <div className="flex items-center gap-8">
                    <div className="flex flex-col items-center gap-3">
                      <div className="flex items-center gap-1.5 text-xs text-green-400">
                        <CheckCircle2 size={12} />
                        <span>识别到番号</span>
                      </div>
                      <ArrowDown size={14} className="text-green-400/60" />
                      <div className="rounded-lg px-3 py-2 bg-red-500/10 border border-red-500/30 text-center">
                        <span className="text-xs font-medium text-red-300">AdultProvider</span>
                        <span className="block text-[10px] text-red-400/60">优先级 5</span>
                      </div>
                    </div>
                    <div className="flex flex-col items-center gap-3">
                      <div className="flex items-center gap-1.5 text-xs text-surface-400">
                        <XCircle size={12} />
                        <span>非番号内容</span>
                      </div>
                      <ArrowDown size={14} className="text-surface-500/60" />
                      <div className="rounded-lg px-3 py-2 text-center" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                        <span className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>TMDb Provider</span>
                        <span className="block text-[10px] text-surface-500">优先级 10</span>
                      </div>
                    </div>
                  </div>
                  <ArrowDown size={16} className="text-surface-500" />
                  {/* 三层混合调度 */}
                  <div className="w-full max-w-lg">
                    <div className="mb-3 text-center text-xs font-medium text-surface-400">混合调度引擎 (ScrapeByCode)</div>
                    <div className="grid grid-cols-3 gap-3">
                      {config?.sources.map((source, idx) => (
                        <div
                          key={source.id}
                          className={clsx(
                            'rounded-lg p-3 text-center transition-all',
                            source.enabled
                              ? source.type === 'python_service'
                                ? 'bg-yellow-500/10 border border-yellow-500/30'
                                : 'bg-teal-500/10 border border-teal-500/30'
                              : 'opacity-40'
                          )}
                          style={!source.enabled ? { background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' } : undefined}
                        >
                          <div className="flex items-center justify-center gap-1.5 mb-1">
                            {source.type === 'go_native' ? (
                              <Zap size={14} className="text-teal-400" />
                            ) : (
                              <Globe size={14} className="text-yellow-400" />
                            )}
                            <span className={clsx(
                              'text-xs font-medium',
                              source.type === 'python_service' ? 'text-yellow-300' : 'text-teal-300'
                            )}>
                              {source.name}
                            </span>
                          </div>
                          <div className="text-[10px] text-surface-500">
                            第 {idx + 1} 层{idx === 2 ? ' (Fallback)' : ''}
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                  {/* 支持的番号格式 */}
                  <div className="w-full mt-2">
                    <div className="mb-2 text-xs font-medium text-surface-400">支持的番号格式</div>
                    <div className="grid grid-cols-2 gap-2">
                      {config?.supported_formats.map((fmt) => (
                        <div
                          key={fmt.type}
                          className="rounded-lg p-2.5 flex items-center gap-2.5"
                          style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}
                        >
                          <Tag size={12} className="text-neon flex-shrink-0" />
                          <div className="min-w-0 flex-1">
                            <p className="text-xs font-medium font-mono" style={{ color: 'var(--text-primary)' }}>{fmt.pattern}</p>
                            <p className="text-[10px] text-surface-500 truncate">{fmt.example}</p>
                          </div>
                          <span className="text-[10px] px-1.5 py-0.5 rounded bg-neon/10 text-neon whitespace-nowrap">{fmt.type}</span>
                        </div>
                      ))}
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* ===== 可折叠：高级配置（数据源详细配置） ===== */}
            <div style={{ borderTop: '1px solid var(--border-default)' }} className="pt-4">
              <button
                onClick={() => setShowAdvanced(!showAdvanced)}
                className="flex items-center gap-2 text-sm font-medium w-full text-left group"
                style={{ color: 'var(--text-secondary)' }}
              >
                <ChevronDown size={14} className={clsx('transition-transform text-surface-500', showAdvanced && 'rotate-180')} />
                数据源详细配置
                <span className="text-[10px] text-surface-500 font-normal">（镜像地址、Python 微服务、请求间隔）</span>
              </button>
              {showAdvanced && (
                <div className="mt-4 space-y-4">
                  {/* JavBus */}
                  <div className="rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-2">
                        <Zap size={14} className="text-teal-400" />
                        <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>JavBus（Go 原生爬虫）</span>
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-teal-500/10 text-teal-400">第 1 层</span>
                      </div>
                      <button
                        onClick={() => setEditJavBus(!editJavBus)}
                        className="relative inline-flex h-5 w-9 items-center rounded-full transition-colors shrink-0"
                        style={{
                          background: editJavBus ? '#14b8a6' : 'var(--border-default)',
                          border: '1px solid var(--border-default)',
                        }}
                        aria-label={editJavBus ? '关闭 JavBus' : '开启 JavBus'}
                      >
                        <span className="inline-block h-3.5 w-3.5 transform rounded-full transition-transform" style={{ background: '#ffffff', boxShadow: '0 1px 3px rgba(0,0,0,0.3)', transform: editJavBus ? 'translateX(18px)' : 'translateX(2px)' }} />
                      </button>
                    </div>
                    <div>
                      <label className="text-xs text-surface-500 mb-1 block">镜像地址（留空使用默认）</label>
                      <input
                        type="text"
                        value={editJavBusURL}
                        onChange={(e) => setEditJavBusURL(e.target.value)}
                        className="input text-sm"
                        placeholder="https://www.javbus.com"
                      />
                    </div>
                  </div>

                  {/* JavDB */}
                  <div className="rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-2">
                        <Zap size={14} className="text-teal-400" />
                        <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>JavDB（Go 原生爬虫）</span>
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-teal-500/10 text-teal-400">第 2 层</span>
                      </div>
                      <button
                        onClick={() => setEditJavDB(!editJavDB)}
                        className="relative inline-flex h-5 w-9 items-center rounded-full transition-colors shrink-0"
                        style={{
                          background: editJavDB ? '#14b8a6' : 'var(--border-default)',
                          border: '1px solid var(--border-default)',
                        }}
                        aria-label={editJavDB ? '关闭 JavDB' : '开启 JavDB'}
                      >
                        <span className="inline-block h-3.5 w-3.5 transform rounded-full transition-transform" style={{ background: '#ffffff', boxShadow: '0 1px 3px rgba(0,0,0,0.3)', transform: editJavDB ? 'translateX(18px)' : 'translateX(2px)' }} />
                      </button>
                    </div>
                    <div>
                      <label className="text-xs text-surface-500 mb-1 block">镜像地址（留空使用默认）</label>
                      <input
                        type="text"
                        value={editJavDBURL}
                        onChange={(e) => setEditJavDBURL(e.target.value)}
                        className="input text-sm"
                        placeholder="https://javdb.com"
                      />
                    </div>
                  </div>

                  {/* Python 微服务 */}
                  <div className="rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                    <div className="flex items-center justify-between mb-3">
                      <div className="flex items-center gap-2">
                        <Globe size={14} className="text-yellow-400" />
                        <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>Python 微服务（Fallback）</span>
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-yellow-500/10 text-yellow-400">第 3 层</span>
                      </div>
                      <div className="flex items-center gap-2">
                        {pythonHealth && (
                          <span className={clsx(
                            'flex items-center gap-1 text-[10px]',
                            pythonHealth.status === 'online' ? 'text-green-400' : pythonHealth.status === 'offline' ? 'text-red-400' : 'text-surface-500'
                          )}>
                            {pythonHealth.status === 'online' ? <Wifi size={10} /> : pythonHealth.status === 'offline' ? <WifiOff size={10} /> : null}
                            {pythonHealth.status === 'online' ? '在线' : pythonHealth.status === 'offline' ? '离线' : pythonHealth.status === 'not_configured' ? '未配置' : '异常'}
                          </span>
                        )}
                        <button
                          onClick={checkPythonHealth}
                          disabled={checkingHealth}
                          className="btn-ghost gap-1 px-2 py-1 text-[10px]"
                        >
                          {checkingHealth ? <Loader2 size={10} className="animate-spin" /> : <RefreshCw size={10} />}
                          检测
                        </button>
                      </div>
                    </div>
                    <div className="space-y-3">
                      <div>
                        <label className="text-xs text-surface-500 mb-1 block">微服务地址</label>
                        <input
                          type="text"
                          value={editPythonURL}
                          onChange={(e) => setEditPythonURL(e.target.value)}
                          className="input text-sm"
                          placeholder="http://localhost:5000（留空则不使用）"
                        />
                      </div>
                      <div>
                        <label className="text-xs text-surface-500 mb-1 block">API Key（可选）</label>
                        <div className="relative">
                          <input
                            type={showPythonKey ? 'text' : 'password'}
                            value={editPythonKey}
                            onChange={(e) => setEditPythonKey(e.target.value)}
                            className="input text-sm pr-10"
                            placeholder="留空则不认证"
                          />
                          <button
                            onClick={() => setShowPythonKey(!showPythonKey)}
                            className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-500 hover:text-surface-300"
                          >
                            {showPythonKey ? <EyeOff size={14} /> : <Eye size={14} />}
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>

                  {/* 请求间隔 */}
                  <div className="space-y-3">
                    <p className="text-xs font-medium text-surface-400">请求间隔（防止被封 IP）</p>
                    <div className="grid grid-cols-2 gap-4">
                      <div>
                        <label className="text-xs text-surface-500 mb-1 block">最小间隔（ms）</label>
                        <input
                          type="number"
                          value={editMinInterval}
                          onChange={(e) => setEditMinInterval(Number(e.target.value))}
                          className="input text-sm"
                          min={500}
                          max={10000}
                        />
                      </div>
                      <div>
                        <label className="text-xs text-surface-500 mb-1 block">最大间隔（ms）</label>
                        <input
                          type="number"
                          value={editMaxInterval}
                          onChange={(e) => setEditMaxInterval(Number(e.target.value))}
                          className="input text-sm"
                          min={1000}
                          max={30000}
                        />
                      </div>
                    </div>
                  </div>
                </div>
              )}
            </div>

            {/* ===== 可折叠：刮削测试 ===== */}
            <div style={{ borderTop: '1px solid var(--border-default)' }} className="pt-4">
              <button
                onClick={() => setShowTest(!showTest)}
                className="flex items-center gap-2 text-sm font-medium w-full text-left group"
                style={{ color: 'var(--text-secondary)' }}
              >
                <ChevronDown size={14} className={clsx('transition-transform text-surface-500', showTest && 'rotate-180')} />
                刮削测试工具
                <span className="text-[10px] text-surface-500 font-normal">（番号识别 & 手动刮削）</span>
              </button>
              {showTest && (
                <div className="mt-4 space-y-5">
                  {/* 番号识别测试 */}
                  <div>
                    <p className="text-xs font-medium text-surface-400 mb-2 flex items-center gap-1.5">
                      <Search size={12} />
                      番号识别测试
                    </p>
                    <p className="text-[10px] text-surface-500 mb-2">输入文件名或标题，测试番号识别引擎是否能正确提取番号</p>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={parseInput}
                        onChange={(e) => setParseInput(e.target.value)}
                        onKeyDown={(e) => e.key === 'Enter' && handleParseCode()}
                        className="input text-sm flex-1"
                        placeholder="例如：[FHD] SSIS-001 三上悠亚.mp4"
                      />
                      <button
                        onClick={handleParseCode}
                        disabled={parseLoading || !parseInput.trim()}
                        className="btn-primary gap-1.5 px-3 py-2 text-xs whitespace-nowrap"
                      >
                        {parseLoading ? <Loader2 size={12} className="animate-spin" /> : <Search size={12} />}
                        识别
                      </button>
                    </div>
                    {parseResult && (
                      <div className="mt-2 rounded-lg p-3" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                        <div className="flex items-center gap-3">
                          {parseResult.is_adult ? (
                            <CheckCircle2 size={16} className="text-green-400" />
                          ) : (
                            <XCircle size={16} className="text-surface-500" />
                          )}
                          <div>
                            {parseResult.is_adult ? (
                              <>
                                <p className="text-sm font-medium text-green-400">
                                  识别到番号：<span className="font-mono">{parseResult.code}</span>
                                </p>
                                <p className="text-[10px] text-surface-500">类型：{parseResult.code_type}</p>
                              </>
                            ) : (
                              <p className="text-sm text-surface-400">未识别到番号</p>
                            )}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>

                  {/* 手动刮削测试 */}
                  <div>
                    <p className="text-xs font-medium text-surface-400 mb-2 flex items-center gap-1.5">
                      <TestTube size={12} />
                      手动刮削测试
                    </p>
                    <p className="text-[10px] text-surface-500 mb-2">输入番号，测试混合调度引擎的刮削效果</p>
                    <div className="flex gap-2">
                      <input
                        type="text"
                        value={testCode}
                        onChange={(e) => setTestCode(e.target.value)}
                        onKeyDown={(e) => e.key === 'Enter' && handleTestScrape()}
                        className="input text-sm flex-1 font-mono"
                        placeholder="例如：SSIS-001"
                      />
                      <button
                        onClick={handleTestScrape}
                        disabled={testLoading || !testCode.trim() || !editEnabled}
                        className="btn-primary gap-1.5 px-3 py-2 text-xs whitespace-nowrap"
                      >
                        {testLoading ? <Loader2 size={12} className="animate-spin" /> : <Zap size={12} />}
                        刮削
                      </button>
                    </div>

                    {!editEnabled && (
                      <div className="mt-2 flex items-center gap-2 text-xs text-yellow-400">
                        <AlertTriangle size={12} />
                        请先启用番号刮削功能
                      </div>
                    )}

                    {testError && (
                      <div className="mt-2 rounded-lg bg-red-500/10 border border-red-500/30 p-3">
                        <div className="flex items-center gap-2 text-sm text-red-400">
                          <XCircle size={14} />
                          {testError}
                        </div>
                      </div>
                    )}

                    {testResult && (
                      <div className="mt-3 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
                        <div className="flex gap-4">
                          {testResult.cover && (
                            <div className="flex-shrink-0">
                              <div className="w-20 h-28 rounded-lg overflow-hidden bg-surface-800 flex items-center justify-center">
                                <Image size={20} className="text-surface-600" />
                              </div>
                              <p className="text-[10px] text-surface-500 mt-1 text-center">封面已获取</p>
                            </div>
                          )}
                          <div className="flex-1 min-w-0 space-y-2">
                            <div className="flex items-center gap-2">
                              <span className="font-mono text-xs px-1.5 py-0.5 rounded bg-neon/10 text-neon">{testResult.code}</span>
                              <span className="text-[10px] px-1.5 py-0.5 rounded bg-surface-700 text-surface-400">来源: {testResult.source}</span>
                            </div>
                            <h3 className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>{testResult.title}</h3>
                            <div className="grid grid-cols-2 gap-x-4 gap-y-1.5 text-xs">
                              {testResult.actresses?.length > 0 && (
                                <div className="flex items-center gap-1.5 col-span-2">
                                  <Users size={12} className="text-surface-500 flex-shrink-0" />
                                  <span className="text-surface-400 truncate">{testResult.actresses.join(', ')}</span>
                                </div>
                              )}
                              {testResult.studio && (
                                <div className="flex items-center gap-1.5">
                                  <Building2 size={12} className="text-surface-500 flex-shrink-0" />
                                  <span className="text-surface-400 truncate">{testResult.studio}</span>
                                </div>
                              )}
                              {testResult.release_date && (
                                <div className="flex items-center gap-1.5">
                                  <Calendar size={12} className="text-surface-500 flex-shrink-0" />
                                  <span className="text-surface-400">{testResult.release_date}</span>
                                </div>
                              )}
                              {testResult.duration > 0 && (
                                <div className="flex items-center gap-1.5">
                                  <Clock size={12} className="text-surface-500 flex-shrink-0" />
                                  <span className="text-surface-400">{testResult.duration} 分钟</span>
                                </div>
                              )}
                              {testResult.rating > 0 && (
                                <div className="flex items-center gap-1.5">
                                  <Star size={12} className="text-yellow-400 flex-shrink-0" />
                                  <span className="text-surface-400">{testResult.rating.toFixed(1)}</span>
                                </div>
                              )}
                            </div>
                            {testResult.genres?.length > 0 && (
                              <div className="flex flex-wrap gap-1 pt-1">
                                {testResult.genres.map((g, i) => (
                                  <span key={i} className="text-[10px] px-1.5 py-0.5 rounded bg-surface-700 text-surface-400">{g}</span>
                                ))}
                              </div>
                            )}
                          </div>
                        </div>
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>

            {/* ===== 保存按钮 ===== */}
            <div className="flex items-center gap-3 pt-3" style={{ borderTop: '1px solid var(--border-default)' }}>
              <button
                onClick={handleSave}
                disabled={saving}
                className="btn-primary gap-1.5 px-4 py-2 text-sm"
              >
                {saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
                {saving ? '保存中...' : '保存配置'}
              </button>
              <button
                onClick={loadConfig}
                className="btn-ghost gap-1.5 px-4 py-2 text-sm"
              >
                <RefreshCw size={14} />
                重新加载
              </button>
            </div>
          </div>
        )}
      </div>
    </section>
  )
}
