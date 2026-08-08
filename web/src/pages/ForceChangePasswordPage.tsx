import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { KeyRound, Loader2, LogOut } from 'lucide-react'
import { authApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import AuthShell from '@/components/auth/AuthShell'
import { Button, Input } from '@/components/design-system'

export default function ForceChangePasswordPage() {
  const navigate = useNavigate()
  const setAuth = useAuthStore((state) => state.setAuth)
  const logout = useAuthStore((state) => state.logout)
  const [oldPwd, setOldPwd] = useState('')
  const [newPwd, setNewPwd] = useState('')
  const [newPwd2, setNewPwd2] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    setError('')
    if (newPwd.length < 6) { setError('新密码至少 6 位'); return }
    if (newPwd !== newPwd2) { setError('两次输入的新密码不一致'); return }
    if (newPwd === oldPwd) { setError('新密码不能与当前密码相同'); return }

    setLoading(true)
    try {
      const res = await authApi.changePassword(oldPwd, newPwd)
      const tokenData = res.data.data
      if (!tokenData?.token || !tokenData.user) {
        setError(res.data.message || '密码已修改，但登录状态刷新失败，请退出后使用新密码重新登录')
        return
      }

      setAuth(tokenData.token, { ...tokenData.user, must_change_pwd: false })
      navigate('/', { replace: true })
    } catch (err: any) {
      setError(err?.response?.data?.error || '修改密码失败')
    } finally {
      setLoading(false)
    }
  }

  const handleLogout = () => {
    logout()
    navigate('/login', { replace: true })
  }

  return (
    <AuthShell
      title="首次登录 — 请修改密码"
      description="为了账号安全，您必须先修改初始密码才能进入系统"
      eyebrow="账号安全"
      icon={<KeyRound size={26} aria-hidden="true" />}
    >
      <form onSubmit={handleSubmit} className="space-y-5">
        {error && (
          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-status-danger)] bg-[var(--nv-bg-surface-soft)] px-4 py-3 text-sm text-[var(--nv-status-danger)]" role="alert">
            {error}
          </div>
        )}

        <PasswordField
          id="current-password"
          label="当前密码"
          value={oldPwd}
          onChange={setOldPwd}
          autoComplete="current-password"
          autoFocus
        />
        <PasswordField
          id="new-password"
          label="新密码"
          value={newPwd}
          onChange={setNewPwd}
          autoComplete="new-password"
          minLength={6}
        />
        <PasswordField
          id="confirm-password"
          label="确认新密码"
          value={newPwd2}
          onChange={setNewPwd2}
          autoComplete="new-password"
          minLength={6}
        />

        <Button type="submit" variant="primary" size="lg" className="w-full" disabled={loading}>
          {loading && <Loader2 size={17} className="animate-spin" aria-hidden="true" />}
          {loading ? '处理中...' : '修改密码并继续'}
        </Button>

        <div className="border-t border-[var(--nv-border-subtle)] pt-4 text-center">
          <Button type="button" variant="ghost" size="sm" onClick={handleLogout}>
            <LogOut size={15} aria-hidden="true" />
            退出登录
          </Button>
        </div>
      </form>
    </AuthShell>
  )
}

function PasswordField({
  id,
  label,
  value,
  onChange,
  autoComplete,
  autoFocus = false,
  minLength,
}: {
  id: string
  label: string
  value: string
  onChange: (value: string) => void
  autoComplete: string
  autoFocus?: boolean
  minLength?: number
}) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={id} className="block text-xs font-semibold uppercase tracking-[0.08em] text-[var(--nv-text-secondary)]">{label}</label>
      <Input
        id={id}
        name={id}
        type="password"
        autoComplete={autoComplete}
        value={value}
        onChange={(event) => onChange(event.target.value)}
        required
        minLength={minLength}
        maxLength={64}
        autoFocus={autoFocus}
      />
    </div>
  )
}
