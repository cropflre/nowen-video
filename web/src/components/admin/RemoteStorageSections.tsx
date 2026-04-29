import { useEffect, useState, useCallback } from 'react'
import { storageApi } from '@/api'
import type { AlistConfig, AlistStatus, S3Config, S3Status } from '@/api/storage'
import {
  Cloud,
  Loader2,
  Save,
  Wifi,
  Server,
  Database,
  RefreshCw,
  Link2,
} from 'lucide-react'
import {
  SectionShell,
  FieldGroup,
  Field,
  Input,
  Toggle,
  ActionBar,
  ActionButton,
  Toast,
  StatusBadge,
  VersionBadge,
  EyeToggle,
  EnableRow,
  ProviderState,
} from './storage/StorageUIKit'

// ==================== V2.3: 远程存储扩展管理 ====================
// Alist / S3 两个 section，样式与 WebDAV 完全对齐

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

function toProviderState(status: { enabled?: boolean; connected?: boolean } | null | undefined): ProviderState {
  if (!status) return 'disabled'
  if (!status.enabled) return 'disabled'
  if (status.connected) return 'connected'
  return 'error'
}

// ==================== Alist Section ====================

export function AlistSection() {
  const [config, setConfig] = useState<AlistConfig>(DEFAULT_ALIST)
  const [status, setStatus] = useState<AlistStatus | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [toast, setToast] = useState<{ ok: boolean; msg: string } | null>(null)
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
    setToast(null)
    try {
      const payload: Partial<AlistConfig> = { ...config }
      if (!pwdDirty) delete payload.password
      if (!tokenDirty) delete payload.token
      await storageApi.updateAlistConfig(payload)
      setPwdDirty(false)
      setTokenDirty(false)
      setToast({ ok: true, msg: 'Alist 配置已保存' })
      await load()
    } catch (e: any) {
      setToast({ ok: false, msg: '保存失败: ' + (e.response?.data?.error || e.message) })
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    setTesting(true)
    setToast(null)
    try {
      await storageApi.testAlistConnection({
        server_url: config.server_url,
        username: config.username,
        password: pwdDirty ? config.password : '',
        token: tokenDirty ? config.token : '',
        base_path: config.base_path,
      })
      setToast({ ok: true, msg: 'Alist 连接测试成功' })
    } catch (e: any) {
      setToast({ ok: false, msg: e.response?.data?.error || '连接测试失败' })
    } finally {
      setTesting(false)
    }
  }

  if (loading) {
    return (
      <section className="flex items-center justify-center py-8">
        <Loader2 className="animate-spin text-primary-400" size={20} />
      </section>
    )
  }

  return (
    <SectionShell
      icon={<Cloud size={18} />}
      title="Alist 聚合网盘"
      subtitle="统一对接阿里云盘 / 115 / 夸克 / 百度网盘 / OneDrive 等"
      badge={<VersionBadge accent="purple">V2.3</VersionBadge>}
      statusSlot={<StatusBadge state={toProviderState(status)} />}
      accent="purple"
      description={
        <>
          媒体库路径使用{' '}
          <code
            className="mx-0.5 rounded px-1.5 py-0.5 font-mono text-[11px] text-purple-600 dark:text-purple-300"
            style={{ background: 'rgba(168,85,247,0.08)', border: '1px solid rgba(168,85,247,0.25)' }}
          >
            alist://
          </code>
          {' '}前缀。推荐使用 <strong>长期 Token</strong> 模式，避免明文保存密码。
        </>
      }
    >
      {/* 启用开关行 */}
      <EnableRow
        icon={<Link2 size={16} />}
        title="启用 Alist 存储"
        description="启用后可将 Alist 挂载的网盘作为媒体库数据源"
        checked={config.enabled}
        onChange={(v) => setConfig({ ...config, enabled: v })}
        accent="purple"
        iconColorClass="text-purple-600 dark:text-purple-300"
      />

      {/* 连接配置 */}
      <FieldGroup title="连接" description="Alist 服务地址与基础路径">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="服务器地址" required fullWidth>
            <Input
              type="url"
              value={config.server_url}
              onChange={(e) => setConfig({ ...config, server_url: e.target.value })}
              placeholder="https://alist.example.com"
              disabled={!config.enabled}
            />
          </Field>
          <Field label="基础路径" hint="所有相对路径基于此路径解析">
            <Input
              type="text"
              value={config.base_path}
              onChange={(e) => setConfig({ ...config, base_path: e.target.value })}
              placeholder="/aliyun/movies"
              disabled={!config.enabled}
            />
          </Field>
          <Field label="超时（秒）">
            <Input
              type="number"
              min={1}
              value={config.timeout}
              onChange={(e) => setConfig({ ...config, timeout: parseInt(e.target.value) || 30 })}
              disabled={!config.enabled}
            />
          </Field>
        </div>
      </FieldGroup>

      {/* 鉴权 */}
      <FieldGroup title="鉴权" description="推荐使用 Token，二者都填时 Token 优先">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="用户名">
            <Input
              type="text"
              value={config.username}
              onChange={(e) => setConfig({ ...config, username: e.target.value })}
              disabled={!config.enabled}
              autoComplete="username"
            />
          </Field>
          <Field label="密码">
            <Input
              type={showPwd ? 'text' : 'password'}
              value={config.password}
              onChange={(e) => {
                setConfig({ ...config, password: e.target.value })
                setPwdDirty(true)
              }}
              disabled={!config.enabled}
              placeholder={pwdDirty ? '' : '留空保持原密码'}
              autoComplete="current-password"
              suffix={<EyeToggle visible={showPwd} onToggle={() => setShowPwd((v) => !v)} />}
            />
          </Field>
          <Field label="长期 Token（推荐）" fullWidth hint="优先于用户名密码使用，避免每次登录换取 JWT">
            <Input
              type={showToken ? 'text' : 'password'}
              value={config.token}
              onChange={(e) => {
                setConfig({ ...config, token: e.target.value })
                setTokenDirty(true)
              }}
              disabled={!config.enabled}
              placeholder={tokenDirty ? '' : '留空保持原 Token'}
              className="font-mono"
              suffix={<EyeToggle visible={showToken} onToggle={() => setShowToken((v) => !v)} />}
            />
          </Field>
        </div>
      </FieldGroup>

      {/* 性能（折叠） */}
      <FieldGroup title="性能调优" collapsible defaultOpen={false} description="影响流式播放的缓存和分块读取策略">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Field label="启用元数据缓存">
            <div className="flex items-center h-9">
              <Toggle
                checked={config.enable_cache}
                onChange={(v) => setConfig({ ...config, enable_cache: v })}
                disabled={!config.enabled}
                accent="purple"
              />
            </div>
          </Field>
          <Field label="缓存 TTL（小时）">
            <Input
              type="number"
              min={0}
              value={config.cache_ttl_hours}
              onChange={(e) => setConfig({ ...config, cache_ttl_hours: parseInt(e.target.value) || 12 })}
              disabled={!config.enabled || !config.enable_cache}
            />
          </Field>
          <Field label="块大小（MiB）">
            <Input
              type="number"
              min={1}
              value={config.read_block_size_mb}
              onChange={(e) => setConfig({ ...config, read_block_size_mb: parseInt(e.target.value) || 8 })}
              disabled={!config.enabled}
            />
          </Field>
          <Field label="每文件最大块数">
            <Input
              type="number"
              min={1}
              value={config.read_block_count}
              onChange={(e) => setConfig({ ...config, read_block_count: parseInt(e.target.value) || 4 })}
              disabled={!config.enabled}
            />
          </Field>
        </div>
      </FieldGroup>

      {toast && <Toast ok={toast.ok} msg={toast.msg} onDismiss={() => setToast(null)} />}

      <ActionBar
        inline
        secondaryActions={
          <>
            <ActionButton
              variant="secondary"
              accent="purple"
              onClick={handleTest}
              disabled={!config.enabled || !config.server_url || testing || saving}
              loading={testing}
              icon={<Wifi size={16} />}
            >
              测试连接
            </ActionButton>
            <ActionButton variant="icon" onClick={load} icon={<RefreshCw size={16} />} aria-label="刷新" />
          </>
        }
        primaryActions={
          <ActionButton
            variant="primary"
            accent="purple"
            onClick={handleSave}
            disabled={saving || testing}
            loading={saving}
            icon={<Save size={16} />}
          >
            保存配置
          </ActionButton>
        }
      />
    </SectionShell>
  )
}

