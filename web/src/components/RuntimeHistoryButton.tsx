import { FormEvent, useCallback, useEffect, useState } from 'react'
import {
  Archive,
  ArrowLeft,
  ChevronLeft,
  ChevronRight,
  CircleAlert,
  Database,
  HardDrive,
  Loader2,
  RefreshCw,
  Search,
  X,
} from 'lucide-react'
import { runtimeHistoryApi } from '@/api'
import type {
  RuntimeHistoryDetail,
  RuntimeHistoryItem,
  RuntimeHistoryList,
  RuntimeHistorySummary,
} from '@/api'
import { Button, Input, Select, Surface, Tag, type TagTone } from '@/components/design-system'
import { useAuthStore } from '@/stores/auth'
import { useServerProfileStore } from '@/stores/serverProfile'

const PAGE_SIZE = 20

function formatBytes(value: number) {
  if (!Number.isFinite(value) || value <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let current = value
  let index = 0
  while (current >= 1024 && index < units.length - 1) {
    current /= 1024
    index += 1
  }
  return `${current >= 10 || index === 0 ? current.toFixed(0) : current.toFixed(1)} ${units[index]}`
}

function formatDate(value?: string) {
  if (!value) return '—'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return '—'
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
  }).format(date)
}

function statusText(status: string) {
  const labels: Record<string, string> = {
    completed: '已完成',
    failed: '失败',
    cancelled: '已取消',
    retired: '已退役',
    queued: '历史排队残留',
    claimed: '历史 Claim 残留',
    running: '历史运行残留',
    cancel_requested: '取消中残留',
  }
  return labels[status] || status || '未知'
}

function statusTone(status: string): TagTone {
  if (status === 'completed') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'cancelled' || status === 'retired') return 'neutral'
  return 'warning'
}

function requestError(error: unknown, fallback: string) {
  if (typeof error === 'object' && error !== null) {
    const response = (error as { response?: { data?: { error?: unknown } } }).response
    if (typeof response?.data?.error === 'string') return response.data.error
  }
  return error instanceof Error ? error.message : fallback
}

function HistoryCard({ item, onOpen }: { item: RuntimeHistoryItem; onOpen: (id: string) => void }) {
  const residual = item.integrity_state === 'active_residual'
  return (
    <button
      type="button"
      onClick={() => onOpen(item.id)}
      className="w-full rounded-[var(--nv-radius-card)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] p-3 text-left transition-[background-color,border-color,box-shadow] hover:border-[var(--nv-border-hover)] hover:bg-[var(--nv-bg-hover)] hover:shadow-[var(--nv-shadow-card)]"
      style={residual ? { borderColor: 'color-mix(in srgb, var(--nv-status-warning) 35%, var(--nv-border-default))' } : undefined}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium text-[var(--nv-text-primary)]">
            {item.media_title || item.media_id || item.id}
          </p>
          <p className="mt-1 truncate text-xs text-[var(--nv-text-tertiary)]">
            {item.intent || '历史执行'}{item.profile_id ? ` · ${item.profile_id}` : ''}{item.last_backend ? ` · ${item.last_backend}` : ''}
          </p>
        </div>
        <Tag tone={statusTone(item.status)}>{statusText(item.status)}</Tag>
      </div>
      <div className="mt-3 grid grid-cols-3 gap-2 text-[11px] text-[var(--nv-text-tertiary)]">
        <span>{item.attempt_count} 次尝试</span>
        <span>{item.artifact_count} 个 Artifact</span>
        <span className="text-right">{formatBytes(item.artifact_bytes)}</span>
      </div>
      {(item.last_error_code || item.last_error_message || residual) && (
        <div className={`mt-2 flex items-start gap-1.5 text-[11px] ${residual ? 'text-[var(--nv-status-warning)]' : 'text-[var(--nv-status-danger)]'}`}>
          <CircleAlert size={13} className="mt-0.5 shrink-0" aria-hidden="true" />
          <span className="line-clamp-2 break-all">
            {residual
              ? '检测到旧 Runtime 活跃状态残留，维护服务会继续执行退役清扫。'
              : [item.last_error_code, item.last_error_message].filter(Boolean).join(' · ')}
          </span>
        </div>
      )}
      <p className="mt-2 text-[11px] text-[var(--nv-text-tertiary)]">{formatDate(item.completed_at || item.updated_at)}</p>
    </button>
  )
}

