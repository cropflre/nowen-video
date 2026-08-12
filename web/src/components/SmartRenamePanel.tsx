import { useEffect, useMemo, useState } from 'react'
import { motion, AnimatePresence } from 'framer-motion'
import toast from 'react-hot-toast'
import { AlertTriangle, CheckCircle2, RotateCcw, Save, ScanSearch, Wand2, X } from 'lucide-react'
import { useDialog } from './Dialog'
import {
  smartRenameApi,
  parseRelatedFiles,
  parseSafety,
  type RenamePlan,
  type RenamePlanItem,
  type NamingStyle,
  type RenameItemStatus,
} from '@/api/smart_rename'
import { fadeInVariants } from '@/lib/motion'
import { Button, Input, Select, Surface, Tag, type TagTone } from '@/components/design-system'

export interface SmartRenamePanelProps {
  defaultPath?: string
  candidatePaths?: string[]
  showHeader?: boolean
  onPlanChange?: (plan: RenamePlan | null) => void
  compact?: boolean
}

const statusLabel: Record<RenameItemStatus, string> = {
  pending: '待执行',
  skipped: '已是目标格式',
  unsafe: '安全检测拦截',
  executed: '已落盘',
  failed: '执行失败',
  reverted: '已回滚',
}

const statusTone: Record<RenameItemStatus, TagTone> = {
  pending: 'brand',
  skipped: 'neutral',
  unsafe: 'warning',
  executed: 'success',
  failed: 'danger',
  reverted: 'neutral',
}

function toneForPlanStatus(status: string): TagTone {
  switch (status) {
    case 'draft': return 'brand'
    case 'executing': return 'brand'
    case 'completed': return 'success'
    case 'failed': return 'danger'
    case 'rolledback': return 'neutral'
    case 'canceled': return 'neutral'
    default: return 'neutral'
  }
}

