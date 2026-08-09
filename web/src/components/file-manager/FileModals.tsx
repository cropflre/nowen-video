import { useState } from 'react'
import type { Library, FileOperationLog } from '@/types'
import { fileManagerApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Plus, Upload, Loader2, Edit3,
  Sparkles, Wand2, History, Trash2,
} from 'lucide-react'
import clsx from 'clsx'

// ==================== 导入文件对话框 ====================
interface ImportFileModalProps {
  libraries: Library[]
  onClose: () => void
  onSuccess: () => void
}

export function ImportFileModal({ libraries, onClose, onSuccess }: ImportFileModalProps) {
  const toast = useToast()
  const [importPath, setImportPath] = useState('')
  const [importTitle, setImportTitle] = useState('')
  const [importMediaType, setImportMediaType] = useState('movie')
  const [importLibraryId, setImportLibraryId] = useState('')
  const [importing, setImporting] = useState(false)

  const handleImport = async () => {
    if (!importPath) { toast.error('请输入文件路径'); return }
    setImporting(true)
    try {
      await fileManagerApi.importFile({
        file_path: importPath, title: importTitle || undefined,
        media_type: importMediaType, library_id: importLibraryId || undefined,
      })
      toast.success('文件导入成功')
      onClose()
      onSuccess()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '导入失败')
    } finally {
      setImporting(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={onClose}>
      <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-lg mx-4" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>
          <Plus className="inline-block mr-2 mb-0.5" size={20} /> 导入影视文件
        </h3>
        <div className="space-y-3">
          <div>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>文件路径 *</label>
            <input type="text" value={importPath} onChange={e => setImportPath(e.target.value)}
              placeholder="/path/to/movie.mkv" className="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div>
            <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>标题（留空自动提取）</label>
            <input type="text" value={importTitle} onChange={e => setImportTitle(e.target.value)}
              placeholder="自动从文件名提取" className="input-field w-full px-3 py-2 rounded-lg text-sm" />
          </div>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>媒体类型</label>
              <select value={importMediaType} onChange={e => setImportMediaType(e.target.value)}
                className="input-field w-full px-3 py-2 rounded-lg text-sm">
                <option value="movie">电影</option>
                <option value="episode">剧集</option>
              </select>
            </div>
            <div>
              <label className="block text-sm mb-1" style={{ color: 'var(--text-secondary)' }}>媒体库</label>
              <select value={importLibraryId} onChange={e => setImportLibraryId(e.target.value)}
                className="input-field w-full px-3 py-2 rounded-lg text-sm">
                <option value="">不指定</option>
                {libraries.map(lib => <option key={lib.id} value={lib.id}>{lib.name}</option>)}
              </select>
            </div>
          </div>
        </div>
        <div className="flex justify-end gap-2 mt-6">
          <button onClick={onClose} className="btn-ghost px-4 py-2 rounded-lg text-sm">取消</button>
          <button onClick={handleImport} disabled={importing}
            className="btn-primary flex items-center gap-1.5 px-4 py-2 rounded-lg text-sm">
            {importing ? <Loader2 size={14} className="animate-spin" /> : <Upload size={14} />}
            {importing ? '导入中...' : '导入'}
          </button>
        </div>
      </div>
    </div>
  )
}

// ==================== 操作日志对话框 ====================
interface OperationLogsModalProps {
  onClose: () => void
}

export function OperationLogsModal({ onClose }: OperationLogsModalProps) {
  const [opLogs, setOpLogs] = useState<FileOperationLog[]>([])

  useState(() => {
    fileManagerApi.getOperationLogs(50).then(res => setOpLogs(res.data.data || [])).catch(() => {})
  })

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={onClose}>
      <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-2xl mx-4 max-h-[80vh] flex flex-col" onClick={e => e.stopPropagation()}>
        <h3 className="text-lg font-bold mb-4" style={{ color: 'var(--text-primary)' }}>
          <History className="inline-block mr-2 mb-0.5" size={20} /> 操作日志
        </h3>
        <div className="flex-1 overflow-y-auto min-h-0">
          {opLogs.length > 0 ? (
            <div className="space-y-2">
              {opLogs.map(log => (
                <div key={log.id} className="flex items-start gap-3 p-3 rounded-lg" style={{ background: 'var(--bg-secondary)' }}>
                  <div className={clsx('mt-0.5 p-1 rounded', {
                    'bg-green-500/10 text-green-400': log.action === 'import',
                    'bg-blue-500/10 text-blue-400': log.action === 'edit',
                    'bg-red-500/10 text-red-400': log.action === 'delete',
                    'bg-purple-500/10 text-purple-400': log.action === 'scrape',
                    'bg-amber-500/10 text-amber-400': log.action === 'rename',
                  })}>
                    {log.action === 'import' && <Upload size={14} />}
                    {log.action === 'edit' && <Edit3 size={14} />}
                    {log.action === 'delete' && <Trash2 size={14} />}
                    {log.action === 'scrape' && <Sparkles size={14} />}
                    {log.action === 'rename' && <Wand2 size={14} />}
                  </div>
                  <div className="flex-1 min-w-0">
                    <div className="text-sm" style={{ color: 'var(--text-primary)' }}>{log.detail}</div>
                    <div className="text-xs mt-0.5" style={{ color: 'var(--text-tertiary)' }}>{new Date(log.created_at).toLocaleString()}</div>
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <div className="text-center py-8 text-sm" style={{ color: 'var(--text-tertiary)' }}>暂无操作记录</div>
          )}
        </div>
        <div className="flex justify-end mt-4 pt-4 border-t" style={{ borderColor: 'var(--border-default)' }}>
          <button onClick={onClose} className="btn-ghost px-4 py-2 rounded-lg text-sm">关闭</button>
        </div>
      </div>
    </div>
  )
}