function SummaryCard({ icon, value, label, warning = false }: { icon: React.ReactNode; value: React.ReactNode; label: React.ReactNode; warning?: boolean }) {
  return (
    <Surface
      className="bg-[var(--nv-bg-surface-soft)] p-3 shadow-none"
      style={warning ? { borderColor: 'color-mix(in srgb, var(--nv-status-warning) 35%, var(--nv-border-default))' } : undefined}
    >
      <div className={warning ? 'text-[var(--nv-status-warning)]' : 'text-[var(--nv-action-primary)]'}>{icon}</div>
      <p className="mt-2 text-xl font-semibold text-[var(--nv-text-primary)]">{value}</p>
      <p className="text-[11px] text-[var(--nv-text-tertiary)]">{label}</p>
    </Surface>
  )
}

export default function RuntimeHistoryButton() {
  const user = useAuthStore((state) => state.user)
  const profile = useServerProfileStore((state) => state.manifest?.profile)
  const [open, setOpen] = useState(false)
  const [page, setPage] = useState(1)
  const [status, setStatus] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [search, setSearch] = useState('')
  const [list, setList] = useState<RuntimeHistoryList | null>(null)
  const [summary, setSummary] = useState<RuntimeHistorySummary | null>(null)
  const [detail, setDetail] = useState<RuntimeHistoryDetail | null>(null)
  const [loading, setLoading] = useState(false)
  const [detailLoading, setDetailLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const enabled = user?.role === 'admin'

  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const [listResponse, summaryResponse] = await Promise.all([
        runtimeHistoryApi.list({
          page,
          page_size: PAGE_SIZE,
          status: status || undefined,
          search: search || undefined,
        }),
        runtimeHistoryApi.summary(),
      ])
      setList(listResponse.data.data)
      setSummary(summaryResponse.data.data)
    } catch (loadError) {
      setError(requestError(loadError, '无法读取运行历史'))
    } finally {
      setLoading(false)
    }
  }, [page, search, status])

  useEffect(() => {
    if (open && enabled) void load()
  }, [enabled, load, open])

  const openDetail = useCallback(async (id: string) => {
    setDetailLoading(true)
    setError(null)
    try {
      const response = await runtimeHistoryApi.detail(id)
      setDetail(response.data.data)
    } catch (detailError) {
      setError(requestError(detailError, '无法读取运行历史详情'))
    } finally {
      setDetailLoading(false)
    }
  }, [])

  const submitSearch = (event: FormEvent) => {
    event.preventDefault()
    setPage(1)
    setSearch(searchInput.trim())
  }

  if (!enabled) return null

  const rightClass = profile === 'lite' ? 'right-28 md:right-32' : 'right-4 md:right-6'

  return (
    <>
      <Button
        type="button"
        variant="secondary"
        size="sm"
        onClick={() => setOpen(true)}
        className={`fixed top-14 z-40 shadow-[var(--nv-shadow-card)] backdrop-blur ${rightClass}`}
        aria-label="打开运行历史"
      >
        <Archive size={18} aria-hidden="true" />
        <span className="hidden sm:inline">历史</span>
      </Button>

      {open && (
        <div className="fixed inset-0 z-[var(--nv-z-modal)]">
          <button
            type="button"
            className="absolute inset-0 bg-[var(--nv-bg-overlay)] backdrop-blur-sm"
            aria-label="关闭运行历史"
            onClick={() => setOpen(false)}
          />
          <aside className="absolute inset-y-0 right-0 flex w-full max-w-2xl flex-col border-l border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] shadow-[var(--nv-shadow-elevated)]">
            <header className="flex items-center justify-between border-b border-[var(--nv-border-subtle)] px-5 py-4">
              <div className="flex min-w-0 items-center gap-2">
                {detail && (
                  <Button variant="ghost" size="sm" iconOnly onClick={() => setDetail(null)} aria-label="返回历史列表">
                    <ArrowLeft size={18} aria-hidden="true" />
                  </Button>
                )}
                <Archive size={19} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                <div className="min-w-0">
                  <h2 className="font-semibold text-[var(--nv-text-primary)]">{detail ? '运行历史详情' : '运行历史'}</h2>
                  <p className="mt-0.5 truncate text-xs text-[var(--nv-text-tertiary)]">只读审计域，不提供重试、取消或恢复旧 Runtime 的操作</p>
                </div>
              </div>
              <div className="flex items-center gap-1">
                {!detail && (
                  <Button variant="ghost" size="sm" iconOnly onClick={() => void load()} disabled={loading} aria-label="刷新运行历史">
                    <RefreshCw size={17} className={loading ? 'animate-spin' : ''} aria-hidden="true" />
                  </Button>
                )}
                <Button variant="ghost" size="sm" iconOnly onClick={() => setOpen(false)} aria-label="关闭运行历史">
                  <X size={19} aria-hidden="true" />
                </Button>
              </div>
            </header>

            <div className="flex-1 overflow-y-auto p-4 sm:p-5">
              {error && (
                <div className="mb-4 flex items-center gap-2 rounded-[var(--nv-radius-control)] border p-3 text-sm text-[var(--nv-status-danger)]" style={{ borderColor: 'color-mix(in srgb, var(--nv-status-danger) 25%, transparent)' }} role="alert">
                  <CircleAlert size={17} aria-hidden="true" />{error}
                </div>
              )}

              {detailLoading ? (
                <div className="flex min-h-64 items-center justify-center">
                  <Loader2 size={26} className="animate-spin text-[var(--nv-action-primary)]" aria-label="加载中" />
                </div>
              ) : detail ? (
                <div className="space-y-5">
                  <Surface className="p-4">
                    <div className="flex items-start justify-between gap-3">
                      <div>
                        <h3 className="font-medium text-[var(--nv-text-primary)]">{detail.job.media_title || detail.job.media_id}</h3>
                        <p className="mt-1 break-all text-xs text-[var(--nv-text-tertiary)]">{detail.job.id}</p>
                      </div>
                      <Tag tone={statusTone(detail.job.status)}>{statusText(detail.job.status)}</Tag>
                    </div>
                    <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 text-xs sm:grid-cols-3">
                      {[
                        ['Intent', detail.job.intent || '—'],
                        ['Profile', detail.job.profile_id || '—'],
                        ['完整性', detail.job.integrity_state],
                        ['创建时间', formatDate(detail.job.created_at)],
                        ['结束时间', formatDate(detail.job.completed_at)],
                        ['Artifact 大小', formatBytes(detail.job.artifact_bytes)],
                      ].map(([label, value]) => (
                        <div key={label}>
                          <dt className="text-[var(--nv-text-tertiary)]">{label}</dt>
                          <dd className="mt-1 break-all text-[var(--nv-text-secondary)]">{value}</dd>
                        </div>
                      ))}
                    </dl>
                  </Surface>

                  <section>
                    <h3 className="mb-2 text-xs font-semibold uppercase tracking-[.14em] text-[var(--nv-text-tertiary)]">Attempts · {detail.attempts.length}</h3>
                    <div className="space-y-2">
                      {detail.attempts.length === 0 ? (
                        <Surface className="p-4 text-sm text-[var(--nv-text-tertiary)]">没有 Attempt 记录</Surface>
                      ) : detail.attempts.map((attempt) => (
                        <Surface key={attempt.id} className="p-3">
                          <div className="flex justify-between gap-3 text-sm">
                            <span className="text-[var(--nv-text-primary)]">#{attempt.number} · {attempt.backend || 'unknown'}</span>
                            <Tag tone={statusTone(attempt.status)}>{statusText(attempt.status)}</Tag>
                          </div>
                          {(attempt.error_code || attempt.error_message) && (
                            <p className="mt-2 break-all text-xs text-[var(--nv-status-danger)]">{[attempt.error_code, attempt.error_message].filter(Boolean).join(' · ')}</p>
                          )}
                          {attempt.stderr_tail && (
                            <pre className="mt-2 max-h-36 overflow-auto whitespace-pre-wrap break-all rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)] p-2 text-[11px] text-[var(--nv-text-tertiary)]">{attempt.stderr_tail}</pre>
                          )}
                        </Surface>
                      ))}
                    </div>
                  </section>

                  <section>
                    <h3 className="mb-2 text-xs font-semibold uppercase tracking-[.14em] text-[var(--nv-text-tertiary)]">Artifacts · {detail.artifacts.length}</h3>
                    <div className="space-y-2">
                      {detail.artifacts.length === 0 ? (
                        <Surface className="p-4 text-sm text-[var(--nv-text-tertiary)]">没有 Artifact 记录</Surface>
                      ) : detail.artifacts.map((artifact) => (
                        <Surface key={artifact.id} className="p-3">
                          <div className="flex justify-between gap-3 text-sm">
                            <span className="truncate text-[var(--nv-text-primary)]">{artifact.kind}{artifact.profile_id ? ` · ${artifact.profile_id}` : ''}</span>
                            <Tag tone={statusTone(artifact.status)}>{statusText(artifact.status)}</Tag>
                          </div>
                          <div className="mt-2 flex justify-between text-xs text-[var(--nv-text-tertiary)]">
                            <span>{artifact.attestation_status || '无 attestation'}</span>
                            <span>{formatBytes(artifact.size_bytes)}</span>
                          </div>
                          {artifact.cleanup_error_code && (
                            <p className="mt-2 break-all text-xs text-[var(--nv-status-danger)]">{artifact.cleanup_error_code} · {artifact.cleanup_error_message}</p>
                          )}
                        </Surface>
                      ))}
                    </div>
                  </section>
                </div>
              ) : (
                <div className="space-y-4">
                  {summary && (
                    <section className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                      <SummaryCard icon={<Database size={16} aria-hidden="true" />} value={summary.jobs} label="Jobs" />
                      <SummaryCard icon={<Archive size={16} aria-hidden="true" />} value={summary.attempts} label="Attempts" />
                      <SummaryCard icon={<HardDrive size={16} aria-hidden="true" />} value={summary.artifacts} label={formatBytes(summary.artifact_bytes)} />
                      <SummaryCard icon={<CircleAlert size={16} aria-hidden="true" />} value={summary.orphan_legacy_tasks} label="孤立 Legacy Tasks" warning={summary.orphan_legacy_tasks > 0} />
                    </section>
                  )}

                  <form onSubmit={submitSearch} className="flex flex-wrap gap-2">
                    <div className="relative min-w-[14rem] flex-1">
                      <Search size={15} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                      <Input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索媒体 ID、Job ID、Intent…" className="pl-9" />
                    </div>
                    <Select value={status} onChange={(event) => { setStatus(event.target.value); setPage(1) }} aria-label="运行状态">
                      <option value="">全部状态</option>
                      <option value="completed">已完成</option>
                      <option value="failed">失败</option>
                      <option value="cancelled">已取消</option>
                      <option value="retired">已退役</option>
                    </Select>
                    <Button type="submit" variant="secondary">查询</Button>
                  </form>

                  {list?.retention && (
                    <Surface className="p-3 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                      元数据按审计历史长期保留，不自动删除；实际 Artifact 文件仍由 Artifact Maintenance 按磁盘压力和清理状态治理。命令行、工作目录和真实文件路径不会通过此接口返回。
                    </Surface>
                  )}

                  {loading && !list ? (
                    <div className="flex min-h-64 items-center justify-center">
                      <Loader2 size={26} className="animate-spin text-[var(--nv-action-primary)]" aria-label="加载中" />
                    </div>
                  ) : list?.items.length ? (
                    <div className="space-y-2">{list.items.map((item) => <HistoryCard key={item.id} item={item} onOpen={(id) => void openDetail(id)} />)}</div>
                  ) : (
                    <div className="flex min-h-52 flex-col items-center justify-center text-center">
                      <Archive size={34} className="mb-3 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                      <p className="text-[var(--nv-text-primary)]">没有匹配的运行历史</p>
                    </div>
                  )}

                  {list && list.total_pages > 1 && (
                    <div className="flex items-center justify-between border-t border-[var(--nv-border-subtle)] pt-4">
                      <span className="text-xs text-[var(--nv-text-tertiary)]">第 {list.page} / {list.total_pages} 页 · 共 {list.total} 条</span>
                      <div className="flex gap-2">
                        <Button variant="ghost" size="sm" iconOnly onClick={() => setPage((value) => Math.max(1, value - 1))} disabled={page <= 1 || loading} aria-label="上一页">
                          <ChevronLeft size={16} aria-hidden="true" />
                        </Button>
                        <Button variant="ghost" size="sm" iconOnly onClick={() => setPage((value) => Math.min(list.total_pages, value + 1))} disabled={page >= list.total_pages || loading} aria-label="下一页">
                          <ChevronRight size={16} aria-hidden="true" />
                        </Button>
                      </div>
                    </div>
                  )}
                </div>
              )}
            </div>
          </aside>
        </div>
      )}
    </>
  )
}
