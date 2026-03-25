import { useState } from 'react'
import type { SystemInfo, SystemSettings } from '@/types'
import type { ScanProgressData, ScrapeProgressData, TranscodeProgressData } from '@/hooks/useWebSocket'
import {
  Server,
  Cpu,
  HardDrive,
  Zap,
  Loader2,
  Check,
  X,
  Settings,
  Activity,
  FolderOpen,
  Users,
  ListTodo,
  FolderCog,
  Link,
  Save,
} from 'lucide-react'
import clsx from 'clsx'
import { adminApi } from '@/api'

// 标签页快捷入口定义
const SHORTCUT_TABS = [
  { id: 'library', label: '媒体库管理', icon: FolderOpen },
  { id: 'users', label: '用户管理', icon: Users },
  { id: 'tasks', label: '任务与转码', icon: ListTodo },
  { id: 'monitor', label: '监控与日志', icon: Activity },
] as const

interface DashboardTabProps {
  systemInfo: SystemInfo | null
  sysSettings: SystemSettings
  setSysSettings: React.Dispatch<React.SetStateAction<SystemSettings>>
  scanProgress: Record<string, ScanProgressData>
  scrapeProgress: Record<string, ScrapeProgressData>
  transcodeProgress: Record<string, TranscodeProgressData>
  realtimeMessages: string[]
  switchTab: (tab: string) => void
}

