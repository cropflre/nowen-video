import { useEffect, useState, type ReactNode } from 'react'
import {
  CircleDot,
  Edit3,
  History,
  Loader2,
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
      return {
        label: '导入',
        tone: 'success',
        icon: <Upload size={15} aria-hidden="true" />,
      }
    case 'edit':
      return {
        label: '编辑',
        tone: 'brand',
        icon: <Edit3 size={15} aria-hidden="true" />,
      }
    case 'delete':
      return {
        label: '删除',
        tone: 'danger',
        icon: <Trash2 size={15} aria-hidden="true" />,
      }
    case 'scrape':
      return {
        label: '刮削',
        tone: 'brand',
        icon: <Sparkles size={15} aria-hidden="true" />,
      }
    case 'rename':
      return {
        label: '重命名',
        tone: 'warning',
        icon: <Wand2 size={15} aria-hidden="true" />,
      }
    default:
      return {
        label: action || '操作',
        tone: 'neutral',
        icon: <CircleDot size={15} aria-hidden="true" />,
      }
  }
}

export default function OperationLogsModal({ onClose }: OperationLogsModalProps) {
  const [operationLogs, setOperationLogs] = useState<FileOperationLog[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let active = true

    fileManagerApi.getOperationLogs(50)
      .then((res) => {
        if (active) setOperationLogs(res.data.data || [])
      })
      .catch(() => {})
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
          description="查看最近 50 条文件导入、编辑、删除、刮削与重命名记录。"
          icon={<History size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody className="min-h-0">
          {loading ? (
            <div className="flex min-h-52 flex-col items-center justify-center gap-3 text-[var(--nv-text-tertiary)]">
              <Loader2 size={22} className="animate-spin motion-reduce:animate-none" aria-hidden="true" />
              <span className="text-sm">正在加载操作日志...</span>
            </div>
          ) : operationLogs.length > 0 ? (
            <div className="space-y-2">
              {operationLogs.map((log) => {
                const meta = getActionMeta(log.action)
                return (
                  <article
                    key={log.id}
                    className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3 transition-colors hover:bg-[var(--nv-bg-hover)] sm:p-4"
                  >
                    <div className="flex items-start gap-3">
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-control)] text-[var(--nv-text-secondary)]">
                        {meta.icon}
                      </div>

                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-start justify-between gap-2">
                          <p className="min-w-0 flex-1 break-words text-sm leading-6 text-[var(--nv-text-primary)]">
                            {log.detail || '未提供操作详情'}
                          </p>
                          <Tag tone={meta.tone}>{meta.label}</Tag>
                        </div>
                        <time
                          dateTime={log.created_at}
                          className="mt-1.5 block text-xs text-[var(--nv-text-tertiary)]"
                        >
                          {new Date(log.created_at).toLocaleString()}
                        </time>
                      </div>
                    </div>
                  </article>
                )
              })}
            </div>
          ) : (
            <div className="flex min-h-52 flex-col items-center justify-center px-6 py-10 text-center">
              <div className="mb-3 flex h-11 w-11 items-center justify-center rounded-[var(--nv-radius-container)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-tertiary)]">
                <History size={20} aria-hidden="true" />
              </div>
              <div className="text-sm font-medium text-[var(--nv-text-secondary)]">暂无操作记录</div>
              <div className="mt-1 max-w-sm text-xs leading-5 text-[var(--nv-text-tertiary)]">
                文件导入、编辑、删除、刮削和重命名操作会显示在这里。
              </div>
            </div>
          )}
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose}>
            关闭
          </Button>
        </ModalFooter>
      </div>
    </Modal>
  )
}
