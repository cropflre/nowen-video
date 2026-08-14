import { createContext, useContext, useState, useCallback, useRef, useMemo } from 'react'
import { X, CheckCircle2, AlertTriangle, Info, XCircle } from 'lucide-react'
import { Button } from '@/components/design-system'

type ToastType = 'success' | 'error' | 'warning' | 'info'

interface Toast {
  id: string
  type: ToastType
  message: string
  duration?: number
}

interface ToastContextType {
  toast: (type: ToastType, message: string, duration?: number) => void
  success: (message: string) => void
  error: (message: string) => void
  warning: (message: string) => void
  info: (message: string) => void
}

const ToastContext = createContext<ToastContextType | null>(null)

export function useToast() {
  const context = useContext(ToastContext)
  if (!context) throw new Error('useToast must be used within <ToastProvider>')
  return context
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const idRef = useRef(0)

  const removeToast = useCallback((id: string) => {
    setToasts((previous) => previous.filter((toast) => toast.id !== id))
  }, [])

  const toast = useCallback((type: ToastType, message: string, duration = 3500) => {
    const id = `toast-${++idRef.current}`
    setToasts((previous) => [...previous, { id, type, message, duration }])
    if (duration > 0) window.setTimeout(() => removeToast(id), duration)
  }, [removeToast])

  const success = useCallback((message: string) => toast('success', message), [toast])
  const error = useCallback((message: string) => toast('error', message), [toast])
  const warning = useCallback((message: string) => toast('warning', message), [toast])
  const info = useCallback((message: string) => toast('info', message), [toast])
  const value = useMemo(() => ({ toast, success, error, warning, info }), [toast, success, error, warning, info])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        className="nv-toast-stack pointer-events-none fixed right-3 top-3 z-[var(--nv-z-toast)] flex max-w-[calc(100vw-1.5rem)] flex-col items-end gap-1.5 sm:right-4 sm:top-4"
        style={{
          top: 'max(12px, env(safe-area-inset-top, 0px))',
          right: 'max(12px, env(safe-area-inset-right, 0px))',
          maxWidth: 'calc(100vw - max(12px, env(safe-area-inset-left, 0px)) - max(12px, env(safe-area-inset-right, 0px)))',
        }}
        aria-live="polite"
        aria-relevant="additions"
      >
        {toasts.map((item) => (
          <ToastItem key={item.id} toast={item} onClose={() => removeToast(item.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  )
}

const iconMap: Record<ToastType, React.ReactNode> = {
  success: <CheckCircle2 size={16} className="text-[var(--nv-status-success)]" aria-hidden="true" />,
  error: <XCircle size={16} className="text-[var(--nv-status-danger)]" aria-hidden="true" />,
  warning: <AlertTriangle size={16} className="text-[var(--nv-status-warning)]" aria-hidden="true" />,
  info: <Info size={16} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />,
}

function ToastItem({ toast, onClose }: { toast: Toast; onClose: () => void }) {
  return (
    <div
      className="nv-toast pointer-events-auto flex w-full min-w-0 items-center gap-2.5 rounded-[var(--nv-radius-popover)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] px-3 py-2.5 shadow-[var(--nv-shadow-elevated)] sm:min-w-[260px] sm:max-w-[390px]"
      data-tone={toast.type}
      role={toast.type === 'error' ? 'alert' : 'status'}
    >
      {iconMap[toast.type]}
      <p className="min-w-0 flex-1 break-words text-xs leading-5 text-[var(--nv-text-secondary)]">{toast.message}</p>
      <Button variant="ghost" size="sm" iconOnly onClick={onClose} className="shrink-0" aria-label="关闭通知">
        <X size={13} aria-hidden="true" />
      </Button>
    </div>
  )
}
