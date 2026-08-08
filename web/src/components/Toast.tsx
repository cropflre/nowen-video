import { createContext, useContext, useState, useCallback, useRef, useMemo, forwardRef } from 'react'
import { X, CheckCircle2, AlertTriangle, Info, XCircle } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { Button } from '@/components/design-system'
import { toastVariants } from '@/lib/motion'

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
    if (duration > 0) setTimeout(() => removeToast(id), duration)
  }, [removeToast])

  const success = useCallback((message: string) => toast('success', message), [toast])
  const error = useCallback((message: string) => toast('error', message), [toast])
  const warning = useCallback((message: string) => toast('warning', message), [toast])
  const info = useCallback((message: string) => toast('info', message), [toast])

  const value: ToastContextType = useMemo(() => ({ toast, success, error, warning, info }), [toast, success, error, warning, info])

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        className="pointer-events-none fixed right-4 top-4 z-[var(--nv-z-toast)] flex max-w-[calc(100vw-2rem)] flex-col items-end gap-2"
        aria-live="polite"
        aria-relevant="additions"
      >
        <AnimatePresence mode="popLayout">
          {toasts.map((item) => (
            <ToastItem key={item.id} toast={item} onClose={() => removeToast(item.id)} />
          ))}
        </AnimatePresence>
      </div>
    </ToastContext.Provider>
  )
}

const iconMap: Record<ToastType, React.ReactNode> = {
  success: <CheckCircle2 size={18} className="text-[var(--nv-status-success)]" aria-hidden="true" />,
  error: <XCircle size={18} className="text-[var(--nv-status-danger)]" aria-hidden="true" />,
  warning: <AlertTriangle size={18} className="text-[var(--nv-status-warning)]" aria-hidden="true" />,
  info: <Info size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />,
}

function borderColor(type: ToastType) {
  if (type === 'success') return 'color-mix(in srgb, var(--nv-status-success) 25%, var(--nv-border-default))'
  if (type === 'warning') return 'color-mix(in srgb, var(--nv-status-warning) 25%, var(--nv-border-default))'
  if (type === 'error') return 'color-mix(in srgb, var(--nv-status-danger) 25%, var(--nv-border-default))'
  return 'var(--nv-border-default)'
}

const ToastItem = forwardRef<HTMLDivElement, { toast: Toast; onClose: () => void }>(
  function ToastItem({ toast, onClose }, ref) {
    return (
      <motion.div
        ref={ref}
        layout
        variants={toastVariants}
        initial="initial"
        animate="animate"
        exit="exit"
        className="pointer-events-auto flex w-full min-w-0 items-center gap-3 rounded-[var(--nv-radius-container)] bg-[var(--nv-bg-elevated)] px-4 py-3 shadow-[var(--nv-shadow-elevated)] sm:min-w-[280px] sm:max-w-[420px]"
        style={{ border: `1px solid ${borderColor(toast.type)}` }}
        role={toast.type === 'error' ? 'alert' : 'status'}
      >
        {iconMap[toast.type]}
        <p className="min-w-0 flex-1 break-words text-sm text-[var(--nv-text-primary)]">{toast.message}</p>
        <Button variant="ghost" size="sm" iconOnly onClick={onClose} className="shrink-0" aria-label="关闭通知">
          <X size={14} aria-hidden="true" />
        </Button>
      </motion.div>
    )
  },
)
