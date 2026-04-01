import { useState } from 'react'
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
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)

    try {
      const api = isRegister ? authApi.register : authApi.login
      const res = await api({ username, password })
      setAuth(res.data.token, res.data.user)
      navigate('/')
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
      {/* 深空背景：多层光效 */}
      <div className="pointer-events-none absolute inset-0">
        {/* 主光晕 */}
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
        transition={{ duration: durations.slow, ease: easeSmooth as unknown as number[] }}
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
          transition={{ delay: 0.4, duration: durations.slow, ease: easeSmooth as unknown as number[] }}
        >
          {/* 表单顶部霓虹线 */}
          <div className="absolute top-0 left-4 right-4 h-px bg-gradient-to-r from-transparent via-neon-blue/30 to-transparent" />

          <h2 className="mb-6 text-center font-display text-base font-semibold tracking-wider" style={{ color: 'var(--text-primary)' }}>
            {isRegister ? t('auth.registerTitle') : t('auth.loginTitle')}
          </h2>

          {/* 错误提示 — 动画进出 */}
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
          </div>
        </motion.form>

        {/* 默认账号提示 */}
        <AnimatePresence>
          {!isRegister && (
            <motion.p
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0, y: 8 }}
              transition={{ duration: durations.normal }}
              className="mt-4 text-center text-xs"
              style={{ color: 'var(--text-muted)' }}
            >
              {t('auth.defaultAccount')}
            </motion.p>
          )}
        </AnimatePresence>
      </motion.div>
    </div>
  )
}
