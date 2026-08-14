import { useState } from 'react'
import type { SystemInfo, SystemSettings } from '@/types'
import type { ScanProgressData, ScrapeProgressData, TranscodeProgressData, ScanPhaseData } from '@/hooks/useWebSocket'
import {
  Activity,
  AlertTriangle,
  Check,
  Cpu,
  FolderCog,
  HardDrive,
  Link,
  Loader2,
  Merge,
  MonitorPlay,
  Play,
  Save,
  Scan,
  Server,
  Settings,
  ShieldAlert,
  Trash2,
  X,
  Zap,
} from 'lucide-react'
import { adminApi } from '@/api'
import { AdminPanel, AdminStatus } from '@/components/admin/AdminPrimitives'
import { Button, Input, Modal, ModalBody, ModalFooter, ModalHeader, Tag } from '@/components/design-system'

const webAppVersion = import.meta.env.VITE_APP_VERSION || '0.1.0'

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

type MergeResult = {
  type: 'success' | 'error' | 'info'
  message: string
  groups_processed?: number
  total_merged?: number
}

type ClearResult = {
  status: string
  message: string
  total_cleared: number
  success_count: number
  error_count: number
  details: { table: string; cleared: number; status: string; message?: string }[]
}

export default function DashboardTab({
  systemInfo,
  sysSettings,
  setSysSettings,
  scanProgress,
  scrapeProgress,
  transcodeProgress,
  scanPhase,
}: DashboardTabProps) {
  const [sysSettingsSaving, setSysSettingsSaving] = useState(false)
  const [sysSettingsMsg, setSysSettingsMsg] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const [clearDialogOpen, setClearDialogOpen] = useState(false)
  const [clearConfirmText, setClearConfirmText] = useState('')
  const [clearLoading, setClearLoading] = useState(false)
  const [clearResult, setClearResult] = useState<ClearResult | null>(null)
  const [mergeLoading, setMergeLoading] = useState(false)
  const [mergeCandidatesLoading, setMergeCandidatesLoading] = useState(false)
  const [mergeResult, setMergeResult] = useState<MergeResult | null>(null)
  const [mergeCandidates, setMergeCandidates] = useState<{
    normalized_title: string
    count: number
    series: { id: string; title: string; season_count: number; episode_count: number }[]
  }[] | null>(null)

  const hasActiveProgress = Object.keys(scanProgress).length > 0
    || Object.keys(scrapeProgress).length > 0
    || Object.keys(transcodeProgress).length > 0
    || Object.keys(scanPhase).length > 0

  const phaseLabels: Record<string, string> = {
    scanning: '扫描文件',
    scraping: '识别信息',
    merging: '合并剧集',
    matching: '匹配合集',
    cleaning: '清理数据',
    completed: '处理完成',
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
      const response = await adminApi.updateSystemSettings(sysSettings)
      if (response.data.data) setSysSettings(response.data.data)
      setSysSettingsMsg({ type: 'success', text: '系统设置已保存' })
      window.setTimeout(() => setSysSettingsMsg(null), 4000)
    } catch (error: any) {
      setSysSettingsMsg({ type: 'error', text: error?.response?.data?.error || '保存失败，请稍后重试' })
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
        setMergeResult({ type: 'info', message: `发现 ${data.length} 组可合并的重复剧集` })
      } else {
        setMergeCandidates([])
        setMergeResult({ type: 'success', message: '没有发现需要合并的重复剧集，数据已是最佳状态' })
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
      setMergeCandidates(null)
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '自动合并失败，请稍后重试'
      setMergeResult({ type: 'error', message: msg })
    } finally {
      setMergeLoading(false)
    }
  }

  return (
    <div className="space-y-6">
      {hasActiveProgress && (
        <AdminPanel
          title="实时进度"
          description="扫描、刮削与转码任务的当前运行状态。"
          icon={<Loader2 size={18} className="animate-spin" />}
          bodyClassName="space-y-3"
        >
          {Object.entries(scanPhase).map(([libId, data]) => (
            <ProgressItem
              key={`phase-${libId}`}
              title={`${phaseLabels[data.phase] || data.phase} · ${data.library_name}`}
              meta={`步骤 ${data.step_current}/${data.step_total}`}
              message={data.message}
              progress={data.step_total > 0 ? (data.step_current / data.step_total) * 100 : 0}
            />
          ))}

          {Object.entries(scanProgress).map(([libId, data]) => (
            <ProgressItem
              key={`scan-${libId}`}
              title={`扫描 · ${data.library_name}`}
              meta={`新增 ${data.new_found} 个文件`}
              message={data.message}
            />
          ))}

          {Object.entries(scrapeProgress).map(([key, data]) => (
            <ProgressItem
              key={`scrape-${key}`}
              title="元数据刮削"
              meta={`${data.current}/${data.total} · 成功 ${data.success} · 失败 ${data.failed}`}
              message={data.message}
              progress={data.total > 0 ? (data.current / data.total) * 100 : 0}
            />
          ))}

          {Object.entries(transcodeProgress).map(([taskId, data]) => (
            <ProgressItem
              key={`transcode-${taskId}`}
              title={`转码 · ${data.title} (${data.quality})`}
              meta={`${data.progress.toFixed(1)}%${data.speed ? ` · ${data.speed}` : ''}`}
              progress={data.progress}
              tone="warning"
            />
          ))}
        </AdminPanel>
      )}

      {systemInfo && (
        <AdminPanel
          title="系统状态"
          description="当前服务运行环境与资源概览。"
          icon={<Server size={18} />}
        >
          <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 xl:grid-cols-5">
            <MetricCard icon={<Cpu size={16} />} label="CPU 核心数" value={`${systemInfo.cpus} 核`} />
            <MetricCard icon={<Activity size={16} />} label="Go 协程" value={String(systemInfo.goroutines)} detail="活跃 goroutine" />
            <MemoryMetric systemInfo={systemInfo} />
            <MetricCard
              icon={<Zap size={16} />}
              label="硬件加速"
              value={hwAccelLabel(systemInfo.hw_accel)}
              status={systemInfo.hw_accel !== 'none' ? 'success' : 'warning'}
            />
            <MetricCard
              icon={<Server size={16} />}
              label="版本"
              value={`v${systemInfo.version}`}
              detail={`Web v${webAppVersion} · ${systemInfo.go_version} / ${systemInfo.os}_${systemInfo.arch}`}
            />
          </div>
        </AdminPanel>
      )}

      <AdminPanel
        title="系统设置"
        description="以下配置对所有媒体库统一生效；媒体库独立设置请在媒体库管理中调整。"
        icon={<Settings size={18} />}
        bodyClassName="space-y-0"
      >
        <SettingRow
          icon={<Zap size={16} />}
          title="GPU 加速转码"
          description="启用 GPU 硬件加速转码，显著提升转码速度。"
          control={(
            <ToggleButton
              checked={sysSettings.enable_gpu_transcode}
              onChange={() => setSysSettings((s) => ({ ...s, enable_gpu_transcode: !s.enable_gpu_transcode }))}
            />
          )}
        >
          {sysSettings.enable_gpu_transcode && (
            <div className="mt-3 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3">
              <div className="flex items-start justify-between gap-4">
                <div>
                  <h4 className="text-xs font-semibold text-[var(--nv-text-secondary)]">GPU 不支持时自动回退 CPU</h4>
                  <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">当 GPU 不支持特定格式解码时，系统自动切换至 CPU 转码。</p>
                </div>
                <ToggleButton
                  checked={sysSettings.gpu_fallback_cpu}
                  onChange={() => setSysSettings((s) => ({ ...s, gpu_fallback_cpu: !s.gpu_fallback_cpu }))}
                />
              </div>
            </div>
          )}
        </SettingRow>

        <SettingRow
          icon={<FolderCog size={16} />}
          title="媒体元数据存储位置"
          description="自定义媒体元数据的保存路径，留空使用默认。"
        >
          <Input
            value={sysSettings.metadata_store_path}
            onChange={(event) => setSysSettings((s) => ({ ...s, metadata_store_path: event.target.value }))}
            placeholder="留空使用默认路径"
          />
        </SettingRow>

        <SettingRow
          icon={<HardDrive size={16} />}
          title="播放缓存目录"
          description="自定义转码缓存目录，留空使用默认。"
        >
          <Input
            value={sysSettings.play_cache_path}
            onChange={(event) => setSysSettings((s) => ({ ...s, play_cache_path: event.target.value }))}
            placeholder="留空使用默认路径"
          />
        </SettingRow>

        <SettingRow
          icon={<Link size={16} />}
          title="网盘优先直连播放"
          description="播放网盘文件时优先使用直链进行在线播放。"
          control={(
            <ToggleButton
              checked={sysSettings.enable_direct_link}
              onChange={() => setSysSettings((s) => ({ ...s, enable_direct_link: !s.enable_direct_link }))}
            />
          )}
        />

        <SettingRow
          icon={<MonitorPlay size={16} />}
          title="优先直接播放"
          description="开启后播放器默认使用原始格式直接播放，不自动触发转码。"
          control={(
            <ToggleButton
              checked={sysSettings.prefer_direct_play}
              onChange={() => setSysSettings((s) => ({ ...s, prefer_direct_play: !s.prefer_direct_play }))}
            />
          )}
        />

        <SettingRow
          icon={<Scan size={16} />}
          title="扫描后自动预处理"
          description="扫描媒体库完成时自动触发视频预处理和字幕预处理。"
          control={(
            <ToggleButton
              checked={sysSettings.auto_preprocess_on_scan}
              onChange={() => setSysSettings((s) => ({ ...s, auto_preprocess_on_scan: !s.auto_preprocess_on_scan }))}
            />
          )}
        />

        <SettingRow
          icon={<Play size={16} />}
          title="播放时自动转码"
          description="播放不支持直接播放的格式时自动触发实时转码。"
          control={(
            <ToggleButton
              checked={sysSettings.auto_transcode_on_play}
              onChange={() => setSysSettings((s) => ({ ...s, auto_transcode_on_play: !s.auto_transcode_on_play }))}
            />
          )}
        />

        <div className="border-t border-[var(--nv-border-subtle)] pt-4">
          {sysSettingsMsg && (
            <div className="mb-3">
              <AdminStatus tone={sysSettingsMsg.type === 'success' ? 'success' : 'danger'}>
                {sysSettingsMsg.type === 'success' ? <Check size={13} /> : <X size={13} />}
                {sysSettingsMsg.text}
              </AdminStatus>
            </div>
          )}
          <Button variant="primary" onClick={handleSaveSettings} loading={sysSettingsSaving}>
            {sysSettingsSaving ? <Loader2 size={15} className="animate-spin" /> : <Save size={15} />}
            {sysSettingsSaving ? '保存中...' : '保存设置'}
          </Button>
        </div>
      </AdminPanel>

      <AdminPanel
        title="剧集合并管理"
        description="检测并合并同名但分季的剧集记录，让多季内容统一展示。"
        icon={<Merge size={18} />}
        actions={(
          <>
            <Button
              variant="secondary"
              size="sm"
              onClick={handleCheckMergeCandidates}
              disabled={mergeCandidatesLoading || mergeLoading}
            >
              {mergeCandidatesLoading && <Loader2 size={14} className="animate-spin" />}
              {mergeCandidatesLoading ? '检测中...' : '检测可合并剧集'}
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleAutoMerge}
              disabled={mergeLoading || mergeCandidatesLoading}
            >
              {mergeLoading ? <Loader2 size={14} className="animate-spin" /> : <Merge size={14} />}
              {mergeLoading ? '合并中...' : '一键自动合并'}
            </Button>
          </>
        )}
        bodyClassName="space-y-4"
      >
        {mergeResult && (
          <AdminStatus tone={mergeResult.type === 'success' ? 'success' : mergeResult.type === 'error' ? 'danger' : 'active'}>
            {mergeResult.type === 'success' ? <Check size={13} /> : mergeResult.type === 'error' ? <X size={13} /> : <Merge size={13} />}
            {mergeResult.message}
            {mergeResult.groups_processed !== undefined && (
              <span>· {mergeResult.groups_processed} 组 / {mergeResult.total_merged} 条</span>
            )}
          </AdminStatus>
        )}

        {mergeCandidates && mergeCandidates.length > 0 && (
          <div className="overflow-hidden rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)]">
            <div className="overflow-x-auto">
              <table className="w-full min-w-[640px] text-sm">
                <thead className="bg-[var(--nv-bg-surface-soft)] text-left text-xs text-[var(--nv-text-tertiary)]">
                  <tr>
                    <th className="px-4 py-3 font-semibold">系列名</th>
                    <th className="px-4 py-3 text-center font-semibold">重复条数</th>
                    <th className="px-4 py-3 font-semibold">包含的标题</th>
                  </tr>
                </thead>
                <tbody>
                  {mergeCandidates.map((group) => (
                    <tr key={group.normalized_title} className="border-t border-[var(--nv-border-subtle)]">
                      <td className="px-4 py-3 font-medium text-[var(--nv-text-primary)]">{group.normalized_title}</td>
                      <td className="px-4 py-3 text-center"><Tag tone="brand">{group.count}</Tag></td>
                      <td className="px-4 py-3 text-[var(--nv-text-tertiary)]">{group.series.map((series) => series.title).join('、')}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </AdminPanel>

      <AdminPanel
        title="危险区域"
        description="这些操作会永久改变服务器数据，请确认影响范围后再执行。"
        icon={<ShieldAlert size={18} />}
        className="border-[color-mix(in_srgb,var(--nv-status-danger)_24%,var(--nv-border-subtle))]"
      >
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start">
          <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[color-mix(in_srgb,var(--nv-status-danger)_10%,transparent)] text-[var(--nv-status-danger)]">
            <Trash2 size={20} />
          </div>
          <div className="min-w-0 flex-1">
            <h4 className="text-sm font-semibold text-[var(--nv-text-primary)]">一键彻底清空所有数据</h4>
            <p className="mt-1 text-xs leading-6 text-[var(--nv-text-tertiary)]">
              清除用户数据、元数据、观看历史、收藏、播放列表、评论、缓存、媒体库配置和系统设置。
              磁盘影视文件与当前管理员账号会保留，此操作不可撤销。
            </p>
          </div>
          <Button variant="danger" size="sm" onClick={() => { setClearDialogOpen(true); setClearResult(null) }}>
            <Trash2 size={14} />
            清空数据
          </Button>
        </div>

        {clearResult && (
          <div className="mt-4 space-y-3 border-t border-[var(--nv-border-subtle)] pt-4">
            <AdminStatus tone={clearResult.status === 'success' ? 'success' : clearResult.status === 'partial' ? 'warning' : 'danger'}>
              {clearResult.status === 'success' ? <Check size={13} /> : clearResult.status === 'partial' ? <AlertTriangle size={13} /> : <X size={13} />}
              {clearResult.message}
            </AdminStatus>

            {clearResult.details.length > 0 && (
              <div className="max-h-64 overflow-auto rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)]">
                <table className="w-full min-w-[520px] text-xs">
                  <thead className="sticky top-0 bg-[var(--nv-bg-elevated)] text-[var(--nv-text-tertiary)]">
                    <tr>
                      <th className="px-3 py-2 text-left font-semibold">数据类型</th>
                      <th className="px-3 py-2 text-right font-semibold">清理条数</th>
                      <th className="px-3 py-2 text-center font-semibold">状态</th>
                    </tr>
                  </thead>
                  <tbody>
                    {clearResult.details.map((detail) => (
                      <tr key={detail.table} className="border-t border-[var(--nv-border-subtle)]">
                        <td className="px-3 py-2 text-[var(--nv-text-primary)]">{detail.table}</td>
                        <td className="px-3 py-2 text-right font-mono text-[var(--nv-text-secondary)]">{detail.cleared}</td>
                        <td className="px-3 py-2 text-center">
                          <Tag tone={detail.status === 'success' ? 'success' : 'danger'} title={detail.message}>
                            {detail.status === 'success' ? '成功' : '失败'}
                          </Tag>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}

            <div className="flex flex-wrap gap-2">
              <Tag tone="success">成功 {clearResult.success_count} 项</Tag>
              {clearResult.error_count > 0 && <Tag tone="danger">失败 {clearResult.error_count} 项</Tag>}
              <Tag>共清理 {clearResult.total_cleared} 条记录</Tag>
            </div>
          </div>
        )}
      </AdminPanel>

      <Modal
        open={clearDialogOpen}
        onClose={() => {
          if (!clearLoading) {
            setClearDialogOpen(false)
            setClearConfirmText('')
          }
        }}
        size="sm"
        closeOnBackdrop={!clearLoading}
        closeOnEscape={!clearLoading}
        ariaLabel="确认彻底清空所有数据"
      >
        <ModalHeader
          title="确认彻底清空所有数据"
          description="此操作不可撤销，将彻底清除服务器业务数据。"
          icon={<AlertTriangle size={18} className="text-[var(--nv-status-danger)]" />}
          onClose={clearLoading ? undefined : () => {
            setClearDialogOpen(false)
            setClearConfirmText('')
          }}
        />
        <ModalBody className="space-y-4">
          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4 text-xs leading-6 text-[var(--nv-text-tertiary)]">
            <p className="font-semibold text-[var(--nv-text-secondary)]">将彻底清除：</p>
            <ul className="mt-2 list-disc space-y-1 pl-5">
              <li>观看历史、收藏、播放列表、书签和评论</li>
              <li>媒体与剧集记录、元数据、海报与缓存</li>
              <li>刮削、转码、AI、分享、同步等业务数据</li>
              <li>媒体库配置、系统设置和其他用户账号</li>
            </ul>
            <p className="mt-3 font-semibold text-[var(--nv-status-success)]">磁盘影视文件和当前管理员账号会保留。</p>
          </div>
          <div>
            <label htmlFor="clear-confirm" className="mb-2 block text-xs font-semibold text-[var(--nv-text-secondary)]">
              请输入 <span className="text-[var(--nv-status-danger)]">彻底清空</span> 以确认
            </label>
            <Input
              id="clear-confirm"
              value={clearConfirmText}
              onChange={(event) => setClearConfirmText(event.target.value)}
              placeholder="彻底清空"
              disabled={clearLoading}
              invalid={clearConfirmText.length > 0 && clearConfirmText !== '彻底清空'}
              autoFocus
            />
          </div>
        </ModalBody>
        <ModalFooter>
          <Button
            variant="secondary"
            onClick={() => {
              setClearDialogOpen(false)
              setClearConfirmText('')
            }}
            disabled={clearLoading}
          >
            取消
          </Button>
          <Button
            variant="danger"
            onClick={handleClearAllData}
            disabled={clearConfirmText !== '彻底清空' || clearLoading}
            loading={clearLoading}
          >
            {clearLoading ? <Loader2 size={14} className="animate-spin" /> : <Trash2 size={14} />}
            {clearLoading ? '正在清理...' : '彻底清空'}
          </Button>
        </ModalFooter>
      </Modal>
    </div>
  )
}

function ProgressItem({
  title,
  meta,
  message,
  progress,
  tone = 'active',
}: {
  title: string
  meta: string
  message?: string
  progress?: number
  tone?: 'active' | 'warning'
}) {
  const value = progress === undefined ? undefined : Math.max(0, Math.min(100, progress))
  return (
    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <span className="text-sm font-semibold text-[var(--nv-text-primary)]">{title}</span>
        <AdminStatus tone={tone}>{meta}</AdminStatus>
      </div>
      {value !== undefined && (
        <div className="mt-3 h-1.5 overflow-hidden rounded-full bg-[var(--nv-bg-control)]">
          <div
            className={`h-full rounded-full transition-[width] duration-300 ${tone === 'warning' ? 'bg-[var(--nv-status-warning)]' : 'bg-[var(--nv-action-primary)]'}`}
            style={{ width: `${value}%` }}
          />
        </div>
      )}
      {message && <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{message}</p>}
    </div>
  )
}

function MetricCard({
  icon,
  label,
  value,
  detail,
  status,
}: {
  icon: React.ReactNode
  label: string
  value: string
  detail?: string
  status?: 'success' | 'warning'
}) {
  return (
    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
      <div className="flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
        <span className="text-[var(--nv-action-primary)]">{icon}</span>
        {label}
      </div>
      <div className="mt-2 flex items-center gap-2">
        <p className="text-lg font-bold text-[var(--nv-text-primary)]">{value}</p>
        {status && <Tag tone={status}>{status === 'success' ? '可用' : 'CPU'}</Tag>}
      </div>
      {detail && <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">{detail}</p>}
    </div>
  )
}

function MemoryMetric({ systemInfo }: { systemInfo: SystemInfo }) {
  const processMB = systemInfo.memory.process_used_mb ?? systemInfo.memory.sys_mb ?? systemInfo.memory.alloc_mb
  const hostTotalMB = systemInfo.memory.system_total_mb
  const pct = systemInfo.memory.process_used_percent ?? (hostTotalMB ? (processMB / hostTotalMB) * 100 : 0)
  const pctText = pct < 1 ? pct.toFixed(2) : pct.toFixed(1)
  const fmt = (mb: number) => mb >= 1024 ? `${(mb / 1024).toFixed(2)} GB` : `${mb.toFixed(1)} MB`
  const tone = pct > 50 ? 'danger' : pct > 20 ? 'warning' : 'success'

  return (
    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
      <div className="flex items-center gap-2 text-xs text-[var(--nv-text-tertiary)]">
        <HardDrive size={16} className="text-[var(--nv-action-primary)]" />
        进程内存
      </div>
      <p className="mt-2 text-lg font-bold text-[var(--nv-text-primary)]">{fmt(processMB)}</p>
      {hostTotalMB && (
        <div className="mt-2 h-1.5 overflow-hidden rounded-full bg-[var(--nv-bg-control)]">
          <div
            className={`h-full rounded-full ${tone === 'danger' ? 'bg-[var(--nv-status-danger)]' : tone === 'warning' ? 'bg-[var(--nv-status-warning)]' : 'bg-[var(--nv-status-success)]'}`}
            style={{ width: `${Math.min(Math.max(pct, 0.5), 100)}%` }}
          />
        </div>
      )}
      <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">
        {hostTotalMB ? `占主机 ${pctText}% · ` : ''}堆 {systemInfo.memory.alloc_mb.toFixed(1)} MB
      </p>
    </div>
  )
}

function SettingRow({
  icon,
  title,
  description,
  control,
  children,
}: {
  icon: React.ReactNode
  title: string
  description: string
  control?: React.ReactNode
  children?: React.ReactNode
}) {
  return (
    <div className="border-b border-[var(--nv-border-subtle)] py-5 last:border-b-0">
      <div className="flex items-start justify-between gap-4">
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2 text-[var(--nv-action-primary)]">
            {icon}
            <h4 className="text-sm font-semibold text-[var(--nv-text-primary)]">{title}</h4>
          </div>
          <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">{description}</p>
        </div>
        {control && <div className="shrink-0">{control}</div>}
      </div>
      {children && <div className="mt-3">{children}</div>}
    </div>
  )
}

function ToggleButton({ checked, onChange }: { checked: boolean; onChange: () => void }) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      onClick={onChange}
      className="relative inline-flex h-6 w-11 shrink-0 rounded-full border transition-[background-color,border-color] duration-200 focus:outline-none"
      style={{
        background: checked ? 'var(--nv-action-primary)' : 'var(--nv-bg-control)',
        borderColor: checked ? 'var(--nv-action-primary)' : 'var(--nv-border-default)',
      }}
    >
      <span
        className="pointer-events-none absolute top-[2px] h-[18px] w-[18px] rounded-full bg-[var(--nv-bg-elevated)] shadow-sm transition-transform duration-200"
        style={{ transform: checked ? 'translateX(20px)' : 'translateX(2px)' }}
      />
    </button>
  )
}
