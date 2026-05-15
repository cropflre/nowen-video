import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  scanClassifyApi,
  type MediaClassification,
  type ClassificationStats,
  type ClassificationStatus,
  type ClassificationCategory,
  categoryDisplay,
  regionDisplay,
  statusDisplay,
  statusColor,
} from '@/api/scanClassify'
import { libraryApi } from '@/api'
import type { Library } from '@/types'
import {
  Layers,
  RefreshCw,
  Sparkles,
  Search,
  Loader2,
  Database,
  ShieldCheck,
  Tag,
  MapPin,
  Calendar,
  FileSignature,
  Trash2,
} from 'lucide-react'
import clsx from 'clsx'

// ==================== 扫描后处理 · 虚拟归类与命名映射 ====================
//
// 影视库扫描入库后的统一规则处理产出：
//   1. AI 智能识别  - 标题/年份/TMDb ID + 置信度
//   2. 虚拟归类     - 类别/地区/年代/类型标签/质量档/虚拟路径
//   3. 命名映射     - 仅 DB 记录的 Jellyfin/Emby 风格建议命名
//
// 重要：本页所有操作绝不修改任何磁盘文件，仅在数据库层面映射与存储。

const STATUS_OPTIONS: { value: ClassificationStatus | ''; label: string }[] = [
  { value: '', label: '全部状态' },
  { value: 'pending', label: '待处理' },
  { value: 'running', label: '处理中' },
  { value: 'processed', label: '已完成' },
  { value: 'partial', label: '部分完成' },
  { value: 'failed', label: '失败' },
  { value: 'skipped', label: '跳过' },
]

const CATEGORY_OPTIONS: { value: ClassificationCategory | ''; label: string }[] = [
  { value: '', label: '全部类别' },
  { value: 'movie', label: '电影' },
  { value: 'tvshow', label: '剧集' },
  { value: 'anime', label: '动画' },
  { value: 'documentary', label: '纪录片' },
  { value: 'variety', label: '综艺' },
  { value: 'music', label: '音乐' },
  { value: 'adult', label: '成人' },
  { value: 'other', label: '其他' },
]

const REGION_OPTIONS = [
  { value: '', label: '全部地区' },
  { value: 'CN', label: '中国大陆' },
  { value: 'HK', label: '中国香港' },
  { value: 'TW', label: '中国台湾' },
  { value: 'JP', label: '日本' },
  { value: 'KR', label: '韩国' },
  { value: 'US', label: '美国' },
  { value: 'EU', label: '欧洲' },
  { value: 'IN', label: '印度' },
  { value: 'OTHER', label: '其他' },
]

