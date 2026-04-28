import { useEffect, useState, useCallback } from 'react'
import { adminApi } from '@/api'
import type { TidalStatus, TidalConfig, TidalMode } from '@/types'
import {
  Activity,
  Moon,
  Users,
  Play,
  Loader2,
  Save,
  Check,
  X,
  Gauge,
  PauseCircle,
  Zap,
} from 'lucide-react'

// 潮汐模式展示用元数据
const MODE_META: Record<TidalMode, {
  label: string
  desc: string
  color: string
  bgColor: string
  borderColor: string
  Icon: typeof Moon
}> = {
  idle: {
    label: '空闲模式',
    desc: '无人在线，全力预处理',
    color: '#10B981',
    bgColor: 'rgba(16, 185, 129, 0.10)',
    borderColor: 'rgba(16, 185, 129, 0.30)',
    Icon: Moon,
  },
  busy: {
    label: '在线模式',
    desc: '有用户在线，适度让路',
    color: '#F59E0B',
    bgColor: 'rgba(245, 158, 11, 0.10)',
    borderColor: 'rgba(245, 158, 11, 0.30)',
    Icon: Users,
  },
  playing: {
    label: '播放模式',
    desc: '有用户在播放，优先保障',
    color: '#EF4444',
    bgColor: 'rgba(239, 68, 68, 0.10)',
    borderColor: 'rgba(239, 68, 68, 0.30)',
    Icon: Play,
  },
}

// Toggle 开关（与 DashboardTab 保持一致）
function ToggleButton({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={onChange}
      className="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full transition-colors duration-300 focus:outline-none"
      style={{
        background: checked ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' : 'var(--border-default)',
        boxShadow: checked ? 'var(--neon-glow-shadow-md)' : 'none',
      }}
    >
      <span
        className="pointer-events-none inline-block h-5 w-5 rounded-full shadow-lg transition-transform duration-300"
        style={{
          transform: checked ? 'translateX(20px) translateY(2px)' : 'translateX(2px) translateY(2px)',
          background: checked ? '#fff' : 'var(--text-muted)',
        }}
      />
    </button>
  )
}

// 默认配置
const DEFAULT_CONFIG: TidalConfig = {
  enabled: true,
  idle_cpu_percent: 80,
  busy_cpu_percent: 30,
  playing_cpu_percent: 5,
  playing_action: 'pause',
  debounce_sec: 10,
}

/**
 * 潮汐调度面板（方案 A+B）
 * - 无人在线时：放开 ResourceLimit，预处理全力跑
 * - 有人在线但未播放：降低到 busy_cpu_percent
 * - 有人在播放：暂停正在运行的任务 或 降到 playing_cpu_percent
 */
