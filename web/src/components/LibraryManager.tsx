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
  Film,
  FolderPlus,
  Layers,
  MoreHorizontal,
  Pencil,
  RefreshCw,
  RotateCcw,
  Trash2,
  Tv,
  Video,
} from 'lucide-react'
import { AdminPanel, AdminStatus } from '@/components/admin/AdminPrimitives'
import { Button, EmptyState } from '@/components/design-system'
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
  scanFailures?: Record<string, string>
}

function LibraryManager({
  libraries,
  setLibraries,
  scanning,
  setScanning,
  scanProgress,
  scrapeProgress,
  scanPhase,
  scanFailures,
}: LibraryManagerProps) {
  const toast = useToast()
  const dialog = useDialog()
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [sortBy, setSortBy] = useState<'name' | 'created' | 'type'>('created')
  const [startFailures, setStartFailures] = useState<Record<string, string>>({})
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
    return cmp
  })

  const clearStartFailure = (id: string) => {
    setStartFailures(({ [id]: _previous, ...remaining }) => remaining)
  }

  const handleCreate = async (data: CreateLibraryRequest) => {
    const response = await libraryApi.create(data)
    const created = response.data.data
    if (created) {
      setLibraries((current) => [created, ...current.filter((library) => library.id !== created.id)])
    }
    invalidateMediaListCaches()
    if (created) {
      setScanning((current) => new Set(current).add(created.id))
      try {
        await libraryApi.scan(created.id)
        toast.success('文件夹已添加，正在查找影片')
      } catch (error: any) {
        setScanning((current) => { const next = new Set(current); next.delete(created.id); return next })
        setStartFailures((current) => ({ ...current, [created.id]: error?.response?.data?.error || '未能开始查找影片，请重试。' }))
        toast.warning('文件夹已添加，但未能开始查找影片。')
      }
    }

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
    clearStartFailure(id)
    setScanning((current) => new Set(current).add(id))
    try {
      await libraryApi.scan(id)
    } catch (err: any) {
      setScanning((current) => {
        const next = new Set(current)
        next.delete(id)
        return next
      })
      setStartFailures((current) => ({ ...current, [id]: err?.response?.data?.error || '未能开始更新，请重试。' }))
    }
  }

  const handleScanAll = async () => {
    const toScan = libraries.filter((library) => !scanning.has(library.id) && !deletingLibraries.has(library.id))
    if (toScan.length === 0) {
      toast.info('所有媒体库已在扫描或删除中')
      return
    }

    setScanAllLoading(true)
    setStartFailures((current) => Object.fromEntries(Object.entries(current).filter(([id]) => !toScan.some((library) => library.id === id))))
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
        setStartFailures((current) => ({ ...current, ...Object.fromEntries(errors.map(({ library_id, error }) => [library_id, error])) }))
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
    clearStartFailure(id)
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

  return (
    <>
      <AdminPanel
        className="nv-library-manager"
        title="影片库"
        description={libraries.length > 0 ? `共 ${libraries.length} 个影片库` : undefined}
        actions={libraries.length > 0 ? (
          <>
            {libraries.length > 1 && (
              <>
                <label className="sr-only" htmlFor="library-order">排序影片库</label>
                <select id="library-order" className="nv-input text-sm" value={sortBy} onChange={(event) => setSortBy(event.target.value as typeof sortBy)}>
                  <option value="created">最近添加</option>
                  <option value="name">按名称</option>
                  <option value="type">按类型</option>
                </select>
                <Button variant="secondary" size="sm" onClick={handleScanAll} loading={scanAllLoading}>
                  全部更新
                </Button>
              </>
            )}
            <Button variant="primary" size="sm" onClick={() => setShowCreateModal(true)}>
              <FolderPlus size={15} aria-hidden="true" />
              添加文件夹
            </Button>
          </>
        ) : undefined}
        bodyClassName="p-0"
      >
        {libraries.length > 0 ? (
          <div className="nv-library-list">
            {sortedLibraries.map((library) => (
              <LibraryRow
                key={library.id}
                library={library}
                isScanning={scanning.has(library.id)}
                isDeleting={deletingLibraries.has(library.id)}
                progress={scanProgress[library.id]}
                scrape={scrapeProgress[library.id]}
                phase={scanPhase[library.id]}
                failure={startFailures[library.id] || scanFailures?.[library.id]}
                activeMenu={activeMenu === library.id}
                onScan={() => handleScan(library.id)}
                onDelete={() => {
                  setActiveMenu(null)
                  void handleDelete(library.id)
                }}
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
              />
            ))}
          </div>
        ) : (
          <EmptyState
            icon={<FolderPlus size={28} />}
            title="还没有影片库"
            description="添加一个放有电影或剧集的文件夹，系统会自动查找影片。"
            action={(
              <Button variant="primary" onClick={() => setShowCreateModal(true)}>
                <FolderPlus size={16} />
                添加影片文件夹
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

function LibraryRow({
  library,
  isScanning,
  isDeleting,
  progress,
  scrape,
  phase,
  failure,
  activeMenu,
  onScan,
  onDelete,
  onMenu,
  onCloseMenu,
  onEdit,
  onReindex,
}: {
  library: Library
  isScanning: boolean
  isDeleting: boolean
  progress?: ScanProgressData
  scrape?: ScrapeProgressData
  phase?: ScanPhaseData
  failure?: string
  activeMenu: boolean
  onScan: () => void
  onDelete: () => void
  onMenu: () => void
  onCloseMenu: () => void
  onEdit: () => void
  onReindex: () => void
}) {
  const typeConfig = TYPE_CONFIG[library.type] || TYPE_CONFIG.movie
  const TypeIcon = typeConfig.icon
  const paths = getLibraryPaths(library)
  const path = paths.length > 1 ? `${paths[0]} 等 ${paths.length} 个文件夹` : paths[0] || '未配置文件夹'
  const isScraping = phase?.phase === 'scraping' || (!phase && scrape)
  const updatingLabel = isScraping
    ? '正在匹配海报与资料'
    : phase?.phase === 'ai_organizing' ? '正在整理影片' : '正在查找影片'
  const total = isScraping ? (scrape?.total || phase?.total) : (phase?.total || progress?.total)
  const current = isScraping ? (scrape?.current ?? phase?.current) : (phase?.current ?? progress?.current)
  const showProgress = total && typeof current === 'number' && Number.isFinite(current)

  return (
    <article aria-label={library.name} className="nv-library-card">
      <div className="nv-library-card-main">
        <div className="nv-library-card-info">
          <span className="nv-library-card-icon"><TypeIcon size={20} aria-hidden="true" /></span>
          <div className="nv-library-card-copy">
            <h3 className="nv-library-card-title">{library.name}</h3>
            <p className="nv-library-card-meta">
              {typeConfig.label}{typeof library.media_count === 'number' ? ` · ${library.media_count} 部影片` : ''}
              {!isScanning && !isDeleting && !failure && (library.last_scan
                ? ` · 已更新 ${new Date(library.last_scan).toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })}`
                : ' · 尚未更新')}
            </p>
            <p className="nv-library-card-path" title={paths.join('\n')}><span aria-hidden="true">↳</span> {path}</p>
          </div>
        </div>

        <div className="nv-library-card-actions">
          <Button variant="secondary" size="sm" onClick={onScan} disabled={isScanning || isDeleting}>
            <RefreshCw size={14} aria-hidden="true" className={isScanning ? 'animate-spin' : undefined} />
            更新影片
          </Button>
          <div className="relative">
            <Button variant="ghost" size="sm" iconOnly onClick={onMenu} disabled={isDeleting} aria-label="更多操作" aria-expanded={activeMenu} aria-haspopup="true">
              <MoreHorizontal size={16} aria-hidden="true" />
            </Button>
            {activeMenu && !isDeleting && (
              <>
                <div className="fixed inset-0 z-[var(--nv-z-dropdown)]" onClick={onCloseMenu} />
                <div className="absolute right-0 top-full z-[calc(var(--nv-z-dropdown)+1)] mt-1 w-44 overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] py-1 shadow-[var(--nv-shadow-elevated)]">
                  <Button variant="ghost" size="sm" className="w-full !justify-start" onClick={onEdit}>
                    <Pencil size={14} aria-hidden="true" />编辑影片库
                  </Button>
                  <Button variant="ghost" size="sm" className="w-full !justify-start" onClick={onReindex} disabled={isScanning}>
                    <RotateCcw size={14} aria-hidden="true" />重建索引
                  </Button>
                  <Button variant="ghost" size="sm" className="w-full !justify-start text-[var(--nv-status-danger)]" onClick={onDelete}>
                    <Trash2 size={14} aria-hidden="true" />删除影片库
                  </Button>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {isDeleting ? (
        <p role="status" className="text-sm text-[var(--nv-text-secondary)]">正在移除影片库…</p>
      ) : isScanning ? (
        <div role="status" className="space-y-1 text-sm text-[var(--nv-text-secondary)]">
          <p>{updatingLabel}{showProgress ? ` · ${current}/${total}` : ''}</p>
          <progress
            aria-label={updatingLabel}
            max={showProgress ? total : undefined}
            value={showProgress ? Math.max(0, Math.min(current, total)) : undefined}
            className="block h-1.5 w-full accent-[var(--nv-action-primary)]"
          />
        </div>
      ) : failure ? (
        <div role="alert" className="flex flex-wrap items-center gap-x-3 gap-y-1">
          <AdminStatus tone="danger">更新失败</AdminStatus>
          <p className="text-sm text-[var(--nv-text-secondary)]">{failure}</p>
          <Button variant="secondary" size="sm" onClick={onEdit}>检查路径</Button>
        </div>
      ) : null}
    </article>
  )
}

export default LibraryManager
