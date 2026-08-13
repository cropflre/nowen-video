import { useCallback, useState } from 'react'
import type { Media } from '@/types'
import {
  AlertCircle,
  Check,
  CheckSquare,
  ChevronRight,
  Copy,
  Edit3,
  Eye,
  FileVideo,
  Film,
  Folder,
  FolderOpen,
  FolderPlus,
  Loader2,
  Pencil,
  Play,
  RefreshCw,
  Sparkles,
  Square,
  Trash2,
  Tv,
} from 'lucide-react'
import { formatFileSize } from './constants'
import { streamApi } from '@/api/stream'
import Pagination from '@/components/Pagination'
import { Button, EmptyState, Tag } from '@/components/design-system'
import ContextMenu from './ContextMenu'
import type { ContextMenuItem } from './ContextMenu'

interface FileListViewProps {
  files: Media[]
  loading: boolean
  viewMode: 'table' | 'grid'
  selectedIds: Set<string>
  onToggleSelect: (id: string) => void
  onToggleSelectAll: () => void
  onViewDetail: (media: Media) => void
  onEdit: (media: Media) => void
  onScrape: (id: string) => void
  onDelete: (id: string) => void
  page: number
  totalPages: number
  total: number
  pageSize: number
  pageSizeOptions: number[]
  onPageChange: (page: number) => void
  onPageSizeChange: (size: number) => void
  subFolders?: string[]
  currentFolderPath?: string
  onNavigateFolder?: (path: string) => void
  onPlayFile?: (media: Media) => void
  onCopyFilePath?: (path: string) => void
  onCreateSubFolder?: (parentPath: string) => void
  onRenameSubFolder?: (folderPath: string) => void
  onDeleteSubFolder?: (folderPath: string) => void
  onRefreshSubFolder?: () => void
  onCopyFolderPath?: (path: string) => void
}

