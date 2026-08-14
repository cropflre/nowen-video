import { useEffect, useMemo, useRef, useState, type Dispatch, type SetStateAction } from 'react'
import type { Library, User } from '@/types'
import { adminApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import { useDialog } from '@/components/Dialog'
import {
  AlertCircle,
  Ban,
  Check,
  Clock,
  FolderOpen,
  KeyRound,
  Loader2,
  RotateCcw,
  Search,
  Shield,
  Trash2,
  UserPlus,
  Users,
} from 'lucide-react'
import {
  Button,
  EmptyState,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Select,
  Surface,
  Tag,
} from '@/components/design-system'
import { AdminPanel, AdminStatus } from '@/components/admin/AdminPrimitives'

const RATING_OPTIONS = [
  { value: 'G', label: 'G — 所有年龄' },
  { value: 'PG', label: 'PG — 家长指导' },
  { value: 'PG-13', label: 'PG-13 — 13岁以上' },
  { value: 'R', label: 'R — 限制级' },
  { value: 'NC-17', label: 'NC-17 — 17岁以下禁止' },
]

interface UsersTabProps {
  users: User[]
  setUsers: Dispatch<SetStateAction<User[]>>
}

export default function UsersTab({ users, setUsers }: UsersTabProps) {
  const toast = useToast()
  const dialog = useDialog()
  const [libraries, setLibraries] = useState<Library[]>([])
  const [editingUser, setEditingUser] = useState<string | null>(null)
  const [loadingPerm, setLoadingPerm] = useState(false)
  const [savingPerm, setSavingPerm] = useState(false)
  const [keyword, setKeyword] = useState('')
  const permissionRequestRef = useRef(0)

  const [resetPwdUser, setResetPwdUser] = useState<User | null>(null)
  const [resetPwdValue, setResetPwdValue] = useState('')
  const [resetForceChange, setResetForceChange] = useState(true)
  const [resettingPwd, setResettingPwd] = useState(false)

  const [showCreateModal, setShowCreateModal] = useState(false)
  const [creatingUser, setCreatingUser] = useState(false)
  const [newUser, setNewUser] = useState({
    username: '',
    password: '',
    role: 'user' as 'user' | 'admin',
    nickname: '',
    email: '',
  })

  const [permLibraries, setPermLibraries] = useState<string[]>([])
  const [permRating, setPermRating] = useState('NC-17')
  const [permTimeLimit, setPermTimeLimit] = useState(0)

  useEffect(() => {
    libraryApi.list().then((res) => setLibraries(res.data.data || [])).catch(() => {
      toast.error('媒体库列表加载失败，暂时无法配置用户媒体库权限')
    })
  }, [toast])

  const filteredUsers = useMemo(() => {
    if (!keyword.trim()) return users
    const kw = keyword.toLowerCase()
    return users.filter((user) =>
      user.username.toLowerCase().includes(kw) ||
      (user.nickname || '').toLowerCase().includes(kw) ||
      (user.email || '').toLowerCase().includes(kw)
    )
  }, [users, keyword])

  const handleDeleteUser = async (id: string) => {
    const ok = await dialog.confirm({
      title: '删除用户',
      message: '确定删除此用户？',
      confirmText: '删除',
      variant: 'danger',
    })
    if (!ok) return
    try {
      await adminApi.deleteUser(id)
      setUsers((current) => current.filter((user) => user.id !== id))
      toast.success('用户已删除')
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '删除用户失败')
    }
  }

  const handleToggleDisabled = async (user: User) => {
    const next = !user.disabled
    const actionText = next ? '禁用' : '启用'
    const ok = await dialog.confirm({
      title: `${actionText}用户`,
      message: `确定${actionText} ${user.username}？${next ? '该用户将无法登录。' : ''}`,
      confirmText: actionText,
      variant: next ? 'warning' : 'primary',
    })
    if (!ok) return
    try {
      await adminApi.setUserDisabled(user.id, next)
      setUsers((current) => current.map((item) => item.id === user.id ? { ...item, disabled: next } : item))
      toast.success(`已${actionText}用户 ${user.username}`)
    } catch (err: any) {
      toast.error(err?.response?.data?.error || `${actionText}失败`)
    }
  }

  const handleCreateUser = async () => {
    if (newUser.username.trim().length < 3) {
      toast.error('用户名至少3位')
      return
    }
    if (newUser.password.length < 6) {
      toast.error('密码至少6位')
      return
    }
    setCreatingUser(true)
    try {
      const res = await adminApi.createUser({
        username: newUser.username.trim(),
        password: newUser.password,
        role: newUser.role,
        nickname: newUser.nickname.trim() || undefined,
        email: newUser.email.trim() || undefined,
      })
      setUsers((current) => [res.data.data, ...current])
      toast.success(`已创建用户 ${res.data.data.username}`)
      setShowCreateModal(false)
      setNewUser({ username: '', password: '', role: 'user', nickname: '', email: '' })
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '创建失败')
    } finally {
      setCreatingUser(false)
    }
  }

  const closePermEditor = () => {
    permissionRequestRef.current += 1
    setEditingUser(null)
    setLoadingPerm(false)
  }

  const openPermEditor = async (userId: string) => {
    if (savingPerm) return
    if (editingUser === userId) {
      closePermEditor()
      return
    }

    const requestId = ++permissionRequestRef.current
    setEditingUser(userId)
    setLoadingPerm(true)
    try {
      const res = await adminApi.getUserPermission(userId)
      if (permissionRequestRef.current !== requestId) return
      const permission = res.data.data
      setPermLibraries(permission.allowed_libraries ? permission.allowed_libraries.split(',').filter(Boolean) : [])
      setPermRating(permission.max_rating_level || 'NC-17')
      setPermTimeLimit(Math.min(1440, Math.max(0, permission.daily_time_limit || 0)))
    } catch (err: any) {
      if (permissionRequestRef.current !== requestId) return
      toast.error(err?.response?.data?.error || '权限加载失败，未对现有权限做任何修改')
      setEditingUser(null)
    } finally {
      if (permissionRequestRef.current === requestId) setLoadingPerm(false)
    }
  }

  const savePerm = async () => {
    const userId = editingUser
    if (!userId || loadingPerm) return
    setSavingPerm(true)
    try {
      await adminApi.updateUserPermission(userId, {
        allowed_libraries: permLibraries.join(','),
        max_rating_level: permRating,
        daily_time_limit: Math.min(1440, Math.max(0, permTimeLimit)),
      })
      toast.success('权限已保存')
      if (editingUser === userId) closePermEditor()
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '保存权限失败')
    } finally {
      setSavingPerm(false)
    }
  }

  const handleResetPassword = async () => {
    if (!resetPwdUser || resetPwdValue.length < 6) {
      toast.error('新密码至少6位')
      return
    }
    setResettingPwd(true)
    try {
      await adminApi.resetUserPassword(resetPwdUser.id, resetPwdValue, resetForceChange)
      toast.success(`已重置 ${resetPwdUser.username} 的密码`)
      setResetPwdUser(null)
      setResetPwdValue('')
      setResetForceChange(true)
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '重置密码失败')
    } finally {
      setResettingPwd(false)
    }
  }

  const toggleLibrary = (libraryId: string) => {
    setPermLibraries((current) =>
      current.includes(libraryId)
        ? current.filter((id) => id !== libraryId)
        : [...current, libraryId]
    )
  }

  const formatLastLogin = (user: User) => {
    if (!user.last_login_at) return '从未登录'
    const date = new Date(user.last_login_at)
    const diff = Date.now() - date.getTime()
    const minutes = Math.floor(diff / 60000)
    if (minutes < 1) return '刚刚'
    if (minutes < 60) return `${minutes} 分钟前`
    if (minutes < 60 * 24) return `${Math.floor(minutes / 60)} 小时前`
    if (minutes < 60 * 24 * 30) return `${Math.floor(minutes / 60 / 24)} 天前`
    return date.toLocaleDateString('zh-CN')
  }

  return (
    <div className="space-y-5">
      <AdminPanel
        title="用户管理"
        description="创建账号、管理访问权限与登录状态。管理员账号不会显示禁用或删除操作。"
        icon={<Users size={18} />}
        actions={(
          <div className="flex flex-wrap items-center gap-2">
            <div className="relative min-w-[220px] flex-1 sm:flex-none">
              <Search size={14} className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-[var(--nv-text-tertiary)]" />
              <Input
                value={keyword}
                onChange={(event) => setKeyword(event.target.value)}
                placeholder="搜索用户名 / 昵称 / 邮箱"
                className="pl-9 sm:w-64"
              />
            </div>
            <Button variant="primary" onClick={() => setShowCreateModal(true)}>
              <UserPlus size={15} />
              新建用户
            </Button>
          </div>
        )}
        bodyClassName="p-0"
      >
        <div className="flex items-center justify-between gap-3 border-b border-[var(--nv-border-subtle)] px-5 py-3 text-xs text-[var(--nv-text-tertiary)] sm:px-6">
          <span>当前显示 {filteredUsers.length} 位用户</span>
          <span>总计 {users.length}</span>
        </div>

        {filteredUsers.length === 0 ? (
          <EmptyState
            icon={<Users size={24} />}
            title={keyword.trim() ? '没有匹配的用户' : '还没有用户'}
            description={keyword.trim() ? '尝试使用其他用户名、昵称或邮箱关键词。' : '创建用户后可在这里配置媒体库访问与内容限制。'}
            action={keyword.trim() ? <Button variant="secondary" onClick={() => setKeyword('')}>清除搜索</Button> : undefined}
          />
        ) : (
          <div className="divide-y divide-[var(--nv-border-subtle)]">
            {filteredUsers.map((user) => (
              <div key={user.id} className={user.disabled ? 'bg-[var(--nv-bg-surface-soft)] opacity-75' : undefined}>
                <div className="flex flex-col gap-4 px-5 py-4 sm:px-6 lg:flex-row lg:items-center lg:justify-between">
                  <div className="flex min-w-0 items-start gap-3">
                    <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-[var(--nv-bg-active)] text-sm font-semibold text-[var(--nv-action-primary)]">
                      {user.username.charAt(0).toUpperCase()}
                    </div>
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <span className="font-medium text-[var(--nv-text-primary)]">{user.username}</span>
                        {user.nickname && <span className="text-xs text-[var(--nv-text-tertiary)]">{user.nickname}</span>}
                        <Tag tone={user.role === 'admin' ? 'brand' : 'neutral'}>{user.role === 'admin' ? '管理员' : '普通用户'}</Tag>
                        {user.disabled && <Tag tone="danger">已禁用</Tag>}
                        {user.must_change_pwd && <Tag tone="warning">需改密</Tag>}
                      </div>
                      <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-[var(--nv-text-tertiary)]">
                        <span>注册 {new Date(user.created_at).toLocaleDateString('zh-CN')}</span>
                        <span>最近登录 {formatLastLogin(user)}</span>
                        {user.last_login_ip && <span title="最近登录 IP">IP {user.last_login_ip}</span>}
                        {user.email && <span className="truncate">{user.email}</span>}
                      </div>
                    </div>
                  </div>

                  <div className="flex flex-wrap items-center gap-2 lg:justify-end">
                    {user.role !== 'admin' && (
                      <Button
                        size="sm"
                        variant={editingUser === user.id ? 'primary' : 'secondary'}
                        onClick={() => void openPermEditor(user.id)}
                        disabled={savingPerm}
                      >
                        <Shield size={14} />
                        权限
                      </Button>
                    )}
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => {
                        setResetPwdUser(user)
                        setResetPwdValue('')
                        setResetForceChange(true)
                      }}
                    >
                      <KeyRound size={14} />
                      密码
                    </Button>
                    {user.role !== 'admin' && (
                      <Button size="sm" variant="ghost" onClick={() => handleToggleDisabled(user)}>
                        {user.disabled ? <RotateCcw size={14} /> : <Ban size={14} />}
                        {user.disabled ? '启用' : '禁用'}
                      </Button>
                    )}
                    {user.role !== 'admin' && (
                      <Button
                        size="sm"
                        variant="danger"
                        iconOnly
                        aria-label={`删除用户 ${user.username}`}
                        title="删除用户"
                        onClick={() => handleDeleteUser(user.id)}
                      >
                        <Trash2 size={15} />
                      </Button>
                    )}
                  </div>
                </div>

                {editingUser === user.id && (
                  <div className="border-t border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-5 py-5 sm:px-6">
                    {loadingPerm ? (
                      <div className="flex items-center justify-center py-8 text-[var(--nv-text-tertiary)]">
                        <Loader2 size={20} className="animate-spin" />
                      </div>
                    ) : (
                      <div className="space-y-5">
                        <div className="flex flex-wrap items-start justify-between gap-3">
                          <div>
                            <h4 className="flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
                              <Shield size={15} className="text-[var(--nv-action-primary)]" />
                              {user.username} 的权限
                            </h4>
                            <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">不选择媒体库代表允许访问全部媒体库。</p>
                          </div>
                          <AdminStatus tone="active">自定义权限</AdminStatus>
                        </div>

                        <div>
                          <label className="mb-2 flex items-center gap-1.5 text-xs font-medium text-[var(--nv-text-secondary)]">
                            <FolderOpen size={13} />
                            可访问媒体库
                          </label>
                          {libraries.length > 0 ? (
                            <div className="flex flex-wrap gap-2">
                              {libraries.map((library) => {
                                const selected = permLibraries.includes(library.id)
                                return (
                                  <Button
                                    key={library.id}
                                    size="sm"
                                    variant={selected ? 'primary' : 'secondary'}
                                    onClick={() => toggleLibrary(library.id)}
                                    disabled={savingPerm}
                                  >
                                    {selected && <Check size={13} />}
                                    {library.name}
                                  </Button>
                                )
                              })}
                            </div>
                          ) : (
                            <p className="text-xs text-[var(--nv-text-tertiary)]">当前没有可配置的媒体库。</p>
                          )}
                        </div>

                        <div className="grid gap-4 md:grid-cols-2">
                          <div>
                            <label className="mb-2 block text-xs font-medium text-[var(--nv-text-secondary)]">最高可观看内容分级</label>
                            <Select value={permRating} onChange={(event) => setPermRating(event.target.value)} className="w-full" disabled={savingPerm}>
                              {RATING_OPTIONS.map((option) => (
                                <option key={option.value} value={option.value}>{option.label}</option>
                              ))}
                            </Select>
                          </div>
                          <div>
                            <label className="mb-2 flex items-center gap-1.5 text-xs font-medium text-[var(--nv-text-secondary)]">
                              <Clock size={13} />
                              每日观看时长限制
                            </label>
                            <Input
                              type="number"
                              min={0}
                              max={1440}
                              value={permTimeLimit}
                              disabled={savingPerm}
                              onChange={(event) => {
                                const value = parseInt(event.target.value, 10) || 0
                                setPermTimeLimit(Math.min(1440, Math.max(0, value)))
                              }}
                            />
                            <p className="mt-1.5 text-xs text-[var(--nv-text-tertiary)]">
                              {permTimeLimit > 0
                                ? `${Math.floor(permTimeLimit / 60)} 小时 ${permTimeLimit % 60} 分钟 / 天`
                                : '0 表示不限制'}
                            </p>
                          </div>
                        </div>

                        <div className="flex flex-wrap items-center justify-end gap-2 border-t border-[var(--nv-border-subtle)] pt-4">
                          <Button variant="ghost" onClick={closePermEditor} disabled={savingPerm}>取消</Button>
                          <Button variant="primary" loading={savingPerm} onClick={() => void savePerm()} disabled={loadingPerm}>
                            {!savingPerm && <Check size={14} />}
                            {savingPerm ? '保存中...' : '保存权限'}
                          </Button>
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </AdminPanel>

      <Surface className="flex items-start gap-2 border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] p-4 text-xs leading-5 text-[var(--nv-text-secondary)]">
        <AlertCircle size={15} className="mt-0.5 shrink-0 text-[var(--nv-status-warning)]" />
        <span>管理员可直接创建账号，默认要求首次登录强制改密；也可让用户通过邀请码自行注册。禁用账号会立即吊销该用户持有的登录凭证。</span>
      </Surface>

      <Modal open={showCreateModal} onClose={() => setShowCreateModal(false)} size="sm" ariaLabel="创建新用户">
        <ModalHeader
          title="创建新用户"
          description="创建账号后可继续配置媒体库访问和内容限制。"
          icon={<UserPlus size={18} />}
          onClose={() => setShowCreateModal(false)}
        />
        <ModalBody className="space-y-4">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">用户名 *</label>
            <Input
              value={newUser.username}
              onChange={(event) => setNewUser({ ...newUser, username: event.target.value })}
              placeholder="至少 3 位"
              autoFocus
              disabled={creatingUser}
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">初始密码 *</label>
            <Input
              type="password"
              value={newUser.password}
              onChange={(event) => setNewUser({ ...newUser, password: event.target.value })}
              placeholder="至少 6 位，首次登录须修改"
              minLength={6}
              disabled={creatingUser}
            />
          </div>
          <div>
            <label className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">角色</label>
            <Select
              value={newUser.role}
              onChange={(event) => setNewUser({ ...newUser, role: event.target.value as 'user' | 'admin' })}
              className="w-full"
              disabled={creatingUser}
            >
              <option value="user">普通用户</option>
              <option value="admin">管理员</option>
            </Select>
          </div>
          <div className="grid gap-3 sm:grid-cols-2">
            <div>
              <label className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">昵称（可选）</label>
              <Input value={newUser.nickname} onChange={(event) => setNewUser({ ...newUser, nickname: event.target.value })} disabled={creatingUser} />
            </div>
            <div>
              <label className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">邮箱（可选）</label>
              <Input type="email" value={newUser.email} onChange={(event) => setNewUser({ ...newUser, email: event.target.value })} disabled={creatingUser} />
            </div>
          </div>
        </ModalBody>
        <ModalFooter>
          <Button variant="ghost" onClick={() => setShowCreateModal(false)} disabled={creatingUser}>取消</Button>
          <Button variant="primary" loading={creatingUser} onClick={() => void handleCreateUser()}>
            {!creatingUser && <Check size={14} />}
            {creatingUser ? '创建中...' : '创建'}
          </Button>
        </ModalFooter>
      </Modal>

      <Modal open={Boolean(resetPwdUser)} onClose={() => !resettingPwd && setResetPwdUser(null)} size="sm" ariaLabel="重置用户密码">
        <ModalHeader
          title="重置密码"
          description={resetPwdUser ? `为用户 ${resetPwdUser.username} 设置新的登录密码。` : undefined}
          icon={<KeyRound size={18} />}
          onClose={() => !resettingPwd && setResetPwdUser(null)}
        />
        <ModalBody className="space-y-4">
          <div>
            <label className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">新密码</label>
            <Input
              type="password"
              value={resetPwdValue}
              onChange={(event) => setResetPwdValue(event.target.value)}
              placeholder="至少 6 位"
              minLength={6}
              autoFocus
              disabled={resettingPwd}
            />
          </div>
          <label className="flex items-start gap-2.5 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3 text-xs leading-5 text-[var(--nv-text-secondary)]">
            <input
              type="checkbox"
              checked={resetForceChange}
              onChange={(event) => setResetForceChange(event.target.checked)}
              className="mt-0.5 h-4 w-4"
              style={{ accentColor: 'var(--nv-action-primary)' }}
              disabled={resettingPwd}
            />
            <span>要求用户下次登录强制修改密码（推荐）</span>
          </label>
        </ModalBody>
        <ModalFooter>
          <Button variant="ghost" onClick={() => setResetPwdUser(null)} disabled={resettingPwd}>取消</Button>
          <Button
            variant="primary"
            loading={resettingPwd}
            disabled={resetPwdValue.length < 6}
            onClick={() => void handleResetPassword()}
          >
            {!resettingPwd && <Check size={14} />}
            {resettingPwd ? '重置中...' : '确认重置'}
          </Button>
        </ModalFooter>
      </Modal>
    </div>
  )
}
