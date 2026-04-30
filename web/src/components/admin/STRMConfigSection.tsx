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

/**
 * STRM 全局配置区块
 *
 * 可嵌入到媒体库/影视文件管理页面，用于统一管理 .strm 远程流的默认请求头与行为。
 * 核心配置：
 *  - 默认 User-Agent / Referer（当 media 自身未指定时使用）
 *  - HLS 重写开关（让分片继承媒体的鉴权头）
 *  - 远程 FFprobe 开关（扫描时获取真实时长/分辨率）
 *  - 域名级白名单（针对 115/阿里云盘等特定源自动应用 UA/Referer）
 */
export default function STRMConfigSection() {
  const [open, setOpen] = useState(false)
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [cfg, setCfg] = useState<STRMGlobalConfig | null>(null)
  const [tip, setTip] = useState<{ kind: 'ok' | 'err'; msg: string } | null>(null)

  const defaultCfg: STRMGlobalConfig = {
    default_user_agent:
      'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36',
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
      const res = await strmApi.getConfig()
      setCfg({ ...defaultCfg, ...(res.data.data || {}) })
    } catch (e: unknown) {
      setTip({ kind: 'err', msg: e instanceof Error ? e.message : '加载配置失败' })
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (open && !cfg) {
      load()
    }
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
      const res = await strmApi.updateConfig(cfg)
      setCfg({ ...defaultCfg, ...(res.data.data || {}) })
      setTip({ kind: 'ok', msg: '配置已保存' })
      setTimeout(() => setTip(null), 2500)
    } catch (e: unknown) {
      setTip({ kind: 'err', msg: e instanceof Error ? e.message : '保存失败' })
    } finally {
      setSaving(false)
    }
  }

  const reset = () => setCfg({ ...defaultCfg })

  // ---- 域名表格操作 ----
  const addDomain = (target: 'ua' | 'ref') => {
    if (!cfg) return
    const key = `新域名-${Date.now()}`
    if (target === 'ua') {
      setCfg({ ...cfg, domain_user_agents: { ...cfg.domain_user_agents, [key]: '' } })
    } else {
      setCfg({ ...cfg, domain_referers: { ...cfg.domain_referers, [key]: '' } })
    }
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

  const renameDomain = (target: 'ua' | 'ref', oldK: string, newK: string) => {
    if (!cfg || oldK === newK || !newK) return
    const src = target === 'ua' ? cfg.domain_user_agents : cfg.domain_referers
    if (src[newK] !== undefined) return
    const value = src[oldK] || ''
    const next = { ...src }
    delete next[oldK]
    next[newK] = value
    if (target === 'ua') setCfg({ ...cfg, domain_user_agents: next })
    else setCfg({ ...cfg, domain_referers: next })
  }

  const setDomainValue = (target: 'ua' | 'ref', domain: string, v: string) => {
    if (!cfg) return
    if (target === 'ua') {
      setCfg({ ...cfg, domain_user_agents: { ...cfg.domain_user_agents, [domain]: v } })
    } else {
      setCfg({ ...cfg, domain_referers: { ...cfg.domain_referers, [domain]: v } })
    }
  }

  // ---- 样式（复用主题变量，兼容日夜模式） ----
  const cardStyle: React.CSSProperties = {
    background: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
  }
  const inputStyle: React.CSSProperties = {
    background: 'var(--bg-elevated)',
    border: '1px solid var(--border-default)',
    color: 'var(--text-primary)',
  }

  return (
    <div className="rounded-xl p-4 md:p-5" style={cardStyle}>
      {/* 头部：可折叠 */}
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className="flex w-full items-center gap-3 text-left"
      >
        <div
          className="flex h-9 w-9 items-center justify-center rounded-lg"
          style={{ background: 'var(--neon-blue-15)' }}
        >
          <Radio size={18} className="text-cyan-400" />
        </div>
        <div className="flex-1">
          <div className="text-base font-semibold" style={{ color: 'var(--text-primary)' }}>
            STRM 远程流配置
          </div>
          <div className="text-xs" style={{ color: 'var(--text-secondary)' }}>
            针对 .strm 云盘/CDN 远程流的统一代理、HLS 重写、FFprobe 探测与域名级白名单
          </div>
        </div>
        {open ? <ChevronUp size={18} /> : <ChevronDown size={18} />}
      </button>

      {open && (
        <div className="mt-4 space-y-4">
          {loading || !cfg ? (
            <div className="flex items-center gap-2 py-6 text-sm" style={{ color: 'var(--text-secondary)' }}>
              <Loader2 size={14} className="animate-spin" />
              正在加载配置...
            </div>
          ) : (
            <>
              {/* 提示条 */}
              {tip && (
                <div
                  className="flex items-center gap-2 rounded-md px-3 py-2 text-sm"
                  style={{
                    background:
                      tip.kind === 'ok' ? 'rgba(16,185,129,0.12)' : 'rgba(244,63,94,0.12)',
                    border:
                      '1px solid ' +
                      (tip.kind === 'ok' ? 'rgba(16,185,129,0.35)' : 'rgba(244,63,94,0.35)'),
                    color: tip.kind === 'ok' ? '#10b981' : '#f43f5e',
                  }}
                >
                  {tip.kind === 'ok' ? (
                    <CheckCircle2 size={14} />
                  ) : (
                    <AlertCircle size={14} />
                  )}
                  <span>{tip.msg}</span>
                </div>
              )}

              {/* 基础 Header */}
              <div className="grid gap-3 md:grid-cols-2">
                <div>
                  <label
                    className="mb-1 flex items-center gap-1.5 text-xs font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    <Globe size={12} /> 默认 User-Agent
                  </label>
                  <input
                    type="text"
                    className="w-full rounded-md px-2.5 py-1.5 text-xs focus:outline-none"
                    style={inputStyle}
                    value={cfg.default_user_agent}
                    onChange={(e) =>
                      setCfg({ ...cfg, default_user_agent: e.target.value })
                    }
                    placeholder="Mozilla/5.0 ..."
                  />
                </div>
                <div>
                  <label
                    className="mb-1 flex items-center gap-1.5 text-xs font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    <Globe size={12} /> 默认 Referer
                  </label>
                  <input
                    type="text"
                    className="w-full rounded-md px-2.5 py-1.5 text-xs focus:outline-none"
                    style={inputStyle}
                    value={cfg.default_referer}
                    onChange={(e) => setCfg({ ...cfg, default_referer: e.target.value })}
                    placeholder="https://example.com/"
                  />
                </div>
                <div>
                  <label
                    className="mb-1 flex items-center gap-1.5 text-xs font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    <Clock size={12} /> 连接超时（秒）
                  </label>
                  <input
                    type="number"
                    min={0}
                    max={600}
                    className="w-full rounded-md px-2.5 py-1.5 text-xs focus:outline-none"
                    style={inputStyle}
                    value={cfg.connect_timeout}
                    onChange={(e) =>
                      setCfg({ ...cfg, connect_timeout: parseInt(e.target.value) || 0 })
                    }
                  />
                </div>
                <div>
                  <label
                    className="mb-1 flex items-center gap-1.5 text-xs font-medium"
                    style={{ color: 'var(--text-secondary)' }}
                  >
                    <Clock size={12} /> 远程 FFprobe 超时（秒）
                  </label>
                  <input
                    type="number"
                    min={0}
                    max={120}
                    className="w-full rounded-md px-2.5 py-1.5 text-xs focus:outline-none"
                    style={inputStyle}
                    value={cfg.remote_probe_timeout}
                    onChange={(e) =>
                      setCfg({ ...cfg, remote_probe_timeout: parseInt(e.target.value) || 0 })
                    }
                  />
                </div>
              </div>

              {/* 开关 */}
              <div className="grid gap-3 md:grid-cols-2">
                <Toggle
                  icon={<Film size={14} />}
                  label="HLS 主清单重写"
                  hint="让分片走后端代理，统一继承媒体的 UA/Referer/Cookie（解决跨域/鉴权）"
                  checked={cfg.rewrite_hls}
                  onChange={(v) => setCfg({ ...cfg, rewrite_hls: v })}
                />
                <Toggle
                  icon={<Zap size={14} />}
                  label="扫描时远程 FFprobe 探测"
                  hint="对 mp4/mkv 直链启用，可获取真实时长/分辨率/编码（耗时+1~3s/条）"
                  checked={cfg.remote_probe}
                  onChange={(v) => setCfg({ ...cfg, remote_probe: v })}
                />
              </div>

              {/* 域名白名单 */}
              <DomainTable
                title="域名级 User-Agent 白名单"
                hint="当远程 URL 的 host 命中某个域名时，自动应用对应 UA（Media 级自定义优先）"
                rows={cfg.domain_user_agents}
                onAdd={() => addDomain('ua')}
                onRemove={(d) => removeDomain('ua', d)}
                onRename={(o, n) => renameDomain('ua', o, n)}
                onSetValue={(d, v) => setDomainValue('ua', d, v)}
                placeholder="Mozilla/5.0 ..."
              />
              <DomainTable
                title="域名级 Referer 白名单"
                hint="同上，匹配 host 时自动注入 Referer"
                rows={cfg.domain_referers}
                onAdd={() => addDomain('ref')}
                onRemove={(d) => removeDomain('ref', d)}
                onRename={(o, n) => renameDomain('ref', o, n)}
                onSetValue={(d, v) => setDomainValue('ref', d, v)}
                placeholder="https://example.com/"
              />

              {/* 底部按钮 */}
              <div className="flex items-center gap-2 pt-2">
                <button
                  type="button"
                  onClick={save}
                  disabled={saving}
                  className="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium text-white disabled:opacity-60"
                  style={{ background: 'var(--neon-blue)' }}
                >
                  {saving ? <Loader2 size={12} className="animate-spin" /> : <Save size={12} />}
                  保存配置
                </button>
                <button
                  type="button"
                  onClick={reset}
                  className="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs"
                  style={{
                    background: 'var(--bg-elevated)',
                    border: '1px solid var(--border-default)',
                    color: 'var(--text-primary)',
                  }}
                >
                  <RotateCcw size={12} />
                  重置为默认
                </button>
                <span className="ml-auto text-[11px]" style={{ color: 'var(--text-secondary)' }}>
                  提示：Media 级 UA/Referer/Cookie 优先级高于全局；全局又高于域名白名单
                </span>
              </div>
            </>
          )}
        </div>
      )}
    </div>
  )
}

