import { useState, useEffect } from 'react'
import type { Library } from '@/types'
import { batchMoveApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  FolderInput,
  Loader2,
  Check,
  X,
  ArrowRight,
} from 'lucide-react'
import clsx from 'clsx'

interface BatchMoveDialogProps {
  mediaIds: string[]
  currentLibraryId?: string
  onClose: () => void
  onSuccess?: () => void
}

export default function BatchMoveDialog({ mediaIds, currentLibraryId, onClose, onSuccess }: BatchMoveDialogProps) {
  const toast = useToast()
  const [libraries, setLibraries] = useState<Library[]>([])
  const [targetId, setTargetId] = useState('')
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    libraryApi.list().then(res => {
      const libs = (res.data.data || []).filter((l: Library) => l.id !== currentLibraryId)
      setLibraries(libs)
      if (libs.length > 0) setTargetId(libs[0].id)
    }).catch(() => {})
  }, [currentLibraryId])

  const handleMove = async () => {
    if (!targetId) { toast.error('请选择目标媒体库'); return }
    setLoading(true)
    try {
      const res = await batchMoveApi.batchMove({ media_ids: mediaIds, target_library_id: targetId })
      const result = res.data.data
      toast.success(`移动完成：成功 ${result.success}，失败 ${result.failed}`)
      onSuccess?.()
      onClose()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '移动失败')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm" onClick={onClose}>
      <div
        className="glass-panel w-full max-w-md rounded-2xl p-6 animate-slide-up"
        onClick={e => e.stopPropagation()}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className="flex items-center gap-2 text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <FolderInput size={20} className="text-neon" />
            批量移动媒体
          </h3>
          <button onClick={onClose} className="text-surface-400 hover:text-surface-200">
            <X size={18} />
          </button>
        </div>

        <p className="mb-4 text-sm" style={{ color: 'var(--text-secondary)' }}>
          将选中的 <span className="font-semibold text-neon">{mediaIds.length}</span> 个媒体移动到目标媒体库
        </p>

        <div className="mb-6">
          <label className="mb-2 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
            目标媒体库
          </label>
          {libraries.length > 0 ? (
            <div className="space-y-2">
              {libraries.map(lib => (
                <button
                  key={lib.id}
                  onClick={() => setTargetId(lib.id)}
                  className={clsx(
                    'flex w-full items-center gap-3 rounded-xl p-3 text-left transition-all',
                    targetId === lib.id
                      ? 'bg-neon/10 border border-neon/30'
                      : 'hover:bg-[var(--nav-hover-bg)]'
                  )}
                  style={targetId !== lib.id ? { border: '1px solid var(--border-default)' } : undefined}
                >
                  <FolderInput size={16} className={targetId === lib.id ? 'text-neon' : 'text-surface-500'} />
                  <div className="flex-1">
                    <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{lib.name}</span>
                    <span className="ml-2 text-xs text-surface-500">{lib.type}</span>
                  </div>
                  {targetId === lib.id && <Check size={16} className="text-neon" />}
                </button>
              ))}
            </div>
          ) : (
            <p className="text-sm text-surface-500">没有可用的目标媒体库</p>
          )}
        </div>

        <div className="flex items-center justify-end gap-2">
          <button onClick={onClose} className="btn-ghost px-4 py-2 text-sm">取消</button>
          <button
            onClick={handleMove}
            disabled={loading || !targetId}
            className="btn-primary gap-1.5 px-4 py-2 text-sm disabled:opacity-50"
          >
            {loading ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              <ArrowRight size={14} />
            )}
            确认移动
          </button>
        </div>
      </div>
    </div>
  )
}
