import { useState, type FormEvent } from 'react'
import { Loader2, Plus, Upload } from 'lucide-react'
import type { Library } from '@/types'
import { fileManagerApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Select,
} from '@/components/design-system'

interface ImportFileModalProps {
  libraries: Library[]
  onClose: () => void
  onSuccess: () => void
}

export default function ImportFileModal({ libraries, onClose, onSuccess }: ImportFileModalProps) {
  const toast = useToast()
  const [importPath, setImportPath] = useState('')
  const [importTitle, setImportTitle] = useState('')
  const [importMediaType, setImportMediaType] = useState('movie')
  const [importLibraryId, setImportLibraryId] = useState('')
  const [importing, setImporting] = useState(false)

  const handleImport = async () => {
    const normalizedPath = importPath.trim()
    if (!normalizedPath) {
      toast.error('请输入文件路径')
      return
    }

    setImporting(true)
    try {
      await fileManagerApi.importFile({
        file_path: normalizedPath,
        title: importTitle.trim() || undefined,
        media_type: importMediaType,
        library_id: importLibraryId || undefined,
      })
      toast.success('文件导入成功')
      onClose()
      onSuccess()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!importing) void handleImport()
  }

  return (
    <Modal
      open
      onClose={onClose}
      size="md"
      ariaLabel="导入影视文件"
      panelClassName="max-w-lg"
    >
      <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title="导入影视文件"
          description="添加服务器可访问的视频文件路径，可选指定标题、类型和媒体库。"
          icon={<Plus size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody>
          <div className="space-y-4">
            <label className="block space-y-1.5">
              <span className="text-sm font-medium text-[var(--nv-text-secondary)]">文件路径 *</span>
              <Input
                type="text"
                value={importPath}
                onChange={(event) => setImportPath(event.target.value)}
                placeholder="/path/to/movie.mkv"
                autoFocus
                aria-required="true"
                disabled={importing}
              />
            </label>

            <label className="block space-y-1.5">
              <span className="text-sm font-medium text-[var(--nv-text-secondary)]">标题（留空自动提取）</span>
              <Input
                type="text"
                value={importTitle}
                onChange={(event) => setImportTitle(event.target.value)}
                placeholder="自动从文件名提取"
                disabled={importing}
              />
            </label>

            <div className="grid gap-4 sm:grid-cols-2">
              <label className="block space-y-1.5">
                <span className="text-sm font-medium text-[var(--nv-text-secondary)]">媒体类型</span>
                <Select
                  value={importMediaType}
                  onChange={(event) => setImportMediaType(event.target.value)}
                  className="w-full"
                  disabled={importing}
                >
                  <option value="movie">电影</option>
                  <option value="episode">剧集</option>
                </Select>
              </label>

              <label className="block space-y-1.5">
                <span className="text-sm font-medium text-[var(--nv-text-secondary)]">媒体库</span>
                <Select
                  value={importLibraryId}
                  onChange={(event) => setImportLibraryId(event.target.value)}
                  className="w-full"
                  disabled={importing}
                >
                  <option value="">不指定</option>
                  {libraries.map((library) => (
                    <option key={library.id} value={library.id}>{library.name}</option>
                  ))}
                </Select>
              </label>
            </div>

            <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-2.5 text-xs leading-5 text-[var(--nv-text-tertiary)]">
              文件不会上传到浏览器；这里填写的是 Nowen Video 服务端能够读取的文件路径。
            </div>
          </div>
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose} disabled={importing}>
            取消
          </Button>
          <Button type="submit" variant="primary" loading={importing}>
            {importing
              ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
              : <Upload size={15} aria-hidden="true" />}
            {importing ? '导入中...' : '导入'}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
