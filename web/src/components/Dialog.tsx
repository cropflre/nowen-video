import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
  type RefObject,
} from 'react'
import { createPortal } from 'react-dom'
import { AnimatePresence, motion } from 'framer-motion'
import {
  AlertTriangle,
  CheckCircle2,
  HelpCircle,
  Info,
  X,
  XCircle,
} from 'lucide-react'
import { Button, Input } from '@/components/design-system'
import { modalOverlayVariants, modalContentVariants } from '@/lib/motion'

type Variant = 'default' | 'primary' | 'danger' | 'warning' | 'success' | 'error' | 'info'

interface BaseOptions {
  title?: string
  message?: ReactNode
  variant?: Variant
  /** 默认 true，为 false 则点击遮罩不关闭（仅按按钮） */
  dismissible?: boolean
}

interface ConfirmOptions extends BaseOptions {
  confirmText?: string
  cancelText?: string
}

interface AlertOptions extends BaseOptions {
  okText?: string
}

interface PromptOptions extends BaseOptions {
  defaultValue?: string
  placeholder?: string
  /** 输入类型：text | password | textarea */
  inputType?: 'text' | 'password' | 'textarea'
  confirmText?: string
  cancelText?: string
  /** 校验函数；返回字符串表示错误信息，返回空表示通过 */
  validator?: (value: string) => string | null | undefined
}

interface DialogContextType {
  confirm: (opts: ConfirmOptions) => Promise<boolean>
  alert: (opts: AlertOptions) => Promise<void>
  prompt: (opts: PromptOptions) => Promise<string | null>
}

const DialogContext = createContext<DialogContextType | null>(null)

export function useDialog() {
  const context = useContext(DialogContext)
  if (!context) throw new Error('useDialog must be used within <DialogProvider>')
  return context
}

type DialogKind = 'confirm' | 'alert' | 'prompt'

interface DialogState {
  id: string
  kind: DialogKind
  options: ConfirmOptions | AlertOptions | PromptOptions
  resolve: (value: any) => void
}

const iconClassName = 'shrink-0'
const variantIconMap: Record<Variant, ReactNode> = {
  default: <HelpCircle size={22} className={`${iconClassName} text-[var(--nv-action-primary)]`} />,
  primary: <Info size={22} className={`${iconClassName} text-[var(--nv-action-primary)]`} />,
  info: <Info size={22} className={`${iconClassName} text-[var(--nv-action-primary)]`} />,
  success: <CheckCircle2 size={22} className={`${iconClassName} text-[var(--nv-status-success)]`} />,
  warning: <AlertTriangle size={22} className={`${iconClassName} text-[var(--nv-status-warning)]`} />,
  danger: <AlertTriangle size={22} className={`${iconClassName} text-[var(--nv-status-danger)]`} />,
  error: <XCircle size={22} className={`${iconClassName} text-[var(--nv-status-danger)]`} />,
}

function borderForVariant(variant: Variant) {
  if (variant === 'danger' || variant === 'error') {
    return 'color-mix(in srgb, var(--nv-status-danger) 26%, var(--nv-border-default))'
  }
  if (variant === 'warning') {
    return 'color-mix(in srgb, var(--nv-status-warning) 24%, var(--nv-border-default))'
  }
  if (variant === 'success') {
    return 'color-mix(in srgb, var(--nv-status-success) 22%, var(--nv-border-default))'
  }
  return 'var(--nv-border-default)'
}

function actionVariant(variant: Variant): 'primary' | 'danger' {
  return variant === 'danger' || variant === 'error' ? 'danger' : 'primary'
}

