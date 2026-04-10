import { useState, useEffect, useCallback } from 'react'
import { adminApi } from '@/api'
import {
  Folder,
  FolderOpen,
  ChevronRight,
  ChevronUp,
  X,
  Loader2,
  Check,
  HardDrive,
} from 'lucide-react'

interface FsEntry {
  name: string
  path: string
  is_dir: boolean
}

interface FileBrowserProps {
  open: boolean
  onClose: () => void
  onSelect: (path: string) => void
  initialPath?: string
}

export default function FileBrowser({ open, onClose, onSelect, initialPath }: FileBrowserProps) {
  const [currentPath, setCurrentPath] = useState(initialPath || '/')
  const [parentPath, setParentPath] = useState('')
  const [items, setItems] = useState<FsEntry[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')

  const browse = useCallback(async (path: string) => {
    setLoading(true)
    setError('')
    try {
      const res = await adminApi.browseFS(path)
      const data = res.data.data
      setCurrentPath(data.current)
      setParentPath(data.parent)
      setItems(data.items || [])
    } catch {
      setError('无法访问该目录，请检查路径是否存在及权限')
      setItems([])
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    if (open) {
      browse(initialPath || '/')
    }
  }, [open, initialPath, browse])

  // ESC 关闭
  useEffect(() => {
    const handleKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && open) onClose()
    }
    document.addEventListener('keydown', handleKey)
    return () => document.removeEventListener('keydown', handleKey)
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="modal-overlay flex items-center justify-center animate-fade-in"
      onClick={(e) => { if (e.target === e.currentTarget) onClose() }}
    >
      <div
        className="relative w-full max-w-lg mx-4 rounded-2xl overflow-hidden animate-slide-up"
        style={{
          background: 'var(--bg-elevated)',
          border: '1px solid var(--border-strong)',
          boxShadow: 'var(--shadow-elevated), var(--modal-panel-glow)',
          backdropFilter: 'blur(30px)',
          maxHeight: '80vh',
        }}
      >
        {/* 顶部霓虹光条 */}
        <div
          className="absolute top-0 left-0 right-0 h-[2px] z-10"
          style={{
            background: 'linear-gradient(90deg, transparent, var(--neon-blue), var(--neon-purple), transparent)',
          }}
        />

        {/* 头部 */}
        <div className="flex items-center justify-between px-5 pt-5 pb-3">
          <h2
            className="font-display text-base font-bold tracking-wide"
            style={{ color: 'var(--text-primary)' }}
          >
            选择文件夹
          </h2>
          <button
            onClick={onClose}
            className="rounded-lg p-1.5 transition-all hover:bg-[var(--nav-hover-bg)]"
            style={{ color: 'var(--text-tertiary)' }}
          >
            <X size={18} />
          </button>
        </div>

        {/* 当前路径栏 */}
        <div className="mx-5 mb-3 flex items-center gap-2 rounded-lg px-3 py-2"
          style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}
        >
          <HardDrive size={14} className="flex-shrink-0 text-neon/60" />
          <span
            className="flex-1 truncate text-sm font-mono"
            style={{ color: 'var(--text-primary)' }}
          >
            {currentPath}
          </span>
        </div>

        {/* 目录列表 */}
        <div className="mx-5 overflow-y-auto" style={{ maxHeight: 'calc(80vh - 200px)' }}>
          {/* 上级目录 */}
          {parentPath && (
            <button
              onClick={() => browse(parentPath)}
              className="flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-[var(--nav-hover-bg)]"
            >
              <ChevronUp size={16} className="text-neon/60" />
              <span className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>
                上级目录
              </span>
            </button>
          )}

          {loading && (
            <div className="flex items-center justify-center py-12">
              <Loader2 size={24} className="animate-spin text-neon/40" />
            </div>
          )}

          {error && (
            <div className="rounded-lg px-4 py-3 text-sm" style={{
              background: 'rgba(239, 68, 68, 0.08)',
              border: '1px solid rgba(239, 68, 68, 0.15)',
              color: '#EF4444',
            }}>
              {error}
            </div>
          )}

          {!loading && !error && items.length === 0 && (
            <div className="py-12 text-center">
              <Folder size={32} className="mx-auto mb-2 text-surface-600" />
              <p className="text-sm text-surface-500">此目录下没有子文件夹</p>
            </div>
          )}

          {!loading && items.map((item) => (
            <button
              key={item.path}
              onClick={() => browse(item.path)}
              className="group flex w-full items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-[var(--nav-hover-bg)]"
            >
              <FolderOpen size={16} className="flex-shrink-0 text-amber-400/70 transition-colors group-hover:text-amber-400" />
              <span className="flex-1 truncate text-sm" style={{ color: 'var(--text-primary)' }}>
                {item.name}
              </span>
              <ChevronRight size={14} className="flex-shrink-0 text-surface-600 opacity-0 transition-opacity group-hover:opacity-100" />
            </button>
          ))}
        </div>

        {/* 底部操作栏 */}
        <div
          className="flex items-center justify-between gap-3 px-5 py-4 mt-2"
          style={{
            borderTop: '1px solid var(--border-default)',
          }}
        >
          <p className="truncate text-xs" style={{ color: 'var(--text-muted)' }}>
            已选择: {currentPath}
          </p>
          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="rounded-xl px-4 py-2 text-sm font-medium transition-all"
              style={{
                color: 'var(--text-secondary)',
                border: '1px solid var(--border-default)',
              }}
            >
              取消
            </button>
            <button
              onClick={() => { onSelect(currentPath); onClose() }}
              className="btn-primary gap-1.5 px-4 py-2 text-sm"
            >
              <Check size={14} />
              选择此文件夹
            </button>
          </div>
        </div>
      </div>
    </div>
  )
}
