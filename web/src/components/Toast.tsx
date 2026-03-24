import { createContext, useContext, useState, useCallback, useRef, useEffect } from 'react'
import { X, CheckCircle2, AlertTriangle, Info, XCircle } from 'lucide-react'
import clsx from 'clsx'

// ============================================================
// 全局 Toast 通知系统 - 深空流体风格
// ============================================================

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
  const ctx = useContext(ToastContext)
  if (!ctx) throw new Error('useToast must be used within <ToastProvider>')
  return ctx
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toasts, setToasts] = useState<Toast[]>([])
  const idRef = useRef(0)

  const removeToast = useCallback((id: string) => {
    setToasts(prev => prev.filter(t => t.id !== id))
  }, [])

  const toast = useCallback((type: ToastType, message: string, duration = 3500) => {
    const id = `toast-${++idRef.current}`
    setToasts(prev => [...prev, { id, type, message, duration }])
    if (duration > 0) {
      setTimeout(() => removeToast(id), duration)
    }
  }, [removeToast])

  const value: ToastContextType = {
    toast,
    success: (msg) => toast('success', msg),
    error: (msg) => toast('error', msg),
    warning: (msg) => toast('warning', msg),
    info: (msg) => toast('info', msg),
  }

  return (
    <ToastContext.Provider value={value}>
      {children}
      {/* Toast 容器 */}
      <div className="pointer-events-none fixed right-4 top-4 z-[999] flex flex-col items-end gap-2">
        {toasts.map((t) => (
          <ToastItem key={t.id} toast={t} onClose={() => removeToast(t.id)} />
        ))}
      </div>
    </ToastContext.Provider>
  )
}

function ToastItem({ toast, onClose }: { toast: Toast; onClose: () => void }) {
  const [visible, setVisible] = useState(false)

  useEffect(() => {
    requestAnimationFrame(() => setVisible(true))
    // 提前200ms播放退出动画
    if (toast.duration && toast.duration > 0) {
      const timer = setTimeout(() => setVisible(false), toast.duration - 300)
      return () => clearTimeout(timer)
    }
  }, [toast.duration])

  const iconMap: Record<ToastType, React.ReactNode> = {
    success: <CheckCircle2 size={18} style={{ color: 'var(--neon-green)' }} />,
    error: <XCircle size={18} className="text-red-400" />,
    warning: <AlertTriangle size={18} className="text-yellow-400" />,
    info: <Info size={18} style={{ color: 'var(--neon-blue)' }} />,
  }

  const borderColorMap: Record<ToastType, string> = {
    success: 'rgba(0, 255, 136, 0.15)',
    error: 'rgba(239, 68, 68, 0.15)',
    warning: 'rgba(234, 179, 8, 0.15)',
    info: 'rgba(0, 240, 255, 0.15)',
  }

  return (
    <div
      className={clsx(
        'pointer-events-auto flex items-center gap-3 rounded-xl px-4 py-3 shadow-2xl transition-all duration-300',
        visible ? 'translate-x-0 opacity-100' : 'translate-x-8 opacity-0'
      )}
      style={{
        background: 'var(--bg-elevated)',
        border: `1px solid ${borderColorMap[toast.type]}`,
        backdropFilter: 'blur(20px)',
        minWidth: '280px',
        maxWidth: '420px',
      }}
    >
      {iconMap[toast.type]}
      <p className="flex-1 text-sm" style={{ color: 'var(--text-primary)' }}>{toast.message}</p>
      <button
        onClick={onClose}
        className="shrink-0 rounded-lg p-1 text-surface-500 transition-colors hover:text-white"
      >
        <X size={14} />
      </button>
    </div>
  )
}
