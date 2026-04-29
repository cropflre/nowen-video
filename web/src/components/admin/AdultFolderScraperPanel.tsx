// 番号刮削 - 文件夹刮削面板（参考 mdcx）
// 功能：
//   - 选择服务器本地文件夹（复用 /admin/fs/browse 浏览器）
//   - 扫描视频并识别番号
//   - 勾选后批量刮削，结果直接落到视频旁（NFO + poster/fanart）
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
} from 'lucide-react'
import clsx from 'clsx'

// --- 文件夹浏览器（复用后端 /admin/fs/browse） ---
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
  // 文件夹浏览 state
  const [browseCur, setBrowseCur] = useState<string>('/')
  const [browseParent, setBrowseParent] = useState<string>('')
  const [browseItems, setBrowseItems] = useState<FsBrowseItem[]>([])
  const [browseLoading, setBrowseLoading] = useState(false)
  const [browseErr, setBrowseErr] = useState<string>('')

  // 扫描 state
  const [scanResult, setScanResult] = useState<FolderScanResult | null>(null)
  const [scanLoading, setScanLoading] = useState(false)
  const [recursive, setRecursive] = useState(true)
  const [maxDepth, setMaxDepth] = useState(0)
  const [filter, setFilter] = useState<'all' | 'with_code' | 'without_code' | 'undone'>('undone')

  // 选中项 state
  const [selected, setSelected] = useState<Set<string>>(new Set())

  // 任务 state
  const [tasks, setTasks] = useState<FolderBatchTask[]>([])
  const [history, setHistory] = useState<FolderBatchTask[]>([])
  const [starting, setStarting] = useState(false)
  const [opts, setOpts] = useState({
    aggregated: false,
    concurrency: 2,
    skip_if_has_nfo: true,
  })

  // --- 文件夹浏览 ---
  const loadBrowse = useCallback(async (path: string) => {
    setBrowseLoading(true)
    setBrowseErr('')
    try {
      const res = await api.get<{ data: FsBrowseResp }>('/admin/fs/browse', { params: { path } })
      setBrowseCur(res.data.data.current)
      setBrowseParent(res.data.data.parent)
      setBrowseItems(res.data.data.items || [])
    } catch (err: any) {
      setBrowseErr(err?.response?.data?.error || '浏览目录失败')
    } finally {
      setBrowseLoading(false)
    }
  }, [])

  useEffect(() => {
    loadBrowse('/')
  }, [loadBrowse])

  // --- 扫描 ---
  const handleScan = useCallback(async (path: string) => {
    setScanLoading(true)
    setSelected(new Set())
    try {
      const res = await adultScraperApi.scanFolder(path, recursive, maxDepth)
      setScanResult(res.data.data)
    } catch (err: any) {
      alert('扫描失败: ' + (err?.response?.data?.error || err?.message))
    } finally {
      setScanLoading(false)
    }
  }, [recursive, maxDepth])

  // --- 任务列表轮询 ---
  const loadTasks = useCallback(async () => {
    try {
      const res = await adultScraperApi.listFolderBatch()
      setTasks(res.data.data.active || [])
      setHistory(res.data.data.history || [])
    } catch {}
  }, [])
  useEffect(() => {
    loadTasks()
    const t = setInterval(loadTasks, 3000)
    return () => clearInterval(t)
  }, [loadTasks])

  // --- 过滤后的列表 ---
  const filtered = useMemo(() => {
    if (!scanResult) return []
    return scanResult.entries.filter((e) => {
      if (filter === 'with_code') return e.has_code
      if (filter === 'without_code') return !e.has_code
      if (filter === 'undone') return e.has_code && !e.has_nfo
      return true
    })
  }, [scanResult, filter])

  const allSelected = filtered.length > 0 && filtered.every((e) => selected.has(e.path))
  const toggleAll = () => {
    if (allSelected) {
      setSelected(new Set())
    } else {
      setSelected(new Set(filtered.map((e) => e.path)))
    }
  }
  const toggleOne = (path: string) => {
    const next = new Set(selected)
    if (next.has(path)) next.delete(path)
    else next.add(path)
    setSelected(next)
  }

  const handleStart = async () => {
    if (selected.size === 0) {
      alert('请至少选择一个视频')
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
    } catch (err: any) {
      alert('启动失败: ' + (err?.response?.data?.error || err?.message))
    } finally {
      setStarting(false)
    }
  }

  return (
    <div className="space-y-4">
      {/* 说明 */}
      <div className="rounded-lg p-3 text-xs" style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        color: 'var(--text-secondary)',
      }}>
        <strong style={{ color: 'var(--text-primary)' }}>📁 文件夹刮削（参考 mdcx）</strong>：选择服务器上任意文件夹扫描视频，
        识别番号后一键刮削；封面、剧照、NFO 会直接写入视频旁边，无需媒体库导入即可被 Emby / Jellyfin / Infuse 识别。
      </div>

      {/* ---- 文件夹浏览器 ---- */}
      <div className="rounded-lg p-4" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
        <div className="mb-3 flex items-center gap-2">
          <FolderOpen className="h-4 w-4 text-blue-500" />
          <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>选择要扫描的文件夹</span>
        </div>

        {/* 路径栏 */}
        <div className="mb-3 flex items-center gap-2">
          <button
            onClick={() => loadBrowse('/')}
            className="flex items-center gap-1 rounded px-2 py-1 text-xs hover:bg-[var(--nav-hover-bg)]"
            style={{ border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
          >
            <Home className="h-3 w-3" /> 根
          </button>
          {browseParent && (
            <button
              onClick={() => loadBrowse(browseParent)}
              className="flex items-center gap-1 rounded px-2 py-1 text-xs hover:bg-[var(--nav-hover-bg)]"
              style={{ border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}
            >
              <ArrowLeft className="h-3 w-3" /> 上级
            </button>
          )}
          <div className="flex-1 truncate rounded px-2 py-1 font-mono text-xs"
            style={{ background: 'var(--nav-hover-bg)', color: 'var(--text-primary)' }}>
            {browseCur || '/'}
          </div>
          <button
            onClick={() => handleScan(browseCur)}
            disabled={!browseCur || browseCur === '/' || scanLoading}
            className="flex items-center gap-1 rounded bg-purple-500 px-3 py-1 text-xs font-medium text-white hover:bg-purple-600 disabled:opacity-60"
          >
            {scanLoading ? <Loader2 className="h-3 w-3 animate-spin" /> : <Search className="h-3 w-3" />}
            扫描此目录
          </button>
        </div>

        {/* 目录列表 */}
        {browseErr && (
          <div className="mb-2 rounded bg-red-500/10 p-2 text-xs text-red-500">{browseErr}</div>
        )}
        <div className="max-h-48 overflow-auto rounded"
          style={{ border: '1px solid var(--border-default)' }}>
          {browseLoading ? (
            <div className="flex items-center justify-center p-4 text-xs"
              style={{ color: 'var(--text-secondary)' }}>
              <Loader2 className="mr-1 h-3 w-3 animate-spin" /> 加载中...
            </div>
          ) : browseItems.length === 0 ? (
            <div className="p-4 text-center text-xs" style={{ color: 'var(--text-secondary)' }}>
              当前目录下无子文件夹
            </div>
          ) : (
            browseItems.map((it) => (
              <button
                key={it.path}
                onClick={() => loadBrowse(it.path)}
                className="flex w-full items-center gap-2 px-3 py-1.5 text-left text-xs hover:bg-[var(--nav-hover-bg)]"
                style={{ borderBottom: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
              >
                <Folder className="h-3.5 w-3.5 text-yellow-500" />
                <span className="flex-1 truncate">{it.name}</span>
                <ChevronRight className="h-3.5 w-3.5 opacity-50" />
              </button>
            ))
          )}
        </div>

        {/* 扫描选项 */}
        <div className="mt-3 flex flex-wrap items-center gap-4 text-xs" style={{ color: 'var(--text-secondary)' }}>
          <label className="flex items-center gap-1">
            <input type="checkbox" checked={recursive} onChange={(e) => setRecursive(e.target.checked)} />
            递归扫描子目录
          </label>
          <label className="flex items-center gap-1">
            最大深度：
            <input
              type="number"
              min={0}
              max={20}
              value={maxDepth}
              onChange={(e) => setMaxDepth(Number(e.target.value))}
              className="w-12 rounded px-1 text-center"
              style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
            />
            <span className="opacity-60">（0 = 无限）</span>
          </label>
        </div>
      </div>

      {/* ---- 扫描结果 ---- */}
      {scanResult && (
        <div className="rounded-lg p-4" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
          <div className="mb-3 flex items-center gap-2 text-sm">
            <FileVideo className="h-4 w-4 text-green-500" />
            <span style={{ color: 'var(--text-primary)' }}>扫描结果</span>
            <span style={{ color: 'var(--text-secondary)' }}>
              共 {scanResult.total} 个视频 · 识别番号 {scanResult.with_code} · 未识别 {scanResult.without_code} · 已刮削 {scanResult.already_done}
            </span>
          </div>

          {/* 过滤器 */}
          <div className="mb-2 flex flex-wrap items-center gap-2 text-xs">
            <FilterBtn label="全部" active={filter === 'all'} onClick={() => setFilter('all')} />
            <FilterBtn label="仅待刮削" active={filter === 'undone'} onClick={() => setFilter('undone')} />
            <FilterBtn label="有番号" active={filter === 'with_code'} onClick={() => setFilter('with_code')} />
            <FilterBtn label="未识别" active={filter === 'without_code'} onClick={() => setFilter('without_code')} />
            <span className="ml-2" style={{ color: 'var(--text-secondary)' }}>
              已选 {selected.size} / {filtered.length}
            </span>
          </div>

          {/* 列表 */}
          <div className="max-h-80 overflow-auto rounded"
            style={{ border: '1px solid var(--border-default)' }}>
            <table className="w-full text-left text-xs">
              <thead>
                <tr style={{ background: 'var(--nav-hover-bg)' }}>
                  <th className="px-2 py-2 w-8">
                    <input type="checkbox" checked={allSelected} onChange={toggleAll} />
                  </th>
                  <th className="px-2 py-2" style={{ color: 'var(--text-secondary)' }}>文件名</th>
                  <th className="px-2 py-2 w-24" style={{ color: 'var(--text-secondary)' }}>番号</th>
                  <th className="px-2 py-2 w-20" style={{ color: 'var(--text-secondary)' }}>大小</th>
                  <th className="px-2 py-2 w-24" style={{ color: 'var(--text-secondary)' }}>状态</th>
                </tr>
              </thead>
              <tbody>
                {filtered.length === 0 && (
                  <tr>
                    <td colSpan={5} className="p-4 text-center" style={{ color: 'var(--text-secondary)' }}>
                      没有符合过滤条件的视频
                    </td>
                  </tr>
                )}
                {filtered.map((e) => (
                  <FileRow
                    key={e.path}
                    e={e}
                    checked={selected.has(e.path)}
                    onToggle={() => toggleOne(e.path)}
                  />
                ))}
              </tbody>
            </table>
          </div>

          {/* 启动按钮 */}
          <div className="mt-3 flex flex-wrap items-center gap-3">
            <label className="flex items-center gap-1 text-xs" style={{ color: 'var(--text-secondary)' }}>
              <input type="checkbox" checked={opts.aggregated}
                onChange={(e) => setOpts({ ...opts, aggregated: e.target.checked })} />
              聚合模式（精刮）
            </label>
            <label className="flex items-center gap-1 text-xs" style={{ color: 'var(--text-secondary)' }}>
              <input type="checkbox" checked={opts.skip_if_has_nfo}
                onChange={(e) => setOpts({ ...opts, skip_if_has_nfo: e.target.checked })} />
              已有 NFO 自动跳过
            </label>
            <label className="flex items-center gap-1 text-xs" style={{ color: 'var(--text-secondary)' }}>
              并发：
              <input
                type="number"
                min={1}
                max={8}
                value={opts.concurrency}
                onChange={(e) => setOpts({ ...opts, concurrency: Number(e.target.value) })}
                className="w-12 rounded px-1 text-center"
                style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
              />
            </label>
            <button
              onClick={handleStart}
              disabled={starting || selected.size === 0}
              className="flex items-center gap-1 rounded bg-purple-500 px-4 py-1.5 text-sm font-medium text-white hover:bg-purple-600 disabled:opacity-60"
            >
              {starting ? <Loader2 className="h-4 w-4 animate-spin" /> : <Play className="h-4 w-4" />}
              对 {selected.size} 个视频启动刮削
            </button>
          </div>
        </div>
      )}

      {/* ---- 任务进度 ---- */}
      {(tasks.length > 0 || history.length > 0) && (
        <div className="rounded-lg p-4"
          style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}>
          <div className="mb-3 flex items-center gap-2 text-sm">
            <Loader2 className={clsx('h-4 w-4', tasks.length > 0 ? 'animate-spin text-green-500' : 'text-gray-400')} />
            <span style={{ color: 'var(--text-primary)' }}>
              运行中 {tasks.length} · 最近 {history.length}
            </span>
            <button onClick={loadTasks}
              className="ml-auto rounded p-1 hover:bg-[var(--nav-hover-bg)]"
              style={{ border: '1px solid var(--border-default)', color: 'var(--text-secondary)' }}>
              <RefreshCw className="h-3 w-3" />
            </button>
          </div>
          <div className="space-y-2">
            {tasks.map((t) => <FolderTaskRow key={t.id} task={t} onCancel={async () => {
              await adultScraperApi.cancelFolderBatch(t.id); loadTasks()
            }} />)}
            {history.slice(0, 5).map((t) => <FolderTaskRow key={t.id} task={t} />)}
          </div>
        </div>
      )}
    </div>
  )
}

function FilterBtn({ label, active, onClick }: { label: string; active: boolean; onClick: () => void }) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'rounded px-2 py-1 transition-colors',
        active ? 'bg-purple-500/20 text-purple-500 dark:text-purple-300' : 'hover:bg-[var(--nav-hover-bg)]',
      )}
      style={!active ? { border: '1px solid var(--border-default)', color: 'var(--text-secondary)' } : undefined}
    >
      {label}
    </button>
  )
}

