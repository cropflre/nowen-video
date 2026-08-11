// 番号刮削 - 文件夹刮削面板（参考 mdcx）
// 功能：选择服务器本地文件夹、扫描识别番号、批量刮削并直接落盘 NFO/图片。
import { useCallback, useEffect, useMemo, useState } from 'react'
import api from '@/api/client'
import { adultScraperApi } from '@/api'
import type { FolderBatchTask, FolderScanEntry, FolderScanResult } from '@/api/adultScraper'
import {
  Folder,
  FolderOpen,
  ChevronRight,
  RefreshCw,
  Search,
  CheckCircle2,
  AlertCircle,
  Play,
  Loader2,
  ArrowLeft,
  FileVideo,
  Home,
  XCircle,
} from 'lucide-react'
import { useDialog } from '@/components/Dialog'
import { Button, Input, Surface, Tag, type TagTone } from '@/components/design-system'

interface FsBrowseItem {
  name: string
  path: string
  is_dir: boolean
}

interface FsBrowseResp {
  current: string
  parent: string
  items: FsBrowseItem[]
}

export default function AdultFolderScraperPanel() {
  const dialog = useDialog()
  const [browseCur, setBrowseCur] = useState<string>('/')
  const [browseParent, setBrowseParent] = useState<string>('')
  const [browseItems, setBrowseItems] = useState<FsBrowseItem[]>([])
  const [browseLoading, setBrowseLoading] = useState(false)
  const [browseErr, setBrowseErr] = useState<string>('')

  const [scanResult, setScanResult] = useState<FolderScanResult | null>(null)
  const [scanLoading, setScanLoading] = useState(false)
  const [recursive, setRecursive] = useState(true)
  const [maxDepth, setMaxDepth] = useState(0)
  const [filter, setFilter] = useState<'all' | 'with_code' | 'without_code' | 'undone'>('undone')
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const [tasks, setTasks] = useState<FolderBatchTask[]>([])
  const [history, setHistory] = useState<FolderBatchTask[]>([])
  const [starting, setStarting] = useState(false)
  const [opts, setOpts] = useState({
    aggregated: false,
    concurrency: 2,
    skip_if_has_nfo: true,
  })

  const loadBrowse = useCallback(async (path: string) => {
    setBrowseLoading(true)
    setBrowseErr('')
    try {
      const response = await api.get<{ data: FsBrowseResp }>('/admin/fs/browse', { params: { path } })
      setBrowseCur(response.data.data.current)
      setBrowseParent(response.data.data.parent)
      setBrowseItems(response.data.data.items || [])
    } catch (error: any) {
      setBrowseErr(error?.response?.data?.error || '浏览目录失败')
    } finally {
      setBrowseLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadBrowse('/')
  }, [loadBrowse])

  const handleScan = useCallback(async (path: string) => {
    setScanLoading(true)
    setSelected(new Set())
    try {
      const response = await adultScraperApi.scanFolder(path, recursive, maxDepth)
      setScanResult(response.data.data)
    } catch (error: any) {
      await dialog.alert({ title: '扫描失败', message: error?.response?.data?.error || error?.message, variant: 'error' })
    } finally {
      setScanLoading(false)
    }
  }, [dialog, maxDepth, recursive])

  const loadTasks = useCallback(async () => {
    try {
      const response = await adultScraperApi.listFolderBatch()
      setTasks(response.data.data.active || [])
      setHistory(response.data.data.history || [])
    } catch {
      // 后台轮询失败时保留当前快照，下一次轮询继续尝试。
    }
  }, [])

  useEffect(() => {
    void loadTasks()
    const timer = window.setInterval(() => void loadTasks(), 3000)
    return () => window.clearInterval(timer)
  }, [loadTasks])

  const filtered = useMemo(() => {
    if (!scanResult) return []
    return scanResult.entries.filter((entry) => {
      if (filter === 'with_code') return entry.has_code
      if (filter === 'without_code') return !entry.has_code
      if (filter === 'undone') return entry.has_code && !entry.has_nfo
      return true
    })
  }, [filter, scanResult])

  const allSelected = filtered.length > 0 && filtered.every((entry) => selected.has(entry.path))

  const toggleAll = () => {
    if (allSelected) setSelected(new Set())
    else setSelected(new Set(filtered.map((entry) => entry.path)))
  }

  const toggleOne = (path: string) => {
    const next = new Set(selected)
    if (next.has(path)) next.delete(path)
    else next.add(path)
    setSelected(next)
  }

  const handleStart = async () => {
    if (selected.size === 0) {
      await dialog.alert({ title: '未选择视频', message: '请至少选择一个视频', variant: 'warning' })
      return
    }
    setStarting(true)
    try {
      await adultScraperApi.startFolderBatch({
        paths: Array.from(selected),
        aggregated: opts.aggregated,
        concurrency: opts.concurrency,
        skip_if_has_nfo: opts.skip_if_has_nfo,
      })
      await loadTasks()
      setSelected(new Set())
    } catch (error: any) {
      await dialog.alert({ title: '启动失败', message: error?.response?.data?.error || error?.message, variant: 'error' })
    } finally {
      setStarting(false)
    }
  }

  return (
    <div className="space-y-4">
      <Surface className="p-4">
        <div className="flex items-start gap-3">
          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
            <FolderOpen size={17} aria-hidden="true" />
          </div>
          <div className="min-w-0">
            <div className="text-sm font-semibold text-[var(--nv-text-primary)]">文件夹刮削（参考 mdcx）</div>
            <p className="mt-1 text-xs leading-5 text-[var(--nv-text-secondary)]">
              选择服务器上任意文件夹扫描视频，识别番号后一键刮削；封面、剧照、NFO 会直接写入视频旁边，无需媒体库导入即可被 Emby / Jellyfin / Infuse 识别。
            </p>
          </div>
        </div>
      </Surface>

      <Surface className="p-4">
        <div className="mb-3 flex items-center gap-2">
          <FolderOpen className="h-4 w-4 text-[var(--nv-action-primary)]" aria-hidden="true" />
          <span className="text-sm font-medium text-[var(--nv-text-primary)]">选择要扫描的文件夹</span>
        </div>

        <div className="mb-3 flex flex-wrap items-center gap-2">
          <Button type="button" variant="secondary" size="sm" onClick={() => void loadBrowse('/')}>
            <Home className="h-3 w-3" aria-hidden="true" />根
          </Button>
          {browseParent && (
            <Button type="button" variant="secondary" size="sm" onClick={() => void loadBrowse(browseParent)}>
              <ArrowLeft className="h-3 w-3" aria-hidden="true" />上级
            </Button>
          )}
          <div className="min-w-[12rem] flex-1 truncate rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-2 font-mono text-xs text-[var(--nv-text-secondary)]" title={browseCur || '/'}>
            {browseCur || '/'}
          </div>
          <Button type="button" variant="primary" size="sm" onClick={() => void handleScan(browseCur)} disabled={!browseCur || browseCur === '/' || scanLoading} loading={scanLoading}>
            {!scanLoading && <Search className="h-3 w-3" aria-hidden="true" />}
            扫描此目录
          </Button>
        </div>

        {browseErr && (
          <div className="mb-2 rounded-[var(--nv-radius-control)] border px-3 py-2 text-xs text-[var(--nv-status-danger)]" style={{ borderColor: 'color-mix(in srgb, var(--nv-status-danger) 28%, transparent)', background: 'color-mix(in srgb, var(--nv-status-danger) 8%, transparent)' }} role="alert">
            {browseErr}
          </div>
        )}

        <div className="max-h-48 overflow-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]">
          {browseLoading ? (
            <div className="flex items-center justify-center p-4 text-xs text-[var(--nv-text-tertiary)]">
              <Loader2 className="mr-1 h-3 w-3 animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" />加载中...
            </div>
          ) : browseItems.length === 0 ? (
            <div className="p-4 text-center text-xs text-[var(--nv-text-tertiary)]">当前目录下无子文件夹</div>
          ) : (
            browseItems.map((item) => (
              <button
                key={item.path}
                type="button"
                onClick={() => void loadBrowse(item.path)}
                className="flex w-full items-center gap-2 border-b border-[var(--nv-border-subtle)] px-3 py-2 text-left text-xs text-[var(--nv-text-primary)] transition-colors last:border-b-0 hover:bg-[var(--nv-bg-hover)]"
              >
                <Folder className="h-3.5 w-3.5 text-[var(--nv-status-warning)]" aria-hidden="true" />
                <span className="flex-1 truncate">{item.name}</span>
                <ChevronRight className="h-3.5 w-3.5 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
              </button>
            ))
          )}
        </div>

        <div className="mt-3 flex flex-wrap items-center gap-4 text-xs text-[var(--nv-text-secondary)]">
          <label className="flex items-center gap-2">
            <input type="checkbox" checked={recursive} onChange={(event) => setRecursive(event.target.checked)} className="accent-[var(--nv-action-primary)]" />
            递归扫描子目录
          </label>
          <label className="flex items-center gap-2">
            最大深度：
            <Input type="number" min={0} max={20} value={maxDepth} onChange={(event) => setMaxDepth(Number(event.target.value))} className="h-8 w-20 text-center text-xs" />
            <span className="text-[var(--nv-text-tertiary)]">（0 = 无限）</span>
          </label>
        </div>
      </Surface>

      {scanResult && (
        <Surface className="p-4">
          <div className="mb-3 flex flex-wrap items-center gap-2 text-sm">
            <FileVideo className="h-4 w-4 text-[var(--nv-status-success)]" aria-hidden="true" />
            <span className="font-medium text-[var(--nv-text-primary)]">扫描结果</span>
            <span className="text-xs text-[var(--nv-text-tertiary)]">
              共 {scanResult.total} 个视频 · 识别番号 {scanResult.with_code} · 未识别 {scanResult.without_code} · 已刮削 {scanResult.already_done}
            </span>
          </div>

          <div className="mb-3 flex flex-wrap items-center gap-2">
            <FilterBtn label="全部" active={filter === 'all'} onClick={() => setFilter('all')} />
            <FilterBtn label="仅待刮削" active={filter === 'undone'} onClick={() => setFilter('undone')} />
            <FilterBtn label="有番号" active={filter === 'with_code'} onClick={() => setFilter('with_code')} />
            <FilterBtn label="未识别" active={filter === 'without_code'} onClick={() => setFilter('without_code')} />
            <span className="ml-1 text-xs text-[var(--nv-text-tertiary)]">已选 <strong className="text-[var(--nv-action-primary)]">{selected.size}</strong> / {filtered.length}</span>
          </div>

          <div className="max-h-80 overflow-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)]">
            <table className="w-full text-left text-xs">
              <thead className="sticky top-0 z-10 bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-tertiary)]">
                <tr>
                  <th className="w-8 px-2 py-2"><input type="checkbox" checked={allSelected} onChange={toggleAll} className="accent-[var(--nv-action-primary)]" aria-label="全选当前过滤结果" /></th>
                  <th className="px-2 py-2 font-medium">文件名</th>
                  <th className="w-24 px-2 py-2 font-medium">番号</th>
                  <th className="w-20 px-2 py-2 font-medium">大小</th>
                  <th className="w-24 px-2 py-2 font-medium">状态</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--nv-border-subtle)]">
                {filtered.length === 0 && (
                  <tr><td colSpan={5} className="p-5 text-center text-[var(--nv-text-tertiary)]">没有符合过滤条件的视频</td></tr>
                )}
                {filtered.map((entry) => (
                  <FileRow key={entry.path} entry={entry} checked={selected.has(entry.path)} onToggle={() => toggleOne(entry.path)} />
                ))}
              </tbody>
            </table>
          </div>

          <div className="mt-3 flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-2 text-xs text-[var(--nv-text-secondary)]">
              <input type="checkbox" checked={opts.aggregated} onChange={(event) => setOpts({ ...opts, aggregated: event.target.checked })} className="accent-[var(--nv-action-primary)]" />
              聚合模式（精刮）
            </label>
            <label className="flex items-center gap-2 text-xs text-[var(--nv-text-secondary)]">
              <input type="checkbox" checked={opts.skip_if_has_nfo} onChange={(event) => setOpts({ ...opts, skip_if_has_nfo: event.target.checked })} className="accent-[var(--nv-action-primary)]" />
              已有 NFO 自动跳过
            </label>
            <label className="flex items-center gap-2 text-xs text-[var(--nv-text-secondary)]">
              并发：
              <Input type="number" min={1} max={8} value={opts.concurrency} onChange={(event) => setOpts({ ...opts, concurrency: Number(event.target.value) })} className="h-8 w-20 text-center text-xs" />
            </label>
            <Button type="button" variant="primary" size="sm" onClick={handleStart} disabled={starting || selected.size === 0} loading={starting}>
              {!starting && <Play className="h-4 w-4" aria-hidden="true" />}
              对 {selected.size} 个视频启动刮削
            </Button>
          </div>
        </Surface>
      )}

      {(tasks.length > 0 || history.length > 0) && (
        <Surface className="p-4">
          <div className="mb-3 flex items-center gap-2 text-sm">
            {tasks.length > 0 ? <Loader2 className="h-4 w-4 animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" /> : <CheckCircle2 className="h-4 w-4 text-[var(--nv-status-success)]" aria-hidden="true" />}
            <span className="font-medium text-[var(--nv-text-primary)]">运行中 {tasks.length} · 最近 {history.length}</span>
            <Button type="button" variant="ghost" size="sm" iconOnly className="ml-auto" onClick={() => void loadTasks()} title="刷新任务" aria-label="刷新文件夹刮削任务">
              <RefreshCw className="h-3.5 w-3.5" aria-hidden="true" />
            </Button>
          </div>
          <div className="space-y-2">
            {tasks.map((task) => (
              <FolderTaskRow
                key={task.id}
                task={task}
                onCancel={async () => {
                  await adultScraperApi.cancelFolderBatch(task.id)
                  await loadTasks()
                }}
              />
            ))}
            {history.slice(0, 5).map((task) => <FolderTaskRow key={task.id} task={task} />)}
          </div>
        </Surface>
      )}
    </div>
  )
}

