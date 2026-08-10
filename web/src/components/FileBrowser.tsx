import { useCallback, useEffect, useState } from 'react'
import { adminApi } from '@/api'
import {
  Check,
  ChevronRight,
  ChevronUp,
  Folder,
  FolderOpen,
  HardDrive,
  Loader2,
} from 'lucide-react'
import {
  Button,
  EmptyState,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
} from '@/components/design-system'

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
    if (open) void browse(initialPath || '/')
  }, [open, initialPath, browse])

  const selectCurrentFolder = () => {
    onSelect(currentPath)
    onClose()
  }

  return (
    <Modal open={open} onClose={onClose} size="md" ariaLabel="选择文件夹">
      <ModalHeader
        title="选择文件夹"
        description="浏览服务器目录并选择要使用的文件夹。"
        icon={<FolderOpen size={18} aria-hidden="true" />}
        onClose={onClose}
      />

      <ModalBody className="space-y-4">
        <div className="flex items-center gap-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-2.5">
          <HardDrive size={15} className="shrink-0 text-[var(--nv-action-primary)]" aria-hidden="true" />
          <span className="min-w-0 flex-1 truncate font-mono text-sm text-[var(--nv-text-primary)]" title={currentPath}>
            {currentPath}
          </span>
        </div>

        <div className="min-h-56 space-y-1">
          {parentPath && !loading && (
            <button
              type="button"
              onClick={() => void browse(parentPath)}
              className="flex w-full items-center gap-3 rounded-[var(--nv-radius-control)] px-3 py-2.5 text-left text-sm font-medium text-[var(--nv-text-secondary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]"
            >
              <ChevronUp size={16} className="shrink-0 text-[var(--nv-action-primary)]" aria-hidden="true" />
              上级目录
            </button>
          )}

          {loading && (
            <div className="flex min-h-56 flex-col items-center justify-center gap-3 text-[var(--nv-text-tertiary)]" aria-live="polite">
              <Loader2 size={22} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
              <span className="text-sm">正在读取目录...</span>
            </div>
          )}

          {!loading && error && (
            <div
              role="alert"
              className="rounded-[var(--nv-radius-control)] border px-4 py-3 text-sm leading-6 text-[var(--nv-status-danger)]"
              style={{
                background: 'color-mix(in srgb, var(--nv-status-danger) 8%, var(--nv-bg-surface))',
                borderColor: 'color-mix(in srgb, var(--nv-status-danger) 28%, var(--nv-border-subtle))',
              }}
            >
              {error}
            </div>
          )}

          {!loading && !error && items.length === 0 && (
            <EmptyState
              icon={<Folder size={24} aria-hidden="true" />}
              title="没有子文件夹"
              description="可以选择当前目录，或返回上级目录继续浏览。"
            />
          )}

          {!loading && !error && items.map((item) => (
            <button
              key={item.path}
              type="button"
              onClick={() => void browse(item.path)}
              className="group flex w-full items-center gap-3 rounded-[var(--nv-radius-control)] px-3 py-2.5 text-left transition-colors hover:bg-[var(--nv-bg-hover)]"
              title={item.path}
            >
              <FolderOpen size={16} className="shrink-0 text-[var(--nv-status-warning)]" aria-hidden="true" />
              <span className="min-w-0 flex-1 truncate text-sm text-[var(--nv-text-primary)]">{item.name}</span>
              <ChevronRight
                size={14}
                className="shrink-0 text-[var(--nv-text-tertiary)] opacity-60 transition-opacity group-hover:opacity-100"
                aria-hidden="true"
              />
            </button>
          ))}
        </div>
      </ModalBody>

      <ModalFooter className="justify-between">
        <p className="min-w-0 flex-1 truncate text-xs text-[var(--nv-text-tertiary)]" title={currentPath}>
          已选择：{currentPath}
        </p>
        <div className="flex items-center gap-2">
          <Button type="button" variant="ghost" onClick={onClose}>取消</Button>
          <Button type="button" variant="primary" onClick={selectCurrentFolder}>
            <Check size={14} aria-hidden="true" />
            选择此文件夹
          </Button>
        </div>
      </ModalFooter>
    </Modal>
  )
}
