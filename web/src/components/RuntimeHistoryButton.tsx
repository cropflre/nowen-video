import { FormEvent, useCallback, useEffect, useState } from 'react'
import { Archive, ArrowLeft, ChevronLeft, ChevronRight, CircleAlert, Database, HardDrive, Loader2, RefreshCw, Search, X } from 'lucide-react'
import { runtimeHistoryApi } from '@/api'
import type { RuntimeHistoryDetail, RuntimeHistoryItem, RuntimeHistoryList, RuntimeHistorySummary } from '@/api'
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
    year: 'numeric', month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit',
  }).format(date)
}

function statusText(status: string) {
  const labels: Record<string, string> = {
    completed: '已完成', failed: '失败', cancelled: '已取消', retired: '已退役',
    queued: '历史排队残留', claimed: '历史 Claim 残留', running: '历史运行残留', cancel_requested: '取消中残留',
  }
  return labels[status] || status || '未知'
}

function statusColor(status: string) {
  if (status === 'completed') return '#16A34A'
  if (status === 'failed') return '#DC2626'
  if (status === 'cancelled' || status === 'retired') return 'var(--text-muted)'
  return '#CA8A04'
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
      className="w-full rounded-xl border p-3 text-left transition-colors hover:bg-[var(--nav-hover-bg)]"
      style={{ borderColor: residual ? 'rgba(202,138,4,.35)' : 'var(--border-default)', background: 'var(--card-bg)' }}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{item.media_title || item.media_id || item.id}</p>
          <p className="mt-1 truncate text-xs" style={{ color: 'var(--text-tertiary)' }}>
            {item.intent || '历史执行'}{item.profile_id ? ` · ${item.profile_id}` : ''}{item.last_backend ? ` · ${item.last_backend}` : ''}
          </p>
        </div>
        <span className="shrink-0 text-xs font-medium" style={{ color: statusColor(item.status) }}>{statusText(item.status)}</span>
      </div>
      <div className="mt-3 grid grid-cols-3 gap-2 text-[11px]" style={{ color: 'var(--text-muted)' }}>
        <span>{item.attempt_count} 次尝试</span>
        <span>{item.artifact_count} 个 Artifact</span>
        <span className="text-right">{formatBytes(item.artifact_bytes)}</span>
      </div>
      {(item.last_error_code || item.last_error_message || residual) && (
        <div className="mt-2 flex items-start gap-1.5 text-[11px]" style={{ color: residual ? '#CA8A04' : '#DC2626' }}>
          <CircleAlert size={13} className="mt-0.5 shrink-0" />
          <span className="line-clamp-2 break-all">{residual ? '检测到旧 Runtime 活跃状态残留，维护服务会继续执行退役清扫。' : [item.last_error_code, item.last_error_message].filter(Boolean).join(' · ')}</span>
        </div>
      )}
      <p className="mt-2 text-[11px]" style={{ color: 'var(--text-muted)' }}>{formatDate(item.completed_at || item.updated_at)}</p>
    </button>
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
        runtimeHistoryApi.list({ page, page_size: PAGE_SIZE, status: status || undefined, search: search || undefined }),
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
      <button
        type="button"
        onClick={() => setOpen(true)}
        className={`fixed top-14 z-40 flex h-10 items-center gap-2 rounded-xl border px-3 text-sm font-medium shadow-lg backdrop-blur ${rightClass}`}
        style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)', color: 'var(--text-secondary)' }}
        aria-label="打开运行历史"
      >
        <Archive size={18} />
        <span className="hidden sm:inline">历史</span>
      </button>

      {open && (
        <div className="fixed inset-0 z-[110]">
          <button type="button" className="absolute inset-0 bg-black/50 backdrop-blur-sm" aria-label="关闭运行历史" onClick={() => setOpen(false)} />
          <aside className="absolute inset-y-0 right-0 flex w-full max-w-2xl flex-col border-l shadow-2xl" style={{ borderColor: 'var(--border-default)', background: 'var(--bg-base)' }}>
            <header className="flex items-center justify-between border-b px-5 py-4" style={{ borderColor: 'var(--border-default)' }}>
              <div className="flex min-w-0 items-center gap-2">
                {detail && <button type="button" onClick={() => setDetail(null)} className="rounded-lg p-1.5 hover:bg-[var(--nav-hover-bg)]" aria-label="返回历史列表"><ArrowLeft size={18} /></button>}
                <Archive size={19} className="text-neon" />
                <div className="min-w-0">
                  <h2 className="font-semibold" style={{ color: 'var(--text-primary)' }}>{detail ? '运行历史详情' : '运行历史'}</h2>
                  <p className="mt-0.5 truncate text-xs" style={{ color: 'var(--text-tertiary)' }}>只读审计域，不提供重试、取消或恢复旧 Runtime 的操作</p>
                </div>
              </div>
              <div className="flex items-center gap-1">
                {!detail && <button type="button" onClick={() => void load()} disabled={loading} className="rounded-lg p-2 hover:bg-[var(--nav-hover-bg)] disabled:opacity-50" aria-label="刷新运行历史"><RefreshCw size={17} className={loading ? 'animate-spin' : ''} /></button>}
                <button type="button" onClick={() => setOpen(false)} className="rounded-lg p-2 hover:bg-[var(--nav-hover-bg)]" aria-label="关闭运行历史"><X size={19} /></button>
              </div>
            </header>

            <div className="flex-1 overflow-y-auto p-4 sm:p-5">
              {error && <div className="mb-4 flex items-center gap-2 rounded-xl border p-3 text-sm" style={{ borderColor: 'rgba(220,38,38,.25)', color: '#DC2626' }}><CircleAlert size={17} />{error}</div>}

              {detailLoading ? (
                <div className="flex min-h-64 items-center justify-center"><Loader2 size={26} className="animate-spin text-neon" /></div>
              ) : detail ? (
                <div className="space-y-5">
                  <section className="rounded-xl border p-4" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}>
                    <div className="flex items-start justify-between gap-3">
                      <div><h3 className="font-medium" style={{ color: 'var(--text-primary)' }}>{detail.job.media_title || detail.job.media_id}</h3><p className="mt-1 break-all text-xs" style={{ color: 'var(--text-tertiary)' }}>{detail.job.id}</p></div>
                      <span className="text-sm font-medium" style={{ color: statusColor(detail.job.status) }}>{statusText(detail.job.status)}</span>
                    </div>
                    <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 text-xs sm:grid-cols-3">
                      <div><dt style={{ color: 'var(--text-muted)' }}>Intent</dt><dd className="mt-1 break-all" style={{ color: 'var(--text-secondary)' }}>{detail.job.intent || '—'}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>Profile</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{detail.job.profile_id || '—'}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>完整性</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{detail.job.integrity_state}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>创建时间</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{formatDate(detail.job.created_at)}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>结束时间</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{formatDate(detail.job.completed_at)}</dd></div>
                      <div><dt style={{ color: 'var(--text-muted)' }}>Artifact 大小</dt><dd className="mt-1" style={{ color: 'var(--text-secondary)' }}>{formatBytes(detail.job.artifact_bytes)}</dd></div>
                    </dl>
                  </section>

                  <section><h3 className="mb-2 text-xs font-semibold uppercase tracking-[.14em]" style={{ color: 'var(--text-tertiary)' }}>Attempts · {detail.attempts.length}</h3><div className="space-y-2">{detail.attempts.length === 0 ? <p className="rounded-xl border p-4 text-sm" style={{ borderColor: 'var(--border-default)', color: 'var(--text-muted)' }}>没有 Attempt 记录</p> : detail.attempts.map((attempt) => <div key={attempt.id} className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><div className="flex justify-between text-sm"><span style={{ color: 'var(--text-primary)' }}>#{attempt.number} · {attempt.backend || 'unknown'}</span><span style={{ color: statusColor(attempt.status) }}>{statusText(attempt.status)}</span></div>{(attempt.error_code || attempt.error_message) && <p className="mt-2 break-all text-xs" style={{ color: '#DC2626' }}>{[attempt.error_code, attempt.error_message].filter(Boolean).join(' · ')}</p>}{attempt.stderr_tail && <pre className="mt-2 max-h-36 overflow-auto whitespace-pre-wrap break-all rounded-lg p-2 text-[11px]" style={{ background: 'var(--nav-hover-bg)', color: 'var(--text-tertiary)' }}>{attempt.stderr_tail}</pre>}</div>)}</div></section>

                  <section><h3 className="mb-2 text-xs font-semibold uppercase tracking-[.14em]" style={{ color: 'var(--text-tertiary)' }}>Artifacts · {detail.artifacts.length}</h3><div className="space-y-2">{detail.artifacts.length === 0 ? <p className="rounded-xl border p-4 text-sm" style={{ borderColor: 'var(--border-default)', color: 'var(--text-muted)' }}>没有 Artifact 记录</p> : detail.artifacts.map((artifact) => <div key={artifact.id} className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><div className="flex justify-between gap-3 text-sm"><span className="truncate" style={{ color: 'var(--text-primary)' }}>{artifact.kind}{artifact.profile_id ? ` · ${artifact.profile_id}` : ''}</span><span className="shrink-0" style={{ color: statusColor(artifact.status) }}>{statusText(artifact.status)}</span></div><div className="mt-2 flex justify-between text-xs" style={{ color: 'var(--text-muted)' }}><span>{artifact.attestation_status || '无 attestation'}</span><span>{formatBytes(artifact.size_bytes)}</span></div>{artifact.cleanup_error_code && <p className="mt-2 break-all text-xs" style={{ color: '#DC2626' }}>{artifact.cleanup_error_code} · {artifact.cleanup_error_message}</p>}</div>)}</div></section>
                </div>
              ) : (
                <div className="space-y-4">
                  {summary && (
                    <section className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                      <div className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><Database size={16} className="text-neon" /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.jobs}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>Jobs</p></div>
                      <div className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><Archive size={16} /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.attempts}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>Attempts</p></div>
                      <div className="rounded-xl border p-3" style={{ borderColor: 'var(--border-default)', background: 'var(--card-bg)' }}><HardDrive size={16} /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.artifacts}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>{formatBytes(summary.artifact_bytes)}</p></div>
                      <div className="rounded-xl border p-3" style={{ borderColor: summary.orphan_legacy_tasks > 0 ? 'rgba(202,138,4,.35)' : 'var(--border-default)', background: 'var(--card-bg)' }}><CircleAlert size={16} /><p className="mt-2 text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>{summary.orphan_legacy_tasks}</p><p className="text-[11px]" style={{ color: 'var(--text-muted)' }}>孤立 Legacy Tasks</p></div>
                    </section>
                  )}

                  <form onSubmit={submitSearch} className="flex gap-2">
                    <div className="relative min-w-0 flex-1"><Search size={15} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: 'var(--text-muted)' }} /><input value={searchInput} onChange={(event) => setSearchInput(event.target.value)} placeholder="搜索媒体 ID、Job ID、Intent…" className="h-10 w-full rounded-xl border bg-transparent pl-9 pr-3 text-sm outline-none" style={{ borderColor: 'var(--border-default)', color: 'var(--text-primary)' }} /></div>
                    <select value={status} onChange={(event) => { setStatus(event.target.value); setPage(1) }} className="h-10 rounded-xl border bg-transparent px-3 text-sm outline-none" style={{ borderColor: 'var(--border-default)', color: 'var(--text-secondary)', background: 'var(--card-bg)' }}><option value="">全部状态</option><option value="completed">已完成</option><option value="failed">失败</option><option value="cancelled">已取消</option><option value="retired">已退役</option></select>
                    <button type="submit" className="h-10 rounded-xl border px-3 text-sm font-medium" style={{ borderColor: 'var(--border-default)', color: 'var(--text-secondary)' }}>查询</button>
                  </form>

                  {list?.retention && <div className="rounded-xl border p-3 text-xs leading-5" style={{ borderColor: 'var(--border-default)', color: 'var(--text-tertiary)' }}>元数据按审计历史长期保留，不自动删除；实际 Artifact 文件仍由 Artifact Maintenance 按磁盘压力和清理状态治理。命令行、工作目录和真实文件路径不会通过此接口返回。</div>}

                  {loading && !list ? <div className="flex min-h-64 items-center justify-center"><Loader2 size={26} className="animate-spin text-neon" /></div> : list?.items.length ? <div className="space-y-2">{list.items.map((item) => <HistoryCard key={item.id} item={item} onOpen={(id) => void openDetail(id)} />)}</div> : <div className="flex min-h-52 flex-col items-center justify-center text-center"><Archive size={34} className="mb-3" style={{ color: 'var(--text-muted)' }} /><p style={{ color: 'var(--text-primary)' }}>没有匹配的运行历史</p></div>}

                  {list && list.total_pages > 1 && <div className="flex items-center justify-between border-t pt-4" style={{ borderColor: 'var(--border-default)' }}><span className="text-xs" style={{ color: 'var(--text-muted)' }}>第 {list.page} / {list.total_pages} 页 · 共 {list.total} 条</span><div className="flex gap-2"><button type="button" onClick={() => setPage((value) => Math.max(1, value - 1))} disabled={page <= 1 || loading} className="rounded-lg border p-2 disabled:opacity-40" style={{ borderColor: 'var(--border-default)' }}><ChevronLeft size={16} /></button><button type="button" onClick={() => setPage((value) => Math.min(list.total_pages, value + 1))} disabled={page >= list.total_pages || loading} className="rounded-lg border p-2 disabled:opacity-40" style={{ borderColor: 'var(--border-default)' }}><ChevronRight size={16} /></button></div></div>}
                </div>
              )}
            </div>
          </aside>
        </div>
      )}
    </>
  )
}
