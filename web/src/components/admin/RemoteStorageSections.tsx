import { useEffect, useState, useCallback } from 'react'
import { storageApi } from '@/api'
import type { AlistConfig, AlistStatus, S3Config, S3Status } from '@/api/storage'
import {
  Cloud,
  CheckCircle2,
  XCircle,
  Loader2,
  Save,
  Wifi,
  Eye,
  EyeOff,
  Server,
  Database,
  RefreshCw,
} from 'lucide-react'
import clsx from 'clsx'

// ==================== V2.3: 远程存储扩展管理 ====================
// 为 StorageTab 提供 Alist / S3 两个独立 section

// ---------- 默认值 ----------

const DEFAULT_ALIST: AlistConfig = {
  enabled: false,
  server_url: '',
  username: '',
  password: '',
  token: '',
  base_path: '/',
  timeout: 30,
  enable_cache: true,
  cache_ttl_hours: 12,
  read_block_size_mb: 8,
  read_block_count: 4,
}

const DEFAULT_S3: S3Config = {
  enabled: false,
  endpoint: '',
  region: 'us-east-1',
  access_key: '',
  secret_key: '',
  bucket: '',
  base_path: '',
  path_style: true,
  timeout: 30,
  enable_cache: true,
  cache_ttl_hours: 24,
  read_block_size_mb: 8,
  read_block_count: 4,
}

// ==================== Alist Section ====================

