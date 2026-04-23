import { useState, useEffect, useMemo } from 'react'
import type { User, UserPermission, Library } from '@/types'
import { adminApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Users,
  Trash2,
  AlertCircle,
  Shield,
  Check,
  Loader2,
  Clock,
  Eye,
  FolderOpen,
  KeyRound,
  X,
  UserPlus,
  Search,
  Ban,
  RotateCcw,
} from 'lucide-react'
import clsx from 'clsx'

// 内容分级选项
const RATING_OPTIONS = [
  { value: 'G', label: 'G — 所有年龄', color: 'text-green-400' },
  { value: 'PG', label: 'PG — 家长指导', color: 'text-blue-400' },
  { value: 'PG-13', label: 'PG-13 — 13岁以上', color: 'text-yellow-400' },
  { value: 'R', label: 'R — 限制级', color: 'text-orange-400' },
  { value: 'NC-17', label: 'NC-17 — 17岁以下禁止', color: 'text-red-400' },
]

interface UsersTabProps {
  users: User[]
  setUsers: React.Dispatch<React.SetStateAction<User[]>>
}

export default function UsersTab({ users, setUsers }: UsersTabProps) {
  const toast = useToast()
  const [libraries, setLibraries] = useState<Library[]>([])
  const [editingUser, setEditingUser] = useState<string | null>(null)
  const [, setPerm] = useState<UserPermission | null>(null)
  const [loadingPerm, setLoadingPerm] = useState(false)
  const [savingPerm, setSavingPerm] = useState(false)

  // 搜索
  const [keyword, setKeyword] = useState('')

  // 重置密码
  const [resetPwdUser, setResetPwdUser] = useState<User | null>(null)
  const [resetPwdValue, setResetPwdValue] = useState('')
  const [resetForceChange, setResetForceChange] = useState(true)
  const [resettingPwd, setResettingPwd] = useState(false)

  // 创建用户
  const [showCreateModal, setShowCreateModal] = useState(false)
  const [creatingUser, setCreatingUser] = useState(false)
  const [newUser, setNewUser] = useState({
    username: '',
    password: '',
    role: 'user' as 'user' | 'admin',
    nickname: '',
    email: '',
  })

  // 权限编辑表单
  const [permLibraries, setPermLibraries] = useState<string[]>([])
  const [permRating, setPermRating] = useState('NC-17')
  const [permTimeLimit, setPermTimeLimit] = useState(0)

  useEffect(() => {
    libraryApi.list().then((res) => setLibraries(res.data.data || [])).catch(() => {})
  }, [])

  // 过滤后的用户列表
  const filteredUsers = useMemo(() => {
    if (!keyword.trim()) return users
    const kw = keyword.toLowerCase()
    return users.filter((u) =>
      u.username.toLowerCase().includes(kw) ||
      (u.nickname || '').toLowerCase().includes(kw) ||
      (u.email || '').toLowerCase().includes(kw)
    )
  }, [users, keyword])

  const handleDeleteUser = async (id: string) => {
    if (!confirm('确定删除此用户？')) return
    try {
      await adminApi.deleteUser(id)
      setUsers((u) => u.filter((user) => user.id !== id))
      toast.success('用户已删除')
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '删除用户失败')
    }
  }

  // 切换用户启用/禁用
  const handleToggleDisabled = async (user: User) => {
    const next = !user.disabled
    const actionText = next ? '禁用' : '启用'
    if (!confirm(`确定${actionText} ${user.username}？${next ? '该用户将无法登录。' : ''}`)) return
    try {
      await adminApi.setUserDisabled(user.id, next)
      setUsers((prev) => prev.map((u) => (u.id === user.id ? { ...u, disabled: next } : u)))
      toast.success(`已${actionText}用户 ${user.username}`)
    } catch (err: any) {
      toast.error(err?.response?.data?.error || `${actionText}失败`)
    }
  }

  // 创建用户
  const handleCreateUser = async () => {
    if (newUser.username.trim().length < 3) { toast.error('用户名至少3位'); return }
    if (newUser.password.length < 6) { toast.error('密码至少6位'); return }
    setCreatingUser(true)
    try {
      const res = await adminApi.createUser({
        username: newUser.username.trim(),
        password: newUser.password,
        role: newUser.role,
        nickname: newUser.nickname || undefined,
        email: newUser.email || undefined,
      })
      setUsers((prev) => [res.data.data, ...prev])
      toast.success(`已创建用户 ${res.data.data.username}`)
      setShowCreateModal(false)
      setNewUser({ username: '', password: '', role: 'user', nickname: '', email: '' })
    } catch (err: any) {
      toast.error(err?.response?.data?.error || '创建失败')
    } finally {
      setCreatingUser(false)
    }
  }

  // 打开权限编辑面板
  const openPermEditor = async (userId: string) => {
    if (editingUser === userId) { setEditingUser(null); return }
    setEditingUser(userId)
    setLoadingPerm(true)
    try {
      const res = await adminApi.getUserPermission(userId)
      const p = res.data.data
      setPerm(p)
      setPermLibraries(p.allowed_libraries ? p.allowed_libraries.split(',').filter(Boolean) : [])
      setPermRating(p.max_rating_level || 'NC-17')
      setPermTimeLimit(p.daily_time_limit || 0)
    } catch {
      setPermLibraries([])
      setPermRating('NC-17')
      setPermTimeLimit(0)
    } finally {
      setLoadingPerm(false)
    }
  }

  // 保存权限
  const savePerm = async () => {
    if (!editingUser) return
    setSavingPerm(true)
    try {
      await adminApi.updateUserPermission(editingUser, {
        allowed_libraries: permLibraries.join(','),
        max_rating_level: permRating,
        daily_time_limit: permTimeLimit,
      })
      toast.success('权限已保存')
      setEditingUser(null)
    } catch {
      toast.error('保存权限失败')
    } finally {
      setSavingPerm(false)
    }
  }

  // 重置密码
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

  const toggleLibrary = (libId: string) => {
    setPermLibraries((prev) =>
      prev.includes(libId) ? prev.filter((id) => id !== libId) : [...prev, libId]
    )
  }

  const formatLastLogin = (user: User) => {
    if (!user.last_login_at) return '从未登录'
    const d = new Date(user.last_login_at)
    const now = Date.now()
    const diff = now - d.getTime()
    const min = Math.floor(diff / 60000)
    if (min < 1) return '刚刚'
    if (min < 60) return `${min} 分钟前`
    if (min < 60 * 24) return `${Math.floor(min / 60)} 小时前`
    if (min < 60 * 24 * 30) return `${Math.floor(min / 60 / 24)} 天前`
    return d.toLocaleDateString('zh-CN')
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between gap-3 flex-wrap">
        <h2 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Users size={20} className="text-neon/60" />
          用户管理
        </h2>
        <div className="flex items-center gap-2">
          <div className="relative">
            <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-surface-500" />
            <input
              value={keyword}
              onChange={(e) => setKeyword(e.target.value)}
              placeholder="搜索用户名/昵称/邮箱"
              className="input pl-9 w-60 py-1.5 text-sm"
            />
          </div>
          <button
            onClick={() => setShowCreateModal(true)}
            className="btn-primary gap-1.5 px-3 py-1.5 text-sm"
          >
            <UserPlus size={14} />
            新建用户
          </button>
          <span className="text-sm text-surface-400">共 {filteredUsers.length} / {users.length}</span>
        </div>
      </div>

      <div className="space-y-2">
        {filteredUsers.map((user) => (
          <div key={user.id}>
            <div
              className={clsx(
                'glass-panel-subtle flex items-center justify-between rounded-xl p-4 transition-all',
                user.disabled ? 'opacity-60' : 'hover:border-neon-blue/20'
              )}
            >
              <div className="flex items-center gap-3">
                <div
                  className="flex h-10 w-10 items-center justify-center rounded-full text-sm font-bold"
                  style={{
                    background: user.disabled
                      ? 'linear-gradient(135deg, #6b7280, #4b5563)'
                      : 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))',
                    boxShadow: user.disabled ? 'none' : 'var(--shadow-neon)',
                    color: 'var(--text-on-neon)',
                  }}
                >
                  {user.username.charAt(0).toUpperCase()}
                </div>
                <div>
                  <p className="font-medium flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                    {user.username}
                    {user.nickname && <span className="text-xs text-surface-400">({user.nickname})</span>}
                    {user.disabled && (
                      <span className="rounded px-1.5 py-0.5 text-[10px] font-semibold" style={{ background: 'rgba(239,68,68,0.15)', color: '#f87171' }}>
                        已禁用
                      </span>
                    )}
                    {user.must_change_pwd && (
                      <span className="rounded px-1.5 py-0.5 text-[10px] font-semibold" style={{ background: 'rgba(234,179,8,0.12)', color: '#fbbf24' }}>
                        需改密
                      </span>
                    )}
                  </p>
                  <p className="text-xs text-surface-500 flex items-center gap-2 flex-wrap">
                    <span>{user.role === 'admin' ? '管理员' : '普通用户'}</span>
                    <span>·</span>
                    <span>注册于 {new Date(user.created_at).toLocaleDateString('zh-CN')}</span>
                    <span>·</span>
                    <span>最近登录 {formatLastLogin(user)}</span>
                    {user.last_login_ip && <>
                      <span>·</span>
                      <span title="最近登录 IP">{user.last_login_ip}</span>
                    </>}
                  </p>
                </div>
              </div>
              <div className="flex items-center gap-1">
                {/* 权限管理按钮（非管理员） */}
                {user.role !== 'admin' && (
                  <button
                    onClick={() => openPermEditor(user.id)}
                    className={clsx(
                      'btn-ghost gap-1 px-2.5 py-1.5 text-xs transition-all',
                      editingUser === user.id ? 'text-neon' : 'text-surface-400 hover:text-neon'
                    )}
                    title="权限设置"
                  >
                    <Shield size={14} />
                    权限
                  </button>
                )}
                {/* 重置密码 */}
                <button
                  onClick={() => { setResetPwdUser(user); setResetPwdValue(''); setResetForceChange(true) }}
                  className="btn-ghost gap-1 px-2.5 py-1.5 text-xs text-surface-400 hover:text-yellow-400 transition-all"
                  title="重置密码"
                >
                  <KeyRound size={14} />
                  密码
                </button>
                {/* 启用/禁用 */}
                {user.role !== 'admin' && (
                  <button
                    onClick={() => handleToggleDisabled(user)}
                    className={clsx(
                      'btn-ghost gap-1 px-2.5 py-1.5 text-xs transition-all',
                      user.disabled ? 'text-green-400 hover:text-green-300' : 'text-surface-400 hover:text-orange-400'
                    )}
                    title={user.disabled ? '启用账号' : '禁用账号'}
                  >
                    {user.disabled ? <RotateCcw size={14} /> : <Ban size={14} />}
                    {user.disabled ? '启用' : '禁用'}
                  </button>
                )}
                {user.role !== 'admin' && (
                  <button onClick={() => handleDeleteUser(user.id)} className="btn-ghost p-2 text-red-400 hover:text-red-300" title="删除用户">
                    <Trash2 size={16} />
                  </button>
                )}
              </div>
            </div>

            {/* 权限编辑面板 */}
            {editingUser === user.id && (
              <div className="animate-slide-up mx-2 mt-1 rounded-xl p-5 space-y-5" style={{ background: 'var(--bg-card)', border: '1px solid var(--border-hover)' }}>
                {loadingPerm ? (
                  <div className="flex items-center justify-center py-6">
                    <Loader2 size={20} className="animate-spin text-neon/40" />
                  </div>
                ) : (
                  <>
                    <h4 className="flex items-center gap-2 text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>
                      <Shield size={16} className="text-neon/60" />
                      {user.username} 的权限设置
                    </h4>

                    {/* 媒体库访问控制 */}
                    <div>
                      <label className="mb-2 flex items-center gap-1.5 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                        <FolderOpen size={12} />
                        可访问的媒体库
                        <span className="text-surface-500">（不选 = 全部可访问）</span>
                      </label>
                      <div className="flex flex-wrap gap-2">
                        {libraries.map((lib) => (
                          <button
                            key={lib.id}
                            onClick={() => toggleLibrary(lib.id)}
                            className={clsx(
                              'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                              permLibraries.includes(lib.id)
                                ? 'bg-neon-blue/15 text-neon border border-neon-blue/30'
                                : 'text-surface-400 hover:text-surface-300'
                            )}
                            style={!permLibraries.includes(lib.id) ? { border: '1px solid var(--border-default)' } : {}}
                          >
                            {lib.name}
                          </button>
                        ))}
                      </div>
                    </div>

                    {/* 内容分级限制 */}
                    <div>
                      <label className="mb-2 flex items-center gap-1.5 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                        <Eye size={12} />
                        最高可观看的内容分级
                      </label>
                      <div className="flex flex-wrap gap-2">
                        {RATING_OPTIONS.map((opt) => (
                          <button
                            key={opt.value}
                            onClick={() => setPermRating(opt.value)}
                            className={clsx(
                              'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                              permRating === opt.value
                                ? 'bg-neon-blue/15 text-neon border border-neon-blue/30'
                                : 'text-surface-400 hover:text-surface-300'
                            )}
                            style={permRating !== opt.value ? { border: '1px solid var(--border-default)' } : {}}
                          >
                            {opt.label}
                          </button>
                        ))}
                      </div>
                    </div>

                    {/* 每日观看时长限制 */}
                    <div>
                      <label className="mb-2 flex items-center gap-1.5 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                        <Clock size={12} />
                        每日观看时长限制
                        <span className="text-surface-500">(0 = 不限制，单位: 分钟)</span>
                      </label>
                      <input
                        type="number"
                        min={0}
                        max={1440}
                        value={permTimeLimit}
                        onChange={(e) => setPermTimeLimit(parseInt(e.target.value) || 0)}
                        className="input w-40"
                        placeholder="0"
                      />
                      {permTimeLimit > 0 && (
                        <span className="ml-2 text-xs text-surface-400">
                          = {Math.floor(permTimeLimit / 60)} 小时 {permTimeLimit % 60} 分钟/天
                        </span>
                      )}
                    </div>

                    <div className="flex items-center justify-end gap-2 pt-2" style={{ borderTop: '1px solid var(--border-default)' }}>
                      <button
                        onClick={() => setEditingUser(null)}
                        className="rounded-xl px-4 py-2 text-sm font-medium transition-all"
                        style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}
                      >
                        取消
                      </button>
                      <button onClick={savePerm} disabled={savingPerm} className="btn-primary gap-1.5 px-4 py-2 text-sm">
                        {savingPerm ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                        保存权限
                      </button>
                    </div>
                  </>
                )}
              </div>
            )}
          </div>
        ))}
      </div>

      <div className="flex items-start gap-2 rounded-xl p-3 text-xs text-yellow-400/80" style={{ background: 'rgba(234, 179, 8, 0.03)', border: '1px solid rgba(234, 179, 8, 0.08)' }}>
        <AlertCircle size={14} className="mt-0.5 flex-shrink-0" />
        <span>管理员可通过「新建用户」直接创建账号（默认要求首次登录强制改密）。也可在登录页让用户通过邀请码自行注册。禁用账号将立即吊销用户持有的所有登录凭证。</span>
      </div>

      {/* 创建用户弹窗 */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setShowCreateModal(false)}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-md mx-4 space-y-3" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between">
              <h3 className="text-sm font-semibold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                <UserPlus size={16} className="text-neon" />
                创建新用户
              </h3>
              <button onClick={() => setShowCreateModal(false)} className="text-surface-500 hover:text-surface-300">
                <X size={16} />
              </button>
            </div>
            <div className="space-y-3">
              <div>
                <label className="text-xs mb-1 block" style={{ color: 'var(--text-secondary)' }}>用户名 *</label>
                <input value={newUser.username} onChange={e => setNewUser({ ...newUser, username: e.target.value })} className="input w-full" placeholder="至少 3 位" />
              </div>
              <div>
                <label className="text-xs mb-1 block" style={{ color: 'var(--text-secondary)' }}>初始密码 *</label>
                <input type="password" value={newUser.password} onChange={e => setNewUser({ ...newUser, password: e.target.value })} className="input w-full" placeholder="至少 6 位，用户首次登录须修改" />
              </div>
              <div>
                <label className="text-xs mb-1 block" style={{ color: 'var(--text-secondary)' }}>角色</label>
                <div className="flex gap-2">
                  {(['user', 'admin'] as const).map(r => (
                    <button
                      key={r}
                      onClick={() => setNewUser({ ...newUser, role: r })}
                      className={clsx(
                        'rounded-lg px-3 py-1.5 text-xs font-medium transition-all',
                        newUser.role === r ? 'bg-neon-blue/15 text-neon border border-neon-blue/30' : 'text-surface-400'
                      )}
                      style={newUser.role !== r ? { border: '1px solid var(--border-default)' } : {}}
                    >
                      {r === 'admin' ? '管理员' : '普通用户'}
                    </button>
                  ))}
                </div>
              </div>
              <div className="grid grid-cols-2 gap-3">
                <div>
                  <label className="text-xs mb-1 block" style={{ color: 'var(--text-secondary)' }}>昵称（可选）</label>
                  <input value={newUser.nickname} onChange={e => setNewUser({ ...newUser, nickname: e.target.value })} className="input w-full" />
                </div>
                <div>
                  <label className="text-xs mb-1 block" style={{ color: 'var(--text-secondary)' }}>邮箱（可选）</label>
                  <input value={newUser.email} onChange={e => setNewUser({ ...newUser, email: e.target.value })} className="input w-full" />
                </div>
              </div>
            </div>
            <div className="flex items-center justify-end gap-2 pt-2">
              <button onClick={() => setShowCreateModal(false)} className="rounded-xl px-4 py-2 text-sm font-medium" style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}>取消</button>
              <button onClick={handleCreateUser} disabled={creatingUser} className="btn-primary gap-1.5 px-4 py-2 text-sm">
                {creatingUser ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                创建
              </button>
            </div>
          </div>
        </div>
      )}

      {/* 重置密码弹窗 */}
      {resetPwdUser && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60" onClick={() => setResetPwdUser(null)}>
          <div className="glass-panel-strong rounded-2xl p-6 w-full max-w-sm mx-4" onClick={e => e.stopPropagation()}>
            <div className="flex items-center justify-between mb-4">
              <h3 className="text-sm font-semibold flex items-center gap-2" style={{ color: 'var(--text-primary)' }}>
                <KeyRound size={16} className="text-yellow-400" />
                重置密码
              </h3>
              <button onClick={() => setResetPwdUser(null)} className="text-surface-500 hover:text-surface-300">
                <X size={16} />
              </button>
            </div>
            <p className="text-xs mb-3" style={{ color: 'var(--text-secondary)' }}>
              为用户 <strong style={{ color: 'var(--text-primary)' }}>{resetPwdUser.username}</strong> 设置新密码
            </p>
            <input
              type="password"
              value={resetPwdValue}
              onChange={e => setResetPwdValue(e.target.value)}
              className="input w-full mb-3"
              placeholder="输入新密码（至少6位）"
              minLength={6}
              autoFocus
            />
            <label className="flex items-center gap-2 mb-4 text-xs" style={{ color: 'var(--text-secondary)' }}>
              <input type="checkbox" checked={resetForceChange} onChange={e => setResetForceChange(e.target.checked)} />
              要求用户下次登录强制修改密码（推荐）
            </label>
            <div className="flex items-center justify-end gap-2">
              <button
                onClick={() => setResetPwdUser(null)}
                className="rounded-xl px-4 py-2 text-sm font-medium transition-all"
                style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={handleResetPassword}
                disabled={resettingPwd || resetPwdValue.length < 6}
                className="btn-primary gap-1.5 px-4 py-2 text-sm"
              >
                {resettingPwd ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                确认重置
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}