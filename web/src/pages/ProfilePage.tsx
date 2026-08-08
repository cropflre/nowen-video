import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AtSign, Key, Loader2, LogOut, Save, Shield, User } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { authApi, userApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { Button, Input, PageContainer, Section, Surface, Tag } from '@/components/design-system'

export default function ProfilePage() {
  const { user, setAuth, updateUser, logout } = useAuthStore()
  const navigate = useNavigate()
  const toast = useToast()
  const { t } = useTranslation()

  const [newUsername, setNewUsername] = useState(user?.username ?? '')
  const [savingUsername, setSavingUsername] = useState(false)
  const [oldPassword, setOldPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [changingPwd, setChangingPwd] = useState(false)

  const handleChangeUsername = async (event: React.FormEvent) => {
    event.preventDefault()
    const trimmed = newUsername.trim()
    if (trimmed.length < 3 || trimmed.length > 32) {
      toast.error(t('profile.usernameInvalid'))
      return
    }
    if (trimmed === user?.username) return

    setSavingUsername(true)
    try {
      const res = await userApi.updateProfile({ username: trimmed })
      const updatedUser = res.data.data
      if (res.data.token) setAuth(res.data.token, updatedUser)
      else updateUser(updatedUser)
      toast.success(t('profile.usernameChangeSuccess'))
    } catch (err: any) {
      if (err?.response?.status === 409) toast.error(t('profile.usernameTaken'))
      else toast.error(err?.response?.data?.error || t('profile.usernameChangeFailed'))
    } finally {
      setSavingUsername(false)
    }
  }

  const handleChangePassword = async (event: React.FormEvent) => {
    event.preventDefault()
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
      const res = await authApi.changePassword(oldPassword, newPassword)
      const tokenData = res.data.data
      if (!tokenData?.token || !tokenData.user) {
        toast.error(res.data.message || t('profile.passwordChangeFailed'))
        return
      }
      setAuth(tokenData.token, { ...tokenData.user, must_change_pwd: false })
      toast.success(t('profile.passwordChangeSuccess'))
      setOldPassword('')
      setNewPassword('')
      setConfirmPassword('')
    } catch (err: any) {
      const errorMsg = err?.response?.data?.error
      if (err?.response?.status === 401) toast.error(t('profile.passwordVerifyFailed'))
      else toast.error(errorMsg || t('profile.passwordChangeFailed'))
    } finally {
      setChangingPwd(false)
    }
  }

  const handleLogout = () => {
    logout()
    navigate('/login')
  }

  return (
    <PageContainer className="max-w-3xl">
      <div className="space-y-8">
        <div>
          <div className="mb-2 flex items-center gap-2 text-sm font-medium text-[var(--nv-action-primary)]">
            <User size={17} aria-hidden="true" />
            {t('profile.title')}
          </div>
          <h1 className="text-2xl font-semibold tracking-[-0.02em] text-[var(--nv-text-primary)]">{t('profile.title')}</h1>
          <p className="mt-2 text-sm text-[var(--nv-text-tertiary)]">管理账号身份、登录凭据与会话。</p>
        </div>

        <Surface className="p-5 sm:p-6">
          <div className="flex flex-col gap-5 sm:flex-row sm:items-center">
            <div className="flex h-20 w-20 shrink-0 items-center justify-center rounded-[var(--nv-radius-container)] bg-[var(--nv-action-primary)] text-2xl font-bold text-[var(--nv-text-on-brand)] shadow-[var(--nv-shadow-card)]">
              {user?.username?.charAt(0).toUpperCase()}
            </div>
            <div className="min-w-0 flex-1">
              <h2 className="truncate text-xl font-semibold text-[var(--nv-text-primary)]">{user?.username}</h2>
              <div className="mt-2 flex flex-wrap items-center gap-2">
                <Tag tone={user?.role === 'admin' ? 'brand' : 'neutral'}>
                  <Shield size={12} aria-hidden="true" />
                  {user?.role === 'admin' ? t('profile.roleAdmin') : t('profile.roleUser')}
                </Tag>
                {user?.created_at && (
                  <span className="text-xs text-[var(--nv-text-tertiary)]">
                    {t('profile.registeredAt', { date: new Date(user.created_at).toLocaleDateString() })}
                  </span>
                )}
              </div>
            </div>
          </div>
        </Surface>

        <Section
          title={t('profile.updateUsername')}
          description={t('profile.usernameHint')}
          action={<AtSign size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />}
        >
          <Surface className="p-5 sm:p-6">
            <form onSubmit={handleChangeUsername} className="space-y-4">
              <FormField label={t('profile.username')} htmlFor="profile-username">
                <Input
                  id="profile-username"
                  type="text"
                  value={newUsername}
                  onChange={(event) => setNewUsername(event.target.value)}
                  placeholder={t('profile.usernamePlaceholder')}
                  minLength={3}
                  maxLength={32}
                  required
                  autoComplete="username"
                />
              </FormField>
              <Button
                type="submit"
                variant="primary"
                disabled={savingUsername || !newUsername.trim() || newUsername.trim() === user?.username}
              >
                {savingUsername ? <Loader2 size={15} className="animate-spin" aria-hidden="true" /> : <Save size={15} aria-hidden="true" />}
                {t('profile.saveUsername')}
              </Button>
            </form>
          </Surface>
        </Section>

        <Section
          title={t('profile.changePassword')}
          description="更新密码后会刷新当前登录会话。"
          action={<Key size={18} className="text-[var(--nv-action-primary)]" aria-hidden="true" />}
        >
          <Surface className="p-5 sm:p-6">
            <form onSubmit={handleChangePassword} className="space-y-4">
              <FormField label={t('profile.currentPassword')} htmlFor="profile-current-password">
                <Input
                  id="profile-current-password"
                  type="password"
                  value={oldPassword}
                  onChange={(event) => setOldPassword(event.target.value)}
                  placeholder={t('profile.currentPasswordPlaceholder')}
                  required
                  autoComplete="current-password"
                />
              </FormField>
              <FormField label={t('profile.newPassword')} htmlFor="profile-new-password">
                <Input
                  id="profile-new-password"
                  type="password"
                  value={newPassword}
                  onChange={(event) => setNewPassword(event.target.value)}
                  placeholder={t('profile.newPasswordPlaceholder')}
                  required
                  minLength={6}
                  autoComplete="new-password"
                />
              </FormField>
              <FormField label={t('profile.confirmPassword')} htmlFor="profile-confirm-password">
                <Input
                  id="profile-confirm-password"
                  type="password"
                  value={confirmPassword}
                  onChange={(event) => setConfirmPassword(event.target.value)}
                  placeholder={t('profile.confirmPasswordPlaceholder')}
                  required
                  minLength={6}
                  autoComplete="new-password"
                />
              </FormField>
              <Button type="submit" variant="primary" disabled={changingPwd || !oldPassword || !newPassword}>
                {changingPwd ? <Loader2 size={15} className="animate-spin" aria-hidden="true" /> : <Save size={15} aria-hidden="true" />}
                {t('profile.verifyAndChange')}
              </Button>
            </form>
          </Surface>
        </Section>

        <Surface className="p-5 sm:p-6">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">{t('profile.logout')}</h3>
              <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">{t('profile.logoutHint')}</p>
            </div>
            <Button type="button" variant="danger" onClick={handleLogout}>
              <LogOut size={16} aria-hidden="true" />
              {t('profile.logout')}
            </Button>
          </div>
        </Surface>
      </div>
    </PageContainer>
  )
}

function FormField({ label, htmlFor, children }: { label: string; htmlFor: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={htmlFor} className="block text-xs font-semibold uppercase tracking-[0.08em] text-[var(--nv-text-secondary)]">{label}</label>
      {children}
    </div>
  )
}
