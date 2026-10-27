import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Loader2, LogIn, UserPlus } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import { useTranslation } from '@/i18n'
import AuthShell from '@/components/auth/AuthShell'
import DevicePairingRequest from '@/components/auth/DevicePairingRequest'
import { Button, Input } from '@/components/design-system'

export default function LoginPage() {
  const navigate = useNavigate()
  const setAuth = useAuthStore((state) => state.setAuth)
  const { t } = useTranslation()

  const [invitation] = useState(() => new URLSearchParams(window.location.hash.slice(1)).get('invite') || '')
  const [pairCode] = useState(() => new URLSearchParams(window.location.hash.slice(1)).get('pair') || '')
  const [pairMode, setPairMode] = useState(false)
  const [isRegister, setIsRegister] = useState(Boolean(invitation))
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [inviteCode, setInviteCode] = useState(invitation)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [initialized, setInitialized] = useState(true)
  const [inviteRequired, setInviteRequired] = useState(false)
  const [registrationOpen, setRegistrationOpen] = useState(true)

  useEffect(() => {
    authApi.getStatus().then((res) => {
      const status = res.data.data
      setInitialized(status.initialized)
      setInviteRequired(!!(status as { invite_required?: boolean }).invite_required)
      setRegistrationOpen(!!status.registration_open)
      if (!status.initialized) setIsRegister(true)
    }).catch(() => {
      // 接口不可用时保持登录模式，沿用原有容错行为。
    })
  }, [])

  const handleSubmit = async (event: React.FormEvent) => {
    event.preventDefault()
    if (loading) return
    setError('')
    setLoading(true)

    try {
      const res = isRegister
        ? await authApi.register({ username, password, invite_code: inviteCode || undefined })
        : await authApi.login({ username, password })

      const { token, user } = res.data
      if (!token || !user) {
        setError('登录响应缺少认证信息，请刷新页面后重试')
        return
      }

      setAuth(token, user)
      const mustChange = Boolean(
        (res.data as { must_change_password?: boolean }).must_change_password || user.must_change_pwd,
      )
      navigate(mustChange ? '/force-change-password' : pairCode ? '/pair#' + new URLSearchParams({ code: pairCode }).toString() : '/', { replace: true, state: mustChange && pairCode ? { pairCode } : undefined })
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      setError(axiosErr.response?.data?.error || t('auth.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  const canSwitchMode = !initialized || registrationOpen || Boolean(invitation)
  const footer = !initialized && isRegister
    ? t('auth.firstUserHint')
    : !isRegister && initialized
      ? t('auth.defaultAccount')
      : null

  return (
    <AuthShell
      title={pairMode ? '登录新设备' : 'NOWEN'}
      eyebrow={pairMode ? '手机确认' : isRegister ? t('auth.registerTitle') : t('auth.loginTitle')}
      description={pairMode ? '用已登录的手机确认，即可开始观看。' : t('auth.slogan')}
      footer={pairMode ? null : footer}
    >
      {pairMode ? <DevicePairingRequest onBack={() => setPairMode(false)} /> : <form onSubmit={handleSubmit} className="space-y-5">
        {invitation && isRegister && <p className="text-sm">家人邀请你加入。设置账号和密码后，就能开始观看。</p>}
        {error && (
          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-status-danger)] bg-[var(--nv-bg-surface-soft)] px-4 py-3 text-sm text-[var(--nv-status-danger)]" role="alert">
            {error}
          </div>
        )}

        <div className="space-y-1.5">
          <label htmlFor="auth-username" className="block text-xs font-semibold uppercase tracking-[0.08em] text-[var(--nv-text-secondary)]">
            {t('auth.username')}
          </label>
          <Input
            id="auth-username"
            type="text"
            value={username}
            onChange={(event) => setUsername(event.target.value)}
            placeholder={t('auth.usernamePlaceholder')}
            required
            minLength={3}
            autoComplete="username"
            autoFocus
          />
        </div>

        <div className="space-y-1.5">
          <label htmlFor="auth-password" className="block text-xs font-semibold uppercase tracking-[0.08em] text-[var(--nv-text-secondary)]">
            {t('auth.password')}
          </label>
          <Input
            id="auth-password"
            type="password"
            value={password}
            onChange={(event) => setPassword(event.target.value)}
            placeholder={t('auth.passwordPlaceholder')}
            required
            minLength={6}
            autoComplete={isRegister ? 'new-password' : 'current-password'}
          />
        </div>

        {isRegister && initialized && (inviteRequired || Boolean(invitation)) && (
          <div className="space-y-1.5">
            <label htmlFor="auth-invite-code" className="block text-xs font-semibold uppercase tracking-[0.08em] text-[var(--nv-text-secondary)]">
              邀请码
            </label>
            <Input
              id="auth-invite-code"
              type="text"
              value={inviteCode}
              onChange={(event) => setInviteCode(event.target.value)}
              placeholder="请输入管理员下发的邀请码"
              required
              autoComplete="off"
            />
          </div>
        )}

        <Button type="submit" variant="primary" size="lg" className="w-full" disabled={loading}>
          {loading ? <Loader2 size={17} className="animate-spin" aria-hidden="true" /> : isRegister ? <UserPlus size={17} aria-hidden="true" /> : <LogIn size={17} aria-hidden="true" />}
          {loading ? t('auth.processing') : isRegister ? t('auth.register') : t('auth.enterDeepSpace')}
        </Button>
        {!isRegister && initialized && <Button type="button" className="w-full" disabled={loading} onClick={() => setPairMode(true)}>用已登录设备授权</Button>}

        {canSwitchMode && (
          <div className="border-t border-[var(--nv-border-subtle)] pt-4 text-center">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={() => {
                setIsRegister((current) => !current)
                setError('')
              }}
            >
              {isRegister ? t('auth.switchToLogin') : t('auth.switchToRegister')}
            </Button>
          </div>
        )}
      </form>}
    </AuthShell>
  )
}
