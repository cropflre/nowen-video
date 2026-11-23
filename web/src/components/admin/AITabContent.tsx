import { useCallback, useEffect, useRef, useState, type ReactNode } from 'react'
import {
  AlertTriangle,
  BarChart3,
  Check,
  ChevronDown,
  ChevronUp,
  Clock,
  Database as DatabaseIcon,
  Eye,
  EyeOff,
  Loader2,
  MessageSquare,
  RefreshCw,
  Search,
  Settings,
  Shield,
  Sparkles,
  Wifi,
  WifiOff,
  X,
  Zap,
} from 'lucide-react'
import { aiApi } from '@/api'
import { useDialog } from '@/components/Dialog'
import { useToast } from '@/components/Toast'
import {
  Button,
  EmptyState,
  Input,
  Surface,
  Tag,
  type TagTone,
} from '@/components/design-system'
import { useTranslation } from '@/i18n'
import { useServerProfileStore } from '@/stores/serverProfile'
import type { AICacheStats, AIErrorLog, AIStatus, AITestResult } from '@/types'
import {
  AdminPanel,
  AdminStatus,
  type AdminStatusTone,
} from './AdminPrimitives'

const PROVIDERS = [
  {
    id: 'openai',
    name: 'OpenAI',
    apiBase: 'https://api.openai.com/v1',
    models: ['gpt-4o-mini', 'gpt-4o', 'gpt-4-turbo', 'gpt-3.5-turbo'],
  },
  {
    id: 'deepseek',
    name: 'DeepSeek',
    apiBase: 'https://api.deepseek.com/v1',
    models: ['deepseek-chat', 'deepseek-reasoner'],
  },
  {
    id: 'qwen',
    name: '通义千问',
    apiBase: 'https://dashscope.aliyuncs.com/compatible-mode/v1',
    models: ['qwen-turbo', 'qwen-plus', 'qwen-max', 'qwen-long'],
  },
  {
    id: 'ollama',
    name: 'Ollama (本地)',
    apiBase: 'http://localhost:11434/v1',
    models: ['llama3', 'qwen2', 'gemma2', 'mistral'],
  },
  {
    id: 'custom',
    name: '自定义',
    apiBase: '',
    models: [],
  },
]

const SEARCH_TEST_CASES = [
  '帮我找一部2023年的科幻电影',
  '有没有评分8分以上的日本动画',
  '最近有什么好看的悬疑剧',
  '诺兰导演的电影',
]

type ProviderDraft = {
  api_base: string
  api_key: string
  model: string
}

