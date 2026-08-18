import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  BatteryCharging,
  Cpu,
  Loader2,
  Monitor,
  RefreshCw,
  Server,
  Smartphone,
  Wifi,
  Zap,
} from 'lucide-react'
import {
  mediaAnalysisApi,
  type MediaAnalysisExecutionMode,
  type MediaAnalysisWorker,
} from '@/api/mediaAnalysis'
import { AdminPanel, AdminStatus, type AdminStatusTone } from './AdminPrimitives'
import { Button, Surface } from '@/components/design-system'

const MODE_OPTIONS: Array<{
  value: MediaAnalysisExecutionMode
  title: string
  description: string
  badge: string
}> = [
  {
    value: 'auto',
    title: '自动（推荐）',
    description: '优先把精彩片段计算交给可用客户端；短时间没有客户端时自动回退服务端。',
    badge: '客户端优先 + 自动兜底',
  },
  {
    value: 'client_preferred',
    title: '客户端优先',
    description: '只等待客户端计算，不让 NAS 承担精彩片段分析。适合性能较弱的服务器。',
    badge: '最低服务端负载',
  },
  {
    value: 'server_only',
    title: '仅服务端',
    description: '保持原有 Sparse V2 行为，全部由服务器 FFmpeg 完成分析和缩略图生成。',
    badge: '传统模式',
  },
  {
    value: 'off',
    title: '关闭',
    description: '禁止启动新的精彩片段分析任务，已有精彩片段仍可正常读取。',
    badge: '停止新任务',
  },
]

function workerIcon(kind: string) {
  if (kind === 'android') return <Smartphone size={17} />
  if (kind === 'desktop') return <Monitor size={17} />
  return <Cpu size={17} />
}

function workerState(worker: MediaAnalysisWorker): { label: string; tone: AdminStatusTone } {
  switch (worker.state) {
    case 'busy': return { label: '计算中', tone: 'active' }
    case 'idle': return { label: '可用', tone: 'success' }
    case 'unavailable': return { label: '暂不可用', tone: 'warning' }
    default: return { label: worker.state || '未知', tone: 'neutral' }
  }
}

function workerKindLabel(kind: string) {
  if (kind === 'android') return 'Android'
  if (kind === 'desktop') return 'Desktop'
  return kind || 'Client'
}

function relativeSeen(value: string) {
  const timestamp = new Date(value).getTime()
  if (!Number.isFinite(timestamp)) return '刚刚'
  const seconds = Math.max(0, Math.round((Date.now() - timestamp) / 1000))
  if (seconds < 10) return '刚刚'
  if (seconds < 60) return `${seconds} 秒前`
  return `${Math.floor(seconds / 60)} 分钟前`
}

