import { useState, useEffect, useCallback } from 'react'
import { aiApi } from '@/api'
import { useToast } from '@/components/Toast'
import type { AIStatus, AIErrorLog, AICacheStats, AITestResult } from '@/types'
import {
  Sparkles,
  Power,
  Key,
  Eye,
  EyeOff,
  Server,
  Cpu,
  Zap,
  Search,
  MessageSquare,
  Database as DatabaseIcon,
  Trash2,
  Play,
  Loader2,
  Check,
  X,
  AlertTriangle,
  Clock,
  BarChart3,
  Settings,
  Shield,
  RefreshCw,
  ChevronDown,
  ChevronUp,
  Wifi,
  WifiOff,
} from 'lucide-react'
import clsx from 'clsx'

// ==================== 提供商预设 ====================
const PROVIDERS = [
  {
    id: 'openai',
    name: 'OpenAI',
    apiBase: 'https://api.openai.com/v1',
    models: ['gpt-4o-mini', 'gpt-4o', 'gpt-4-turbo', 'gpt-3.5-turbo'],
    color: 'text-green-400',
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    apiBase: 'https://api.deepseek.com/v1',
    models: ['deepseek-chat', 'deepseek-reasoner'],
    color: 'text-blue-400',
  },
  {
    id: 'qwen',
    name: '通义千问',
    apiBase: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    models: ['qwen-turbo', 'qwen-plus', 'qwen-max', 'qwen-long'],
    color: 'text-purple-400',
  },
  {
    id: 'ollama',
    name: 'Ollama (本地)',
    apiBase: 'http://localhost:11434/v1',
    models: ['llama3', 'qwen2', 'gemma2', 'mistral'],
    color: 'text-orange-400',
  },
  {
    id: 'custom',
    name: '自定义',
    apiBase: '',
    models: [],
    color: 'text-surface-400',
  },
]

// ==================== 测试用例预设 ====================
const SEARCH_TEST_CASES = [
  '帮我找一部2023年的科幻电影',
  '有没有评分8分以上的日本动画',
  '最近有什么好看的悬疑剧',
  '诺兰导演的电影',
]

const RECOMMEND_TEST_CASES = [
  { title: '星际穿越', genres: '科幻,冒险,剧情' },
  { title: '你的名字', genres: '动画,爱情,奇幻' },
  { title: '肖申克的救赎', genres: '剧情,犯罪' },
]

