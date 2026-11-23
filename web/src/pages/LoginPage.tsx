import { useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Clapperboard, Eye, EyeOff, Film, Loader2, Lock, Popcorn, QrCode, Sparkles, Ticket, UserPlus } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import { useTranslation } from '@/i18n'
import DevicePairingRequest from '@/components/auth/DevicePairingRequest'
import { Button, Input } from '@/components/design-system'

/** 根据当前时间返回俏皮的问候语 */
function getPlayfulGreeting(): { badge: string; headline: string; sub: string } {
  const hour = new Date().getHours()
  if (hour >= 5 && hour < 11) {
    return { badge: '早安场 · Matinee', headline: '早安，片单已备好', sub: '来杯咖啡，挑部好片开启今天吧' }
  }
  if (hour >= 11 && hour < 14) {
    return { badge: '午休时刻 · Break', headline: '午间小憩，看点轻松的', sub: '爆米花和电影都在等你入座' }
  }
  if (hour >= 14 && hour < 18) {
    return { badge: '下午茶 · Cinema', headline: '欢迎回到私人影院', sub: '拉上窗帘，随时开场放映' }
  }
  if (hour >= 18 && hour < 23) {
    return { badge: '黄金夜场 · Prime', headline: '今晚，想看点什么？', sub: '灯光调暗，好戏马上开场' }
  }
  return { badge: '午夜场 · Late Night', headline: '夜深了，独家放映中', sub: '只属于你的安静影院时刻' }
}

