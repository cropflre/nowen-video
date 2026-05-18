import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import {
  scanClassifyApi,
  type MediaClassification,
  type ClassificationStats,
  type ClassificationCategory,
  categoryDisplay,
  regionDisplay,
  statusDisplay,
  statusColor,
} from '@/api/scanClassify'
import { libraryApi, aiApi } from '@/api'
import type { Library } from '@/types'
import {
  RefreshCw,
  Sparkles,
  Search,
  Loader2,
  Database,
  CheckCircle2,
  AlertTriangle,
  Bot,
  Trash2,
  Settings2,
  X,
  RotateCw,
  Pencil,
  Keyboard,
  Zap,
  ChevronDown,
} from 'lucide-react'
import clsx from 'clsx'
import { useDialog } from '@/components/Dialog'

// ==================== 智能归类（原"扫描后处理 / 虚拟归类与命名映射"） ====================
//
// 三阶段产出（仅写数据库，绝不修改任何磁盘文件）：
//   1. AI 智能识别  - 标题/年份/TMDb ID + 匹配度
//   2. 智能分类     - 类别/地区/年代/类型标签/质量档/虚拟路径
//   3. 建议名称     - Jellyfin/Emby 风格命名建议
//
// 傻瓜化设计原则：
//   - 头部行动召唤区一键可达
//   - 统计卡可点击直接筛选
//   - 默认置顶展示「待修正」让用户首屏即看到问题
//   - AI 未配置时给出明确引导
//   - 提供小白模式（默认）与专家模式切换

// ============ 常量 ============

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

// 快捷筛选标签：4 个核心场景
type QuickFilter = 'all' | 'todo' | 'done' | 'ai'

const QUICK_TABS: { key: QuickFilter; label: string; icon: React.ReactNode; tint: string }[] = [
  { key: 'all', label: '全部', icon: <Database className="h-4 w-4" />, tint: 'text-blue-300' },
  { key: 'todo', label: '⚠️ 待修正', icon: <AlertTriangle className="h-4 w-4" />, tint: 'text-red-300' },
  { key: 'done', label: '已完成', icon: <CheckCircle2 className="h-4 w-4" />, tint: 'text-emerald-300' },
  { key: 'ai', label: 'AI 识别', icon: <Bot className="h-4 w-4" />, tint: 'text-purple-300' },
]

// ============ 主组件 ============