// ---------- 子组件 ----------

interface ToggleProps {
  icon: React.ReactNode
  label: string
  hint?: string
  checked: boolean
  onChange: (v: boolean) => void
}

function Toggle({ icon, label, hint, checked, onChange }: ToggleProps) {
  return (
    <div
      className="flex items-center justify-between gap-3 rounded-md p-2.5"
      style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}
    >
      <div>
        <div
          className="flex items-center gap-1.5 text-xs font-medium"
          style={{ color: 'var(--text-primary)' }}
        >
          {icon}
          {label}
        </div>
        {hint && (
          <div className="mt-0.5 text-[11px] leading-snug" style={{ color: 'var(--text-secondary)' }}>
            {hint}
          </div>
        )}
      </div>
      <button
        type="button"
        onClick={() => onChange(!checked)}
        className="relative h-5 w-9 flex-shrink-0 rounded-full transition-colors"
        style={{
          background: checked ? 'var(--neon-blue)' : 'var(--border-default)',
        }}
      >
        <span
          className="absolute top-0.5 h-4 w-4 rounded-full bg-white transition-transform"
          style={{ left: checked ? 'calc(100% - 18px)' : '2px' }}
        />
      </button>
    </div>
  )
}

interface DomainTableProps {
  title: string
  hint?: string
  rows: Record<string, string>
  onAdd: () => void
  onRemove: (domain: string) => void
  onRename: (oldK: string, newK: string) => void
  onSetValue: (domain: string, v: string) => void
  placeholder?: string
}

