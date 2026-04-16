import { useState } from 'react'
import { useAuthStore } from '@/stores/auth'
import { authApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import {
  User,
  Key,
  Shield,
  Save,
  Loader2,
  LogOut,
} from 'lucide-react'
import { useNavigate } from 'react-router-dom'

export default function ProfilePage() {
  const { user, logout } = useAuthStore()
  const navigate = useNavigate()
  const toast = useToast()
  const { t } = useTranslation()

  // 修改密码
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [changingPwd, setChangingPwd] = useState(false)

  const handleChangePassword = async (e: React.FormEvent) => {
    e.preventDefault()
    if (newPassword.length < 6) {
toast.error(t('profile.passwordMinLength'))
      return
    }
    if (newPassword !== confirmPassword) {
toast.error(t('profile.passwordMismatch'))
      return
    }

    setChangingPwd(true)
    try {
      await authApi.changePassword(oldPassword, newPassword)
      toast.success(t('profile.passwordChangeSuccess'))
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error
      if (err?.response?.status === 401) {
        toast.error(t('profile.passwordVerifyFailed'))
      } else {
        toast.error(errorMsg || t('profile.passwordChangeFailed'))
      }
    } finally {
      setChangingPwd(false)
    }
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <div className="mx-auto max-w-2xl space-y-8">
      {/* 用户资料卡片 */}
      <section>
        <h1 className="mb-6 flex items-center gap-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <User size={24} className="text-neon" />
          {t('profile.title')}
        </h1>

        <div className="glass-panel rounded-2xl p-6">
          <div className="flex items-center gap-5">
            {/* 大头像 */}
            <div
              className="flex h-20 w-20 flex-shrink-0 items-center justify-center rounded-2xl text-2xl font-bold"
              style={{
                background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                boxShadow: '0 0 30px var(--neon-blue-20)',
                color: 'var(--text-on-neon)',
              }}
            >
              {user?.username?.charAt(0).toUpperCase()}
            </div>
            <div>
              <h2 className="font-display text-xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                {user?.username}
              </h2>
              <div className="mt-1 flex items-center gap-3">
                <span className="flex items-center gap-1 text-sm" style={{ color: 'var(--text-tertiary)' }}>
                  <Shield size={14} />
                  {user?.role === 'admin' ? t('profile.roleAdmin') : t('profile.roleUser')}
                </span>
                {user?.created_at && (
                  <span className="text-sm" style={{ color: 'var(--text-muted)' }}>
                    {t('profile.registeredAt', { date: new Date(user.created_at).toLocaleDateString() })}
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* 修改密码 */}
      <section>
        <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Key size={20} className="text-neon/60" />
          {t('profile.changePassword')}
        </h2>
        <form onSubmit={handleChangePassword} className="glass-panel rounded-2xl p-6 space-y-4">
          <div>
            <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
              {t('profile.currentPassword')}
            </label>
            <input
              type="password"
              value={oldPassword}
              onChange={(e) => setOldPassword(e.target.value)}
              className="input w-full"
              placeholder={t('profile.currentPasswordPlaceholder')}
              required
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
              {t('profile.newPassword')}
            </label>
            <input
              type="password"
              value={newPassword}
              onChange={(e) => setNewPassword(e.target.value)}
              className="input w-full"
              placeholder={t('profile.newPasswordPlaceholder')}
              required
              minLength={6}
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-semibold uppercase tracking-wider" style={{ color: 'var(--text-secondary)' }}>
              {t('profile.confirmPassword')}
            </label>
            <input
              type="password"
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              className="input w-full"
              placeholder={t('profile.confirmPasswordPlaceholder')}
              required
              minLength={6}
            />
          </div>
          <button
            type="submit"
            disabled={changingPwd || !oldPassword || !newPassword}
            className="btn-primary gap-1.5 px-5 py-2.5 text-sm"
          >
            {changingPwd ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
            {t('profile.verifyAndChange')}
          </button>
        </form>
      </section>

      {/* 登出 */}
      <section>
        <div className="glass-panel rounded-2xl p-6">
          <div className="flex items-center justify-between">
            <div>
              <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{t('profile.logout')}</h3>
              <p className="mt-0.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                {t('profile.logoutHint')}
              </p>
            </div>
            <button
              onClick={handleLogout}
              className="flex items-center gap-2 rounded-xl px-4 py-2.5 text-sm font-medium text-red-400 transition-all hover:bg-red-400/5"
              style={{ border: '1px solid rgba(239, 68, 68, 0.2)' }}
            >
              <LogOut size={16} />
              {t('profile.logout')}
            </button>
          </div>
        </div>
      </section>
    </div>
  )
}
