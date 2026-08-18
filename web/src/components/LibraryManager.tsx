import { useRef, useState } from 'react'
import { adminApi, libraryApi } from '@/api'
import type { Library, CreateLibraryRequest } from '@/types'
import { getLibraryPaths } from '@/types'
import type { ScanProgressData, ScrapeProgressData, ScanPhaseData } from '@/hooks/useWebSocket'
import { useToast } from './Toast'
import { useDialog } from './Dialog'
import CreateLibraryModal from './CreateLibraryModal'
import EditLibraryModal from './EditLibraryModal'
import {
  ArrowUpDown,
  Calendar,
  ChevronRight,
  Film,
  FolderOpen,
  FolderPlus,
  HardDrive,
  Layers,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  RotateCcw,
  ScanLine,
  Trash2,
  Tv,
  Video,
} from 'lucide-react'
import clsx from 'clsx'
import { AdminPanel, AdminStatus } from '@/components/admin/AdminPrimitives'
import { Button, EmptyState, Tag } from '@/components/design-system'
import { invalidateMediaListCaches } from '@/utils/invalidateMediaCaches'

const TYPE_CONFIG: Record<string, { label: string; icon: typeof Film }> = {
  movie: { label: '电影', icon: Film },
  tvshow: { label: '电视节目', icon: Tv },
  mixed: { label: '混合影片', icon: Layers },
  other: { label: '其他视频', icon: Video },
}

interface LibraryManagerProps {
  libraries: Library[]
  setLibraries: React.Dispatch<React.SetStateAction<Library[]>>
  scanning: Set<string>
  setScanning: React.Dispatch<React.SetStateAction<Set<string>>>
  scanProgress: Record<string, ScanProgressData>
  scrapeProgress: Record<string, ScrapeProgressData>
  scanPhase: Record<string, ScanPhaseData>
}

type MainScanStage = 'scanning' | 'ai_organizing' | 'scraping'

const MAIN_SCAN_STAGES: { id: MainScanStage; label: string; short: string }[] = [
  { id: 'scanning', label: '入库进度', short: '入库' },
  { id: 'ai_organizing', label: 'AI整理进度', short: 'AI整理' },
  { id: 'scraping', label: '元数据刮削进度', short: '刮削' },
]

function clampPercent(value: number) {
  if (!Number.isFinite(value)) return 0
  return Math.max(0, Math.min(100, value))
}

