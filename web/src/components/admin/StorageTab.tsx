import { useEffect, useMemo, useState, useCallback } from 'react'
import { storageApi, libraryApi } from '@/api'
import type { WebDAVConfig, WebDAVStatus, AlistStatus, S3Status } from '@/api/storage'
import type { Library } from '@/types'
import {
  HardDrive,
  Cloud,
  Loader2,
  Save,
  Wifi,
  Link2,
  Database,
  Server,
  RefreshCw,
} from 'lucide-react'
import { AlistSection, S3Section } from './RemoteStorageSections'
import {
  ProviderCard,
  SectionShell,
  FieldGroup,
  Field,
  Input,
  Toggle,
  ActionBar,
  ActionButton,
  Toast,
  StatusBadge,
  EyeToggle,
  EnableRow,
  type ProviderState,
} from './storage/StorageUIKit'

// ==================== 存储管理标签页 ====================
// V2.3 UI 重构：四 provider 统一 Provider Registry 布局
//   顶部：全局概览（Local / WebDAV / Alist / S3 四张卡片，充当 Tab 入口）
//   下部：当前选中 provider 的完整配置表单
// WebDAV 直接在此文件实现；Alist / S3 复用 RemoteStorageSections

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

type ProviderKey = 'local' | 'webdav' | 'alist' | 's3'

function toState(enabled?: boolean, connected?: boolean): ProviderState {
  if (!enabled) return 'disabled'
  if (connected) return 'connected'
  return 'error'
}

