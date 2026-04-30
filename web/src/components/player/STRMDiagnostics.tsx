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

/**
 * STRM 远程流诊断面板
 * - 播放器右上角 STRM 标识旁使用，一键检测链路
 * - 展示状态码、响应头、响应耗时
 * - 支持复制排错信息到剪贴板
 */
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
      setResult({
        media_id: mediaId,
        url: '',
        status_code: 0,
        ok: false,
        response_ms: 0,
        error: msg,
      })
      setOpen(true)
    } finally {
      setLoading(false)
    }
  }

  const copyDiag = () => {
    if (!result) return
    const text = [
      `STRM 诊断报告`,
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
        ? `响应头:\n${Object.entries(result.headers)
            .map(([k, v]) => `  ${k}: ${v}`)
            .join('\n')}`
        : '',
    ]
      .filter(Boolean)
      .join('\n')
    navigator.clipboard?.writeText(text).catch(() => {})
  }

  return (
    <div className={compact ? 'inline-flex' : 'flex flex-col gap-2'}>
      <button
        onClick={runCheck}
        disabled={loading}
        className="inline-flex items-center gap-1.5 rounded-md px-2.5 py-1 text-xs transition-colors disabled:opacity-60"
        style={{
          background: 'rgba(0,0,0,0.55)',
          border: '1px solid var(--neon-blue-15)',
          color: '#e5e7eb',
        }}
        title="一键诊断远程流链路"
      >
        {loading ? (
          <Loader2 size={12} className="animate-spin" />
        ) : (
          <Activity size={12} />
        )}
        <span>STRM 诊断</span>
        {result ? (open ? <ChevronUp size={12} /> : <ChevronDown size={12} />) : null}
      </button>

      {open && result && (
        <div
          className="mt-1 max-w-[360px] rounded-md p-2.5 text-[11px] leading-relaxed text-gray-200 shadow-lg"
          style={{ background: 'rgba(0,0,0,0.85)', border: '1px solid var(--neon-blue-15)' }}
        >
          <div className="mb-1.5 flex items-center gap-1.5">
            {result.ok ? (
              <>
                <CheckCircle2 size={14} className="text-emerald-400" />
                <span className="font-medium text-emerald-400">连通正常</span>
              </>
            ) : (
              <>
                <XCircle size={14} className="text-rose-400" />
                <span className="font-medium text-rose-400">连通异常</span>
              </>
            )}
            <span className="ml-auto text-gray-400">{result.response_ms}ms</span>
          </div>

          <div className="space-y-0.5 font-mono">
            <div>
              <span className="text-gray-400">HTTP:</span> {result.status_code || '-'}
            </div>
            {result.content_type && (
              <div className="truncate">
                <span className="text-gray-400">CT:</span> {result.content_type}
              </div>
            )}
            {typeof result.content_length === 'number' && result.content_length > 0 && (
              <div>
                <span className="text-gray-400">Size:</span>{' '}
                {(result.content_length / 1024 / 1024).toFixed(2)} MB
              </div>
            )}
            {result.accept_ranges && (
              <div>
                <span className="text-gray-400">Range:</span> {result.accept_ranges}
              </div>
            )}
            {result.error && (
              <div className="mt-1 break-words text-rose-400">
                <span className="text-gray-400">Error:</span> {result.error}
              </div>
            )}
            {result.url && (
              <div className="mt-1 break-all text-gray-400">
                {result.url.length > 80 ? result.url.slice(0, 80) + '…' : result.url}
              </div>
            )}
          </div>

          <div className="mt-2 flex flex-wrap gap-1.5">
            <button
              onClick={copyDiag}
              className="flex-1 rounded px-2 py-1 text-[10px] transition-colors"
              style={{
                background: 'var(--neon-blue-15)',
                color: '#e5e7eb',
              }}
            >
              复制诊断信息
            </button>
            <button
              onClick={runCheck}
              className="flex-1 rounded px-2 py-1 text-[10px] transition-colors"
              style={{
                background: 'rgba(255,255,255,0.05)',
                color: '#e5e7eb',
              }}
            >
              重试
            </button>
            <button
              onClick={() => setEditorOpen(true)}
              className="inline-flex items-center gap-1 rounded px-2 py-1 text-[10px] transition-colors"
              style={{
                background: 'rgba(255,255,255,0.05)',
                color: '#e5e7eb',
              }}
              title="手动覆盖 UA / Referer / Cookie（会立即生效）"
            >
              <Settings2 size={10} /> 编辑请求头
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
            // 保存后自动跑一次诊断
            void runCheck()
          }}
        />
      )}
    </div>
  )
}

