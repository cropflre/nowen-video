import { useEffect } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import { X, Wand2 } from 'lucide-react'
import type { Library } from '@/types'
import { getLibraryPaths } from '@/types'
import SmartRenamePanel from './SmartRenamePanel'
import { Button } from '@/components/design-system'

export interface SmartRenameDrawerProps {
  open: boolean
  library: Library | null
  onClose: () => void
}

/**
 * 智能扫描重命名抽屉
 * - 右侧 80vw 抽屉，避免与“确认落盘”二级 Modal 嵌套冲突
 * - 标题区由抽屉头部承担，Panel 隐藏自身 header
 * - 接收 library 注入扫描根目录候选
 */
export default function SmartRenameDrawer({ open, library, onClose }: SmartRenameDrawerProps) {
  useEffect(() => {
    if (!open) return
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [open, onClose])

  useEffect(() => {
    if (!open) return
    const previous = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previous
    }
  }, [open])

  const paths = library ? getLibraryPaths(library) : []
  const defaultPath = paths[0] || library?.path || ''

  return (
    <AnimatePresence>
      {open && library && (
        <motion.div
          key="smart-rename-drawer-root"
          className="fixed inset-0 z-[var(--nv-z-modal)] flex"
          initial={{ pointerEvents: 'none' }}
          animate={{ pointerEvents: 'auto' }}
          exit={{ pointerEvents: 'none' }}
        >
          <motion.div
            className="absolute inset-0 backdrop-blur-sm"
            style={{ background: 'var(--nv-bg-overlay)' }}
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.2 }}
            onClick={onClose}
          />

          <motion.aside
            className="relative ml-auto flex h-full w-full flex-col border-l border-[var(--nv-border-default)] bg-[var(--nv-bg-canvas)] shadow-[var(--nv-shadow-elevated)] md:w-[80vw]"
            initial={{ x: '100%' }}
            animate={{ x: 0 }}
            exit={{ x: '100%' }}
            transition={{ type: 'spring', stiffness: 320, damping: 34 }}
            onClick={(event) => event.stopPropagation()}
            aria-label="智能扫描重命名"
          >
            <header className="flex flex-shrink-0 items-center gap-3 border-b border-[var(--nv-border-subtle)] px-6 py-4">
              <div className="flex h-9 w-9 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
                <Wand2 size={18} aria-hidden="true" />
              </div>
              <div className="min-w-0 flex-1">
                <h2 className="truncate text-base font-semibold text-[var(--nv-text-primary)]">智能扫描重命名</h2>
                <p className="truncate text-xs text-[var(--nv-text-tertiary)]" title={defaultPath}>
                  媒体库：<span className="text-[var(--nv-text-secondary)]">{library.name}</span>
                  {defaultPath && (
                    <>
                      <span className="mx-1.5">·</span>
                      <span className="font-mono">{defaultPath}</span>
                    </>
                  )}
                </p>
              </div>
              <Button type="button" variant="ghost" size="sm" iconOnly onClick={onClose} title="关闭 (Esc)" aria-label="关闭智能重命名抽屉">
                <X size={18} aria-hidden="true" />
              </Button>
            </header>

            <div className="flex-1 overflow-y-auto px-6 py-5">
              <SmartRenamePanel
                defaultPath={defaultPath}
                candidatePaths={paths}
                showHeader={false}
                compact
              />
            </div>
          </motion.aside>
        </motion.div>
      )}
    </AnimatePresence>
  )
}
