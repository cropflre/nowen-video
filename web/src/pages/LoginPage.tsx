import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import { Zap } from 'lucide-react'

export default function LoginPage() {
  const navigate = useNavigate()
  const setAuth = useAuthStore((s) => s.setAuth)

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
      setError(axiosErr.response?.data?.error || '操作失败，请重试')
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
          style={{ background: 'radial-gradient(circle, rgba(0,240,255,0.15), transparent)' }}
        />
        <div
          className="absolute bottom-1/4 right-1/4 h-[400px] w-[400px] rounded-full opacity-20 blur-[100px]"
          style={{ background: 'radial-gradient(circle, rgba(138,43,226,0.15), transparent)' }}
        />
        {/* 网格线（科技感） */}
        <div
          className="absolute inset-0 opacity-[0.02]"
          style={{
            backgroundImage: `
              linear-gradient(rgba(0,240,255,0.3) 1px, transparent 1px),
              linear-gradient(90deg, rgba(0,240,255,0.3) 1px, transparent 1px)
            `,
            backgroundSize: '60px 60px',
          }}
        />
      </div>

      <div className="relative z-10 w-full max-w-sm animate-scale-in">
        {/* Logo */}
        <div className="mb-10 text-center">
          <div
            className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl animate-float"
            style={{
              background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
              boxShadow: '0 0 30px rgba(0, 240, 255, 0.3), 0 0 60px rgba(0, 240, 255, 0.1)',
            }}
          >
            <Zap size={32} className="text-white" />
          </div>
          <h1 className="font-display text-3xl font-bold tracking-wider" style={{ color: 'var(--text-primary)' }}>
            <span className="text-neon text-neon-glow">N</span>OWEN
          </h1>
          <p className="mt-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
            深空影音 · 未来枢纽
          </p>
        </div>

        {/* 表单 */}
        <form
          onSubmit={handleSubmit}
          className="glass-panel rounded-2xl p-6"
        >
          {/* 表单顶部霓虹线 */}
          <div className="absolute top-0 left-4 right-4 h-px bg-gradient-to-r from-transparent via-neon-blue/30 to-transparent" />

          <h2 className="mb-6 text-center font-display text-base font-semibold tracking-wider" style={{ color: 'var(--text-primary)' }}>
            {isRegister ? '创建账号' : '欢迎回来'}
          </h2>

          {error && (
            <div className="mb-4 rounded-xl px-4 py-3 text-sm text-red-400"
              style={{
                background: 'rgba(239, 68, 68, 0.08)',
                border: '1px solid rgba(239, 68, 68, 0.15)',
              }}
            >
              {error}
            </div>
          )}

          <div className="space-y-4">
            <div>
              <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                用户名
              </label>
              <input
                type="text"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                className="input"
                placeholder="请输入用户名"
                required
                minLength={3}
                autoFocus
              />
            </div>

            <div>
              <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
                密码
              </label>
              <input
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                className="input"
                placeholder="请输入密码"
                required
                minLength={6}
              />
            </div>
          </div>

          <button
            type="submit"
            disabled={loading}
            className="btn-primary mt-6 w-full py-3"
          >
            {loading ? (
              <span className="flex items-center gap-2">
                <span className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
                处理中...
              </span>
            ) : isRegister ? '注册' : '进入深空'}
          </button>

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
              {isRegister ? '已有账号？去登录' : '没有账号？创建一个'}
            </button>
          </div>
        </form>

        {/* 默认账号提示 */}
        {!isRegister && (
          <p className="mt-4 text-center text-xs" style={{ color: 'var(--text-muted)' }}>
            首次使用默认账号: admin / admin123
          </p>
        )}
      </div>
    </div>
  )
}
