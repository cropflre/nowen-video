import { useCallback, useEffect, useState, type ReactNode } from 'react'
import type { ScrapeHistory, ScrapeStatistics, ScrapeTask } from '@/types'
import { scrapeApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useDialog } from '@/components/Dialog'
import { useWebSocket } from '@/hooks/useWebSocket'
import { usePagination } from '@/hooks/usePagination'
import Pagination from '@/components/Pagination'
import {
  AlertCircle,
  BarChart3,
  Check,
  CheckSquare,
  Clock,
  Download,
  Edit3,
  ExternalLink,
  FileText,
  Filter,
  Globe,
  Languages,
  Loader2,
  Play,
  Plus,
  RefreshCw,
  Square,
  Trash2,
  Upload,
  X,
} from 'lucide-react'
import clsx from 'clsx'
import {
  Button,
  EmptyState,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Select,
  Surface,
  Tag,
  Textarea,
  type TagTone,
} from '@/components/design-system'

const SOURCE_OPTIONS = [
  { value: '', label: '自动识别' },
  { value: 'tmdb', label: 'TMDb' },
  { value: 'douban', label: '豆瓣' },
  { value: 'bangumi', label: 'Bangumi' },
  { value: 'url', label: '通用URL' },
]

const LANG_OPTIONS = [
  { value: 'zh-CN', label: '简体中文' },
  { value: 'zh-TW', label: '繁体中文' },
  { value: 'en', label: 'English' },
  { value: 'ja', label: '日本語' },
  { value: 'ko', label: '한국어' },
]

const TRANSLATE_FIELDS = [
  { value: 'title', label: '标题' },
  { value: 'overview', label: '简介' },
  { value: 'genres', label: '类型' },
  { value: 'tagline', label: '宣传语' },
]

const STATUS_CONFIG: Record<string, { label: string; tone: TagTone; icon: typeof Check }> = {
  pending: { label: '待处理', tone: 'neutral', icon: Clock },
  scraping: { label: '刮削中', tone: 'warning', icon: Loader2 },
  scraped: { label: '已刮削', tone: 'brand', icon: Check },
  failed: { label: '失败', tone: 'danger', icon: X },
  translating: { label: '翻译中', tone: 'brand', icon: Loader2 },
  completed: { label: '已完成', tone: 'success', icon: Check },
}

const TRANSLATE_STATUS_CONFIG: Record<string, { label: string; tone: TagTone }> = {
  none: { label: '未翻译', tone: 'neutral' },
  pending: { label: '待翻译', tone: 'neutral' },
  translating: { label: '翻译中', tone: 'brand' },
  done: { label: '已翻译', tone: 'success' },
  failed: { label: '翻译失败', tone: 'danger' },
}

interface ScrapeManagerPageProps {
  embedded?: boolean
}

function qualityTone(score: number): TagTone {
  if (score >= 80) return 'success'
  if (score >= 50) return 'warning'
  if (score > 0) return 'danger'
  return 'neutral'
}

function historyTone(action: string): TagTone {
  if (action.includes('fail')) return 'danger'
  if (action.includes('done') || action === 'created') return 'success'
  return 'neutral'
}