// ==================== S3 Section ====================

export function S3Section() {
  const [config, setConfig] = useState<S3Config>(DEFAULT_S3)
  const [status, setStatus] = useState<S3Status | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [toast, setToast] = useState<{ ok: boolean; msg: string } | null>(null)
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
    setToast(null)
    try {
      const payload: Partial<S3Config> = { ...config }
      if (!secretDirty) delete payload.secret_key
      await storageApi.updateS3Config(payload)
      setSecretDirty(false)
      setToast({ ok: true, msg: 'S3 配置已保存' })
      await load()
    } catch (e: any) {
      setToast({ ok: false, msg: '保存失败: ' + (e.response?.data?.error || e.message) })
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    setTesting(true)
    setToast(null)
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
      setToast({ ok: true, msg: 'S3 连接测试成功' })
    } catch (e: any) {
      setToast({ ok: false, msg: e.response?.data?.error || '连接测试失败' })
    } finally {
      setTesting(false)
    }
  }

  if (loading) {
    return (
      <section className="flex items-center justify-center py-8">
        <Loader2 className="animate-spin text-primary-400" size={20} />
      </section>
    )
  }

  return (
    <SectionShell
      icon={<Database size={18} />}
      title="S3 兼容对象存储"
      subtitle="AWS S3 / MinIO / R2 / 阿里云 OSS / 腾讯云 COS / B2"
      badge={<VersionBadge accent="amber">V2.3</VersionBadge>}
      statusSlot={<StatusBadge state={toProviderState(status)} />}
      accent="amber"
      description={
        <>
          媒体库路径使用{' '}
          <code
            className="mx-0.5 rounded px-1.5 py-0.5 font-mono text-[11px] text-amber-600 dark:text-amber-300"
            style={{ background: 'rgba(245,158,11,0.08)', border: '1px solid rgba(245,158,11,0.25)' }}
          >
            s3://
          </code>
          {' '}前缀；MinIO 等自建服务请勾选 <strong>Path-Style</strong>；播放通过预签名 URL（有效期 2 小时）。
        </>
      }
    >
      {/* 启用 */}
      <EnableRow
        icon={<Database size={16} />}
        title="启用 S3 存储"
        description="启用后可将对象存储桶作为媒体库数据源"
        checked={config.enabled}
        onChange={(v) => setConfig({ ...config, enabled: v })}
        accent="amber"
        iconColorClass="text-amber-600 dark:text-amber-300"
      />

      {/* 连接 */}
      <FieldGroup title="连接" description="Endpoint、Region 与 Bucket">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field
            label="Endpoint"
            required
            fullWidth
            hint="如 https://s3.amazonaws.com、https://minio.example.com:9000"
          >
            <Input
              type="url"
              value={config.endpoint}
              onChange={(e) => setConfig({ ...config, endpoint: e.target.value })}
              placeholder="https://s3.amazonaws.com"
              disabled={!config.enabled}
            />
          </Field>
          <Field label="Region">
            <Input
              type="text"
              value={config.region}
              onChange={(e) => setConfig({ ...config, region: e.target.value })}
              placeholder="us-east-1"
              disabled={!config.enabled}
            />
          </Field>
          <Field label="Bucket" required>
            <Input
              type="text"
              value={config.bucket}
              onChange={(e) => setConfig({ ...config, bucket: e.target.value })}
              disabled={!config.enabled}
            />
          </Field>
          <Field label="基础路径前缀" hint="例如 media/，所有对象 key 以此为根">
            <Input
              type="text"
              value={config.base_path}
              onChange={(e) => setConfig({ ...config, base_path: e.target.value })}
              placeholder="media/"
              disabled={!config.enabled}
            />
          </Field>
          <Field label="Path-Style 寻址">
            <div className="flex items-center h-9 gap-2">
              <Toggle
                checked={config.path_style}
                onChange={(v) => setConfig({ ...config, path_style: v })}
                disabled={!config.enabled}
                accent="amber"
              />
              <span className="text-xs inline-flex items-center gap-1" style={{ color: 'var(--text-tertiary)' }}>
                <Server size={12} /> MinIO 等自建必选
              </span>
            </div>
          </Field>
        </div>
      </FieldGroup>

      {/* 鉴权 */}
      <FieldGroup title="鉴权" description="Access Key + Secret Key">
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <Field label="Access Key" required>
            <Input
              type="text"
              value={config.access_key}
              onChange={(e) => setConfig({ ...config, access_key: e.target.value })}
              disabled={!config.enabled}
              className="font-mono"
              autoComplete="off"
            />
          </Field>
          <Field label="Secret Key" required>
            <Input
              type={showSecret ? 'text' : 'password'}
              value={config.secret_key}
              onChange={(e) => {
                setConfig({ ...config, secret_key: e.target.value })
                setSecretDirty(true)
              }}
              disabled={!config.enabled}
              placeholder={secretDirty ? '' : '留空保持原 Secret'}
              className="font-mono"
              autoComplete="off"
              suffix={<EyeToggle visible={showSecret} onToggle={() => setShowSecret((v) => !v)} />}
            />
          </Field>
        </div>
      </FieldGroup>

      {/* 性能（折叠） */}
      <FieldGroup
        title="性能调优"
        collapsible
        defaultOpen={false}
        description="影响流式播放的缓存和分块读取策略"
      >
        <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
          <Field label="启用元数据缓存">
            <div className="flex items-center h-9">
              <Toggle
                checked={config.enable_cache}
                onChange={(v) => setConfig({ ...config, enable_cache: v })}
                disabled={!config.enabled}
                accent="amber"
              />
            </div>
          </Field>
          <Field label="缓存 TTL（小时）">
            <Input
              type="number"
              min={0}
              value={config.cache_ttl_hours}
              onChange={(e) => setConfig({ ...config, cache_ttl_hours: parseInt(e.target.value) || 24 })}
              disabled={!config.enabled || !config.enable_cache}
            />
          </Field>
          <Field label="块大小（MiB）">
            <Input
              type="number"
              min={1}
              value={config.read_block_size_mb}
              onChange={(e) => setConfig({ ...config, read_block_size_mb: parseInt(e.target.value) || 8 })}
              disabled={!config.enabled}
            />
          </Field>
          <Field label="每文件最大块数">
            <Input
              type="number"
              min={1}
              value={config.read_block_count}
              onChange={(e) => setConfig({ ...config, read_block_count: parseInt(e.target.value) || 4 })}
              disabled={!config.enabled}
            />
          </Field>
        </div>
      </FieldGroup>

      {toast && <Toast ok={toast.ok} msg={toast.msg} onDismiss={() => setToast(null)} />}

      <ActionBar
        inline
        secondaryActions={
          <>
            <ActionButton
              variant="secondary"
              accent="amber"
              onClick={handleTest}
              disabled={!config.enabled || !config.endpoint || !config.bucket || testing || saving}
              loading={testing}
              icon={<Wifi size={16} />}
            >
              测试连接
            </ActionButton>
            <ActionButton variant="icon" onClick={load} icon={<RefreshCw size={16} />} aria-label="刷新" />
          </>
        }
        primaryActions={
          <ActionButton
            variant="primary"
            accent="amber"
            onClick={handleSave}
            disabled={saving || testing}
            loading={saving}
            icon={<Save size={16} />}
          >
            保存配置
          </ActionButton>
        }
      />
    </SectionShell>
  )
}
