import { useEffect, useState, useCallback } from 'react'
import { Server, Wifi } from 'lucide-react'
import { desktop } from './bridge'
import { Button, Input, Surface } from '@/components/design-system'

const LS_KEY = 'nowen_server_url'

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

/**
 * Desktop 2.0 服务器连接引导。
 *
 * 内嵌模式首先读取 Tauri Runtime 分配的动态 Sidecar 地址；只有内嵌 Media Core
 * 确实不可用、且用户也没有配置远程服务器时，才显示手动连接界面。
 */
export default function DesktopServerPicker() {
  const [need, setNeed] = useState(false)
  const [input, setInput] = useState('http://127.0.0.1:8080')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!desktop.isDesktop) return
    let cancelled = false

    ;(async () => {
      let configured: string | null = null
      try {
        configured = localStorage.getItem(LS_KEY)
      } catch {
        // ignore
      }

      if (configured) {
        if (await probe(configured, 2500)) return
        if (!cancelled) {
          setInput(configured)
          setNeed(true)
          setError('已保存的服务器当前无法连接，请检查地址或改用其他服务器')
        }
        return
      }

      const runtimeBase = await desktop.serverBaseUrl()
      if (cancelled) return
      if (runtimeBase && await probe(runtimeBase, 3000)) return

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
                内嵌 Media Core 当前不可用。你可以连接局域网、NAS 或远程运行的 Nowen Video Server。
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
              使用本机服务器
            </Button>
            <Button type="submit" variant="primary" loading={submitting} disabled={submitting}>
              {!submitting && <Wifi size={15} aria-hidden="true" />}
              {submitting ? '连接中...' : '连接并保存'}
            </Button>
          </div>

          <p className="mt-4 border-t border-[var(--nv-border-subtle)] pt-3 text-xs leading-5 text-[var(--nv-text-tertiary)]">
            手动保存的地址优先于内嵌模式；之后可在设置中清除远程服务器配置恢复本地 Media Core。
          </p>
        </Surface>
      </form>
    </div>
  )
}