export default function AITab() {
  const toast = useToast()
  const dialog = useDialog()
  const { t } = useTranslation()

  const aiCapability = useServerProfileStore((state) => state.manifest?.capabilities.ai)
  const profileLoaded = useServerProfileStore((state) => state.loaded)
  const profileLoading = useServerProfileStore((state) => state.loading)
  const refreshProfile = useServerProfileStore((state) => state.load)

  const [status, setStatus] = useState<AIStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)

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
  const [showExpert, setShowExpert] = useState(false)

  const draftProfilesRef = useRef<Record<string, ProviderDraft>>({})
  const skipDraftSnapshotRef = useRef(false)

  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<AITestResult | null>(null)
  const [cacheStats, setCacheStats] = useState<AICacheStats | null>(null)
  const [clearingCache, setClearingCache] = useState(false)
  const [errorLogs, setErrorLogs] = useState<AIErrorLog[]>([])
  const [showErrors, setShowErrors] = useState(false)
  const [autoPilotBusy, setAutoPilotBusy] = useState(false)

  const [testingSearch, setTestingSearch] = useState(false)
  const [searchTestQuery, setSearchTestQuery] = useState('')
  const [searchTestResult, setSearchTestResult] = useState<AITestResult | null>(null)
  const [testingRecommend, setTestingRecommend] = useState(false)
  const [recommendTestTitle, setRecommendTestTitle] = useState('')
  const [recommendTestGenres, setRecommendTestGenres] = useState('')
  const [recommendTestResult, setRecommendTestResult] = useState<AITestResult | null>(null)

  const fetchStatus = useCallback(async () => {
    try {
      const res = await aiApi.getStatus()
      const nextStatus = res.data.data
      setStatus(nextStatus)
      skipDraftSnapshotRef.current = true
      setEditEnabled(nextStatus.enabled)
      setEditProvider(nextStatus.provider)
      setEditApiBase(nextStatus.api_base || '')
      setEditModel(nextStatus.model)
      setEditTimeout(nextStatus.timeout || 30)
      setEditSmartSearch(nextStatus.enable_smart_search)
      setEditRecommendReason(nextStatus.enable_recommend_reason)
      setEditMetadataEnhance(nextStatus.enable_metadata_enhance)
      setEditMonthlyBudget(nextStatus.monthly_budget)
      setEditCacheTTL(nextStatus.cache_ttl_hours || 168)
      setEditMaxConcurrent(nextStatus.max_concurrent || 3)
      setEditRequestInterval(nextStatus.request_interval_ms || 200)
      draftProfilesRef.current = {}
      window.setTimeout(() => {
        skipDraftSnapshotRef.current = false
      }, 0)
    } catch {
      // Preserve the existing silent-failure behavior for status loading.
    }
  }, [])

  const fetchCacheStats = useCallback(async () => {
    try {
      const res = await aiApi.getCacheStats()
      setCacheStats(res.data.data)
    } catch {
      // Preserve the existing silent-failure behavior for cache statistics.
    }
  }, [])

  const fetchErrorLogs = useCallback(async () => {
    try {
      const res = await aiApi.getErrorLogs()
      setErrorLogs(res.data.data || [])
    } catch {
      // Preserve the existing silent-failure behavior for error logs.
    }
  }, [])

  useEffect(() => {
    const load = async () => {
      setLoading(true)
      await Promise.all([fetchStatus(), fetchCacheStats(), fetchErrorLogs()])
      setLoading(false)
    }
    void load()
  }, [fetchStatus, fetchCacheStats, fetchErrorLogs])

  const handleToggleAutoPilot = async (enable: boolean) => {
    if (autoPilotBusy) return
    setAutoPilotBusy(true)
    try {
      if (enable) {
        const params: { provider?: string; api_key?: string } = {}
        if (editProvider) params.provider = editProvider
        if (editApiKey) params.api_key = editApiKey
        await aiApi.enableAutoPilot(params)
        toast.success('AI 全自动托管模式已开启')
      } else {
        await aiApi.updateConfig({ auto_pilot: false })
        toast.success('AI 全自动托管模式已关闭')
      }
      await fetchStatus()
    } catch {
      toast.error(enable ? '开启托管模式失败' : '关闭托管模式失败')
    } finally {
      setAutoPilotBusy(false)
    }
  }

  const handleSaveAndTest = async () => {
    const ok = await handleSaveConfig()
    if (ok && editEnabled) {
      await handleTestConnection()
    }
  }

  const handleSaveConfig = async (): Promise<boolean> => {
    setSaving(true)
    try {
      const currentProfile: { api_base: string; api_key?: string; model: string } = {
        api_base: editApiBase,
        model: editModel,
      }
      if (editApiKey) currentProfile.api_key = editApiKey

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
        profiles: {
          [editProvider]: currentProfile,
        },
      }
      if (editApiKey) updates.api_key = editApiKey

      await aiApi.updateConfig(updates)
      delete draftProfilesRef.current[editProvider]
      toast.success(t('aiTab.configSaved'))
      setEditApiKey('')
      await fetchStatus()
      return true
    } catch {
      toast.error(t('aiTab.configSaveFailed'))
      return false
    } finally {
      setSaving(false)
    }
  }

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
      toast.error(t('aiTab.connectionTestFailed'))
    } finally {
      setTesting(false)
    }
  }

  const handleClearCache = async () => {
    const ok = await dialog.confirm({
      title: '清空 AI 缓存',
      message: '确定清空所有 AI 缓存？清空后下次请求将重新调用 AI API。',
      confirmText: '清空',
      variant: 'warning',
    })
    if (!ok) return

    setClearingCache(true)
    try {
      const res = await aiApi.clearCache()
      toast.success(`已清空 ${res.data.data.cleared} 条缓存`)
      await fetchCacheStats()
    } catch {
      toast.error(t('aiTab.clearCacheFailed'))
    } finally {
      setClearingCache(false)
    }
  }

  const handleTestSearch = async (query?: string) => {
    const nextQuery = query || searchTestQuery
    if (!nextQuery.trim()) return
    setTestingSearch(true)
    setSearchTestResult(null)
    if (query) setSearchTestQuery(query)
    try {
      const res = await aiApi.testSmartSearch(nextQuery)
      setSearchTestResult(res.data.data)
    } catch {
      toast.error(t('aiTab.searchTestFailed'))
    } finally {
      setTestingSearch(false)
    }
  }

  const handleTestRecommend = async (title?: string, genres?: string) => {
    const nextTitle = title || recommendTestTitle
    const nextGenres = genres || recommendTestGenres
    if (!nextTitle.trim()) return
    setTestingRecommend(true)
    setRecommendTestResult(null)
    if (title) {
      setRecommendTestTitle(title)
      setRecommendTestGenres(genres || '')
    }
    try {
      const res = await aiApi.testRecommendReason(nextTitle, nextGenres)
      setRecommendTestResult(res.data.data)
    } catch {
      toast.error(t('aiTab.recommendTestFailed'))
    } finally {
      setTestingRecommend(false)
    }
  }

  const handleProviderChange = (providerId: string) => {
    if (providerId === editProvider) return

    if (!skipDraftSnapshotRef.current && editProvider) {
      draftProfilesRef.current[editProvider] = {
        api_base: editApiBase,
        api_key: editApiKey,
        model: editModel,
      }
    }

    setEditProvider(providerId)

    const draft = draftProfilesRef.current[providerId]
    const savedProfile = status?.profiles?.[providerId]
    const preset = PROVIDERS.find((provider) => provider.id === providerId)

    if (draft) {
      setEditApiBase(draft.api_base)
      setEditApiKey(draft.api_key)
      setEditModel(draft.model)
    } else if (savedProfile) {
      setEditApiBase(savedProfile.api_base || preset?.apiBase || '')
      setEditApiKey('')
      setEditModel(savedProfile.model || preset?.models[0] || '')
    } else if (preset) {
      setEditApiBase(preset.apiBase)
      setEditApiKey('')
      setEditModel(preset.models[0] || '')
    }
  }

  const currentProfileSavedKey = Boolean(status?.profiles?.[editProvider]?.api_key_configured)
  const currentProvider = PROVIDERS.find((provider) => provider.id === editProvider)
  const availableModels = currentProvider?.models || []

  if (loading) {
    return (
      <div className="flex min-h-80 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
        <Loader2 size={24} className="animate-spin text-[var(--nv-action-primary)]" />
        正在读取 AI 配置...
      </div>
    )
  }

  const capabilityKnown = profileLoaded && Boolean(aiCapability)
  const configured = capabilityKnown && aiCapability?.configured === true
  const runtimeRunning = capabilityKnown && aiCapability?.available === true && aiCapability?.enabled === true && status?.enabled === true
  const pendingRestart = capabilityKnown && aiCapability?.pending_restart === true

  return (
    <div className="space-y-4">
      <AdminPanel
        title="AI 智能助手"
        description="开启后支持自然语言找片、自动推荐理由，以及影片资料缺失时的 AI 智能补充。"
        icon={<Sparkles size={18} />}
        actions={
          <div className="flex items-center gap-3">
            <Tag tone={runtimeRunning ? 'success' : status?.enabled ? 'warning' : 'neutral'}>
              {runtimeRunning ? '运行中' : status?.enabled ? '已配置' : '未启用'}
            </Tag>
            <SemanticSwitch
              checked={editEnabled}
              onChange={(checked) => setEditEnabled(checked)}
              label="启用 AI 功能"
            />
          </div>
        }
        bodyClassName="space-y-4"
      >
        {!editEnabled ? (
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3">
            <span className="text-sm text-[var(--nv-text-secondary)]">AI 功能已关闭。打开右上角开关并保存即可使用。</span>
            {status?.enabled && (
              <Button type="button" variant="primary" size="sm" onClick={() => void handleSaveConfig()} loading={saving}>
                保存关闭状态
              </Button>
            )}
          </div>
        ) : (
          <div className="space-y-4">
            <FieldGroup label="1. 选择 AI 服务商">
              <div className="flex flex-wrap gap-2">
                {PROVIDERS.map((provider) => {
                  const active = editProvider === provider.id
                  const profileConfigured = Boolean(status?.profiles?.[provider.id]?.api_key_configured)
                  return (
                    <Button
                      key={provider.id}
                      type="button"
                      size="sm"
                      variant={active ? 'primary' : 'secondary'}
                      onClick={() => handleProviderChange(provider.id)}
                    >
                      <span>{provider.name}</span>
                      {profileConfigured && <Check size={12} className="opacity-80" />}
                    </Button>
                  )
                })}
              </div>
            </FieldGroup>

            <FieldGroup label="2. 填写 API Key">
              <div className="relative">
                <Input
                  type={showApiKey ? 'text' : 'password'}
                  value={editApiKey}
                  onChange={(event) => setEditApiKey(event.target.value)}
                  className="pr-11 font-mono"
                  placeholder={currentProfileSavedKey ? '已保存密钥（留空则保持不变）' : '粘贴 API Key'}
                  autoComplete="off"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  iconOnly
                  aria-label={showApiKey ? '隐藏 API 密钥' : '显示 API 密钥'}
                  className="absolute right-1 top-1/2 -translate-y-1/2"
                  onClick={() => setShowApiKey((value) => !value)}
                >
                  {showApiKey ? <EyeOff size={15} /> : <Eye size={15} />}
                </Button>
              </div>
            </FieldGroup>

            {(editProvider === 'custom' || editProvider === 'ollama') && (
              <div className="grid gap-3 sm:grid-cols-2">
                <FieldGroup label="API 地址">
                  <Input
                    type="text"
                    value={editApiBase}
                    onChange={(event) => setEditApiBase(event.target.value)}
                    className="font-mono"
                    placeholder="https://api.openai.com/v1"
                  />
                </FieldGroup>
                <FieldGroup label="模型名称">
                  <Input
                    type="text"
                    value={editModel}
                    onChange={(event) => setEditModel(event.target.value)}
                    className="font-mono"
                    placeholder="如 gpt-4o-mini / qwen2.5"
                  />
                </FieldGroup>
              </div>
            )}

            <div className="flex flex-wrap items-center justify-between gap-3 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3">
              <div>
                <p className="text-sm font-medium text-[var(--nv-text-primary)]">新影片自动识别与刮削</p>
                <p className="text-xs text-[var(--nv-text-tertiary)]">扫描新影片时自动调用 AI 归类与补全信息，不修改原文件。</p>
              </div>
              <SemanticSwitch
                checked={Boolean(status?.auto_pilot)}
                onChange={(checked) => void handleToggleAutoPilot(checked)}
                disabled={autoPilotBusy}
                label="新影片自动识别与刮削"
              />
            </div>

            <div className="flex flex-wrap items-center gap-2 pt-1">
              <Button type="button" variant="primary" onClick={() => void handleSaveAndTest()} loading={saving || testing}>
                {!(saving || testing) && <Zap size={15} />}
                {saving ? '保存中...' : testing ? '测试连接中...' : '保存并测试连接'}
              </Button>
              <Button type="button" variant="ghost" onClick={() => void handleSaveConfig()} loading={saving}>
                仅保存
              </Button>
              {currentProvider && (
                <span className="text-xs text-[var(--nv-text-tertiary)]">
                  默认模型：<code className="font-mono text-[var(--nv-text-secondary)]">{editModel || availableModels[0] || '默认'}</code>
                </span>
              )}
            </div>

            {testResult && <TestResultPanel result={testResult} />}
          </div>
        )}

        {pendingRestart && (
          <div className="flex items-center gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-status-warning)] px-3 py-2 text-xs text-[var(--nv-status-warning)]">
            <AlertTriangle size={14} className="shrink-0" />
            AI 开关已变更，重启服务端后生效。
          </div>
        )}

        <div className="pt-1">
          <Button type="button" variant="ghost" size="sm" onClick={() => setShowExpert((v) => !v)}>
            {showExpert ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            高级设置与诊断工具（模型、参数、统计与测试）
          </Button>
        </div>
      </AdminPanel>

      {showExpert && (
        <div className="space-y-6">
          <AdminPanel
            title="运行状态详情"
            description="来自服务端能力的详细状态。"
            icon={<Sparkles size={18} />}
            actions={
              <Button size="sm" variant="secondary" onClick={() => void refreshProfile(true)} disabled={profileLoading}>
                <RefreshCw size={14} className={profileLoading ? 'animate-spin' : undefined} />
                重新检测
              </Button>
            }
          >
            <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
              <CapabilityStateCard
                icon={<Settings size={17} />}
                label="配置状态"
                value={!profileLoaded ? '检测中' : configured ? '配置开启' : '配置关闭'}
                detail="capabilities.ai.configured"
                tone={configured ? 'active' : 'neutral'}
              />
              <CapabilityStateCard
                icon={runtimeRunning ? <Wifi size={17} /> : <WifiOff size={17} />}
                label="实际运行"
                value={!profileLoaded ? '检测中' : runtimeRunning ? '运行中' : configured ? '尚未运行' : '已停止'}
                detail="AIStatus"
                tone={runtimeRunning ? 'success' : configured ? 'warning' : 'neutral'}
              />
              <CapabilityStateCard
                icon={<RefreshCw size={17} />}
                label="进程状态"
                value={!profileLoaded ? '检测中' : pendingRestart ? '等待重启' : '无需重启'}
                detail={aiCapability?.requires_restart ? '需要服务重启' : '热更新支持'}
                tone={pendingRestart ? 'warning' : 'neutral'}
              />
              <CapabilityStateCard
                icon={<Shield size={17} />}
                label="API 配置"
                value={status?.api_configured ? '密钥已配置' : '密钥未配置'}
                detail={`${status?.provider || editProvider} · ${status?.model || editModel || '未选模型'}`}
                tone={status?.api_configured ? 'success' : 'warning'}
              />
            </div>
          </AdminPanel>

          <AdminPanel title="高级参数与功能开关" description="自定义 API 地址、模型版本、独立功能开关与请求并发。" icon={<Settings size={18} />} bodyClassName="space-y-4">
            <div className="grid gap-4 sm:grid-cols-2">
              <FieldGroup label="API 地址">
                <Input type="text" value={editApiBase} onChange={(event) => setEditApiBase(event.target.value)} className="font-mono" placeholder="https://api.openai.com/v1" />
              </FieldGroup>
              <FieldGroup label="模型">
                <Input type="text" value={editModel} onChange={(event) => setEditModel(event.target.value)} className="font-mono" placeholder="输入模型名称" />
              </FieldGroup>
            </div>
            {availableModels.length > 0 && (
              <div className="flex flex-wrap gap-2">
                {availableModels.map((model) => (
                  <Button key={model} type="button" size="sm" variant={editModel === model ? 'primary' : 'secondary'} className="font-mono" onClick={() => setEditModel(model)}>
                    {model}
                  </Button>
                ))}
              </div>
            )}

            <div className="grid gap-2 lg:grid-cols-3">
              <FeatureToggle icon={<Search size={16} />} label="智能搜索" description="自然语言查询" checked={editSmartSearch} onChange={setEditSmartSearch} />
              <FeatureToggle icon={<MessageSquare size={16} />} label="推荐理由" description="个性化推荐文案" checked={editRecommendReason} onChange={setEditRecommendReason} />
              <FeatureToggle icon={<Sparkles size={16} />} label="元数据增强" description="传统源失败时 AI 兜底" checked={editMetadataEnhance} onChange={setEditMetadataEnhance} />
            </div>

            <div>
              <Button type="button" variant="ghost" size="sm" onClick={() => setShowAdvanced((value) => !value)}>
                {showAdvanced ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
                请求参数（超时、预算、并发）
              </Button>
              {showAdvanced && (
                <Surface className="mt-3 grid gap-4 p-4 sm:grid-cols-2 xl:grid-cols-3">
                  <NumberField label="请求超时（秒）" value={editTimeout} min={5} max={120} onChange={setEditTimeout} />
                  <NumberField label="月度预算上限（0=不限）" value={editMonthlyBudget} min={0} onChange={setEditMonthlyBudget} />
                  <NumberField label="缓存时长（小时）" value={editCacheTTL} min={0} onChange={setEditCacheTTL} />
                  <NumberField label="最大并发数" value={editMaxConcurrent} min={1} max={10} onChange={setEditMaxConcurrent} />
                  <NumberField label="请求间隔（毫秒）" value={editRequestInterval} min={0} onChange={setEditRequestInterval} />
                </Surface>
              )}
            </div>

            <div className="flex flex-wrap items-center gap-2 border-t border-[var(--nv-border-subtle)] pt-4">
              <Button type="button" variant="primary" onClick={() => void handleSaveConfig()} loading={saving}>
                保存高级配置
              </Button>
              <Button type="button" variant="secondary" onClick={() => void handleTestConnection()} loading={testing} disabled={!status?.api_configured}>
                测试连接
              </Button>
            </div>
          </AdminPanel>

          <div className="grid gap-6 xl:grid-cols-2">
            <AdminPanel title="使用统计" description="调用量与 Token 数据。" icon={<BarChart3 size={18} />}>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-2 2xl:grid-cols-4">
                <MetricCard label="本月请求" value={(status?.monthly_calls || 0).toLocaleString()} />
                <MetricCard label="输入 Token" value={`${((status?.total_prompt_tokens || 0) / 1000).toFixed(1)}K`} />
                <MetricCard label="输出 Token" value={`${((status?.total_completion_tokens || 0) / 1000).toFixed(1)}K`} />
                <MetricCard label="费用估算" value={`$${(((status?.total_tokens || 0) / 1_000_000) * 0.15).toFixed(4)}`} detail="参考估算" />
              </div>
            </AdminPanel>

            <AdminPanel
              title="缓存管理"
              description="清空后下次相同请求会重新调用 AI API。"
              icon={<DatabaseIcon size={18} />}
              actions={
                <div className="flex items-center gap-2">
                  <Button size="sm" variant="secondary" onClick={() => void fetchCacheStats()}><RefreshCw size={13} />刷新</Button>
                  <Button size="sm" variant="danger" onClick={handleClearCache} loading={clearingCache} disabled={!cacheStats?.total_entries}>清空缓存</Button>
                </div>
              }
            >
              <div className="grid grid-cols-3 gap-3">
                <MetricCard label="有效" value={(cacheStats?.active_entries || 0).toLocaleString()} />
                <MetricCard label="总计" value={(cacheStats?.total_entries || 0).toLocaleString()} />
                <MetricCard label="已过期" value={(cacheStats?.expired_entries || 0).toLocaleString()} detail={`TTL ${cacheStats?.ttl_hours || 0}h`} />
              </div>
            </AdminPanel>
          </div>

          <div className="grid gap-6 xl:grid-cols-2">
            <AdminPanel title="智能搜索测试" description="使用已保存配置解析自然语言查询。" icon={<Search size={18} />} bodyClassName="space-y-3">
              <div className="flex flex-col gap-2 sm:flex-row">
                <Input type="text" value={searchTestQuery} onChange={(event) => setSearchTestQuery(event.target.value)} placeholder="输入自然语言查询..." onKeyDown={(event) => { if (event.key === 'Enter') void handleTestSearch() }} />
                <Button type="button" variant="secondary" onClick={() => void handleTestSearch()} loading={testingSearch} disabled={!searchTestQuery.trim()} className="shrink-0">测试</Button>
              </div>
              <PresetButtons values={SEARCH_TEST_CASES} disabled={testingSearch} onSelect={(value) => void handleTestSearch(value)} />
              {searchTestResult && <TestResultPanel result={searchTestResult} showPayload />}
            </AdminPanel>

            <AdminPanel title="推荐理由测试" description="输入影片名称和类型测试生成。" icon={<MessageSquare size={18} />} bodyClassName="space-y-3">
              <div className="grid gap-2 sm:grid-cols-2">
                <Input type="text" value={recommendTestTitle} onChange={(event) => setRecommendTestTitle(event.target.value)} placeholder="影片名称" />
                <Input type="text" value={recommendTestGenres} onChange={(event) => setRecommendTestGenres(event.target.value)} placeholder="类型，如 科幻,悬疑" />
              </div>
              <Button type="button" variant="secondary" onClick={() => void handleTestRecommend()} loading={testingRecommend} disabled={!recommendTestTitle.trim()}>生成测试</Button>
              {recommendTestResult && <TestResultPanel result={recommendTestResult} />}
            </AdminPanel>
          </div>

          <AdminPanel
            title="错误日志"
            description="最近一次调用失败记录。"
            icon={<AlertTriangle size={18} />}
            actions={
              <Button size="sm" variant="secondary" onClick={() => { const next = !showErrors; setShowErrors(next); if (next) void fetchErrorLogs() }}>
                {showErrors ? '收起' : '查看错误日志'}
              </Button>
            }
          >
            {showErrors && (
              errorLogs.length === 0 ? (
                <EmptyState icon={<Check size={24} />} title="暂无错误记录" description="当前没有 AI 错误日志。" className="min-h-32" />
              ) : (
                <div className="divide-y divide-[var(--nv-border-subtle)] overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)]">
                  {errorLogs.map((log, index) => (
                    <div key={`${log.time}-${index}`} className="flex items-start gap-3 bg-[var(--nv-bg-surface)] px-4 py-3">
                      <X size={14} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" />
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <Tag tone="danger" className="font-mono">{log.action}</Tag>
                          <span className="flex items-center gap-1 text-xs text-[var(--nv-text-tertiary)]"><Clock size={11} />{log.time}</span>
                        </div>
                        <p className="mt-1 break-all text-xs text-[var(--nv-text-secondary)]">{log.error}</p>
                      </div>
                    </div>
                  ))}
                </div>
              )
            )}
          </AdminPanel>
        </div>
      )}
    </div>
  )
}

function CapabilityStateCard({
  icon,
  label,
  value,
  detail,
  tone,
}: {
  icon: ReactNode
  label: string
  value: string
  detail: string
  tone: AdminStatusTone
}) {
  return (
    <Surface className="p-4">
      <div className="flex items-center justify-between gap-3">
        <div className="flex h-9 w-9 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
          {icon}
        </div>
        <AdminStatus tone={tone}>{value}</AdminStatus>
      </div>
      <p className="mt-3 text-sm font-medium text-[var(--nv-text-primary)]">{label}</p>
      <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">{detail}</p>
    </Surface>
  )
}

function SemanticSwitch({
  checked,
  onChange,
  label,
  disabled = false,
}: {
  checked: boolean
  onChange: (checked: boolean) => void
  label: string
  disabled?: boolean
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!checked)}
      className={`relative inline-flex h-7 w-12 shrink-0 items-center rounded-full border transition-[background-color,border-color] duration-200 disabled:cursor-not-allowed disabled:opacity-50 ${
        checked
          ? 'border-[var(--nv-action-primary)] bg-[var(--nv-action-primary)]'
          : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-control)]'
      }`}
    >
      <span
        className={`h-5 w-5 rounded-full bg-[var(--nv-text-on-brand)] shadow-sm transition-transform duration-200 ${
          checked ? 'translate-x-6' : 'translate-x-1'
        }`}
      />
    </button>
  )
}

