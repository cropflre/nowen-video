import { useMemo, useState, type FormEvent } from 'react'
import {
  Check,
  CheckSquare,
  FileVideo,
  Loader2,
  ScanLine,
  Search,
  Square,
  Upload,
} from 'lucide-react'
import type { FileImportRequest, Library, ScannedFile } from '@/types'
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
  Tag,
} from '@/components/design-system'
import { formatFileSize } from './constants'

interface ScanDirectoryModalProps {
  libraries: Library[]
  onClose: () => void
  onSuccess: () => void
}

function failedImportPaths(errors: string[], selectedPaths: Set<string>) {
  const failed = new Set<string>()
  for (const error of errors) {
    if (error.includes('已存在')) continue
    for (const path of selectedPaths) {
      if (error.includes(path)) failed.add(path)
    }
  }
  return failed
}

export default function ScanDirectoryModal({ libraries, onClose, onSuccess }: ScanDirectoryModalProps) {
  const toast = useToast()
  const [scanPath, setScanPath] = useState('')
  const [scannedFiles, setScannedFiles] = useState<ScannedFile[]>([])
  const [scanning, setScanning] = useState(false)
  const [scanSelectedPaths, setScanSelectedPaths] = useState<Set<string>>(new Set())
  const [importMediaType, setImportMediaType] = useState('movie')
  const [importLibraryId, setImportLibraryId] = useState('')
  const [importing, setImporting] = useState(false)

  const unimportedPaths = useMemo(
    () => scannedFiles.filter((file) => !file.imported).map((file) => file.path),
    [scannedFiles],
  )
  const allUnimportedSelected = unimportedPaths.length > 0
    && unimportedPaths.every((path) => scanSelectedPaths.has(path))

  const handleScan = async () => {
    const normalizedPath = scanPath.trim()
    if (!normalizedPath) {
      toast.error('请输入目录路径')
      return
    }

    setScanning(true)
    try {
      const res = await fileManagerApi.scanDirectory(normalizedPath)
      setScanPath(normalizedPath)
      setScannedFiles(res.data.data || [])
      setScanSelectedPaths(new Set())
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '扫描失败')
    } finally {
      setScanning(false)
    }
  }

  const handleScanSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (!scanning && !importing) void handleScan()
  }

  const toggleFile = (file: ScannedFile) => {
    if (file.imported) return

    setScanSelectedPaths((previous) => {
      const next = new Set(previous)
      if (next.has(file.path)) next.delete(file.path)
      else next.add(file.path)
      return next
    })
  }

  const toggleAllUnimported = () => {
    setScanSelectedPaths(allUnimportedSelected ? new Set() : new Set(unimportedPaths))
  }

  const handleBatchImport = async () => {
    const selectedBeforeImport = new Set(scanSelectedPaths)
    const filesToImport: FileImportRequest[] = Array.from(selectedBeforeImport).map((path) => {
      const file = scannedFiles.find((item) => item.path === path)
      return {
        file_path: path,
        title: file?.title || '',
        media_type: importMediaType,
        library_id: importLibraryId || undefined,
      }
    })

    if (filesToImport.length === 0) {
      toast.error('请选择要导入的文件')
      return
    }

    setImporting(true)
    try {
      const res = await fileManagerApi.batchImportFiles(filesToImport)
      const result = res.data.data
      const errors = result.errors || []
      const failedPaths = failedImportPaths(errors, selectedBeforeImport)

      if (result.success > 0 || result.skipped > 0) {
        toast.success(`导入完成: 成功 ${result.success}, 跳过 ${result.skipped}, 失败 ${result.failed}`)
        onSuccess()
      }

      if (result.failed > 0) {
        toast.error(`${result.failed} 个文件导入失败，失败项目已保留供重试`)
        try {
          const refreshed = await fileManagerApi.scanDirectory(scanPath.trim())
          setScannedFiles(refreshed.data.data || [])
        } catch {
          // Keep the previous scan result when the refresh itself fails.
        }
        setScanSelectedPaths(failedPaths)
        return
      }

      onClose()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '批量导入失败')
    } finally {
      setImporting(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      size="lg"
      ariaLabel="扫描目录导入"
      panelClassName="max-w-3xl"
    >
      <div className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title="扫描目录导入"
          description="扫描服务器可访问的目录，选择未导入的视频后批量加入媒体库。"
          icon={<ScanLine size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody className="flex min-h-0 flex-1 flex-col gap-4">
          <form onSubmit={handleScanSubmit} className="flex flex-col gap-2 sm:flex-row">
            <Input
              type="text"
              value={scanPath}
              onChange={(event) => setScanPath(event.target.value)}
              placeholder="输入目录路径，如 /media/movies"
              autoFocus
              aria-label="目录路径"
              className="min-w-0 flex-1"
            />
            <Button
              type="submit"
              variant="primary"
              loading={scanning}
              disabled={importing}
              className="shrink-0 sm:min-w-24"
            >
              {scanning
                ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
                : <Search size={15} aria-hidden="true" />}
              {scanning ? '扫描中...' : '扫描'}
            </Button>
          </form>

          <div className="grid gap-3 sm:grid-cols-[minmax(0,10rem)_minmax(0,14rem)_1fr] sm:items-end">
            <label className="block space-y-1.5">
              <span className="text-xs font-medium text-[var(--nv-text-tertiary)]">媒体类型</span>
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
              <span className="text-xs font-medium text-[var(--nv-text-tertiary)]">目标媒体库</span>
              <Select
                value={importLibraryId}
                onChange={(event) => setImportLibraryId(event.target.value)}
                className="w-full"
                disabled={importing}
              >
                <option value="">不指定媒体库</option>
                {libraries.map((library) => (
                  <option key={library.id} value={library.id}>{library.name}</option>
                ))}
              </Select>
            </label>

            {scannedFiles.length > 0 && (
              <div className="flex flex-wrap items-center gap-2 pb-0.5 text-xs text-[var(--nv-text-tertiary)]">
                <span>找到 {scannedFiles.length} 个视频文件</span>
                <span aria-hidden="true">·</span>
                <span>已选 {scanSelectedPaths.size} 个</span>
              </div>
            )}
          </div>

          <div className="min-h-0 flex-1 overflow-hidden rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]">
            {scannedFiles.length > 0 ? (
              <div className="flex h-full min-h-0 flex-col">
                <div className="flex items-center justify-between gap-3 border-b border-[var(--nv-border-subtle)] px-3 py-2.5 sm:px-4">
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    onClick={toggleAllUnimported}
                    disabled={unimportedPaths.length === 0 || importing}
                  >
                    {allUnimportedSelected
                      ? <CheckSquare size={15} aria-hidden="true" />
                      : <Square size={15} aria-hidden="true" />}
                    {allUnimportedSelected ? '取消全选' : '全选未导入'}
                  </Button>
                  <span className="text-xs text-[var(--nv-text-tertiary)]">
                    {unimportedPaths.length} 个可导入
                  </span>
                </div>

                <div className="min-h-0 flex-1 overflow-y-auto p-2 sm:p-3">
                  <div className="space-y-1">
                    {scannedFiles.map((file) => {
                      const selected = scanSelectedPaths.has(file.path)
                      return (
                        <div
                          key={file.path}
                          className="group flex items-center gap-3 rounded-[var(--nv-radius-control)] px-2.5 py-2.5 transition-colors hover:bg-[var(--nv-bg-hover)] sm:px-3"
                          data-disabled={file.imported || undefined}
                        >
                          <button
                            type="button"
                            onClick={() => toggleFile(file)}
                            disabled={file.imported || importing}
                            aria-label={file.imported ? `${file.name} 已导入` : `${selected ? '取消选择' : '选择'} ${file.name}`}
                            aria-pressed={file.imported ? undefined : selected}
                            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] text-[var(--nv-text-tertiary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)] focus-visible:outline-none focus-visible:shadow-[var(--nv-shadow-focus)] disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {file.imported
                              ? <Check size={17} className="text-[var(--nv-status-success)]" aria-hidden="true" />
                              : selected
                                ? <CheckSquare size={17} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
                                : <Square size={17} aria-hidden="true" />}
                          </button>

                          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] text-[var(--nv-text-tertiary)]">
                            <FileVideo size={17} aria-hidden="true" />
                          </div>

                          <div className="min-w-0 flex-1">
                            <div className="truncate text-sm font-medium text-[var(--nv-text-primary)]">{file.name}</div>
                            <div className="mt-0.5 truncate text-xs text-[var(--nv-text-tertiary)]" title={file.path}>{file.path}</div>
                          </div>

                          <div className="flex shrink-0 items-center gap-2">
                            <span className="hidden text-xs text-[var(--nv-text-tertiary)] sm:inline">{formatFileSize(file.size)}</span>
                            {file.imported && <Tag tone="success">已导入</Tag>}
                          </div>
                        </div>
                      )
                    })}
                  </div>
                </div>
              </div>
            ) : (
              <div className="flex min-h-56 flex-col items-center justify-center px-6 py-10 text-center">
                <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] text-[var(--nv-text-tertiary)]">
                  <ScanLine size={20} aria-hidden="true" />
                </div>
                <div className="text-sm font-medium text-[var(--nv-text-secondary)]">
                  {scanning ? '正在扫描目录...' : '输入目录路径后开始扫描'}
                </div>
                <div className="mt-1 max-w-sm text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  扫描结果会标记已导入文件，批量导入只会提交你主动选择的项目。
                </div>
              </div>
            )}
          </div>
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose} disabled={importing}>
            取消
          </Button>
          <Button
            type="button"
            variant="primary"
            loading={importing}
            onClick={() => void handleBatchImport()}
            disabled={scanning || scanSelectedPaths.size === 0}
          >
            {importing
              ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
              : <Upload size={15} aria-hidden="true" />}
            {importing ? '导入中...' : `导入选中 (${scanSelectedPaths.size})`}
          </Button>
        </ModalFooter>
      </div>
    </Modal>
  )
}
