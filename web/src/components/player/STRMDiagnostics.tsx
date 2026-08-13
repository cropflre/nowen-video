import { useEffect, useState } from 'react'
import { streamApi, strmApi, type MediaSTRMInfo } from '@/api'
import {
  Activity,
  CheckCircle2,
  ChevronDown,
  ChevronUp,
  Loader2,
  Save,
  Settings2,
  XCircle,
} from 'lucide-react'
import {
  Button,
  Input,
  Modal,
  ModalBody,
  ModalFooter,
  ModalHeader,
  Textarea,
} from '@/components/design-system'

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
      <Button
        type="button"
        variant="ghost"
        size="sm"
        onClick={runCheck}
        disabled={loading}
        title="一键诊断远程流链路"
        className="text-[var(--nv-player-text-secondary)]"
      >
        {loading ? <Loader2 size={12} className="animate-spin motion-reduce:animate-none" aria-hidden="true" /> : <Activity size={12} aria-hidden="true" />}
        <span>STRM 诊断</span>
        {result ? (open ? <ChevronUp size={12} aria-hidden="true" /> : <ChevronDown size={12} aria-hidden="true" />) : null}
      </Button>

      {open && result && (
        <div className="mt-1 max-w-[360px] rounded-[var(--nv-radius-popover)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-elevated)] p-3 text-[11px] leading-relaxed text-[var(--nv-text-secondary)] shadow-[var(--nv-shadow-elevated)]">
          <div className="mb-2 flex items-center gap-1.5">
            {result.ok ? (
              <><CheckCircle2 size={14} className="text-[var(--nv-status-success)]" aria-hidden="true" /><span className="font-medium text-[var(--nv-status-success)]">连通正常</span></>
            ) : (
              <><XCircle size={14} className="text-[var(--nv-status-danger)]" aria-hidden="true" /><span className="font-medium text-[var(--nv-status-danger)]">连通异常</span></>
            )}
            <span className="ml-auto text-[var(--nv-text-tertiary)]">{result.response_ms}ms</span>
          </div>

          <div className="space-y-0.5 font-mono">
            <div><span className="text-[var(--nv-text-tertiary)]">HTTP:</span> {result.status_code || '-'}</div>
            {result.content_type && <div className="truncate"><span className="text-[var(--nv-text-tertiary)]">CT:</span> {result.content_type}</div>}
            {typeof result.content_length === 'number' && result.content_length > 0 && (
              <div><span className="text-[var(--nv-text-tertiary)]">Size:</span> {(result.content_length / 1024 / 1024).toFixed(2)} MB</div>
            )}
            {result.accept_ranges && <div><span className="text-[var(--nv-text-tertiary)]">Range:</span> {result.accept_ranges}</div>}
            {result.error && <div className="mt-1 break-words text-[var(--nv-status-danger)]"><span className="text-[var(--nv-text-tertiary)]">Error:</span> {result.error}</div>}
            {result.url && <div className="mt-1 break-all text-[var(--nv-text-tertiary)]">{result.url.length > 80 ? `${result.url.slice(0, 80)}…` : result.url}</div>}
          </div>

          <div className="mt-2.5 flex flex-wrap items-center gap-1">
            <Button type="button" variant="ghost" size="sm" onClick={copyDiag}>复制诊断信息</Button>
            <Button type="button" variant="ghost" size="sm" onClick={runCheck}>重试</Button>
            <Button type="button" variant="ghost" size="sm" onClick={() => setEditorOpen(true)} title="手动覆盖 UA / Referer / Cookie（会立即生效）">
              <Settings2 size={12} aria-hidden="true" /> 编辑请求头
            </Button>
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
    <Modal open onClose={onClose} size="md" ariaLabel="STRM 请求头覆写">
      <div className="flex min-h-0 flex-1 flex-col">
        <ModalHeader
          title="STRM 请求头覆写"
          description="只影响当前媒体；保存后立即生效，不需要重新扫描。"
          icon={<Settings2 size={18} aria-hidden="true" />}
          onClose={onClose}
        />

        <ModalBody>
          {loading ? (
            <div className="flex min-h-40 items-center justify-center gap-2 text-sm text-[var(--nv-text-tertiary)]">
              <Loader2 size={16} className="animate-spin motion-reduce:animate-none" aria-hidden="true" /> 加载中...
            </div>
          ) : (
            <div className="space-y-4">
              {info && !info.is_strm && (
                <div className="border-y border-[color-mix(in_srgb,var(--nv-status-danger)_24%,transparent)] py-2 text-xs text-[var(--nv-status-danger)]">
                  当前媒体不是 STRM 远程流，覆写不会生效。
                </div>
              )}

              <Field label="远程 URL（可选，token 过期时手动刷新）">
                <Input value={url} onChange={(event) => setURL(event.target.value)} placeholder="https://..." className="font-mono" />
              </Field>
              <Field label="User-Agent">
                <Input value={ua} onChange={(event) => setUA(event.target.value)} placeholder="Mozilla/5.0 ..." className="font-mono" />
              </Field>
              <Field label="Referer">
                <Input value={referer} onChange={(event) => setReferer(event.target.value)} placeholder="https://example.com/" className="font-mono" />
              </Field>
              <Field label="Cookie">
                <Textarea value={cookie} onChange={(event) => setCookie(event.target.value)} placeholder="sid=xxx; uid=yyy" rows={2} className="font-mono" />
              </Field>
              <Field label="额外 Header（每行 Key: Value）">
                <Textarea value={headersText} onChange={(event) => setHeadersText(event.target.value)} placeholder={'X-Auth: secret-token\nAccept: */*'} rows={4} className="font-mono" />
              </Field>

              {err && (
                <div className="border-y border-[color-mix(in_srgb,var(--nv-status-danger)_24%,transparent)] py-2 text-xs text-[var(--nv-status-danger)]">
                  {err}
                </div>
              )}
            </div>
          )}
        </ModalBody>

        <ModalFooter>
          <Button type="button" variant="ghost" onClick={onClose} disabled={saving}>取消</Button>
          <Button type="button" variant="primary" onClick={save} disabled={loading || saving} loading={saving}>
            {saving ? <Loader2 size={14} className="animate-spin motion-reduce:animate-none" aria-hidden="true" /> : <Save size={14} aria-hidden="true" />}
            保存并重试
          </Button>
        </ModalFooter>
      </div>
    </Modal>
  )
}

function Field({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className="block space-y-1.5">
      <span className="text-xs font-medium text-[var(--nv-text-secondary)]">{label}</span>
      {children}
    </label>
  )
}
