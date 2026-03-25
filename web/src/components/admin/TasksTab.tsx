import { useState, useEffect, useCallback } from 'react'
import type { TranscodeJob, ScheduledTask, Library } from '@/types'
import type { TranscodeProgressData } from '@/hooks/useWebSocket'
import { adminApi, libraryApi } from '@/api'
import { useToast } from '@/components/Toast'
import {
  Zap,
  Loader2,
  ListTodo,
  Plus,
  Trash2,
  Play,
  Pause,
  Clock,
  AlertCircle,
  Check,
  RefreshCw,
} from 'lucide-react'
import clsx from 'clsx'

// 任务类型标签
const TASK_TYPE_LABELS: Record<string, string> = {
  scan: '📂 扫描',
  scrape: '🎨 刮削',
  cleanup: '🧹 清理',
}

// 调度表达式预设
const SCHEDULE_PRESETS = [
  { label: '每小时', value: '@every 1h' },
  { label: '每6小时', value: '@every 6h' },
  { label: '每天', value: '@daily' },
  { label: '每周', value: '@weekly' },
  { label: '每12小时', value: '@every 12h' },
]

interface TasksTabProps {
  transcodeJobs: TranscodeJob[]
  transcodeProgress: Record<string, TranscodeProgressData>
}

export default function TasksTab({ transcodeJobs, transcodeProgress }: TasksTabProps) {
  const toast = useToast()
  const [tasks, setTasks] = useState<ScheduledTask[]>([])
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loadingTasks, setLoadingTasks] = useState(true)
  const [showCreateForm, setShowCreateForm] = useState(false)

  // 创建表单状态
  const [formName, setFormName] = useState('')
  const [formType, setFormType] = useState<'scan' | 'scrape' | 'cleanup'>('scan')
  const [formSchedule, setFormSchedule] = useState('@daily')
  const [formTarget, setFormTarget] = useState('')
  const [creating, setCreating] = useState(false)

  // 加载定时任务
  const fetchTasks = useCallback(async () => {
    try {
      const res = await adminApi.listTasks()
      setTasks(res.data.data || [])
    } catch {
      toast.error('加载定时任务失败')
    } finally {
      setLoadingTasks(false)
    }
  }, [])

  // 加载媒体库列表（用于任务目标选择）
  useEffect(() => {
    fetchTasks()
    libraryApi.list().then((res) => setLibraries(res.data.data || [])).catch(() => {})
  }, [fetchTasks])

  // 创建任务
  const handleCreate = async () => {
    if (!formName.trim()) { toast.error('请输入任务名称'); return }
    setCreating(true)
    try {
      await adminApi.createTask({
        name: formName.trim(),
        type: formType,
        schedule: formSchedule,
        target_id: formTarget || undefined,
      })
      toast.success('定时任务已创建')
      setShowCreateForm(false)
      setFormName('')
      setFormTarget('')
      fetchTasks()
    } catch {
      toast.error('创建失败')
    } finally {
      setCreating(false)
    }
  }

  // 切换启用/禁用
  const handleToggle = async (task: ScheduledTask) => {
    try {
      await adminApi.updateTask(task.id, {
        name: task.name,
        schedule: task.schedule,
        enabled: !task.enabled,
      })
      setTasks((prev) => prev.map((t) => t.id === task.id ? { ...t, enabled: !t.enabled } : t))
      toast.success(task.enabled ? '已暂停' : '已启用')
    } catch {
      toast.error('操作失败')
    }
  }

  // 立即运行
  const handleRunNow = async (id: string) => {
    try {
      await adminApi.runTaskNow(id)
      toast.success('任务已触发执行')
      setTimeout(fetchTasks, 2000)
    } catch {
      toast.error('触发失败，任务可能正在运行')
    }
  }

  // 删除任务
  const handleDelete = async (id: string) => {
    if (!confirm('确定删除该定时任务？')) return
    try {
      await adminApi.deleteTask(id)
      setTasks((prev) => prev.filter((t) => t.id !== id))
      toast.success('已删除')
    } catch {
      toast.error('删除失败')
    }
  }

  const formatNextRun = (dateStr: string | null) => {
    if (!dateStr) return '—'
    const d = new Date(dateStr)
    const now = new Date()
    const diffMs = d.getTime() - now.getTime()
    if (diffMs < 0) return '即将执行'
    const diffMin = Math.floor(diffMs / 60000)
    if (diffMin < 60) return `${diffMin} 分钟后`
    const diffH = Math.floor(diffMin / 60)
    if (diffH < 24) return `${diffH} 小时后`
    return d.toLocaleString('zh-CN', { month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  return (
    <div className="space-y-8">
      {/* 实时转码进度 */}
      {Object.keys(transcodeProgress).length > 0 && (
        <section>
          <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <Loader2 size={20} className="animate-spin text-neon" />
            转码进行中
          </h2>
          <div className="space-y-3">
            {Object.entries(transcodeProgress).map(([taskId, data]) => (
              <div key={`transcode-${taskId}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'rgba(245,158,11,0.15)' }}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>🎥 {data.title} ({data.quality})</span>
                  <span className="text-xs text-amber-400">{data.progress.toFixed(1)}% {data.speed && `| ${data.speed}`}</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full" style={{ background: 'var(--neon-blue-6)' }}>
                  <div className="h-full rounded-full bg-amber-500 transition-all duration-300" style={{ width: `${data.progress}%` }} />
                </div>
              </div>
            ))}
          </div>
        </section>
      )}

      {/* 转码任务列表 */}
      <section>
        <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Zap size={20} className="text-neon/60" />
          转码任务
        </h2>
        {transcodeJobs.length > 0 ? (
          <div className="space-y-2">
            {transcodeJobs.map((job) => (
              <div key={job.id} className="glass-panel-subtle flex items-center justify-between rounded-xl p-3">
                <div className="text-sm" style={{ color: 'var(--text-primary)' }}>
                  <span className="text-surface-400">媒体ID:</span> {job.media_id.slice(0, 8)}...
                  <span className="ml-3 text-surface-400">质量:</span> {job.quality}
                </div>
                <div className="flex items-center gap-3">
                  <div className="h-2 w-32 overflow-hidden rounded-full" style={{ background: 'var(--neon-blue-6)' }}>
                    <div className="h-full rounded-full transition-all" style={{ width: `${job.progress}%`, background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))' }} />
                  </div>
                  <span className={clsx('text-xs font-medium', job.status === 'running' && 'text-neon', job.status === 'pending' && 'text-yellow-400', job.status === 'done' && 'text-green-400', job.status === 'failed' && 'text-red-400')}>
                    {job.status}
                  </span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-12 text-center">
            <div>
              <Zap size={32} className="mx-auto mb-2 text-surface-600" />
              <p className="text-sm text-surface-500">暂无转码任务</p>
            </div>
          </div>
        )}
      </section>

      {/* 定时任务管理 */}
      <section>
        <div className="mb-4 flex items-center justify-between">
          <h2 className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <ListTodo size={20} className="text-neon/60" />
            定时任务
          </h2>
          <div className="flex items-center gap-2">
            <button onClick={fetchTasks} className="btn-ghost p-2 text-surface-400 hover:text-neon" title="刷新">
              <RefreshCw size={16} />
            </button>
            <button
              onClick={() => setShowCreateForm(!showCreateForm)}
              className="btn-primary gap-1.5 px-3.5 py-2 text-xs"
            >
              <Plus size={14} />
              新建任务
            </button>
          </div>
        </div>

        {/* 创建表单 */}
        {showCreateForm && (
          <div className="glass-panel mb-4 animate-slide-up rounded-xl p-5 space-y-4">
            <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>创建定时任务</h3>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>任务名称</label>
                <input
                  type="text"
                  value={formName}
                  onChange={(e) => setFormName(e.target.value)}
                  className="input w-full"
                  placeholder="如：每日自动扫描"
                />
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>任务类型</label>
                <select
                  value={formType}
                  onChange={(e) => setFormType(e.target.value as 'scan' | 'scrape' | 'cleanup')}
                  className="input w-full"
                >
                  <option value="scan">扫描媒体库</option>
                  <option value="scrape">刮削元数据</option>
                  <option value="cleanup">清理缓存</option>
                </select>
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>执行频率</label>
                <select
                  value={formSchedule}
                  onChange={(e) => setFormSchedule(e.target.value)}
                  className="input w-full"
                >
                  {SCHEDULE_PRESETS.map((p) => (
                    <option key={p.value} value={p.value}>{p.label}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
                  目标媒体库 <span className="text-surface-500">(可选，空 = 全部)</span>
                </label>
                <select
                  value={formTarget}
                  onChange={(e) => setFormTarget(e.target.value)}
                  className="input w-full"
                >
                  <option value="">全部媒体库</option>
                  {libraries.map((lib) => (
                    <option key={lib.id} value={lib.id}>{lib.name}</option>
                  ))}
                </select>
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 pt-2">
              <button
                onClick={() => setShowCreateForm(false)}
                className="rounded-xl px-4 py-2 text-sm font-medium transition-all"
                style={{ color: 'var(--text-secondary)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button onClick={handleCreate} disabled={creating} className="btn-primary gap-1.5 px-4 py-2 text-sm">
                {creating ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
                创建
              </button>
            </div>
          </div>
        )}

        {/* 任务列表 */}
        {loadingTasks ? (
          <div className="flex items-center justify-center py-12">
            <Loader2 size={24} className="animate-spin text-neon/40" />
          </div>
        ) : tasks.length > 0 ? (
          <div className="space-y-2">
            {tasks.map((task) => (
              <div key={task.id} className="glass-panel-subtle group flex items-center gap-4 rounded-xl p-4 transition-all hover:border-neon-blue/20">
                {/* 状态指示 */}
                <div className={clsx(
                  'flex h-9 w-9 flex-shrink-0 items-center justify-center rounded-lg text-sm',
                  task.status === 'running' && 'bg-neon-blue/10 text-neon',
                  task.status === 'error' && 'bg-red-500/10 text-red-400',
                  task.status === 'idle' && task.enabled && 'bg-green-500/10 text-green-400',
                  task.status === 'idle' && !task.enabled && 'bg-surface-800 text-surface-500',
                )}>
                  {task.status === 'running' ? <Loader2 size={16} className="animate-spin" /> : <Clock size={16} />}
                </div>

                {/* 信息 */}
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{task.name}</span>
                    <span className="rounded-md px-1.5 py-0.5 text-[10px] font-medium" style={{
                      background: 'var(--nav-hover-bg)',
                      color: 'var(--text-tertiary)',
                    }}>
                      {TASK_TYPE_LABELS[task.type] || task.type}
                    </span>
                    {!task.enabled && (
                      <span className="rounded-md bg-surface-800 px-1.5 py-0.5 text-[10px] text-surface-500">已暂停</span>
                    )}
                  </div>
                  <div className="mt-0.5 flex items-center gap-3 text-xs" style={{ color: 'var(--text-muted)' }}>
                    <span>频率: {task.schedule}</span>
                    <span>下次: {formatNextRun(task.next_run)}</span>
                    {task.last_error && (
                      <span className="flex items-center gap-1 text-red-400">
                        <AlertCircle size={10} /> {task.last_error.slice(0, 30)}
                      </span>
                    )}
                  </div>
                </div>

                {/* 操作 */}
                <div className="flex items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100">
                  <button onClick={() => handleRunNow(task.id)} className="rounded-lg p-1.5 text-surface-400 hover:text-neon hover:bg-neon-blue/5" title="立即执行">
                    <Play size={14} />
                  </button>
                  <button onClick={() => handleToggle(task)} className="rounded-lg p-1.5 text-surface-400 hover:text-yellow-400 hover:bg-yellow-400/5" title={task.enabled ? '暂停' : '启用'}>
                    {task.enabled ? <Pause size={14} /> : <Play size={14} />}
                  </button>
                  <button onClick={() => handleDelete(task.id)} className="rounded-lg p-1.5 text-surface-400 hover:text-red-400 hover:bg-red-400/5" title="删除">
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-12 text-center">
            <div>
              <ListTodo size={32} className="mx-auto mb-2 text-surface-600" />
              <p className="text-sm text-surface-500">暂无定时任务</p>
              <p className="mt-1 text-xs text-surface-600">点击「新建任务」添加自动扫描、刮削或缓存清理任务</p>
            </div>
          </div>
        )}
      </section>
    </div>
  )
}