export default function SmartRenamePanel({
  defaultPath = '',
  candidatePaths,
  showHeader = true,
  onPlanChange,
  compact = false,
}: SmartRenamePanelProps) {
  const dialog = useDialog()
  const [rootPath, setRootPath] = useState(defaultPath)
  const [style, setStyle] = useState<NamingStyle>('jellyfin')
  const [enableAI, setEnableAI] = useState(true)
  const [threshold, setThreshold] = useState(0.7)
  const [scanning, setScanning] = useState(false)
  const [scanProgress, setScanProgress] = useState(0)
  const [scanPhase, setScanPhase] = useState('')
  const [scanElapsed, setScanElapsed] = useState(0)
  const [plan, setPlan] = useState<RenamePlan | null>(null)
  const [filter, setFilter] = useState<'all' | 'pending' | 'unsafe' | 'skipped' | 'executed' | 'failed'>('all')
  const [confirmModal, setConfirmModal] = useState(false)
  const [executing, setExecuting] = useState(false)
  const [rollingBack, setRollingBack] = useState(false)

  useEffect(() => {
    setRootPath(defaultPath)
    setPlan(null)
    setFilter('all')
  }, [defaultPath])

  function emitPlan(nextPlan: RenamePlan | null) {
    setPlan(nextPlan)
    onPlanChange?.(nextPlan)
  }

  async function onScan() {
    if (!rootPath.trim()) {
      toast.error('请填写扫描根目录')
      return
    }
    setScanning(true)
    setScanProgress(0)
    setScanPhase('正在连接服务…')
    setScanElapsed(0)

    const startedAt = Date.now()
    const ceiling = enableAI ? 92 : 95
    const elapsedTimer = window.setInterval(() => {
      setScanElapsed(Math.floor((Date.now() - startedAt) / 1000))
    }, 500)
    const progressTimer = window.setInterval(() => {
      setScanProgress((progress) => {
        if (progress >= ceiling) return progress
        const remain = ceiling - progress
        const step = Math.max(0.3, remain * 0.03)
        const next = Math.min(ceiling, progress + step)
        if (next < 25) setScanPhase('① 枚举文件…')
        else if (next < 55) setScanPhase('② 解析命名规则…')
        else if (next < 80) setScanPhase(enableAI ? '③ 调用 AI Fallback 识别…' : '③ 评分与归一化…')
        else setScanPhase('④ 生成重命名规划…')
        return next
      })
    }, 250)

    try {
      const response = await smartRenameApi.scan({
        root_path: rootPath.trim(),
        naming_style: style,
        enable_ai_fallback: enableAI,
        ai_confidence_threshold: threshold,
      })
      setScanProgress(100)
      setScanPhase('✓ 扫描完成')
      emitPlan(response.data.data)
      toast.success(`扫描完成：${response.data.data.total_items} 个文件，需改名 ${response.data.data.need_rename}`)
    } catch (error: any) {
      setScanPhase('✕ 扫描失败')
      toast.error(`扫描失败：${error?.response?.data?.error || error.message || '未知错误'}`)
    } finally {
      window.clearInterval(progressTimer)
      window.clearInterval(elapsedTimer)
      window.setTimeout(() => {
        setScanning(false)
        setScanProgress(0)
        setScanPhase('')
        setScanElapsed(0)
      }, 800)
    }
  }

  async function onDryRun() {
    if (!plan) return
    setExecuting(true)
    try {
      const response = await smartRenameApi.execute({ plan_id: plan.id, confirm: false })
      toast.success('预演通过（未动盘）')
      emitPlan(response.data.data)
    } catch (error: any) {
      toast.error(`预演失败：${error?.response?.data?.error || error.message}`)
    } finally {
      setExecuting(false)
    }
  }

  async function onConfirmExecute(ignoreSafety: boolean) {
    if (!plan) return
    setExecuting(true)
    setConfirmModal(false)
    try {
      const response = await smartRenameApi.execute({
        plan_id: plan.id,
        confirm: true,
        ignore_safety: ignoreSafety,
      })
      toast.success(`落盘完成：成功 ${response.data.data.executed_items}，失败 ${response.data.data.failed_items}`)
      emitPlan(response.data.data)
    } catch (error: any) {
      toast.error(`落盘失败：${error?.response?.data?.error || error.message}`)
    } finally {
      setExecuting(false)
    }
  }

  async function onRollback() {
    if (!plan) return
    const ok = await dialog.confirm({
      title: '回滚重命名',
      message: '确定要回滚本次重命名吗？所有已落盘的文件将恢复原名。',
      confirmText: '回滚',
      variant: 'warning',
    })
    if (!ok) return
    setRollingBack(true)
    try {
      const response = await smartRenameApi.rollback(plan.id)
      toast.success('回滚完成')
      emitPlan(response.data.data)
    } catch (error: any) {
      toast.error(`回滚失败：${error?.response?.data?.error || error.message}`)
    } finally {
      setRollingBack(false)
    }
  }

  async function onUpdateItem(item: RenamePlanItem, patch: { override_name?: string; excluded?: boolean }) {
    try {
      const response = await smartRenameApi.updateItem(item.id, patch)
      setPlan((current) => {
        if (!current?.items) return current
        return {
          ...current,
          items: current.items.map((existing) => existing.id === item.id ? response.data.data : existing),
        }
      })
    } catch (error: any) {
      toast.error(`保存失败：${error?.response?.data?.error || error.message}`)
    }
  }

  const filteredItems = useMemo(() => {
    if (!plan?.items) return []
    if (filter === 'all') return plan.items
    return plan.items.filter((item) => item.status === filter)
  }, [plan, filter])

  const panelPadding = compact ? 'p-4' : 'p-5'

  return (
    <div className="w-full text-[var(--nv-text-primary)]">
      {showHeader && (
        <header className="mb-5">
          <h1 className="flex items-center gap-3 text-2xl font-semibold text-[var(--nv-text-primary)]">
            <Wand2 size={22} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
            智能扫描重命名
          </h1>
          <p className="mt-1 text-sm text-[var(--nv-text-secondary)]">
            基于规则评分 + AI Fallback 自动识别影视命名，按 Jellyfin/Emby/Plex 风格规范化。
            <span className="ml-1 text-[var(--nv-action-primary)]">默认 dry-run</span>，必须显式确认才会真正动盘。
          </p>
        </header>
      )}

      <Surface className={`mb-5 ${panelPadding}`}>
        <h2 className="mb-3 flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
          <ScanSearch size={15} className="text-[var(--nv-action-primary)]" aria-hidden="true" />
          ① 扫描配置
        </h2>
        <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
          <div className="md:col-span-2">
            <label className="mb-1 block text-xs font-medium text-[var(--nv-text-secondary)]">扫描根目录（绝对路径）</label>
            {candidatePaths && candidatePaths.length > 1 ? (
              <Select value={rootPath} onChange={(event) => setRootPath(event.target.value)}>
                {candidatePaths.map((path) => <option key={path} value={path}>{path}</option>)}
              </Select>
            ) : (
              <Input value={rootPath} onChange={(event) => setRootPath(event.target.value)} placeholder="例如：D:\Media\Movies" />
            )}
          </div>

          <div>
            <label className="mb-1 block text-xs font-medium text-[var(--nv-text-secondary)]">命名风格</label>
            <Select value={style} onChange={(event) => setStyle(event.target.value as NamingStyle)}>
              <option value="jellyfin">Jellyfin/Emby - [tmdbid-12345]</option>
              <option value="plex">Plex - {`{tmdb-12345}`}</option>
            </Select>
          </div>

          <div>
            <label className="mb-1 block text-xs font-medium text-[var(--nv-text-secondary)]">
              AI 触发阈值（规则评分 &lt; {threshold} 时启用 AI）
            </label>
            <input
              type="range"
              min={0}
              max={1}
              step={0.05}
              value={threshold}
              onChange={(event) => setThreshold(parseFloat(event.target.value))}
              className="w-full accent-[var(--nv-action-primary)]"
              aria-label="AI 触发阈值"
            />
          </div>

          <div className="flex items-center gap-3 md:col-span-2">
            <label className="flex items-center gap-2 text-sm text-[var(--nv-text-secondary)]">
              <input type="checkbox" checked={enableAI} onChange={(event) => setEnableAI(event.target.checked)} className="accent-[var(--nv-action-primary)]" />
              启用 AI Fallback
            </label>
            <Button type="button" variant="primary" className="ml-auto" onClick={onScan} loading={scanning} disabled={scanning}>
              {!scanning && <ScanSearch size={14} aria-hidden="true" />}
              {scanning ? '扫描中…' : '开始扫描'}
            </Button>
          </div>

          {scanning && (
            <div className="mt-2 md:col-span-2">
              <div className="mb-1.5 flex items-center justify-between text-xs">
                <span className="text-[var(--nv-text-secondary)]">{scanPhase || '准备中…'}</span>
                <span className="text-[var(--nv-text-tertiary)]">{scanProgress.toFixed(0)}% · 已用时 {formatElapsed(scanElapsed)}</span>
              </div>
              <div className="relative h-2 w-full overflow-hidden rounded-full border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)]">
                <div className="h-full rounded-full bg-[var(--nv-action-primary)] transition-[width] duration-300 ease-out" style={{ width: `${scanProgress}%` }} />
                {scanProgress < 100 && (
                  <div
                    className="pointer-events-none absolute inset-y-0 w-1/3 rounded-full opacity-60"
                    style={{ background: 'linear-gradient(90deg, transparent, rgba(255,255,255,.30), transparent)', animation: 'smartRenameShimmer 1.4s linear infinite' }}
                  />
                )}
              </div>
              <style>{`@keyframes smartRenameShimmer { 0% { transform: translateX(-100%);} 100% { transform: translateX(400%);} }`}</style>
              <div className="mt-1.5 text-[11px] text-[var(--nv-text-tertiary)]">💡 大目录可能耗时较长（依目录大小与 AI 响应速度而定），请勿关闭本面板</div>
            </div>
          )}
        </div>
      </Surface>

      {plan && (
        <motion.div variants={fadeInVariants} initial="hidden" animate="visible">
          <Surface className={`mb-5 ${panelPadding}`}>
            <div className="mb-4 flex flex-wrap items-center gap-3">
              <h2 className="text-sm font-semibold text-[var(--nv-text-primary)]">② 规划详情</h2>
              <Tag tone={toneForPlanStatus(plan.status)}>状态：{plan.status}</Tag>
              <span className="text-xs text-[var(--nv-text-tertiary)]">规划 ID: {plan.id.slice(0, 8)}…</span>
              <span className="min-w-0 truncate text-xs text-[var(--nv-text-tertiary)]" title={plan.root_path}>根目录：{plan.root_path}</span>
            </div>

            <div className="mb-4 grid grid-cols-2 gap-3 md:grid-cols-6">
              <Stat label="总文件" value={plan.total_items} />
              <Stat label="需改名" value={plan.need_rename} tone="brand" />
              <Stat label="已是目标" value={plan.skipped_items} />
              <Stat label="安全拦截" value={plan.unsafe_items} tone="warning" />
              <Stat label="已落盘" value={plan.executed_items} tone="success" />
              <Stat label="失败" value={plan.failed_items} tone="danger" />
            </div>

            <div className="mb-4 flex flex-wrap items-center gap-2">
              <Button type="button" variant="secondary" size="sm" onClick={onDryRun} disabled={executing || plan.status !== 'draft'} loading={executing && !confirmModal}>
                预演执行（dry-run）
              </Button>
              <Button type="button" variant="danger" size="sm" onClick={() => setConfirmModal(true)} disabled={executing || (plan.status !== 'draft' && plan.status !== 'failed')}>
                <AlertTriangle size={13} aria-hidden="true" />确认落盘（动盘）
              </Button>
              <Button type="button" variant="secondary" size="sm" onClick={onRollback} disabled={rollingBack || (plan.status !== 'completed' && plan.status !== 'failed')} loading={rollingBack}>
                {!rollingBack && <RotateCcw size={13} aria-hidden="true" />}回滚
              </Button>

              <div className="ml-auto flex flex-wrap items-center gap-1.5">
                <span className="text-xs text-[var(--nv-text-tertiary)]">筛选：</span>
                {(['all', 'pending', 'unsafe', 'skipped', 'executed', 'failed'] as const).map((value) => (
                  <Button
                    key={value}
                    type="button"
                    variant={filter === value ? 'primary' : 'ghost'}
                    size="sm"
                    aria-pressed={filter === value}
                    onClick={() => setFilter(value)}
                    className="h-8 px-2 text-xs"
                  >
                    {value === 'all' ? '全部' : statusLabel[value as RenameItemStatus] || value}
                  </Button>
                ))}
              </div>
            </div>

            <div className="space-y-2">
              <AnimatePresence>
                {filteredItems.map((item) => (
                  <ItemCard key={item.id} item={item} onUpdate={onUpdateItem} planEditable={plan.status === 'draft'} />
                ))}
              </AnimatePresence>
              {filteredItems.length === 0 && (
                <div className="rounded-[var(--nv-radius-control)] border border-dashed border-[var(--nv-border-default)] p-6 text-center text-sm text-[var(--nv-text-tertiary)]">没有匹配的条目</div>
              )}
            </div>
          </Surface>
        </motion.div>
      )}

      <AnimatePresence>
        {confirmModal && plan && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            className="fixed inset-0 z-[var(--nv-z-modal)] flex items-center justify-center p-4 backdrop-blur-sm"
            style={{ background: 'var(--nv-bg-overlay)' }}
            onClick={() => setConfirmModal(false)}
          >
            <motion.div
              initial={{ scale: 0.96, opacity: 0, y: 8 }}
              animate={{ scale: 1, opacity: 1, y: 0 }}
              exit={{ scale: 0.96, opacity: 0, y: 8 }}
              onClick={(event) => event.stopPropagation()}
              className="w-full max-w-md rounded-[var(--nv-radius-container)] border bg-[var(--nv-bg-elevated)] p-6 shadow-[var(--nv-shadow-elevated)]"
              style={{ borderColor: 'color-mix(in srgb, var(--nv-status-danger) 30%, var(--nv-border-default))' }}
              role="dialog"
              aria-modal="true"
              aria-label="确认落盘"
            >
              <div className="mb-2 flex items-center gap-2 text-[var(--nv-status-danger)]">
                <AlertTriangle size={20} aria-hidden="true" />
                <h3 className="text-lg font-semibold">确认落盘</h3>
              </div>
              <p className="mb-4 text-sm text-[var(--nv-text-secondary)]">
                即将对 <strong className="text-[var(--nv-action-primary)]">{plan.need_rename}</strong> 个文件执行物理重命名。
                此操作会修改磁盘上的真实文件，但全程记录在 journal 中，事后可一键回滚。
              </p>
              <p className="mb-4 text-xs text-[var(--nv-text-tertiary)]">安全拦截：{plan.unsafe_items} 条</p>
              <div className="flex flex-col gap-2">
                <Button type="button" variant="primary" onClick={() => onConfirmExecute(false)} loading={executing}>
                  <CheckCircle2 size={14} aria-hidden="true" />确认执行（跳过安全拦截条目）
                </Button>
                {plan.unsafe_items > 0 && (
                  <Button type="button" variant="danger" onClick={() => onConfirmExecute(true)} disabled={executing}>
                    <AlertTriangle size={14} aria-hidden="true" />强制执行（包含 {plan.unsafe_items} 条安全警告）
                  </Button>
                )}
                <Button type="button" variant="secondary" onClick={() => setConfirmModal(false)} disabled={executing}>
                  <X size={14} aria-hidden="true" />取消
                </Button>
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  )
}

function formatElapsed(seconds: number): string {
  const normalized = Number.isFinite(seconds) && seconds >= 0 ? seconds : 0
  const minutes = Math.floor(normalized / 60)
  const rest = normalized % 60
  return `${String(minutes).padStart(2, '0')}:${String(rest).padStart(2, '0')}`
}

function Stat({ label, value, tone = 'neutral' }: { label: string; value: number; tone?: TagTone }) {
  const color = tone === 'brand'
    ? 'var(--nv-action-primary)'
    : tone === 'success'
      ? 'var(--nv-status-success)'
      : tone === 'warning'
        ? 'var(--nv-status-warning)'
        : tone === 'danger'
          ? 'var(--nv-status-danger)'
          : 'var(--nv-text-primary)'

  return (
    <Surface className="bg-[var(--nv-bg-surface-soft)] px-3 py-2 shadow-none">
      <div className="text-[10px] uppercase text-[var(--nv-text-tertiary)]">{label}</div>
      <div className="mt-0.5 text-xl font-semibold" style={{ color }}>{value}</div>
    </Surface>
  )
}

interface ItemCardProps {
  item: RenamePlanItem
  onUpdate: (item: RenamePlanItem, patch: { override_name?: string; excluded?: boolean }) => void
  planEditable: boolean
}

function ItemCard({ item, onUpdate, planEditable }: ItemCardProps) {
  const [editing, setEditing] = useState(false)
  const [draftName, setDraftName] = useState(item.target_name)
  const related = parseRelatedFiles(item)
  const safety = parseSafety(item)

  useEffect(() => {
    setDraftName(item.target_name)
  }, [item.target_name])

  return (
    <motion.div
      layout
      initial={{ opacity: 0, y: 4 }}
      animate={{ opacity: 1, y: 0 }}
      exit={{ opacity: 0, scale: 0.98 }}
      className={`rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3 transition-opacity ${item.excluded ? 'opacity-50' : ''}`}
    >
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-0 flex-1">
          <div className="flex flex-wrap items-center gap-2 text-xs">
            <Tag tone={statusTone[item.status]}>{statusLabel[item.status] || item.status}</Tag>
            <span className="text-[var(--nv-text-tertiary)]">置信度</span>
            <span style={{ color: confidenceColor(item.confidence) }}>{(item.confidence * 100).toFixed(0)}%</span>
            {item.ai_invoked && <Tag tone="brand">AI</Tag>}
            {item.media_type === 'episode' && item.season_num > 0 && (
              <Tag tone="neutral">S{String(item.season_num).padStart(2, '0')}E{String(item.episode_num).padStart(2, '0')}</Tag>
            )}
            {item.parsed_tmdb_id > 0 && <Tag tone="success">TMDb {item.parsed_tmdb_id}</Tag>}
            {item.parsed_year > 0 && <span className="text-[var(--nv-text-secondary)]">{item.parsed_year}</span>}
          </div>

          <div className="mt-1.5 break-all font-mono text-xs text-[var(--nv-text-secondary)]">{item.source_path}</div>
          <div className="mt-1 flex items-center gap-2 text-xs">
            <span className="text-[var(--nv-text-tertiary)]">→</span>
            {editing && planEditable ? (
              <>
                <Input value={draftName} onChange={(event) => setDraftName(event.target.value)} className="h-8 flex-1 font-mono text-xs" />
                <Button
                  type="button"
                  variant="primary"
                  size="sm"
                  onClick={() => {
                    onUpdate(item, { override_name: draftName })
                    setEditing(false)
                  }}
                >
                  <Save size={12} aria-hidden="true" />保存
                </Button>
                <Button
                  type="button"
                  variant="ghost"
                  size="sm"
                  onClick={() => {
                    setDraftName(item.target_name)
                    setEditing(false)
                  }}
                >
                  取消
                </Button>
              </>
            ) : (
              <>
                <span className="break-all font-mono text-[var(--nv-action-primary)]">{item.target_name}</span>
                {planEditable && <Button type="button" variant="ghost" size="sm" onClick={() => setEditing(true)}>编辑</Button>}
              </>
            )}
          </div>

          {related.length > 0 && (
            <div className="mt-1.5 text-xs text-[var(--nv-text-tertiary)]">
              <span className="text-[var(--nv-text-secondary)]">关联资源：</span>
              {related.map((relatedFile, index) => (
                <Tag key={`${relatedFile.kind}-${index}`} tone="neutral" className="ml-1" title={`${relatedFile.source} → ${relatedFile.target}`}>
                  {relatedFile.kind}
                </Tag>
              ))}
            </div>
          )}

          {safety && (safety.issues?.length ?? 0) > 0 && (
            <div className="mt-1.5 rounded-[var(--nv-radius-sm)] border px-2 py-1 text-[11px] text-[var(--nv-status-warning)]" style={{ borderColor: 'color-mix(in srgb, var(--nv-status-warning) 30%, transparent)', background: 'color-mix(in srgb, var(--nv-status-warning) 6%, transparent)' }}>
              ⚠ {(safety.issues ?? []).join('; ')}
            </div>
          )}
          {item.error_msg && (
            <div className="mt-1.5 rounded-[var(--nv-radius-sm)] border px-2 py-1 text-[11px] text-[var(--nv-status-danger)]" style={{ borderColor: 'color-mix(in srgb, var(--nv-status-danger) 30%, transparent)', background: 'color-mix(in srgb, var(--nv-status-danger) 6%, transparent)' }}>
              ✕ {item.error_msg}
            </div>
          )}
        </div>

        {planEditable && (
          <Button type="button" variant="secondary" size="sm" onClick={() => onUpdate(item, { excluded: !item.excluded })}>
            {item.excluded ? '已排除（点击恢复）' : '排除本条'}
          </Button>
        )}
      </div>
    </motion.div>
  )
}

function confidenceColor(confidence: number) {
  if (confidence >= 0.85) return 'var(--nv-status-success)'
  if (confidence >= 0.6) return 'var(--nv-action-primary)'
  if (confidence >= 0.4) return 'var(--nv-status-warning)'
  return 'var(--nv-status-danger)'
}

export function planStatusBadge(status: string) {
  switch (toneForPlanStatus(status)) {
    case 'brand': return 'border border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]'
    case 'success': return 'bg-[color-mix(in_srgb,var(--nv-status-success)_10%,transparent)] text-[var(--nv-status-success)]'
    case 'warning': return 'bg-[color-mix(in_srgb,var(--nv-status-warning)_10%,transparent)] text-[var(--nv-status-warning)]'
    case 'danger': return 'bg-[color-mix(in_srgb,var(--nv-status-danger)_10%,transparent)] text-[var(--nv-status-danger)]'
    default: return 'bg-[var(--nv-bg-surface-soft)] text-[var(--nv-text-secondary)]'
  }
}