export default function FileListView({
  files,
  loading,
  viewMode,
  selectedIds,
  onToggleSelect,
  onToggleSelectAll,
  onViewDetail,
  onEdit,
  onScrape,
  onDelete,
  page,
  totalPages,
  total,
  pageSize,
  pageSizeOptions,
  onPageChange,
  onPageSizeChange,
  subFolders,
  currentFolderPath,
  onNavigateFolder,
  onPlayFile,
  onCopyFilePath,
  onCreateSubFolder,
  onRenameSubFolder,
  onDeleteSubFolder,
  onRefreshSubFolder,
  onCopyFolderPath,
}: FileListViewProps) {
  const [ctxMenu, setCtxMenu] = useState<{
    visible: boolean
    x: number
    y: number
    type: 'file' | 'folder'
    media?: Media
    folderPath?: string
  }>({ visible: false, x: 0, y: 0, type: 'file' })

  const closeCtxMenu = useCallback(() => {
    setCtxMenu((prev) => ({ ...prev, visible: false }))
  }, [])

  const handleFileContextMenu = useCallback((event: React.MouseEvent, file: Media) => {
    event.preventDefault()
    event.stopPropagation()
    setCtxMenu({ visible: true, x: event.clientX, y: event.clientY, type: 'file', media: file })
  }, [])

  const handleFolderContextMenu = useCallback((event: React.MouseEvent, folderFullPath: string) => {
    event.preventDefault()
    event.stopPropagation()
    setCtxMenu({ visible: true, x: event.clientX, y: event.clientY, type: 'folder', folderPath: folderFullPath })
  }, [])

  const getFileMenuItems = useCallback((): ContextMenuItem[] => {
    const file = ctxMenu.media
    if (!file) return []
    return [
      {
        key: 'play',
        label: '播放/预览',
        icon: <Play size={14} />,
        onClick: () => onPlayFile?.(file) || onViewDetail(file),
      },
      {
        key: 'detail',
        label: '查看详情',
        icon: <Eye size={14} />,
        onClick: () => onViewDetail(file),
      },
      {
        key: 'edit',
        label: '编辑信息',
        icon: <Edit3 size={14} />,
        divider: true,
        onClick: () => onEdit(file),
      },
      {
        key: 'scrape',
        label: '重新刮削',
        icon: <Sparkles size={14} />,
        onClick: () => onScrape(file.id),
      },
      {
        key: 'copy-path',
        label: '复制文件路径',
        icon: <Copy size={14} />,
        divider: true,
        onClick: () => onCopyFilePath?.(file.file_path),
      },
      {
        key: 'delete',
        label: '删除文件',
        icon: <Trash2 size={14} />,
        danger: true,
        divider: true,
        onClick: () => onDelete(file.id),
      },
    ]
  }, [ctxMenu.media, onPlayFile, onViewDetail, onEdit, onScrape, onDelete, onCopyFilePath])

  const getFolderMenuItems = useCallback((): ContextMenuItem[] => {
    const folderPath = ctxMenu.folderPath
    if (!folderPath) return []
    return [
      {
        key: 'open',
        label: '打开文件夹',
        icon: <FolderOpen size={14} />,
        onClick: () => onNavigateFolder?.(folderPath),
      },
      {
        key: 'create',
        label: '新建子文件夹',
        icon: <FolderPlus size={14} />,
        divider: true,
        disabled: !onCreateSubFolder,
        onClick: () => onCreateSubFolder?.(folderPath),
      },
      {
        key: 'rename',
        label: '重命名',
        icon: <Pencil size={14} />,
        disabled: !onRenameSubFolder,
        onClick: () => onRenameSubFolder?.(folderPath),
      },
      {
        key: 'refresh',
        label: '刷新',
        icon: <RefreshCw size={14} />,
        divider: true,
        disabled: !onRefreshSubFolder,
        onClick: () => onRefreshSubFolder?.(),
      },
      {
        key: 'copy-path',
        label: '复制路径',
        icon: <Copy size={14} />,
        onClick: () => onCopyFolderPath?.(folderPath),
      },
      {
        key: 'delete',
        label: '删除文件夹',
        icon: <Trash2 size={14} />,
        danger: true,
        divider: true,
        disabled: !onDeleteSubFolder,
        onClick: () => onDeleteSubFolder?.(folderPath),
      },
    ]
  }, [ctxMenu.folderPath, onNavigateFolder, onCreateSubFolder, onRenameSubFolder, onDeleteSubFolder, onRefreshSubFolder, onCopyFolderPath])

  if (loading) {
    return (
      <div className="flex min-h-52 items-center justify-center" role="status" aria-live="polite">
        <div className="flex flex-col items-center gap-3 text-[var(--nv-text-tertiary)]">
          <Loader2 size={28} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
          <span className="text-sm">正在加载文件...</span>
        </div>
      </div>
    )
  }

  const hasSubFolders = Boolean(subFolders?.length)

  if (files.length === 0 && !hasSubFolders) {
    return (
      <EmptyState
        className="min-h-52 border-y border-[var(--nv-border-subtle)]"
        icon={<FolderOpen size={28} />}
        title="暂无影视文件"
        description={'点击“导入文件”或“扫描目录”开始添加'}
      />
    )
  }

  const isScraped = (file: Media) => {
    const status = file.scrape_status
    if (status === 'scraped' || status === 'partial' || status === 'manual') return true
    if (!status) return file.tmdb_id > 0 || file.bangumi_id > 0 || (file.douban_id && file.douban_id !== '')
    return false
  }

  const renderScrapeBadge = (file: Media) => {
    const status = file.scrape_status
    if (status === 'partial') {
      return <Tag tone="warning"><AlertCircle size={11} />部分刮削</Tag>
    }
    if (status === 'failed') {
      return <Tag tone="danger"><AlertCircle size={11} />刮削失败</Tag>
    }
    if (status === 'manual') {
      return <Tag><Check size={11} />手动锁定</Tag>
    }
    if (isScraped(file)) {
      return <Tag tone="success"><Check size={11} />已刮削</Tag>
    }
    return <Tag tone="warning"><AlertCircle size={11} />未刮削</Tag>
  }

  const SubFolderCards = () => {
    if (!hasSubFolders || !onNavigateFolder || !currentFolderPath) return null
    const normalizedCurrent = currentFolderPath.replace(/\\/g, '/')
    return (
      <div className="mb-4 grid border-y border-[var(--nv-border-subtle)] sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
        {subFolders!.map((folder, index) => (
          <button
            key={folder}
            type="button"
            onClick={() => {
              const separator = normalizedCurrent.endsWith('/') ? '' : '/'
              onNavigateFolder(normalizedCurrent + separator + folder)
            }}
            onContextMenu={(event) => {
              const separator = normalizedCurrent.endsWith('/') ? '' : '/'
              handleFolderContextMenu(event, normalizedCurrent + separator + folder)
            }}
            className={`group flex min-w-0 items-center gap-2 px-2 py-2.5 text-left transition-colors duration-150 hover:bg-[var(--nv-fill-hover)] ${index > 0 ? 'border-t border-[var(--nv-border-subtle)] sm:border-t-0' : ''} sm:[&:nth-child(n+3)]:border-t sm:[&:nth-child(n+3)]:border-[var(--nv-border-subtle)] lg:[&:nth-child(n+3)]:border-t-0 lg:[&:nth-child(n+4)]:border-t lg:[&:nth-child(n+4)]:border-[var(--nv-border-subtle)] xl:[&:nth-child(n+4)]:border-t-0 xl:[&:nth-child(n+5)]:border-t xl:[&:nth-child(n+5)]:border-[var(--nv-border-subtle)]`}
          >
            <Folder size={17} className="shrink-0 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
            <span className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{folder}</span>
            <ChevronRight size={14} className="ml-auto shrink-0 text-[var(--nv-text-tertiary)] transition-transform duration-150 group-hover:translate-x-0.5" aria-hidden="true" />
          </button>
        ))}
      </div>
    )
  }

  return (
    <>
      <SubFolderCards />

      {files.length === 0 ? (
        <EmptyState
          className="min-h-40 border-y border-[var(--nv-border-subtle)] py-8"
          icon={<FolderOpen size={24} />}
          title="当前文件夹下无直接文件"
          description="请继续浏览子文件夹。"
        />
      ) : viewMode === 'table' ? (
        <div className="overflow-hidden border-y border-[var(--nv-border-subtle)]">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[var(--nv-border-subtle)] text-[var(--nv-text-secondary)]">
                  <th className="w-10 px-3 py-2.5 text-left">
                    <button
                      type="button"
                      onClick={onToggleSelectAll}
                      className="rounded-[var(--nv-radius-sm)] p-1 text-[var(--nv-text-tertiary)] transition-colors duration-150 hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-primary)]"
                      aria-label={selectedIds.size === files.length ? '取消全选' : '全选当前页'}
                      aria-pressed={selectedIds.size === files.length}
                    >
                      {selectedIds.size === files.length
                        ? <CheckSquare size={16} className="text-[var(--nv-text-primary)]" />
                        : <Square size={16} />}
                    </button>
                  </th>
                  <th className="px-3 py-2.5 text-left font-medium">标题</th>
                  <th className="hidden px-3 py-2.5 text-left font-medium md:table-cell">类型</th>
                  <th className="hidden px-3 py-2.5 text-left font-medium lg:table-cell">年份</th>
                  <th className="hidden px-3 py-2.5 text-left font-medium lg:table-cell">评分</th>
                  <th className="hidden px-3 py-2.5 text-left font-medium xl:table-cell">大小</th>
                  <th className="hidden px-3 py-2.5 text-left font-medium xl:table-cell">状态</th>
                  <th className="px-3 py-2.5 text-right font-medium">操作</th>
                </tr>
              </thead>
              <tbody>
                {files.map((file) => {
                  const selected = selectedIds.has(file.id)
                  return (
                    <tr
                      key={file.id}
                      className={`border-b border-[var(--nv-border-subtle)] transition-colors duration-150 last:border-b-0 ${selected ? 'bg-[var(--nv-fill-active)]' : 'hover:bg-[var(--nv-fill-hover)]'}`}
                      onContextMenu={(event) => handleFileContextMenu(event, file)}
                    >
                      <td className="px-3 py-3">
                        <button
                          type="button"
                          onClick={() => onToggleSelect(file.id)}
                          className="rounded-[var(--nv-radius-sm)] p-1 text-[var(--nv-text-tertiary)] transition-colors duration-150 hover:bg-[var(--nv-fill-hover)] hover:text-[var(--nv-text-primary)]"
                          aria-label={selected ? `取消选择 ${file.title}` : `选择 ${file.title}`}
                          aria-pressed={selected}
                        >
                          {selected
                            ? <CheckSquare size={16} className="text-[var(--nv-text-primary)]" />
                            : <Square size={16} />}
                        </button>
                      </td>
                      <td className="px-3 py-3">
                        <div className="flex items-center gap-3">
                          <img
                            src={streamApi.getPosterUrl(file.id)}
                            alt=""
                            className="h-12 w-8 shrink-0 rounded-[var(--nv-radius-sm)] object-cover"
                            onError={(event) => {
                              const image = event.target as HTMLImageElement
                              image.style.display = 'none'
                              image.nextElementSibling?.classList.remove('hidden')
                            }}
                          />
                          <div className="hidden h-12 w-8 shrink-0 items-center justify-center rounded-[var(--nv-radius-sm)] bg-[var(--nv-bg-surface-soft)]">
                            <FileVideo size={16} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />
                          </div>
                          <div className="min-w-0">
                            <div className="flex items-center gap-2">
                              <span className="truncate font-medium text-[var(--nv-text-primary)]">{file.title}</span>
                              {file.media_type === 'episode' && file.episode_num > 0 && (
                                <Tag className="shrink-0 text-[10px]">
                                  {file.season_num > 0
                                    ? `S${String(file.season_num).padStart(2, '0')}E${String(file.episode_num).padStart(2, '0')}`
                                    : `EP${String(file.episode_num).padStart(2, '0')}`}
                                </Tag>
                              )}
                            </div>
                            {file.media_type === 'episode' && file.episode_title ? (
                              <div className="mt-0.5 truncate text-xs text-[var(--nv-text-secondary)]">{file.episode_title}</div>
                            ) : file.orig_title && file.orig_title !== file.title ? (
                              <div className="mt-0.5 truncate text-xs text-[var(--nv-text-tertiary)]">{file.orig_title}</div>
                            ) : null}
                            <div className="mt-0.5 truncate text-xs text-[var(--nv-text-tertiary)]">
                              {file.file_path.split(/[\\/]/).pop()}
                            </div>
                          </div>
                        </div>
                      </td>
                      <td className="hidden px-3 py-3 md:table-cell">
                        <Tag>
                          {file.media_type === 'movie' ? <Film size={12} /> : <Tv size={12} />}
                          {file.media_type === 'movie' ? '电影' : '剧集'}
                        </Tag>
                      </td>
                      <td className="hidden px-3 py-3 text-[var(--nv-text-secondary)] lg:table-cell">{file.year || '-'}</td>
                      <td className="hidden px-3 py-3 lg:table-cell">
                        {file.rating > 0
                          ? <span className="font-medium text-[var(--nv-status-rating)]">★ {file.rating.toFixed(1)}</span>
                          : <span className="text-[var(--nv-text-tertiary)]">-</span>}
                      </td>
                      <td className="hidden px-3 py-3 text-[var(--nv-text-secondary)] xl:table-cell">
                        {file.file_size > 0 ? formatFileSize(file.file_size) : '-'}
                      </td>
                      <td className="hidden px-3 py-3 xl:table-cell">{renderScrapeBadge(file)}</td>
                      <td className="px-3 py-3 text-right">
                        <div className="flex items-center justify-end gap-0.5">
                          <Button variant="ghost" size="sm" iconOnly onClick={() => onViewDetail(file)} title="查看详情" aria-label={`查看 ${file.title} 详情`}>
                            <Eye size={14} />
                          </Button>
                          <Button variant="ghost" size="sm" iconOnly onClick={() => onEdit(file)} title="编辑" aria-label={`编辑 ${file.title}`}>
                            <Edit3 size={14} />
                          </Button>
                          <Button variant="ghost" size="sm" iconOnly onClick={() => onScrape(file.id)} title="刮削" aria-label={`刮削 ${file.title}`}>
                            <Sparkles size={14} />
                          </Button>
                          <Button variant="ghost" size="sm" iconOnly onClick={() => onDelete(file.id)} className="text-[var(--nv-text-tertiary)] hover:text-[var(--nv-status-danger)]" title="删除" aria-label={`删除 ${file.title}`}>
                            <Trash2 size={14} />
                          </Button>
                        </div>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        <div className="grid grid-cols-2 gap-x-4 gap-y-5 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6">
          {files.map((file) => {
            const selected = selectedIds.has(file.id)
            return (
              <article
                key={file.id}
                className="group relative min-w-0 cursor-pointer transition-transform duration-150 hover:-translate-y-0.5"
                onClick={() => onViewDetail(file)}
                onContextMenu={(event) => handleFileContextMenu(event, file)}
              >
                <div className={`relative aspect-[2/3] overflow-hidden rounded-[var(--nv-radius-card)] bg-[var(--nv-bg-surface-soft)] shadow-[var(--nv-shadow-card)] transition-shadow duration-150 group-hover:shadow-[var(--nv-shadow-card-hover)] ${selected ? 'ring-1 ring-[var(--nv-text-secondary)]' : ''}`}>
                  <img
                    src={streamApi.getPosterUrl(file.id)}
                    alt=""
                    className="h-full w-full object-cover"
                    onError={(event) => { (event.target as HTMLImageElement).style.display = 'none' }}
                  />

                  <button
                    type="button"
                    className={`absolute left-2 top-2 z-10 grid h-7 w-7 place-items-center rounded-full bg-black/60 text-white/80 transition-[background-color,color,opacity] duration-150 hover:bg-black/75 hover:text-white ${selected ? 'opacity-100' : 'opacity-70 sm:opacity-0 sm:group-hover:opacity-100 sm:group-focus-within:opacity-100'}`}
                    onClick={(event) => {
                      event.stopPropagation()
                      onToggleSelect(file.id)
                    }}
                    aria-label={selected ? `取消选择 ${file.title}` : `选择 ${file.title}`}
                    aria-pressed={selected}
                  >
                    {selected ? <Check size={15} aria-hidden="true" /> : <Square size={15} aria-hidden="true" />}
                  </button>

                  <div className="absolute right-2 top-2">{renderScrapeBadge(file)}</div>
                  <div className="absolute inset-0 flex items-center justify-center gap-1.5 bg-black/35 opacity-0 transition-opacity duration-150 group-hover:opacity-100 group-focus-within:opacity-100">
                    <Button
                      variant="secondary"
                      size="sm"
                      iconOnly
                      onClick={(event) => { event.stopPropagation(); onEdit(file) }}
                      title="编辑"
                      aria-label={`编辑 ${file.title}`}
                    >
                      <Edit3 size={15} />
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      iconOnly
                      onClick={(event) => { event.stopPropagation(); onScrape(file.id) }}
                      title="刮削"
                      aria-label={`刮削 ${file.title}`}
                    >
                      <Sparkles size={15} />
                    </Button>
                    <Button
                      variant="secondary"
                      size="sm"
                      iconOnly
                      onClick={(event) => { event.stopPropagation(); onDelete(file.id) }}
                      className="hover:text-[var(--nv-status-danger)]"
                      title="删除"
                      aria-label={`删除 ${file.title}`}
                    >
                      <Trash2 size={15} />
                    </Button>
                  </div>
                </div>

                <div className="px-0.5 pt-2.5">
                  <div className="flex min-w-0 items-center gap-1.5">
                    <div className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{file.title}</div>
                    {file.media_type === 'episode' && file.episode_num > 0 && (
                      <span className="shrink-0 text-[10px] font-medium text-[var(--nv-text-tertiary)]">
                        {file.season_num > 0
                          ? `S${String(file.season_num).padStart(2, '0')}E${String(file.episode_num).padStart(2, '0')}`
                          : `EP${String(file.episode_num).padStart(2, '0')}`}
                      </span>
                    )}
                  </div>
                  {file.media_type === 'episode' && file.episode_title && (
                    <div className="mt-0.5 truncate text-xs text-[var(--nv-text-secondary)]">{file.episode_title}</div>
                  )}
                  <div className="mt-1.5 flex flex-wrap items-center gap-1.5 text-xs text-[var(--nv-text-tertiary)]">
                    <span>{file.year || '-'}</span>
                    {file.rating > 0 && <span className="text-[var(--nv-status-rating)]">★ {file.rating.toFixed(1)}</span>}
                    <span>{file.media_type === 'movie' ? '电影' : '剧集'}</span>
                  </div>
                </div>
              </article>
            )
          })}
        </div>
      )}

      {files.length > 0 && (
        <Pagination
          page={page}
          totalPages={totalPages}
          total={total}
          pageSize={pageSize}
          pageSizeOptions={pageSizeOptions}
          onPageChange={onPageChange}
          onPageSizeChange={onPageSizeChange}
          showTotal
          showJumper
        />
      )}

      <ContextMenu
        visible={ctxMenu.visible}
        x={ctxMenu.x}
        y={ctxMenu.y}
        items={ctxMenu.type === 'file' ? getFileMenuItems() : getFolderMenuItems()}
        onClose={closeCtxMenu}
      />
    </>
  )
}
