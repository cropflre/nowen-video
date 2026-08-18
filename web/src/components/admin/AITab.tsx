import { useEffect, useState } from 'react'
import { AlertTriangle, Loader2, RefreshCw, Sparkles } from 'lucide-react'
import { aiApi } from '@/api'
import { Button, Surface } from '@/components/design-system'
import { AdminPanel, AdminStatus } from './AdminPrimitives'
import AITabContent from './AITabContent'
import MediaAnalysisComputePanel from './MediaAnalysisComputePanel'

type LoadState = 'loading' | 'ready' | 'error'

export default function AITab() {
  const [loadState, setLoadState] = useState<LoadState>('loading')
  const [retryKey, setRetryKey] = useState(0)

  useEffect(() => {
    let active = true
    setLoadState('loading')

    aiApi.getStatus({ allowCachedOnError: false })
      .then(() => {
        if (active) setLoadState('ready')
      })
      .catch(() => {
        if (active) setLoadState('error')
      })

    return () => { active = false }
  }, [retryKey])

  return (
    <div className="space-y-6">
      <MediaAnalysisComputePanel />

      {loadState === 'ready' ? (
        <AITabContent />
      ) : (
        <AdminPanel
          title="AI 配置状态"
          description="精彩片段计算不依赖 AI；下面的 AI 配置仍单独检查服务端 AIStatus，避免接口失败时误覆盖已有配置。"
          icon={<Sparkles size={18} />}
        >
          {loadState === 'loading' ? (
            <div className="flex min-h-44 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
              <Loader2 size={22} className="animate-spin text-[var(--nv-action-primary)]" />
              正在读取 AI 配置状态...
            </div>
          ) : (
            <Surface className="flex flex-col gap-4 border-[color-mix(in_srgb,var(--nv-status-danger)_28%,var(--nv-border-subtle))] p-4 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex min-w-0 items-start gap-3">
                <AlertTriangle size={18} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" />
                <div>
                  <AdminStatus tone="danger">AIStatus 读取失败</AdminStatus>
                  <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                    当前无法确认 API Key、Provider、模型和运行开关的真实状态。精彩片段计算节点仍可独立使用；AI 配置表单暂不展示。
                  </p>
                </div>
              </div>
              <Button variant="secondary" size="sm" onClick={() => setRetryKey((value) => value + 1)}>
                <RefreshCw size={14} />
                重新读取
              </Button>
            </Surface>
          )}
        </AdminPanel>
      )}
    </div>
  )
}
