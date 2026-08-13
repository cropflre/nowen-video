import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { AtSign, Key, Loader2, LogOut, Save, Shield, User } from 'lucide-react'
import { useAuthStore } from '@/stores/auth'
import { authApi, userApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useTranslation } from '@/i18n'
import { Button, Input, PageContainer, Section, Tag } from '@/components/design-system'

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
      <div className="space-y-7">
        <header className="border-b border-[var(--nv-border-subtle)] pb-5">
          <div className="mb-1.5 flex items-center gap-2 text-xs font-medium text-[var(--nv-text-tertiary)]">
            <User size={15} aria-hidden="true" />
            {t('profile.title')}
          </div>
          <h1 className="text-xl font-semibold tracking-[-0.015em] text-[var(--nv-text-primary)]">{t('profile.title')}</h1>
          <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">管理账号身份、登录凭据与当前会话。</p>
        </header>

        <section className="flex items-center gap-4 border-b border-[var(--nv-border-subtle)] pb-6">
          <div className="grid h-12 w-12 shrink-0 place-items-center rounded-[var(--nv-radius-control)] bg-[var(--nv-fill-active)] text-base font-semibold text-[var(--nv-text-secondary)]">
            {user?.username?.charAt(0).toUpperCase()}
          </div>
          <div className="min-w-0 flex-1">
            <h2 className="truncate text-base font-semibold text-[var(--nv-text-primary)]">{user?.username}</h2>
            <div className="mt-1.5 flex flex-wrap items-center gap-2">
              <Tag>
                <Shield size={11} aria-hidden="true" />
                {user?.role === 'admin' ? t('profile.roleAdmin') : t('profile.roleUser')}
              </Tag>
              {user?.created_at && (
                <span className="text-[11px] text-[var(--nv-text-tertiary)]">
                  {t('profile.registeredAt', { date: new Date(user.created_at).toLocaleDateString() })}
                </span>
              )}
            </div>
          </div>
        </section>

        <Section
          title={t('profile.updateUsername')}
          description={t('profile.usernameHint')}
          action={<AtSign size={16} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />}
        >
          <form onSubmit={handleChangeUsername} className="max-w-lg space-y-3 border-y border-[var(--nv-border-subtle)] py-4">
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
              {savingUsername ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" aria-hidden="true" /> : <Save size={15} aria-hidden="true" />}
              {t('profile.saveUsername')}
            </Button>
          </form>
        </Section>

        <Section
          title={t('profile.changePassword')}
          description="更新密码后会刷新当前登录会话。"
          action={<Key size={16} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />}
        >
          <form onSubmit={handleChangePassword} className="max-w-lg space-y-3 border-y border-[var(--nv-border-subtle)] py-4">
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
              {changingPwd ? <Loader2 size={15} className="animate-spin motion-reduce:animate-none" aria-hidden="true" /> : <Save size={15} aria-hidden="true" />}
              {t('profile.verifyAndChange')}
            </Button>
          </form>
        </Section>

        <Section title={t('profile.logout')} description={t('profile.logoutHint')}>
          <div className="flex items-center justify-between gap-4 border-y border-[var(--nv-border-subtle)] py-3">
            <div className="min-w-0">
              <div className="text-sm font-medium text-[var(--nv-text-primary)]">结束当前会话</div>
              <p className="mt-0.5 text-[11px] leading-5 text-[var(--nv-text-tertiary)]">退出后需要重新登录才能访问媒体库。</p>
            </div>
            <Button type="button" variant="danger" onClick={handleLogout}>
              <LogOut size={15} aria-hidden="true" />
              {t('profile.logout')}
            </Button>
          </div>
        </Section>
      </div>
    </PageContainer>
  )
}

function FormField({ label, htmlFor, children }: { label: string; htmlFor: string; children: React.ReactNode }) {
  return (
    <div className="space-y-1.5">
      <label htmlFor={htmlFor} className="block text-[11px] font-medium uppercase tracking-[0.07em] text-[var(--nv-text-tertiary)]">{label}</label>
      {children}
    </div>
  )
}