export default function StorageTab() {
  // WebDAV 本地 state
  const [config, setConfig] = useState<WebDAVConfig>(DEFAULT_CONFIG)
  const [status, setStatus] = useState<WebDAVStatus | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [toast, setToast] = useState<{ ok: boolean; msg: string } | null>(null)
  const [showPassword, setShowPassword] = useState(false)
  const [passwordDirty, setPasswordDirty] = useState(false)
  const [registeringLib, setRegisteringLib] = useState<string | null>(null)

  // Alist / S3 状态（仅用于概览卡，详细表单仍由各自 Section 自己拉取）
  const [alistStatus, setAlistStatus] = useState<AlistStatus | null>(null)
  const [s3Status, setS3Status] = useState<S3Status | null>(null)

  // 当前激活的 provider tab
  const [activeTab, setActiveTab] = useState<ProviderKey>('webdav')

  // 加载 WebDAV 配置 + 统一状态概览 + 媒体库
  const loadAll = useCallback(async () => {
    setLoading(true)
    try {
      const [cfgRes, webdavStatusRes, libsRes, aggregateStatusRes] = await Promise.all([
        storageApi.getWebDAVConfig(),
        storageApi.getWebDAVStatus(),
        libraryApi.list(),
        storageApi.getStorageStatus().catch(() => null),
      ])
      setConfig({ ...DEFAULT_CONFIG, ...cfgRes.data.data })
      setStatus(webdavStatusRes.data.data)
      setLibraries(libsRes.data.data || [])
      setPasswordDirty(false)
      if (aggregateStatusRes) {
        setAlistStatus(aggregateStatusRes.data.data.alist || null)
        setS3Status(aggregateStatusRes.data.data.s3 || null)
      }
    } catch (e: any) {
      console.error('加载存储配置失败', e)
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    loadAll()
  }, [loadAll])

  // ----- WebDAV 动作 -----
  const handleSave = async () => {
    setSaving(true)
    setToast(null)
    try {
      const payload: Partial<WebDAVConfig> = { ...config }
      if (!passwordDirty) payload.password = ''
      await storageApi.updateWebDAVConfig(payload)
      setToast({ ok: true, msg: 'WebDAV 配置已保存' })
      await loadAll()
    } catch (e: any) {
      setToast({ ok: false, msg: e?.response?.data?.error || '保存失败' })
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    setTesting(true)
    setToast(null)
    try {
      await storageApi.testWebDAVConnection({
        server_url: config.server_url,
        username: config.username,
        password: passwordDirty ? config.password : '',
        base_path: config.base_path,
      })
      setToast({ ok: true, msg: 'WebDAV 连接测试成功' })
    } catch (e: any) {
      setToast({ ok: false, msg: e?.response?.data?.error || '连接测试失败' })
    } finally {
      setTesting(false)
    }
  }

  const handleRegisterLib = async (libId: string) => {
    setRegisteringLib(libId)
    try {
      await storageApi.registerWebDAVLibrary(libId)
      setToast({ ok: true, msg: '已为媒体库注册 WebDAV 存储' })
      await loadAll()
    } catch (e: any) {
      setToast({ ok: false, msg: e?.response?.data?.error || '注册失败' })
    } finally {
      setRegisteringLib(null)
    }
  }

  // 概览卡数据
  const providers = useMemo(
    () => [
      {
        key: 'local' as const,
        name: '本地存储',
        subtitle: '文件系统直读',
        icon: <HardDrive size={20} />,
        state: 'connected' as ProviderState,
        accent: 'emerald' as const,
      },
      {
        key: 'webdav' as const,
        name: 'WebDAV',
        subtitle: status?.server_url || '远程文件协议',
        icon: <Cloud size={20} />,
        state: toState(status?.enabled, status?.connected),
        accent: 'blue' as const,
      },
      {
        key: 'alist' as const,
        name: 'Alist 聚合网盘',
        subtitle: '阿里云盘 / 115 / 夸克 等',
        icon: <Server size={20} />,
        state: toState(alistStatus?.enabled, alistStatus?.connected),
        accent: 'purple' as const,
      },
      {
        key: 's3' as const,
        name: 'S3 对象存储',
        subtitle: 'AWS S3 / MinIO / R2 / OSS / COS',
        icon: <Database size={20} />,
        state: toState(s3Status?.enabled, s3Status?.connected),
        accent: 'amber' as const,
      },
    ],
    [status, alistStatus, s3Status]
  )

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="animate-spin text-primary-400" size={32} />
      </div>
    )
  }

  return (
    <div className="space-y-8 animate-fade-in">
      {/* ============ 顶部：存储概览 + Tab 切换 ============ */}
      <section>
        <div className="mb-4 flex items-end justify-between gap-4">
          <div>
            <h2
              className="flex items-center gap-2 font-display text-lg font-semibold tracking-wide"
              style={{ color: 'var(--text-primary)' }}
            >
              <HardDrive size={20} className="text-primary-400" />
              存储概览
            </h2>
            <p className="mt-1 text-xs" style={{ color: 'var(--text-tertiary)' }}>
              点击下方卡片切换 provider 配置
            </p>
          </div>
          <ActionButton variant="icon" onClick={loadAll} icon={<RefreshCw size={16} />} aria-label="刷新状态" />
        </div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {providers.map((p) => (
            <ProviderCard
              key={p.key}
              name={p.name}
              subtitle={p.subtitle}
              icon={p.icon}
              state={p.state}
              accent={p.accent}
              active={activeTab === p.key}
              onClick={p.key === 'local' ? undefined : () => setActiveTab(p.key)}
            />
          ))}
        </div>
      </section>

      {/* ============ 详情区：根据激活 tab 渲染 ============ */}
      {activeTab === 'local' && (
        <SectionShell
          icon={<HardDrive size={18} />}
          title="本地存储"
          subtitle="直接读取宿主机挂载的目录"
          statusSlot={<StatusBadge state="connected" label="始终启用" />}
          description="本地存储始终启用，媒体库路径使用标准文件系统路径（如 /vol01/Media/电影）。无需额外配置。"
        >
          <div className="text-sm text-center py-4" style={{ color: 'var(--text-tertiary)' }}>
            本地存储无配置项。请在「媒体库」菜单中新增本地路径的媒体库。
          </div>
        </SectionShell>
      )}

      {activeTab === 'webdav' && (
        <>
          <SectionShell
            icon={<Cloud size={18} />}
            title="WebDAV 存储"
            subtitle="兼容坚果云 / Nextcloud / ownCloud / Synology WebDAV"
            statusSlot={<StatusBadge state={toState(status?.enabled, status?.connected)} />}
            description={
              <>
                媒体库路径使用{' '}
                <code
                  className="mx-0.5 rounded px-1.5 py-0.5 font-mono text-[11px]"
                  style={{
                    background: 'var(--nav-hover-bg)',
                    color: 'var(--neon-blue)',
                    border: '1px solid var(--border-default)',
                  }}
                >
                  webdav://
                </code>{' '}
                前缀。扫描时会自动从远程目录拉取文件列表。
                {status?.error && (
                  <div className="mt-2 text-red-500 dark:text-red-400">
                    <strong>错误：</strong>{status.error}
                  </div>
                )}
              </>
            }
          >
            {/* 启用开关 */}
            <EnableRow
              icon={<Link2 size={16} />}
              title="启用 WebDAV 存储"
              description="启用后可为媒体库挂载远程 WebDAV 目录"
              checked={config.enabled}
              onChange={(v) => setConfig({ ...config, enabled: v })}
              accent="neon"
            />

            {/* 连接 */}
            <FieldGroup title="连接" description="WebDAV 服务器地址与基础路径">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                <Field label="服务器地址" required fullWidth>
                  <Input
                    type="url"
                    value={config.server_url}
                    onChange={(e) => setConfig({ ...config, server_url: e.target.value })}
                    placeholder="https://webdav.example.com/dav/"
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="基础路径" hint="所有媒体库路径基于此路径解析" fullWidth>
                  <Input
                    type="text"
                    value={config.base_path}
                    onChange={(e) => setConfig({ ...config, base_path: e.target.value })}
                    placeholder="/ 或 /media/videos"
                    disabled={!config.enabled}
                  />
                </Field>
              </div>
            </FieldGroup>

            {/* 鉴权 */}
            <FieldGroup title="鉴权" description="HTTP Basic 认证凭据">
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
                    type={showPassword ? 'text' : 'password'}
                    value={config.password}
                    onChange={(e) => {
                      setConfig({ ...config, password: e.target.value })
                      setPasswordDirty(true)
                    }}
                    disabled={!config.enabled}
                    placeholder={passwordDirty ? '' : '留空保持原密码'}
                    autoComplete="current-password"
                    suffix={
                      <EyeToggle
                        visible={showPassword}
                        onToggle={() => setShowPassword((v) => !v)}
                      />
                    }
                  />
                </Field>
              </div>
            </FieldGroup>

            {/* 性能与可靠性（折叠） */}
            <FieldGroup
              title="性能与可靠性"
              collapsible
              defaultOpen={false}
              description="连接池、缓存、重试等高级参数"
            >
              <div className="grid grid-cols-2 md:grid-cols-3 gap-4">
                <Field label="请求超时（秒）">
                  <Input
                    type="number"
                    min={1}
                    value={config.timeout}
                    onChange={(e) => setConfig({ ...config, timeout: Number(e.target.value) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="连接池大小">
                  <Input
                    type="number"
                    min={1}
                    value={config.pool_size}
                    onChange={(e) => setConfig({ ...config, pool_size: Number(e.target.value) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="最大重试次数">
                  <Input
                    type="number"
                    min={0}
                    value={config.max_retries}
                    onChange={(e) => setConfig({ ...config, max_retries: Number(e.target.value) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="重试间隔（秒）">
                  <Input
                    type="number"
                    min={1}
                    value={config.retry_interval}
                    onChange={(e) => setConfig({ ...config, retry_interval: Number(e.target.value) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="启用元数据缓存">
                  <div className="flex items-center h-9">
                    <Toggle
                      checked={config.enable_cache}
                      onChange={(v) => setConfig({ ...config, enable_cache: v })}
                      disabled={!config.enabled}
                      accent="neon"
                    />
                  </div>
                </Field>
                <Field label="缓存 TTL（小时）">
                  <Input
                    type="number"
                    min={0}
                    value={config.cache_ttl_hours}
                    onChange={(e) => setConfig({ ...config, cache_ttl_hours: Number(e.target.value) })}
                    disabled={!config.enabled || !config.enable_cache}
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
                    accent="neon"
                    onClick={handleTest}
                    disabled={!config.enabled || !config.server_url || testing || saving}
                    loading={testing}
                    icon={<Wifi size={16} />}
                  >
                    测试连接
                  </ActionButton>
                  <ActionButton
                    variant="icon"
                    onClick={loadAll}
                    icon={<RefreshCw size={16} />}
                    aria-label="刷新"
                  />
                </>
              }
              primaryActions={
                <ActionButton
                  variant="primary"
                  accent="neon"
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

          {/* 媒体库挂载（仅 WebDAV 可用时显示） */}
          {status?.enabled && status?.connected && (
            <SectionShell
              icon={<Link2 size={18} />}
              title="媒体库挂载"
              subtitle="将 WebDAV 注册为指定媒体库的数据源"
              description={
                <>
                  将 WebDAV 存储挂载到媒体库后，扫描时会自动从远程读取文件。
                  媒体库路径需以{' '}
                  <code
                    className="mx-0.5 rounded px-1.5 py-0.5 font-mono text-[11px]"
                    style={{ background: 'var(--nav-hover-bg)', color: 'var(--neon-blue)', border: '1px solid var(--border-default)' }}
                  >
                    webdav://
                  </code>
                  、
                  <code
                    className="mx-0.5 rounded px-1.5 py-0.5 font-mono text-[11px] text-purple-600 dark:text-purple-300"
                    style={{ background: 'rgba(168,85,247,0.08)', border: '1px solid rgba(168,85,247,0.2)' }}
                  >
                    alist://
                  </code>
                  {' '}或{' '}
                  <code
                    className="mx-0.5 rounded px-1.5 py-0.5 font-mono text-[11px] text-amber-600 dark:text-amber-300"
                    style={{ background: 'rgba(245,158,11,0.08)', border: '1px solid rgba(245,158,11,0.2)' }}
                  >
                    s3://
                  </code>
                  {' '}开头才会走远程存储。
                </>
              }
            >
              {libraries.length === 0 ? (
                <div
                  className="rounded-lg py-10 text-center text-sm"
                  style={{
                    color: 'var(--text-tertiary)',
                    border: '1px dashed var(--storage-enable-row-border, var(--border-strong))',
                  }}
                >
                  暂无媒体库
                </div>
              ) : (
                <ul className="space-y-2">
                  {libraries.map((lib) => {
                    const isWebDAV = lib.path?.startsWith('webdav://')
                    const isAlist = lib.path?.startsWith('alist://')
                    const isS3 = lib.path?.startsWith('s3://')
                    const isRemote = isWebDAV || isAlist || isS3
                    return (
                      <li
                        key={lib.id}
                        className="flex items-center justify-between gap-3 rounded-lg p-3 transition-colors"
                        style={{
                          background: 'var(--storage-enable-row-bg, var(--nav-hover-bg))',
                          border: '1px solid var(--storage-enable-row-border, var(--border-strong))',
                        }}
                        onMouseEnter={(e) => (e.currentTarget.style.borderColor = 'var(--neon-blue)')}
                        onMouseLeave={(e) => (e.currentTarget.style.borderColor = 'var(--storage-enable-row-border, var(--border-strong))')}
                      >
                        <div className="flex items-center gap-3 min-w-0">
                          <div
                            className={
                              'flex h-9 w-9 shrink-0 items-center justify-center rounded-lg ' +
                              (isWebDAV
                                ? 'bg-primary-400/10 text-primary-600 dark:text-primary-300'
                                : isAlist
                                ? 'bg-purple-500/10 text-purple-600 dark:text-purple-300'
                                : isS3
                                ? 'bg-amber-500/10 text-amber-600 dark:text-amber-300'
                                : 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-300')
                            }
                          >
                            {isWebDAV ? (
                              <Cloud size={16} />
                            ) : isAlist ? (
                              <Server size={16} />
                            ) : isS3 ? (
                              <Database size={16} />
                            ) : (
                              <HardDrive size={16} />
                            )}
                          </div>
                          <div className="min-w-0">
                            <div
                              className="truncate text-sm font-medium"
                              style={{ color: 'var(--text-primary)' }}
                            >
                              {lib.name}
                            </div>
                            <div
                              className="truncate text-[11px] font-mono"
                              style={{ color: 'var(--text-tertiary)' }}
                            >
                              {lib.path}
                            </div>
                          </div>
                        </div>
                        {!isRemote && (
                          <ActionButton
                            variant="secondary"
                            accent="neon"
                            onClick={() => handleRegisterLib(lib.id)}
                            disabled={registeringLib === lib.id}
                            loading={registeringLib === lib.id}
                            icon={<Link2 size={14} />}
                            className="flex-shrink-0"
                          >
                            挂载 WebDAV
                          </ActionButton>
                        )}
                        {isRemote && (
                          <StatusBadge
                            state="connected"
                            label={isWebDAV ? 'WebDAV' : isAlist ? 'Alist' : 'S3'}
                            size="sm"
                          />
                        )}
                      </li>
                    )
                  })}
                </ul>
              )}
            </SectionShell>
          )}
        </>
      )}

      {activeTab === 'alist' && <AlistSection />}
      {activeTab === 's3' && <S3Section />}
    </div>
  )
}