function DomainTable({
  title,
  hint,
  rows,
  onAdd,
  onRemove,
  onRename,
  onSetValue,
  placeholder,
}: DomainTableProps) {
  const entries = Object.entries(rows)
  return (
    <div
      className="rounded-md p-3"
      style={{ background: 'var(--bg-elevated)', border: '1px solid var(--border-default)' }}
    >
      <div className="mb-2 flex items-center">
        <div>
          <div className="text-xs font-medium" style={{ color: 'var(--text-primary)' }}>
            {title}
          </div>
          {hint && (
            <div className="text-[11px]" style={{ color: 'var(--text-secondary)' }}>
              {hint}
            </div>
          )}
        </div>
        <button
          type="button"
          onClick={onAdd}
          className="ml-auto inline-flex items-center gap-1 rounded px-2 py-1 text-[11px]"
          style={{ background: 'var(--neon-blue-15)', color: '#22d3ee' }}
        >
          <Plus size={10} /> 新增
        </button>
      </div>
      {entries.length === 0 ? (
        <div className="py-2 text-center text-[11px]" style={{ color: 'var(--text-secondary)' }}>
          暂无规则
        </div>
      ) : (
        <div className="space-y-1.5">
          {entries.map(([domain, value]) => (
            <div key={domain} className="flex items-center gap-1.5">
              <input
                type="text"
                defaultValue={domain}
                onBlur={(e) => onRename(domain, e.target.value.trim())}
                className="w-32 flex-shrink-0 rounded-md px-2 py-1 text-[11px] focus:outline-none"
                style={{
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                }}
                placeholder="115.com"
              />
              <input
                type="text"
                value={value}
                onChange={(e) => onSetValue(domain, e.target.value)}
                className="flex-1 rounded-md px-2 py-1 text-[11px] focus:outline-none"
                style={{
                  background: 'var(--bg-primary)',
                  border: '1px solid var(--border-default)',
                  color: 'var(--text-primary)',
                }}
                placeholder={placeholder}
              />
              <button
                type="button"
                onClick={() => onRemove(domain)}
                className="inline-flex h-7 w-7 items-center justify-center rounded-md text-rose-400 hover:text-rose-300"
                style={{ background: 'var(--bg-primary)', border: '1px solid var(--border-default)' }}
                title="删除"
              >
                <Trash2 size={12} />
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
