import { useEffect, useState } from 'react'
import { Check, Loader2, Trash2, Wifi, X } from 'lucide-react'
import clsx from 'clsx'
import { adminApi } from '@/api'
import type { TMDbConfigStatus } from '@/types'

type ResultItem = { ok: boolean; message: string; target: string }
type TestResult = {
  api: ResultItem
  image: ResultItem
  network: ResultItem & { configured: boolean }
}

type Props = {
  config: TMDbConfigStatus | null
  onConfigChange: (config: TMDbConfigStatus) => void
}

const validateReverseBase = (raw: string) => {
  const value = raw.trim()
  if (!value) return null
  try {
    const url = new URL(value)
    if (!['http:', 'https:'].includes(url.protocol)) return '必须以 http:// 或 https:// 开头'
    return url.hostname ? null : '缺少主机'
  } catch {
    return '地址格式不正确'
  }
}

const validateNetworkProxy = (raw: string) => {
  const value = raw.trim()
  if (!value) return null
  try {
    const url = new URL(value)
    if (!['http:', 'https:', 'socks5:', 'socks5h:'].includes(url.protocol)) {
      return '仅支持 http、https、socks5 或 socks5h'
    }
    if (!url.hostname) return '缺少主机或端口'
    if (url.pathname && url.pathname !== '/') return '不能包含路径'
    return null
  } catch {
    return '地址格式不正确'
  }
}