// ===================== 单条 Media 请求头覆写弹窗 =====================

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
        const d = res.data.data
        setInfo(d)
        setUA(d.stream_ua || '')
        setReferer(d.stream_referer || '')
        setCookie(d.stream_cookie || '')
        setURL(d.stream_url || '')
        if (d.stream_headers && Object.keys(d.stream_headers).length > 0) {
          setHeadersText(
            Object.entries(d.stream_headers)
              .map(([k, v]) => `${k}: ${v}`)
              .join('\n'),
          )
        } else {
          setHeadersText('')
        }
      } catch (e) {
        if (alive) setErr(e instanceof Error ? e.message : '加载失败')
      } finally {
        if (alive) setLoading(false)
      }
    })()
    return () => {
      alive = false
    }
  }, [mediaId])

  const parseHeaders = (text: string): Record<string, string> => {
    const out: Record<string, string> = {}
    for (const raw of text.split(/\r?\n/)) {
      const line = raw.trim()
      if (!line || line.startsWith('#')) continue
      const idx = line.indexOf(':')
      if (idx <= 0) continue
      const k = line.slice(0, idx).trim()
      const v = line.slice(idx + 1).trim()
      if (k) out[k] = v
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
      className="fixed inset-0 z-[9999] flex items-center justify-center p-4"
      style={{ background: 'rgba(0,0,0,0.65)' }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        className="w-full max-w-lg rounded-xl p-5 shadow-2xl"
        style={{
          background: 'var(--bg-elevated, #1a1b2e)',
          border: '1px solid var(--border-default, rgba(255,255,255,0.1))',
          color: 'var(--text-primary, #e5e7eb)',
        }}
      >
        <div className="mb-3 flex items-center justify-between">
          <div>
            <div className="text-sm font-semibold">STRM 请求头覆写</div>
            <div className="text-[11px]" style={{ color: 'var(--text-secondary, #9ca3af)' }}>
              只影响当前这条媒体；粘贴后立即生效，不需重新扫描
            </div>
          </div>
          <button onClick={onClose} title="关闭">
            <X size={16} />
          </button>
        </div>

        {loading ? (
          <div className="flex items-center gap-2 py-8 text-xs text-gray-400">
            <Loader2 size={12} className="animate-spin" /> 加载中...
          </div>
        ) : (
          <div className="space-y-2.5 text-xs">
            {info && !info.is_strm && (
              <div className="rounded-md px-2 py-1.5 text-[11px]" style={{ background: 'rgba(244,63,94,0.12)', color: '#f43f5e' }}>
                当前媒体不是 STRM 远程流，覆写无效
              </div>
            )}
            <LabeledInput label="远程 URL (可选，token 过期时手动刷新)" value={url} onChange={setURL} placeholder="https://..." />
            <LabeledInput label="User-Agent" value={ua} onChange={setUA} placeholder="Mozilla/5.0 ..." />
            <LabeledInput label="Referer" value={referer} onChange={setReferer} placeholder="https://example.com/" />
            <LabeledInput label="Cookie" value={cookie} onChange={setCookie} placeholder="sid=xxx; uid=yyy" textarea />
            <LabeledInput
              label="额外 Header（每行 Key: Value）"
              value={headersText}
              onChange={setHeadersText}
              placeholder={'X-Auth: secret-token\nAccept: */*'}
              textarea
              rows={4}
            />

            {err && (
              <div className="rounded-md px-2 py-1.5 text-[11px]" style={{ background: 'rgba(244,63,94,0.12)', color: '#f43f5e' }}>
                {err}
              </div>
            )}

            <div className="flex items-center gap-2 pt-1">
              <button
                onClick={save}
                disabled={saving}
                className="inline-flex items-center gap-1.5 rounded-md px-3 py-1.5 text-xs font-medium text-white disabled:opacity-60"
                style={{ background: 'var(--neon-blue, #0ea5e9)' }}
              >
                {saving ? <Loader2 size={12} className="animate-spin" /> : <Save size={12} />}
                保存并重试
              </button>
              <button
                onClick={onClose}
                className="rounded-md px-3 py-1.5 text-xs"
                style={{ background: 'rgba(255,255,255,0.06)', color: 'var(--text-primary, #e5e7eb)' }}
              >
                取消
              </button>
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
  onChange: (v: string) => void
  placeholder?: string
  textarea?: boolean
  rows?: number
}

function LabeledInput({ label, value, onChange, placeholder, textarea, rows }: LabeledInputProps) {
  const common = {
    value,
    onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement>) => onChange(e.target.value),
    placeholder,
    className:
      'w-full rounded-md px-2.5 py-1.5 text-xs focus:outline-none font-mono',
    style: {
      background: 'var(--bg-primary, rgba(255,255,255,0.04))',
      border: '1px solid var(--border-default, rgba(255,255,255,0.1))',
      color: 'var(--text-primary, #e5e7eb)',
    } as React.CSSProperties,
  }
  return (
    <div>
      <label className="mb-1 block text-[11px] font-medium" style={{ color: 'var(--text-secondary, #9ca3af)' }}>
        {label}
      </label>
      {textarea ? <textarea rows={rows || 2} {...common} /> : <input type="text" {...common} />}
    </div>
  )
}
