import { useEffect, useState, type ReactNode } from 'react'
import { AlertTriangle, Cloud, Database, Loader2, RefreshCw } from 'lucide-react'
import { storageApi } from '@/api'
import { Button, Surface } from '@/components/design-system'
import { AdminPanel, AdminStatus } from './AdminPrimitives'
import {
  AlistSection as AlistSectionContent,
  S3Section as S3SectionContent,
} from './RemoteStorageSectionsContent'

type Provider = 'alist' | 's3'
type LoadState = 'loading' | 'ready' | 'error'

function RemoteStorageGuard({
  provider,
  title,
  description,
  icon,
  children,
}: {
  provider: Provider
  title: string
  description: string
  icon: ReactNode
  children: ReactNode
}) {
  const [loadState, setLoadState] = useState<LoadState>('loading')
  const [retryKey, setRetryKey] = useState(0)
  const [errorMessage, setErrorMessage] = useState('')

  useEffect(() => {
    let active = true
    setLoadState('loading')
    setErrorMessage('')

    const configRequest = provider === 'alist'
      ? storageApi.getAlistConfig({ allowCachedOnError: false })
      : storageApi.getS3Config({ allowCachedOnError: false })

    Promise.all([
      configRequest,
      storageApi.getStorageStatus({ allowCachedOnError: false }),
    ])
      .then(() => {
        if (active) setLoadState('ready')
      })
      .catch((error: unknown) => {
        if (!active) return
        const message = (error as { response?: { data?: { error?: string } } })?.response?.data?.error
        setErrorMessage(message || `无法读取 ${title} 配置或运行状态`)
        setLoadState('error')
      })

    return () => { active = false }
  }, [provider, retryKey, title])

  if (loadState === 'ready') return <>{children}</>

  return (
    <AdminPanel title={title} description={description} icon={icon}>
      {loadState === 'loading' ? (
        <div className="flex min-h-40 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
          <Loader2 size={21} className="animate-spin text-[var(--nv-action-primary)]" />
          正在读取真实存储配置与运行状态...
        </div>
      ) : (
        <Surface className="flex flex-col gap-4 border-[color-mix(in_srgb,var(--nv-status-danger)_28%,var(--nv-border-subtle))] p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <AlertTriangle size={18} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" />
            <div>
              <AdminStatus tone="danger">配置读取失败</AdminStatus>
              <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                {errorMessage}。为避免把请求失败误显示成“未启用”并覆盖原配置，当前暂不展示编辑表单。
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

export function AlistSection() {
  return (
    <RemoteStorageGuard
      provider="alist"
      title="Alist 聚合网盘"
      description="先确认服务端 Alist 配置与聚合运行状态可读取，再开放编辑。"
      icon={<Cloud size={18} />}
    >
      <AlistSectionContent />
    </RemoteStorageGuard>
  )
}

export function S3Section() {
  return (
    <RemoteStorageGuard
      provider="s3"
      title="S3 兼容对象存储"
      description="先确认服务端 S3 配置与聚合运行状态可读取，再开放编辑。"
      icon={<Database size={18} />}
    >
      <S3SectionContent />
    </RemoteStorageGuard>
  )
}