export default function TMDbProxySettings({ config, onConfigChange }: Props) {
  const [apiBase, setApiBase] = useState('')
  const [imageBase, setImageBase] = useState('')
  const [networkProxy, setNetworkProxy] = useState('')
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [result, setResult] = useState<TestResult | null>(null)
  const [message, setMessage] = useState<{ type: 'success' | 'error' | 'info'; text: string } | null>(null)

  useEffect(() => {
    setApiBase(config?.api_proxy || '')
    setImageBase(config?.image_proxy || '')
    setNetworkProxy(config?.network_proxy || '')
  }, [config?.api_proxy, config?.image_proxy, config?.network_proxy])

  const show = (type: 'success' | 'error' | 'info', text: string) => {
    setMessage({ type, text })
    window.setTimeout(() => setMessage(null), 5000)
  }

  const validate = () => {
    const checks = [
      ['API 反向代理', validateReverseBase(apiBase)],
      ['图片反向代理', validateReverseBase(imageBase)],
      ['网络出口代理', validateNetworkProxy(networkProxy)],
    ] as const
    const failed = checks.find(([, error]) => error)
    if (failed) show('error', `${failed[0]}：${failed[1]}`)
    return !failed
  }

  const save = async () => {
    if (!validate()) return
    setSaving(true)
    try {
      const response = await adminApi.updateTMDbProxy(apiBase.trim(), imageBase.trim(), networkProxy.trim())
      const data = response.data.data
      setApiBase(data.api_proxy)
      setImageBase(data.image_proxy)
      setNetworkProxy(data.network_proxy)
      onConfigChange({
        ...(config || { configured: false, masked_key: '' }),
        api_proxy: data.api_proxy,
        image_proxy: data.image_proxy,
        network_proxy: data.network_proxy,
        network_proxy_configured: Boolean(data.network_proxy),
      })
      show('success', response.data.message || 'TMDb 连接配置已保存')
    } catch (error: unknown) {
      show('error', (error as { response?: { data?: { error?: string } } })?.response?.data?.error || '保存失败')
    } finally {
      setSaving(false)
    }
  }

  const clear = async () => {
    setSaving(true)
    try {
      await adminApi.clearTMDbProxy()
      setApiBase('')
      setImageBase('')
      setNetworkProxy('')
      setResult(null)
      onConfigChange({
        ...(config || { configured: false, masked_key: '' }),
        api_proxy: '',
        image_proxy: '',
        network_proxy: '',
        network_proxy_configured: false,
      })
      show('success', '已恢复 TMDb 官方直连')
    } catch {
      show('error', '恢复官方直连失败')
    } finally {
      setSaving(false)
    }
  }

  const test = async () => {
    if (!validate()) return
    setTesting(true)
    setResult(null)
    try {
      const response = await adminApi.testTMDbProxy(apiBase.trim(), imageBase.trim(), networkProxy.trim())
      const data = response.data.data
      setResult(data)
      if (data.api.ok && data.image.ok && data.network.ok) show('success', '连接测试通过')
      else if (data.api.ok || data.image.ok) show('info', '部分目标可达，请查看测试明细')
      else show('error', 'TMDb API 与图片均不可达')
    } catch (error: unknown) {
      show('error', (error as { response?: { data?: { error?: string } } })?.response?.data?.error || '测试失败')
    } finally {
      setTesting(false)
    }
  }

  const configured = Boolean(apiBase || imageBase || networkProxy || config?.api_proxy || config?.image_proxy || config?.network_proxy)

  return (
    <div className="mt-5 pt-4" style={{ borderTop: '1px solid var(--border-default)' }}>
      <div className="mb-2 flex items-center gap-2">
        <Wifi size={14} className="text-neon/70" />
        <p className="text-sm font-medium" style={{ color: 'var(--text-secondary)' }}>TMDb 网络连接与代理</p>
      </div>
      <p className="mb-3 text-xs text-surface-500">两类代理语义不同，可单独使用；同时填写时，反向代理目标也会经过网络出口代理。保存后无需重启。</p>

      <div className="mb-4 grid gap-2 rounded-lg p-3 text-xs sm:grid-cols-2" style={{ background: 'var(--nav-hover-bg)', border: '1px solid var(--border-default)' }}>
        <div><p className="font-medium text-neon">反向代理 Base URL</p><p className="mt-1 text-surface-500">适合 nginx 或 Worker 镜像，程序自动拼接 /3/... 与 /t/p/...，不是 curl -x。</p></div>
        <div><p className="font-medium text-neon">HTTP/SOCKS 网络出口</p><p className="mt-1 text-surface-500">适合 Clash、v2ray、Shadowsocks、Karing，目标仍是 TMDb，只改变网络出口。</p></div>
      </div>

      {message && <div className={clsx('mb-3 flex items-center gap-2 rounded-lg px-3 py-2 text-xs', message.type === 'success' && 'bg-green-500/10 text-green-400', message.type === 'error' && 'bg-red-500/10 text-red-400', message.type === 'info' && 'bg-blue-500/10 text-blue-400')}>{message.type === 'success' ? <Check size={14} /> : message.type === 'error' ? <X size={14} /> : <Wifi size={14} />}{message.text}</div>}

      <div className="space-y-3">
        <ProxyInput label="API 反向代理 Base URL" hint="程序自动请求 {Base URL}/3/..." value={apiBase} onChange={setApiBase} placeholder="https://example.com/tmdbapi" />
        <ProxyInput label="图片反向代理 Base URL" hint="程序自动请求 {Base URL}/t/p/..." value={imageBase} onChange={setImageBase} placeholder="https://example.com/tmdbimg" />
        <ProxyInput label="HTTP/SOCKS 网络出口代理" hint="支持 http、https、socks5、socks5h，可填写局域网地址" value={networkProxy} onChange={setNetworkProxy} placeholder="http://127.0.0.1:7890 或 socks5://127.0.0.1:7891" />
        <div className="flex flex-wrap gap-2 pt-1">
          <button onClick={save} disabled={saving} className="btn-primary gap-1.5 px-4 py-2 text-sm disabled:opacity-50">{saving ? <><Loader2 size={14} className="animate-spin" />保存中...</> : <><Check size={14} />保存连接配置</>}</button>
          {configured && <button onClick={clear} disabled={saving} className="btn-ghost gap-1.5 px-4 py-2 text-sm text-red-400 disabled:opacity-50"><Trash2 size={14} />恢复官方直连</button>}
          <button onClick={test} disabled={testing} className="btn-ghost gap-1.5 px-4 py-2 text-sm disabled:opacity-50">{testing ? <><Loader2 size={14} className="animate-spin" />测试中...</> : <><Wifi size={14} />测试连接</>}</button>
        </div>
        {result && <div className="space-y-1.5 rounded-lg border px-3 py-2 text-xs" style={{ borderColor: 'var(--border-default)', background: 'var(--bg-surface)' }}>{([['API', result.api], ['图片', result.image], ['网络出口', result.network]] as const).map(([label, item]) => <div key={label} className="flex items-start gap-2">{item.ok ? <Check size={12} className="mt-0.5 shrink-0 text-green-400" /> : <X size={12} className="mt-0.5 shrink-0 text-red-400" />}<span className="shrink-0 text-surface-500">{label}：</span><span className={item.ok ? 'break-all text-green-400' : 'break-all text-red-400'}>{item.message}</span></div>)}</div>}
      </div>
    </div>
  )
}

function ProxyInput({ label, hint, value, onChange, placeholder }: { label: string; hint: string; value: string; onChange: (value: string) => void; placeholder: string }) {
  return <div><label className="mb-1 block text-xs font-medium" style={{ color: 'var(--text-tertiary)' }}>{label}</label><input type="text" value={value} onChange={(event) => onChange(event.target.value)} className="input font-mono text-sm" placeholder={placeholder} /><p className="mt-1 text-xs text-surface-500">{hint}</p></div>
}