export function AlistSection() {
  const [config, setConfig] = useState<AlistConfig>(DEFAULT_ALIST)
  const [status, setStatus] = useState<AlistStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null)
  const [showPwd, setShowPwd] = useState(false)
  const [showToken, setShowToken] = useState(false)
  const [pwdDirty, setPwdDirty] = useState(false)
  const [tokenDirty, setTokenDirty] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [cfgRes, statusRes] = await Promise.all([
        storageApi.getAlistConfig(),
        storageApi.getStorageStatus(),
      ])
      setConfig({ ...DEFAULT_ALIST, ...cfgRes.data.data })
      setStatus(statusRes.data.data.alist || null)
    } catch (e) {
      console.error('加载 Alist 配置失败', e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const handleSave = async () => {
    setSaving(true)
    try {
      const payload: Partial<AlistConfig> = { ...config }
      if (!pwdDirty) delete payload.password
      if (!tokenDirty) delete payload.token
      await storageApi.updateAlistConfig(payload)
      setPwdDirty(false)
      setTokenDirty(false)
      await load()
    } catch (e: any) {
      alert('保存失败: ' + (e.response?.data?.error || e.message))
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      await storageApi.testAlistConnection({
        server_url: config.server_url,
        username: config.username,
        password: pwdDirty ? config.password : '',
        token: tokenDirty ? config.token : '',
        base_path: config.base_path,
      })
      setTestResult({ ok: true, msg: '连接测试成功' })
    } catch (e: any) {
      setTestResult({ ok: false, msg: e.response?.data?.error || '连接测试失败' })
    } finally {
      setTesting(false)
    }
  }

  if (loading) {
    return (
      <section className="flex items-center justify-center py-8">
        <Loader2 className="animate-spin text-neon" size={20} />
      </section>
    )
  }

  return (
    <section>
      <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <Cloud size={20} className="text-purple-400/80" />
        Alist 聚合网盘
        <span className="ml-2 rounded-full bg-purple-500/15 px-2 py-0.5 text-[10px] font-bold uppercase text-purple-300">
          V2.3
        </span>
        {status && (
          <span
            className={clsx(
              'ml-auto inline-flex items-center gap-1.5 text-xs rounded-full px-3 py-1 font-medium',
              status.connected
                ? 'bg-green-500/10 text-green-400'
                : status.enabled
                ? 'bg-red-500/10 text-red-400'
                : 'bg-surface-700 text-surface-400'
            )}
          >
            {status.connected ? (
              <>
                <CheckCircle2 size={12} />
                已连接
              </>
            ) : status.enabled ? (
              <>
                <XCircle size={12} />
                未连接
              </>
            ) : (
              <>未启用</>
            )}
          </span>
        )}
      </h2>

      <p className="text-xs text-surface-500 mb-4 leading-relaxed">
        通过 Alist 统一对接 20+ 网盘（阿里云盘/115/夸克/百度网盘/OneDrive/Google Drive 等），
        媒体库路径使用 <code className="px-1 py-0.5 bg-surface-800 rounded">alist://</code> 前缀。
        推荐使用 Token 模式避免明文密码。
      </p>

      <div className="glass-panel-subtle rounded-xl p-5 space-y-4">
        <label className="flex items-center gap-3 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
            className="w-4 h-4 accent-purple-500"
          />
          <span className="text-sm">启用 Alist 存储</span>
        </label>

        <div>
          <label className="block text-xs text-surface-400 mb-1.5">服务器地址</label>
          <input
            type="text"
            value={config.server_url}
            onChange={(e) => setConfig({ ...config, server_url: e.target.value })}
            placeholder="https://alist.example.com"
            className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
          />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">用户名（可选）</label>
            <input
              type="text"
              value={config.username}
              onChange={(e) => setConfig({ ...config, username: e.target.value })}
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">密码（可选）</label>
            <div className="relative">
              <input
                type={showPwd ? 'text' : 'password'}
                value={config.password}
                onChange={(e) => {
                  setConfig({ ...config, password: e.target.value })
                  setPwdDirty(true)
                }}
                className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm pr-9"
              />
              <button
                type="button"
                onClick={() => setShowPwd((v) => !v)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-500 hover:text-surface-300"
              >
                {showPwd ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </div>
        </div>

        <div>
          <label className="block text-xs text-surface-400 mb-1.5">
            长期 Token（推荐，优先于用户名密码）
          </label>
          <div className="relative">
            <input
              type={showToken ? 'text' : 'password'}
              value={config.token}
              onChange={(e) => {
                setConfig({ ...config, token: e.target.value })
                setTokenDirty(true)
              }}
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm pr-9 font-mono"
            />
            <button
              type="button"
              onClick={() => setShowToken((v) => !v)}
              className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-500 hover:text-surface-300"
            >
              {showToken ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">基础路径</label>
            <input
              type="text"
              value={config.base_path}
              onChange={(e) => setConfig({ ...config, base_path: e.target.value })}
              placeholder="/aliyun/movies"
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">超时时间（秒）</label>
            <input
              type="number"
              value={config.timeout}
              onChange={(e) =>
                setConfig({ ...config, timeout: parseInt(e.target.value) || 30 })
              }
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">缓存 TTL（小时）</label>
            <input
              type="number"
              value={config.cache_ttl_hours}
              onChange={(e) =>
                setConfig({ ...config, cache_ttl_hours: parseInt(e.target.value) || 12 })
              }
              disabled={!config.enable_cache}
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm disabled:opacity-40"
            />
          </div>
          <div>
            <label className="flex items-center gap-2 mt-6 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={config.enable_cache}
                onChange={(e) => setConfig({ ...config, enable_cache: e.target.checked })}
                className="w-4 h-4 accent-purple-500"
              />
              <span className="text-xs">启用元数据缓存</span>
            </label>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">
              播放缓存块大小（MiB）
            </label>
            <input
              type="number"
              value={config.read_block_size_mb}
              onChange={(e) =>
                setConfig({ ...config, read_block_size_mb: parseInt(e.target.value) || 8 })
              }
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">每文件最大块数</label>
            <input
              type="number"
              value={config.read_block_count}
              onChange={(e) =>
                setConfig({ ...config, read_block_count: parseInt(e.target.value) || 4 })
              }
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
        </div>

        {testResult && (
          <div
            className={clsx(
              'text-xs rounded-lg p-2',
              testResult.ok
                ? 'bg-green-500/10 text-green-400'
                : 'bg-red-500/10 text-red-400'
            )}
          >
            {testResult.msg}
          </div>
        )}

        <div className="flex items-center gap-2 pt-2">
          <button
            onClick={handleTest}
            disabled={testing || !config.server_url}
            className="inline-flex items-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm hover:bg-surface-700 disabled:opacity-40"
          >
            {testing ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              <Wifi size={14} />
            )}
            测试连接
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="inline-flex items-center gap-2 rounded-lg bg-purple-500 px-4 py-2 text-sm font-medium text-white hover:bg-purple-600 disabled:opacity-40"
          >
            {saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
            保存配置
          </button>
          <button
            onClick={load}
            className="ml-auto text-surface-500 hover:text-surface-300"
            title="刷新状态"
          >
            <RefreshCw size={14} />
          </button>
        </div>
      </div>
    </section>
  )
}

// ==================== S3 Section ====================

export function S3Section() {
  const [config, setConfig] = useState<S3Config>(DEFAULT_S3)
  const [status, setStatus] = useState<S3Status | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null)
  const [showSecret, setShowSecret] = useState(false)
  const [secretDirty, setSecretDirty] = useState(false)

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const [cfgRes, statusRes] = await Promise.all([
        storageApi.getS3Config(),
        storageApi.getStorageStatus(),
      ])
      setConfig({ ...DEFAULT_S3, ...cfgRes.data.data })
      setStatus(statusRes.data.data.s3 || null)
    } catch (e) {
      console.error('加载 S3 配置失败', e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    load()
  }, [load])

  const handleSave = async () => {
    setSaving(true)
    try {
      const payload: Partial<S3Config> = { ...config }
      if (!secretDirty) delete payload.secret_key
      await storageApi.updateS3Config(payload)
      setSecretDirty(false)
      await load()
    } catch (e: any) {
      alert('保存失败: ' + (e.response?.data?.error || e.message))
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      await storageApi.testS3Connection({
        endpoint: config.endpoint,
        region: config.region,
        access_key: config.access_key,
        secret_key: secretDirty ? config.secret_key : '',
        bucket: config.bucket,
        base_path: config.base_path,
        path_style: config.path_style,
      })
      setTestResult({ ok: true, msg: '连接测试成功' })
    } catch (e: any) {
      setTestResult({ ok: false, msg: e.response?.data?.error || '连接测试失败' })
    } finally {
      setTesting(false)
    }
  }

  if (loading) {
    return (
      <section className="flex items-center justify-center py-8">
        <Loader2 className="animate-spin text-neon" size={20} />
      </section>
    )
  }

  return (
    <section>
      <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
        <Database size={20} className="text-amber-400/80" />
        S3 兼容对象存储
        <span className="ml-2 rounded-full bg-amber-500/15 px-2 py-0.5 text-[10px] font-bold uppercase text-amber-300">
          V2.3
        </span>
        {status && (
          <span
            className={clsx(
              'ml-auto inline-flex items-center gap-1.5 text-xs rounded-full px-3 py-1 font-medium',
              status.connected
                ? 'bg-green-500/10 text-green-400'
                : status.enabled
                ? 'bg-red-500/10 text-red-400'
                : 'bg-surface-700 text-surface-400'
            )}
          >
            {status.connected ? (
              <>
                <CheckCircle2 size={12} />
                已连接
              </>
            ) : status.enabled ? (
              <>
                <XCircle size={12} />
                未连接
              </>
            ) : (
              <>未启用</>
            )}
          </span>
        )}
      </h2>

      <p className="text-xs text-surface-500 mb-4 leading-relaxed">
        支持 AWS S3 / MinIO / Cloudflare R2 / 阿里云 OSS / 腾讯云 COS / Backblaze B2 等，
        媒体库路径使用 <code className="px-1 py-0.5 bg-surface-800 rounded">s3://</code> 前缀。
        MinIO 等请勾选"Path-Style"。播放走预签名 URL，默认有效期 2 小时。
      </p>

      <div className="glass-panel-subtle rounded-xl p-5 space-y-4">
        <label className="flex items-center gap-3 cursor-pointer select-none">
          <input
            type="checkbox"
            checked={config.enabled}
            onChange={(e) => setConfig({ ...config, enabled: e.target.checked })}
            className="w-4 h-4 accent-amber-500"
          />
          <span className="text-sm">启用 S3 存储</span>
        </label>

        <div className="grid grid-cols-2 gap-3">
          <div className="col-span-2">
            <label className="block text-xs text-surface-400 mb-1.5">
              Endpoint
              <span className="ml-1 text-surface-600">
                （如 https://s3.amazonaws.com 或 https://minio.example.com:9000）
              </span>
            </label>
            <input
              type="text"
              value={config.endpoint}
              onChange={(e) => setConfig({ ...config, endpoint: e.target.value })}
              placeholder="https://s3.amazonaws.com"
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">Region</label>
            <input
              type="text"
              value={config.region}
              onChange={(e) => setConfig({ ...config, region: e.target.value })}
              placeholder="us-east-1"
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">Bucket</label>
            <input
              type="text"
              value={config.bucket}
              onChange={(e) => setConfig({ ...config, bucket: e.target.value })}
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">Access Key</label>
            <input
              type="text"
              value={config.access_key}
              onChange={(e) => setConfig({ ...config, access_key: e.target.value })}
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm font-mono"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">Secret Key</label>
            <div className="relative">
              <input
                type={showSecret ? 'text' : 'password'}
                value={config.secret_key}
                onChange={(e) => {
                  setConfig({ ...config, secret_key: e.target.value })
                  setSecretDirty(true)
                }}
                className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm pr-9 font-mono"
              />
              <button
                type="button"
                onClick={() => setShowSecret((v) => !v)}
                className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-500 hover:text-surface-300"
              >
                {showSecret ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">基础路径前缀</label>
            <input
              type="text"
              value={config.base_path}
              onChange={(e) => setConfig({ ...config, base_path: e.target.value })}
              placeholder="media/"
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="flex items-center gap-2 mt-6 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={config.path_style}
                onChange={(e) => setConfig({ ...config, path_style: e.target.checked })}
                className="w-4 h-4 accent-amber-500"
              />
              <span className="text-xs flex items-center gap-1">
                <Server size={12} />
                Path-Style 寻址（MinIO 必选）
              </span>
            </label>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">缓存 TTL（小时）</label>
            <input
              type="number"
              value={config.cache_ttl_hours}
              onChange={(e) =>
                setConfig({ ...config, cache_ttl_hours: parseInt(e.target.value) || 24 })
              }
              disabled={!config.enable_cache}
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm disabled:opacity-40"
            />
          </div>
          <div>
            <label className="flex items-center gap-2 mt-6 cursor-pointer select-none">
              <input
                type="checkbox"
                checked={config.enable_cache}
                onChange={(e) => setConfig({ ...config, enable_cache: e.target.checked })}
                className="w-4 h-4 accent-amber-500"
              />
              <span className="text-xs">启用元数据缓存</span>
            </label>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">
              播放缓存块大小（MiB）
            </label>
            <input
              type="number"
              value={config.read_block_size_mb}
              onChange={(e) =>
                setConfig({ ...config, read_block_size_mb: parseInt(e.target.value) || 8 })
              }
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
          <div>
            <label className="block text-xs text-surface-400 mb-1.5">每文件最大块数</label>
            <input
              type="number"
              value={config.read_block_count}
              onChange={(e) =>
                setConfig({ ...config, read_block_count: parseInt(e.target.value) || 4 })
              }
              className="w-full rounded-lg bg-surface-800 px-3 py-2 text-sm"
            />
          </div>
        </div>

        {testResult && (
          <div
            className={clsx(
              'text-xs rounded-lg p-2',
              testResult.ok
                ? 'bg-green-500/10 text-green-400'
                : 'bg-red-500/10 text-red-400'
            )}
          >
            {testResult.msg}
          </div>
        )}

        <div className="flex items-center gap-2 pt-2">
          <button
            onClick={handleTest}
            disabled={testing || !config.endpoint || !config.bucket}
            className="inline-flex items-center gap-2 rounded-lg bg-surface-800 px-3 py-2 text-sm hover:bg-surface-700 disabled:opacity-40"
          >
            {testing ? (
              <Loader2 size={14} className="animate-spin" />
            ) : (
              <Wifi size={14} />
            )}
            测试连接
          </button>
          <button
            onClick={handleSave}
            disabled={saving}
            className="inline-flex items-center gap-2 rounded-lg bg-amber-500 px-4 py-2 text-sm font-medium text-white hover:bg-amber-600 disabled:opacity-40"
          >
            {saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
            保存配置
          </button>
          <button
            onClick={load}
            className="ml-auto text-surface-500 hover:text-surface-300"
            title="刷新状态"
          >
            <RefreshCw size={14} />
          </button>
        </div>
      </div>
    </section>
  )
}