export default function DashboardTab({
  systemInfo,
  sysSettings,
  setSysSettings,
  scanProgress,
  scrapeProgress,
  transcodeProgress,
  realtimeMessages,
  switchTab,
}: DashboardTabProps) {
  const [sysSettingsSaving, setSysSettingsSaving] = useState(false)
  const [sysSettingsMsg, setSysSettingsMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const hasActiveProgress = Object.keys(scanProgress).length > 0 || Object.keys(scrapeProgress).length > 0 || Object.keys(transcodeProgress).length > 0

  const hwAccelLabel = (hw: string) => {
    switch (hw) {
      case 'qsv': return 'Intel QSV'
      case 'vaapi': return 'VAAPI'
      case 'nvenc': return 'NVIDIA NVENC'
      case 'none': return '软件编码'
      default: return hw
    }
  }

  const handleSaveSettings = async () => {
    setSysSettingsSaving(true)
    setSysSettingsMsg(null)
    try {
      await adminApi.updateSystemSettings(sysSettings)
      setSysSettingsMsg({ type: 'success', text: '系统设置已保存' })
      setTimeout(() => setSysSettingsMsg(null), 4000)
    } catch {
      setSysSettingsMsg({ type: 'error', text: '保存失败，请稍后重试' })
    } finally {
      setSysSettingsSaving(false)
    }
  }

  return (
    <div className="space-y-8">
      {/* 实时进度面板 */}
      {hasActiveProgress && (
        <section className="animate-slide-up">
          <h2 className="mb-4 flex items-center gap-2 text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
            <Loader2 size={20} className="animate-spin text-neon" />
            实时进度
          </h2>
          <div className="space-y-3">
            {Object.entries(scanProgress).map(([libId, data]) => (
              <div key={`scan-${libId}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'var(--neon-blue-15)' }}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>📂 扫描: {data.library_name}</span>
                  <span className="text-xs text-neon">新增 {data.new_found} 个文件</span>
                </div>
                <p className="text-xs text-surface-400">{data.message}</p>
              </div>
            ))}
            {Object.entries(scrapeProgress).map(([key, data]) => (
              <div key={`scrape-${key}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'var(--neon-purple-15)' }}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>🎨 元数据刮削</span>
                  <span className="text-xs text-purple-400">{data.current}/{data.total} (成功:{data.success} 失败:{data.failed})</span>
                </div>
                <div className="mb-2 h-2 overflow-hidden rounded-full" style={{ background: 'var(--neon-blue-6)' }}>
                  <div className="h-full rounded-full transition-all duration-300" style={{ background: 'linear-gradient(90deg, var(--neon-purple), var(--neon-pink))', width: `${data.total > 0 ? (data.current / data.total) * 100 : 0}%` }} />
                </div>
                <p className="text-xs text-surface-400">{data.message}</p>
              </div>
            ))}
            {Object.entries(transcodeProgress).map(([taskId, data]) => (
              <div key={`transcode-${taskId}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'rgba(245,158,11,0.15)' }}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>🎥 转码: {data.title} ({data.quality})</span>
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

      {/* 系统信息 */}
      {systemInfo && (
        <section>
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Server size={20} className="text-neon/60" />
            系统状态
          </h2>
          <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <Cpu size={16} className="text-neon/60" />
                <span className="text-xs">CPU / 协程</span>
              </div>
              <p className="mt-2 font-display text-lg font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                {systemInfo.cpus} 核 / {systemInfo.goroutines}
              </p>
            </div>
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <HardDrive size={16} className="text-neon/60" />
                <span className="text-xs">内存使用</span>
              </div>
              <p className="mt-2 font-display text-lg font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>
                {systemInfo.memory.alloc_mb} MB
              </p>
            </div>
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <Zap size={16} className="text-neon/60" />
                <span className="text-xs">硬件加速</span>
              </div>
              <p className={clsx('mt-2 text-lg font-bold', systemInfo.hw_accel !== 'none' ? 'text-green-400' : 'text-yellow-400')}>
                {hwAccelLabel(systemInfo.hw_accel)}
              </p>
            </div>
            <div className="glass-panel-subtle rounded-xl p-4">
              <div className="flex items-center gap-2 text-surface-400">
                <Server size={16} className="text-neon/60" />
                <span className="text-xs">版本</span>
              </div>
              <p className="mt-2 font-display text-lg font-bold tracking-wide" style={{ color: 'var(--text-primary)' }}>v{systemInfo.version}</p>
              <p className="text-xs text-surface-500">{systemInfo.go_version} / {systemInfo.os}_{systemInfo.arch}</p>
            </div>
          </div>
        </section>
      )}

      {/* 系统设置 */}
      <section>
        <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Settings size={20} className="text-neon/60" />
          系统设置
        </h2>
        <div className="glass-panel rounded-xl p-5 space-y-6">
          <div className="rounded-lg p-3 text-xs" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)', color: 'var(--text-tertiary)' }}>
            以下设置为系统全局配置，对所有媒体库统一生效。媒体库的独立设置请在「媒体库管理」标签页中配置。
          </div>

          {/* GPU 加速转码 */}
          <div>
            <div className="flex items-start justify-between gap-4">
              <div className="flex-1">
                <div className="flex items-center gap-2">
                  <Zap size={16} style={{ color: 'var(--neon-blue)' }} />
                  <h4 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>GPU 加速转码</h4>
                </div>
                <p className="mt-1 text-xs leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>启用 GPU 硬件加速转码，显著提升转码速度。</p>
              </div>
              <ToggleButton checked={sysSettings.enable_gpu_transcode} onChange={() => setSysSettings((s) => ({ ...s, enable_gpu_transcode: !s.enable_gpu_transcode }))} />
            </div>
            {sysSettings.enable_gpu_transcode && (
              <div className="mt-3 ml-6 flex items-start justify-between gap-4 rounded-lg p-3" style={{ background: 'var(--nav-hover-bg)' }}>
                <div className="flex-1">
                  <h4 className="text-xs font-semibold" style={{ color: 'var(--text-secondary)' }}>GPU 不支持时自动回退 CPU</h4>
                  <p className="mt-0.5 text-[11px] leading-relaxed" style={{ color: 'var(--text-muted)' }}>当 GPU 不支持特定格式解码时，系统自动切换至 CPU 转码。</p>
                </div>
                <ToggleButton checked={sysSettings.gpu_fallback_cpu} onChange={() => setSysSettings((s) => ({ ...s, gpu_fallback_cpu: !s.gpu_fallback_cpu }))} />
              </div>
            )}
          </div>

          <div style={{ borderTop: '1px solid var(--border-default)' }} />

          {/* 元数据存储路径 */}
          <div>
            <div className="flex items-center gap-2 mb-1">
              <FolderCog size={16} style={{ color: '#F59E0B' }} />
              <h4 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>媒体元数据存储位置</h4>
            </div>
            <p className="mt-1 mb-2.5 text-xs leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>自定义媒体元数据的保存路径，留空使用默认。</p>
            <input type="text" value={sysSettings.metadata_store_path} onChange={(e) => setSysSettings((s) => ({ ...s, metadata_store_path: e.target.value }))} className="input w-full" placeholder="留空使用默认路径" />
          </div>

          <div style={{ borderTop: '1px solid var(--border-default)' }} />

          {/* 播放缓存目录 */}
          <div>
            <div className="flex items-center gap-2 mb-1">
              <HardDrive size={16} style={{ color: '#10B981' }} />
              <h4 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>播放缓存目录</h4>
            </div>
            <p className="mt-1 mb-2.5 text-xs leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>自定义转码缓存目录，留空使用默认。</p>
            <input type="text" value={sysSettings.play_cache_path} onChange={(e) => setSysSettings((s) => ({ ...s, play_cache_path: e.target.value }))} className="input w-full" placeholder="留空使用默认路径" />
          </div>

          <div style={{ borderTop: '1px solid var(--border-default)' }} />

          {/* 网盘直连 */}
          <div className="flex items-start justify-between gap-4">
            <div className="flex-1">
              <div className="flex items-center gap-2">
                <Link size={16} style={{ color: '#F59E0B' }} />
                <h4 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>网盘优先直连播放</h4>
              </div>
              <p className="mt-1 text-xs leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>播放网盘文件时优先使用直链进行在线播放。</p>
            </div>
            <ToggleButton checked={sysSettings.enable_direct_link} onChange={() => setSysSettings((s) => ({ ...s, enable_direct_link: !s.enable_direct_link }))} />
          </div>

          {/* 保存 */}
          <div style={{ borderTop: '1px solid var(--border-default)', paddingTop: '1rem' }}>
            {sysSettingsMsg && (
              <div className={clsx('mb-3 flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm', sysSettingsMsg.type === 'success' && 'bg-green-500/10 text-green-400', sysSettingsMsg.type === 'error' && 'bg-red-500/10 text-red-400')}>
                {sysSettingsMsg.type === 'success' ? <Check size={16} /> : <X size={16} />} {sysSettingsMsg.text}
              </div>
            )}
            <button onClick={handleSaveSettings} disabled={sysSettingsSaving} className="btn-primary gap-1.5 px-5 py-2.5 text-sm">
              {sysSettingsSaving ? (<><Loader2 size={14} className="animate-spin" />保存中...</>) : (<><Save size={14} />保存设置</>)}
            </button>
          </div>
        </div>
      </section>

      {/* 活动日志 */}
      <section>
        <h2 className="mb-3 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Activity size={20} className="text-neon/60" />
          活动日志
        </h2>
        {realtimeMessages.length > 0 ? (
          <div className="glass-panel-subtle max-h-48 overflow-y-auto rounded-xl p-4 space-y-1.5">
            {realtimeMessages.map((msg, i) => (
              <p key={i} className={clsx('text-xs font-mono', i === 0 ? 'text-surface-300' : 'text-surface-500')}>{msg}</p>
            ))}
          </div>
        ) : (
          <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-12 text-center">
            <p className="text-sm text-surface-500">暂无活动日志</p>
          </div>
        )}
      </section>

      {/* 快捷入口 */}
      <section>
        <h2 className="mb-4 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>快捷入口</h2>
        <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
          {SHORTCUT_TABS.map((tab) => {
            const Icon = tab.icon
            return (
              <button key={tab.id} onClick={() => switchTab(tab.id)} className="glass-panel-subtle group flex flex-col items-center gap-3 rounded-xl p-5 transition-all duration-300 hover:border-neon-blue/20 hover:shadow-card-hover">
                <div className="flex h-12 w-12 items-center justify-center rounded-xl transition-all duration-300 group-hover:scale-110" style={{ background: 'var(--neon-blue-6)', border: '1px solid var(--neon-blue-10)' }}>
                  <Icon size={22} className="text-neon/70 transition-colors group-hover:text-neon" />
                </div>
                <span className="text-sm font-medium transition-colors group-hover:text-neon" style={{ color: 'var(--text-secondary)' }}>{tab.label}</span>
              </button>
            )
          })}
        </div>
      </section>
    </div>
  )
}

// 可复用的 Toggle 按钮
function ToggleButton({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <button
      type="button" role="switch" aria-checked={checked}
      onClick={onChange}
      className="relative inline-flex h-6 w-11 flex-shrink-0 cursor-pointer rounded-full transition-colors duration-300 focus:outline-none"
      style={{
        background: checked ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-purple))' : 'var(--border-default)',
        boxShadow: checked ? 'var(--neon-glow-shadow-md)' : 'none',
      }}
    >
      <span className="pointer-events-none inline-block h-5 w-5 rounded-full shadow-lg transition-transform duration-300" style={{ transform: checked ? 'translateX(20px) translateY(2px)' : 'translateX(2px) translateY(2px)', background: checked ? '#fff' : 'var(--text-muted)' }} />
    </button>
  )
}
