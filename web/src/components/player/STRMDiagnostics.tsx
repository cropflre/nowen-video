import { useState, useEffect } from 'react'
import { streamApi, strmApi, type MediaSTRMInfo } from '@/api'
import {
  Loader2,
  Activity,
  CheckCircle2,
  XCircle,
  ChevronDown,
  ChevronUp,
  Settings2,
  X,
  Save,
} from 'lucide-react'

interface STRMDiagnosticsProps {
  mediaId: string
  compact?: boolean
}

type CheckResult = {
  media_id: string
  url: string
  status_code: number
  ok: boolean
  content_type?: string
  content_length?: number
  accept_ranges?: string
  response_ms: number
  error?: string
  effective_url?: string
  headers?: Record<string, string>
}

export default function STRMDiagnostics({ mediaId, compact = false }: STRMDiagnosticsProps) {
  const [loading, setLoading] = useState(false)
  const [open, setOpen] = useState(false)
  const [result, setResult] = useState<CheckResult | null>(null)
  const [editorOpen, setEditorOpen] = useState(false)

  const runCheck = async () => {
    setLoading(true)
    try {
      const res = await streamApi.checkSTRM(mediaId)
      setResult(res.data.data)
      setOpen(true)
    } catch (e: unknown) {
      const msg = e instanceof Error ? e.message : '诊断请求失败'
      setResult({ media_id: mediaId, url: '', status_code: 0, ok: false, response_ms: 0, error: msg })
      setOpen(true)
    } finally {
      setLoading(false)
    }
  }

  const copyDiag = () => {
    if (!result) return
    const text = [
      'STRM 诊断报告',
      `时间: ${new Date().toISOString()}`,
      `Media: ${result.media_id}`,
      `URL: ${result.url || '-'}`,
      `状态: ${result.ok ? 'OK' : 'FAIL'}  HTTP ${result.status_code}`,
      `耗时: ${result.response_ms}ms`,
      `Content-Type: ${result.content_type || '-'}`,
      `Content-Length: ${result.content_length ?? '-'}`,
      `Accept-Ranges: ${result.accept_ranges || '-'}`,
      result.effective_url ? `最终 URL: ${result.effective_url}` : '',
      result.error ? `错误: ${result.error}` : '',
      result.headers
        ? `响应头:\n${Object.entries(result.headers).map(([key, value]) => `  ${key}: ${value}`).join('\n')}`
        : '',
    ].filter(Boolean).join('\n')
    navigator.clipboard?.writeText(text).catch(() => {})
  }

  return (
    <div className={compact ? 'inline-flex' : 'flex flex-col gap-2'}>
      <button
        type="button"
        onClick={runCheck}
        disabled={loading}
        className="inline-flex items-center gap-1.5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-2.5 py-1 text-xs text-[var(--nv-player-text-secondary)] transition-[background-color,border-color,color] hover:border-[var(--nv-player-border-hover)] hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)] disabled:opacity-60"
        title="一键诊断远程流链路"
      >
        {loading ? <Loader2 size={12} className="animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" /> : <Activity size={12} aria-hidden="true" />}
        <span>STRM 诊断</span>
        {result ? (open ? <ChevronUp size={12} aria-hidden="true" /> : <ChevronDown size={12} aria-hidden="true" />) : null}
      </button>

      {open && result && (
        <div className="mt-1 max-w-[360px] rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] p-2.5 text-[11px] leading-relaxed text-[var(--nv-player-text-secondary)] shadow-[var(--nv-player-shadow)] backdrop-blur-xl">
          <div className="mb-1.5 flex items-center gap-1.5">
            {result.ok ? (
              <><CheckCircle2 size={14} className="text-[var(--nv-player-success)]" aria-hidden="true" /><span className="font-medium text-[var(--nv-player-success)]">连通正常</span></>
            ) : (
              <><XCircle size={14} className="text-[var(--nv-player-danger)]" aria-hidden="true" /><span className="font-medium text-[var(--nv-player-danger)]">连通异常</span></>
            )}
            <span className="ml-auto text-[var(--nv-player-text-tertiary)]">{result.response_ms}ms</span>
          </div>

          <div className="space-y-0.5 font-mono">
            <div><span className="text-[var(--nv-player-text-tertiary)]">HTTP:</span> {result.status_code || '-'}</div>
            {result.content_type && <div className="truncate"><span className="text-[var(--nv-player-text-tertiary)]">CT:</span> {result.content_type}</div>}
            {typeof result.content_length === 'number' && result.content_length > 0 && (
              <div><span className="text-[var(--nv-player-text-tertiary)]">Size:</span> {(result.content_length / 1024 / 1024).toFixed(2)} MB</div>
            )}
            {result.accept_ranges && <div><span className="text-[var(--nv-player-text-tertiary)]">Range:</span> {result.accept_ranges}</div>}
            {result.error && <div className="mt-1 break-words text-[var(--nv-player-danger)]"><span className="text-[var(--nv-player-text-tertiary)]">Error:</span> {result.error}</div>}
            {result.url && <div className="mt-1 break-all text-[var(--nv-player-text-tertiary)]">{result.url.length > 80 ? `${result.url.slice(0, 80)}…` : result.url}</div>}
          </div>

          <div className="mt-2 flex flex-wrap gap-1.5">
            <button type="button" onClick={copyDiag} className="flex-1 rounded-[var(--nv-radius-sm)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] px-2 py-1 text-[10px] text-[var(--nv-player-accent)] transition-colors hover:bg-[var(--nv-player-accent-soft-hover)]">复制诊断信息</button>
            <button type="button" onClick={runCheck} className="flex-1 rounded-[var(--nv-radius-sm)] bg-[var(--nv-player-surface-subtle)] px-2 py-1 text-[10px] text-[var(--nv-player-text-secondary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]">重试</button>
            <button type="button" onClick={() => setEditorOpen(true)} className="inline-flex items-center gap-1 rounded-[var(--nv-radius-sm)] bg-[var(--nv-player-surface-subtle)] px-2 py-1 text-[10px] text-[var(--nv-player-text-secondary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]" title="手动覆盖 UA / Referer / Cookie（会立即生效）">
              <Settings2 size={10} aria-hidden="true" /> 编辑请求头
            </button>
          </div>
        </div>
      )}

      {editorOpen && (
        <STRMHeaderEditor
          mediaId={mediaId}
          onClose={() => setEditorOpen(false)}
          onSaved={() => {
            setEditorOpen(false)
            void runCheck()
          }}
        />
      )}
    </div>
  )
}