/** 灵动俏皮的影院徽章：用温和的渐变光晕与场记板/爆米花图标随输入状态轻快互动 */
function PlayfulCinemaMascot({
  focusField,
  showPassword,
  hasError,
  loading,
}: {
  focusField: 'none' | 'username' | 'password' | 'invite'
  showPassword: boolean
  hasError: boolean
  loading: boolean
}) {
  const isHiding = focusField === 'password' && !showPassword
  const isPeeking = focusField === 'password' && showPassword
  const isLooking = focusField === 'username' || focusField === 'invite'

  return (
    <div
      className={`relative mx-auto flex h-16 w-16 items-center justify-center rounded-2xl text-white shadow-lg transition-all duration-300 ${
        hasError
          ? 'animate-[nv-wiggle_0.4s_ease-in-out] bg-gradient-to-br from-rose-500 to-orange-500'
          : loading
            ? '-translate-y-1 scale-105 bg-gradient-to-br from-violet-500 via-purple-500 to-fuchsia-500'
            : isHiding
              ? 'scale-95 bg-gradient-to-br from-slate-600 to-slate-700'
              : isLooking
                ? '-rotate-6 scale-105 bg-gradient-to-br from-violet-500 to-indigo-500'
                : 'bg-gradient-to-br from-violet-500 via-purple-500 to-pink-500 hover:rotate-6 hover:scale-105'
      }`}
      aria-hidden="true"
    >
      {/* 顶部小角标 */}
      <span className="absolute -right-1.5 -top-1.5 flex h-5 w-5 items-center justify-center rounded-full border-2 border-[var(--nv-bg-surface)] bg-amber-400 text-[10px] font-bold text-slate-900 shadow-sm">
        {hasError ? '!' : isHiding ? <Lock size={10} /> : <Sparkles size={10} />}
      </span>

      {/* 主体图标随状态切换 */}
      {isHiding ? (
        <Lock size={26} className="transition-transform duration-200" />
      ) : isPeeking ? (
        <Eye size={26} className="transition-transform duration-200" />
      ) : isLooking ? (
        <Popcorn size={28} className="transition-transform duration-200" />
      ) : (
        <Clapperboard size={28} className="transition-transform duration-200" />
      )}
    </div>
  )
}

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
  const [showPassword, setShowPassword] = useState(false)
  const [inviteCode, setInviteCode] = useState(invitation)
  const [focusField, setFocusField] = useState<'none' | 'username' | 'password' | 'invite'>('none')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [initialized, setInitialized] = useState(true)
  const [inviteRequired, setInviteRequired] = useState(false)
  const [registrationOpen, setRegistrationOpen] = useState(true)

  const greeting = useMemo(() => getPlayfulGreeting(), [])

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
      navigate(
        mustChange ? '/force-change-password' : pairCode ? '/pair#' + new URLSearchParams({ code: pairCode }).toString() : '/',
        { replace: true, state: mustChange && pairCode ? { pairCode } : undefined },
      )
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      setError(axiosErr.response?.data?.error || t('auth.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  const canSwitchMode = !initialized || registrationOpen || Boolean(invitation)
  const footerText = !initialized && isRegister
    ? t('auth.firstUserHint')
    : !isRegister && initialized
      ? t('auth.defaultAccount')
      : null

  return (
    <div className="relative flex min-h-[calc(100dvh-4rem)] w-full items-center justify-center overflow-hidden px-4 py-6">
      <style>{`
        @keyframes nv-float-slow {
          0%, 100% { transform: translate(0, 0) scale(1); }
          50% { transform: translate(18px, -14px) scale(1.06); }
        }
        @keyframes nv-wiggle {
          0%, 100% { transform: translateX(0); }
          25% { transform: translateX(-6px) rotate(-4deg); }
          75% { transform: translateX(6px) rotate(4deg); }
        }
      `}</style>

      {/* 柔和漂浮的影院氛围光斑 */}
      <div className="pointer-events-none absolute inset-0 overflow-hidden" aria-hidden="true">
        <div
          className="absolute -left-24 -top-24 h-80 w-80 rounded-full opacity-25 blur-3xl"
          style={{ background: 'var(--nv-action-primary)', animation: 'nv-float-slow 11s ease-in-out infinite' }}
        />
        <div
          className="absolute -bottom-28 -right-20 h-96 w-96 rounded-full opacity-20 blur-3xl"
          style={{ background: 'radial-gradient(circle, #fb7185 0%, #a855f7 100%)', animation: 'nv-float-slow 14s ease-in-out infinite reverse' }}
        />
      </div>

      {/* 居中磨砂玻璃主卡片 */}
      <main className="relative z-10 w-full max-w-[400px] rounded-3xl border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)]/90 p-7 shadow-[0_20px_50px_-15px_rgba(0,0,0,0.12)] backdrop-blur-xl sm:p-8">
        {/* 头部：灵动小精灵 + 俏皮问候 */}
        <header className="mb-6 text-center">
          <PlayfulCinemaMascot
            focusField={focusField}
            showPassword={showPassword}
            hasError={Boolean(error)}
            loading={loading}
          />

          <div className="mt-4 inline-flex items-center gap-1.5 rounded-full border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3 py-1 text-[11px] font-medium text-[var(--nv-text-secondary)]">
            <Film size={12} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            <span>{pairMode ? '扫码 / 配对码快传' : isRegister ? '新成员入座' : greeting.badge}</span>
          </div>

          <h1 className="mt-3 text-xl font-bold tracking-tight text-[var(--nv-text-primary)]">
            {pairMode ? '用已登录设备授权' : isRegister ? '创建你的专属影院账号' : greeting.headline}
          </h1>
          <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">
            {pairMode ? '拿出已经登录的手机确认一下，秒速同步片单。' : greeting.sub}
          </p>
        </header>

        {pairMode ? (
          <DevicePairingRequest onBack={() => setPairMode(false)} />
        ) : (
          <form onSubmit={handleSubmit} className="space-y-4">
            {invitation && isRegister && (
              <div className="flex items-center gap-2 rounded-xl border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-3.5 py-2.5 text-xs text-[var(--nv-text-secondary)]">
                <Popcorn size={15} className="shrink-0 text-[var(--nv-action-primary)]" aria-hidden="true" />
                <span>家人邀请你加入！设置账号密码后即可一起看片。</span>
              </div>
            )}

            {error && (
              <div
                className="rounded-xl border border-[var(--nv-status-danger)]/50 bg-[var(--nv-status-danger)]/10 px-3.5 py-2.5 text-xs font-medium text-[var(--nv-status-danger)]"
                role="alert"
              >
                {error}
              </div>
            )}

            <div className="space-y-1.5">
              <label htmlFor="auth-username" className="block text-xs font-medium text-[var(--nv-text-secondary)]">
                {t('auth.username')}
              </label>
              <Input
                id="auth-username"
                type="text"
                value={username}
                onChange={(event) => {
                  setUsername(event.target.value)
                  if (error) setError('')
                }}
                onFocus={() => setFocusField('username')}
                onBlur={() => setFocusField('none')}
                placeholder={t('auth.usernamePlaceholder')}
                required
                minLength={3}
                autoComplete="username"
                autoFocus
                className="h-11 rounded-xl"
              />
            </div>

            <div className="space-y-1.5">
              <label htmlFor="auth-password" className="block text-xs font-medium text-[var(--nv-text-secondary)]">
                {t('auth.password')}
              </label>
              <div className="relative">
                <Input
                  id="auth-password"
                  type={showPassword ? 'text' : 'password'}
                  value={password}
                  onChange={(event) => {
                    setPassword(event.target.value)
                    if (error) setError('')
                  }}
                  onFocus={() => setFocusField('password')}
                  onBlur={() => setFocusField('none')}
                  placeholder={t('auth.passwordPlaceholder')}
                  required
                  minLength={6}
                  autoComplete={isRegister ? 'new-password' : 'current-password'}
                  className="h-11 rounded-xl pr-10"
                />
                <button
                  type="button"
                  onClick={() => setShowPassword((v) => !v)}
                  onMouseDown={(e) => e.preventDefault()}
                  className="absolute right-2.5 top-1/2 -translate-y-1/2 rounded-lg p-1 text-[var(--nv-text-tertiary)] transition-colors hover:bg-[var(--nv-bg-hover)] hover:text-[var(--nv-text-primary)]"
                  aria-label={showPassword ? '隐藏密码' : '显示密码'}
                  title={showPassword ? '隐藏密码' : '显示密码'}
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>

            {isRegister && initialized && (inviteRequired || Boolean(invitation)) && (
              <div className="space-y-1.5">
                <label htmlFor="auth-invite-code" className="block text-xs font-medium text-[var(--nv-text-secondary)]">
                  邀请码
                </label>
                <Input
                  id="auth-invite-code"
                  type="text"
                  value={inviteCode}
                  onChange={(event) => setInviteCode(event.target.value)}
                  onFocus={() => setFocusField('invite')}
                  onBlur={() => setFocusField('none')}
                  placeholder="请输入管理员下发的邀请码"
                  required
                  autoComplete="off"
                  className="h-11 rounded-xl"
                />
              </div>
            )}

            <div className="space-y-2.5 pt-1">
              <Button
                type="submit"
                variant="primary"
                size="lg"
                className="h-11 w-full rounded-xl font-semibold shadow-sm transition-transform active:scale-[0.98]"
                disabled={loading}
              >
                {loading ? (
                  <Loader2 size={17} className="animate-spin" aria-hidden="true" />
                ) : isRegister ? (
                  <UserPlus size={17} aria-hidden="true" />
                ) : (
                  <Ticket size={17} aria-hidden="true" />
                )}
                {loading ? t('auth.processing') : isRegister ? '领票入场 · 注册' : '开场放映'}
              </Button>

              {!isRegister && initialized && (
                <Button
                  type="button"
                  variant="secondary"
                  className="h-10 w-full rounded-xl"
                  disabled={loading}
                  onClick={() => setPairMode(true)}
                >
                  <QrCode size={15} aria-hidden="true" />
                  用已登录设备授权
                </Button>
              )}
            </div>

            {canSwitchMode && (
              <div className="pt-1 text-center">
                <button
                  type="button"
                  onClick={() => {
                    setIsRegister((current) => !current)
                    setError('')
                  }}
                  className="text-xs font-medium text-[var(--nv-text-tertiary)] transition-colors hover:text-[var(--nv-action-primary)]"
                >
                  {isRegister ? '已有账号？直接去登录' : '没有账号？注册一个新身份'}
                </button>
              </div>
            )}
          </form>
        )}

        {!pairMode && footerText && (
          <footer className="mt-5 border-t border-[var(--nv-border-subtle)] pt-3.5 text-center text-[11px] text-[var(--nv-text-tertiary)]">
            {footerText}
          </footer>
        )}
      </main>
    </div>
  )
}
