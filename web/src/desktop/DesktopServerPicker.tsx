/**
 * DesktopServerPicker —— 桌面端首次启动"服务器地址"引导
 *
 * 触发条件（同时满足）：
 * 1. 运行在 Tauri 桌面端
 * 2. localStorage.nowen_server_url 未设置（用户从未配置）
 * 3. 默认地址（内嵌 sidecar 或当前 API_BASE）探活失败
 *
 * 用户输入后写入 localStorage 并整页 reload，确保 axios 基址被重新计算。
 *
 * 为什么需要：
 * - 用户可能已在本机 8080 端口独立运行 Go server（开发调试 / 老安装），
 *   内嵌 sidecar 抢不到 8080 就会失败，前端若仍请求 127.0.0.1:<sidecar_port>
 *   会全部 404，海报 / 播放全挂。
 * - 用户可能把 server 部署在 NAS / 其他机器，需要填局域网地址（例：http://192.168.1.10:8080）。
 */
import { useEffect, useState, useCallback } from 'react'
import { desktop } from './bridge'

const LS_KEY = 'nowen_server_url'
const DEFAULT_CANDIDATES = [
  'http://127.0.0.1:21114', // 内嵌 sidecar
  'http://127.0.0.1:8080', // 本机独立 Go server
]

/** 探活 /api/health 接口 */
async function probe(base: string, timeoutMs = 1500): Promise<boolean> {
  try {
    const ctl = new AbortController()
    const tid = setTimeout(() => ctl.abort(), timeoutMs)
    const url = base.replace(/\/+$/, '') + '/api/health'
    const resp = await fetch(url, { signal: ctl.signal, cache: 'no-store' })
    clearTimeout(tid)
    return resp.ok
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
      // 用户已配置 → 不打扰
      try {
        if (localStorage.getItem(LS_KEY)) return
      } catch {
        /* ignore */
      }

      // 依次探活候选地址
      for (const base of DEFAULT_CANDIDATES) {
        if (cancelled) return
        if (await probe(base)) {
          // 默认 sidecar 可通 —— 不需要弹窗
          return
        }
      }
      if (!cancelled) {
        setNeed(true)
      }
    })()

    return () => {
      cancelled = true
    }
  }, [])

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault()
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
      // 整页 reload，让 axios baseURL 重新计算
      window.location.reload()
    },
    [input],
  )

  if (!need) return null

  return (
    <div
      style={{
        position: 'fixed',
        inset: 0,
        zIndex: 99999,
        background: 'rgba(5, 10, 20, 0.92)',
        backdropFilter: 'blur(12px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        padding: 24,
      }}
    >
      <form
        onSubmit={handleSubmit}
        style={{
          width: 'min(480px, 100%)',
          background: 'rgba(15, 22, 40, 0.96)',
          border: '1px solid var(--border-default, rgba(255,255,255,0.08))',
          borderRadius: 16,
          padding: '28px 28px 24px',
          boxShadow: '0 20px 60px rgba(0,0,0,0.5)',
          color: 'var(--text-primary, #e6eaf2)',
        }}
      >
        <h2 style={{ fontSize: 20, fontWeight: 700, marginBottom: 8 }}>
          连接到 nowen-video 服务器
        </h2>
        <p
          style={{
            fontSize: 13,
            lineHeight: 1.6,
            color: 'var(--text-secondary, #9aa3b2)',
            marginBottom: 18,
          }}
        >
          未检测到可用的后端服务。请填写你本机或局域网内运行的 nowen-video
          服务器地址（包含协议与端口）。
        </p>

        <label style={{ display: 'block', fontSize: 12, marginBottom: 6, opacity: 0.75 }}>
          服务器地址
        </label>
        <input
          autoFocus
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="http://192.168.1.10:8080"
          style={{
            width: '100%',
            padding: '10px 12px',
            borderRadius: 8,
            border: '1px solid var(--border-default, rgba(255,255,255,0.12))',
            background: 'rgba(0,0,0,0.35)',
            color: 'inherit',
            fontSize: 14,
            outline: 'none',
          }}
        />
        {error && (
          <div style={{ marginTop: 10, fontSize: 12, color: '#f87171' }}>{error}</div>
        )}

        <div style={{ display: 'flex', gap: 8, marginTop: 18, justifyContent: 'flex-end' }}>
          <button
            type="button"
            onClick={() => setInput('http://127.0.0.1:8080')}
            style={{
              padding: '8px 14px',
              borderRadius: 8,
              border: '1px solid var(--border-default, rgba(255,255,255,0.12))',
              background: 'transparent',
              color: 'inherit',
              fontSize: 13,
              cursor: 'pointer',
            }}
          >
            使用本机 8080
          </button>
          <button
            type="submit"
            disabled={submitting}
            style={{
              padding: '8px 18px',
              borderRadius: 8,
              border: 'none',
              background: submitting ? 'rgba(86, 134, 255, 0.4)' : 'var(--neon-blue, #5686ff)',
              color: '#fff',
              fontSize: 13,
              fontWeight: 600,
              cursor: submitting ? 'wait' : 'pointer',
            }}
          >
            {submitting ? '连接中...' : '连接并保存'}
          </button>
        </div>

        <p
          style={{
            marginTop: 14,
            fontSize: 11,
            lineHeight: 1.5,
            color: 'var(--text-tertiary, #6b7280)',
          }}
        >
          提示：保存后将写入本地配置。随时可在"设置 / 关于"里重置。
        </p>
      </form>
    </div>
  )
}
