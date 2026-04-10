import { useState } from 'react'
import type { SystemInfo, SystemSettings } from '@/types'
import type { ScanProgressData, ScrapeProgressData, TranscodeProgressData, ScanPhaseData } from '@/hooks/useWebSocket'
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
  AlertTriangle,
  Trash2,
  ShieldAlert,
  Merge,
} from 'lucide-react'
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
  scanPhase: Record<string, ScanPhaseData>
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
  scanPhase,
  realtimeMessages,
  switchTab,
}: DashboardTabProps) {
  const [sysSettingsSaving, setSysSettingsSaving] = useState(false)
  const [sysSettingsMsg, setSysSettingsMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  // 一键清空数据相关状态
  const [clearDialogOpen, setClearDialogOpen] = useState(false)
  const [clearConfirmText, setClearConfirmText] = useState('')
  const [clearLoading, setClearLoading] = useState(false)
  const [clearResult, setClearResult] = useState<{
    status: string
    message: string
    total_cleared: number
    success_count: number
    error_count: number
    details: { table: string; cleared: number; status: string; message?: string }[]
  } | null>(null)

  // 剧集合并相关状态
  const [mergeLoading, setMergeLoading] = useState(false)
  const [mergeCandidatesLoading, setMergeCandidatesLoading] = useState(false)
  const [mergeResult, setMergeResult] = useState<{
    type: 'success' | 'error' | 'info'
    message: string
    groups_processed?: number
    total_merged?: number
  } | null>(null)
  const [mergeCandidates, setMergeCandidates] = useState<{
    normalized_title: string
    count: number
    series: { id: string; title: string; season_count: number; episode_count: number }[]
  }[] | null>(null)

  const hasActiveProgress = Object.keys(scanProgress).length > 0 || Object.keys(scrapeProgress).length > 0 || Object.keys(transcodeProgress).length > 0 || Object.keys(scanPhase).length > 0

  // 阶段名称映射
  const phaseLabels: Record<string, string> = {
    scanning: '📂 扫描文件',
    scraping: '🎨 识别信息',
    merging: '🔗 合并剧集',
    matching: '🎬 匹配合集',
    cleaning: '🧹 清理数据',
    completed: '✅ 处理完成',
  }

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

  const handleClearAllData = async () => {
    if (clearConfirmText !== '彻底清空') return
    setClearLoading(true)
    setClearResult(null)
    try {
      const res = await adminApi.clearAllData('CONFIRM_CLEAR_ALL')
      setClearResult(res.data.data)
      setClearDialogOpen(false)
      setClearConfirmText('')
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '清空数据失败，请稍后重试'
      setClearResult({
        status: 'error',
        message: msg,
        total_cleared: 0,
        success_count: 0,
        error_count: 1,
        details: [],
      })
    } finally {
      setClearLoading(false)
    }
  }

  const handleCheckMergeCandidates = async () => {
    setMergeCandidatesLoading(true)
    setMergeCandidates(null)
    setMergeResult(null)
    try {
      const res = await adminApi.mergeCandidates()
      const data = res.data.data
      if (data && data.length > 0) {
        setMergeCandidates(data)
        setMergeResult({
          type: 'info',
          message: `发现 ${data.length} 组可合并的重复剧集`,
        })
      } else {
        setMergeCandidates([])
        setMergeResult({
          type: 'success',
          message: '没有发现需要合并的重复剧集，数据已是最佳状态',
        })
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '检测失败，请稍后重试'
      setMergeResult({ type: 'error', message: msg })
    } finally {
      setMergeCandidatesLoading(false)
    }
  }

  const handleAutoMerge = async () => {
    setMergeLoading(true)
    setMergeResult(null)
    try {
      const res = await adminApi.autoMergeSeries()
      const data = res.data.data
      setMergeResult({
        type: 'success',
        message: res.data.message,
        groups_processed: data.groups_processed,
        total_merged: data.total_merged,
      })
      // 合并完成后清空候选列表
      setMergeCandidates(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '自动合并失败，请稍后重试'
      setMergeResult({ type: 'error', message: msg })
    } finally {
      setMergeLoading(false)
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
            {/* 扫描阶段进度（多步骤流程） */}
            {Object.entries(scanPhase).map(([libId, data]) => (
              <div key={`phase-${libId}`} className="glass-panel-subtle rounded-xl p-4" style={{ borderColor: 'var(--neon-blue-15)' }}>
                <div className="flex items-center justify-between mb-2">
                  <span className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                    {phaseLabels[data.phase] || data.phase} {data.library_name}
                  </span>
                  <span className="text-xs font-mono text-neon">
                    步骤 {data.step_current}/{data.step_total}
                  </span>
                </div>
                {/* 步骤进度条 */}
                <div className="mb-2 h-2 overflow-hidden rounded-full" style={{ background: 'var(--neon-blue-6)' }}>
                  <div
                    className="h-full rounded-full transition-all duration-700"
                    style={{
                      background: 'linear-gradient(90deg, var(--neon-blue), var(--neon-purple))',
                      width: `${data.step_total > 0 ? (data.step_current / data.step_total) * 100 : 0}%`,
                    }}
                  />
                </div>
                {/* 步骤圆点指示器 */}
                <div className="flex items-center gap-1.5 mb-1">
                  {Array.from({ length: data.step_total }, (_, i) => (
                    <div
                      key={i}
                      className="h-1.5 rounded-full transition-all duration-500"
                      style={{
                        width: i < data.step_current ? '16px' : '6px',
                        background: i < data.step_current ? 'var(--neon-blue)' : 'var(--neon-blue-10)',
                        opacity: i < data.step_current ? 1 : 0.4,
                      }}
                    />
                  ))}
                </div>
                <p className="text-xs text-surface-400">{data.message}</p>
              </div>
            ))}
            {/* 扫描进度 */}
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
                  <span className="text-xs" style={{ color: 'var(--neon-purple)' }}>{data.current}/{data.total} (成功:{data.success} 失败:{data.failed})</span>
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
                  <span className="text-xs" style={{ color: '#B45309' }}>{data.progress.toFixed(1)}% {data.speed && `| ${data.speed}`}</span>
                </div>
                <div className="h-2 overflow-hidden rounded-full" style={{ background: 'var(--neon-blue-6)' }}>
                  <div className="h-full rounded-full transition-all duration-300" style={{ width: `${data.progress}%`, background: '#D97706' }} />
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
              <p className="mt-2 text-lg font-bold" style={{ color: systemInfo.hw_accel !== 'none' ? '#16A34A' : '#CA8A04' }}>
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
              <div className="mb-3 flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm" style={{
                background: sysSettingsMsg.type === 'success' ? 'rgba(22, 163, 74, 0.08)' : 'rgba(220, 38, 38, 0.08)',
                color: sysSettingsMsg.type === 'success' ? '#16A34A' : '#DC2626',
              }}>
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
              <p key={i} className="text-xs font-mono" style={{ color: i === 0 ? 'var(--text-primary)' : 'var(--text-muted)' }}>{msg}</p>
            ))}
          </div>
        ) : (
          <div className="glass-panel-subtle flex items-center justify-center rounded-xl py-12 text-center">
            <p className="text-sm" style={{ color: 'var(--text-muted)' }}>暂无活动日志</p>
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

      {/* 剧集合并管理 */}
      <section>
        <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Merge size={20} className="text-neon/60" />
          剧集合并管理
        </h2>
        <div className="glass-panel rounded-xl p-5 space-y-4">
          <div className="rounded-lg p-3 text-xs leading-relaxed" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)', color: 'var(--text-tertiary)' }}>
            自动检测并合并同名但分季的剧集记录（如「女神咖啡厅 第一季」和「女神咖啡厅 第二季」），
            合并后多个季的剧集将统一在同一个条目下展示，用户看到的是无缝衔接的整体内容。
          </div>

          <div className="flex items-center gap-3">
            <button
              onClick={handleCheckMergeCandidates}
              disabled={mergeCandidatesLoading || mergeLoading}
              className="btn-secondary gap-1.5 px-4 py-2 text-sm"
            >
              {mergeCandidatesLoading ? (
                <><Loader2 size={14} className="animate-spin" />检测中...</>
              ) : (
                <>检测可合并剧集</>
              )}
            </button>
            <button
              onClick={handleAutoMerge}
              disabled={mergeLoading || mergeCandidatesLoading}
              className="btn-primary gap-1.5 px-4 py-2 text-sm"
            >
              {mergeLoading ? (
                <><Loader2 size={14} className="animate-spin" />合并中...</>
              ) : (
                <><Merge size={14} />一键自动合并</>
              )}
            </button>
          </div>

          {/* 合并结果提示 */}
          {mergeResult && (
            <div className="flex items-center gap-2 rounded-lg px-4 py-2.5 text-sm" style={{
              background: mergeResult.type === 'success' ? 'rgba(22, 163, 74, 0.08)' : mergeResult.type === 'error' ? 'rgba(220, 38, 38, 0.08)' : 'rgba(59, 130, 246, 0.08)',
              color: mergeResult.type === 'success' ? '#16A34A' : mergeResult.type === 'error' ? '#DC2626' : '#3B82F6',
            }}>
              {mergeResult.type === 'success' ? <Check size={16} /> : mergeResult.type === 'error' ? <X size={16} /> : <Merge size={16} />}
              {mergeResult.message}
              {mergeResult.groups_processed !== undefined && (
                <span className="ml-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                  （{mergeResult.groups_processed} 组，共合并 {mergeResult.total_merged} 条记录）
                </span>
              )}
            </div>
          )}

          {/* 候选列表 */}
          {mergeCandidates && mergeCandidates.length > 0 && (
            <div className="space-y-2">
              <h4 className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>可合并的剧集组：</h4>
              <div className="max-h-60 overflow-y-auto rounded-lg" style={{ background: 'var(--bg-elevated)' }}>
                <table className="w-full text-xs">
                  <thead>
                    <tr style={{ borderBottom: '1px solid var(--border-default)' }}>
                      <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-secondary)' }}>系列名</th>
                      <th className="px-3 py-2 text-center font-medium" style={{ color: 'var(--text-secondary)' }}>重复条数</th>
                      <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-secondary)' }}>包含的标题</th>
                    </tr>
                  </thead>
                  <tbody>
                    {mergeCandidates.map((group, idx) => (
                      <tr key={idx} style={{ borderBottom: idx < mergeCandidates.length - 1 ? '1px solid var(--border-default)' : 'none' }}>
                        <td className="px-3 py-2 font-medium" style={{ color: 'var(--text-primary)' }}>{group.normalized_title}</td>
                        <td className="px-3 py-2 text-center" style={{ color: 'var(--neon-blue)' }}>{group.count}</td>
                        <td className="px-3 py-2" style={{ color: 'var(--text-tertiary)' }}>
                          {group.series.map(s => s.title).join('、')}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}
        </div>
      </section>

      {/* 危险区域 - 一键清空数据 */}
      <section>
        <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: '#DC2626' }}>
          <ShieldAlert size={20} />
          危险区域
        </h2>
        <div className="rounded-xl p-5 space-y-4" style={{ background: 'rgba(220, 38, 38, 0.04)', border: '1px solid rgba(220, 38, 38, 0.15)' }}>
          <div className="flex items-start gap-4">
            <div className="flex h-12 w-12 flex-shrink-0 items-center justify-center rounded-xl" style={{ background: 'rgba(220, 38, 38, 0.08)', border: '1px solid rgba(220, 38, 38, 0.15)' }}>
              <Trash2 size={22} style={{ color: '#DC2626' }} />
            </div>
            <div className="flex-1">
              <h4 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>一键彻底清空所有数据</h4>
              <p className="mt-1 text-xs leading-relaxed" style={{ color: 'var(--text-tertiary)' }}>
                彻底清除所有数据：用户数据、元数据、观看历史、收藏、播放列表、评论、AI缓存、
                媒体库配置、系统设置、影视元数据缓存、封面数据等，<strong style={{ color: '#DC2626' }}>不保留任何记录</strong>。
                仅保留<strong style={{ color: '#16A34A' }}>磁盘上的影视文件</strong>（不做任何文件操作）和
                <strong style={{ color: '#16A34A' }}>当前管理员账号</strong>。此操作不可撤销。
              </p>
            </div>
            <button
              onClick={() => { setClearDialogOpen(true); setClearResult(null) }}
              className="flex-shrink-0 rounded-lg px-4 py-2 text-sm font-medium transition-all duration-200"
              style={{ background: 'rgba(220, 38, 38, 0.08)', border: '1px solid rgba(220, 38, 38, 0.2)', color: '#DC2626' }}
              onMouseEnter={(e) => { e.currentTarget.style.background = 'rgba(220, 38, 38, 0.15)'; e.currentTarget.style.borderColor = 'rgba(220, 38, 38, 0.4)' }}
              onMouseLeave={(e) => { e.currentTarget.style.background = 'rgba(220, 38, 38, 0.08)'; e.currentTarget.style.borderColor = 'rgba(220, 38, 38, 0.2)' }}
            >
              清空数据
            </button>
          </div>

          {/* 清理结果展示 */}
          {clearResult && (
            <div className="rounded-lg p-4 space-y-3" style={{
              background: clearResult.status === 'success' ? 'rgba(22, 163, 74, 0.06)' : clearResult.status === 'partial' ? 'rgba(202, 138, 4, 0.06)' : 'rgba(220, 38, 38, 0.06)',
              border: `1px solid ${clearResult.status === 'success' ? 'rgba(22, 163, 74, 0.15)' : clearResult.status === 'partial' ? 'rgba(202, 138, 4, 0.15)' : 'rgba(220, 38, 38, 0.15)'}`,
            }}>
              <div className="flex items-center gap-2">
                {clearResult.status === 'success' ? (
                  <Check size={16} style={{ color: '#16A34A' }} />
                ) : clearResult.status === 'partial' ? (
                  <AlertTriangle size={16} style={{ color: '#CA8A04' }} />
                ) : (
                  <X size={16} style={{ color: '#DC2626' }} />
                )}
                <span className="text-sm font-medium" style={{
                  color: clearResult.status === 'success' ? '#16A34A' : clearResult.status === 'partial' ? '#CA8A04' : '#DC2626',
                }}>
                  {clearResult.message}
                </span>
              </div>

              {/* 详细结果表格 */}
              {clearResult.details.length > 0 && (
                <div className="max-h-60 overflow-y-auto rounded-lg" style={{ background: 'var(--bg-elevated)' }}>
                  <table className="w-full text-xs">
                    <thead>
                      <tr style={{ borderBottom: '1px solid var(--border-default)' }}>
                        <th className="px-3 py-2 text-left font-medium" style={{ color: 'var(--text-secondary)' }}>数据类型</th>
                        <th className="px-3 py-2 text-right font-medium" style={{ color: 'var(--text-secondary)' }}>清理条数</th>
                        <th className="px-3 py-2 text-center font-medium" style={{ color: 'var(--text-secondary)' }}>状态</th>
                      </tr>
                    </thead>
                    <tbody>
                      {clearResult.details.map((detail, idx) => (
                        <tr key={idx} style={{ borderBottom: idx < clearResult.details.length - 1 ? '1px solid var(--border-default)' : 'none' }}>
                          <td className="px-3 py-1.5" style={{ color: 'var(--text-primary)' }}>{detail.table}</td>
                          <td className="px-3 py-1.5 text-right font-mono" style={{ color: 'var(--text-secondary)' }}>{detail.cleared}</td>
                          <td className="px-3 py-1.5 text-center">
                            {detail.status === 'success' ? (
                              <span className="inline-flex items-center gap-1" style={{ color: '#16A34A' }}><Check size={12} />成功</span>
                            ) : (
                              <span className="inline-flex items-center gap-1" style={{ color: '#DC2626' }} title={detail.message}><X size={12} />失败</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}

              {/* 汇总 */}
              <div className="flex items-center gap-4 text-xs" style={{ color: 'var(--text-tertiary)' }}>
                <span>成功: <strong style={{ color: '#16A34A' }}>{clearResult.success_count}</strong> 项</span>
                {clearResult.error_count > 0 && (
                  <span>失败: <strong style={{ color: '#DC2626' }}>{clearResult.error_count}</strong> 项</span>
                )}
                <span>共清理: <strong style={{ color: 'var(--text-primary)' }}>{clearResult.total_cleared}</strong> 条记录</span>
              </div>
            </div>
          )}
        </div>
      </section>

      {/* 二次确认弹窗 */}
      {clearDialogOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center backdrop-blur-sm" style={{ background: 'var(--bg-overlay)' }} onClick={() => !clearLoading && setClearDialogOpen(false)}>
          <div className="mx-4 w-full max-w-md rounded-2xl p-6 space-y-5" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)', boxShadow: 'var(--shadow-elevated)' }} onClick={(e) => e.stopPropagation()}>
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-full" style={{ background: 'rgba(220, 38, 38, 0.08)' }}>
                <AlertTriangle size={20} style={{ color: '#DC2626' }} />
              </div>
              <div>
                <h3 className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>确认彻底清空所有数据</h3>
                <p className="text-xs" style={{ color: '#DC2626' }}>此操作不可撤销，将彻底清除所有数据</p>
              </div>
            </div>

            <div className="rounded-lg p-3 text-xs leading-relaxed" style={{ background: 'var(--bg-subtle)', border: '1px solid var(--border-subtle)', color: 'var(--text-tertiary)' }}>
              <p className="mb-2 font-medium" style={{ color: 'var(--text-secondary)' }}>以下数据将被<strong style={{ color: '#DC2626' }}>彻底清除</strong>：</p>
              <ul className="list-disc pl-4 space-y-0.5">
                <li>观看历史、收藏、播放列表、书签、评论</li>
                <li>播放统计、演员信息、类型标签、内容分级</li>
                <li>刮削任务/历史、转码任务、定时任务</li>
                <li>AI缓存、推荐缓存、AI分析数据</li>
                <li>家庭社交、直播、同步、分享链接等数据</li>
                <li><strong>所有媒体和剧集记录</strong>（包括元数据、海报、封面路径等）</li>
                <li><strong>媒体库配置</strong>（需要重新添加媒体库）</li>
                <li><strong>系统设置</strong>（恢复默认配置）</li>
                <li><strong>其他用户账号</strong>（仅保留当前管理员）</li>
              </ul>
              <p className="mt-2 font-medium" style={{ color: '#16A34A' }}>仅保留：</p>
              <ul className="list-disc pl-4 space-y-0.5">
                <li>磁盘上的影视文件（不做任何文件操作）</li>
                <li>当前管理员账号</li>
              </ul>
            </div>

            <div>
              <label className="block text-xs font-medium mb-1.5" style={{ color: 'var(--text-secondary)' }}>
                请输入 <strong style={{ color: '#DC2626' }}>彻底清空</strong> 以确认操作
              </label>
              <input
                type="text"
                value={clearConfirmText}
                onChange={(e) => setClearConfirmText(e.target.value)}
                placeholder="彻底清空"
                className="input w-full"
                disabled={clearLoading}
                autoFocus
              />
            </div>

            <div className="flex items-center justify-end gap-3">
              <button
                onClick={() => { setClearDialogOpen(false); setClearConfirmText('') }}
                disabled={clearLoading}
                className="rounded-lg px-4 py-2 text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-subtle)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={handleClearAllData}
                disabled={clearConfirmText !== '彻底清空' || clearLoading}
                className="flex items-center gap-1.5 rounded-lg px-4 py-2 text-sm font-medium transition-all duration-200"
                style={{
                  background: clearConfirmText === '彻底清空' && !clearLoading ? '#DC2626' : 'rgba(220, 38, 38, 0.12)',
                  color: clearConfirmText === '彻底清空' && !clearLoading ? '#ffffff' : 'rgba(220, 38, 38, 0.4)',
                  cursor: clearConfirmText === '彻底清空' && !clearLoading ? 'pointer' : 'not-allowed',
                }}
                onMouseEnter={(e) => { if (clearConfirmText === '彻底清空' && !clearLoading) e.currentTarget.style.background = '#B91C1C' }}
                onMouseLeave={(e) => { if (clearConfirmText === '彻底清空' && !clearLoading) e.currentTarget.style.background = '#DC2626' }}
              >
                {clearLoading ? (
                  <><Loader2 size={14} className="animate-spin" />正在清理...</>
                ) : (
                  <><Trash2 size={14} />彻底清空</>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
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