export default function ClassificationTab() {
  // ---------- 状态 ----------
  const [libraries, setLibraries] = useState<Library[]>([])
  const [libraryID, setLibraryID] = useState<string>('')
  const [status, setStatus] = useState<ClassificationStatus | ''>('')
  const [category, setCategory] = useState<ClassificationCategory | ''>('')
  const [region, setRegion] = useState<string>('')
  const [keyword, setKeyword] = useState<string>('')
  const [keywordInput, setKeywordInput] = useState<string>('')

  const [page, setPage] = useState(1)
  const [size] = useState(30)
  const [items, setItems] = useState<MediaClassification[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)

  const [stats, setStats] = useState<ClassificationStats | null>(null)
  const [reprocessing, setReprocessing] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [message, setMessage] = useState<{ type: 'ok' | 'err'; text: string } | null>(null)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / size)), [total, size])

  // ---------- 数据加载 ----------
  const loadLibraries = useCallback(async () => {
    try {
      const res = await libraryApi.list()
      setLibraries(res.data.data || [])
    } catch {
      /* 静默 */
    }
  }, [])

  const loadStats = useCallback(async () => {
    try {
      const res = await scanClassifyApi.stats(libraryID || undefined)
      setStats(res.data.data)
    } catch {
      /* 静默 */
    }
  }, [libraryID])

  const loadList = useCallback(async () => {
    setLoading(true)
    try {
      const res = await scanClassifyApi.list({
        library_id: libraryID || undefined,
        status: (status || undefined) as ClassificationStatus | undefined,
        category: (category || undefined) as ClassificationCategory | undefined,
        region: region || undefined,
        keyword: keyword || undefined,
        page,
        size,
      })
      setItems(res.data.data.items || [])
      setTotal(res.data.data.total || 0)
    } catch (e: any) {
      setMessage({ type: 'err', text: e?.response?.data?.error || '加载失败' })
    } finally {
      setLoading(false)
    }
  }, [libraryID, status, category, region, keyword, page, size])

  useEffect(() => {
    loadLibraries()
  }, [loadLibraries])

  useEffect(() => {
    loadList()
  }, [loadList])

  useEffect(() => {
    loadStats()
  }, [loadStats])

  // 搜索防抖
  useEffect(() => {
    const t = setTimeout(() => {
      setKeyword(keywordInput.trim())
      setPage(1)
    }, 400)
    return () => clearTimeout(t)
  }, [keywordInput])

  // ---------- 操作 ----------
  const handleClear = async () => {
    const label = libraryID ? '当前筛选的媒体库' : '全部'
    if (!confirm(`确定清空 ${label} 的分类记录吗？此操作不可恢复。`)) return
    setClearing(true)
    setMessage(null)
    try {
      const res = await scanClassifyApi.clear(libraryID || undefined)
      const deleted = res.data.data.deleted
      setMessage({ type: 'ok', text: `已清空 ${deleted} 条分类记录` })
      loadList()
      loadStats()
    } catch (e: any) {
      setMessage({ type: 'err', text: e?.response?.data?.error || '清空失败' })
    } finally {
      setClearing(false)
    }
  }

  const handleReprocess = async (mode: 'library' | 'all') => {
    if (mode === 'library' && !libraryID) {
      setMessage({ type: 'err', text: '请先选择媒体库' })
      return
    }
    setReprocessing(true)
    setMessage(null)
    try {
      if (mode === 'all') {
        // 整库重跑 = 不传 library_id 是不被允许的；提示用户
        setMessage({ type: 'err', text: '整库重跑请选择具体媒体库' })
        return
      }
      const res = await scanClassifyApi.reprocess({
        library_id: libraryID,
        async: true,
      })
      const data: any = res.data.data
      setMessage({
        type: 'ok',
        text: `已入队重处理 ${data?.count ?? 0} 条媒体（异步执行，稍后自动刷新）`,
      })
      // 等几秒再刷新列表 / 统计
      setTimeout(() => {
        loadList()
        loadStats()
      }, 1500)
    } catch (e: any) {
      setMessage({ type: 'err', text: e?.response?.data?.error || '重跑失败' })
    } finally {
      setReprocessing(false)
    }
  }

  // ---------- 渲染 ----------
  return (
    <div className="space-y-6">
      {/* 顶部说明 + 安全提示 */}
      <div className="glass-panel-subtle rounded-xl p-5">
        <div className="flex items-center gap-3">
          <div className="rounded-lg bg-emerald-500/10 p-2">
            <ShieldCheck className="h-5 w-5 text-emerald-400" />
          </div>
          <div>
            <h2 className="text-lg font-semibold">扫描后处理 · 虚拟归类与命名映射</h2>
            <p className="mt-1 text-sm text-surface-400">
              影视文件扫描入库后自动执行：AI 智能识别 → 虚拟归类 → Jellyfin/Emby 风格命名映射。
              <span className="font-medium text-emerald-400">所有结果仅写入数据库</span>
              ，绝不修改任何磁盘文件。
            </p>
          </div>
        </div>
      </div>

      {/* 统计卡片 */}
      {stats && (
        <div className="grid grid-cols-1 gap-4 md:grid-cols-4">
          <StatCard
            icon={<Database className="h-5 w-5" />}
            title="总记录数"
            value={stats.total}
            tint="text-blue-400 bg-blue-500/10"
          />
          <BucketCard
            icon={<Tag className="h-4 w-4" />}
            title="按类别"
            buckets={stats.by_category}
            displayMap={categoryDisplay}
          />
          <BucketCard
            icon={<MapPin className="h-4 w-4" />}
            title="按地区"
            buckets={stats.by_region}
            displayMap={regionDisplay}
          />
          <BucketCard
            icon={<Calendar className="h-4 w-4" />}
            title="按年代"
            buckets={stats.by_decade}
          />
        </div>
      )}

      {/* 过滤栏 + 操作 */}
      <div className="glass-panel-subtle rounded-xl p-4">
        <div className="grid grid-cols-1 gap-3 md:grid-cols-6">
          <Select
            value={libraryID}
            onChange={(v) => {
              setLibraryID(v)
              setPage(1)
            }}
            options={[
              { value: '', label: '全部媒体库' },
              ...libraries.map((l) => ({ value: l.id, label: l.name })),
            ]}
          />
          <Select
            value={status}
            onChange={(v) => {
              setStatus(v as ClassificationStatus | '')
              setPage(1)
            }}
            options={STATUS_OPTIONS}
          />
          <Select
            value={category}
            onChange={(v) => {
              setCategory(v as ClassificationCategory | '')
              setPage(1)
            }}
            options={CATEGORY_OPTIONS}
          />
          <Select
            value={region}
            onChange={(v) => {
              setRegion(v)
              setPage(1)
            }}
            options={REGION_OPTIONS}
          />
          <div className="md:col-span-2 flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-surface-500" />
              <input
                value={keywordInput}
                onChange={(e) => setKeywordInput(e.target.value)}
                placeholder="搜索：识别标题 / 建议命名"
                className="w-full rounded-lg border border-surface-700 bg-[var(--bg-input)] py-2 pl-9 pr-3 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
            <button
              onClick={() => {
                loadList()
                loadStats()
              }}
              className="flex items-center gap-1 rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-sm hover:border-primary-500"
              title="刷新"
            >
              <RefreshCw className={clsx('h-4 w-4', loading && 'animate-spin')} />
            </button>
          </div>
        </div>

        {/* 重跑按钮 + 清空按钮 */}
        <div className="mt-3 flex items-center gap-3 border-t border-surface-700/60 pt-3">
          <button
            onClick={() => handleReprocess('library')}
            disabled={reprocessing || !libraryID}
            className={clsx(
              'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition',
              reprocessing || !libraryID
                ? 'cursor-not-allowed bg-surface-700 text-surface-500'
                : 'bg-primary-600 text-white hover:bg-primary-500',
            )}
          >
            {reprocessing ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Sparkles className="h-4 w-4" />
            )}
            {libraryID ? `重跑该媒体库（异步）` : '请先选择媒体库'}
          </button>
          <button
            onClick={handleClear}
            disabled={clearing}
            className={clsx(
              'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-medium transition',
              clearing
                ? 'cursor-not-allowed bg-surface-700 text-surface-500'
                : 'bg-red-500/10 text-red-300 hover:bg-red-500/20',
            )}
          >
            {clearing ? (
              <Loader2 className="h-4 w-4 animate-spin" />
            ) : (
              <Trash2 className="h-4 w-4" />
            )}
            清空记录
          </button>
          <span className="text-xs text-surface-500">
            异步入队执行，规则置信度低于 0.7 时会调用 AI Fallback 兜底识别。
          </span>
        </div>

        {/* 反馈消息 */}
        {message && (
          <div
            className={clsx(
              'mt-3 rounded-lg px-3 py-2 text-sm',
              message.type === 'ok'
                ? 'bg-emerald-500/10 text-emerald-300'
                : 'bg-red-500/10 text-red-300',
            )}
          >
            {message.text}
          </div>
        )}
      </div>

      {/* 列表 */}
      <div className="glass-panel-subtle overflow-hidden rounded-xl">
        <div className="border-b border-surface-700/60 px-4 py-3 text-sm text-surface-400">
          共 <span className="font-semibold text-surface-100">{total}</span> 条记录
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-12 text-surface-400">
            <Loader2 className="mr-2 h-5 w-5 animate-spin" />
            加载中…
          </div>
        ) : items.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-12 text-surface-500">
            <Layers className="h-10 w-10 opacity-40" />
            <p className="mt-2 text-sm">暂无记录</p>
            <p className="mt-1 text-xs">
              扫描媒体库后将自动产出，或点上方"重跑"立即生成。
            </p>
          </div>
        ) : (
          <div className="divide-y divide-surface-700/60">
            {items.map((it) => (
              <Row
                key={it.id}
                item={it}
                expanded={expandedId === it.id}
                onToggle={() =>
                  setExpandedId(expandedId === it.id ? null : it.id)
                }
              />
            ))}
          </div>
        )}

        {/* 分页 */}
        {totalPages > 1 && (
          <div className="flex items-center justify-between border-t border-surface-700/60 px-4 py-3 text-sm">
            <span className="text-surface-500">
              第 {page} / {totalPages} 页
            </span>
            <div className="flex gap-2">
              <button
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
                className="rounded border border-surface-700 px-3 py-1 disabled:opacity-40"
              >
                上一页
              </button>
              <button
                disabled={page >= totalPages}
                onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                className="rounded border border-surface-700 px-3 py-1 disabled:opacity-40"
              >
                下一页
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

// ==================== 子组件 ====================

function Select({
  value,
  onChange,
  options,
}: {
  value: string
  onChange: (v: string) => void
  options: { value: string; label: string }[]
}) {
  return (
    <select
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="rounded-lg border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}

function StatCard({
  icon,
  title,
  value,
  tint,
}: {
  icon: React.ReactNode
  title: string
  value: number
  tint: string
}) {
  return (
    <div className="glass-panel rounded-xl p-4">
      <div className="flex items-center gap-3">
        <div className={clsx('rounded-lg p-2', tint)}>{icon}</div>
        <div>
          <p className="text-xs text-surface-400">{title}</p>
          <p className="text-2xl font-semibold text-theme-primary">{value.toLocaleString()}</p>
        </div>
      </div>
    </div>
  )
}

function BucketCard({
  icon,
  title,
  buckets,
  displayMap,
}: {
  icon: React.ReactNode
  title: string
  buckets: { key: string; count: number }[]
  displayMap?: Record<string, string>
}) {
  const top = buckets.slice(0, 4)
  return (
    <div className="glass-panel rounded-xl p-4">
      <div className="mb-3 flex items-center gap-2 text-sm font-medium text-surface-200">
        {icon}
        {title}
      </div>
      {top.length === 0 ? (
        <p className="text-xs text-surface-500">暂无数据</p>
      ) : (
        <ul className="space-y-1.5 text-sm">
          {top.map((b) => (
            <li key={b.key} className="flex items-center justify-between">
              <span className="text-surface-300">
                {displayMap?.[b.key] ?? b.key}
              </span>
              <span className="text-surface-400">{b.count}</span>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}

function Row({
  item,
  expanded,
  onToggle,
}: {
  item: MediaClassification
  expanded: boolean
  onToggle: () => void
}) {
  return (
    <div className="px-4 py-3 transition-colors hover:bg-[var(--nav-hover-bg)]">
      <div className="flex items-center gap-3">
        {/* 标题 + 副信息 */}
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <button
              onClick={onToggle}
              className="truncate text-left text-sm font-medium text-surface-100 hover:text-primary-300"
              title="点击展开"
            >
              {item.parsed_title || '<未识别>'}
            </button>
            {item.parsed_year > 0 && (
              <span className="rounded bg-[var(--nav-hover-bg)] px-1.5 py-0.5 text-xs text-surface-300">
                {item.parsed_year}
              </span>
            )}
            {item.parsed_tmdb_id > 0 && (
              <span className="rounded bg-blue-500/10 px-1.5 py-0.5 text-xs text-blue-300">
                TMDb {item.parsed_tmdb_id}
              </span>
            )}
            {item.ai_invoked && (
              <span
                className="rounded bg-purple-500/10 px-1.5 py-0.5 text-xs text-purple-300"
                title={item.ai_model ? `${item.ai_provider || 'AI'} · ${item.ai_model}` : 'AI'}
              >
                AI{item.ai_provider ? ` · ${item.ai_provider}` : ''}
              </span>
            )}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-surface-400">
            {item.virtual_path && (
              <span className="rounded bg-[var(--nav-hover-bg)] px-2 py-0.5">
                {item.virtual_path}
              </span>
            )}
            {item.quality_tier && (
              <span className="rounded bg-amber-500/10 px-2 py-0.5 text-amber-300">
                {item.quality_tier}
              </span>
            )}
            <span>置信度 {(item.confidence * 100).toFixed(0)}%</span>
          </div>
        </div>

        {/* 状态 */}
        <span
          className={clsx(
            'shrink-0 rounded px-2 py-0.5 text-xs font-medium',
            statusColor[item.status] || 'bg-gray-500/15 text-gray-300',
          )}
        >
          {statusDisplay[item.status] || item.status}
        </span>
      </div>

      {/* 展开详情 */}
      {expanded && (
        <div className="mt-3 grid grid-cols-1 gap-2 rounded-lg bg-[var(--nav-hover-bg)] p-3 text-xs md:grid-cols-2">
          <KV label="原文件路径" value={item.media_id} mono />
          <KV
            label="类别 / 地区 / 年代"
            value={`${categoryDisplay[item.category as string] || item.category || '-'} / ${
              regionDisplay[item.region] || item.region || '-'
            } / ${item.decade || '-'}`}
          />
          <KV label="规范化类型" value={item.genre_tags || '-'} />
          <KV label="语言" value={item.language_tag || '-'} />
          <KV
            label={
              <span className="inline-flex items-center gap-1">
                <FileSignature className="h-3 w-3" />
                建议命名（{item.naming_style || 'jellyfin'}）
              </span>
            }
            value={item.suggested_name || '-'}
            mono
          />
          <KV label="建议子目录" value={item.suggested_dir || '-'} mono />
          <div className="md:col-span-2">
            <KV
              label="建议完整路径（仅 DB 映射，不会落盘）"
              value={item.suggested_full_path || '-'}
              mono
            />
          </div>
          {item.ai_invoked && (
            <>
              <KV
                label="AI 服务商（来自 AI 配置）"
                value={item.ai_provider || '-'}
              />
              <KV label="AI 模型" value={item.ai_model || '-'} mono />
            </>
          )}
          {item.error_msg && (
            <div className="md:col-span-2">
              <KV label="错误" value={item.error_msg} mono />
            </div>
          )}
        </div>
      )}
    </div>
  )
}

function KV({
  label,
  value,
  mono,
}: {
  label: React.ReactNode
  value: string
  mono?: boolean
}) {
  return (
    <div>
      <p className="text-surface-500">{label}</p>
      <p
        className={clsx(
          'mt-0.5 break-all text-surface-200',
          mono && 'font-mono',
        )}
      >
        {value}
      </p>
    </div>
  )
}
