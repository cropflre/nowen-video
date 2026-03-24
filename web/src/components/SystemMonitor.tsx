import { useState, useEffect, useCallback } from 'react'
import { useWebSocket, WS_EVENTS } from '@/hooks/useWebSocket'
import { adminApi } from '@/api'
import type { SystemMetrics } from '@/types'
import { Cpu, HardDrive, MemoryStick, Activity, Wifi, Clapperboard } from 'lucide-react'

export default function SystemMonitor() {
  const [metrics, setMetrics] = useState<SystemMetrics | null>(null)
  const { on, off, connected } = useWebSocket()

  // 通过WebSocket接收实时指标
  const handleMetrics = useCallback((data: SystemMetrics) => {
    setMetrics(data)
  }, [])

  useEffect(() => {
    on('system_metrics' as any, handleMetrics)
    return () => off('system_metrics' as any, handleMetrics)
  }, [on, off, handleMetrics])

  // 初始加载
  useEffect(() => {
    adminApi.getMetrics().then((res) => setMetrics(res.data.data)).catch(() => {})
  }, [])

  if (!metrics) {
    return (
      <div className="space-y-3">
        <div className="skeleton h-24 rounded-xl" />
        <div className="skeleton h-24 rounded-xl" />
      </div>
    )
  }

  const ProgressBar = ({ value, color = 'primary' }: { value: number; color?: string }) => (
    <div className="h-1.5 w-full rounded-full" style={{ background: 'rgba(0,240,255,0.06)' }}>
      <div
        className="h-full rounded-full transition-all duration-500"
        style={{
          width: `${Math.min(value, 100)}%`,
          background: color === 'red' ? '#ef4444'
            : color === 'yellow' ? '#eab308'
            : 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
          boxShadow: color === 'green' ? '0 0 6px rgba(0,240,255,0.3)' : undefined,
        }}
      />
    </div>
  )

  const getColorByPercent = (v: number) => v > 90 ? 'red' : v > 70 ? 'yellow' : 'green'

  return (
    <div className="space-y-4">
      <h3 className="flex items-center gap-2 font-display text-base font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <Activity size={18} className="text-neon" />
        系统监控
        <span className={`ml-auto flex items-center gap-1 text-xs ${connected ? 'text-neon' : 'text-red-400'}`}>
          <Wifi size={12} />
          {connected ? '实时' : '离线'}
        </span>
      </h3>

      <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-4">
        {/* CPU */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="flex items-center gap-2 text-sm text-surface-400">
            <Cpu size={14} className="text-neon/60" />
            CPU
          </div>
          <p className="mt-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>{metrics.cpu.usage_percent.toFixed(1)}%</p>
          <ProgressBar value={metrics.cpu.usage_percent} color={getColorByPercent(metrics.cpu.usage_percent)} />
          <p className="mt-1 text-xs text-surface-500">
            {metrics.cpu.cores} 核心 · {metrics.cpu.goroutines} 协程
          </p>
        </div>

        {/* 内存 */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="flex items-center gap-2 text-sm text-surface-400">
            <MemoryStick size={14} className="text-neon/60" />
            内存
          </div>
          <p className="mt-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>{metrics.memory.used_percent.toFixed(1)}%</p>
          <ProgressBar value={metrics.memory.used_percent} color={getColorByPercent(metrics.memory.used_percent)} />
          <p className="mt-1 text-xs text-surface-500">
            {metrics.memory.used_mb} / {metrics.memory.total_mb} MB
            {metrics.memory.go_alloc_mb > 0 && ` · Go ${metrics.memory.go_alloc_mb} MB`}
          </p>
        </div>

        {/* 磁盘 */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="flex items-center gap-2 text-sm text-surface-400">
            <HardDrive size={14} className="text-neon/60" />
            磁盘
          </div>
          <p className="mt-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>{metrics.disk.used_percent.toFixed(1)}%</p>
          <ProgressBar value={metrics.disk.used_percent} color={getColorByPercent(metrics.disk.used_percent)} />
          <p className="mt-1 text-xs text-surface-500">
            {metrics.disk.used_gb.toFixed(1)} / {metrics.disk.total_gb.toFixed(1)} GB
            · 缓存 {metrics.disk.cache_size_mb.toFixed(0)} MB
          </p>
        </div>

        {/* 转码 */}
        <div className="glass-panel-subtle rounded-xl p-4">
          <div className="flex items-center gap-2 text-sm text-surface-400">
            <Clapperboard size={14} className="text-neon/60" />
            转码
          </div>
          <p className="mt-2 font-display text-2xl font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>{metrics.transcode.active_jobs}</p>
          <p className="mt-1 text-xs text-surface-500">活跃任务</p>
          <p className="mt-1 text-xs text-surface-500">
            加速: {metrics.transcode.hw_accel || 'none'}
            · 连接: {metrics.connections}
          </p>
        </div>
      </div>
    </div>
  )
}
