import { useEffect, useState } from 'react'
import { AlertTriangle, Loader2, RefreshCw, Sparkles } from 'lucide-react'
import { aiApi } from '@/api'
import { Button, Surface } from '@/components/design-system'
import { AdminPanel, AdminStatus } from './AdminPrimitives'
import AITabContent from './AITabContent'

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

  if (loadState === 'ready') return <AITabContent />

  return (
    <AdminPanel
      title="AI 配置状态"
      description="先确认服务端 AIStatus 可读取，再进入配置页，避免把接口失败误显示成未配置或未运行。"
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
                当前无法确认 API Key、Provider、模型和运行开关的真实状态。为避免误覆盖已保存配置，暂不展示配置表单。
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
  )
}