function FileRow({ e, checked, onToggle }: {
  e: FolderScanEntry
  checked: boolean
  onToggle: () => void
}) {
  return (
    <tr style={{ borderBottom: '1px solid var(--border-default)' }}>
      <td className="px-2 py-1.5">
        <input type="checkbox" checked={checked} onChange={onToggle} disabled={!e.has_code} />
      </td>
      <td className="truncate px-2 py-1.5 max-w-0" style={{ color: 'var(--text-primary)' }}>
        <span title={e.rel_path}>{e.filename}</span>
      </td>
      <td className="px-2 py-1.5 font-mono">
        {e.has_code ? (
          <span className="rounded bg-purple-500/20 px-1.5 py-0.5 text-purple-500 dark:text-purple-300">
            {e.detected_code}
          </span>
        ) : (
          <span style={{ color: 'var(--text-secondary)' }}>—</span>
        )}
      </td>
      <td className="px-2 py-1.5" style={{ color: 'var(--text-secondary)' }}>{e.size_mb} MB</td>
      <td className="px-2 py-1.5">
        {e.has_nfo ? (
          <span className="flex items-center gap-1 text-green-500"><CheckCircle2 className="h-3 w-3" /> 已刮削</span>
        ) : e.has_code ? (
          <span className="flex items-center gap-1" style={{ color: 'var(--text-secondary)' }}>
            <AlertCircle className="h-3 w-3" /> 待刮削
          </span>
        ) : (
          <span className="flex items-center gap-1 text-orange-500">
            <AlertCircle className="h-3 w-3" /> 无番号
          </span>
        )}
      </td>
    </tr>
  )
}

