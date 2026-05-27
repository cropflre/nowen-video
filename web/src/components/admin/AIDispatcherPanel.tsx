/**
 * AIDispatcherPanel - 切换审计日志面板
 *
 * 仅展示 AI Provider 切换的历史记录。
 * "当前调度状态"和"用量趋势"已整合进 AITab 的使用统计区块。
 */
import { useEffect, useState } from 'react'
import { aiApi } from '@/api/ai'
import { useToast } from '@/components/Toast'
import type { AIFailoverLog } from '@/types'

interface AIDispatcherPanelProps {
  onConfigChanged?: () => void
}

export default function AIDispatcherPanel(_props: AIDispatcherPanelProps) {
  const toast = useToast()
  const [logs, setLogs] = useState<AIFailoverLog[]>([])
  const [loading, setLoading] = useState(true)

  const loadLogs = async () => {
    try {
      const res = await aiApi.listFailoverLogs(50)
      setLogs(res.data.data ?? [])
    } catch (err: any) {
      toast.error('加载切换日志失败：' + (err?.response?.data?.error || err?.message || ''))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    loadLogs()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  if (loading) {
    return (
      <div className="glass-panel rounded-xl p-6 text-sm text-theme-muted">
        正在加载切换日志...
      </div>
    )
  }

  return (
    <div className="glass-panel rounded-xl p-5">
      <div className="mb-3 font-display text-lg font-semibold tracking-wide text-theme-primary flex items-center gap-2">
        <span className="text-neon">🛟</span> 切换日志
      </div>
      {logs.length === 0 ? (
        <div className="py-4 text-center text-sm text-theme-muted">暂无切换记录</div>
      ) : (
        <div className="max-h-72 overflow-auto">
          <table className="w-full text-xs">
            <thead className="sticky top-0 text-theme-muted" style={{ background: 'var(--bg-surface)' }}>
              <tr>
                <th className="px-3 py-2 text-left font-medium">时间</th>
                <th className="px-3 py-2 text-left font-medium">从</th>
                <th className="px-3 py-2 text-left font-medium">到</th>
                <th className="px-3 py-2 text-left font-medium">原因</th>
                <th className="px-3 py-2 text-left font-medium">操作者</th>
              </tr>
            </thead>
            <tbody>
              {logs.map((l) => (
                <tr key={l.id} style={{ borderTop: '1px solid var(--border-subtle)' }}>
                  <td className="px-3 py-2 text-theme-secondary">{new Date(l.occurred_at).toLocaleString()}</td>
                  <td className="px-3 py-2 text-theme-secondary">{l.from_provider}</td>
                  <td className="px-3 py-2 font-medium text-theme-primary">{l.to_provider}</td>
                  <td className="px-3 py-2 text-theme-secondary">{l.reason}</td>
                  <td className="px-3 py-2 text-theme-muted">{l.operator}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}
