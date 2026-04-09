import { createContext, useContext, useState, useCallback, useRef, useMemo } from 'react'
import { X, CheckCircle2, AlertTriangle, Info, XCircle } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { toastVariants } from '@/lib/motion'

// ============================================================
// 全局 Toast 通知系统 - 深空流体风格 + framer-motion 动画
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

  const success = useCallback((msg: string) => toast('success', msg), [toast])
  const error = useCallback((msg: string) => toast('error', msg), [toast])
  const warning = useCallback((msg: string) => toast('warning', msg), [toast])
  const info = useCallback((msg: string) => toast('info', msg), [toast])

  const value: ToastContextType = useMemo(() => ({
    toast,
    success,
    error,
    warning,
    info,
  }), [toast, success, error, warning, info])

  return (
    <ToastContext.Provider value={value}>
      {children}
      {/* Toast 容器 — AnimatePresence 实现退出动画 + layout 实现布局过渡 */}
      <div className="pointer-events-none fixed right-4 top-4 z-[999] flex flex-col items-end gap-2">
        <AnimatePresence mode="popLayout">
          {toasts.map((t) => (
            <ToastItem key={t.id} toast={t} onClose={() => removeToast(t.id)} />
          ))}
        </AnimatePresence>
      </div>
    </ToastContext.Provider>
  )
}

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
  info: 'var(--neon-blue-15)',
}

function ToastItem({ toast, onClose }: { toast: Toast; onClose: () => void }) {
  return (
    <motion.div
      layout
      variants={toastVariants}
      initial="initial"
      animate="animate"
      exit="exit"
      className="pointer-events-auto flex items-center gap-3 rounded-xl px-4 py-3 shadow-2xl"
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
    </motion.div>
  )
}