function FilterBtn({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <Button type="button" variant={active ? 'primary' : 'secondary'} size="sm" aria-pressed={active} onClick={onClick} className="h-8 px-2 text-xs">
      {label}
    </Button>
  )
}

function FileRow({ entry, checked, onToggle }: { entry: FolderScanEntry; checked: boolean; onToggle: () => void }) {
  return (
    <tr className={checked ? 'bg-[var(--nv-bg-active)]' : 'hover:bg-[var(--nv-bg-hover)]'}>
      <td className="px-2 py-2">
        <input type="checkbox" checked={checked} onChange={onToggle} disabled={!entry.has_code} className="accent-[var(--nv-action-primary)] disabled:opacity-40" aria-label={`选择 ${entry.filename}`} />
      </td>
      <td className="max-w-0 truncate px-2 py-2 text-[var(--nv-text-primary)]"><span title={entry.rel_path}>{entry.filename}</span></td>
      <td className="px-2 py-2 font-mono">
        {entry.has_code ? <Tag tone="brand">{entry.detected_code}</Tag> : <span className="text-[var(--nv-text-tertiary)]">—</span>}
      </td>
      <td className="px-2 py-2 text-[var(--nv-text-tertiary)]">{entry.size_mb} MB</td>
      <td className="px-2 py-2">
        {entry.has_nfo ? (
          <span className="inline-flex items-center gap-1 text-[var(--nv-status-success)]"><CheckCircle2 className="h-3 w-3" aria-hidden="true" />已刮削</span>
        ) : entry.has_code ? (
          <span className="inline-flex items-center gap-1 text-[var(--nv-text-tertiary)]"><AlertCircle className="h-3 w-3" aria-hidden="true" />待刮削</span>
        ) : (
          <span className="inline-flex items-center gap-1 text-[var(--nv-status-warning)]"><AlertCircle className="h-3 w-3" aria-hidden="true" />无番号</span>
        )}
      </td>
    </tr>
  )
}