function FeatureToggle({
  icon,
  label,
  description,
  checked,
  onChange,
}: {
  icon: ReactNode
  label: string
  description: string
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  return (
    <Surface className="flex items-center justify-between gap-3 p-3">
      <div className="flex min-w-0 items-start gap-3">
        <span className="mt-0.5 text-[var(--nv-action-primary)]">{icon}</span>
        <div>
          <p className="text-sm font-medium text-[var(--nv-text-primary)]">{label}</p>
          <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">{description}</p>
        </div>
      </div>
      <SemanticSwitch checked={checked} onChange={onChange} label={label} />
    </Surface>
  )
}

function FieldGroup({
  label,
  description,
  children,
}: {
  label: string
  description?: string
  children: ReactNode
}) {
  return (
    <div>
      <label className="mb-1.5 block text-sm font-medium text-[var(--nv-text-secondary)]">{label}</label>
      {description && <p className="mb-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{description}</p>}
      {children}
    </div>
  )
}

function NumberField({
  label,
  value,
  min,
  max,
  onChange,
}: {
  label: string
  value: number
  min?: number
  max?: number
  onChange: (value: number) => void
}) {
  return (
    <label className="block">
      <span className="mb-1.5 block text-xs font-medium text-[var(--nv-text-tertiary)]">{label}</span>
      <Input
        type="number"
        value={value}
        min={min}
        max={max}
        onChange={(event) => onChange(Number(event.target.value))}
      />
    </label>
  )
}

function MetricCard({ label, value, detail }: { label: string; value: string; detail?: string }) {
  return (
    <Surface className="p-3 text-center">
      <p className="text-xl font-semibold tracking-tight text-[var(--nv-text-primary)]">{value}</p>
      <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{label}</p>
      {detail && <p className="mt-1 text-[10px] leading-4 text-[var(--nv-text-tertiary)]">{detail}</p>}
    </Surface>
  )
}

function PresetButtons({
  values,
  disabled,
  onSelect,
}: {
  values: string[]
  disabled: boolean
  onSelect: (value: string) => void
}) {
  return (
    <div className="flex flex-wrap gap-1.5">
      {values.map((value) => (
        <Button key={value} type="button" size="sm" variant="ghost" disabled={disabled} onClick={() => onSelect(value)}>
          {value}
        </Button>
      ))}
    </div>
  )
}

function TestResultPanel({ result, showPayload = false }: { result: AITestResult; showPayload?: boolean }) {
  const tone: TagTone = result.success ? 'success' : 'danger'
  return (
    <Surface className="p-4">
      <div className="flex flex-wrap items-center gap-2">
        <Tag tone={tone}>
          {result.success ? <Check size={12} /> : <X size={12} />}
          {result.success ? '成功' : '失败'}
        </Tag>
        <span className="text-xs text-[var(--nv-text-tertiary)]">{result.latency_ms}ms</span>
        {result.provider && <Tag tone="neutral">{result.provider}</Tag>}
        {result.model && <Tag tone="neutral">{result.model}</Tag>}
      </div>
      {result.reason && <p className="mt-3 text-sm leading-6 text-[var(--nv-text-secondary)]">{result.reason}</p>}
      {result.response && <p className="mt-3 text-sm leading-6 text-[var(--nv-text-secondary)]">{result.response}</p>}
      {showPayload && result.intent && (
        <pre className="mt-3 max-h-56 overflow-auto whitespace-pre-wrap break-all rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] p-3 font-mono text-[11px] leading-5 text-[var(--nv-text-secondary)]">
          {JSON.stringify(result.intent, null, 2)}
        </pre>
      )}
      {result.error && <p className="mt-3 break-all text-xs leading-5 text-[var(--nv-status-danger)]">{result.error}</p>}
    </Surface>
  )
}