export default function ClassificationTab() {
  const dialog = useDialog()

  // ---------- 状态 ----------
  const [libraries, setLibraries] = useState<Library[]>([])
  const [libraryID, setLibraryID] = useState<string>('')

  // 快捷筛选优先；高级筛选可展开
  const [quick, setQuick] = useState<QuickFilter>('todo') // 默认置顶「待修正」
  const [advancedOpen, setAdvancedOpen] = useState(false)
  const [category, setCategory] = useState<ClassificationCategory | ''>('')
  const [region, setRegion] = useState<string>('')
  const [keywordInput, setKeywordInput] = useState<string>('')
  const [keyword, setKeyword] = useState<string>('')

  // 模式切换：小白模式 vs 专家模式
  const [expertMode, setExpertMode] = useState<boolean>(() => {
    return localStorage.getItem('classify-expert-mode') === 'true'
  })

  // 列表 / 分页
  const [page, setPage] = useState(1)
  const [size] = useState(30)
  const [items, setItems] = useState<MediaClassification[]>([])
  const [total, setTotal] = useState(0)
  const [loading, setLoading] = useState(false)
  const [expandedId, setExpandedId] = useState<string | null>(null)

  // 统计
  const [stats, setStats] = useState<ClassificationStats | null>(null)

  // AI 状态（用于引导）
  const [aiReady, setAiReady] = useState<boolean | null>(null)

  // 操作状态
  const [reprocessing, setReprocessing] = useState(false)
  const [clearing, setClearing] = useState(false)
  const [progress, setProgress] = useState<{
    running: boolean
    queued: number
    startedAt: number
  } | null>(null)
  const [message, setMessage] = useState<{ type: 'ok' | 'err' | 'info'; text: string } | null>(null)

  // 修正弹层
  const [correctTarget, setCorrectTarget] = useState<MediaClassification | null>(null)

  // 快捷键帮助
  const [shortcutHelpOpen, setShortcutHelpOpen] = useState(false)

  // 首次访问引导
  const [tourSeen, setTourSeen] = useState<boolean>(() => {
    return localStorage.getItem('classify-tour-seen') === '1'
  })

  // 当前用户在搜索框内
  const searchRef = useRef<HTMLInputElement | null>(null)

  // ---------- 派生 ----------
  const totalPages = useMemo(() => Math.max(1, Math.ceil(total / size)), [total, size])

  // 把"快捷筛选"映射成实际请求参数
  const computeListParams = useCallback(() => {
    const base: Record<string, unknown> = {
      library_id: libraryID || undefined,
      keyword: keyword || undefined,
      page,
      size,
    }
    if (expertMode) {
      base.category = category || undefined
      base.region = region || undefined
    }
    switch (quick) {
      case 'todo':
        // 「待修正」= failed + partial（前端发两次请求合并代价过高，先按 failed 优先；专家模式可在高级筛选里手选 partial）
        base.status = 'failed'
        break
      case 'done':
        base.status = 'processed'
        break
      case 'ai':
        // 「AI 识别」目前没有专属字段索引，先按 processed 取，再前端过滤 ai_invoked
        base.status = 'processed'
        break
      case 'all':
      default:
        break
    }
    return base
  }, [libraryID, keyword, page, size, expertMode, category, region, quick])

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

  const loadAIStatus = useCallback(async () => {
    try {
      const res = await aiApi.getStatus()
      const st = res.data.data as unknown as {
        api_configured?: boolean
        enabled?: boolean
        auto_pilot?: boolean
      }
      setAiReady(Boolean(st?.api_configured && (st?.enabled || st?.auto_pilot)))
    } catch {
      setAiReady(false)
    }
  }, [])

  const loadList = useCallback(async () => {
    setLoading(true)
    try {
      const res = await scanClassifyApi.list(computeListParams())
      let arr = res.data.data.items || []
      // 「AI 识别」标签：前端二次过滤
      if (quick === 'ai') {
        arr = arr.filter((it) => it.ai_invoked)
      }
      setItems(arr)
      setTotal(res.data.data.total || 0)
    } catch (e) {
      const err = e as { response?: { data?: { error?: string } } }
      setMessage({ type: 'err', text: err?.response?.data?.error || '加载失败' })
    } finally {
      setLoading(false)
    }
  }, [computeListParams, quick])

  useEffect(() => {
    loadLibraries()
    loadAIStatus()
  }, [loadLibraries, loadAIStatus])

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

  // 异步进度轮询（粗略估算）：进入 progress 状态后每 2s 刷一次列表 + 统计，持续 90s
  useEffect(() => {
    if (!progress?.running) return
    const startedAt = progress.startedAt
    const interval = setInterval(() => {
      const elapsed = Date.now() - startedAt
      // 90s 后强制结束（用户想再看进度可手动刷新）
      if (elapsed > 90_000) {
        setProgress(null)
        clearInterval(interval)
        return
      }
      loadList()
      loadStats()
    }, 2500)
    return () => clearInterval(interval)
  }, [progress, loadList, loadStats])

  // 切库 / 切快捷标签 → page 重置
  useEffect(() => {
    setPage(1)
  }, [libraryID, quick, category, region])

  // 持久化模式
  useEffect(() => {
    localStorage.setItem('classify-expert-mode', expertMode ? 'true' : 'false')
  }, [expertMode])

  // 快捷键
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      // 焦点在输入框时除 ESC 外不响应快捷键
      const target = e.target as HTMLElement | null
      const inField =
        !!target && (target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable)
      if (e.key === 'Escape') {
        setCorrectTarget(null)
        setShortcutHelpOpen(false)
        setExpandedId(null)
        return
      }
      if (inField) return
      if (e.key === '?' || (e.shiftKey && e.key === '/')) {
        setShortcutHelpOpen((v) => !v)
        return
      }
      if (e.key === 'r' || e.key === 'R') {
        handleReprocess()
        return
      }
      if (e.key === 'f' || e.key === 'F') {
        e.preventDefault()
        searchRef.current?.focus()
        return
      }
      if (e.key === '1') setQuick('all')
      else if (e.key === '2') setQuick('todo')
      else if (e.key === '3') setQuick('done')
      else if (e.key === '4') setQuick('ai')
    }
    window.addEventListener('keydown', h)
    return () => window.removeEventListener('keydown', h)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [libraryID])

  // ---------- 操作 ----------

  // 一键重跑：未选库 = 全部媒体库，已选库 = 该库
  const handleReprocess = async () => {
    if (reprocessing) return
    if (aiReady === false) {
      const ok = await dialog.confirm({
        title: 'AI 未配置',
        message:
          '未配置云端 AI 服务时，将仅使用规则识别（准确率约 70%）。\n配置 AI 后可达 95%+。\n\n是否仍要继续？',
        confirmText: '仍要继续',
        cancelText: '去配置 AI',
      })
      if (!ok) {
        // 跳到 AI Tab（通过 hash）
        window.location.hash = '#ai'
        return
      }
    }

    setReprocessing(true)
    setMessage(null)
    try {
      const res = await scanClassifyApi.reprocess({
        library_id: libraryID || undefined,
        async: true,
      })
      const data = res.data.data
      const targetText = libraryID
        ? `《${libraries.find((l) => l.id === libraryID)?.name || '当前库'}》`
        : '全部媒体库'
      setMessage({
        type: 'ok',
        text: `已入队 ${targetText} 的 ${data?.count ?? 0} 条媒体（异步执行，列表将自动刷新）`,
      })
      setProgress({ running: true, queued: data?.count ?? 0, startedAt: Date.now() })
    } catch (e) {
      const err = e as { response?: { data?: { error?: string } } }
      setMessage({ type: 'err', text: err?.response?.data?.error || '重跑失败' })
    } finally {
      setReprocessing(false)
    }
  }

  // 单条重试
  const handleRetryOne = async (mediaId: string) => {
    setMessage(null)
    try {
      await scanClassifyApi.reprocess({ media_ids: [mediaId] })
      setMessage({ type: 'ok', text: '已重新识别该条' })
      loadList()
      loadStats()
    } catch (e) {
      const err = e as { response?: { data?: { error?: string } } }
      setMessage({ type: 'err', text: err?.response?.data?.error || '重试失败' })
    }
  }

  // 清空 - 二级保险
  const handleClear = async () => {
    const label = libraryID ? '当前筛选的媒体库' : '【全部】'
    const txt = await dialog.prompt({
      title: '⚠️ 危险操作 · 删除归类记录',
      message: `即将清空 ${label} 的归类记录。此操作不可恢复。\n\n请输入 DELETE 以确认：`,
      placeholder: 'DELETE',
      confirmText: '确认清空',
    })
    if (!txt || txt.trim().toUpperCase() !== 'DELETE') {
      if (txt !== null) setMessage({ type: 'err', text: '输入不匹配，已取消' })
      return
    }
    setClearing(true)
    setMessage(null)
    try {
      const res = await scanClassifyApi.clear(libraryID || undefined)
      setMessage({ type: 'ok', text: `已清空 ${res.data.data.deleted} 条记录` })
      loadList()
      loadStats()
    } catch (e) {
      const err = e as { response?: { data?: { error?: string } } }
      setMessage({ type: 'err', text: err?.response?.data?.error || '清空失败' })
    } finally {
      setClearing(false)
    }
  }

  // 提交修正
  const handleCorrectSubmit = async (
    item: MediaClassification,
    payload: {
      title?: string
      year?: number
      tmdb_id?: number
      imdb_id?: string
      category?: string
      region?: string
    },
  ) => {
    try {
      await scanClassifyApi.correct({ media_id: item.media_id, ...payload })
      setMessage({ type: 'ok', text: `已修正：${payload.title || item.parsed_title}` })
      setCorrectTarget(null)
      loadList()
      loadStats()
    } catch (e) {
      const err = e as { response?: { data?: { error?: string } } }
      setMessage({ type: 'err', text: err?.response?.data?.error || '修正失败' })
    }
  }

  // 关闭引导
  const dismissTour = () => {
    localStorage.setItem('classify-tour-seen', '1')
    setTourSeen(true)
  }

  // ---------- 渲染 ----------
  const targetLibName = libraryID ? libraries.find((l) => l.id === libraryID)?.name : ''

  // 派生统计
  const failedCount = useMemo(() => {
    const b = stats?.by_status?.find((s) => s.key === 'failed')
    const p = stats?.by_status?.find((s) => s.key === 'partial')
    return (b?.count || 0) + (p?.count || 0)
  }, [stats])
  const doneCount = useMemo(() => {
    return stats?.by_status?.find((s) => s.key === 'processed')?.count || 0
  }, [stats])
  const aiCount = useMemo(() => {
    // 后端没有 by_ai_invoked，先用 (total - rule-only) 的近似（无 by_ai_invoked 时显示 ?）
    return null as number | null
  }, [])

  return (
    <div className="space-y-5">
      {/* ============ 行动召唤区 ============ */}
      <div className="glass-panel rounded-xl p-5">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
          <div className="flex items-center gap-3">
            <div className="rounded-lg bg-primary-500/15 p-2.5">
              <Sparkles className="h-5 w-5 text-primary-300" />
            </div>
            <div>
              <h2 className="text-lg font-semibold">智能归类</h2>
              <p className="mt-0.5 text-xs text-surface-400">
                AI 识别 → 智能分类 → 建议名称（仅写数据库，不动磁盘）
              </p>
            </div>
          </div>

          {/* 右上：库选择 + 一键重跑 + 模式切换 */}
          <div className="flex flex-wrap items-center gap-2">
            <select
              value={libraryID}
              onChange={(e) => setLibraryID(e.target.value)}
              className="rounded-lg border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
            >
              <option value="">全部媒体库</option>
              {libraries.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.name}
                </option>
              ))}
            </select>
            <button
              onClick={handleReprocess}
              disabled={reprocessing}
              className={clsx(
                'flex items-center gap-2 rounded-lg px-4 py-2 text-sm font-semibold transition',
                reprocessing
                  ? 'cursor-not-allowed bg-surface-700 text-surface-500'
                  : 'bg-primary-600 text-white shadow-lg shadow-primary-500/20 hover:bg-primary-500',
              )}
              title="按 R 键快捷触发"
            >
              {reprocessing ? <Loader2 className="h-4 w-4 animate-spin" /> : <Zap className="h-4 w-4" />}
              {libraryID ? `重跑《${targetLibName || '当前库'}》` : '🚀 重跑全部'}
            </button>
            <button
              onClick={() => setExpertMode((v) => !v)}
              className="flex items-center gap-1.5 rounded-lg border border-surface-700 bg-surface-900 px-3 py-2 text-xs text-surface-300 hover:border-primary-500"
              title={expertMode ? '切换到小白模式（默认）' : '切换到专家模式'}
            >
              <Settings2 className="h-3.5 w-3.5" />
              {expertMode ? '专家' : '小白'}
            </button>
            <button
              onClick={() => setShortcutHelpOpen(true)}
              className="rounded-lg border border-surface-700 bg-surface-900 p-2 text-surface-300 hover:border-primary-500"
              title="键盘快捷键 (?)"
            >
              <Keyboard className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        {/* AI 未配置黄条 */}
        {aiReady === false && (
          <div className="mt-3 flex items-center justify-between rounded-lg border border-amber-500/30 bg-amber-500/10 px-3 py-2 text-sm text-amber-200">
            <div className="flex items-center gap-2">
              <AlertTriangle className="h-4 w-4 shrink-0" />
              <span>
                <span className="font-medium">AI 未配置</span> · 当前仅使用规则识别（准确率约 70%）。
                配置云端 AI 后可达 95%+。
              </span>
            </div>
            <button
              onClick={() => {
                window.location.hash = '#ai'
              }}
              className="shrink-0 rounded-md bg-amber-500/20 px-3 py-1 text-xs font-medium text-amber-100 hover:bg-amber-500/30"
            >
              立即配置 →
            </button>
          </div>
        )}

        {/* 异步进度条 */}
        {progress?.running && (
          <div className="mt-3 rounded-lg border border-primary-500/30 bg-primary-500/10 px-3 py-2 text-sm">
            <div className="flex items-center justify-between text-primary-200">
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>
                  正在后台处理 <span className="font-semibold">{progress.queued}</span> 条媒体…
                </span>
              </div>
              <button
                onClick={() => setProgress(null)}
                className="text-xs text-primary-200 hover:text-white"
              >
                关闭进度
              </button>
            </div>
            <p className="mt-1 text-xs text-primary-300/70">
              页面将每 2.5 秒自动刷新一次列表与统计；可继续操作其他功能。
            </p>
          </div>
        )}

        {/* 引导提示（首次访问） */}
        {!tourSeen && (
          <div className="mt-3 flex items-start justify-between rounded-lg border border-blue-500/30 bg-blue-500/10 px-3 py-2 text-sm text-blue-200">
            <div className="flex items-start gap-2">
              <Bot className="mt-0.5 h-4 w-4 shrink-0" />
              <div>
                <p className="font-medium">第一次来？三步上手：</p>
                <ol className="mt-1 list-inside list-decimal text-xs text-blue-300/90">
                  <li>下方统计卡点「⚠️ 待修正」直接定位需要处理的条目</li>
                  <li>点条目右侧「修改」即可在本页直接修正识别结果</li>
                  <li>识别错误的整库都可点上方「🚀 重跑全部」让 AI 重新识别</li>
                </ol>
              </div>
            </div>
            <button onClick={dismissTour} className="shrink-0 text-blue-200 hover:text-white">
              <X className="h-4 w-4" />
            </button>
          </div>
        )}
      </div>

      {/* ============ 统计卡（可点击筛选） ============ */}
      <div className="grid grid-cols-2 gap-3 md:grid-cols-4">
        <ClickableStatCard
          icon={<Database className="h-5 w-5" />}
          title="总数"
          value={stats?.total ?? 0}
          tint="text-blue-300 bg-blue-500/10"
          active={quick === 'all'}
          onClick={() => setQuick('all')}
        />
        <ClickableStatCard
          icon={<AlertTriangle className="h-5 w-5" />}
          title="待修正"
          value={failedCount}
          tint="text-red-300 bg-red-500/10"
          active={quick === 'todo'}
          highlight={failedCount > 0}
          onClick={() => setQuick('todo')}
        />
        <ClickableStatCard
          icon={<CheckCircle2 className="h-5 w-5" />}
          title="已完成"
          value={doneCount}
          tint="text-emerald-300 bg-emerald-500/10"
          active={quick === 'done'}
          onClick={() => setQuick('done')}
        />
        <ClickableStatCard
          icon={<Bot className="h-5 w-5" />}
          title="AI 识别"
          value={aiCount ?? '—'}
          tint="text-purple-300 bg-purple-500/10"
          active={quick === 'ai'}
          onClick={() => setQuick('ai')}
          subtitle={aiReady === false ? '未配置' : ''}
        />
      </div>

      {/* ============ 快捷标签 + 搜索 ============ */}
      <div className="glass-panel-subtle rounded-xl p-3">
        <div className="flex flex-wrap items-center gap-2">
          {/* 快捷标签 */}
          <div className="flex items-center gap-1.5">
            {QUICK_TABS.map((t) => (
              <button
                key={t.key}
                onClick={() => setQuick(t.key)}
                className={clsx(
                  'flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs transition',
                  quick === t.key
                    ? 'bg-primary-600 text-white'
                    : 'border border-surface-700 bg-surface-900 text-surface-300 hover:border-primary-500',
                )}
              >
                <span className={quick === t.key ? '' : t.tint}>{t.icon}</span>
                {t.label}
              </button>
            ))}
          </div>

          <div className="ml-auto flex items-center gap-2">
            <div className="relative">
              <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-surface-500" />
              <input
                ref={searchRef}
                value={keywordInput}
                onChange={(e) => setKeywordInput(e.target.value)}
                placeholder="搜索标题 / 建议名称（F）"
                className="w-56 rounded-md border border-surface-700 bg-[var(--bg-input)] py-1.5 pl-8 pr-3 text-sm focus:border-primary-500 focus:outline-none"
              />
            </div>
            <button
              onClick={() => {
                loadList()
                loadStats()
              }}
              className="rounded-md border border-surface-700 bg-surface-900 p-2 text-surface-300 hover:border-primary-500"
              title="刷新"
            >
              <RefreshCw className={clsx('h-3.5 w-3.5', loading && 'animate-spin')} />
            </button>
            {expertMode && (
              <button
                onClick={() => setAdvancedOpen((v) => !v)}
                className="flex items-center gap-1 rounded-md border border-surface-700 bg-surface-900 px-3 py-1.5 text-xs text-surface-300 hover:border-primary-500"
              >
                高级筛选
                <ChevronDown
                  className={clsx('h-3 w-3 transition-transform', advancedOpen && 'rotate-180')}
                />
              </button>
            )}
          </div>
        </div>

        {/* 高级筛选（仅专家模式 + 展开） */}
        {expertMode && advancedOpen && (
          <div className="mt-3 grid grid-cols-2 gap-2 border-t border-surface-700/60 pt-3 md:grid-cols-4">
            <Select
              value={category}
              onChange={(v) => setCategory(v as ClassificationCategory | '')}
              options={CATEGORY_OPTIONS}
            />
            <Select value={region} onChange={(v) => setRegion(v)} options={REGION_OPTIONS} />
            <button
              onClick={handleClear}
              disabled={clearing}
              className={clsx(
                'flex items-center justify-center gap-2 rounded-md px-3 py-2 text-xs font-medium transition',
                clearing
                  ? 'cursor-not-allowed bg-surface-700 text-surface-500'
                  : 'bg-red-500/10 text-red-300 hover:bg-red-500/20',
              )}
            >
              {clearing ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Trash2 className="h-3.5 w-3.5" />}
              ⚠️ 删除归类记录
            </button>
          </div>
        )}

        {message && (
          <div
            className={clsx(
              'mt-3 flex items-center justify-between rounded-md px-3 py-2 text-sm',
              message.type === 'ok' && 'bg-emerald-500/10 text-emerald-200',
              message.type === 'err' && 'bg-red-500/10 text-red-200',
              message.type === 'info' && 'bg-blue-500/10 text-blue-200',
            )}
          >
            <span>{message.text}</span>
            <button onClick={() => setMessage(null)}>
              <X className="h-3.5 w-3.5" />
            </button>
          </div>
        )}
      </div>

      {/* ============ 列表 ============ */}
      <div className="glass-panel-subtle overflow-hidden rounded-xl">
        <div className="border-b border-surface-700/60 px-4 py-2.5 text-xs text-surface-400">
          共 <span className="font-semibold text-surface-100">{total}</span> 条
          {quick === 'todo' && <span className="ml-2 text-red-300">· 仅显示待修正</span>}
          {libraryID && (
            <span className="ml-2 text-surface-300">· 库：{targetLibName}</span>
          )}
        </div>

        {loading ? (
          <div className="flex items-center justify-center py-16 text-surface-400">
            <Loader2 className="mr-2 h-5 w-5 animate-spin" />
            加载中…
          </div>
        ) : items.length === 0 ? (
          <EmptyState quick={quick} onReprocess={handleReprocess} />
        ) : (
          <div className="divide-y divide-surface-700/60">
            {items.map((it) => (
              <Row
                key={it.id}
                item={it}
                expertMode={expertMode}
                expanded={expandedId === it.id}
                onToggle={() => setExpandedId(expandedId === it.id ? null : it.id)}
                onCorrect={() => setCorrectTarget(it)}
                onRetry={() => handleRetryOne(it.media_id)}
              />
            ))}
          </div>
        )}

        {totalPages > 1 && (
          <div className="flex items-center justify-between border-t border-surface-700/60 px-4 py-2 text-xs">
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

      {/* 修正弹层 */}
      {correctTarget && (
        <CorrectModal
          item={correctTarget}
          onCancel={() => setCorrectTarget(null)}
          onSubmit={handleCorrectSubmit}
        />
      )}

      {/* 快捷键帮助 */}
      {shortcutHelpOpen && (
        <ShortcutHelpModal onClose={() => setShortcutHelpOpen(false)} />
      )}
    </div>
  )
}