export default function ScrapeManagerPage({ embedded = false }: ScrapeManagerPageProps) {
  const toast = useToast()
  const dialog = useDialog()
  const { on, off } = useWebSocket()

  const [tasks, setTasks] = useState<ScrapeTask[]>([])
  const [total, setTotal] = useState(0)
  const { page, size: pageSize, setPage, setSize, totalPages } = usePagination({ initialSize: 20 })
  const [loading, setLoading] = useState(true)
  const [stats, setStats] = useState<ScrapeStatistics | null>(null)

  const [filterStatus, setFilterStatus] = useState('')
  const [filterSource, setFilterSource] = useState('')
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set())

  const [showCreateForm, setShowCreateForm] = useState(false)
  const [createMode, setCreateMode] = useState<'single' | 'batch'>('single')
  const [urlInput, setUrlInput] = useState('')
  const [batchUrlInput, setBatchUrlInput] = useState('')
  const [createSource, setCreateSource] = useState('')
  const [createMediaType, setCreateMediaType] = useState('movie')
  const [creating, setCreating] = useState(false)

  const [showTranslateDialog, setShowTranslateDialog] = useState(false)
  const [translateTargetLang, setTranslateTargetLang] = useState('zh-CN')
  const [translateFields, setTranslateFields] = useState<string[]>([])
  const [translateTaskIds, setTranslateTaskIds] = useState<string[]>([])

  const [editingTask, setEditingTask] = useState<ScrapeTask | null>(null)
  const [showHistory, setShowHistory] = useState(false)
  const [history, setHistory] = useState<ScrapeHistory[]>([])

  const fetchTasks = useCallback(async () => {
    try {
      const res = await scrapeApi.listTasks({
        page,
        size: pageSize,
        status: filterStatus || undefined,
        source: filterSource || undefined,
      })
      setTasks(res.data.data || [])
      setTotal(res.data.total)
    } catch {
      toast.error('加载刮削任务失败')
    } finally {
      setLoading(false)
    }
  }, [page, pageSize, filterStatus, filterSource])

  const fetchStats = useCallback(async () => {
    try {
      const res = await scrapeApi.getStatistics()
      setStats(res.data.data)
    } catch {
      // 保持原来的统计静默失败行为。
    }
  }, [])

  useEffect(() => {
    void fetchTasks()
    void fetchStats()
  }, [fetchTasks, fetchStats])

  useEffect(() => {
    const handleTaskUpdate = (data: ScrapeTask) => {
      setTasks((previous) => previous.map((task) => task.id === data.id ? data : task))
      void fetchStats()
    }

    on('scrape_task_update' as any, handleTaskUpdate)
    return () => {
      off('scrape_task_update' as any, handleTaskUpdate)
    }
  }, [on, off, fetchStats])

  const handleCreate = async () => {
    if (createMode === 'single') {
      if (!urlInput.trim()) {
        toast.error('请输入URL')
        return
      }
      setCreating(true)
      try {
        await scrapeApi.createTask({
          url: urlInput.trim(),
          source: createSource || undefined,
          media_type: createMediaType,
        })
        toast.success('刮削任务已创建')
        setUrlInput('')
        setShowCreateForm(false)
        void fetchTasks()
        void fetchStats()
      } catch (err: any) {
        toast.error(err?.response?.data?.error || '创建失败')
      } finally {
        setCreating(false)
      }
      return
    }

    const urls = batchUrlInput.split('\n').map((url) => url.trim()).filter(Boolean)
    if (urls.length === 0) {
      toast.error('请输入至少一个URL')
      return
    }

    setCreating(true)
    try {
      const res = await scrapeApi.batchCreateTasks({
        urls,
        source: createSource || undefined,
        media_type: createMediaType,
      })
      toast.success(`批量创建完成: 成功 ${res.data.created}, 跳过 ${res.data.skipped}`)
      setBatchUrlInput('')
      setShowCreateForm(false)
      void fetchTasks()
      void fetchStats()
    } catch {
      toast.error('批量创建失败')
    } finally {
      setCreating(false)
    }
  }

  const handleStartScrape = async (id: string) => {
    try {
      await scrapeApi.startScrape(id)
      toast.success('刮削已启动')
      setTasks((previous) => previous.map((task) => task.id === id ? { ...task, status: 'scraping' as const } : task))
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '启动失败')
    }
  }

  const handleBatchScrape = async () => {
    if (selectedIds.size === 0) {
      toast.error('请先选择任务')
      return
    }
    try {
      const res = await scrapeApi.batchStartScrape(Array.from(selectedIds))
      toast.success(`批量刮削已启动: ${res.data.started} 个任务`)
      setSelectedIds(new Set())
      void fetchTasks()
    } catch {
      toast.error('批量刮削失败')
    }
  }

  const openTranslateDialog = (taskIds: string[]) => {
    setTranslateTaskIds(taskIds)
    setShowTranslateDialog(true)
  }

  const handleTranslate = async () => {
    try {
      if (translateTaskIds.length === 1) {
        await scrapeApi.translateTask(translateTaskIds[0], {
          target_lang: translateTargetLang,
          fields: translateFields.length > 0 ? translateFields : undefined,
        })
        toast.success('翻译已启动')
      } else {
        const res = await scrapeApi.batchTranslate({
          task_ids: translateTaskIds,
          target_lang: translateTargetLang,
          fields: translateFields.length > 0 ? translateFields : undefined,
        })
        toast.success(`批量翻译已启动: ${res.data.started} 个任务`)
      }
      setShowTranslateDialog(false)
      setSelectedIds(new Set())
      void fetchTasks()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '翻译启动失败')
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await dialog.confirm({
      title: '删除刮削任务',
      message: '确定删除该刮削任务？',
      confirmText: '删除',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await scrapeApi.deleteTask(id)
      toast.success('已删除')
      void fetchTasks()
      void fetchStats()
    } catch {
      toast.error('删除失败')
    }
  }

  const handleBatchDelete = async () => {
    if (selectedIds.size === 0) return
    const ok = await dialog.confirm({
      title: '批量删除刮削任务',
      message: `确定删除选中的 ${selectedIds.size} 个任务？`,
      confirmText: '删除',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await scrapeApi.batchDeleteTasks(Array.from(selectedIds))
      toast.success('批量删除完成')
      setSelectedIds(new Set())
      void fetchTasks()
      void fetchStats()
    } catch {
      toast.error('批量删除失败')
    }
  }

  const handleExport = async () => {
    const ids = selectedIds.size > 0 ? Array.from(selectedIds) : tasks.map((task) => task.id)
    try {
      const res = await scrapeApi.exportTasks(ids)
      const blob = new Blob([JSON.stringify(res.data.data, null, 2)], { type: 'application/json' })
      const url = URL.createObjectURL(blob)
      const anchor = document.createElement('a')
      anchor.href = url
      anchor.download = `scrape-export-${new Date().toISOString().slice(0, 10)}.json`
      anchor.click()
      URL.revokeObjectURL(url)
      toast.success('导出成功')
    } catch {
      toast.error('导出失败')
    }
  }

  const handleSaveEdit = async () => {
    if (!editingTask) return
    try {
      await scrapeApi.updateTask(editingTask.id, {
        result_title: editingTask.result_title,
        result_orig_title: editingTask.result_orig_title,
        result_year: editingTask.result_year,
        result_overview: editingTask.result_overview,
        result_genres: editingTask.result_genres,
        result_rating: editingTask.result_rating,
        result_country: editingTask.result_country,
        result_language: editingTask.result_language,
      })
      toast.success('保存成功')
      setEditingTask(null)
      void fetchTasks()
    } catch {
      toast.error('保存失败')
    }
  }

  const loadHistory = async (taskId?: string) => {
    try {
      const res = await scrapeApi.getHistory({ task_id: taskId, limit: 50 })
      setHistory(res.data.data || [])
      setShowHistory(true)
    } catch {
      toast.error('加载历史失败')
    }
  }

  const toggleSelect = (id: string) => {
    setSelectedIds((previous) => {
      const next = new Set(previous)
      if (next.has(id)) next.delete(id)
      else next.add(id)
      return next
    })
  }

  const toggleSelectAll = () => {
    setSelectedIds((previous) => previous.size === tasks.length ? new Set() : new Set(tasks.map((task) => task.id)))
  }

  const refresh = () => {
    void fetchTasks()
    void fetchStats()
  }

  return (
    <div className={clsx('space-y-6', embedded && 'pt-0')}>
      {!embedded && (
        <header className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <h1 className="flex items-center gap-2 text-2xl font-semibold text-[var(--nv-text-primary)]">
              <Globe size={23} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
              刮削数据管理
            </h1>
            <p className="mt-1 text-sm leading-6 text-[var(--nv-text-tertiary)]">
              管理元数据刮削任务，支持多数据源、AI 增强和多语言翻译。
            </p>
          </div>
          <ScrapeActions
            onHistory={() => void loadHistory()}
            onExport={() => void handleExport()}
            onRefresh={refresh}
            onCreate={() => setShowCreateForm((value) => !value)}
            createOpen={showCreateForm}
          />
        </header>
      )}

      {embedded && (
        <div className="flex flex-wrap items-center justify-between gap-3">
          <ScrapeActions
            onHistory={() => void loadHistory()}
            onExport={() => void handleExport()}
            onRefresh={refresh}
            onCreate={() => setShowCreateForm((value) => !value)}
            createOpen={showCreateForm}
          />
        </div>
      )}

      {stats && <StatisticsGrid stats={stats} />}

      {showCreateForm && (
        <Surface className="space-y-4 p-4 sm:p-5">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">新建刮削任务</h3>
              <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">支持单条 URL 或每行一个 URL 的批量创建。</p>
            </div>
            <div className="flex overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] p-1" role="group" aria-label="创建模式">
              <ModeButton selected={createMode === 'single'} onClick={() => setCreateMode('single')}>单条输入</ModeButton>
              <ModeButton selected={createMode === 'batch'} onClick={() => setCreateMode('batch')}>
                <Upload size={12} aria-hidden="true" />
                批量导入
              </ModeButton>
            </div>
          </div>

          {createMode === 'single' ? (
            <Field label="URL 地址">
              <Input
                type="text"
                value={urlInput}
                onChange={(event) => setUrlInput(event.target.value)}
                placeholder="输入 TMDb / 豆瓣 / IMDb / Bangumi 链接或任意 URL"
                onKeyDown={(event) => {
                  if (event.key === 'Enter') void handleCreate()
                }}
              />
            </Field>
          ) : (
            <Field label="URL 列表（每行一个）" hint={`已输入 ${batchUrlInput.split('\n').filter((url) => url.trim()).length} 条 URL`}>
              <Textarea
                value={batchUrlInput}
                onChange={(event) => setBatchUrlInput(event.target.value)}
                className="min-h-32 resize-y font-mono text-xs"
                placeholder={'https://www.themoviedb.org/movie/550\nhttps://movie.douban.com/subject/1292052/\nhttps://bgm.tv/subject/12345'}
              />
            </Field>
          )}

          <div className="grid gap-4 sm:grid-cols-2">
            <Field label="数据源">
              <Select value={createSource} onChange={(event) => setCreateSource(event.target.value)} className="w-full">
                {SOURCE_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </Select>
            </Field>
            <Field label="媒体类型">
              <Select value={createMediaType} onChange={(event) => setCreateMediaType(event.target.value)} className="w-full">
                <option value="movie">电影</option>
                <option value="tvshow">电视剧</option>
              </Select>
            </Field>
          </div>

          <div className="flex justify-end gap-2 border-t border-[var(--nv-border-subtle)] pt-4">
            <Button type="button" variant="ghost" onClick={() => setShowCreateForm(false)}>取消</Button>
            <Button type="button" variant="primary" onClick={() => void handleCreate()} loading={creating}>
              {!creating && <Check size={14} aria-hidden="true" />}
              {createMode === 'single' ? '创建任务' : '批量创建'}
            </Button>
          </div>
        </Surface>
      )}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-wrap items-center gap-2">
          <Filter size={14} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
          <Select
            value={filterStatus}
            onChange={(event) => { setFilterStatus(event.target.value); setPage(1) }}
            className="h-9 text-xs"
            aria-label="状态筛选"
          >
            <option value="">全部状态</option>
            <option value="pending">待处理</option>
            <option value="scraping">刮削中</option>
            <option value="scraped">已刮削</option>
            <option value="failed">失败</option>
            <option value="translating">翻译中</option>
            <option value="completed">已完成</option>
          </Select>
          <Select
            value={filterSource}
            onChange={(event) => { setFilterSource(event.target.value); setPage(1) }}
            className="h-9 text-xs"
            aria-label="来源筛选"
          >
            <option value="">全部来源</option>
            <option value="tmdb">TMDb</option>
            <option value="douban">豆瓣</option>
            <option value="bangumi">Bangumi</option>
            <option value="url">通用URL</option>
          </Select>
        </div>

        {selectedIds.size > 0 && (
          <div className="flex flex-wrap items-center gap-2">
            <Tag tone="brand">已选 {selectedIds.size} 项</Tag>
            <Button type="button" size="sm" variant="secondary" onClick={() => void handleBatchScrape()}>
              <Play size={12} aria-hidden="true" />
              批量刮削
            </Button>
            <Button type="button" size="sm" variant="secondary" onClick={() => openTranslateDialog(Array.from(selectedIds))}>
              <Languages size={12} aria-hidden="true" />
              批量翻译
            </Button>
            <Button type="button" size="sm" variant="danger" onClick={() => void handleBatchDelete()}>
              <Trash2 size={12} aria-hidden="true" />
              批量删除
            </Button>
          </div>
        )}
      </div>

      <Surface className="overflow-hidden p-0">
        {loading ? (
          <div className="flex min-h-64 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
            <Loader2 size={22} className="animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" />
            加载刮削任务中...
          </div>
        ) : tasks.length > 0 ? (
          <>
            <div className="hidden grid-cols-[32px_minmax(170px,1.4fr)_72px_94px_82px_64px_minmax(120px,1fr)_132px] items-center gap-3 border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3 text-[10px] font-semibold uppercase tracking-wide text-[var(--nv-text-tertiary)] lg:grid">
              <button type="button" onClick={toggleSelectAll} className="text-[var(--nv-text-tertiary)] hover:text-[var(--nv-action-primary)]" aria-label="全选刮削任务">
                {selectedIds.size === tasks.length ? <CheckSquare size={14} /> : <Square size={14} />}
              </button>
              <span>标题 / URL</span>
              <span className="text-center">来源</span>
              <span className="text-center">状态</span>
              <span className="text-center">翻译</span>
              <span className="text-center">质量</span>
              <span>错误</span>
              <span className="text-right">操作</span>
            </div>

            <div className="divide-y divide-[var(--nv-border-subtle)]">
              {tasks.map((task) => (
                <TaskRow
                  key={task.id}
                  task={task}
                  selected={selectedIds.has(task.id)}
                  onToggle={() => toggleSelect(task.id)}
                  onScrape={() => void handleStartScrape(task.id)}
                  onTranslate={() => openTranslateDialog([task.id])}
                  onEdit={() => setEditingTask({ ...task })}
                  onDelete={() => void handleDelete(task.id)}
                />
              ))}
            </div>
          </>
        ) : (
          <EmptyState
            icon={<Globe size={25} />}
            title="暂无刮削任务"
            description="点击「新建任务」开始添加需要处理的元数据来源。"
            action={(
              <Button type="button" variant="primary" size="sm" onClick={() => setShowCreateForm(true)}>
                <Plus size={14} aria-hidden="true" />
                新建任务
              </Button>
            )}
          />
        )}
      </Surface>

      <Pagination
        page={page}
        totalPages={totalPages(total)}
        total={total}
        pageSize={pageSize}
        pageSizeOptions={[10, 20, 50, 100]}
        onPageChange={setPage}
        onPageSizeChange={setSize}
      />

      {showTranslateDialog && (
        <Modal open onClose={() => setShowTranslateDialog(false)} size="sm" ariaLabel="翻译设置">
          <ModalHeader
            title="翻译设置"
            description={`将对 ${translateTaskIds.length} 个任务执行翻译，需要 AI 服务支持。`}
            icon={<Languages size={18} aria-hidden="true" />}
            onClose={() => setShowTranslateDialog(false)}
          />
          <ModalBody className="space-y-4">
            <Field label="目标语言">
              <Select value={translateTargetLang} onChange={(event) => setTranslateTargetLang(event.target.value)} className="w-full">
                {LANG_OPTIONS.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </Select>
            </Field>

            <Field label="翻译字段" hint="不选择字段时翻译全部可翻译内容。">
              <div className="flex flex-wrap gap-2">
                {TRANSLATE_FIELDS.map((field) => {
                  const selected = translateFields.includes(field.value)
                  return (
                    <button
                      key={field.value}
                      type="button"
                      aria-pressed={selected}
                      onClick={() => setTranslateFields((previous) =>
                        previous.includes(field.value)
                          ? previous.filter((value) => value !== field.value)
                          : [...previous, field.value]
                      )}
                      className="rounded-[var(--nv-radius-control)] border px-3 py-1.5 text-xs font-medium outline-none transition-[background-color,border-color,color,box-shadow] focus-visible:shadow-[var(--nv-shadow-focus)]"
                      style={{
                        borderColor: selected ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
                        background: selected ? 'var(--nv-bg-active)' : 'var(--nv-bg-control)',
                        color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
                      }}
                    >
                      {field.label}
                    </button>
                  )
                })}
              </div>
            </Field>
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="ghost" onClick={() => setShowTranslateDialog(false)}>取消</Button>
            <Button type="button" variant="primary" onClick={() => void handleTranslate()}>
              <Languages size={14} aria-hidden="true" />
              开始翻译
            </Button>
          </ModalFooter>
        </Modal>
      )}

      {editingTask && (
        <Modal open onClose={() => setEditingTask(null)} size="lg" ariaLabel="编辑刮削结果">
          <ModalHeader
            title="编辑刮削结果"
            description="查看来源并修正已识别的元数据。"
            icon={<Edit3 size={18} aria-hidden="true" />}
            onClose={() => setEditingTask(null)}
          />
          <ModalBody className="space-y-4">
            <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3 text-xs">
              <span className="text-[var(--nv-text-tertiary)]">URL：</span>
              <a
                href={editingTask.url}
                target="_blank"
                rel="noopener noreferrer"
                className="inline-flex max-w-full items-center gap-1 break-all text-[var(--nv-action-primary)] hover:underline"
              >
                {editingTask.url}
                <ExternalLink size={10} className="shrink-0" aria-hidden="true" />
              </a>
            </div>

            <div className="grid gap-3 sm:grid-cols-2">
              <Field label="标题">
                <Input value={editingTask.result_title} onChange={(event) => setEditingTask({ ...editingTask, result_title: event.target.value })} />
              </Field>
              <Field label="原始标题">
                <Input value={editingTask.result_orig_title} onChange={(event) => setEditingTask({ ...editingTask, result_orig_title: event.target.value })} />
              </Field>
              <Field label="年份">
                <Input type="number" value={editingTask.result_year || ''} onChange={(event) => setEditingTask({ ...editingTask, result_year: Number.parseInt(event.target.value, 10) || 0 })} />
              </Field>
              <Field label="评分">
                <Input type="number" step="0.1" value={editingTask.result_rating || ''} onChange={(event) => setEditingTask({ ...editingTask, result_rating: Number.parseFloat(event.target.value) || 0 })} />
              </Field>
              <Field label="类型">
                <Input value={editingTask.result_genres} onChange={(event) => setEditingTask({ ...editingTask, result_genres: event.target.value })} placeholder="动作,科幻,冒险" />
              </Field>
              <Field label="国家">
                <Input value={editingTask.result_country} onChange={(event) => setEditingTask({ ...editingTask, result_country: event.target.value })} />
              </Field>
            </div>

            <Field label="简介">
              <Textarea value={editingTask.result_overview} onChange={(event) => setEditingTask({ ...editingTask, result_overview: event.target.value })} className="min-h-28 resize-y text-xs" />
            </Field>

            {editingTask.translate_status === 'done' && (
              <div className="space-y-2 rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
                <div className="flex items-center gap-2 text-xs font-semibold text-[var(--nv-action-primary)]">
                  <Languages size={13} aria-hidden="true" />
                  翻译结果 ({editingTask.translate_lang})
                </div>
                {editingTask.translated_title && (
                  <PreviewLine label="标题">{editingTask.translated_title}</PreviewLine>
                )}
                {editingTask.translated_overview && (
                  <PreviewLine label="简介">{editingTask.translated_overview}</PreviewLine>
                )}
                {editingTask.translated_genres && (
                  <PreviewLine label="类型">{editingTask.translated_genres}</PreviewLine>
                )}
              </div>
            )}
          </ModalBody>
          <ModalFooter className="justify-between">
            <Tag tone={qualityTone(editingTask.quality_score)}>
              <BarChart3 size={12} aria-hidden="true" />
              质量评分：{editingTask.quality_score}
            </Tag>
            <div className="flex gap-2">
              <Button type="button" variant="ghost" onClick={() => setEditingTask(null)}>取消</Button>
              <Button type="button" variant="primary" onClick={() => void handleSaveEdit()}>
                <Check size={14} aria-hidden="true" />
                保存修改
              </Button>
            </div>
          </ModalFooter>
        </Modal>
      )}

      {showHistory && (
        <Modal open onClose={() => setShowHistory(false)} size="md" ariaLabel="操作历史">
          <ModalHeader
            title="操作历史"
            description="最近 50 条刮削任务操作记录。"
            icon={<FileText size={18} aria-hidden="true" />}
            onClose={() => setShowHistory(false)}
          />
          <ModalBody>
            {history.length > 0 ? (
              <div className="space-y-2">
                {history.map((item) => (
                  <div key={item.id} className="flex items-start gap-3 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3">
                    <Tag tone={historyTone(item.action)} className="shrink-0">{item.action}</Tag>
                    <div className="min-w-0 flex-1">
                      {item.detail && <p className="break-words text-xs leading-5 text-[var(--nv-text-secondary)]">{item.detail}</p>}
                      <time className="mt-1 block text-[10px] text-[var(--nv-text-tertiary)]" dateTime={item.created_at}>
                        {new Date(item.created_at).toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })}
                      </time>
                    </div>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyState icon={<Clock size={24} />} title="暂无操作记录" className="min-h-52" />
            )}
          </ModalBody>
          <ModalFooter>
            <Button type="button" variant="ghost" onClick={() => setShowHistory(false)}>关闭</Button>
          </ModalFooter>
        </Modal>
      )}
    </div>
  )
}

function ScrapeActions({
  onHistory,
  onExport,
  onRefresh,
  onCreate,
  createOpen,
}: {
  onHistory: () => void
  onExport: () => void
  onRefresh: () => void
  onCreate: () => void
  createOpen: boolean
}) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <Button type="button" variant="secondary" size="sm" onClick={onHistory}>
        <Clock size={14} aria-hidden="true" />
        操作历史
      </Button>
      <Button type="button" variant="secondary" size="sm" onClick={onExport}>
        <Download size={14} aria-hidden="true" />
        导出
      </Button>
      <Button type="button" variant="ghost" size="sm" iconOnly onClick={onRefresh} aria-label="刷新刮削任务" title="刷新">
        <RefreshCw size={15} aria-hidden="true" />
      </Button>
      <Button type="button" variant={createOpen ? 'secondary' : 'primary'} size="sm" onClick={onCreate}>
        <Plus size={14} aria-hidden="true" />
        {createOpen ? '收起创建' : '新建任务'}
      </Button>
    </div>
  )
}

function StatisticsGrid({ stats }: { stats: ScrapeStatistics }) {
  const items: Array<{ label: string; value: number; tone: TagTone }> = [
    { label: '总计', value: stats.total, tone: 'brand' },
    { label: '待处理', value: stats.pending, tone: 'neutral' },
    { label: '刮削中', value: stats.scraping, tone: 'warning' },
    { label: '已刮削', value: stats.scraped, tone: 'brand' },
    { label: '翻译中', value: stats.translating, tone: 'brand' },
    { label: '已完成', value: stats.completed, tone: 'success' },
    { label: '失败', value: stats.failed, tone: 'danger' },
  ]

  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-4 xl:grid-cols-7">
      {items.map((item) => (
        <Surface key={item.label} className="p-3 text-center">
          <div className="flex justify-center"><Tag tone={item.tone}>{item.label}</Tag></div>
          <p className="mt-2 text-xl font-semibold text-[var(--nv-text-primary)]">{item.value || 0}</p>
        </Surface>
      ))}
    </div>
  )
}

function ModeButton({ selected, onClick, children }: { selected: boolean; onClick: () => void; children: ReactNode }) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-pressed={selected}
      className="flex items-center gap-1.5 rounded-[var(--nv-radius-control)] px-3 py-1.5 text-xs font-medium outline-none transition-[background-color,color,box-shadow] focus-visible:shadow-[var(--nv-shadow-focus)]"
      style={{
        background: selected ? 'var(--nv-bg-active)' : 'transparent',
        color: selected ? 'var(--nv-action-primary)' : 'var(--nv-text-secondary)',
      }}
    >
      {children}
    </button>
  )
}

