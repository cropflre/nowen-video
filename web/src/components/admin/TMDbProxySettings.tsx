import { useEffect, useState } from 'react'
import { Check, Loader2, Trash2, Wifi, X } from 'lucide-react'
import { adminApi } from '@/api'
import type { TMDbConfigStatus } from '@/types'
import { AdminStatus } from '@/components/admin/AdminPrimitives'
import { Button, Input, Tag } from '@/components/design-system'

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
    <div className="mt-5 border-t border-[var(--nv-border-subtle)] pt-5">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <div className="flex items-center gap-2 text-sm font-semibold text-[var(--nv-text-primary)]">
            <Wifi size={15} className="text-[var(--nv-action-primary)]" />
            TMDb 网络连接与代理
          </div>
          <p className="mt-1 max-w-3xl text-xs leading-5 text-[var(--nv-text-tertiary)]">
            反向代理与网络出口代理可独立配置；同时填写时，反向代理目标也会经过网络出口代理。保存后无需重启。
          </p>
        </div>
        <Tag tone={configured ? 'brand' : 'neutral'}>{configured ? '已自定义网络路径' : '官方直连'}</Tag>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2">
        <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3">
          <p className="text-xs font-semibold text-[var(--nv-text-primary)]">反向代理 Base URL</p>
          <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">适合 nginx 或 Worker 镜像，程序自动拼接 /3/... 与 /t/p/...。</p>
        </div>
        <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3">
          <p className="text-xs font-semibold text-[var(--nv-text-primary)]">HTTP / SOCKS 网络出口</p>
          <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">适合 Clash、v2ray、Shadowsocks、Karing，仅改变访问 TMDb 的网络出口。</p>
        </div>
      </div>

      {message && (
        <div className="mt-3">
          <AdminStatus tone={message.type === 'success' ? 'success' : message.type === 'error' ? 'danger' : 'active'}>
            {message.type === 'success' ? <Check size={13} /> : message.type === 'error' ? <X size={13} /> : <Wifi size={13} />}
            {message.text}
          </AdminStatus>
        </div>
      )}

      <div className="mt-4 space-y-3">
        <ProxyInput label="API 反向代理 Base URL" hint="程序自动请求 {Base URL}/3/..." value={apiBase} onChange={setApiBase} placeholder="https://example.com/tmdbapi" />
        <ProxyInput label="图片反向代理 Base URL" hint="程序自动请求 {Base URL}/t/p/..." value={imageBase} onChange={setImageBase} placeholder="https://example.com/tmdbimg" />
        <ProxyInput label="HTTP / SOCKS 网络出口代理" hint="支持 http、https、socks5、socks5h，可填写局域网地址" value={networkProxy} onChange={setNetworkProxy} placeholder="http://127.0.0.1:7890 或 socks5://127.0.0.1:7891" />

        <div className="flex flex-wrap gap-2 pt-1">
          <Button variant="primary" size="sm" onClick={save} loading={saving}>
            {saving ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}
            {saving ? '保存中...' : '保存连接配置'}
          </Button>
          <Button variant="secondary" size="sm" onClick={test} loading={testing}>
            {testing ? <Loader2 size={14} className="animate-spin" /> : <Wifi size={14} />}
            {testing ? '测试中...' : '测试连接'}
          </Button>
          {configured && (
            <Button variant="danger" size="sm" onClick={clear} disabled={saving}>
              <Trash2 size={14} />
              恢复官方直连
            </Button>
          )}
        </div>

        {result && (
          <div className="space-y-2 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3 text-xs">
            {([['API', result.api], ['图片', result.image], ['网络出口', result.network]] as const).map(([label, item]) => (
              <div key={label} className="flex items-start gap-2">
                {item.ok ? <Check size={13} className="mt-0.5 shrink-0 text-[var(--nv-status-success)]" /> : <X size={13} className="mt-0.5 shrink-0 text-[var(--nv-status-danger)]" />}
                <span className="shrink-0 text-[var(--nv-text-tertiary)]">{label}：</span>
                <span className={item.ok ? 'break-all text-[var(--nv-status-success)]' : 'break-all text-[var(--nv-status-danger)]'}>{item.message}</span>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}

function ProxyInput({
  label,
  hint,
  value,
  onChange,
  placeholder,
}: {
  label: string
  hint: string
  value: string
  onChange: (value: string) => void
  placeholder: string
}) {
  return (
    <div>
      <label className="mb-1.5 block text-xs font-semibold text-[var(--nv-text-secondary)]">{label}</label>
      <Input
        type="text"
        value={value}
        onChange={(event) => onChange(event.target.value)}
        className="font-mono"
        placeholder={placeholder}
      />
      <p className="mt-1 text-xs text-[var(--nv-text-tertiary)]">{hint}</p>
    </div>
  )
}