// ============ 子组件 ============

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
      className="rounded-md border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-xs focus:border-primary-500 focus:outline-none"
    >
      {options.map((o) => (
        <option key={o.value} value={o.value}>
          {o.label}
        </option>
      ))}
    </select>
  )
}

function ClickableStatCard({
  icon,
  title,
  value,
  tint,
  active,
  highlight,
  onClick,
  subtitle,
}: {
  icon: React.ReactNode
  title: string
  value: number | string
  tint: string
  active?: boolean
  highlight?: boolean
  onClick?: () => void
  subtitle?: string
}) {
  return (
    <button
      onClick={onClick}
      className={clsx(
        'group glass-panel rounded-xl p-4 text-left transition-all',
        active && 'ring-2 ring-primary-500',
        highlight && !active && 'ring-1 ring-red-500/40',
        onClick && 'hover:scale-[1.02]',
      )}
    >
      <div className="flex items-center gap-3">
        <div className={clsx('rounded-lg p-2', tint)}>{icon}</div>
        <div className="min-w-0 flex-1">
          <p className="text-xs text-surface-400">
            {title}
            {subtitle && <span className="ml-1.5 text-amber-300">· {subtitle}</span>}
          </p>
          <p className="text-2xl font-semibold text-theme-primary">
            {typeof value === 'number' ? value.toLocaleString() : value}
          </p>
        </div>
      </div>
    </button>
  )
}