function Field({ label, hint, children }: { label: string; hint?: string; children: ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-[var(--nv-text-secondary)]">{label}</span>
      {children}
      {hint && <span className="block text-xs text-[var(--nv-text-tertiary)]">{hint}</span>}
    </label>
  )
}

function TaskRow({
  task,
  selected,
  onToggle,
  onScrape,
  onTranslate,
  onEdit,
  onDelete,
}: {
  task: ScrapeTask
  selected: boolean
  onToggle: () => void
  onScrape: () => void
  onTranslate: () => void
  onEdit: () => void
  onDelete: () => void
}) {
  const status = STATUS_CONFIG[task.status] || STATUS_CONFIG.pending
  const translateStatus = TRANSLATE_STATUS_CONFIG[task.translate_status] || TRANSLATE_STATUS_CONFIG.none
  const StatusIcon = status.icon

  return (
    <article
      className={clsx(
        'grid gap-3 px-4 py-3 transition-colors hover:bg-[var(--nv-bg-hover)] lg:grid-cols-[32px_minmax(170px,1.4fr)_72px_94px_82px_64px_minmax(120px,1fr)_132px] lg:items-center',
        selected && 'bg-[var(--nv-bg-active)]',
      )}
    >
      <button type="button" onClick={onToggle} className="text-left text-[var(--nv-text-tertiary)] hover:text-[var(--nv-action-primary)]" aria-label={selected ? '取消选择任务' : '选择任务'}>
        {selected ? <CheckSquare size={15} className="text-[var(--nv-action-primary)]" /> : <Square size={15} />}
      </button>

      <div className="min-w-0">
        <p className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{task.result_title || task.title || '未识别'}</p>
        <p className="mt-0.5 truncate text-[10px] text-[var(--nv-text-tertiary)]">{task.url}</p>
      </div>

      <div><Tag tone="neutral">{task.source.toUpperCase()}</Tag></div>
      <div>
        <Tag tone={status.tone}>
          <StatusIcon size={10} className={task.status === 'scraping' || task.status === 'translating' ? 'animate-spin' : undefined} />
          {status.label}
        </Tag>
      </div>
      <div><Tag tone={translateStatus.tone}>{translateStatus.label}</Tag></div>
      <div><Tag tone={qualityTone(task.quality_score)}>{task.quality_score > 0 ? task.quality_score : '—'}</Tag></div>

      <div className="min-w-0">
        {task.error_message && (
          <span className="flex items-center gap-1 truncate text-[10px] text-[var(--nv-status-danger)]">
            <AlertCircle size={10} className="shrink-0" aria-hidden="true" />
            {task.error_message}
          </span>
        )}
      </div>

      <div className="flex flex-wrap items-center gap-1 lg:justify-end">
        {(task.status === 'pending' || task.status === 'failed') && (
          <Button type="button" variant="ghost" size="sm" iconOnly onClick={onScrape} title="开始刮削" aria-label="开始刮削">
            <Play size={13} />
          </Button>
        )}
        {(task.status === 'scraped' || task.status === 'completed') && (
          <Button type="button" variant="ghost" size="sm" iconOnly onClick={onTranslate} title="翻译" aria-label="翻译">
            <Languages size={13} />
          </Button>
        )}
        <Button type="button" variant="ghost" size="sm" iconOnly onClick={onEdit} title="查看/编辑" aria-label="查看或编辑">
          <Edit3 size={13} />
        </Button>
        <Button type="button" variant="danger" size="sm" iconOnly onClick={onDelete} title="删除" aria-label="删除任务">
          <Trash2 size={13} />
        </Button>
      </div>
    </article>
  )
}

function PreviewLine({ label, children }: { label: string; children: ReactNode }) {
  return (
    <p className="text-xs leading-5">
      <span className="text-[var(--nv-text-tertiary)]">{label}：</span>
      <span className="text-[var(--nv-text-secondary)]">{children}</span>
    </p>
  )
}
