import { useState, useEffect } from 'react'
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

  // 权限编辑表单
  const [permLibraries, setPermLibraries] = useState<string[]>([])
  const [permRating, setPermRating] = useState('NC-17')
  const [permTimeLimit, setPermTimeLimit] = useState(0)

  useEffect(() => {
    libraryApi.list().then((res) => setLibraries(res.data.data || [])).catch(() => {})
  }, [])

  const handleDeleteUser = async (id: string) => {
    if (!confirm('确定删除此用户？')) return
    try {
      await adminApi.deleteUser(id)
      setUsers((u) => u.filter((user) => user.id !== id))
      toast.success('用户已删除')
    } catch {
      toast.error('删除用户失败')
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
      // 无权限记录，使用默认值
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

  // 切换媒体库访问权限
  const toggleLibrary = (libId: string) => {
    setPermLibraries((prev) =>
      prev.includes(libId) ? prev.filter((id) => id !== libId) : [...prev, libId]
    )
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <h2 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Users size={20} className="text-neon/60" />
          用户管理
        </h2>
        <span className="text-sm text-surface-400">共 {users.length} 个用户</span>
      </div>

      <div className="space-y-2">
        {users.map((user) => (
          <div key={user.id}>
            <div className="glass-panel-subtle flex items-center justify-between rounded-xl p-4 transition-all hover:border-neon-blue/20">
              <div className="flex items-center gap-3">
                <div className="flex h-10 w-10 items-center justify-center rounded-full text-sm font-bold" style={{ background: 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))', boxShadow: 'var(--shadow-neon)', color: 'var(--text-on-neon)' }}>
                  {user.username.charAt(0).toUpperCase()}
                </div>
                <div>
                  <p className="font-medium" style={{ color: 'var(--text-primary)' }}>{user.username}</p>
                  <p className="text-xs text-surface-500">
                    {user.role === 'admin' ? '管理员' : '普通用户'}
                    <span className="ml-2">注册于 {new Date(user.created_at).toLocaleDateString('zh-CN')}</span>
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

                    {/* 操作按钮 */}
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
        <span>新用户可以通过登录页面的"创建账号"自行注册。第一个注册的用户将自动成为管理员。点击「权限」按钮可配置用户的媒体库访问、内容分级和观看时长限制。</span>
      </div>
    </div>
  )
}