export default function AITab() {
  const toast = useToast()

  // ==================== 状态 ====================
  const [status, setStatus] = useState<AIStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

  // 配置编辑
  const [editEnabled, setEditEnabled] = useState(false)
  const [editProvider, setEditProvider] = useState('openai')
  const [editApiBase, setEditApiBase] = useState('')
  const [editApiKey, setEditApiKey] = useState('')
  const [editModel, setEditModel] = useState('')
  const [editTimeout, setEditTimeout] = useState(30)
  const [editSmartSearch, setEditSmartSearch] = useState(true)
  const [editRecommendReason, setEditRecommendReason] = useState(true)
  const [editMetadataEnhance, setEditMetadataEnhance] = useState(true)
  const [editMonthlyBudget, setEditMonthlyBudget] = useState(0)
  const [editCacheTTL, setEditCacheTTL] = useState(168)
  const [editMaxConcurrent, setEditMaxConcurrent] = useState(3)
  const [editRequestInterval, setEditRequestInterval] = useState(200)

  const [showApiKey, setShowApiKey] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)

  // 连接测试
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<AITestResult | null>(null)

  // 缓存
  const [cacheStats, setCacheStats] = useState<AICacheStats | null>(null)
  const [clearingCache, setClearingCache] = useState(false)

  // 错误日志
  const [errorLogs, setErrorLogs] = useState<AIErrorLog[]>([])
  const [showErrors, setShowErrors] = useState(false)

  // 功能测试
  const [testingSearch, setTestingSearch] = useState(false)
  const [searchTestQuery, setSearchTestQuery] = useState('')
  const [searchTestResult, setSearchTestResult] = useState<AITestResult | null>(null)
  const [testingRecommend, setTestingRecommend] = useState(false)
  const [recommendTestTitle, setRecommendTestTitle] = useState('')
  const [recommendTestGenres, setRecommendTestGenres] = useState('')
  const [recommendTestResult, setRecommendTestResult] = useState<AITestResult | null>(null)

  // ==================== 数据加载 ====================
  const fetchStatus = useCallback(async () => {
    try {
      const res = await aiApi.getStatus()
      const s = res.data.data
      setStatus(s)
      // 同步到编辑状态
      setEditEnabled(s.enabled)
      setEditProvider(s.provider)
      setEditApiBase(s.api_base || '')
      setEditModel(s.model)
      setEditTimeout(s.timeout || 30)
      setEditSmartSearch(s.enable_smart_search)
      setEditRecommendReason(s.enable_recommend_reason)
      setEditMetadataEnhance(s.enable_metadata_enhance)
      setEditMonthlyBudget(s.monthly_budget)
      setEditCacheTTL(s.cache_ttl_hours || 168)
      setEditMaxConcurrent(s.max_concurrent || 3)
      setEditRequestInterval(s.request_interval_ms || 200)
    } catch {
      // 静默
    }
  }, [])

  const fetchCacheStats = useCallback(async () => {
    try {
      const res = await aiApi.getCacheStats()
      setCacheStats(res.data.data)
    } catch {
      // 静默
    }
  }, [])

  const fetchErrorLogs = useCallback(async () => {
    try {
      const res = await aiApi.getErrorLogs()
      setErrorLogs(res.data.data || [])
    } catch {
      // 静默
    }
  }, [])

  useEffect(() => {
    const load = async () => {
      setLoading(true)
      await Promise.all([fetchStatus(), fetchCacheStats(), fetchErrorLogs()])
      setLoading(false)
    }
    load()
  }, [fetchStatus, fetchCacheStats, fetchErrorLogs])

  // ==================== 配置保存 ====================
  const handleSaveConfig = async () => {
    setSaving(true)
    try {
      const updates: Record<string, unknown> = {
        enabled: editEnabled,
        provider: editProvider,
        api_base: editApiBase,
        model: editModel,
        timeout: editTimeout,
        enable_smart_search: editSmartSearch,
        enable_recommend_reason: editRecommendReason,
        enable_metadata_enhance: editMetadataEnhance,
        monthly_budget: editMonthlyBudget,
        cache_ttl_hours: editCacheTTL,
        max_concurrent: editMaxConcurrent,
        request_interval_ms: editRequestInterval,
      }
      // 只在用户输入了新密钥时才更新
      if (editApiKey) {
        updates.api_key = editApiKey
      }

      await aiApi.updateConfig(updates)
      toast.success('AI 配置已保存')
      setEditApiKey('') // 清空密钥输入
      await fetchStatus()
    } catch {
      toast.error('保存配置失败')
    } finally {
      setSaving(false)
    }
  }

  // ==================== 连接测试 ====================
  const handleTestConnection = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      const res = await aiApi.testConnection()
      setTestResult(res.data.data)
      if (res.data.data.success) {
        toast.success(`连接成功 (${res.data.data.latency_ms}ms)`)
      } else {
        toast.error(`连接失败: ${res.data.data.error}`)
      }
    } catch {
      toast.error('连接测试请求失败')
    } finally {
      setTesting(false)
    }
  }

  // ==================== 缓存管理 ====================
  const handleClearCache = async () => {
    if (!confirm('确定清空所有 AI 缓存？清空后下次请求将重新调用 AI API。')) return
    setClearingCache(true)
    try {
      const res = await aiApi.clearCache()
      toast.success(`已清空 ${res.data.data.cleared} 条缓存`)
      await fetchCacheStats()
    } catch {
      toast.error('清空缓存失败')
    } finally {
      setClearingCache(false)
    }
  }

  // ==================== 功能测试 ====================
  const handleTestSearch = async (query?: string) => {
    const q = query || searchTestQuery
    if (!q.trim()) return
    setTestingSearch(true)
    setSearchTestResult(null)
    if (query) setSearchTestQuery(query)
    try {
      const res = await aiApi.testSmartSearch(q)
      setSearchTestResult(res.data.data)
    } catch {
      toast.error('搜索测试失败')
    } finally {
      setTestingSearch(false)
    }
  }

  const handleTestRecommend = async (title?: string, genres?: string) => {
    const t = title || recommendTestTitle
    const g = genres || recommendTestGenres
    if (!t.trim()) return
    setTestingRecommend(true)
    setRecommendTestResult(null)
    if (title) {
      setRecommendTestTitle(title)
      setRecommendTestGenres(genres || '')
    }
    try {
      const res = await aiApi.testRecommendReason(t, g)
      setRecommendTestResult(res.data.data)
    } catch {
      toast.error('推荐理由测试失败')
    } finally {
      setTestingRecommend(false)
    }
  }

  // ==================== 提供商切换 ====================
  const handleProviderChange = (providerId: string) => {
    setEditProvider(providerId)
    const provider = PROVIDERS.find((p) => p.id === providerId)
    if (provider && provider.apiBase) {
      setEditApiBase(provider.apiBase)
      if (provider.models.length > 0) {
        setEditModel(provider.models[0])
      }
    }
  }

  // 当前提供商的可用模型
  const currentProvider = PROVIDERS.find((p) => p.id === editProvider)
  const availableModels = currentProvider?.models || []

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 size={28} className="animate-spin text-neon" />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      {/* ==================== 状态概览卡片 ==================== */}
      <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
        {/* 服务状态 */}
        <div className="glass-panel rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            {status?.enabled && status?.api_configured ? (
              <Wifi size={16} className="text-green-400" />
            ) : (
              <WifiOff size={16} className="text-surface-500" />
            )}
            <span className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
              服务状态
            </span>
          </div>
          <p
            className={clsx(
              'text-sm font-semibold',
              status?.enabled && status?.api_configured ? 'text-green-400' : 'text-surface-500'
            )}
          >
            {status?.enabled ? (status?.api_configured ? '已启用' : '未配置密钥') : '未启用'}
          </p>
        </div>

        {/* 本月调用 */}
        <div className="glass-panel rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            <BarChart3 size={16} className="text-neon/60" />
            <span className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
              本月调用
            </span>
          </div>
          <p className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
            {status?.monthly_calls || 0}
            {status?.monthly_budget ? (
              <span className="text-xs font-normal text-surface-500"> / {status.monthly_budget}</span>
            ) : (
              <span className="text-xs font-normal text-surface-500"> 次</span>
            )}
          </p>
        </div>

        {/* Token 消耗 */}
        <div className="glass-panel rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            <Cpu size={16} className="text-purple-400/60" />
            <span className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
              Token 消耗
            </span>
          </div>
          <p className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
            {((status?.total_tokens || 0) / 1000).toFixed(1)}K
          </p>
        </div>

        {/* 缓存条目 */}
        <div className="glass-panel rounded-xl p-4">
          <div className="flex items-center gap-2 mb-2">
            <DatabaseIcon size={16} className="text-cyan-400/60" />
            <span className="text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
              缓存条目
            </span>
          </div>
          <p className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
            {status?.cache_entries || 0}
          </p>
        </div>
      </div>

      {/* ==================== 配置管理 ==================== */}
      <section>
        <h2
          className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide"
          style={{ color: 'var(--text-primary)' }}
        >
          <Settings size={20} className="text-neon/60" />
          AI 服务配置
        </h2>

        <div className="glass-panel rounded-xl p-5 space-y-5">
          {/* 总开关 */}
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <div
                className={clsx(
                  'flex h-10 w-10 items-center justify-center rounded-lg'
                )}
                style={{ background: editEnabled ? 'var(--neon-blue-10)' : 'var(--nav-hover-bg)' }}
              >
                <Power size={18} className={editEnabled ? 'text-neon' : 'text-surface-500'} />
              </div>
              <div>
                <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                  启用 AI 功能
                </p>
                <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  全局开关，关闭后所有 AI 功能将停用
                </p>
              </div>
            </div>
            <button
              onClick={() => setEditEnabled(!editEnabled)}
              className="toggle-switch toggle-switch-lg"
              role="switch"
              aria-checked={editEnabled}
              aria-label="启用 AI 功能"
            >
              <span className="toggle-switch-thumb" />
            </button>
          </div>

          {/* 提供商选择 */}
          <div>
            <label className="mb-2 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              LLM 提供商
            </label>
            <div className="grid grid-cols-2 gap-2 sm:grid-cols-5">
              {PROVIDERS.map((p) => (
                <button
                  key={p.id}
                  onClick={() => handleProviderChange(p.id)}
                  className={clsx(
                    'rounded-lg px-3 py-2.5 text-sm font-medium transition-all text-center',
                    editProvider === p.id
                      ? 'ring-2'
                      : 'hover:bg-white/[0.03]'
                  )}
                  style={{
                    background: editProvider === p.id ? 'var(--neon-blue-10)' : 'var(--bg-surface)',
                    border: editProvider === p.id ? undefined : '1px solid var(--border-subtle)',
                    color: editProvider === p.id ? 'var(--neon-blue)' : 'var(--text-secondary)',
                    ...(editProvider === p.id ? { boxShadow: '0 0 0 2px var(--neon-blue)' } : {}),
                  }}
                >
                  {p.name}
                </button>
              ))}
            </div>
          </div>

          {/* API Base */}
          <div>
            <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              API 地址
            </label>
            <input
              type="text"
              value={editApiBase}
              onChange={(e) => setEditApiBase(e.target.value)}
              className="input font-mono text-sm"
              placeholder="https://api.openai.com/v1"
            />
          </div>

          {/* API Key */}
          <div>
            <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              API 密钥
            </label>
            <div className="relative">
              <input
                type={showApiKey ? 'text' : 'password'}
                value={editApiKey}
                onChange={(e) => setEditApiKey(e.target.value)}
                className="input pr-10 font-mono text-sm"
                placeholder={status?.api_configured ? '已配置（输入新值可覆盖）' : '请输入 API Key'}
              />
              <button
                onClick={() => setShowApiKey(!showApiKey)}
                className="absolute right-3 top-1/2 -translate-y-1/2 text-surface-500 hover:text-surface-300 transition-colors"
              >
                {showApiKey ? <EyeOff size={16} /> : <Eye size={16} />}
              </button>
            </div>
            {status?.api_configured && (
              <p className="mt-1 flex items-center gap-1 text-xs text-green-400">
                <Check size={12} />
                密钥已配置
              </p>
            )}
          </div>

          {/* 模型选择 */}
          <div>
            <label className="mb-1.5 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              模型
            </label>

            {/* 手动输入框 — 始终显示 */}
            <div className="relative">
              <input
                type="text"
                value={editModel}
                onChange={(e) => setEditModel(e.target.value)}
                className="input font-mono text-sm"
                placeholder={availableModels.length > 0 ? '手动输入模型名称，或从下方列表选择' : '输入模型名称，如 gpt-4o-mini'}
              />
              {/* 当输入值匹配预置模型时，显示匹配标记 */}
              {editModel && availableModels.includes(editModel) && (
                <span
                  className="absolute right-3 top-1/2 -translate-y-1/2 flex items-center gap-1 text-[11px] font-medium"
                  style={{ color: 'var(--neon-blue)' }}
                >
                  <Check size={12} />
                  预置模型
                </span>
              )}
              {/* 当输入值不匹配预置模型且有预置列表时，显示自定义标记 */}
              {editModel && availableModels.length > 0 && !availableModels.includes(editModel) && (
                <span
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-[11px] font-medium"
                  style={{ color: 'var(--text-muted)' }}
                >
                  自定义模型
                </span>
              )}
            </div>

            {/* 预置模型快捷选择列表 — 有预置模型时显示 */}
            {availableModels.length > 0 && (
              <div className="mt-2 flex flex-wrap gap-2">
                {availableModels.map((m) => (
                  <button
                    key={m}
                    onClick={() => setEditModel(m)}
                    className={clsx(
                      'rounded-lg px-3 py-1.5 text-xs font-mono transition-all',
                      editModel === m
                        ? 'ring-1'
                        : 'text-surface-400 hover:text-surface-300'
                    )}
                    style={
                      editModel === m
                        ? { background: 'var(--neon-blue-10)', color: 'var(--neon-blue)', boxShadow: '0 0 0 1px var(--neon-blue)' }
                        : { background: 'var(--bg-surface)', border: '1px solid var(--border-subtle)' }
                    }
                  >
                    {m}
                  </button>
                ))}
              </div>
            )}
          </div>

          {/* 功能开关 */}
          <div>
            <label className="mb-2 block text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
              功能开关
            </label>
            <div className="space-y-3">
              {[
                {
                  key: 'search',
                  label: '智能搜索',
                  desc: '自然语言 → 结构化查询参数',
                  icon: Search,
                  value: editSmartSearch,
                  setter: setEditSmartSearch,
                },
                {
                  key: 'recommend',
                  label: '推荐理由',
                  desc: 'AI 生成个性化推荐文案',
                  icon: MessageSquare,
                  value: editRecommendReason,
                  setter: setEditRecommendReason,
                },
                {
                  key: 'metadata',
                  label: '元数据增强',
                  desc: '当传统数据源失败时用 AI 补充',
                  icon: Sparkles,
                  value: editMetadataEnhance,
                  setter: setEditMetadataEnhance,
                },
              ].map((item) => {
                const Icon = item.icon
                return (
                  <div
                    key={item.key}
                    className="flex items-center justify-between rounded-lg p-3"
                    style={{ background: 'var(--nav-hover-bg)' }}
                  >
                    <div className="flex items-center gap-3">
                      <Icon size={16} className="text-neon/50" />
                      <div>
                        <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                          {item.label}
                        </p>
                        <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                          {item.desc}
                        </p>
                      </div>
                    </div>
                    <button
                      onClick={() => item.setter(!item.value)}
                      className="toggle-switch toggle-switch-sm"
                      role="switch"
                      aria-checked={item.value}
                      aria-label={item.label}
                    >
                      <span className="toggle-switch-thumb" />
                    </button>
                  </div>
                )
              })}
            </div>
          </div>

          {/* 高级设置折叠 */}
          <div>
            <button
              onClick={() => setShowAdvanced(!showAdvanced)}
              className="flex items-center gap-2 text-sm font-medium transition-colors"
              style={{ color: 'var(--text-secondary)' }}
            >
              {showAdvanced ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
              高级设置
            </button>

            {showAdvanced && (
              <div className="mt-3 space-y-4 rounded-lg p-4" style={{ background: 'var(--nav-hover-bg)' }}>
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                  <div>
                    <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
                      请求超时（秒）
                    </label>
                    <input
                      type="number"
                      value={editTimeout}
                      onChange={(e) => setEditTimeout(Number(e.target.value))}
                      className="input text-sm"
                      min={5}
                      max={120}
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
                      月度预算上限（0=不限）
                    </label>
                    <input
                      type="number"
                      value={editMonthlyBudget}
                      onChange={(e) => setEditMonthlyBudget(Number(e.target.value))}
                      className="input text-sm"
                      min={0}
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
                      缓存时长（小时）
                    </label>
                    <input
                      type="number"
                      value={editCacheTTL}
                      onChange={(e) => setEditCacheTTL(Number(e.target.value))}
                      className="input text-sm"
                      min={0}
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
                      最大并发数
                    </label>
                    <input
                      type="number"
                      value={editMaxConcurrent}
                      onChange={(e) => setEditMaxConcurrent(Number(e.target.value))}
                      className="input text-sm"
                      min={1}
                      max={10}
                    />
                  </div>
                  <div>
                    <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-muted)' }}>
                      请求间隔（毫秒）
                    </label>
                    <input
                      type="number"
                      value={editRequestInterval}
                      onChange={(e) => setEditRequestInterval(Number(e.target.value))}
                      className="input text-sm"
                      min={0}
                    />
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* 保存按钮 */}
          <div className="flex items-center gap-3 pt-2" style={{ borderTop: '1px solid var(--border-default)' }}>
            <button
              onClick={handleSaveConfig}
              disabled={saving}
              className="btn-primary gap-1.5 px-5 py-2 text-sm"
            >
              {saving ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
              {saving ? '保存中...' : '保存配置'}
            </button>
            <button
              onClick={handleTestConnection}
              disabled={testing || !status?.api_configured}
              className="btn-ghost gap-1.5 px-4 py-2 text-sm"
            >
              {testing ? <Loader2 size={14} className="animate-spin" /> : <Zap size={14} />}
              {testing ? '测试中...' : '测试连接'}
            </button>
          </div>

          {/* 连接测试结果 */}
          {testResult && (
            <div
              className={clsx(
                'flex items-start gap-3 rounded-lg px-4 py-3 text-sm',
                testResult.success ? 'bg-green-500/10' : 'bg-red-500/10'
              )}
            >
              {testResult.success ? (
                <Check size={16} className="mt-0.5 text-green-400 flex-shrink-0" />
              ) : (
                <X size={16} className="mt-0.5 text-red-400 flex-shrink-0" />
              )}
              <div>
                <p className={testResult.success ? 'text-green-400' : 'text-red-400'}>
                  {testResult.success ? '连接成功' : '连接失败'}
                </p>
                <p className="mt-1 text-xs text-surface-400">
                  {testResult.success
                    ? `响应时间: ${testResult.latency_ms}ms · 提供商: ${testResult.provider} · 模型: ${testResult.model}`
                    : testResult.error}
                </p>
              </div>
            </div>
          )}
        </div>
      </section>

      {/* ==================== 使用统计 ==================== */}
      <section>
        <h2
          className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide"
          style={{ color: 'var(--text-primary)' }}
        >
          <BarChart3 size={20} className="text-neon/60" />
          使用统计
        </h2>

        <div className="glass-panel rounded-xl p-5">
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div className="text-center">
              <p className="text-2xl font-bold text-neon">{status?.monthly_calls || 0}</p>
              <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                本月请求次数
              </p>
              {status?.monthly_budget ? (
                <div className="mt-2">
                  <div className="h-1.5 w-full rounded-full" style={{ background: 'var(--progress-track-bg)' }}>
                    <div
                      className="h-1.5 rounded-full transition-all"
                      style={{
                        background: 'var(--neon-blue)',
                        width: `${Math.min(100, ((status.monthly_calls || 0) / status.monthly_budget) * 100)}%`,
                      }}
                    />
                  </div>
                  <p className="mt-1 text-[10px] text-surface-500">
                    {Math.round(((status.monthly_calls || 0) / status.monthly_budget) * 100)}% 已用
                  </p>
                </div>
              ) : null}
            </div>
            <div className="text-center">
              <p className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
                {((status?.total_prompt_tokens || 0) / 1000).toFixed(1)}K
              </p>
              <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                输入 Token
              </p>
            </div>
            <div className="text-center">
              <p className="text-2xl font-bold" style={{ color: 'var(--text-primary)' }}>
                {((status?.total_completion_tokens || 0) / 1000).toFixed(1)}K
              </p>
              <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                输出 Token
              </p>
            </div>
            <div className="text-center">
              <p className="text-2xl font-bold text-purple-400">
                ${(((status?.total_tokens || 0) / 1000000) * 0.15).toFixed(4)}
              </p>
              <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
                费用估算 (gpt-4o-mini)
              </p>
            </div>
          </div>

          {/* 配额预警 */}
          {status?.monthly_budget && status.monthly_calls >= status.monthly_budget * 0.8 ? (
            <div className="mt-4 flex items-center gap-2 rounded-lg bg-yellow-500/10 px-4 py-2.5 text-sm text-yellow-400">
              <AlertTriangle size={16} />
              {status.monthly_calls >= status.monthly_budget
                ? '月度配额已用尽，AI 功能暂停'
                : `月度配额已使用 ${Math.round((status.monthly_calls / status.monthly_budget) * 100)}%，请注意用量`}
            </div>
          ) : null}
        </div>
      </section>

      {/* ==================== 缓存管理 ==================== */}
      <section>
        <h2
          className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide"
          style={{ color: 'var(--text-primary)' }}
        >
          <DatabaseIcon size={20} className="text-neon/60" />
          缓存管理
        </h2>

        <div className="glass-panel rounded-xl p-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-6">
              <div>
                <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                  {cacheStats?.active_entries || 0} 条有效缓存
                </p>
                <p className="text-xs" style={{ color: 'var(--text-muted)' }}>
                  共 {cacheStats?.total_entries || 0} 条 · {cacheStats?.expired_entries || 0} 条已过期 · TTL{' '}
                  {cacheStats?.ttl_hours || 0}h
                </p>
              </div>
            </div>
            <div className="flex items-center gap-2">
              <button
                onClick={() => fetchCacheStats()}
                className="btn-ghost gap-1 px-3 py-1.5 text-xs"
              >
                <RefreshCw size={12} />
                刷新
              </button>
              <button
                onClick={handleClearCache}
                disabled={clearingCache || !cacheStats?.total_entries}
                className="btn-ghost gap-1 px-3 py-1.5 text-xs text-red-400 hover:text-red-300 disabled:opacity-50"
              >
                {clearingCache ? <Loader2 size={12} className="animate-spin" /> : <Trash2 size={12} />}
                清空缓存
              </button>
            </div>
          </div>
        </div>
      </section>

      {/* ==================== 功能测试 ==================== */}
      <section>
        <h2
          className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide"
          style={{ color: 'var(--text-primary)' }}
        >
          <Play size={20} className="text-neon/60" />
          功能测试
        </h2>

        <div className="space-y-4">
          {/* 智能搜索测试 */}
          <div className="glass-panel rounded-xl p-5">
            <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              <Search size={16} className="text-neon/60" />
              智能搜索测试
            </h3>

            <div className="flex gap-2">
              <input
                type="text"
                value={searchTestQuery}
                onChange={(e) => setSearchTestQuery(e.target.value)}
                className="input flex-1 text-sm"
                placeholder="输入自然语言查询..."
                onKeyDown={(e) => e.key === 'Enter' && handleTestSearch()}
              />
              <button
                onClick={() => handleTestSearch()}
                disabled={testingSearch || !searchTestQuery.trim()}
                className="btn-primary gap-1.5 px-4 py-2 text-sm whitespace-nowrap"
              >
                {testingSearch ? <Loader2 size={14} className="animate-spin" /> : <Play size={14} />}
                测试
              </button>
            </div>

            {/* 预设用例 */}
            <div className="mt-2 flex flex-wrap gap-1.5">
              {SEARCH_TEST_CASES.map((tc) => (
                <button
                  key={tc}
                  onClick={() => handleTestSearch(tc)}
                  disabled={testingSearch}
                  className="rounded-md px-2 py-1 text-[11px] transition-colors hover:bg-white/[0.05]"
                  style={{ background: 'var(--bg-surface)', color: 'var(--text-muted)', border: '1px solid var(--border-subtle)' }}
                >
                  {tc}
                </button>
              ))}
            </div>

            {/* 搜索测试结果 */}
            {searchTestResult && (
              <div className="mt-3 rounded-lg p-3" style={{ background: 'var(--nav-hover-bg)' }}>
                <div className="flex items-center gap-2 mb-2">
                  {searchTestResult.success ? (
                    <Check size={14} className="text-green-400" />
                  ) : (
                    <X size={14} className="text-red-400" />
                  )}
                  <span className="text-xs text-surface-400">
                    {searchTestResult.latency_ms}ms
                  </span>
                </div>
                {searchTestResult.intent && (
                  <pre className="text-xs font-mono text-surface-300 overflow-x-auto">
                    {JSON.stringify(searchTestResult.intent, null, 2)}
                  </pre>
                )}
                {searchTestResult.error && (
                  <p className="text-xs text-red-400">{searchTestResult.error}</p>
                )}
              </div>
            )}
          </div>

          {/* 推荐理由测试 */}
          <div className="glass-panel rounded-xl p-5">
            <h3 className="mb-3 flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
              <MessageSquare size={16} className="text-neon/60" />
              推荐理由测试
            </h3>

            <div className="flex gap-2">
              <input
                type="text"
                value={recommendTestTitle}
                onChange={(e) => setRecommendTestTitle(e.target.value)}
                className="input flex-1 text-sm"
                placeholder="影片名称"
              />
              <input
                type="text"
                value={recommendTestGenres}
                onChange={(e) => setRecommendTestGenres(e.target.value)}
                className="input w-40 text-sm"
                placeholder="类型（逗号分隔）"
              />
              <button
                onClick={() => handleTestRecommend()}
                disabled={testingRecommend || !recommendTestTitle.trim()}
                className="btn-primary gap-1.5 px-4 py-2 text-sm whitespace-nowrap"
              >
                {testingRecommend ? <Loader2 size={14} className="animate-spin" /> : <Play size={14} />}
                测试
              </button>
            </div>

            {/* 预设用例 */}
            <div className="mt-2 flex flex-wrap gap-1.5">
              {RECOMMEND_TEST_CASES.map((tc) => (
                <button
                  key={tc.title}
                  onClick={() => handleTestRecommend(tc.title, tc.genres)}
                  disabled={testingRecommend}
                  className="rounded-md px-2 py-1 text-[11px] transition-colors hover:bg-white/[0.05]"
                  style={{ background: 'var(--bg-surface)', color: 'var(--text-muted)', border: '1px solid var(--border-subtle)' }}
                >
                  {tc.title}
                </button>
              ))}
            </div>

            {/* 推荐理由测试结果 */}
            {recommendTestResult && (
              <div className="mt-3 rounded-lg p-3" style={{ background: 'var(--nav-hover-bg)' }}>
                <div className="flex items-center gap-2 mb-2">
                  {recommendTestResult.success ? (
                    <Check size={14} className="text-green-400" />
                  ) : (
                    <X size={14} className="text-red-400" />
                  )}
                  <span className="text-xs text-surface-400">
                    {recommendTestResult.latency_ms}ms
                  </span>
                </div>
                {recommendTestResult.reason && (
                  <p className="text-sm" style={{ color: 'var(--text-primary)' }}>
                    💡 {recommendTestResult.reason}
                  </p>
                )}
                {recommendTestResult.error && (
                  <p className="text-xs text-red-400">{recommendTestResult.error}</p>
                )}
              </div>
            )}
          </div>
        </div>
      </section>

      {/* ==================== 错误日志 ==================== */}
      <section>
        <button
          onClick={() => {
            setShowErrors(!showErrors)
            if (!showErrors) fetchErrorLogs()
          }}
          className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide transition-colors"
          style={{ color: 'var(--text-primary)' }}
        >
          <AlertTriangle size={20} className="text-neon/60" />
          错误日志
          {errorLogs.length > 0 && (
            <span className="ml-1 rounded-full bg-red-500/20 px-2 py-0.5 text-xs text-red-400">
              {errorLogs.length}
            </span>
          )}
          {showErrors ? <ChevronUp size={16} /> : <ChevronDown size={16} />}
        </button>

        {showErrors && (
          <div className="glass-panel rounded-xl overflow-hidden">
            {errorLogs.length === 0 ? (
              <div className="py-8 text-center">
                <Check size={24} className="mx-auto mb-2 text-green-400" />
                <p className="text-sm" style={{ color: 'var(--text-muted)' }}>
                  暂无错误记录
                </p>
              </div>
            ) : (
              <div className="divide-y" style={{ borderColor: 'var(--border-subtle)' }}>
                {errorLogs.map((log, i) => (
                  <div key={i} className="flex items-start gap-3 px-5 py-3">
                    <X size={14} className="mt-0.5 text-red-400 flex-shrink-0" />
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <span className="rounded bg-red-500/10 px-1.5 py-0.5 text-[10px] font-mono text-red-400">
                          {log.action}
                        </span>
                        <span className="text-[10px] text-surface-500">
                          <Clock size={10} className="inline mr-0.5" />
                          {log.time}
                        </span>
                        <span className="text-[10px] text-surface-500">{log.latency_ms}ms</span>
                      </div>
                      <p className="mt-1 text-xs text-surface-400 break-all">{log.error}</p>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}
      </section>

      {/* ==================== 权限说明 ==================== */}
      <section className="rounded-xl p-4" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
        <div className="flex items-start gap-3">
          <Shield size={16} className="mt-0.5 text-neon/50 flex-shrink-0" />
          <div>
            <p className="text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
              权限说明
            </p>
            <p className="mt-1 text-xs" style={{ color: 'var(--text-muted)' }}>
              AI 配置仅管理员可修改。所有配置变更将实时生效，API 密钥以加密方式存储。
              AI 功能调用不会上传任何用户隐私数据，仅发送影片标题和类型等公开信息。
            </p>
          </div>
        </div>
      </section>
    </div>
  )
}
