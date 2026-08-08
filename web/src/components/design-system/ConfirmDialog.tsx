import type { ReactNode } from 'react'
import { AlertTriangle } from 'lucide-react'
import { Button } from './index'
import { Modal, ModalBody, ModalFooter, ModalHeader } from './Modal'

interface ConfirmDialogProps {
  title: ReactNode
  description: ReactNode
  hint?: ReactNode
  confirmLabel: ReactNode
  cancelLabel?: ReactNode
  onConfirm: () => void | Promise<void>
  onClose: () => void
  tone?: 'warning' | 'danger'
  loading?: boolean
}

export default function ConfirmDialog({
  title,
  description,
  hint,
  confirmLabel,
  cancelLabel = '取消',
  onConfirm,
  onClose,
  tone = 'warning',
  loading = false,
}: ConfirmDialogProps) {
  const danger = tone === 'danger'
  return (
    <Modal onClose={onClose} size="sm" ariaLabel={typeof title === 'string' ? title : '确认操作'} closeOnBackdrop={!loading}>
      <ModalHeader
        title={title}
        description={danger ? '这是一个不可恢复的操作，请确认后继续。' : '请确认是否继续当前操作。'}
        icon={<AlertTriangle size={18} className={danger ? 'text-[var(--nv-status-danger)]' : 'text-[var(--nv-status-warning)]'} aria-hidden="true" />}
        onClose={onClose}
      />
      <ModalBody className="space-y-3">
        <div className="text-sm leading-6 text-[var(--nv-text-secondary)]">{description}</div>
        {hint && <div className="text-xs leading-5 text-[var(--nv-text-tertiary)]">{hint}</div>}
      </ModalBody>
      <ModalFooter>
        <Button type="button" variant="secondary" onClick={onClose} disabled={loading}>{cancelLabel}</Button>
        <Button type="button" variant={danger ? 'danger' : 'primary'} onClick={onConfirm} loading={loading}>{confirmLabel}</Button>
      </ModalFooter>
    </Modal>
  )
}