export default function MediaAnalysisComputePanel() {
  const [mode, setMode] = useState<MediaAnalysisExecutionMode>('auto')
  const [workers, setWorkers] = useState<MediaAnalysisWorker[]>([])
  const [loading, setLoading] = useState(true)
  const [savingMode, setSavingMode] = useState<MediaAnalysisExecutionMode | null>(null)
  const [error, setError] = useState('')

  const load = useCallback(async (silent = false) => {
    if (!silent) setLoading(true)
    try {
      const [configResult, workersResult] = await Promise.all([
        mediaAnalysisApi.getWorkerConfig(),
        mediaAnalysisApi.getWorkers(),
      ])
      setMode(configResult.data.data.execution_mode || 'auto')
      setWorkers(workersResult.data.data || [])
      setError('')
    } catch (requestError: any) {
      setError(requestError?.response?.data?.error || '读取精彩片段计算节点状态失败')
    } finally {
      if (!silent) setLoading(false)
    }
  }, [])

  useEffect(() => {
    void load()
    const timer = window.setInterval(() => void load(true), 10_000)
    return () => window.clearInterval(timer)
  }, [load])

  const applyMode = async (nextMode: MediaAnalysisExecutionMode) => {
    if (savingMode || nextMode === mode) return
    setSavingMode(nextMode)
    setError('')
    try {
      const response = await mediaAnalysisApi.updateWorkerConfig(nextMode)
      setMode(response.data.data.execution_mode)
    } catch (requestError: any) {
      setError(requestError?.response?.data?.error || '保存精彩片段计算模式失败')
    } finally {
      setSavingMode(null)
    }
  }

  const eligibleCount = useMemo(
    () => workers.filter((worker) => worker.state === 'idle' || worker.state === 'busy').length,
    [workers],
  )

  return (
    <AdminPanel
      title="精彩片段计算节点"
      description="精彩片段与 AI 配置相互独立。服务端负责调度、结果校验与持久化，Desktop / Android 客户端可承担稀疏解码、评分和缩略图生成。"
      icon={<Zap size={18} />}
      actions={(
        <div className="flex items-center gap-2">
          <AdminStatus tone={eligibleCount > 0 ? 'connected' : 'neutral'}>
            {eligibleCount > 0 ? <Wifi size={13} /> : <Server size={13} />}
            {eligibleCount > 0 ? `${eligibleCount} 个节点可用` : '当前无客户端节点'}
          </AdminStatus>
          <Button variant="secondary" size="sm" onClick={() => void load()} disabled={loading}>
            <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
            刷新
          </Button>
        </div>
      )}
      bodyClassName="space-y-5"
    >
      {loading ? (
        <div className="flex min-h-36 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
          <Loader2 size={20} className="animate-spin text-[var(--nv-action-primary)]" />
          正在读取计算模式与节点状态...
        </div>
      ) : (
        <>
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
            {MODE_OPTIONS.map((option) => {
              const selected = mode === option.value
              const saving = savingMode === option.value
              return (
                <button
                  type="button"
                  key={option.value}
                  onClick={() => void applyMode(option.value)}
                  disabled={Boolean(savingMode)}
                  className={`rounded-[var(--nv-radius-control)] border p-4 text-left transition-all ${selected
                    ? 'border-[var(--nv-action-primary)] bg-[color-mix(in_srgb,var(--nv-action-primary)_10%,var(--nv-bg-surface))] shadow-[var(--nv-shadow-soft)]'
                    : 'border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] hover:border-[var(--nv-border-default)] hover:bg-[var(--nv-bg-hover)]'}`}
                >
                  <div className="flex items-start justify-between gap-3">
                    <div className="text-sm font-semibold text-[var(--nv-text-primary)]">{option.title}</div>
                    {saving ? <Loader2 size={15} className="animate-spin text-[var(--nv-action-primary)]" /> : selected ? (
                      <AdminStatus tone="active">当前</AdminStatus>
                    ) : null}
                  </div>
                  <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{option.description}</p>
                  <div className="mt-3 text-[11px] font-medium text-[var(--nv-text-secondary)]">{option.badge}</div>
                </button>
              )
            })}
          </div>

          {error && (
            <Surface className="border-[color-mix(in_srgb,var(--nv-status-danger)_28%,var(--nv-border-subtle))] p-3 text-sm text-[var(--nv-status-danger)]">
              {error}
            </Surface>
          )}

          <div>
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">在线计算节点</h3>
                <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">节点超过约 90 秒未上报会自动从列表移除；正在计算的节点会随进度自动续租并刷新在线状态。</p>
              </div>
              <AdminStatus tone={workers.length > 0 ? 'success' : 'neutral'}>{workers.length} 个已发现</AdminStatus>
            </div>

            {workers.length === 0 ? (
              <div className="rounded-[var(--nv-radius-control)] border border-dashed border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] px-4 py-8 text-center">
                <Monitor size={24} className="mx-auto text-[var(--nv-text-tertiary)]" />
                <div className="mt-3 text-sm font-medium text-[var(--nv-text-secondary)]">暂无在线客户端计算节点</div>
                <p className="mx-auto mt-1 max-w-xl text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  Desktop 管理员客户端在 libmpv 可用且未处于播放页时优先参与；Android 管理员客户端在前台、Wi-Fi 且充电中或电量不少于 40% 时参与。自动模式下没有客户端会回退服务端。
                </p>
              </div>
            ) : (
              <div className="grid grid-cols-1 gap-3 lg:grid-cols-2">
                {workers.map((worker) => {
                  const state = workerState(worker)
                  return (
                    <Surface key={worker.worker_id} className="p-4">
                      <div className="flex items-start gap-3">
                        <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
                          {workerIcon(worker.kind)}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex flex-wrap items-center gap-2">
                            <div className="truncate text-sm font-semibold text-[var(--nv-text-primary)]">{worker.name || worker.worker_id}</div>
                            <AdminStatus tone={state.tone}>{state.label}</AdminStatus>
                          </div>
                          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
                            <span>{workerKindLabel(worker.kind)}</span>
                            <span>最后在线 {relativeSeen(worker.last_seen)}</span>
                            {worker.network && <span>{worker.network.toUpperCase()}</span>}
                            {worker.kind === 'android' && (
                              <span className="inline-flex items-center gap-1">
                                <BatteryCharging size={12} />
                                {worker.battery_percent}%{worker.charging ? ' · 充电中' : ''}
                              </span>
                            )}
                          </div>
                          {worker.task_id && (
                            <div className="mt-2 truncate font-mono text-[11px] text-[var(--nv-text-secondary)]">任务 {worker.task_id}</div>
                          )}
                        </div>
                      </div>
                    </Surface>
                  )
                })}
              </div>
            )}
          </div>
        </>
      )}
    </AdminPanel>
  )
}