export function DialogProvider({ children }: { children: ReactNode }) {
  const [stack, setStack] = useState<DialogState[]>([])
  const idRef = useRef(0)

  const push = useCallback((dialog: Omit<DialogState, 'id'>) => {
    const id = `dlg-${++idRef.current}`
    setStack((previous) => [...previous, { ...dialog, id }])
    return id
  }, [])

  const pop = useCallback((id: string) => {
    setStack((previous) => previous.filter((dialog) => dialog.id !== id))
  }, [])

  const confirm = useCallback(
    (options: ConfirmOptions) => new Promise<boolean>((resolve) => {
      push({ kind: 'confirm', options, resolve })
    }),
    [push],
  )

  const alert = useCallback(
    (options: AlertOptions) => new Promise<void>((resolve) => {
      push({ kind: 'alert', options, resolve: () => resolve() })
    }),
    [push],
  )

  const prompt = useCallback(
    (options: PromptOptions) => new Promise<string | null>((resolve) => {
      push({ kind: 'prompt', options, resolve })
    }),
    [push],
  )

  const value = useMemo<DialogContextType>(() => ({ confirm, alert, prompt }), [confirm, alert, prompt])

  return (
    <DialogContext.Provider value={value}>
      {children}
      {typeof document !== 'undefined' && createPortal(
        <AnimatePresence>
          {stack.map((dialog) => (
            <DialogShell
              key={dialog.id}
              state={dialog}
              onClose={(result) => {
                dialog.resolve(result)
                pop(dialog.id)
              }}
            />
          ))}
        </AnimatePresence>,
        document.body,
      )}
    </DialogContext.Provider>
  )
}

function DialogShell({ state, onClose }: { state: DialogState; onClose: (result: any) => void }) {
  const { kind, options } = state
  const variant: Variant = options.variant ?? (kind === 'confirm' ? 'default' : 'info')
  const dismissible = options.dismissible !== false

  const dismiss = useCallback(() => {
    if (!dismissible) return
    if (kind === 'confirm') onClose(false)
    else if (kind === 'prompt') onClose(null)
    else onClose(undefined)
  }, [dismissible, kind, onClose])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape' && dismissible) {
        event.preventDefault()
        dismiss()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [dismiss, dismissible])

  useEffect(() => {
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [])

  return (
    <motion.div
      className="fixed inset-0 z-[var(--nv-z-modal)] flex items-center justify-center p-4"
      variants={modalOverlayVariants}
      initial="initial"
      animate="animate"
      exit="exit"
      style={{ background: 'var(--nv-bg-overlay)', backdropFilter: 'blur(6px)' }}
      onClick={dismiss}
    >
      <motion.div
        variants={modalContentVariants}
        className="relative w-full max-w-md rounded-[var(--nv-radius-container)] bg-[var(--nv-bg-elevated)] shadow-[var(--nv-shadow-elevated)]"
        style={{ border: `1px solid ${borderForVariant(variant)}` }}
        onClick={(event) => event.stopPropagation()}
        role="dialog"
        aria-modal="true"
        aria-label={options.title || '对话框'}
      >
        {dismissible && (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            iconOnly
            aria-label="关闭"
            onClick={dismiss}
            className="absolute right-3 top-3"
          >
            <X size={16} aria-hidden="true" />
          </Button>
        )}

        {kind === 'confirm' && (
          <ConfirmBody options={options as ConfirmOptions} variant={variant} onClose={onClose} />
        )}
        {kind === 'alert' && (
          <AlertBody options={options as AlertOptions} variant={variant} onClose={onClose} />
        )}
        {kind === 'prompt' && (
          <PromptBody options={options as PromptOptions} variant={variant} onClose={onClose} />
        )}
      </motion.div>
    </motion.div>
  )
}

function DialogHeader({ options, variant }: { options: BaseOptions; variant: Variant }) {
  return (
    <div className="flex min-w-0 flex-1 items-start gap-3">
      <div className="mt-0.5">{variantIconMap[variant]}</div>
      <div className="min-w-0 flex-1 pr-6">
        {options.title && (
          <h3 className="text-base font-semibold leading-tight text-[var(--nv-text-primary)]">
            {options.title}
          </h3>
        )}
        {options.message && (
          <div className={`${options.title ? 'mt-2' : ''} text-sm leading-6 text-[var(--nv-text-secondary)]`}>
            {options.message}
          </div>
        )}
      </div>
    </div>
  )
}

