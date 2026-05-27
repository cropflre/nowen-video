import { useState } from 'react'
import { libraryApi } from '@/api'
import type { Library, CreateLibraryRequest } from '@/types'
import { getLibraryPaths } from '@/types'
import type { ScanProgressData, ScrapeProgressData, ScanPhaseData } from '@/hooks/useWebSocket'
import { useToast } from './Toast'
import { useDialog } from './Dialog'
import CreateLibraryModal from './CreateLibraryModal'
import EditLibraryModal from './EditLibraryModal'
import {
  FolderPlus,
  RefreshCw,
  Trash2,
  HardDrive,
  Film,
  Tv,
  Layers,
  Video,
  ArrowUpDown,
  ScanLine,
  MoreHorizontal,
  Calendar,
  FolderOpen,
  ChevronRight,
  RotateCcw,
  Pencil,
} from 'lucide-react'
import clsx from 'clsx'

// 类型配置映射
const TYPE_CONFIG: Record<string, { label: string; icon: typeof Film; color: string; bg: string }> = {
  movie: { label: '电影', icon: Film, color: 'var(--neon-blue)', bg: 'var(--neon-blue-8)' },
  tvshow: { label: '电视节目', icon: Tv, color: 'var(--neon-purple)', bg: 'var(--neon-purple-8)' },
  mixed: { label: '混合影片', icon: Layers, color: '#F59E0B', bg: 'rgba(245, 158, 11, 0.08)' },
  other: { label: '其他视频', icon: Video, color: '#10B981', bg: 'rgba(16, 185, 129, 0.08)' },
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

function clampPercent(n: number) {
  if (!Number.isFinite(n)) return 0
  return Math.max(0, Math.min(100, n))
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

  // 排序逻辑
  const sortedLibraries = [...libraries].sort((a, b) => {
    let cmp = 0
    if (sortBy === 'name') cmp = a.name.localeCompare(b.name)
    else if (sortBy === 'type') cmp = a.type.localeCompare(b.type)
    else cmp = new Date(b.created_at).getTime() - new Date(a.created_at).getTime()
    return sortAsc ? -cmp : cmp
  })

  const handleCreate = async (data: CreateLibraryRequest) => {
    await libraryApi.create(data)
    const res = await libraryApi.list()
    setLibraries(res.data.data || [])
  }

  const handleScan = async (id: string) => {
    setScanning((s) => new Set(s).add(id))
    try {
      await libraryApi.scan(id)
    } catch (err: any) {
      setScanning((s) => {
        const ns = new Set(s)
        ns.delete(id)
        return ns
      })
      const msg = err?.response?.data?.error || '扫描启动失败'
      toast.error(msg)
    }
  }

  const handleScanAll = async () => {
    const toScan = libraries.filter((lib) => !scanning.has(lib.id))
    if (toScan.length === 0) {
      toast.info('所有媒体库已在扫描中')
      return
    }
    setScanAllLoading(true)
    try {
      for (const lib of toScan) {
        await handleScan(lib.id)
      }
      toast.success(`已启动 ${toScan.length} 个媒体库扫描`)
    } finally {
      setScanAllLoading(false)
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await dialog.confirm({
      title: '删除媒体库',
      message: '确定删除此媒体库？关联的媒体记录也会被清除。',
      confirmText: '删除',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await libraryApi.delete(id)
      setLibraries((libs) => libs.filter((l) => l.id !== id))
    } catch {
      toast.error('删除失败')
    }
  }

  const handleReindex = async (id: string) => {
    const ok = await dialog.confirm({
      title: '重建索引',
      message: '确定重建索引？这将清除现有媒体记录并重新扫描文件。',
      confirmText: '重建',
      variant: 'warning',
    })
    if (!ok) return
    setScanning((s) => new Set(s).add(id))
    try {
      await libraryApi.reindex(id)
    } catch {
      setScanning((s) => {
        const ns = new Set(s)
        ns.delete(id)
        return ns
      })
      toast.error('重建索引失败')
    }
  }

  const toggleSort = (field: typeof sortBy) => {
    if (sortBy === field) {
      setSortAsc(!sortAsc)
    } else {
      setSortBy(field)
      setSortAsc(false)
    }
  }

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
    <section>
      {/* ===== 区域头部 — 飞牛风格工具栏 ===== */}
      <div className="mb-5 flex flex-wrap items-center gap-3">
        {/* 标题 */}
        <h2
          className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide"
          style={{ color: 'var(--text-primary)' }}
        >
          <HardDrive size={20} className="text-neon/60" />
          媒体库管理
        </h2>

        <div className="ml-auto flex items-center gap-2">
          {/* 新增媒体库按钮 — 主要操作 */}
          <button
            onClick={() => setShowCreateModal(true)}
            className="btn-primary gap-1.5 px-4 py-2 text-sm"
          >
            <FolderPlus size={16} />
            新增媒体库
          </button>

          {/* 排序按钮 */}
          <div className="relative">
            <button
              onClick={() => toggleSort(sortBy === 'name' ? 'created' : sortBy === 'created' ? 'type' : 'name')}
              className="flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all"
              style={{
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                background: 'transparent',
              }}
              title={`排序: ${sortBy === 'name' ? '名称' : sortBy === 'type' ? '类型' : '创建时间'}`}
            >
              <ArrowUpDown size={14} />
              排序
            </button>
          </div>

          {/* 扫描全部按钮 */}
          {libraries.length > 0 && (
            <button
              onClick={handleScanAll}
              disabled={scanAllLoading}
              className="flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all disabled:opacity-40"
              style={{
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                background: 'transparent',
              }}
              title="扫描所有媒体库文件"
            >
              {scanAllLoading ? (
                <RefreshCw size={14} className="animate-spin" />
              ) : (
                <ScanLine size={14} />
              )}
              {scanAllLoading ? '扫描中...' : '扫描媒体库文件'}
            </button>
          )}
        </div>
      </div>

      {/* ===== 媒体库表格 — 飞牛风格列表 ===== */}
      {libraries.length > 0 ? (
        <div
          className="rounded-xl"
          style={{
            border: '1px solid var(--border-default)',
            background: 'var(--bg-card)',
            overflow: 'visible',
          }}
        >
          {/* 表头 */}
          <div
            className="grid gap-4 px-5 py-3 text-xs font-semibold uppercase tracking-wider rounded-t-xl"
            style={{
              gridTemplateColumns: '2fr 2fr 1fr 1.5fr 120px',
              borderBottom: '1px solid var(--border-default)',
              color: 'var(--text-tertiary)',
              background: 'var(--nav-hover-bg)',
            }}
          >
            <button
              className="flex items-center gap-1 text-left hover:text-[var(--text-primary)] transition-colors"
              onClick={() => toggleSort('name')}
            >
              媒体库
              {sortBy === 'name' && <ChevronRight size={12} className={clsx('transition-transform', sortAsc ? '-rotate-90' : 'rotate-90')} />}
            </button>
            <span>媒体文件夹</span>
            <button
              className="flex items-center gap-1 hover:text-[var(--text-primary)] transition-colors"
              onClick={() => toggleSort('type')}
            >
              类型
              {sortBy === 'type' && <ChevronRight size={12} className={clsx('transition-transform', sortAsc ? '-rotate-90' : 'rotate-90')} />}
            </button>
            <button
              className="flex items-center gap-1 hover:text-[var(--text-primary)] transition-colors"
              onClick={() => toggleSort('created')}
            >
              最近更新
              {sortBy === 'created' && <ChevronRight size={12} className={clsx('transition-transform', sortAsc ? '-rotate-90' : 'rotate-90')} />}
            </button>
            <span className="text-center">操作</span>
          </div>

          {/* 列表项 */}
          {sortedLibraries.map((lib) => {
            const typeConfig = TYPE_CONFIG[lib.type] || TYPE_CONFIG.movie
            const TypeIcon = typeConfig.icon
            const isScanning = scanning.has(lib.id)
            const progress = scanProgress[lib.id]
            const scrape = scrapeProgress[lib.id]
            const phase = scanPhase[lib.id]

            const activeStage: MainScanStage =
              phase?.phase === 'ai_organizing'
                ? 'ai_organizing'
                : phase?.phase === 'scraping' || (!phase && scrape)
                  ? 'scraping'
                  : 'scanning'
            const activeStageIndex = MAIN_SCAN_STAGES.findIndex((s) => s.id === activeStage)
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
              : activeStageIndex > 0
                ? 100
                : 35
            const stageMessage = activeStage === 'scraping' && scrape
              ? `元数据刮削 [${scrape.current}/${scrape.total}] ${scrape.media_title || ''}`
              : activeStage === 'ai_organizing' && phase?.total
                ? `AI整理 [${phase.current || 0}/${phase.total}]`
                : progress?.message || phase?.message || '正在入库...'

            return (
              <div key={lib.id} className="group relative">
                <div
                  className="grid items-center gap-4 px-5 py-4 transition-colors duration-200"
                  style={{
                    gridTemplateColumns: '2fr 2fr 1fr 1.5fr 120px',
                    borderBottom: '1px solid var(--border-default)',
                  }}
                  onMouseEnter={(e) => {
                    e.currentTarget.style.background = 'var(--nav-hover-bg)'
                  }}
                  onMouseLeave={(e) => {
                    e.currentTarget.style.background = 'transparent'
                  }}
                >
                  {/* 媒体库名称 */}
                  <div className="flex items-center gap-3 min-w-0">
                    <div
                      className="flex h-10 w-10 flex-shrink-0 items-center justify-center rounded-lg"
                      style={{ background: typeConfig.bg, color: typeConfig.color }}
                    >
                      <TypeIcon size={20} />
                    </div>
                    <div className="min-w-0">
                      <h3
                        className="truncate text-sm font-semibold"
                        style={{ color: 'var(--text-primary)' }}
                      >
                        {lib.name}
                      </h3>
                    {isScanning && (
                        <p className="mt-0.5 text-xs text-neon animate-pulse">
                          {stageLabel}：{stageMessage}
                        </p>
                      )}
                    </div>
                  </div>

                  {/* 文件夹路径 */}
                  <div className="flex items-center gap-1.5 min-w-0">
                    <FolderOpen
                      size={14}
                      className="flex-shrink-0"
                      style={{ color: 'var(--text-muted)' }}
                    />
                    {(() => {
                      const allPaths = getLibraryPaths(lib)
                      const pathTitle = allPaths.join('\n')
                      const displayText =
                        allPaths.length > 1
                          ? `${allPaths[0]}  +${allPaths.length - 1}`
                          : allPaths[0] || lib.path
                      return (
                        <span
                          className="truncate text-sm font-mono"
                          style={{ color: 'var(--text-secondary)' }}
                          title={pathTitle}
                        >
                          {displayText}
                        </span>
                      )
                    })()}
                  </div>

                  {/* 类型标签 */}
                  <div>
                    <span
                      className="inline-flex items-center gap-1 rounded-md px-2 py-0.5 text-xs font-semibold"
                      style={{
                        background: typeConfig.bg,
                        color: typeConfig.color,
                        border: `1px solid ${typeConfig.bg}`,
                      }}
                    >
                      {typeConfig.label}
                    </span>
                  </div>

                  {/* 更新时间 */}
                  <div className="flex items-center gap-1.5 text-sm" style={{ color: 'var(--text-tertiary)' }}>
                    <Calendar size={13} className="flex-shrink-0" />
                    <span>{formatDate(lib.last_scan)}</span>
                  </div>

                  {/* 操作按钮 */}
                  <div className="flex items-center justify-center gap-1">
                    {/* 扫描 */}
                    <button
                      onClick={() => handleScan(lib.id)}
                      disabled={isScanning}
                      className="rounded-lg p-2 transition-all hover:bg-[var(--nav-hover-bg)] disabled:opacity-40"
                      style={{ color: 'var(--text-tertiary)' }}
                      title="扫描媒体文件"
                    >
                      <RefreshCw
                        size={16}
                        className={clsx(
                          'transition-all',
                          isScanning && 'animate-spin text-neon'
                        )}
                      />
                    </button>

                    {/* 删除 */}
                    <button
                      onClick={() => handleDelete(lib.id)}
                      className="rounded-lg p-2 text-surface-500 transition-all hover:bg-red-500/5 hover:text-red-400"
                      title="删除媒体库"
                    >
                      <Trash2 size={16} />
                    </button>

                    {/* 更多操作 */}
                    <div className="relative">
                      <button
                        onClick={() => setActiveMenu(activeMenu === lib.id ? null : lib.id)}
                        className="rounded-lg p-2 transition-all hover:bg-[var(--nav-hover-bg)]"
                        style={{ color: 'var(--text-tertiary)' }}
                      >
                        <MoreHorizontal size={16} />
                      </button>

                      {/* 下拉菜单 */}
                      {activeMenu === lib.id && (
                        <>
                          <div className="fixed inset-0 z-30" onClick={() => setActiveMenu(null)} />
                          <div
                            className="absolute right-0 top-full z-40 mt-1 w-44 overflow-hidden rounded-xl py-1 animate-slide-up"
                            style={{
                              background: 'var(--bg-elevated)',
                              border: '1px solid var(--border-strong)',
                              boxShadow: 'var(--shadow-elevated)',
                            }}
                          >
                            <button
                              onClick={() => {
                                setActiveMenu(null)
                                setEditingLibrary(lib)
                              }}
                              className="flex w-full items-center gap-2 px-4 py-2.5 text-sm transition-colors hover:bg-[var(--nav-hover-bg)]"
                              style={{ color: 'var(--text-secondary)' }}
                            >
                              <Pencil size={14} />
                              编辑媒体库
                            </button>
                            <button
                              onClick={() => {
                                setActiveMenu(null)
                                handleReindex(lib.id)
                              }}
                              disabled={isScanning}
                              className="flex w-full items-center gap-2 px-4 py-2.5 text-sm transition-colors hover:bg-[var(--nav-hover-bg)] disabled:opacity-40"
                              style={{ color: 'var(--text-secondary)' }}
                            >
                              <RotateCcw size={14} />
                              重建索引
                            </button>
                          </div>
                        </>
                      )}
                    </div>
                  </div>
                </div>

                {/* 扫描进度条（扫描中显示）：固定展示 入库 → 元数据刮削 → AI整理 */}
                {isScanning && (progress || scrape || phase) && (
                  <div className="px-5 pb-3">
                    <div className="mb-2 grid grid-cols-3 gap-2">
                      {MAIN_SCAN_STAGES.map((stage, index) => {
                        const done = index < activeStageIndex
                        const active = index === activeStageIndex
                        return (
                          <div
                            key={stage.id}
                            className="rounded-lg px-2.5 py-2 transition-all"
                            style={{
                              border: active ? '1px solid var(--neon-blue)' : '1px solid var(--border-default)',
                              background: done
                                ? 'rgba(16, 185, 129, 0.10)'
                                : active
                                  ? 'var(--neon-blue-8)'
                                  : 'var(--bg-secondary)',
                            }}
                          >
                            <div className="flex items-center justify-between gap-1">
                              <span className="truncate text-[11px] font-semibold" style={{ color: active ? 'var(--neon-blue)' : done ? '#10B981' : 'var(--text-tertiary)' }}>
                                {index + 1}. {stage.short}
                              </span>
                              <span className="text-[10px]" style={{ color: active ? 'var(--neon-blue)' : done ? '#10B981' : 'var(--text-muted)' }}>
                                {done ? '完成' : active ? '进行中' : '等待'}
                              </span>
                            </div>
                          </div>
                        )
                      })}
                    </div>

                    <div className="flex items-center justify-between mb-1.5">
                      <span className="text-[11px] font-medium" style={{ color: 'var(--neon-blue)' }}>
                        {stageLabel}
                      </span>
                      <span className="text-[11px] font-mono" style={{ color: 'var(--text-tertiary)' }}>
                        {stageProgress.total > 0 ? `${stageProgress.current}/${stageProgress.total}` : `已发现 ${progress?.new_found || 0}`}
                      </span>
                    </div>
                    <div className="h-2 overflow-hidden rounded-full" style={{ background: 'var(--neon-blue-6)' }}>
                      <div
                        className="h-full rounded-full transition-all duration-500"
                        style={{
                          background: activeStage === 'scraping'
                            ? 'linear-gradient(90deg, var(--neon-purple), var(--neon-pink))'
                            : activeStage === 'ai_organizing'
                              ? 'linear-gradient(90deg, #10B981, var(--neon-blue))'
                              : 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                          width: `${stagePercent}%`,
                          animation: stageProgress.total <= 0 ? 'shimmer 2s linear infinite' : undefined,
                          backgroundSize: stageProgress.total <= 0 ? '200% 100%' : undefined,
                        }}
                      />
                    </div>
                    <div className="mt-1.5 flex items-center justify-between gap-3">
                      <span className="truncate text-[11px]" style={{ color: 'var(--text-muted)' }}>
                        {stageMessage}
                      </span>
                      {activeStage === 'scraping' && scrape && scrape.total > 0 && (
                        <span className="shrink-0 text-[11px] font-mono" style={{ color: 'var(--neon-purple)' }}>
                          成功:{scrape.success} 失败:{scrape.failed}
                        </span>
                      )}
                    </div>
                  </div>
                )}
              </div>
            )
          })}
        </div>
      ) : (
        /* ===== 空状态 ===== */
        <div
          className="flex flex-col items-center justify-center py-16 rounded-xl"
          style={{
            border: '2px dashed var(--border-default)',
            background: 'var(--nav-hover-bg)',
          }}
        >
          <div
            className="mb-5 flex h-16 w-16 items-center justify-center rounded-2xl animate-float"
            style={{
              background: 'var(--neon-blue-5)',
              border: '1px solid var(--neon-blue-10)',
            }}
          >
            <FolderPlus size={32} className="text-surface-600" />
          </div>
          <h3
            className="font-display text-base font-semibold tracking-wide"
            style={{ color: 'var(--text-secondary)' }}
          >
            还没有媒体库
          </h3>
          <p
            className="mt-1.5 mb-5 text-sm"
            style={{ color: 'var(--text-muted)' }}
          >
            添加媒体库后系统将自动扫描并索引您的视频文件
          </p>
          <button
            onClick={() => setShowCreateModal(true)}
            className="btn-primary gap-1.5 px-5 py-2.5 text-sm"
          >
            <FolderPlus size={16} />
            新增媒体库
          </button>
        </div>
      )}

      {/* ===== 创建媒体库弹窗 ===== */}
      <CreateLibraryModal
        open={showCreateModal}
        onClose={() => setShowCreateModal(false)}
        onCreate={handleCreate}
      />

      {/* ===== 编辑媒体库弹窗 ===== */}
      <EditLibraryModal
        open={!!editingLibrary}
        library={editingLibrary}
        onClose={() => setEditingLibrary(null)}
        onUpdate={(updated) => {
          setLibraries((libs) => libs.map((l) => (l.id === updated.id ? updated : l)))
          toast.success('媒体库已更新')
        }}
      />


    </section>
  )
}

export default LibraryManager

