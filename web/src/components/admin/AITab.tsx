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
  Play,
  Power,
  RefreshCw,
  Rocket,
  Search,
  Settings,
  Shield,
  Sparkles,
  Trash2,
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

const RECOMMEND_TEST_CASES = [
  { title: '星际穿越', genres: '科幻,冒险,剧情' },
  { title: '你的名字', genres: '动画,爱情,奇幻' },
  { title: '肖申克的救赎', genres: '剧情,犯罪' },
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

  const handleSaveConfig = async () => {
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
    } catch {
      toast.error(t('aiTab.configSaveFailed'))
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
  const currentProviderHasDraft = Boolean(draftProfilesRef.current[editProvider])
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
  const capabilityUnavailable = capabilityKnown && aiCapability?.available !== true

  return (
    <div className="space-y-6">
      <AdminPanel
        title="AI 运行状态"
        description="配置状态、实际运行状态和待重启状态来自服务端能力契约，避免把“已保存配置”误判为“正在运行”。"
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
            detail="来自 capabilities.ai.configured"
            tone={configured ? 'active' : 'neutral'}
          />
          <CapabilityStateCard
            icon={runtimeRunning ? <Wifi size={17} /> : <WifiOff size={17} />}
            label="实际运行"
            value={!profileLoaded ? '检测中' : runtimeRunning ? '运行中' : configured ? '尚未运行' : '已停止'}
            detail="由 capability 运行态与 AIStatus 共同确认"
            tone={runtimeRunning ? 'success' : configured ? 'warning' : 'neutral'}
          />
          <CapabilityStateCard
            icon={<RefreshCw size={17} />}
            label="进程状态"
            value={!profileLoaded ? '检测中' : pendingRestart ? '等待重启' : '无需重启'}
            detail={aiCapability?.requires_restart ? '此能力的开关变更需要服务重启' : '当前能力无需进程级重启'}
            tone={pendingRestart ? 'warning' : 'neutral'}
          />
          <CapabilityStateCard
            icon={<Shield size={17} />}
            label="API 配置"
            value={status?.api_configured ? '密钥已配置' : '密钥未配置'}
            detail={`${status?.provider || editProvider} · ${status?.model || editModel || '未选择模型'}`}
            tone={status?.api_configured ? 'success' : 'warning'}
          />
        </div>

        {pendingRestart && (
          <Surface className="mt-4 border-[var(--nv-status-warning)] p-4">
            <div className="flex items-start gap-3">
              <AlertTriangle size={18} className="mt-0.5 shrink-0 text-[var(--nv-status-warning)]" />
              <div className="min-w-0">
                <p className="text-sm font-semibold text-[var(--nv-text-primary)]">AI 配置等待服务重启</p>
                <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  {configured && !aiCapability?.enabled
                    ? 'AI 配置已经开启，但运行路由、数据表和后台组件需要重启服务后才会装配。当前仍可以继续编辑配置和测试连接。'
                    : 'AI 配置已经关闭，客户端已停止把 AI 视为可用；重启服务后将完成运行组件回收。'}
                </p>
              </div>
            </div>
          </Surface>
        )}

        {capabilityUnavailable && !pendingRestart && (
          <Surface className="mt-4 p-4">
            <div className="flex items-start gap-3">
              <AlertTriangle size={18} className="mt-0.5 shrink-0 text-[var(--nv-status-warning)]" />
              <div>
                <p className="text-sm font-semibold text-[var(--nv-text-primary)]">当前服务未装配 AI 运行能力</p>
                <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  可以继续维护配置；实际可用性以服务端 capabilities.ai 为准。
                </p>
              </div>
            </div>
          </Surface>
        )}
      </AdminPanel>

      <AdminPanel
        title="全自动托管模式"
        description="新增媒体库后自动串联 AI 识别、归类、命名、元数据刮削与 AI 兜底。仅写入数据库，不修改原始文件。"
        icon={<Rocket size={18} />}
        actions={
          <SemanticSwitch
            checked={Boolean(status?.auto_pilot)}
            onChange={(checked) => void handleToggleAutoPilot(checked)}
            disabled={autoPilotBusy}
            label="全自动托管模式"
          />
        }
      >
        <div className="flex flex-wrap items-center gap-2">
          <AdminStatus tone={status?.auto_pilot ? 'active' : 'neutral'}>
            {status?.auto_pilot ? '托管已开启' : '托管已关闭'}
          </AdminStatus>
          <Tag tone="neutral">云端 LLM</Tag>
          <span className="text-xs text-[var(--nv-text-tertiary)]">托管模式会拒绝 Ollama 等本地 AI。</span>
        </div>

        {!status?.api_configured && (
          <div className="mt-4 flex items-start gap-2 text-xs leading-5 text-[var(--nv-status-warning)]">
            <AlertTriangle size={14} className="mt-0.5 shrink-0" />
            请先填写并保存 API Key，托管模式才能稳定生效。
          </div>
        )}
        {status?.auto_pilot && status?.api_configured && (
          <div className="mt-4 flex items-start gap-2 text-xs leading-5 text-[var(--nv-status-success)]">
            <Check size={14} className="mt-0.5 shrink-0" />
            托管模式已生效：后续扫描入库会调用 {status.provider} / {status.model}。
          </div>
        )}
      </AdminPanel>

      <AdminPanel
        title="AI 服务配置"
        description="Provider 切换时会在当前页面内暂存未保存草稿；API Key 留空保存不会清除已存密钥。"
        icon={<Settings size={18} />}
        bodyClassName="space-y-5"
      >
        <Surface className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-3">
            <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
              <Power size={18} />
            </div>
            <div>
              <p className="text-sm font-medium text-[var(--nv-text-primary)]">启用 AI 功能</p>
              <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">这是待保存的配置值；实际运行状态请以上方能力状态为准。</p>
            </div>
          </div>
          <SemanticSwitch checked={editEnabled} onChange={setEditEnabled} label="启用 AI 功能" />
        </Surface>

        <FieldGroup label="LLM 提供商" description="已保存密钥用 Success 标记；未保存草稿用 Warning 标记。">
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-3 xl:grid-cols-5">
            {PROVIDERS.map((provider) => {
              const active = editProvider === provider.id
              const profileConfigured = Boolean(status?.profiles?.[provider.id]?.api_key_configured)
              const hasDraft = Boolean(draftProfilesRef.current[provider.id]) && provider.id !== editProvider
              return (
                <Button
                  key={provider.id}
                  type="button"
                  variant={active ? 'primary' : 'secondary'}
                  className="h-auto min-h-12 flex-col gap-1.5 py-2.5"
                  onClick={() => handleProviderChange(provider.id)}
                >
                  <span>{provider.name}</span>
                  <span className="flex flex-wrap items-center justify-center gap-1">
                    {profileConfigured && <Tag tone="success">已配置</Tag>}
                    {hasDraft && <Tag tone="warning">草稿</Tag>}
                  </span>
                </Button>
              )
            })}
          </div>
        </FieldGroup>

        <FieldGroup label="API 地址">
          <Input
            type="text"
            value={editApiBase}
            onChange={(event) => setEditApiBase(event.target.value)}
            className="font-mono"
            placeholder="https://api.openai.com/v1"
          />
        </FieldGroup>

        <FieldGroup label="API 密钥">
          <div className="relative">
            <Input
              type={showApiKey ? 'text' : 'password'}
              value={editApiKey}
              onChange={(event) => setEditApiKey(event.target.value)}
              className="pr-11 font-mono"
              placeholder={currentProfileSavedKey ? '已配置（输入新值可覆盖）' : '请输入 API Key'}
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
          <div className="mt-2 space-y-1">
            {currentProfileSavedKey && !editApiKey && (
              <p className="flex items-center gap-1.5 text-xs text-[var(--nv-status-success)]">
                <Check size={12} />
                {currentProvider?.name || editProvider} 的密钥已保存，留空提交不会清除。
              </p>
            )}
            {currentProviderHasDraft && (
              <p className="flex items-center gap-1.5 text-xs text-[var(--nv-status-warning)]">
                <AlertTriangle size={12} />
                当前 Provider 有未保存修改，切换时会在本页暂存草稿。
              </p>
            )}
          </div>
        </FieldGroup>

        <FieldGroup label="模型" description={availableModels.length > 0 ? '可以手动输入模型名称，或使用下方预置模型。' : '当前 Provider 没有预置列表，请手动输入模型名称。'}>
          <Input
            type="text"
            value={editModel}
            onChange={(event) => setEditModel(event.target.value)}
            className="font-mono"
            placeholder={availableModels.length > 0 ? '输入模型名称，或从下方列表选择' : '输入模型名称，如 gpt-4o-mini'}
          />
          {editModel && (
            <div className="mt-2">
              <Tag tone={availableModels.includes(editModel) ? 'brand' : 'neutral'}>
                {availableModels.includes(editModel) ? '预置模型' : '自定义模型'}
              </Tag>
            </div>
          )}
          {availableModels.length > 0 && (
            <div className="mt-2 flex flex-wrap gap-2">
              {availableModels.map((model) => (
                <Button
                  key={model}
                  type="button"
                  size="sm"
                  variant={editModel === model ? 'primary' : 'secondary'}
                  className="font-mono"
                  onClick={() => setEditModel(model)}
                >
                  {model}
                </Button>
              ))}
            </div>
          )}
        </FieldGroup>

        <FieldGroup label="功能开关">
          <div className="grid gap-2 lg:grid-cols-3">
            <FeatureToggle
              icon={<Search size={16} />}
              label="智能搜索"
              description="自然语言 → 结构化查询参数"
              checked={editSmartSearch}
              onChange={setEditSmartSearch}
            />
            <FeatureToggle
              icon={<MessageSquare size={16} />}
              label="推荐理由"
              description="AI 生成个性化推荐文案"
              checked={editRecommendReason}
              onChange={setEditRecommendReason}
            />
            <FeatureToggle
              icon={<Sparkles size={16} />}
              label="元数据增强"
              description="传统数据源失败时用 AI 补充"
              checked={editMetadataEnhance}
              onChange={setEditMetadataEnhance}
            />
          </div>
        </FieldGroup>

        <div>
          <Button type="button" variant="ghost" size="sm" onClick={() => setShowAdvanced((value) => !value)}>
            {showAdvanced ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
            高级设置
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
          <Button type="button" variant="primary" onClick={handleSaveConfig} loading={saving}>
            {!saving && <Check size={15} />}
            {saving ? '保存中...' : '保存配置'}
          </Button>
          <Button type="button" variant="secondary" onClick={handleTestConnection} loading={testing} disabled={!status?.api_configured}>
            {!testing && <Zap size={15} />}
            {testing ? '测试中...' : '测试已保存配置'}
          </Button>
          <span className="text-xs text-[var(--nv-text-tertiary)]">连接测试使用服务端已保存配置，不会测试尚未保存的表单草稿。</span>
        </div>

        {testResult && <TestResultPanel result={testResult} />}
      </AdminPanel>

      <div className="grid gap-6 xl:grid-cols-2">
        <AdminPanel title="使用统计" description="调用量与 Token 数据来自当前 AIStatus。" icon={<BarChart3 size={18} />}>
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-2 2xl:grid-cols-4">
            <MetricCard label="本月请求" value={(status?.monthly_calls || 0).toLocaleString()} />
            <MetricCard label="输入 Token" value={`${((status?.total_prompt_tokens || 0) / 1000).toFixed(1)}K`} />
            <MetricCard label="输出 Token" value={`${((status?.total_completion_tokens || 0) / 1000).toFixed(1)}K`} />
            <MetricCard label="费用估算" value={`$${(((status?.total_tokens || 0) / 1_000_000) * 0.15).toFixed(4)}`} detail="按原 gpt-4o-mini 估算规则" />
          </div>

          {status?.monthly_budget ? (
            <div className="mt-4">
              <div className="mb-2 flex items-center justify-between gap-3 text-xs text-[var(--nv-text-tertiary)]">
                <span>月度调用预算</span>
                <span>{status.monthly_calls} / {status.monthly_budget}</span>
              </div>
              <div className="h-2 overflow-hidden rounded-full bg-[var(--nv-bg-control)]">
                <div
                  className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-200"
                  style={{ width: `${Math.min(100, (status.monthly_calls / status.monthly_budget) * 100)}%` }}
                />
              </div>
              {status.monthly_calls >= status.monthly_budget * 0.8 && (
                <div className="mt-3 flex items-start gap-2 text-xs leading-5 text-[var(--nv-status-warning)]">
                  <AlertTriangle size={14} className="mt-0.5 shrink-0" />
                  {status.monthly_calls >= status.monthly_budget
                    ? '月度配额已用尽，AI 功能暂停。'
                    : `月度配额已使用 ${Math.round((status.monthly_calls / status.monthly_budget) * 100)}%，请注意用量。`}
                </div>
              )}
            </div>
          ) : null}
        </AdminPanel>

        <AdminPanel
          title="缓存管理"
          description="清空后下次相同请求会重新调用 AI API。"
          icon={<DatabaseIcon size={18} />}
          actions={
            <div className="flex items-center gap-2">
              <Button size="sm" variant="secondary" onClick={() => void fetchCacheStats()}>
                <RefreshCw size={13} />
                刷新
              </Button>
              <Button
                size="sm"
                variant="danger"
                onClick={handleClearCache}
                loading={clearingCache}
                disabled={!cacheStats?.total_entries}
              >
                {!clearingCache && <Trash2 size={13} />}
                清空缓存
              </Button>
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
        <AdminPanel title="智能搜索测试" description="使用已保存的 AI 配置解析自然语言查询。" icon={<Search size={18} />} bodyClassName="space-y-3">
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              type="text"
              value={searchTestQuery}
              onChange={(event) => setSearchTestQuery(event.target.value)}
              placeholder="输入自然语言查询..."
              onKeyDown={(event) => {
                if (event.key === 'Enter') void handleTestSearch()
              }}
            />
            <Button
              type="button"
              variant="secondary"
              onClick={() => void handleTestSearch()}
              loading={testingSearch}
              disabled={!searchTestQuery.trim()}
              className="shrink-0"
            >
              {!testingSearch && <Play size={14} />}
              测试
            </Button>
          </div>
          <PresetButtons
            values={SEARCH_TEST_CASES}
            disabled={testingSearch}
            onSelect={(value) => void handleTestSearch(value)}
          />
          {searchTestResult && <TestResultPanel result={searchTestResult} showPayload />}
        </AdminPanel>

        <AdminPanel title="推荐理由测试" description="输入影片名称和类型，验证推荐文案生成。" icon={<MessageSquare size={18} />} bodyClassName="space-y-3">
          <div className="grid gap-2 sm:grid-cols-[minmax(0,1fr)_180px_auto]">
            <Input
              type="text"
              value={recommendTestTitle}
              onChange={(event) => setRecommendTestTitle(event.target.value)}
              placeholder="影片名称"
            />
            <Input
              type="text"
              value={recommendTestGenres}
              onChange={(event) => setRecommendTestGenres(event.target.value)}
              placeholder="类型（逗号分隔）"
            />
            <Button
              type="button"
              variant="secondary"
              onClick={() => void handleTestRecommend()}
              loading={testingRecommend}
              disabled={!recommendTestTitle.trim()}
            >
              {!testingRecommend && <Play size={14} />}
              测试
            </Button>
          </div>
          <PresetButtons
            values={RECOMMEND_TEST_CASES.map((item) => item.title)}
            disabled={testingRecommend}
            onSelect={(title) => {
              const preset = RECOMMEND_TEST_CASES.find((item) => item.title === title)
              if (preset) void handleTestRecommend(preset.title, preset.genres)
            }}
          />
          {recommendTestResult && <TestResultPanel result={recommendTestResult} />}
        </AdminPanel>
      </div>

      <AdminPanel
        title="错误日志"
        description="最近的 AI 调用错误，按需展开查看。"
        icon={<AlertTriangle size={18} />}
        actions={
          <Button
            size="sm"
            variant={showErrors ? 'primary' : 'secondary'}
            onClick={() => {
              setShowErrors((value) => !value)
              if (!showErrors) void fetchErrorLogs()
            }}
          >
            {showErrors ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            {showErrors ? '收起' : '展开'}
            {errorLogs.length > 0 && <Tag tone="danger">{errorLogs.length}</Tag>}
          </Button>
        }
      >
        {showErrors ? (
          errorLogs.length === 0 ? (
            <EmptyState
              icon={<Check size={24} />}
              title="暂无错误记录"
              description="当前没有可展示的 AI 错误日志。"
              className="min-h-40"
            />
          ) : (
            <div className="divide-y divide-[var(--nv-border-subtle)] overflow-hidden rounded-[var(--nv-radius-card)] border border-[var(--nv-border-subtle)]">
              {errorLogs.map((log, index) => (
                <div key={`${log.time}-${index}`} className="flex items-start gap-3 bg-[var(--nv-bg-surface)] px-4 py-3">
                  <X size={14} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" />
                  <div className="min-w-0 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <Tag tone="danger" className="font-mono">{log.action}</Tag>
                      <span className="flex items-center gap-1 text-xs text-[var(--nv-text-tertiary)]">
                        <Clock size={11} />
                        {log.time}
                      </span>
                      <AdminStatus tone={latencyTone(log.latency_ms)}>{log.latency_ms}ms</AdminStatus>
                    </div>
                    <p className="mt-2 break-all text-xs leading-5 text-[var(--nv-text-secondary)]">{log.error}</p>
                  </div>
                </div>
              ))}
            </div>
          )
        ) : (
          <p className="text-sm text-[var(--nv-text-tertiary)]">展开后读取并展示最近 AI 错误。</p>
        )}
      </AdminPanel>

      <Surface className="p-4">
        <div className="flex items-start gap-3">
          <Shield size={17} className="mt-0.5 shrink-0 text-[var(--nv-action-primary)]" />
          <div>
            <p className="text-sm font-medium text-[var(--nv-text-primary)]">权限与生效说明</p>
            <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
              AI 配置仅管理员可修改，API 密钥由服务端安全存储。可热加载的参数按后端策略生效；涉及进程级能力装配的开关，以 capabilities.ai.pending_restart 的提示为准。AI 调用仅发送业务所需的媒体标题、类型等信息。
            </p>
          </div>
        </div>
      </Surface>
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

function latencyTone(latencyMs: number): AdminStatusTone {
  if (latencyMs > 1000) return 'danger'
  if (latencyMs > 300) return 'warning'
  return 'neutral'
}