function EmptyState({
  quick,
  onReprocess,
}: {
  quick: QuickFilter
  onReprocess: () => void
}) {
  if (quick === 'todo') {
    return (
      <div className="flex flex-col items-center justify-center py-16 text-surface-500">
        <CheckCircle2 className="h-10 w-10 text-emerald-400 opacity-60" />
        <p className="mt-2 text-sm font-medium text-emerald-300">没有待修正的条目 🎉</p>
        <p className="mt-1 text-xs">所有媒体识别均已完成</p>
      </div>
    )
  }
  return (
    <div className="flex flex-col items-center justify-center py-16 text-surface-500">
      <Database className="h-10 w-10 opacity-40" />
      <p className="mt-2 text-sm">暂无记录</p>
      <p className="mt-1 text-xs">扫描媒体库后将自动产出识别结果</p>
      <button
        onClick={onReprocess}
        className="mt-4 flex items-center gap-2 rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-500"
      >
        <Zap className="h-4 w-4" />
        立即开始识别
      </button>
    </div>
  )
}

function Row({
  item,
  expertMode,
  expanded,
  onToggle,
  onCorrect,
  onRetry,
}: {
  item: MediaClassification
  expertMode: boolean
  expanded: boolean
  onToggle: () => void
  onCorrect: () => void
  onRetry: () => void
}) {
  const isFailed = item.status === 'failed' || item.status === 'partial'
  return (
    <div
      className={clsx(
        'group px-4 py-3 transition-colors hover:bg-[var(--nav-hover-bg)]',
        isFailed && 'bg-red-500/5',
      )}
    >
      <div className="flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2">
            <button
              onClick={onToggle}
              className="truncate text-left text-sm font-medium text-surface-100 hover:text-primary-300"
              title="点击展开详情"
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
                <Bot className="mr-0.5 inline h-3 w-3" />
                AI
              </span>
            )}
          </div>
          <div className="mt-1 flex flex-wrap items-center gap-2 text-xs text-surface-400">
            <span>{categoryDisplay[item.category as string] || item.category || '—'}</span>
            {item.region && <span>· {regionDisplay[item.region] || item.region}</span>}
            {item.decade && <span>· {item.decade}</span>}
            {item.quality_tier && (
              <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-300">
                {item.quality_tier}
              </span>
            )}
            <span className={clsx(item.confidence < 0.6 && 'text-amber-300')}>
              匹配度 {(item.confidence * 100).toFixed(0)}%
            </span>
          </div>
        </div>

        {/* 状态标签 */}
        <span
          className={clsx(
            'shrink-0 rounded px-2 py-0.5 text-xs font-medium',
            statusColor[item.status] || 'bg-gray-500/15 text-gray-300',
          )}
        >
          {statusDisplay[item.status] || item.status}
        </span>

        {/* 行内操作（hover 显示） */}
        <div className="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
          <button
            onClick={onCorrect}
            className="rounded-md border border-surface-700 bg-surface-900 px-2 py-1 text-xs text-surface-300 hover:border-primary-500 hover:text-primary-300"
            title="修改识别结果"
          >
            <Pencil className="mr-1 inline h-3 w-3" />
            修改
          </button>
          <button
            onClick={onRetry}
            className="rounded-md border border-surface-700 bg-surface-900 px-2 py-1 text-xs text-surface-300 hover:border-primary-500 hover:text-primary-300"
            title="重新识别该条"
          >
            <RotateCw className="mr-1 inline h-3 w-3" />
            重试
          </button>
        </div>
      </div>

      {/* 展开详情：核心 4 项 + 完整折叠（专家模式才显示） */}
      {expanded && (
        <div className="mt-3 grid grid-cols-1 gap-2 rounded-lg bg-[var(--nav-hover-bg)] p-3 text-xs md:grid-cols-2">
          <KV label="原文件路径" value={item.media_id} mono />
          <KV
            label="建议名称"
            value={item.suggested_name || '—'}
            mono
          />
          <div className="md:col-span-2">
            <KV
              label="建议路径（仅 DB 映射，不会落盘）"
              value={item.suggested_full_path || '—'}
              mono
            />
          </div>
          {item.error_msg && (
            <div className="md:col-span-2">
              <KV label="错误信息" value={item.error_msg} mono />
            </div>
          )}
          {expertMode && (
            <>
              <KV label="规范化类型" value={item.genre_tags || '—'} />
              <KV label="语言" value={item.language_tag || '—'} />
              <KV label="建议子目录" value={item.suggested_dir || '—'} mono />
              <KV label="命名风格" value={item.naming_style || 'jellyfin'} />
              {item.ai_invoked && (
                <>
                  <KV label="AI 服务商" value={item.ai_provider || '—'} />
                  <KV label="AI 模型" value={item.ai_model || '—'} mono />
                </>
              )}
            </>
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
      <p className={clsx('mt-0.5 break-all text-surface-200', mono && 'font-mono')}>{value}</p>
    </div>
  )
}

// ============ 修正弹层 ============

function CorrectModal({
  item,
  onCancel,
  onSubmit,
}: {
  item: MediaClassification
  onCancel: () => void
  onSubmit: (
    item: MediaClassification,
    payload: {
      title?: string
      year?: number
      tmdb_id?: number
      imdb_id?: string
      category?: string
      region?: string
    },
  ) => void
}) {
  const [title, setTitle] = useState(item.parsed_title || '')
  const [year, setYear] = useState(String(item.parsed_year || ''))
  const [tmdbId, setTmdbId] = useState(String(item.parsed_tmdb_id || ''))
  const [imdbId, setImdbId] = useState(item.parsed_imdb_id || '')
  const [category, setCategory] = useState(item.category || '')
  const [region, setRegion] = useState(item.region || '')

  const handleSave = () => {
    onSubmit(item, {
      title: title.trim() || undefined,
      year: parseInt(year, 10) || 0,
      tmdb_id: parseInt(tmdbId, 10) || 0,
      imdb_id: imdbId.trim() || undefined,
      category: category || undefined,
      region: region || undefined,
    })
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onCancel}
    >
      <div
        className="glass-panel max-h-[90vh] w-full max-w-lg overflow-y-auto rounded-xl p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold">修正识别结果</h3>
          <button onClick={onCancel} className="text-surface-400 hover:text-white">
            <X className="h-4 w-4" />
          </button>
        </div>
        <p className="mt-1 text-xs text-surface-400">
          所有修改仅写入数据库，不会修改任何磁盘文件。
        </p>

        <div className="mt-4 space-y-3">
          <Field label="标题">
            <input
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              className="w-full rounded-md border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="年份">
              <input
                value={year}
                onChange={(e) => setYear(e.target.value.replace(/\D/g, ''))}
                className="w-full rounded-md border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
                placeholder="如 2024"
              />
            </Field>
            <Field label="TMDb ID">
              <input
                value={tmdbId}
                onChange={(e) => setTmdbId(e.target.value.replace(/\D/g, ''))}
                className="w-full rounded-md border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
                placeholder="如 12345"
              />
            </Field>
          </div>
          <Field label="IMDb ID">
            <input
              value={imdbId}
              onChange={(e) => setImdbId(e.target.value)}
              className="w-full rounded-md border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
              placeholder="如 tt0133093"
            />
          </Field>
          <div className="grid grid-cols-2 gap-3">
            <Field label="类别">
              <select
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                className="w-full rounded-md border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
              >
                {CATEGORY_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="地区">
              <select
                value={region}
                onChange={(e) => setRegion(e.target.value)}
                className="w-full rounded-md border border-surface-700 bg-[var(--bg-input)] px-3 py-2 text-sm focus:border-primary-500 focus:outline-none"
              >
                {REGION_OPTIONS.map((o) => (
                  <option key={o.value} value={o.value}>
                    {o.label}
                  </option>
                ))}
              </select>
            </Field>
          </div>
        </div>

        <div className="mt-5 flex justify-end gap-2">
          <button
            onClick={onCancel}
            className="rounded-lg border border-surface-700 px-4 py-2 text-sm text-surface-300 hover:border-primary-500"
          >
            取消
          </button>
          <button
            onClick={handleSave}
            className="rounded-lg bg-primary-600 px-4 py-2 text-sm font-medium text-white hover:bg-primary-500"
          >
            保存修正
          </button>
        </div>
      </div>
    </div>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="mb-1 block text-xs text-surface-400">{label}</label>
      {children}
    </div>
  )
}

// ============ 快捷键帮助 ============

function ShortcutHelpModal({ onClose }: { onClose: () => void }) {
  const items: { key: string; desc: string }[] = [
    { key: 'R', desc: '一键重跑（按当前已选媒体库）' },
    { key: 'F', desc: '聚焦搜索框' },
    { key: '1', desc: '切换到「全部」' },
    { key: '2', desc: '切换到「待修正」' },
    { key: '3', desc: '切换到「已完成」' },
    { key: '4', desc: '切换到「AI 识别」' },
    { key: '?', desc: '显示/隐藏此帮助' },
    { key: 'Esc', desc: '关闭弹层' },
  ]
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4"
      onClick={onClose}
    >
      <div
        className="glass-panel w-full max-w-sm rounded-xl p-5"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between">
          <h3 className="text-lg font-semibold">键盘快捷键</h3>
          <button onClick={onClose} className="text-surface-400 hover:text-white">
            <X className="h-4 w-4" />
          </button>
        </div>
        <ul className="mt-3 space-y-1.5 text-sm">
          {items.map((it) => (
            <li
              key={it.key}
              className="flex items-center justify-between rounded-md px-2 py-1.5 hover:bg-[var(--nav-hover-bg)]"
            >
              <kbd className="rounded border border-surface-700 bg-surface-900 px-2 py-0.5 font-mono text-xs">
                {it.key}
              </kbd>
              <span className="text-surface-300">{it.desc}</span>
            </li>
          ))}
        </ul>
      </div>
    </div>
  )
}