function taskTone(status: string): TagTone {
  if (status === 'completed') return 'success'
  if (status === 'failed') return 'danger'
  if (status === 'cancelled') return 'neutral'
  return 'brand'
}

function FolderTaskRow({ task, onCancel }: { task: FolderBatchTask; onCancel?: () => void }) {
  const percent = task.total > 0 ? Math.round((task.current / task.total) * 100) : 0
  const progressColor = task.status === 'completed'
    ? 'var(--nv-status-success)'
    : task.status === 'failed'
      ? 'var(--nv-status-danger)'
      : task.status === 'cancelled'
        ? 'var(--nv-text-tertiary)'
        : 'var(--nv-action-primary)'

  return (
    <Surface className="bg-[var(--nv-bg-surface-soft)] p-3 shadow-none">
      <div className="flex flex-wrap items-center gap-2 text-xs">
        <Tag tone={taskTone(task.status)}>{task.status.toUpperCase()}</Tag>
        <span className="font-mono text-[var(--nv-text-tertiary)]">#{task.id.slice(0, 8)}</span>
        <span className="ml-auto text-[var(--nv-text-tertiary)]">{task.current}/{task.total}（✓{task.success} ✗{task.failed} ⇢{task.skipped}）</span>
        {onCancel && task.status === 'running' && (
          <Button type="button" variant="danger" size="sm" onClick={onCancel}>
            <XCircle size={12} aria-hidden="true" />取消
          </Button>
        )}
      </div>
      <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[var(--nv-bg-control)]">
        <div className="h-full rounded-full transition-[width] duration-300" style={{ width: `${percent}%`, background: progressColor }} />
      </div>
    </Surface>
  )
}