export default function IdleSchedulerPanel() {
  const [status, setStatus] = useState<TidalStatus | null>(null)
  const [config, setConfig] = useState<TidalConfig>(DEFAULT_CONFIG)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [msg, setMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  // 拉取最新状态
  const fetchStatus = useCallback(async () => {
    try {
      const res = await adminApi.getIdleSchedulerStatus()
      const data = res.data.data
      setStatus(data)
      // 首次加载时把服务端配置同步到本地编辑态
      setConfig((prev) => (loading ? data.config : prev))
    } catch (e) {
      // 静默失败，保持上次状态
    } finally {
      setLoading(false)
    }
  }, [loading])

  // 首次加载 & 定时刷新
  useEffect(() => {
    fetchStatus()
    const timer = setInterval(fetchStatus, 5000)
    return () => clearInterval(timer)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  // 保存配置
  const handleSave = async () => {
    setSaving(true)
    setMsg(null)
    try {
      await adminApi.updateIdleSchedulerConfig(config)
      setMsg({ type: 'success', text: '潮汐调度配置已保存' })
      setTimeout(() => setMsg(null), 4000)
      fetchStatus()
    } catch (e: any) {
      const text = e?.response?.data?.error || '保存失败，请稍后重试'
      setMsg({ type: 'error', text })
    } finally {
      setSaving(false)
    }
  }

  const currentMode: TidalMode = status?.current_mode || 'idle'
  const meta = MODE_META[currentMode]

  return (
    <section>
      <h2
        className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide"
        style={{ color: 'var(--text-primary)' }}
      >
        <Activity size={20} className="text-neon/60" />
        潮汐调度
        <span
          className="ml-2 rounded-full px-2 py-0.5 text-xs font-normal"
          style={{
            background: 'var(--neon-blue-6)',
            color: 'var(--neon-blue)',
            border: '1px solid var(--neon-blue-10)',
          }}
        >
          Beta
        </span>
      </h2>

      <div className="glass-panel rounded-xl p-5 space-y-5">
        {/* 功能说明 */}
        <div
          className="rounded-lg p-3 text-xs leading-relaxed"
          style={{
            background: 'var(--nav-hover-bg)',
            border: '1px solid var(--border-default)',
            color: 'var(--text-tertiary)',
          }}
        >
          根据系统活跃状态自动调节预处理资源占用：<strong style={{ color: 'var(--text-secondary)' }}>无人在线时全力预处理</strong>，
          有用户在线时<strong style={{ color: 'var(--text-secondary)' }}>适度让路</strong>，
          有用户在播放时<strong style={{ color: 'var(--text-secondary)' }}>暂停或限流</strong>以保障播放流畅。
        </div>

        {/* 当前状态卡片 */}
        {loading ? (
          <div className="flex items-center justify-center py-8">
            <Loader2 size={20} className="animate-spin text-neon/60" />
          </div>
        ) : status ? (
          <>
            {/* 模式卡 + 活跃指标 */}
            <div className="grid grid-cols-1 gap-3 lg:grid-cols-3">
              {/* 当前模式 */}
              <div
                className="flex items-center gap-3 rounded-xl p-4"
                style={{
                  background: meta.bgColor,
                  border: `1px solid ${meta.borderColor}`,
                }}
              >
                <div
                  className="flex h-11 w-11 items-center justify-center rounded-lg"
                  style={{ background: meta.color + '22', border: `1px solid ${meta.color}55` }}
                >
                  <meta.Icon size={20} style={{ color: meta.color }} />
                </div>
                <div className="flex-1 min-w-0">
                  <div className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
                    当前模式
                  </div>
                  <div className="font-semibold" style={{ color: meta.color }}>
                    {meta.label}
                  </div>
                  <div className="truncate text-xs" style={{ color: 'var(--text-tertiary)' }}>
                    {meta.desc}
                  </div>
                </div>
              </div>

              {/* 在线用户 */}
              <Metric
                icon={<Users size={18} className="text-neon/60" />}
                label="在线用户"
                value={status.online_users}
                unit="人"
              />

              {/* 正在播放 */}
              <Metric
                icon={<Play size={18} className="text-neon/60" />}
                label="正在播放"
                value={status.playing_jobs}
                unit="个任务"
              />
            </div>

            {/* 详细参数 */}
            <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
              <MiniMetric
                label="有效 CPU 上限"
                value={`${status.preprocess?.effective_resource_limit ?? '-'}%`}
                highlight
              />
              <MiniMetric
                label="活跃 Workers"
                value={`${status.preprocess?.active_workers ?? '-'}/${status.preprocess?.cur_workers ?? '-'}`}
              />
              <MiniMetric
                label="队列任务"
                value={`${status.preprocess?.queue_size ?? 0}`}
              />
              <MiniMetric
                label="潮汐暂停"
                value={`${status.paused_task_cnt}`}
                unit="个"
              />
            </div>

            {/* 防抖提示 */}
            {status.pending_mode && status.pending_mode !== status.current_mode && (
              <div
                className="flex items-center gap-2 rounded-lg px-3 py-2 text-xs"
                style={{
                  background: 'rgba(59, 130, 246, 0.08)',
                  color: '#3B82F6',
                  border: '1px solid rgba(59, 130, 246, 0.2)',
                }}
              >
                <Loader2 size={14} className="animate-spin" />
                即将切换到{' '}
                <strong>{MODE_META[status.pending_mode as TidalMode]?.label || status.pending_mode}</strong>
                （已稳定 {(status.pending_for_ms / 1000).toFixed(1)}s / 需要 {config.debounce_sec}s）
              </div>
            )}
          </>
        ) : (
          <div className="py-6 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
            状态加载失败
          </div>
        )}

        {/* 分隔线 */}
        <div style={{ borderTop: '1px solid var(--border-default)' }} />

        {/* 配置区 */}
        <div className="space-y-4">
          {/* 总开关 */}
          <div className="flex items-center justify-between">
            <div>
              <div className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                启用潮汐调度
              </div>
              <div className="mt-0.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                关闭后将使用全局配置的资源限制，不再自动让路
              </div>
            </div>
            <ToggleButton
              checked={config.enabled}
              onChange={() => setConfig((c) => ({ ...c, enabled: !c.enabled }))}
            />
          </div>

          {config.enabled && (
            <>
              {/* 三档 CPU 上限 */}
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
                <CpuSlider
                  label="空闲时 CPU 上限"
                  hint="无人在线时，预处理最高可用"
                  icon={<Moon size={14} style={{ color: '#10B981' }} />}
                  value={config.idle_cpu_percent}
                  onChange={(v) => setConfig((c) => ({ ...c, idle_cpu_percent: v }))}
                  color="#10B981"
                />
                <CpuSlider
                  label="在线时 CPU 上限"
                  hint="有用户但未播放时，保持一定响应"
                  icon={<Users size={14} style={{ color: '#F59E0B' }} />}
                  value={config.busy_cpu_percent}
                  onChange={(v) => setConfig((c) => ({ ...c, busy_cpu_percent: v }))}
                  color="#F59E0B"
                />
                <CpuSlider
                  label="播放时 CPU 上限"
                  hint="限流模式下使用（暂停模式忽略此值）"
                  icon={<Zap size={14} style={{ color: '#EF4444' }} />}
                  value={config.playing_cpu_percent}
                  onChange={(v) => setConfig((c) => ({ ...c, playing_cpu_percent: v }))}
                  color="#EF4444"
                  disabled={config.playing_action === 'pause'}
                />
              </div>

              {/* 播放策略 */}
              <div>
                <div className="mb-2 text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                  播放时策略
                </div>
                <div className="grid grid-cols-2 gap-2">
                  <ActionOption
                    active={config.playing_action === 'pause'}
                    onClick={() => setConfig((c) => ({ ...c, playing_action: 'pause' }))}
                    icon={<PauseCircle size={16} />}
                    title="暂停任务（推荐）"
                    desc="立即暂停正在运行的 FFmpeg，播放结束后自动恢复"
                  />
                  <ActionOption
                    active={config.playing_action === 'throttle'}
                    onClick={() => setConfig((c) => ({ ...c, playing_action: 'throttle' }))}
                    icon={<Gauge size={16} />}
                    title="限流继续"
                    desc="降低到播放时 CPU 上限，低优先级继续跑"
                  />
                </div>
              </div>

              {/* 防抖时间 */}
              <div className="flex items-center justify-between gap-4">
                <div className="flex-1">
                  <div className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    模式切换防抖（秒）
                  </div>
                  <div className="mt-0.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                    目标模式需连续稳定多少秒才真正切换，避免频繁抖动
                  </div>
                </div>
                <input
                  type="number"
                  min={1}
                  max={300}
                  value={config.debounce_sec}
                  onChange={(e) =>
                    setConfig((c) => ({
                      ...c,
                      debounce_sec: Math.max(1, Math.min(300, Number(e.target.value) || 10)),
                    }))
                  }
                  className="input w-24 text-center"
                />
              </div>
            </>
          )}

          {/* 保存按钮和消息 */}
          <div className="flex items-center justify-end gap-3 pt-1">
            {msg && (
              <div
                className="flex items-center gap-1.5 rounded-lg px-3 py-1.5 text-xs"
                style={{
                  background: msg.type === 'success' ? 'rgba(22, 163, 74, 0.08)' : 'rgba(220, 38, 38, 0.08)',
                  color: msg.type === 'success' ? '#16A34A' : '#DC2626',
                }}
              >
                {msg.type === 'success' ? <Check size={14} /> : <X size={14} />}
                {msg.text}
              </div>
            )}
            <button
              onClick={handleSave}
              disabled={saving}
              className="btn-primary gap-1.5 px-5 py-2.5 text-sm"
            >
              {saving ? (
                <>
                  <Loader2 size={14} className="animate-spin" />
                  保存中...
                </>
              ) : (
                <>
                  <Save size={14} />
                  保存配置
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    </section>
  )
}

// —— 小组件 —— //

function Metric({
  icon,
  label,
  value,
  unit,
}: {
  icon: React.ReactNode
  label: string
  value: number
  unit?: string
}) {
  return (
    <div
      className="flex items-center gap-3 rounded-xl p-4"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
      }}
    >
      <div
        className="flex h-11 w-11 items-center justify-center rounded-lg"
        style={{ background: 'var(--neon-blue-6)', border: '1px solid var(--neon-blue-10)' }}
      >
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <div className="text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {label}
        </div>
        <div className="flex items-baseline gap-1">
          <span className="text-xl font-semibold" style={{ color: 'var(--text-primary)' }}>
            {value}
          </span>
          {unit && (
            <span className="text-xs" style={{ color: 'var(--text-muted)' }}>
              {unit}
            </span>
          )}
        </div>
      </div>
    </div>
  )
}

function MiniMetric({
  label,
  value,
  unit,
  highlight,
}: {
  label: string
  value: string
  unit?: string
  highlight?: boolean
}) {
  return (
    <div
      className="rounded-lg p-3"
      style={{
        background: highlight ? 'var(--neon-blue-6)' : 'var(--bg-elevated)',
        border: highlight ? '1px solid var(--neon-blue-10)' : '1px solid var(--border-default)',
      }}
    >
      <div className="text-[11px]" style={{ color: 'var(--text-tertiary)' }}>
        {label}
      </div>
      <div className="mt-1 flex items-baseline gap-1">
        <span
          className="text-base font-semibold"
          style={{ color: highlight ? 'var(--neon-blue)' : 'var(--text-primary)' }}
        >
          {value}
        </span>
        {unit && (
          <span className="text-[11px]" style={{ color: 'var(--text-muted)' }}>
            {unit}
          </span>
        )}
      </div>
    </div>
  )
}

function CpuSlider({
  label,
  hint,
  icon,
  value,
  onChange,
  color,
  disabled,
}: {
  label: string
  hint?: string
  icon?: React.ReactNode
  value: number
  onChange: (v: number) => void
  color: string
  disabled?: boolean
}) {
  return (
    <div
      className="rounded-lg p-3"
      style={{
        background: 'var(--bg-elevated)',
        border: '1px solid var(--border-default)',
        opacity: disabled ? 0.5 : 1,
      }}
    >
      <div className="flex items-center justify-between gap-2">
        <div className="flex items-center gap-1.5 text-xs font-medium" style={{ color: 'var(--text-secondary)' }}>
          {icon}
          {label}
        </div>
        <span className="text-sm font-semibold tabular-nums" style={{ color }}>
          {value}%
        </span>
      </div>
      {hint && (
        <div className="mt-1 text-[11px]" style={{ color: 'var(--text-tertiary)' }}>
          {hint}
        </div>
      )}
      <input
        type="range"
        min={1}
        max={100}
        step={1}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(Number(e.target.value))}
        className="mt-2 w-full cursor-pointer accent-current"
        style={{ accentColor: color }}
      />
    </div>
  )
}

function ActionOption({
  active,
  onClick,
  icon,
  title,
  desc,
}: {
  active: boolean
  onClick: () => void
  icon: React.ReactNode
  title: string
  desc: string
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className="flex items-start gap-3 rounded-lg p-3 text-left transition-all duration-200"
      style={{
        background: active ? 'var(--neon-blue-6)' : 'var(--bg-elevated)',
        border: active ? '1px solid var(--neon-blue)' : '1px solid var(--border-default)',
        boxShadow: active ? 'var(--neon-glow-shadow-sm)' : 'none',
      }}
    >
      <div
        className="mt-0.5 flex h-8 w-8 flex-shrink-0 items-center justify-center rounded-lg"
        style={{
          background: active ? 'var(--neon-blue)' : 'var(--border-default)',
          color: active ? '#fff' : 'var(--text-tertiary)',
        }}
      >
        {icon}
      </div>
      <div className="flex-1 min-w-0">
        <div
          className="text-sm font-medium"
          style={{ color: active ? 'var(--neon-blue)' : 'var(--text-primary)' }}
        >
          {title}
        </div>
        <div className="mt-0.5 text-xs" style={{ color: 'var(--text-tertiary)' }}>
          {desc}
        </div>
      </div>
    </button>
  )
}
