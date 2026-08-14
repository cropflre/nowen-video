import { useCallback, useEffect, useMemo, useState } from 'react'
import { libraryApi, storageApi } from '@/api'
import type { AlistStatus, S3Status, WebDAVConfig, WebDAVStatus } from '@/api/storage'
import type { Library } from '@/types'
import { getLibraryPaths } from '@/types'
import {
  Cloud,
  Database,
  HardDrive,
  Link2,
  Loader2,
  RefreshCw,
  Save,
  Server,
  Wifi,
} from 'lucide-react'
import { AdminPanel } from './AdminPrimitives'
import { AlistSection, S3Section } from './RemoteStorageSections'
import {
  ActionBar,
  ActionButton,
  EnableRow,
  EyeToggle,
  Field,
  FieldGroup,
  Input,
  ProviderCard,
  SectionShell,
  StatusBadge,
  Toast,
  Toggle,
  type ProviderState,
} from './storage/StorageUIKit'

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

function ProtocolCode({ children }: { children: string }) {
  return (
    <code className="mx-0.5 rounded-[var(--nv-radius-sm)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-1.5 py-0.5 font-mono text-[11px] text-[var(--nv-text-secondary)]">
      {children}
    </code>
  )
}

export default function StorageTab() {
  const [config, setConfig] = useState<WebDAVConfig>(DEFAULT_CONFIG)
  const [status, setStatus] = useState<WebDAVStatus | null>(null)
  const [libraries, setLibraries] = useState<Library[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [testing, setTesting] = useState(false)
  const [toast, setToast] = useState<{ ok: boolean; msg: string } | null>(null)
  const [loadWarning, setLoadWarning] = useState<string | null>(null)
  const [showPassword, setShowPassword] = useState(false)
  const [passwordDirty, setPasswordDirty] = useState(false)
  const [registeringLib, setRegisteringLib] = useState<string | null>(null)
  const [alistStatus, setAlistStatus] = useState<AlistStatus | null>(null)
  const [s3Status, setS3Status] = useState<S3Status | null>(null)
  const [activeTab, setActiveTab] = useState<ProviderKey>('webdav')

  const loadAll = useCallback(async () => {
    setLoading(true)
    setLoadWarning(null)
    try {
      const [cfgResult, webdavStatusResult, libsResult, aggregateStatusResult] = await Promise.allSettled([
        storageApi.getWebDAVConfig(),
        storageApi.getWebDAVStatus(),
        libraryApi.list(),
        storageApi.getStorageStatus(),
      ])

      const failed: string[] = []
      if (cfgResult.status === 'fulfilled') {
        setConfig({ ...DEFAULT_CONFIG, ...cfgResult.value.data.data })
        setPasswordDirty(false)
      } else {
        failed.push('WebDAV 配置')
      }

      if (webdavStatusResult.status === 'fulfilled') {
        setStatus(webdavStatusResult.value.data.data)
      } else {
        failed.push('WebDAV 状态')
      }

      if (libsResult.status === 'fulfilled') {
        setLibraries(libsResult.value.data.data || [])
      } else {
        failed.push('媒体库列表')
      }

      if (aggregateStatusResult.status === 'fulfilled') {
        setAlistStatus(aggregateStatusResult.value.data.data.alist || null)
        setS3Status(aggregateStatusResult.value.data.data.s3 || null)
      } else {
        failed.push('存储聚合状态')
      }

      if (failed.length > 0) {
        setLoadWarning(`${failed.join('、')}加载失败；其他已成功加载的存储配置仍可继续使用。`)
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void loadAll()
  }, [loadAll])

  const handleSave = async () => {
    setSaving(true)
    setToast(null)
    try {
      const payload: Partial<WebDAVConfig> = { ...config }
      if (!passwordDirty) payload.password = ''
      await storageApi.updateWebDAVConfig(payload)
      setToast({ ok: true, msg: 'WebDAV 配置已保存' })
      await loadAll()
    } catch (error: any) {
      setToast({ ok: false, msg: error?.response?.data?.error || '保存失败' })
    } finally {
      setSaving(false)
    }
  }

  const handleTest = async () => {
    setTesting(true)
    setToast(null)
    try {
      await storageApi.testWebDAVConnection({
        server_url: config.server_url.trim(),
        username: config.username.trim(),
        password: passwordDirty ? config.password : '',
        base_path: config.base_path.trim() || '/',
      })
      setToast({ ok: true, msg: 'WebDAV 连接测试成功' })
    } catch (error: any) {
      setToast({ ok: false, msg: error?.response?.data?.error || '连接测试失败' })
    } finally {
      setTesting(false)
    }
  }

  const handleRegisterLib = async (libraryId: string) => {
    if (registeringLib) return
    setRegisteringLib(libraryId)
    try {
      await storageApi.registerWebDAVLibrary(libraryId)
      setToast({ ok: true, msg: '已为媒体库注册 WebDAV 存储' })
      await loadAll()
    } catch (error: any) {
      setToast({ ok: false, msg: error?.response?.data?.error || '注册失败' })
    } finally {
      setRegisteringLib(null)
    }
  }

  const providers = useMemo(
    () => [
      {
        key: 'local' as const,
        name: '本地存储',
        subtitle: '文件系统直读',
        icon: <HardDrive size={20} />,
        state: 'connected' as ProviderState,
      },
      {
        key: 'webdav' as const,
        name: 'WebDAV',
        subtitle: status?.server_url || '远程文件协议',
        icon: <Cloud size={20} />,
        state: toState(status?.enabled, status?.connected),
      },
      {
        key: 'alist' as const,
        name: 'Alist 聚合网盘',
        subtitle: '阿里云盘 / 115 / 夸克 等',
        icon: <Server size={20} />,
        state: toState(alistStatus?.enabled, alistStatus?.connected),
      },
      {
        key: 's3' as const,
        name: 'S3 对象存储',
        subtitle: 'AWS S3 / MinIO / R2 / OSS / COS',
        icon: <Database size={20} />,
        state: toState(s3Status?.enabled, s3Status?.connected),
      },
    ],
    [status, alistStatus, s3Status],
  )

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20">
        <Loader2 className="animate-spin text-[var(--nv-action-primary)]" size={28} />
      </div>
    )
  }

  return (
    <div className="space-y-6">
      <AdminPanel
        icon={<HardDrive size={18} className="text-[var(--nv-action-primary)]" />}
        title="存储概览"
        description="查看各存储 Provider 的连接状态，并切换到对应配置。"
        actions={
          <ActionButton variant="icon" onClick={() => void loadAll()} icon={<RefreshCw size={16} />} aria-label="刷新状态" />
        }
      >
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
          {providers.map((provider) => (
            <ProviderCard
              key={provider.key}
              name={provider.name}
              subtitle={provider.subtitle}
              icon={provider.icon}
              state={provider.state}
              active={activeTab === provider.key}
              onClick={() => setActiveTab(provider.key)}
            />
          ))}
        </div>
        {loadWarning && (
          <div className="mt-4">
            <Toast ok={false} msg={loadWarning} onDismiss={() => setLoadWarning(null)} />
          </div>
        )}
      </AdminPanel>

      {activeTab === 'local' && (
        <SectionShell
          icon={<HardDrive size={18} />}
          title="本地存储"
          subtitle="直接读取宿主机挂载的目录"
          statusSlot={<StatusBadge state="connected" label="始终启用" />}
          description="本地存储始终启用，媒体库路径使用标准文件系统路径（如 /vol01/Media/电影），无需额外配置。"
        >
          <div className="rounded-[var(--nv-radius-control)] border border-dashed border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] px-4 py-8 text-center text-sm text-[var(--nv-text-tertiary)]">
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
                媒体库路径使用 <ProtocolCode>webdav://</ProtocolCode> 前缀，扫描时会自动从远程目录拉取文件列表。
                {status?.error && (
                  <div className="mt-2 rounded-[var(--nv-radius-sm)] border border-[color-mix(in_srgb,var(--nv-status-danger)_28%,var(--nv-border-subtle))] bg-[color-mix(in_srgb,var(--nv-status-danger)_8%,var(--nv-bg-surface))] px-2.5 py-2 text-[var(--nv-status-danger)]">
                    <strong>错误：</strong>
                    {status.error}
                  </div>
                )}
              </>
            }
          >
            <EnableRow
              icon={<Link2 size={16} />}
              title="启用 WebDAV 存储"
              description="启用后可为媒体库挂载远程 WebDAV 目录"
              checked={config.enabled}
              onChange={(value) => setConfig({ ...config, enabled: value })}
            />

            <FieldGroup title="连接" description="WebDAV 服务器地址与基础路径">
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Field label="服务器地址" required fullWidth>
                  <Input
                    type="url"
                    value={config.server_url}
                    onChange={(event) => setConfig({ ...config, server_url: event.target.value })}
                    placeholder="https://webdav.example.com/dav/"
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="基础路径" hint="所有媒体库路径基于此路径解析" fullWidth>
                  <Input
                    type="text"
                    value={config.base_path}
                    onChange={(event) => setConfig({ ...config, base_path: event.target.value })}
                    placeholder="/ 或 /media/videos"
                    disabled={!config.enabled}
                  />
                </Field>
              </div>
            </FieldGroup>

            <FieldGroup title="鉴权" description="HTTP Basic 认证凭据">
              <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
                <Field label="用户名">
                  <Input
                    type="text"
                    value={config.username}
                    onChange={(event) => setConfig({ ...config, username: event.target.value })}
                    disabled={!config.enabled}
                    autoComplete="username"
                  />
                </Field>
                <Field label="密码">
                  <Input
                    type={showPassword ? 'text' : 'password'}
                    value={config.password}
                    onChange={(event) => {
                      setConfig({ ...config, password: event.target.value })
                      setPasswordDirty(true)
                    }}
                    disabled={!config.enabled}
                    placeholder={passwordDirty ? '' : '留空保持原密码'}
                    autoComplete="current-password"
                    suffix={
                      <EyeToggle
                        visible={showPassword}
                        onToggle={() => setShowPassword((value) => !value)}
                      />
                    }
                  />
                </Field>
              </div>
            </FieldGroup>

            <FieldGroup
              title="性能与可靠性"
              collapsible
              defaultOpen={false}
              description="连接池、缓存、重试等高级参数"
            >
              <div className="grid grid-cols-2 gap-4 md:grid-cols-3">
                <Field label="请求超时（秒）">
                  <Input
                    type="number"
                    min={1}
                    value={config.timeout}
                    onChange={(event) => setConfig({ ...config, timeout: Math.max(1, Number(event.target.value) || 1) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="连接池大小">
                  <Input
                    type="number"
                    min={1}
                    value={config.pool_size}
                    onChange={(event) => setConfig({ ...config, pool_size: Math.max(1, Number(event.target.value) || 1) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="最大重试次数">
                  <Input
                    type="number"
                    min={0}
                    value={config.max_retries}
                    onChange={(event) => setConfig({ ...config, max_retries: Math.max(0, Number(event.target.value) || 0) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="重试间隔（秒）">
                  <Input
                    type="number"
                    min={1}
                    value={config.retry_interval}
                    onChange={(event) => setConfig({ ...config, retry_interval: Math.max(1, Number(event.target.value) || 1) })}
                    disabled={!config.enabled}
                  />
                </Field>
                <Field label="启用元数据缓存">
                  <div className="flex h-9 items-center">
                    <Toggle
                      checked={config.enable_cache}
                      onChange={(value) => setConfig({ ...config, enable_cache: value })}
                      disabled={!config.enabled}
                    />
                  </div>
                </Field>
                <Field label="缓存 TTL（小时）">
                  <Input
                    type="number"
                    min={0}
                    value={config.cache_ttl_hours}
                    onChange={(event) => setConfig({ ...config, cache_ttl_hours: Math.max(0, Number(event.target.value) || 0) })}
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
                    onClick={() => void handleTest()}
                    disabled={!config.enabled || !config.server_url.trim() || testing || saving}
                    loading={testing}
                    icon={<Wifi size={16} />}
                  >
                    测试连接
                  </ActionButton>
                  <ActionButton variant="icon" onClick={() => void loadAll()} icon={<RefreshCw size={16} />} aria-label="刷新" />
                </>
              }
              primaryActions={
                <ActionButton
                  variant="primary"
                  onClick={() => void handleSave()}
                  disabled={saving || testing || (config.enabled && !config.server_url.trim())}
                  loading={saving}
                  icon={<Save size={16} />}
                >
                  保存配置
                </ActionButton>
              }
            />
          </SectionShell>

          {status?.enabled && status?.connected && (
            <SectionShell
              icon={<Link2 size={18} />}
              title="媒体库挂载"
              subtitle="将 WebDAV 注册为指定媒体库的数据源"
              description={
                <>
                  将远程存储挂载到媒体库后，扫描会按路径协议读取远程文件。支持{' '}
                  <ProtocolCode>webdav://</ProtocolCode>、<ProtocolCode>alist://</ProtocolCode> 和{' '}
                  <ProtocolCode>s3://</ProtocolCode>。
                </>
              }
            >
              {libraries.length === 0 ? (
                <div className="rounded-[var(--nv-radius-control)] border border-dashed border-[var(--nv-border-default)] bg-[var(--nv-bg-surface-soft)] px-4 py-10 text-center text-sm text-[var(--nv-text-tertiary)]">
                  暂无媒体库
                </div>
              ) : (
                <ul className="space-y-2">
                  {libraries.map((library) => {
                    const paths = getLibraryPaths(library)
                    const remotePath = paths.find((path) => /^(webdav|alist|s3):\/\//i.test(path))
                    const normalizedRemote = remotePath?.toLowerCase() || ''
                    const isWebDAV = normalizedRemote.startsWith('webdav://')
                    const isAlist = normalizedRemote.startsWith('alist://')
                    const isS3 = normalizedRemote.startsWith('s3://')
                    const isRemote = Boolean(remotePath)
                    const providerLabel = isWebDAV ? 'WebDAV' : isAlist ? 'Alist' : isS3 ? 'S3' : '本地'
                    const ProviderIcon = isWebDAV ? Cloud : isAlist ? Server : isS3 ? Database : HardDrive
                    const displayPath = paths.length > 1 ? `${paths[0]} +${paths.length - 1}` : paths[0] || library.path

                    return (
                      <li
                        key={library.id}
                        className="flex items-center justify-between gap-3 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-3 transition-colors hover:border-[var(--nv-border-hover)]"
                      >
                        <div className="flex min-w-0 items-center gap-3">
                          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface)] text-[var(--nv-text-tertiary)]">
                            <ProviderIcon size={16} />
                          </div>
                          <div className="min-w-0">
                            <div className="truncate text-sm font-medium text-[var(--nv-text-primary)]">
                              {library.name}
                            </div>
                            <div className="truncate font-mono text-[11px] text-[var(--nv-text-tertiary)]" title={paths.join('\n')}>
                              {displayPath}
                            </div>
                          </div>
                        </div>
                        {!isRemote ? (
                          <ActionButton
                            variant="secondary"
                            onClick={() => void handleRegisterLib(library.id)}
                            disabled={registeringLib !== null}
                            loading={registeringLib === library.id}
                            icon={<Link2 size={14} />}
                            className="shrink-0"
                          >
                            挂载 WebDAV
                          </ActionButton>
                        ) : (
                          <StatusBadge state="connected" label={providerLabel} size="sm" />
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
