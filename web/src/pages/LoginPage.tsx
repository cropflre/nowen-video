import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import { useTranslation } from '@/i18n'
import { Zap } from 'lucide-react'
import { motion, AnimatePresence } from 'framer-motion'
import { easeSmooth, durations, springDefault } from '@/lib/motion'

export default function LoginPage() {
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)
  const { t } = useTranslation()

  const [isRegister, setIsRegister] = useState(false)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [inviteCode, setInviteCode] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)
  const [initialized, setInitialized] = useState(true) // 系统是否已初始化（有用户）
  const [inviteRequired, setInviteRequired] = useState(false)
  const [registrationOpen, setRegistrationOpen] = useState(true)

  // 检查系统初始化状态
  useEffect(() => {
    authApi.getStatus().then((res) => {
      const status = res.data.data
      setInitialized(status.initialized)
      setInviteRequired(!!(status as { invite_required?: boolean }).invite_required)
      setRegistrationOpen(!!status.registration_open)
      // 系统未初始化时（无用户），自动切换到注册模式
      if (!status.initialized) {
        setIsRegister(true)
      }
    }).catch(() => {
      // 接口不可用时默认显示登录
    })
  }, [])

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const res = isRegister
        ? await authApi.register({ username, password, invite_code: inviteCode || undefined })
        : await authApi.login({ username, password })
      setAuth(res.data.token, res.data.user)
      // 返回 must_change_password 时，跳转到强制改密页
      const mustChange = (res.data as { must_change_password?: boolean }).must_change_password
      if (mustChange) {
        navigate('/force-change-password')
      } else {
        navigate('/')
      }
    } catch (err: unknown) {
      const axiosErr = err as { response?: { data?: { error?: string } } }
      setError(axiosErr.response?.data?.error || t('auth.operationFailed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="relative flex min-h-screen items-center justify-center overflow-hidden px-4"
      style={{ background: 'var(--bg-base)' }}
    >
      {/* 深空背景：多层光�?*/}
      <div className="pointer-events-none absolute inset-0">
        {/* 主光�?*/}
        <div
          className="absolute top-1/4 left-1/4 h-[500px] w-[500px] rounded-full opacity-30 blur-[120px]"
          style={{ background: 'radial-gradient(circle, var(--deco-glow-blue), transparent)' }}
        />
        <div
          className="absolute bottom-1/4 right-1/4 h-[400px] w-[400px] rounded-full opacity-20 blur-[100px]"
          style={{ background: 'radial-gradient(circle, var(--deco-glow-purple), transparent)' }}
        />
        {/* 网格线（科技感） */}
        <div
          className="absolute inset-0 opacity-[0.02]"
          style={{
            backgroundImage: `
              linear-gradient(var(--grid-line-color) 1px, transparent 1px),
              linear-gradient(90deg, var(--grid-line-color) 1px, transparent 1px)
            `,
            backgroundSize: '60px 60px',
          }}
        />
      </div>

      <motion.div
        className="relative z-10 w-full max-w-sm"
        initial={{ opacity: 0, scale: 0.9, filter: 'blur(8px)' }}
        animate={{ opacity: 1, scale: 1, filter: 'blur(0px)' }}
        transition={{ duration: durations.slow, ease: easeSmooth as unknown as [number, number, number, number] }}
      >
        {/* Logo */}
        <motion.div
          className="mb-10 text-center"
          initial={{ opacity: 0, y: -20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.2, ...springDefault }}
        >
          <motion.div
            className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
              boxShadow: 'var(--neon-glow-shadow-xl)',
            }}
            animate={{ y: [0, -10, 0] }}
            transition={{ duration: 6, repeat: Infinity, ease: 'easeInOut' }}
          >
            <Zap size={32} className="text-white" />
          </motion.div>
          <h1 className="font-display text-3xl font-bold tracking-wider" style={{ color: 'var(--text-primary)' }}>
            <span className="text-neon text-neon-glow">N</span>OWEN
          </h1>
          <p className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
            {t('auth.slogan')}
          </p>
        </motion.div>

        {/* 表单 */}
        <motion.form
          onSubmit={handleSubmit}
          className="glass-panel rounded-2xl p-6"
          initial={{ opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ delay: 0.4, duration: durations.slow, ease: easeSmooth as unknown as [number, number, number, number] }}
        >
          {/* 表单顶部霓虹�?*/}
          <div className="absolute top-0 left-4 right-4 h-px bg-gradient-to-r from-transparent via-neon-blue/30 to-transparent" />

          <h2 className="mb-6 text-center font-display text-base font-semibold tracking-wider" style={{ color: 'var(--text-primary)' }}>
            {isRegister ? t('auth.registerTitle') : t('auth.loginTitle')}
          </h2>

          {/* 错误提示 �?动画进出 */}
          <AnimatePresence>
            {error && (
              <motion.div
                initial={{ opacity: 0, height: 0, marginBottom: 0 }}
                animate={{ opacity: 1, height: 'auto', marginBottom: 16 }}
                exit={{ opacity: 0, height: 0, marginBottom: 0 }}
                className="overflow-hidden rounded-xl px-4 py-3 text-sm text-red-400"
                style={{
                  background: 'rgba(239, 68, 68, 0.08)',
                  border: '1px solid rgba(239, 68, 68, 0.15)',
                }}
              >
                {error}
              </motion.div>
            )}
          </AnimatePresence>

          <div className="space-y-4">
            <div>
              <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                {t('auth.username')}
              </label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="input"
                placeholder={t('auth.usernamePlaceholder')}
                required
                minLength={3}
                autoFocus
              />
            </div>

            <div>
              <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                {t('auth.password')}
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="input"
                placeholder={t('auth.passwordPlaceholder')}
                required
                minLength={6}
              />
            </div>

            {/* 邀请码（仅非首用户注册 + 系统要求时显示） */}
            {isRegister && initialized && inviteRequired && (
              <div>
                <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                  邀请码
                </label>
                <input
                  type="text"
                  value={inviteCode}
                  onChange={(e) => setInviteCode(e.target.value)}
                  className="input"
                  placeholder="请输入管理员下发的邀请码"
                  required
                />
              </div>
            )}
          </div>

          <motion.button
            type="submit"
            disabled={loading}
            className="btn-primary mt-6 w-full py-3"
            whileHover={{ scale: 1.02 }}
            whileTap={{ scale: 0.98 }}
          >
            {loading ? (
              <span className="flex items-center justify-center gap-2">
                <span className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
                {t('auth.processing')}
              </span>
            ) : isRegister ? t('auth.register') : t('auth.enterDeepSpace')}
          </motion.button>

          <div className="mt-4 text-center">
            {(!initialized || registrationOpen) && (
              <button
                type="button"
                onClick={() => {
                  setIsRegister(!isRegister)
                  setError('')
                }}
                className="text-sm transition-colors hover:text-neon"
                style={{ color: 'var(--text-secondary)' }}
              >
                {isRegister ? t('auth.switchToLogin') : t('auth.switchToRegister')}
              </button>
            )}
          </div>
        </motion.form>

        {/* 默认账号提示 / 首次注册提示 */}
        <AnimatePresence>
          {!initialized && isRegister ? (
            <motion.p
              key="first-register"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 8 }}
              transition={{ duration: durations.normal }}
              className="mt-4 text-center text-xs"
              style={{ color: 'var(--neon-blue)' }}
            >
              {t('auth.firstUserHint')}
            </motion.p>
          ) : !isRegister && initialized ? (
            <motion.p
              key="default-account"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 8 }}
              transition={{ duration: durations.normal }}
              className="mt-4 text-center text-xs"
              style={{ color: 'var(--text-muted)' }}
            >
              {t('auth.defaultAccount')}
            </motion.p>
          ) : null}
        </AnimatePresence>
      </motion.div>
    </div>
  )
}
