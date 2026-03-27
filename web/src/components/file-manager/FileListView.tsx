import type { Media } from '@/types'
import {
  CheckSquare,
  Square,
  Film,
  Tv,
  FileVideo,
  Eye,
  Edit3,
  Sparkles,
  Trash2,
  Check,
  AlertCircle,
  FolderOpen,
  Loader2,
} from 'lucide-react'
import clsx from 'clsx'
import { formatFileSize } from './constants'

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
  // 分页
  page: number
  totalPages: number
  total: number
  onPageChange: (page: number) => void
}

export default function FileListView({
  files, loading, viewMode, selectedIds,
  onToggleSelect, onToggleSelectAll,
  onViewDetail, onEdit, onScrape, onDelete,
  page, totalPages, total, onPageChange,
}: FileListViewProps) {
  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 size={32} className="animate-spin text-neon" />
      </div>
    )
  }

  if (files.length === 0) {
    return (
      <div className="glass-panel rounded-xl p-12 text-center">
        <FolderOpen size={48} className="mx-auto mb-4 text-surface-500" />
        <p className="text-lg font-medium" style={{ color: 'var(--text-secondary)' }}>暂无影视文件</p>
        <p className="text-sm mt-1" style={{ color: 'var(--text-tertiary)' }}>点击"导入文件"或"扫描目录"开始添加</p>
      </div>
    )
  }

  const isScraped = (file: Media) => file.tmdb_id > 0 || file.bangumi_id > 0 || (file.douban_id && file.douban_id !== '')

  return (
    <>
      {viewMode === 'table' ? (
        /* 表格视图 */
        <div className="glass-panel rounded-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b" style={{ borderColor: 'var(--border-default)' }}>
                  <th className="px-3 py-3 text-left w-10">
                    <button onClick={onToggleSelectAll}>
                      {selectedIds.size === files.length ? <CheckSquare size={16} className="text-neon" /> : <Square size={16} className="text-surface-500" />}
                    </button>
                  </th>
                  <th className="px-3 py-3 text-left" style={{ color: 'var(--text-secondary)' }}>标题</th>
                  <th className="px-3 py-3 text-left hidden md:table-cell" style={{ color: 'var(--text-secondary)' }}>类型</th>
                  <th className="px-3 py-3 text-left hidden lg:table-cell" style={{ color: 'var(--text-secondary)' }}>年份</th>
                  <th className="px-3 py-3 text-left hidden lg:table-cell" style={{ color: 'var(--text-secondary)' }}>评分</th>
                  <th className="px-3 py-3 text-left hidden xl:table-cell" style={{ color: 'var(--text-secondary)' }}>大小</th>
                  <th className="px-3 py-3 text-left hidden xl:table-cell" style={{ color: 'var(--text-secondary)' }}>状态</th>
                  <th className="px-3 py-3 text-right" style={{ color: 'var(--text-secondary)' }}>操作</th>
                </tr>
              </thead>
              <tbody>
                {files.map(file => (
                  <tr key={file.id} className="border-b transition-colors hover:bg-white/[0.02]" style={{ borderColor: 'var(--border-default)' }}>
                    <td className="px-3 py-3">
                      <button onClick={() => onToggleSelect(file.id)}>
                        {selectedIds.has(file.id) ? <CheckSquare size={16} className="text-neon" /> : <Square size={16} className="text-surface-500" />}
                      </button>
                    </td>
                    <td className="px-3 py-3">
                      <div className="flex items-center gap-3">
                        {file.poster_path ? (
                          <img src={file.poster_path} alt="" className="w-8 h-12 rounded object-cover flex-shrink-0" />
                        ) : (
                          <div className="w-8 h-12 rounded bg-surface-800 flex items-center justify-center flex-shrink-0">
                            <FileVideo size={16} className="text-surface-500" />
                          </div>
                        )}
                        <div className="min-w-0">
                          <div className="font-medium truncate" style={{ color: 'var(--text-primary)' }}>{file.title}</div>
                          {file.orig_title && file.orig_title !== file.title && (
                            <div className="text-xs truncate" style={{ color: 'var(--text-tertiary)' }}>{file.orig_title}</div>
                          )}
                          <div className="text-xs truncate mt-0.5" style={{ color: 'var(--text-tertiary)' }}>{file.file_path}</div>
                        </div>
                      </div>
                    </td>
                    <td className="px-3 py-3 hidden md:table-cell">
                      <span className={clsx('inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-xs',
                        file.media_type === 'movie' ? 'bg-purple-500/10 text-purple-400' : 'bg-green-500/10 text-green-400'
                      )}>
                        {file.media_type === 'movie' ? <Film size={12} /> : <Tv size={12} />}
                        {file.media_type === 'movie' ? '电影' : '剧集'}
                      </span>
                    </td>
                    <td className="px-3 py-3 hidden lg:table-cell" style={{ color: 'var(--text-secondary)' }}>
                      {file.year || '-'}
                    </td>
                    <td className="px-3 py-3 hidden lg:table-cell">
                      {file.rating > 0 ? (
                        <span className="text-amber-400">★ {file.rating.toFixed(1)}</span>
                      ) : (
                        <span style={{ color: 'var(--text-tertiary)' }}>-</span>
                      )}
                    </td>
                    <td className="px-3 py-3 hidden xl:table-cell" style={{ color: 'var(--text-secondary)' }}>
                      {file.file_size > 0 ? formatFileSize(file.file_size) : '-'}
                    </td>
                    <td className="px-3 py-3 hidden xl:table-cell">
                      {isScraped(file) ? (
                        <span className="inline-flex items-center gap-1 text-xs text-green-400">
                          <Check size={12} /> 已刮削
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-xs text-amber-400">
                          <AlertCircle size={12} /> 未刮削
                        </span>
                      )}
                    </td>
                    <td className="px-3 py-3 text-right">
                      <div className="flex items-center justify-end gap-1">
                        <button onClick={() => onViewDetail(file)}
                          className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-blue-400" title="查看详情">
                          <Eye size={14} />
                        </button>
                        <button onClick={() => onEdit(file)}
                          className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-amber-400" title="编辑">
                          <Edit3 size={14} />
                        </button>
                        <button onClick={() => onScrape(file.id)}
                          className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-purple-400" title="刮削">
                          <Sparkles size={14} />
                        </button>
                        <button onClick={() => onDelete(file.id)}
                          className="p-1.5 rounded hover:bg-white/5 text-surface-400 hover:text-red-400" title="删除">
                          <Trash2 size={14} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      ) : (
        /* 网格视图 */
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 xl:grid-cols-6 gap-4">
          {files.map(file => (
            <div key={file.id} className="glass-panel rounded-xl overflow-hidden group relative cursor-pointer"
              onClick={() => onViewDetail(file)}>
              {/* 选择框 */}
              <button className="absolute top-2 left-2 z-10" onClick={e => { e.stopPropagation(); onToggleSelect(file.id) }}>
                {selectedIds.has(file.id) ? <CheckSquare size={18} className="text-neon" /> : <Square size={18} className="text-white/50 group-hover:text-white/80" />}
              </button>
              {/* 海报 */}
              <div className="aspect-[2/3] bg-surface-800 relative">
                {file.poster_path ? (
                  <img src={file.poster_path} alt="" className="w-full h-full object-cover" />
                ) : (
                  <div className="w-full h-full flex items-center justify-center">
                    <FileVideo size={32} className="text-surface-600" />
                  </div>
                )}
                {/* 状态标签 */}
                <div className="absolute top-2 right-2">
                  {isScraped(file) ? (
                    <span className="bg-green-500/80 text-white text-[10px] px-1.5 py-0.5 rounded">已刮削</span>
                  ) : (
                    <span className="bg-amber-500/80 text-white text-[10px] px-1.5 py-0.5 rounded">未刮削</span>
                  )}
                </div>
                {/* 悬浮操作 */}
                <div className="absolute inset-0 bg-black/60 opacity-0 group-hover:opacity-100 transition-opacity flex items-center justify-center gap-2">
                  <button onClick={e => { e.stopPropagation(); onEdit(file) }}
                    className="p-2 rounded-full bg-white/20 hover:bg-white/30 text-white" title="编辑">
                    <Edit3 size={16} />
                  </button>
                  <button onClick={e => { e.stopPropagation(); onScrape(file.id) }}
                    className="p-2 rounded-full bg-white/20 hover:bg-white/30 text-white" title="刮削">
                    <Sparkles size={16} />
                  </button>
                  <button onClick={e => { e.stopPropagation(); onDelete(file.id) }}
                    className="p-2 rounded-full bg-white/20 hover:bg-white/30 text-white" title="删除">
                    <Trash2 size={16} />
                  </button>
                </div>
              </div>
              {/* 信息 */}
              <div className="p-2.5">
                <div className="font-medium text-sm truncate" style={{ color: 'var(--text-primary)' }}>{file.title}</div>
                <div className="flex items-center gap-2 mt-1 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  <span>{file.year || '-'}</span>
                  {file.rating > 0 && <span className="text-amber-400">★ {file.rating.toFixed(1)}</span>}
                  <span className={file.media_type === 'movie' ? 'text-purple-400' : 'text-green-400'}>
                    {file.media_type === 'movie' ? '电影' : '剧集'}
                  </span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}

      {/* 分页 */}
      {totalPages > 1 && (
        <div className="flex items-center justify-center gap-2">
          <button onClick={() => onPageChange(Math.max(1, page - 1))} disabled={page <= 1}
            className="btn-ghost px-3 py-1.5 rounded text-sm disabled:opacity-30">上一页</button>
          <span className="text-sm" style={{ color: 'var(--text-secondary)' }}>
            {page} / {totalPages} (共 {total} 条)
          </span>
          <button onClick={() => onPageChange(Math.min(totalPages, page + 1))} disabled={page >= totalPages}
            className="btn-ghost px-3 py-1.5 rounded text-sm disabled:opacity-30">下一页</button>
        </div>
      )}
    </>
  )
}
