import { useEffect, useState, type Dispatch, type SetStateAction } from 'react'
import { AlertTriangle, Loader2, RefreshCw, Users } from 'lucide-react'
import { adminApi } from '@/api'
import type { User } from '@/types'
import { Button, Surface } from '@/components/design-system'
import { AdminPanel, AdminStatus } from './AdminPrimitives'
import UsersTabContent from './UsersTabContent'

interface UsersTabProps {
  users: User[]
  setUsers: Dispatch<SetStateAction<User[]>>
}

export default function UsersTab({ users, setUsers }: UsersTabProps) {
  const [loading, setLoading] = useState(true)
  const [confirmed, setConfirmed] = useState(false)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [retryKey, setRetryKey] = useState(0)

  useEffect(() => {
    let active = true
    setLoading(true)
    setLoadError(null)

    adminApi.listUsers()
      .then((response) => {
        if (!active) return
        setUsers(response.data.data || [])
        setConfirmed(true)
      })
      .catch((error: unknown) => {
        if (!active) return
        const message = (error as { response?: { data?: { error?: string } } })?.response?.data?.error
        setLoadError(message || '用户列表读取失败，请稍后重试')
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => { active = false }
  }, [retryKey, setUsers])

  if (!confirmed && loading && users.length === 0) {
    return (
      <AdminPanel title="用户管理" description="正在读取真实用户列表与账号状态。" icon={<Users size={18} />}>
        <div className="flex min-h-44 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
          <Loader2 size={22} className="animate-spin text-[var(--nv-action-primary)]" />
          正在读取用户列表...
        </div>
      </AdminPanel>
    )
  }

  if (!confirmed && loadError && users.length === 0) {
    return (
      <AdminPanel
        title="用户列表加载失败"
        description="当前无法确认真实账号数量与状态，因此不会把请求失败显示成“还没有用户”。"
        icon={<Users size={18} />}
      >
        <Surface className="flex flex-col gap-4 border-[color-mix(in_srgb,var(--nv-status-danger)_28%,var(--nv-border-subtle))] p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex min-w-0 items-start gap-3">
            <AlertTriangle size={18} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" />
            <div>
              <AdminStatus tone="danger">读取失败</AdminStatus>
              <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{loadError}</p>
            </div>
          </div>
          <Button variant="secondary" size="sm" onClick={() => setRetryKey((value) => value + 1)}>
            <RefreshCw size={14} />
            重新读取
          </Button>
        </Surface>
      </AdminPanel>
    )
  }

  return (
    <div className="space-y-4">
      {loadError && (
        <Surface className="flex flex-col gap-3 border-[color-mix(in_srgb,var(--nv-status-warning)_28%,var(--nv-border-subtle))] p-3 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-start gap-2 text-xs leading-5 text-[var(--nv-text-secondary)]">
            <AlertTriangle size={15} className="mt-0.5 shrink-0 text-[var(--nv-status-warning)]" />
            <span>刷新用户列表失败，当前继续显示上一次已加载的数据：{loadError}</span>
          </div>
          <Button variant="ghost" size="sm" onClick={() => setRetryKey((value) => value + 1)} disabled={loading}>
            <RefreshCw size={13} className={loading ? 'animate-spin' : undefined} />
            重试
          </Button>
        </Surface>
      )}
      <UsersTabContent users={users} setUsers={setUsers} />
    </div>
  )
}
