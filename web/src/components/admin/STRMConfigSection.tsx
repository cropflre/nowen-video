import { useEffect, useState } from 'react'
import {
  Radio,
  Globe,
  Clock,
  Zap,
  Film,
  Save,
  RotateCcw,
  Plus,
  Trash2,
  ChevronDown,
  ChevronUp,
  AlertCircle,
  CheckCircle2,
  Loader2,
} from 'lucide-react'
import { strmApi, type STRMGlobalConfig } from '@/api'
import { Button, Input, Surface } from '@/components/design-system'

export default function STRMConfigSection() {
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [cfg, setCfg] = useState<STRMGlobalConfig | null>(null)
  const [tip, setTip] = useState<{ kind: 'ok' | 'err'; msg: string } | null>(null)

  const defaultCfg: STRMGlobalConfig = {
    default_user_agent: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
    default_referer: '',
    connect_timeout: 30,
    rewrite_hls: true,
    remote_probe: true,
    remote_probe_timeout: 8,
    domain_user_agents: {},
    domain_referers: {},
  }

  const load = async () => {
    setLoading(true)
    try {
      const response = await strmApi.getConfig()
      setCfg({ ...defaultCfg, ...(response.data.data || {}) })
    } catch (error: unknown) {
      setTip({ kind: 'err', msg: error instanceof Error ? error.message : '加载配置失败' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open && !cfg) void load()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const save = async () => {
    if (!cfg) return
    if (cfg.connect_timeout < 0 || cfg.connect_timeout > 600) {
      setTip({ kind: 'err', msg: 'connect_timeout 必须在 0-600 秒之间' })
      return
    }
    if (cfg.remote_probe_timeout < 0 || cfg.remote_probe_timeout > 120) {
      setTip({ kind: 'err', msg: 'remote_probe_timeout 必须在 0-120 秒之间' })
      return
    }
    setSaving(true)
    try {
      const response = await strmApi.updateConfig(cfg)
      setCfg({ ...defaultCfg, ...(response.data.data || {}) })
      setTip({ kind: 'ok', msg: '配置已保存' })
      window.setTimeout(() => setTip(null), 2500)
    } catch (error: unknown) {
      setTip({ kind: 'err', msg: error instanceof Error ? error.message : '保存失败' })
    } finally {
      setSaving(false)
    }
  }

  const reset = () => setCfg({ ...defaultCfg })

  const addDomain = (target: 'ua' | 'ref') => {
    if (!cfg) return
    const key = `新域名-${Date.now()}`
    if (target === 'ua') setCfg({ ...cfg, domain_user_agents: { ...cfg.domain_user_agents, [key]: '' } })
    else setCfg({ ...cfg, domain_referers: { ...cfg.domain_referers, [key]: '' } })
  }

  const removeDomain = (target: 'ua' | 'ref', domain: string) => {
    if (!cfg) return
    if (target === 'ua') {
      const next = { ...cfg.domain_user_agents }
      delete next[domain]
      setCfg({ ...cfg, domain_user_agents: next })
    } else {
      const next = { ...cfg.domain_referers }
      delete next[domain]
      setCfg({ ...cfg, domain_referers: next })
    }
  }

  const renameDomain = (target: 'ua' | 'ref', oldKey: string, newKey: string) => {
    if (!cfg || oldKey === newKey || !newKey) return
    const source = target === 'ua' ? cfg.domain_user_agents : cfg.domain_referers
    if (source[newKey] !== undefined) return
    const next = { ...source }
    const value = next[oldKey] || ''
    delete next[oldKey]
    next[newKey] = value
    if (target === 'ua') setCfg({ ...cfg, domain_user_agents: next })
    else setCfg({ ...cfg, domain_referers: next })
  }

  const setDomainValue = (target: 'ua' | 'ref', domain: string, value: string) => {
    if (!cfg) return
    if (target === 'ua') setCfg({ ...cfg, domain_user_agents: { ...cfg.domain_user_agents, [domain]: value } })
    else setCfg({ ...cfg, domain_referers: { ...cfg.domain_referers, [domain]: value } })
  }

  return (
    <Surface className="p-4 md:p-5">
      <button
        type="button"
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-center gap-3 text-left"
        aria-expanded={open}
      >
        <div className="flex h-9 w-9 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
          <Radio size={18} aria-hidden="true" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="text-base font-semibold text-[var(--nv-text-primary)]">STRM 远程流配置</div>
          <div className="text-xs text-[var(--nv-text-tertiary)]">针对 .strm 云盘/CDN 远程流的统一代理、HLS 重写、FFprobe 探测与域名级白名单</div>
        </div>
        {open ? <ChevronUp size={18} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" /> : <ChevronDown size={18} className="text-[var(--nv-text-tertiary)]" aria-hidden="true" />}
      </button>

      {open && (
        <div className="mt-4 space-y-4 border-t border-[var(--nv-border-subtle)] pt-4">
          {loading || !cfg ? (
            <div className="flex items-center gap-2 py-6 text-sm text-[var(--nv-text-tertiary)]">
              <Loader2 size={14} className="animate-spin text-[var(--nv-action-primary)]" aria-hidden="true" />正在加载配置...
            </div>
          ) : (
            <>
              {tip && (
                <div
                  className="flex items-center gap-2 rounded-[var(--nv-radius-control)] border px-3 py-2 text-sm"
                  style={tip.kind === 'ok'
                    ? {
                        color: 'var(--nv-status-success)',
                        borderColor: 'color-mix(in srgb, var(--nv-status-success) 30%, transparent)',
                        background: 'color-mix(in srgb, var(--nv-status-success) 8%, transparent)',
                      }
                    : {
                        color: 'var(--nv-status-danger)',
                        borderColor: 'color-mix(in srgb, var(--nv-status-danger) 30%, transparent)',
                        background: 'color-mix(in srgb, var(--nv-status-danger) 8%, transparent)',
                      }}
                >
                  {tip.kind === 'ok' ? <CheckCircle2 size={14} aria-hidden="true" /> : <AlertCircle size={14} aria-hidden="true" />}
                  <span>{tip.msg}</span>
                </div>
              )}

              <div className="grid gap-3 md:grid-cols-2">
                <ConfigField icon={<Globe size={12} />} label="默认 User-Agent">
                  <Input value={cfg.default_user_agent} onChange={(event) => setCfg({ ...cfg, default_user_agent: event.target.value })} placeholder="Mozilla/5.0 ..." className="font-mono text-xs" />
                </ConfigField>
                <ConfigField icon={<Globe size={12} />} label="默认 Referer">
                  <Input value={cfg.default_referer} onChange={(event) => setCfg({ ...cfg, default_referer: event.target.value })} placeholder="https://example.com/" className="font-mono text-xs" />
                </ConfigField>
                <ConfigField icon={<Clock size={12} />} label="连接超时（秒）">
                  <Input type="number" min={0} max={600} value={cfg.connect_timeout} onChange={(event) => setCfg({ ...cfg, connect_timeout: parseInt(event.target.value) || 0 })} className="text-xs" />
                </ConfigField>
                <ConfigField icon={<Clock size={12} />} label="远程 FFprobe 超时（秒）">
                  <Input type="number" min={0} max={120} value={cfg.remote_probe_timeout} onChange={(event) => setCfg({ ...cfg, remote_probe_timeout: parseInt(event.target.value) || 0 })} className="text-xs" />
                </ConfigField>
              </div>

              <div className="grid gap-3 md:grid-cols-2">
                <Toggle icon={<Film size={14} />} label="HLS 主清单重写" hint="让分片走后端代理，统一继承媒体的 UA/Referer/Cookie（解决跨域/鉴权）" checked={cfg.rewrite_hls} onChange={(value) => setCfg({ ...cfg, rewrite_hls: value })} />
                <Toggle icon={<Zap size={14} />} label="扫描时远程 FFprobe 探测" hint="对 mp4/mkv 直链启用，可获取真实时长/分辨率/编码（耗时+1~3s/条）" checked={cfg.remote_probe} onChange={(value) => setCfg({ ...cfg, remote_probe: value })} />
              </div>

              <DomainTable title="域名级 User-Agent 白名单" hint="当远程 URL 的 host 命中某个域名时，自动应用对应 UA（Media 级自定义优先）" rows={cfg.domain_user_agents} onAdd={() => addDomain('ua')} onRemove={(domain) => removeDomain('ua', domain)} onRename={(oldKey, newKey) => renameDomain('ua', oldKey, newKey)} onSetValue={(domain, value) => setDomainValue('ua', domain, value)} placeholder="Mozilla/5.0 ..." />
              <DomainTable title="域名级 Referer 白名单" hint="匹配 host 时自动注入 Referer" rows={cfg.domain_referers} onAdd={() => addDomain('ref')} onRemove={(domain) => removeDomain('ref', domain)} onRename={(oldKey, newKey) => renameDomain('ref', oldKey, newKey)} onSetValue={(domain, value) => setDomainValue('ref', domain, value)} placeholder="https://example.com/" />

              <div className="flex flex-wrap items-center gap-2 pt-2">
                <Button type="button" variant="primary" size="sm" onClick={save} loading={saving} disabled={saving}>
                  {!saving && <Save size={12} aria-hidden="true" />}保存配置
                </Button>
                <Button type="button" variant="secondary" size="sm" onClick={reset}>
                  <RotateCcw size={12} aria-hidden="true" />重置为默认
                </Button>
                <span className="ml-auto text-[11px] text-[var(--nv-text-tertiary)]">Media 级 UA/Referer/Cookie 优先级高于全局；全局又高于域名白名单</span>
              </div>
            </>
          )}
        </div>
      )}
    </Surface>
  )
}

function ConfigField({ icon, label, children }: { icon: React.ReactNode; label: string; children: React.ReactNode }) {
  return (
    <div>
      <label className="mb-1 flex items-center gap-1.5 text-xs font-medium text-[var(--nv-text-secondary)]">{icon}{label}</label>
      {children}
    </div>
  )
}

interface ToggleProps {
  icon: React.ReactNode
  label: string
  hint?: string
  checked: boolean
  onChange: (value: boolean) => void
}

function Toggle({ icon, label, hint, checked, onChange }: ToggleProps) {
  return (
    <Surface className="flex items-center justify-between gap-3 bg-[var(--nv-bg-surface-soft)] p-3 shadow-none">
      <div>
        <div className="flex items-center gap-1.5 text-xs font-medium text-[var(--nv-text-primary)]">{icon}{label}</div>
        {hint && <div className="mt-0.5 text-[11px] leading-snug text-[var(--nv-text-tertiary)]">{hint}</div>}
      </div>
      <button
        type="button"
        onClick={() => onChange(!checked)}
        className={`relative h-6 w-10 shrink-0 rounded-full border transition-[background-color,border-color] ${checked ? 'border-[var(--nv-border-hover)] bg-[var(--nv-action-primary)]' : 'border-[var(--nv-border-default)] bg-[var(--nv-bg-control)]'}`}
        role="switch"
        aria-checked={checked}
      >
        <span className={`absolute top-0.5 h-4 w-4 rounded-full bg-white shadow-sm transition-transform ${checked ? 'translate-x-[18px]' : 'translate-x-0.5'}`} />
      </button>
    </Surface>
  )
}

interface DomainTableProps {
  title: string
  hint?: string
  rows: Record<string, string>
  onAdd: () => void
  onRemove: (domain: string) => void
  onRename: (oldKey: string, newKey: string) => void
  onSetValue: (domain: string, value: string) => void
  placeholder?: string
}

function DomainTable({ title, hint, rows, onAdd, onRemove, onRename, onSetValue, placeholder }: DomainTableProps) {
  const entries = Object.entries(rows)
  return (
    <Surface className="bg-[var(--nv-bg-surface-soft)] p-3 shadow-none">
      <div className="mb-2 flex items-center gap-3">
        <div className="min-w-0 flex-1">
          <div className="text-xs font-medium text-[var(--nv-text-primary)]">{title}</div>
          {hint && <div className="text-[11px] text-[var(--nv-text-tertiary)]">{hint}</div>}
        </div>
        <Button type="button" variant="secondary" size="sm" onClick={onAdd}>
          <Plus size={10} aria-hidden="true" />新增
        </Button>
      </div>

      {entries.length === 0 ? (
        <div className="py-2 text-center text-[11px] text-[var(--nv-text-tertiary)]">暂无规则</div>
      ) : (
        <div className="space-y-1.5">
          {entries.map(([domain, value]) => (
            <div key={domain} className="flex items-center gap-1.5">
              <Input defaultValue={domain} onBlur={(event) => onRename(domain, event.target.value.trim())} className="w-32 shrink-0 font-mono text-[11px]" placeholder="115.com" />
              <Input value={value} onChange={(event) => onSetValue(domain, event.target.value)} className="min-w-0 flex-1 font-mono text-[11px]" placeholder={placeholder} />
              <Button type="button" variant="danger" size="sm" iconOnly onClick={() => onRemove(domain)} title="删除" aria-label={`删除 ${domain}`}>
                <Trash2 size={12} aria-hidden="true" />
              </Button>
            </div>
          ))}
        </div>
      )}
    </Surface>
  )
}