function ConfirmBody({ options, variant, onClose }: { options: ConfirmOptions; variant: Variant; onClose: (result: boolean) => void }) {
  const confirmRef = useRef<HTMLButtonElement>(null)
  useEffect(() => { confirmRef.current?.focus() }, [])

  return (
    <div className="px-6 pb-6 pt-7">
      <DialogHeader options={options} variant={variant} />
      <div className="mt-6 flex items-center justify-end gap-2">
        <Button type="button" variant="secondary" onClick={() => onClose(false)}>
          {options.cancelText ?? '取消'}
        </Button>
        <Button ref={confirmRef} type="button" variant={actionVariant(variant)} onClick={() => onClose(true)}>
          {options.confirmText ?? '确定'}
        </Button>
      </div>
    </div>
  )
}

function AlertBody({ options, variant, onClose }: { options: AlertOptions; variant: Variant; onClose: (result: undefined) => void }) {
  const okRef = useRef<HTMLButtonElement>(null)
  useEffect(() => { okRef.current?.focus() }, [])

  return (
    <div className="px-6 pb-6 pt-7">
      <DialogHeader options={options} variant={variant} />
      <div className="mt-6 flex items-center justify-end">
        <Button ref={okRef} type="button" variant={actionVariant(variant)} onClick={() => onClose(undefined)}>
          {options.okText ?? '知道了'}
        </Button>
      </div>
    </div>
  )
}

function PromptBody({ options, variant, onClose }: { options: PromptOptions; variant: Variant; onClose: (result: string | null) => void }) {
  const [value, setValue] = useState(options.defaultValue ?? '')
  const [error, setError] = useState<string | null>(null)
  const inputRef = useRef<HTMLInputElement | HTMLTextAreaElement>(null)

  useEffect(() => {
    inputRef.current?.focus()
    inputRef.current?.select?.()
  }, [])

  const submit = useCallback(() => {
    if (options.validator) {
      const validationError = options.validator(value)
      if (validationError) {
        setError(validationError)
        return
      }
    }
    onClose(value)
  }, [value, options, onClose])

  const inputType = options.inputType ?? 'text'

  return (
    <div className="px-6 pb-6 pt-7">
      <DialogHeader options={options} variant={variant} />

      <div className="mt-4 pl-[34px]">
        {inputType === 'textarea' ? (
          <textarea
            ref={inputRef as RefObject<HTMLTextAreaElement>}
            value={value}
            onChange={(event) => {
              setValue(event.target.value)
              if (error) setError(null)
            }}
            placeholder={options.placeholder}
            rows={4}
            className="w-full resize-y rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] px-3 py-2 text-sm leading-6 text-[var(--nv-text-primary)] outline-none transition-[border-color,box-shadow] placeholder:text-[var(--nv-text-tertiary)] hover:border-[var(--nv-border-hover)] focus:border-[var(--nv-action-primary)] focus:shadow-[var(--nv-shadow-focus)]"
            aria-invalid={!!error}
            onKeyDown={(event) => {
              if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) submit()
            }}
          />
        ) : (
          <Input
            ref={inputRef as RefObject<HTMLInputElement>}
            type={inputType}
            value={value}
            onChange={(event) => {
              setValue(event.target.value)
              if (error) setError(null)
            }}
            placeholder={options.placeholder}
            invalid={!!error}
            onKeyDown={(event) => {
              if (event.key === 'Enter') submit()
            }}
          />
        )}
        {error && <p className="mt-2 text-xs text-[var(--nv-status-danger)]">{error}</p>}
      </div>

      <div className="mt-6 flex items-center justify-end gap-2">
        <Button type="button" variant="secondary" onClick={() => onClose(null)}>
          {options.cancelText ?? '取消'}
        </Button>
        <Button type="button" variant={actionVariant(variant)} onClick={submit}>
          {options.confirmText ?? '确定'}
        </Button>
      </div>
    </div>
  )
}

export default DialogProvider