function FolderTaskRow({ task, onCancel }: { task: FolderBatchTask; onCancel?: () => void }) {
  const percent = task.total > 0 ? Math.round((task.current / task.total) * 100) : 0
  const statusColor = task.status === 'completed' ? 'text-green-500'
    : task.status === 'failed' ? 'text-red-500'
    : task.status === 'cancelled' ? 'text-gray-400'
    : 'text-blue-500'
  return (
    <div className="rounded p-2"
      style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
      <div className="flex items-center gap-2 text-xs">
        <span className={clsx('font-mono font-semibold', statusColor)}>{task.status.toUpperCase()}</span>
        <span style={{ color: 'var(--text-secondary)' }}>#{task.id.slice(0, 8)}</span>
        <span className="ml-auto" style={{ color: 'var(--text-secondary)' }}>
          {task.current}/{task.total}（✓{task.success} ✗{task.failed} ⇢{task.skipped}）
        </span>
        {onCancel && task.status === 'running' && (
          <button onClick={onCancel} className="rounded bg-red-500/20 px-2 py-0.5 text-red-500 hover:bg-red-500/30">
            取消
          </button>
        )}
      </div>
      <div className="mt-1.5 h-1 overflow-hidden rounded" style={{ background: 'var(--border-default)' }}>
        <div
          className="h-full bg-purple-500 transition-all"
          style={{ width: `${percent}%` }}
        />
      </div>
    </div>
  )
}