function LibraryManager({
  libraries,
  setLibraries,
  scanning,
  setScanning,
  scanProgress,
  scrapeProgress,
  scanPhase,
}: LibraryManagerProps) {
  const toast = useToast()
  const dialog = useDialog()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [sortBy, setSortBy] = useState<'name' | 'created' | 'type'>('created')
  const [sortAsc, setSortAsc] = useState(false)
  const [activeMenu, setActiveMenu] = useState<string | null>(null)
  const [scanAllLoading, setScanAllLoading] = useState(false)
  const [editingLibrary, setEditingLibrary] = useState<Library | null>(null)
  const [deletingLibraries, setDeletingLibraries] = useState<Set<string>>(new Set())
  // ref 在 React 状态提交前就能立即生效，用于拦截快速连点造成的重复确认/重复 DELETE。
  const deleteFlowRef = useRef<Set<string>>(new Set())

  const sortedLibraries = [...libraries].sort((a, b) => {
    let cmp = 0
    if (sortBy === 'name') cmp = a.name.localeCompare(b.name)
    else if (sortBy === 'type') cmp = a.type.localeCompare(b.type)
    else cmp = new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    return sortAsc ? -cmp : cmp
  })

  const handleCreate = async (data: CreateLibraryRequest) => {
    const response = await libraryApi.create(data)
    const created = response.data.data
    if (created) {
      setLibraries((current) => [created, ...current.filter((library) => library.id !== created.id)])
    }
    invalidateMediaListCaches()

    // Creation has already succeeded at this point. Refreshing the list is a
    // best-effort reconciliation and must not make the modal report failure.
    try {
      const res = await libraryApi.list()
      setLibraries(res.data.data || [])
    } catch {
      toast.info('媒体库已创建，列表刷新失败，可稍后手动刷新')
    }
  }

  const handleScan = async (id: string) => {
    if (deletingLibraries.has(id)) return
    setScanning((current) => new Set(current).add(id))
    try {
      await libraryApi.scan(id)
    } catch (err: any) {
      setScanning((current) => {
        const next = new Set(current)
        next.delete(id)
        return next
      })
      toast.error(err?.response?.data?.error || '扫描启动失败')
    }
  }

  const handleScanAll = async () => {
    const toScan = libraries.filter((library) => !scanning.has(library.id) && !deletingLibraries.has(library.id))
    if (toScan.length === 0) {
      toast.info('所有媒体库已在扫描或删除中')
      return
    }

    setScanAllLoading(true)
    try {
      const response = await adminApi.batchScan(toScan.map((library) => library.id))
      const started: string[] = response.data.started || []
      const errors: Array<{ library_id: string; error: string }> = response.data.errors || []

      if (started.length > 0) {
        setScanning((current) => {
          const next = new Set(current)
          started.forEach((id) => next.add(id))
          return next
        })
        toast.success(`已启动 ${started.length} 个媒体库扫描`)
      }
      if (errors.length > 0) {
        toast.error(`${errors.length} 个媒体库扫描启动失败`)
      }
      if (started.length === 0 && errors.length === 0) {
        toast.info('没有新的媒体库扫描任务被启动')
      }
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '批量扫描启动失败')
    } finally {
      setScanAllLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    // 在任何异步检查之前先占位，防止用户连续点击打开多个确认框。
    if (deleteFlowRef.current.has(id)) return
    deleteFlowRef.current.add(id)

    try {
      // 删除按钮必须允许修复陈旧扫描状态：本地 scanning 可能因刷新、断线或旧缓存残留。
      // 删除前以服务端 scan-status 为准重新确认，既能自愈陈旧状态，也能避免扫描中误删。
      try {
        const response = await libraryApi.scanStatus()
        const activeScanPhases = response.data.data || []
        const isActuallyScanning = activeScanPhases.some((phase) => phase.library_id === id)

        if (isActuallyScanning) {
          setScanning((current) => new Set(current).add(id))
          toast.info('媒体库正在扫描，请等待扫描结束后再删除')
          return
        }

        if (scanning.has(id)) {
          setScanning((current) => {
            const next = new Set(current)
            next.delete(id)
            return next
          })
        }
      } catch {
        toast.error('无法确认媒体库扫描状态，请稍后重试')
        return
      }

      const ok = await dialog.confirm({
        title: '删除媒体库',
        message: '确定删除此媒体库？关联的媒体记录、推荐快照和本地缓存也会被彻底清除。',
        confirmText: '删除',
        variant: 'danger',
      })
      if (!ok) return

      setActiveMenu(null)
      setDeletingLibraries((current) => new Set(current).add(id))

      try {
        await libraryApi.delete(id)
        invalidateMediaListCaches()
        setLibraries((current) => current.filter((library) => library.id !== id))
        setScanning((current) => {
          const next = new Set(current)
          next.delete(id)
          return next
        })
        toast.success('媒体库已删除，派生缓存将在后台继续回收')
      } catch (err: any) {
        toast.error(err?.response?.data?.error || '删除失败')
      }
    } finally {
      deleteFlowRef.current.delete(id)
      setDeletingLibraries((current) => {
        if (!current.has(id)) return current
        const next = new Set(current)
        next.delete(id)
        return next
      })
    }
  }

  const handleReindex = async (id: string) => {
    if (deletingLibraries.has(id)) return
    const ok = await dialog.confirm({
      title: '重建索引',
      message: '确定重建索引？这将清除现有媒体记录并重新扫描文件。',
      confirmText: '重建',
      variant: 'warning',
    })
    if (!ok) return
    setScanning((current) => new Set(current).add(id))
    try {
      await libraryApi.reindex(id)
    } catch (err: any) {
      setScanning((current) => {
        const next = new Set(current)
        next.delete(id)
        return next
      })
      toast.error(err?.response?.data?.error || '重建索引失败')
    }
  }

  const toggleSort = (field: typeof sortBy) => {
    if (sortBy === field) setSortAsc(!sortAsc)
    else {
      setSortBy(field)
      setSortAsc(false)
    }
  }

  const cycleSort = () => toggleSort(sortBy === 'name' ? 'created' : sortBy === 'created' ? 'type' : 'name')

  const formatDate = (date: string | null) => {
    if (!date) return '从未扫描'
    return new Date(date).toLocaleString('zh-CN', {
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
    })
  }

  return (
    <>
      <AdminPanel
        title="媒体库管理"
        description="管理媒体目录、内容类型和扫描索引状态。"
        icon={<HardDrive size={18} />}
        actions={(
          <>
            <Button variant="secondary" size="sm" onClick={cycleSort} title={`排序: ${sortBy === 'name' ? '名称' : sortBy === 'type' ? '类型' : '创建时间'}`}>
              <ArrowUpDown size={14} />
              排序
            </Button>
            {libraries.length > 0 && (
              <Button variant="secondary" size="sm" onClick={handleScanAll} loading={scanAllLoading}>
                {scanAllLoading ? <RefreshCw size={14} className="animate-spin" /> : <ScanLine size={14} />}
                {scanAllLoading ? '扫描中...' : '扫描全部'}
              </Button>
            )}
            <Button variant="primary" size="sm" onClick={() => setShowCreateModal(true)}>
              <FolderPlus size={15} />
              新增媒体库
            </Button>
          </>
        )}
        bodyClassName="p-0"
      >
        {libraries.length > 0 ? (
          <div className="overflow-visible">
            <div className="hidden grid-cols-[minmax(180px,2fr)_minmax(220px,2fr)_120px_150px_120px] gap-4 border-b border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-5 py-3 text-xs font-semibold uppercase tracking-wide text-[var(--nv-text-tertiary)] lg:grid">
              <SortHeader active={sortBy === 'name'} asc={sortAsc} onClick={() => toggleSort('name')}>媒体库</SortHeader>
              <span>媒体文件夹</span>
              <SortHeader active={sortBy === 'type'} asc={sortAsc} onClick={() => toggleSort('type')}>类型</SortHeader>
              <SortHeader active={sortBy === 'created'} asc={sortAsc} onClick={() => toggleSort('created')}>最近更新</SortHeader>
              <span className="text-center">操作</span>
            </div>

            {sortedLibraries.map((library, index) => (
              <LibraryRow
                key={library.id}
                library={library}
                isLast={index === sortedLibraries.length - 1}
                isScanning={scanning.has(library.id)}
                isDeleting={deletingLibraries.has(library.id)}
                progress={scanProgress[library.id]}
                scrape={scrapeProgress[library.id]}
                phase={scanPhase[library.id]}
                activeMenu={activeMenu === library.id}
                onScan={() => handleScan(library.id)}
                onDelete={() => handleDelete(library.id)}
                onMenu={() => setActiveMenu(activeMenu === library.id ? null : library.id)}
                onCloseMenu={() => setActiveMenu(null)}
                onEdit={() => {
                  setActiveMenu(null)
                  setEditingLibrary(library)
                }}
                onReindex={() => {
                  setActiveMenu(null)
                  handleReindex(library.id)
                }}
                formatDate={formatDate}
              />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<FolderPlus size={28} />}
            title="还没有媒体库"
            description="添加媒体库后系统将自动扫描并索引您的视频文件。"
            action={(
              <Button variant="primary" onClick={() => setShowCreateModal(true)}>
                <FolderPlus size={16} />
                新增媒体库
              </Button>
            )}
          />
        )}
      </AdminPanel>

      <CreateLibraryModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreate}
      />

      <EditLibraryModal
        open={!!editingLibrary}
        library={editingLibrary}
        onClose={() => setEditingLibrary(null)}
        onUpdate={(updated) => {
          invalidateMediaListCaches()
          setLibraries((current) => current.map((library) => (library.id === updated.id ? updated : library)))
          toast.success('媒体库已更新')
        }}
      />
    </>
  )
}

function SortHeader({
  active,
  asc,
  onClick,
  children,
}: {
  active: boolean
  asc: boolean
  onClick: () => void
  children: React.ReactNode
}) {
  return (
    <button type="button" onClick={onClick} className="flex items-center gap-1 text-left transition-colors hover:text-[var(--nv-text-primary)]">
      {children}
      {active && <ChevronRight size={12} className={clsx('transition-transform', asc ? '-rotate-90' : 'rotate-90')} />}
    </button>
  )
}

function LibraryRow({
  library,
  isLast,
  isScanning,
  isDeleting,
  progress,
  scrape,
  phase,
  activeMenu,
  onScan,
  onDelete,
  onMenu,
  onCloseMenu,
  onEdit,
  onReindex,
  formatDate,
}: {
  library: Library
  isLast: boolean
  isScanning: boolean
  isDeleting: boolean
  progress?: ScanProgressData
  scrape?: ScrapeProgressData
  phase?: ScanPhaseData
  activeMenu: boolean
  onScan: () => void
  onDelete: () => void
  onMenu: () => void
  onCloseMenu: () => void
  onEdit: () => void
  onReindex: () => void
  formatDate: (date: string | null) => string
}) {
  const typeConfig = TYPE_CONFIG[library.type] || TYPE_CONFIG.movie
  const TypeIcon = typeConfig.icon
  const allPaths = getLibraryPaths(library)
  const displayPath = allPaths.length > 1 ? `${allPaths[0]} +${allPaths.length - 1}` : allPaths[0] || library.path

  const activeStage: MainScanStage = phase?.phase === 'ai_organizing'
    ? 'ai_organizing'
    : phase?.phase === 'scraping' || (!phase && scrape)
      ? 'scraping'
      : 'scanning'
  const activeStageIndex = MAIN_SCAN_STAGES.findIndex((stage) => stage.id === activeStage)
  const stageLabel = MAIN_SCAN_STAGES[activeStageIndex]?.label || '入库进度'
  const phaseCurrent = phase?.current || 0
  const phaseTotal = phase?.total || 0
  const stageProgress = activeStage === 'scraping'
    ? scrape
      ? { current: scrape.current, total: scrape.total }
      : { current: phaseCurrent, total: phaseTotal }
    : activeStage === 'ai_organizing'
      ? { current: phaseCurrent, total: phaseTotal }
      : { current: progress?.current || progress?.new_found || phaseCurrent, total: progress?.total || phaseTotal }
  const stagePercent = stageProgress.total > 0
    ? clampPercent((stageProgress.current / stageProgress.total) * 100)
    : activeStageIndex > 0 ? 100 : 35
  const stageMessage = activeStage === 'scraping' && scrape
    ? `元数据刮削 [${scrape.current}/${scrape.total}] ${scrape.media_title || ''}`
    : activeStage === 'ai_organizing' && phase?.total
      ? `AI整理 [${phase.current || 0}/${phase.total}]`
      : progress?.message || phase?.message || '正在入库...'

  return (
    <div className={clsx('relative', !isLast && 'border-b border-[var(--nv-border-subtle)]')}>
      <div className="grid gap-4 px-4 py-4 transition-colors hover:bg-[var(--nv-bg-hover)] sm:px-5 lg:grid-cols-[minmax(180px,2fr)_minmax(220px,2fr)_120px_150px_120px] lg:items-center">
        <div className="flex min-w-0 items-center gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
            <TypeIcon size={19} />
          </div>
          <div className="min-w-0">
            <h3 className="truncate text-sm font-semibold text-[var(--nv-text-primary)]">{library.name}</h3>
            {isDeleting ? (
              <div className="mt-1 flex items-center gap-2">
                <AdminStatus tone="active">删除中</AdminStatus>
                <span className="truncate text-xs text-[var(--nv-text-tertiary)]">正在清理媒体库索引</span>
              </div>
            ) : isScanning ? (
              <div className="mt-1 flex items-center gap-2">
                <AdminStatus tone="active">进行中</AdminStatus>
                <span className="truncate text-xs text-[var(--nv-text-tertiary)]">{stageLabel}</span>
              </div>
            ) : null}
          </div>
        </div>

        <div className="flex min-w-0 items-center gap-2 text-sm text-[var(--nv-text-secondary)]" title={allPaths.join('\n')}>
          <FolderOpen size={14} className="shrink-0 text-[var(--nv-text-tertiary)]" />
          <span className="truncate font-mono text-xs sm:text-sm">{displayPath}</span>
        </div>

        <div><Tag>{typeConfig.label}</Tag></div>

        <div className="flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)] sm:text-sm">
          <Calendar size={13} className="shrink-0" />
          <span>{formatDate(library.last_scan)}</span>
        </div>

        <div className="flex items-center gap-1 lg:justify-center">
          <Button variant="ghost" size="sm" iconOnly onClick={onScan} disabled={isScanning || isDeleting} title={isDeleting ? '媒体库正在删除' : '扫描媒体文件'} aria-label="扫描媒体文件">
            <RefreshCw size={16} className={isScanning ? 'animate-spin text-[var(--nv-action-primary)]' : undefined} />
          </Button>
          <Button
            variant="danger"
            size="sm"
            iconOnly
            onClick={onDelete}
            disabled={isDeleting}
            title={isDeleting ? '媒体库正在删除' : isScanning ? '媒体库正在扫描，点击可重新确认状态' : '删除媒体库'}
            aria-label={isDeleting ? '正在删除媒体库' : '删除媒体库'}
          >
            {isDeleting ? <RefreshCw size={16} className="animate-spin" /> : <Trash2 size={16} />}
          </Button>
          <div className="relative">
            <Button variant="ghost" size="sm" iconOnly onClick={onMenu} disabled={isDeleting} aria-label="更多操作">
              <MoreHorizontal size={16} />
            </Button>
            {activeMenu && !isDeleting && (
              <>
                <div className="fixed inset-0 z-[var(--nv-z-dropdown)]" onClick={onCloseMenu} />
                <div className="absolute right-0 top-full z-[calc(var(--nv-z-dropdown)+1)] mt-1 w-44 overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] py-1 shadow-[var(--nv-shadow-elevated)]">
                  <button type="button" onClick={onEdit} className="flex w-full items-center gap-2 px-4 py-2.5 text-sm text-[var(--nv-text-secondary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]">
                    <Pencil size={14} />
                    编辑媒体库
                  </button>
                  <button type="button" onClick={onReindex} disabled={isScanning || isDeleting} className="flex w-full items-center gap-2 px-4 py-2.5 text-sm text-[var(--nv-text-secondary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] disabled:opacity-50">
                    <RotateCcw size={14} />
                    重建索引
                  </button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {isScanning && !isDeleting && (progress || scrape || phase) && (
        <div className="px-4 pb-4 sm:px-5">
          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3">
            <div className="grid grid-cols-3 gap-2">
              {MAIN_SCAN_STAGES.map((stage, index) => {
                const done = index < activeStageIndex
                const active = index === activeStageIndex
                return (
                  <div
                    key={stage.id}
                    className={clsx(
                      'rounded-[var(--nv-radius-sm)] border px-2.5 py-2',
                      active ? 'border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)]' : 'border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)]',
                    )}
                  >
                    <div className="flex items-center justify-between gap-1">
                      <span className={clsx('truncate text-[11px] font-semibold', active ? 'text-[var(--nv-action-primary)]' : done ? 'text-[var(--nv-status-success)]' : 'text-[var(--nv-text-tertiary)]')}>
                        {index + 1}. {stage.short}
                      </span>
                      <span className={clsx('text-[10px]', active ? 'text-[var(--nv-action-primary)]' : done ? 'text-[var(--nv-status-success)]' : 'text-[var(--nv-text-disabled)]')}>
                        {done ? '完成' : active ? '进行中' : '等待'}
                      </span>
                    </div>
                  </div>
                )
              })}
            </div>

            <div className="mt-3 flex items-center justify-between gap-3">
              <span className="text-[11px] font-semibold text-[var(--nv-action-primary)]">{stageLabel}</span>
              <span className="text-[11px] font-mono text-[var(--nv-text-tertiary)]">
                {stageProgress.total > 0 ? `${stageProgress.current}/${stageProgress.total}` : `已发现 ${progress?.new_found || 0}`}
              </span>
            </div>
            <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-[var(--nv-bg-control)]">
              <div className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-300" style={{ width: `${stagePercent}%` }} />
            </div>
            <div className="mt-2 flex items-center justify-between gap-3">
              <span className="truncate text-[11px] text-[var(--nv-text-tertiary)]">{stageMessage}</span>
              {activeStage === 'scraping' && scrape && scrape.total > 0 && (
                <span className="shrink-0 text-[11px] text-[var(--nv-text-secondary)]">成功 {scrape.success} · 失败 {scrape.failed}</span>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  )
}

export default LibraryManager
