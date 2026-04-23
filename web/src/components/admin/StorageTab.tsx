import { useEffect, useState, useCallback } from 'react'
import { storageApi, libraryApi } from '@/api'
import type { WebDAVConfig, WebDAVStatus } from '@/api/storage'
import type { Library } from '@/types'
import {
  HardDrive,
  Cloud,
  CheckCircle2,
  XCircle,
  Loader2,
  Save,
  Wifi,
  Link2,
  Eye,
  EyeOff,
  RefreshCw,
} from 'lucide-react'
import clsx from 'clsx'
import { AlistSection, S3Section } from './RemoteStorageSections'

// ==================== 存储管理标签页 ====================
// V2.1 WebDAV 存储管理

const DEFAULT_CONFIG: WebDAVConfig = {
  enabled: false,
  server_url: '',
  username: '',
  password: '',
  base_path: '/',
  timeout: 30,
  enable_pool: true,
  pool_size: 5,
  enable_cache: true,
  cache_ttl_hours: 24,
  max_retries: 3,
  retry_interval: 5,
}

export default function StorageTab() {
  const [config, setConfig] = useState<WebDAVConfig>(DEFAULT_CONFIG)
  const [status, setStatus] = useState<WebDAVStatus | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [testResult, setTestResult] = useState<{ ok: boolean; msg: string } | null>(null)
  const [showPassword, setShowPassword] = useState(false)
  const [passwordDirty, setPasswordDirty] = useState(false)
  const [registeringLib, setRegisteringLib] = useState<string | null>(null)

  // 加载配置与状态
  const loadAll = useCallback(async () => {
    setLoading(true)
    try {
      const [cfgRes, statusRes, libsRes] = await Promise.all([
        storageApi.getWebDAVConfig(),
        storageApi.getWebDAVStatus(),
        libraryApi.list(),
      ])
      setConfig({ ...DEFAULT_CONFIG, ...cfgRes.data.data })
      setStatus(statusRes.data.data)
      setLibraries(libsRes.data.data || [])
      setPasswordDirty(false)
    } catch (e: any) {
      console.error('加载 WebDAV 配置失败', e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadAll()
  }, [loadAll])

  // 保存配置
  const handleSave = async () => {
    setSaving(true)
    setTestResult(null)
    try {
      // 如果密码未变更（仍是掩码），不提交密码字段
      const payload: Partial<WebDAVConfig> = { ...config }
      if (!passwordDirty) {
        payload.password = ''
      }
      await storageApi.updateWebDAVConfig(payload)
      setTestResult({ ok: true, msg: 'WebDAV 配置已保存' })
      await loadAll()
    } catch (e: any) {
      setTestResult({
        ok: false,
        msg: e?.response?.data?.error || '保存失败',
      })
    } finally {
      setSaving(false)
    }
  }

  // 测试连接
  const handleTest = async () => {
    setTesting(true)
    setTestResult(null)
    try {
      await storageApi.testWebDAVConnection({
        server_url: config.server_url,
        username: config.username,
        password: passwordDirty ? config.password : '',
        base_path: config.base_path,
      })
      setTestResult({ ok: true, msg: 'WebDAV 连接测试成功 ✓' })
    } catch (e: any) {
      setTestResult({
        ok: false,
        msg: e?.response?.data?.error || '连接测试失败',
      })
    } finally {
      setTesting(false)
    }
  }

  // 为媒体库注册 WebDAV 存储
  const handleRegisterLib = async (libId: string) => {
    setRegisteringLib(libId)
    try {
      await storageApi.registerWebDAVLibrary(libId)
      setTestResult({ ok: true, msg: `已为媒体库注册 WebDAV 存储` })
      await loadAll()
    } catch (e: any) {
      setTestResult({
        ok: false,
        msg: e?.response?.data?.error || '注册失败',
      })
    } finally {
      setRegisteringLib(null)
    }
  }

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="animate-spin text-neon" size={32} />
      </div>
    )
  }

  return (
    <div className="space-y-8">
      {/* 概览卡片 */}
      <section>
        <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <HardDrive size={20} className="text-neon/60" />
          存储概览
        </h2>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* 本地存储 */}
          <div className="glass-panel-subtle rounded-xl p-5">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-3">
                <HardDrive size={24} className="text-blue-400" />
                <div>
                  <div className="font-semibold" style={{ color: 'var(--text-primary)' }}>本地存储</div>
                  <div className="text-xs text-surface-500">文件系统直读</div>
                </div>
              </div>
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs text-emerald-400">
                <CheckCircle2 size={12} /> 已启用
              </span>
            </div>
          </div>

          {/* WebDAV 存储 */}
          <div className="glass-panel-subtle rounded-xl p-5">
            <div className="flex items-center justify-between mb-3">
              <div className="flex items-center gap-3">
                <Cloud size={24} className="text-purple-400" />
                <div>
                  <div className="font-semibold" style={{ color: 'var(--text-primary)' }}>WebDAV 存储</div>
                  <div className="text-xs text-surface-500 truncate max-w-[200px]">
                    {status?.server_url || '未配置'}
                  </div>
                </div>
              </div>
              {!status?.enabled ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-surface-500/10 px-2.5 py-1 text-xs text-surface-400">
                  未启用
                </span>
              ) : status?.connected ? (
                <span className="inline-flex items-center gap-1 rounded-full bg-emerald-500/10 px-2.5 py-1 text-xs text-emerald-400">
                  <Wifi size={12} /> 已连接
                </span>
              ) : (
                <span className="inline-flex items-center gap-1 rounded-full bg-red-500/10 px-2.5 py-1 text-xs text-red-400">
                  <XCircle size={12} /> 异常
                </span>
              )}
            </div>
            {status?.error && (
              <p className="text-xs text-red-400 mt-2 break-all">{status.error}</p>
            )}
          </div>
        </div>
      </section>

      {/* WebDAV 配置表单 */}
      <section>
        <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
          <Cloud size={20} className="text-neon/60" />
          WebDAV 配置
        </h2>

        <div className="glass-panel-subtle rounded-xl p-6 space-y-5">
          {/* 启用开关 */}
          <div className="flex items-center justify-between">
            <div>
              <label className="font-semibold" style={{ color: 'var(--text-primary)' }}>启用 WebDAV 存储</label>
              <p className="text-xs text-surface-500 mt-0.5">启用后可为媒体库挂载远程 WebDAV 目录</p>
            </div>
            <button
              onClick={() => setConfig({ ...config, enabled: !config.enabled })}
              className={clsx(
                'relative w-12 h-6 rounded-full transition-colors',
                config.enabled ? 'bg-neon' : 'bg-surface-700'
              )}
            >
              <span
                className={clsx(
                  'absolute top-0.5 w-5 h-5 bg-white rounded-full transition-transform',
                  config.enabled ? 'translate-x-6' : 'translate-x-0.5'
                )}
              />
            </button>
          </div>

          {/* 服务器地址 */}
          <div>
            <label className="block text-sm font-medium mb-1.5" style={{ color: 'var(--text-primary)' }}>
              服务器地址 <span className="text-red-400">*</span>
            </label>
            <input
              type="url"
              value={config.server_url}
              onChange={(e) => setConfig({ ...config, server_url: e.target.value })}
              placeholder="https://webdav.example.com/dav/"
              className="input-field w-full"
              disabled={!config.enabled}
            />
          </div>

          {/* 凭证 */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium mb-1.5" style={{ color: 'var(--text-primary)' }}>
                用户名
              </label>
              <input
                type="text"
                value={config.username}
                onChange={(e) => setConfig({ ...config, username: e.target.value })}
                className="input-field w-full"
                disabled={!config.enabled}
                autoComplete="username"
              />
            </div>
            <div>
              <label className="block text-sm font-medium mb-1.5" style={{ color: 'var(--text-primary)' }}>
                密码
              </label>
              <div className="relative">
                <input
                  type={showPassword ? 'text' : 'password'}
                  value={config.password}
                  onChange={(e) => {
                    setConfig({ ...config, password: e.target.value })
                    setPasswordDirty(true)
                  }}
                  className="input-field w-full pr-10"
                  disabled={!config.enabled}
                  autoComplete="current-password"
                  placeholder={passwordDirty ? '' : '保持原密码请留空'}
                />
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="absolute right-2 top-1/2 -translate-y-1/2 text-surface-500 hover:text-surface-300"
                >
                  {showPassword ? <EyeOff size={16} /> : <Eye size={16} />}
                </button>
              </div>
            </div>
          </div>

          {/* 基础路径 */}
          <div>
            <label className="block text-sm font-medium mb-1.5" style={{ color: 'var(--text-primary)' }}>
              基础路径
            </label>
            <input
              type="text"
              value={config.base_path}
              onChange={(e) => setConfig({ ...config, base_path: e.target.value })}
              placeholder="/ 或 /media/videos"
              className="input-field w-full"
              disabled={!config.enabled}
            />
            <p className="text-xs text-surface-500 mt-1">
              所有媒体库路径会基于此路径解析（相对路径）
            </p>
          </div>

          {/* 高级选项 */}
          <details className="group">
            <summary className="cursor-pointer text-sm font-medium text-neon/80 hover:text-neon select-none">
              高级选项
            </summary>
            <div className="mt-4 grid grid-cols-2 md:grid-cols-3 gap-4">
              <div>
                <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
                  超时 (秒)
                </label>
                <input
                  type="number"
                  min={1}
                  value={config.timeout}
                  onChange={(e) => setConfig({ ...config, timeout: Number(e.target.value) })}
                  className="input-field w-full"
                  disabled={!config.enabled}
                />
              </div>
              <div>
                <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
                  连接池大小
                </label>
                <input
                  type="number"
                  min={1}
                  value={config.pool_size}
                  onChange={(e) => setConfig({ ...config, pool_size: Number(e.target.value) })}
                  className="input-field w-full"
                  disabled={!config.enabled}
                />
              </div>
              <div>
                <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
                  缓存 TTL (小时)
                </label>
                <input
                  type="number"
                  min={0}
                  value={config.cache_ttl_hours}
                  onChange={(e) => setConfig({ ...config, cache_ttl_hours: Number(e.target.value) })}
                  className="input-field w-full"
                  disabled={!config.enabled}
                />
              </div>
              <div>
                <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
                  最大重试次数
                </label>
                <input
                  type="number"
                  min={0}
                  value={config.max_retries}
                  onChange={(e) => setConfig({ ...config, max_retries: Number(e.target.value) })}
                  className="input-field w-full"
                  disabled={!config.enabled}
                />
              </div>
              <div>
                <label className="block text-xs font-medium mb-1" style={{ color: 'var(--text-primary)' }}>
                  重试间隔 (秒)
                </label>
                <input
                  type="number"
                  min={1}
                  value={config.retry_interval}
                  onChange={(e) => setConfig({ ...config, retry_interval: Number(e.target.value) })}
                  className="input-field w-full"
                  disabled={!config.enabled}
                />
              </div>
              <div className="flex items-end">
                <label className="flex items-center gap-2 cursor-pointer">
                  <input
                    type="checkbox"
                    checked={config.enable_cache}
                    onChange={(e) => setConfig({ ...config, enable_cache: e.target.checked })}
                    disabled={!config.enabled}
                    className="w-4 h-4"
                  />
                  <span className="text-xs" style={{ color: 'var(--text-primary)' }}>启用本地元数据缓存</span>
                </label>
              </div>
            </div>
          </details>

          {/* 操作按钮 */}
          <div className="flex flex-wrap gap-3 pt-3 border-t border-surface-700/40">
            <button
              onClick={handleTest}
              disabled={!config.enabled || !config.server_url || testing || saving}
              className="inline-flex items-center gap-2 rounded-lg bg-blue-500/10 px-4 py-2 text-sm text-blue-400 hover:bg-blue-500/20 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {testing ? <Loader2 size={16} className="animate-spin" /> : <Link2 size={16} />}
              测试连接
            </button>
            <button
              onClick={handleSave}
              disabled={saving || testing}
              className="inline-flex items-center gap-2 rounded-lg bg-neon/20 px-4 py-2 text-sm text-neon hover:bg-neon/30 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
            >
              {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
              保存配置
            </button>
            <button
              onClick={loadAll}
              disabled={loading}
              className="inline-flex items-center gap-2 rounded-lg bg-surface-700/40 px-4 py-2 text-sm text-surface-300 hover:bg-surface-700/60 disabled:opacity-40 transition-colors"
            >
              <RefreshCw size={16} />
              刷新
            </button>
          </div>

          {/* 测试/保存结果 */}
          {testResult && (
            <div
              className={clsx(
                'rounded-lg p-3 text-sm flex items-start gap-2',
                testResult.ok
                  ? 'bg-emerald-500/10 text-emerald-400'
                  : 'bg-red-500/10 text-red-400'
              )}
            >
              {testResult.ok ? <CheckCircle2 size={16} className="mt-0.5 flex-shrink-0" /> : <XCircle size={16} className="mt-0.5 flex-shrink-0" />}
              <span className="break-all">{testResult.msg}</span>
            </div>
          )}
        </div>
      </section>

      {/* 媒体库挂载 */}
      {status?.enabled && status?.connected && (
        <section>
          <h2 className="mb-4 flex items-center gap-2 font-display text-lg font-semibold tracking-wide" style={{ color: 'var(--text-primary)' }}>
            <Link2 size={20} className="text-neon/60" />
            媒体库 WebDAV 挂载
          </h2>
          <p className="text-xs text-surface-500 mb-4">
            将 WebDAV 存储挂载到指定媒体库，扫描时会自动从远程读取文件。
            （媒体库路径需以 <code className="px-1 py-0.5 bg-surface-800 rounded">webdav://</code>
            、<code className="px-1 py-0.5 bg-surface-800 rounded">alist://</code>
            或 <code className="px-1 py-0.5 bg-surface-800 rounded">s3://</code> 开头才会走远程存储）
          </p>
          <div className="space-y-2">
            {libraries.length === 0 ? (
              <div className="glass-panel-subtle rounded-xl p-6 text-center text-sm text-surface-500">
                暂无媒体库
              </div>
            ) : (
              libraries.map((lib) => {
                const isWebDAV = lib.path?.startsWith('webdav://')
                return (
                  <div
                    key={lib.id}
                    className="glass-panel-subtle rounded-xl p-4 flex items-center justify-between"
                  >
                    <div className="flex items-center gap-3 min-w-0">
                      {isWebDAV ? (
                        <Cloud size={20} className="text-purple-400 flex-shrink-0" />
                      ) : (
                        <HardDrive size={20} className="text-blue-400 flex-shrink-0" />
                      )}
                      <div className="min-w-0">
                        <div className="font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                          {lib.name}
                        </div>
                        <div className="text-xs text-surface-500 truncate">{lib.path}</div>
                      </div>
                    </div>
                    <button
                      onClick={() => handleRegisterLib(lib.id)}
                      disabled={registeringLib === lib.id}
                      className="inline-flex items-center gap-1.5 rounded-lg bg-purple-500/10 px-3 py-1.5 text-xs text-purple-400 hover:bg-purple-500/20 disabled:opacity-40 transition-colors flex-shrink-0"
                    >
                      {registeringLib === lib.id ? (
                        <Loader2 size={12} className="animate-spin" />
                      ) : (
                        <Link2 size={12} />
                      )}
                      挂载 WebDAV
                    </button>
                  </div>
                )
              })
            )}
          </div>
        </section>
      )}

      {/* V2.3: Alist 聚合网盘 */}
      <AlistSection />

      {/* V2.3: S3 兼容对象存储 */}
      <S3Section />
    </div>
  )
}
