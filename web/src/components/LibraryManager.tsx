import { useState } from 'react'
import { libraryApi } from '@/api'
import type { Library, CreateLibraryRequest } from '@/types'
import type { ScanProgressData, ScrapeProgressData } from '@/hooks/useWebSocket'
import CreateLibraryModal from './CreateLibraryModal'
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
} from 'lucide-react'
import clsx from 'clsx'

// 类型配置映射
const TYPE_CONFIG: Record<string, { label: string; icon: typeof Film; color: string; bg: string }> = {
  movie: { label: '电影', icon: Film, color: 'var(--neon-blue)', bg: 'rgba(0, 240, 255, 0.08)' },
  tvshow: { label: '电视节目', icon: Tv, color: 'var(--neon-purple)', bg: 'rgba(138, 43, 226, 0.08)' },
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
}

export default function LibraryManager({
  libraries,
  setLibraries,
  scanning,
  setScanning,
  scanProgress,
  scrapeProgress,
}: LibraryManagerProps) {
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [sortBy, setSortBy] = useState<'name' | 'created' | 'type'>('created')
  const [sortAsc, setSortAsc] = useState(false)
  const [activeMenu, setActiveMenu] = useState<string | null>(null)

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
    } catch {
      setScanning((s) => {
        const ns = new Set(s)
        ns.delete(id)
        return ns
      })
    }
  }

  const handleScanAll = async () => {
    for (const lib of libraries) {
      if (!scanning.has(lib.id)) {
        handleScan(lib.id)
      }
    }
  }

  const handleDelete = async (id: string) => {
    if (!confirm('确定删除此媒体库？关联的媒体记录也会被清除。')) return
    try {
      await libraryApi.delete(id)
      setLibraries((libs) => libs.filter((l) => l.id !== id))
    } catch {
      alert('删除失败')
    }
  }

  const handleReindex = async (id: string) => {
    if (!confirm('确定重建索引？这将清除现有媒体记录并重新扫描文件。')) return
    setScanning((s) => new Set(s).add(id))
    try {
      await libraryApi.reindex(id)
    } catch {
      setScanning((s) => {
        const ns = new Set(s)
        ns.delete(id)
        return ns
      })
      alert('重建索引失败')
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
              className="flex items-center gap-1.5 rounded-xl px-3 py-2 text-sm font-medium transition-all"
              style={{
                border: '1px solid var(--border-default)',
                color: 'var(--text-secondary)',
                background: 'transparent',
              }}
              title="扫描所有媒体库文件"
            >
              <ScanLine size={14} />
              扫描媒体库文件
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
                          {progress?.message || scrape?.message || '扫描中...'}
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
                    <span
                      className="truncate text-sm font-mono"
                      style={{ color: 'var(--text-secondary)' }}
                      title={lib.path}
                    >
                      {lib.path}
                    </span>
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

                {/* 扫描进度条（扫描中显示） */}
                {isScanning && (progress || scrape) && (
                  <div className="px-5 pb-2">
                    <div
                      className="h-1 overflow-hidden rounded-full"
                      style={{ background: 'rgba(0, 240, 255, 0.06)' }}
                    >
                      <div
                        className="h-full rounded-full transition-all duration-500"
                        style={{
                          background: scrape
                            ? 'linear-gradient(90deg, var(--neon-purple), var(--neon-pink))'
                            : 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                          width: scrape
                            ? `${scrape.total > 0 ? (scrape.current / scrape.total) * 100 : 0}%`
                            : '100%',
                          animation: !scrape ? 'shimmer 2s linear infinite' : undefined,
                          backgroundSize: !scrape ? '200% 100%' : undefined,
                        }}
                      />
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
              background: 'rgba(0, 240, 255, 0.05)',
              border: '1px solid rgba(0, 240, 255, 0.1)',
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
    </section>
  )
}
