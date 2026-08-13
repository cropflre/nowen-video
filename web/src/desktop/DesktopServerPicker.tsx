/**
 * DesktopServerPicker —— 桌面端首次启动“服务器地址”引导。
 * 探活、localStorage 写入与 reload 行为保持不变，仅统一视觉语义。
 */
import { useEffect, useState, useCallback } from 'react'
import { Server, Wifi } from 'lucide-react'
import { desktop } from './bridge'
import { Button, Input, Surface } from '@/components/design-system'

const LS_KEY = 'nowen_server_url'
const DEFAULT_CANDIDATES = [
  'http://127.0.0.1:21114',
  'http://127.0.0.1:8080',
]

async function probe(base: string, timeoutMs = 1500): Promise<boolean> {
  try {
    const controller = new AbortController()
    const timer = window.setTimeout(() => controller.abort(), timeoutMs)
    const url = `${base.replace(/\/+$/, '')}/api/health`
    const response = await fetch(url, { signal: controller.signal, cache: 'no-store' })
    window.clearTimeout(timer)
    return response.ok
  } catch {
    return false
  }
}

export default function DesktopServerPicker() {
  const [need, setNeed] = useState(false)
  const [input, setInput] = useState('http://127.0.0.1:8080')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!desktop.isDesktop) return
    let cancelled = false

    ;(async () => {
      try {
        if (localStorage.getItem(LS_KEY)) return
      } catch {
        // ignore
      }

      for (const base of DEFAULT_CANDIDATES) {
        if (cancelled) return
        if (await probe(base)) return
      }
      if (!cancelled) setNeed(true)
    })()

    return () => {
      cancelled = true
    }
  }, [])

  const handleSubmit = useCallback(async (event: React.FormEvent) => {
    event.preventDefault()
    setError(null)
    const value = input.trim().replace(/\/+$/, '')
    if (!/^https?:\/\//i.test(value)) {
      setError('请输入完整地址，例如 http://192.168.1.10:8080')
      return
    }

    setSubmitting(true)
    const ok = await probe(value, 3000)
    setSubmitting(false)
    if (!ok) {
      setError('无法连接到该服务器，请检查地址与网络')
      return
    }

    try {
      localStorage.setItem(LS_KEY, value)
    } catch {
      setError('写入本地配置失败')
      return
    }
    window.location.reload()
  }, [input])

  if (!need) return null

  return (
    <div className="fixed inset-0 z-[var(--nv-z-modal)] flex items-center justify-center bg-[var(--nv-bg-overlay)] p-6">
      <form onSubmit={handleSubmit} className="w-full max-w-[30rem]">
        <Surface className="border-[var(--nv-border-default)] bg-[var(--nv-bg-elevated)] p-6 shadow-[var(--nv-shadow-elevated)] sm:p-7">
          <div className="mb-5 flex items-start gap-3">
            <div className="grid h-9 w-9 shrink-0 place-items-center rounded-[var(--nv-radius-control)] bg-[var(--nv-fill-hover)] text-[var(--nv-text-tertiary)]">
              <Server size={18} aria-hidden="true" />
            </div>
            <div>
              <h2 className="text-lg font-semibold tracking-[-0.015em] text-[var(--nv-text-primary)]">连接到 Nowen Video 服务器</h2>
              <p className="mt-1 text-sm leading-6 text-[var(--nv-text-tertiary)]">
                未检测到可用后端。请填写本机或局域网内运行的 Nowen Video 服务器地址（包含协议与端口）。
              </p>
            </div>
          </div>

          <label htmlFor="desktop-server-url" className="mb-1.5 block text-xs font-medium text-[var(--nv-text-secondary)]">服务器地址</label>
          <Input
            id="desktop-server-url"
            autoFocus
            type="text"
            value={input}
            onChange={(event) => setInput(event.target.value)}
            placeholder="http://192.168.1.10:8080"
            className="font-mono"
          />

          {error && (
            <div className="mt-3 border-y border-[color-mix(in_srgb,var(--nv-status-danger)_24%,transparent)] py-2 text-xs text-[var(--nv-status-danger)]" role="alert">
              {error}
            </div>
          )}

          <div className="mt-5 flex flex-wrap justify-end gap-2">
            <Button type="button" variant="ghost" onClick={() => setInput('http://127.0.0.1:8080')}>
              使用本机 8080
            </Button>
            <Button type="submit" variant="primary" loading={submitting} disabled={submitting}>
              {!submitting && <Wifi size={15} aria-hidden="true" />}
              {submitting ? '连接中...' : '连接并保存'}
            </Button>
          </div>

          <p className="mt-4 border-t border-[var(--nv-border-subtle)] pt-3 text-xs leading-5 text-[var(--nv-text-tertiary)]">保存后会写入本地配置并重新加载页面；之后可在设置中重置服务器地址。</p>
        </Surface>
      </form>
    </div>
  )
}
