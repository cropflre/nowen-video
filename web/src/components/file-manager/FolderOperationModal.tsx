import type { FormEvent } from 'react'
import { AlertTriangle, Edit3, FolderOpen, Trash2 } from 'lucide-react'
import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Tag,
} from '@/components/design-system'

export type FolderDialogType = 'none' | 'createFolder' | 'renameFolder' | 'deleteFolder'

interface FolderOperationModalProps {
  mode: Exclude<FolderDialogType, 'none'>
  targetPath: string
  value: string
  onValueChange: (value: string) => void
  onClose: () => void
  onCreate: () => void | Promise<void>
  onRename: () => void | Promise<void>
  onDelete: (force: boolean) => void | Promise<void>
}

function getFolderName(path: string) {
  return path.replace(/\\/g, '/').split('/').filter(Boolean).pop() || path || '当前目录'
}

export default function FolderOperationModal({
  mode,
  targetPath,
  value,
  onValueChange,
  onClose,
  onCreate,
  onRename,
  onDelete,
}: FolderOperationModalProps) {
  const targetName = getFolderName(targetPath)
  const isCreate = mode === 'createFolder'
  const isRename = mode === 'renameFolder'
  const isDelete = mode === 'deleteFolder'

  const handleSubmit = (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault()
    if (isCreate) void onCreate()
    if (isRename) void onRename()
  }

  if (isDelete) {
    return (
      <Modal open onClose={onClose} size="sm" ariaLabel="删除文件夹">
        <ModalHeader
          title="删除文件夹"
          description={`确认如何处理「${targetName}」。`}
          icon={<Trash2 size={18} aria-hidden="true" />}
          onClose={onClose}
        />
        <ModalBody className="space-y-4">
          <div
            className="flex items-start gap-3 rounded-[var(--nv-radius-container)] border p-4"
            style={{
              borderColor: 'color-mix(in srgb, var(--nv-status-danger) 24%, transparent)',
              background: 'color-mix(in srgb, var(--nv-status-danger) 8%, transparent)',
            }}
          >
            <AlertTriangle size={18} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" aria-hidden="true" />
            <div className="min-w-0">
              <div className="text-sm font-semibold text-[var(--nv-text-primary)]">强制删除不可恢复</div>
              <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                强制删除会移除文件夹及其中所有文件，并清理数据库中的对应文件记录。若只想清理空目录，请使用“仅删除空文件夹”。
              </p>
            </div>
          </div>

          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3">
            <div className="text-xs font-medium text-[var(--nv-text-tertiary)]">目标目录</div>
            <div className="mt-1 break-all font-mono text-xs leading-5 text-[var(--nv-text-secondary)]">{targetPath}</div>
          </div>
        </ModalBody>
        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose}>取消</Button>
          <Button type="button" variant="secondary" onClick={() => void onDelete(false)}>
            仅删除空文件夹
          </Button>
          <Button type="button" variant="danger" onClick={() => void onDelete(true)}>
            <Trash2 size={15} aria-hidden="true" />
            强制删除
          </Button>
        </ModalFooter>
      </Modal>
    )
  }

  return (
    <Modal open onClose={onClose} size="sm" ariaLabel={isCreate ? '新建文件夹' : '重命名文件夹'}>
      <form onSubmit={handleSubmit} className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title={isCreate ? '新建文件夹' : '重命名文件夹'}
          description={
            isCreate
              ? `在「${targetName}」下创建新的子文件夹。`
              : `修改「${targetName}」的文件夹名称。`
          }
          icon={isCreate ? <FolderOpen size={18} aria-hidden="true" /> : <Edit3 size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody className="space-y-4">
          <label className="block space-y-1.5">
            <span className="text-xs font-medium text-[var(--nv-text-tertiary)]">
              {isCreate ? '文件夹名称' : '新名称'}
            </span>
            <Input
              type="text"
              value={value}
              onChange={(event) => onValueChange(event.target.value)}
              placeholder={isCreate ? '输入文件夹名称' : '输入新名称'}
              autoFocus
              aria-label={isCreate ? '文件夹名称' : '新文件夹名称'}
            />
          </label>

          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3">
            <div className="flex items-center gap-2">
              <Tag tone="neutral">目录</Tag>
              <span className="text-xs text-[var(--nv-text-tertiary)]">{isCreate ? '父级路径' : '当前路径'}</span>
            </div>
            <div className="mt-2 break-all font-mono text-xs leading-5 text-[var(--nv-text-secondary)]">{targetPath}</div>
          </div>
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose}>取消</Button>
          <Button type="submit" variant="primary">
            {isCreate ? <FolderOpen size={15} aria-hidden="true" /> : <Edit3 size={15} aria-hidden="true" />}
            {isCreate ? '创建' : '确认重命名'}
          </Button>
        </ModalFooter>
      </form>
    </Modal>
  )
}
