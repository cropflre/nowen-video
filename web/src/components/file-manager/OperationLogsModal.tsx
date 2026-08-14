import { useCallback, useEffect, useState, type ReactNode } from 'react'
import {
  AlertCircle,
  CircleDot,
  Edit3,
  History,
  Loader2,
  RefreshCw,
  Sparkles,
  Trash2,
  Upload,
  Wand2,
} from 'lucide-react'
import type { FileOperationLog } from '@/types'
import { fileManagerApi } from '@/api'
import {
  Button,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Tag,
  type TagTone,
} from '@/components/design-system'

interface OperationLogsModalProps {
  onClose: () => void
}

interface ActionMeta {
  label: string
  tone: TagTone
  icon: ReactNode
}

function getActionMeta(action: string): ActionMeta {
  switch (action) {
    case 'import':
      return { label: '导入', tone: 'success', icon: <Upload size={14} aria-hidden="true" /> }
    case 'edit':
      return { label: '编辑', tone: 'brand', icon: <Edit3 size={14} aria-hidden="true" /> }
    case 'delete':
      return { label: '删除', tone: 'danger', icon: <Trash2 size={14} aria-hidden="true" /> }
    case 'scrape':
      return { label: '刮削', tone: 'brand', icon: <Sparkles size={14} aria-hidden="true" /> }
    case 'rename':
      return { label: '重命名', tone: 'warning', icon: <Wand2 size={14} aria-hidden="true" /> }
    default:
      return { label: action || '操作', tone: 'neutral', icon: <CircleDot size={14} aria-hidden="true" /> }
  }
}

export default function OperationLogsModal({ onClose }: OperationLogsModalProps) {
  const [operationLogs, setOperationLogs] = useState<FileOperationLog[]>([])
  const [loading, setLoading] = useState(true)
  const [loadError, setLoadError] = useState(false)

  const loadLogs = useCallback(async () => {
    setLoading(true)
    setLoadError(false)
    try {
      const res = await fileManagerApi.getOperationLogs(50)
      setOperationLogs(res.data.data || [])
    } catch {
      setOperationLogs([])
      setLoadError(true)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    let active = true
    setLoading(true)
    setLoadError(false)

    fileManagerApi.getOperationLogs(50)
      .then((res) => {
        if (active) setOperationLogs(res.data.data || [])
      })
      .catch(() => {
        if (active) {
          setOperationLogs([])
          setLoadError(true)
        }
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
    }
  }, [])

  return (
    <Modal open onClose={onClose} size="md" ariaLabel="操作日志">
      <div className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title="操作日志"
          description="最近 50 条文件导入、编辑、删除、刮削与重命名记录。"
          icon={<History size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody className="min-h-0">
          {loading ? (
            <div className="flex min-h-52 flex-col items-center justify-center gap-3 text-[var(--nv-text-tertiary)]">
              <Loader2 size={20} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
              <span className="text-sm">正在加载操作日志...</span>
            </div>
          ) : loadError ? (
            <div className="flex min-h-52 flex-col items-center justify-center px-6 py-10 text-center">
              <AlertCircle size={24} className="mb-3 text-[var(--nv-status-danger)]" aria-hidden="true" />
              <div className="text-sm font-medium text-[var(--nv-text-primary)]">操作日志加载失败</div>
              <div className="mt-1 max-w-sm text-xs leading-5 text-[var(--nv-text-tertiary)]">
                当前无法读取日志记录，请检查连接后重试。
              </div>
              <Button type="button" variant="secondary" size="sm" className="mt-4" onClick={() => void loadLogs()}>
                <RefreshCw size={14} aria-hidden="true" />重试
              </Button>
            </div>
          ) : operationLogs.length > 0 ? (
            <div className="divide-y divide-[var(--nv-border-subtle)] border-y border-[var(--nv-border-subtle)]">
              {operationLogs.map((log) => {
                const meta = getActionMeta(log.action)
                return (
                  <article
                    key={log.id}
                    className="flex items-start gap-3 px-1 py-3 transition-colors duration-150 hover:bg-[var(--nv-fill-hover)]"
                  >
                    <div className="mt-0.5 grid h-7 w-7 shrink-0 place-items-center rounded-[8px] bg-[var(--nv-fill-hover)] text-[var(--nv-text-tertiary)]">
                      {meta.icon}
                    </div>

                    <div className="min-w-0 flex-1">
                      <div className="flex items-start gap-2">
                        <p className="min-w-0 flex-1 break-words text-[13px] leading-5 text-[var(--nv-text-secondary)]">
                          {log.detail || '未提供操作详情'}
                        </p>
                        <Tag tone={meta.tone}>{meta.label}</Tag>
                      </div>
                      <time
                        dateTime={log.created_at}
                        className="mt-1 block text-[11px] text-[var(--nv-text-tertiary)]"
                      >
                        {new Date(log.created_at).toLocaleString()}
                      </time>
                    </div>
                  </article>
                )
              })}
            </div>
          ) : (
            <div className="flex min-h-52 flex-col items-center justify-center px-6 py-10 text-center">
              <History size={22} className="mb-3 text-[var(--nv-text-tertiary)]" aria-hidden="true" />
              <div className="text-sm font-medium text-[var(--nv-text-secondary)]">暂无操作记录</div>
              <div className="mt-1 max-w-sm text-xs leading-5 text-[var(--nv-text-tertiary)]">
                文件导入、编辑、删除、刮削和重命名操作会显示在这里。
              </div>
            </div>
          )}
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose}>关闭</Button>
        </ModalFooter>
      </div>
    </Modal>
  )
}