interface EditorProps {
  mediaId: string
  onClose: () => void
  onSaved: () => void
}

function STRMHeaderEditor({ mediaId, onClose, onSaved }: EditorProps) {
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)
  const [info, setInfo] = useState<MediaSTRMInfo | null>(null)
  const [ua, setUA] = useState('')
  const [referer, setReferer] = useState('')
  const [cookie, setCookie] = useState('')
  const [url, setURL] = useState('')
  const [headersText, setHeadersText] = useState('')

  useEffect(() => {
    let alive = true
    ;(async () => {
      try {
        const res = await strmApi.getMediaSTRM(mediaId)
        if (!alive) return
        const data = res.data.data
        setInfo(data)
        setUA(data.stream_ua || '')
        setReferer(data.stream_referer || '')
        setCookie(data.stream_cookie || '')
        setURL(data.stream_url || '')
        setHeadersText(data.stream_headers && Object.keys(data.stream_headers).length > 0
          ? Object.entries(data.stream_headers).map(([key, value]) => `${key}: ${value}`).join('\n')
          : '')
      } catch (e) {
        if (alive) setErr(e instanceof Error ? e.message : '加载失败')
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => { alive = false }
  }, [mediaId])

  const parseHeaders = (text: string): Record<string, string> => {
    const out: Record<string, string> = {}
    for (const raw of text.split(/\r?\n/)) {
      const line = raw.trim()
      if (!line || line.startsWith('#')) continue
      const index = line.indexOf(':')
      if (index <= 0) continue
      const key = line.slice(0, index).trim()
      const value = line.slice(index + 1).trim()
      if (key) out[key] = value
    }
    return out
  }

  const save = async () => {
    setSaving(true)
    setErr(null)
    try {
      const headers = parseHeaders(headersText)
      await strmApi.updateMediaSTRM(mediaId, {
        stream_url: url.trim() || undefined,
        user_agent: ua,
        referer,
        cookie,
        headers,
        clear_headers: Object.keys(headers).length === 0 && headersText.trim() === '',
      })
      onSaved()
    } catch (e) {
      setErr(e instanceof Error ? e.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-[color-mix(in_srgb,var(--nv-player-canvas)_65%,transparent)] p-4 backdrop-blur-sm"
      onClick={(event) => { if (event.target === event.currentTarget) onClose() }}
    >
      <div className="w-full max-w-lg rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] p-5 text-[var(--nv-player-text-primary)] shadow-[var(--nv-player-shadow)]">
        <div className="mb-3 flex items-center justify-between gap-3">
          <div>
            <div className="text-sm font-semibold">STRM 请求头覆写</div>
            <div className="text-[11px] text-[var(--nv-player-text-tertiary)]">只影响当前这条媒体；粘贴后立即生效，不需重新扫描</div>
          </div>
          <button type="button" onClick={onClose} className="flex h-8 w-8 items-center justify-center rounded-[var(--nv-player-radius-control)] text-[var(--nv-player-text-tertiary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]" title="关闭" aria-label="关闭请求头编辑"><X size={16} aria-hidden="true" /></button>
        </div>

        {loading ? (
          <div className="flex items-center gap-2 py-8 text-xs text-[var(--nv-player-text-tertiary)]"><Loader2 size={12} className="animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" /> 加载中...</div>
        ) : (
          <div className="space-y-2.5 text-xs">
            {info && !info.is_strm && <div className="rounded-[var(--nv-radius-sm)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-danger-soft)] px-2 py-1.5 text-[11px] text-[var(--nv-player-danger)]">当前媒体不是 STRM 远程流，覆写无效</div>}
            <LabeledInput label="远程 URL (可选，token 过期时手动刷新)" value={url} onChange={setURL} placeholder="https://..." />
            <LabeledInput label="User-Agent" value={ua} onChange={setUA} placeholder="Mozilla/5.0 ..." />
            <LabeledInput label="Referer" value={referer} onChange={setReferer} placeholder="https://example.com/" />
            <LabeledInput label="Cookie" value={cookie} onChange={setCookie} placeholder="sid=xxx; uid=yyy" textarea />
            <LabeledInput label="额外 Header（每行 Key: Value）" value={headersText} onChange={setHeadersText} placeholder={'X-Auth: secret-token\nAccept: */*'} textarea rows={4} />

            {err && <div className="rounded-[var(--nv-radius-sm)] border border-[var(--nv-player-danger-border)] bg-[var(--nv-player-danger-soft)] px-2 py-1.5 text-[11px] text-[var(--nv-player-danger)]">{err}</div>}

            <div className="flex items-center gap-2 pt-1">
              <button type="button" onClick={save} disabled={saving} className="inline-flex items-center gap-1.5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent)] px-3 py-1.5 text-xs font-medium text-[var(--nv-player-text-on-accent)] transition-colors hover:bg-[var(--nv-action-primary-hover)] disabled:opacity-60">
                {saving ? <Loader2 size={12} className="animate-spin" aria-hidden="true" /> : <Save size={12} aria-hidden="true" />}保存并重试
              </button>
              <button type="button" onClick={onClose} className="rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-3 py-1.5 text-xs text-[var(--nv-player-text-secondary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]">取消</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}

interface LabeledInputProps {
  label: string
  value: string
  onChange: (value: string) => void
  placeholder?: string
  textarea?: boolean
  rows?: number
}

function LabeledInput({ label, value, onChange, placeholder, textarea, rows }: LabeledInputProps) {
  const common = {
    value,
    onChange: (event: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => onChange(event.target.value),
    placeholder,
    className: 'w-full rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-2.5 py-1.5 font-mono text-xs text-[var(--nv-player-text-primary)] outline-none transition-[background-color,border-color,box-shadow] placeholder:text-[var(--nv-player-text-faint)] focus:border-[var(--nv-player-border-hover)] focus:bg-[var(--nv-player-surface)] focus:shadow-[0_0_0_3px_var(--nv-player-accent-soft)]',
  }
  return (
    <div>
      <label className="mb-1 block text-[11px] font-medium text-[var(--nv-player-text-tertiary)]">{label}</label>
      {textarea ? <textarea rows={rows || 2} {...common} /> : <input type="text" {...common} />}
    </div>
  )
}
