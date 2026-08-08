import { useCallback, useEffect, useState } from 'react'
import { Check, ExternalLink, Eye, EyeOff, Film, Key, Loader2, ShieldAlert, Trash2, Wifi, X } from 'lucide-react'
import { adminApi } from '@/api'
import type { DoubanConfigStatus, TMDbConfigStatus } from '@/types'
import { useTranslation } from '@/i18n'
import { useDialog } from '@/components/Dialog'
import { Button, Input, Tag } from '@/components/design-system'
import { AdminPanel, AdminStatus } from './AdminPrimitives'
import TMDbProxySettings from './TMDbProxySettings'
import DoubanCookieImportModal from './DoubanCookieImportModal'

type Feedback = { type: 'success' | 'error' | 'info'; text: string }

function FeedbackBanner({ feedback }: { feedback: Feedback }) {
  const tone = feedback.type === 'success' ? 'success' : feedback.type === 'error' ? 'danger' : 'active'
  return (
    <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3">
      <AdminStatus tone={tone}>
        {feedback.type === 'success' ? <Check size={13} /> : feedback.type === 'error' ? <X size={13} /> : <Loader2 size={13} className="animate-spin" />}
        {feedback.text}
      </AdminStatus>
    </div>
  )
}

export default function MetadataSettings() {
  const { t } = useTranslation()
  const dialog = useDialog()
  const [tmdbConfig, setTmdbConfig] = useState<TMDbConfigStatus | null>(null)
  const [doubanConfig, setDoubanConfig] = useState<DoubanConfigStatus | null>(null)
  const [tmdbKeyInput, setTmdbKeyInput] = useState('')
  const [tmdbEditing, setTmdbEditing] = useState(false)
  const [tmdbShowKey, setTmdbShowKey] = useState(false)
  const [tmdbSaving, setTmdbSaving] = useState(false)
  const [tmdbTesting, setTmdbTesting] = useState(false)
  const [tmdbMessage, setTmdbMessage] = useState<Feedback | null>(null)
  const [doubanCookieInput, setDoubanCookieInput] = useState('')
  const [doubanEditing, setDoubanEditing] = useState(false)
  const [doubanShowCookie, setDoubanShowCookie] = useState(false)
  const [doubanSaving, setDoubanSaving] = useState(false)
  const [doubanValidating, setDoubanValidating] = useState(false)
  const [doubanMessage, setDoubanMessage] = useState<Feedback | null>(null)
  const [importOpen, setImportOpen] = useState(false)

  useEffect(() => {
    let cancelled = false
    Promise.all([adminApi.getTMDbConfig(), adminApi.getDoubanConfig()])
      .then(([tmdbResponse, doubanResponse]) => {
        if (cancelled) return
        setTmdbConfig(tmdbResponse.data.data)
        setDoubanConfig(doubanResponse.data.data)
      })
      .catch(() => {
        // 保持 AdminPage 旧行为：配置初始化失败不阻断其他管理功能。
      })
    return () => { cancelled = true }
  }, [])

  const showTmdbMessage = useCallback((type: Feedback['type'], text: string) => {
    setTmdbMessage({ type, text })
    window.setTimeout(() => setTmdbMessage(null), 5000)
  }, [])

  const showDoubanMessage = useCallback((type: Feedback['type'], text: string) => {
    setDoubanMessage({ type, text })
    window.setTimeout(() => setDoubanMessage(null), 5000)
  }, [])

  const saveTmdbKey = async () => {
    const key = tmdbKeyInput.trim()
    if (!key) return
    setTmdbSaving(true)
    try {
      const response = await adminApi.updateTMDbConfig(key)
      setTmdbConfig(response.data.data)
      setTmdbKeyInput('')
      setTmdbEditing(false)
      setTmdbShowKey(false)
      showTmdbMessage('success', t('admin.tmdbSaveSuccess'))
    } catch (error: any) {
      showTmdbMessage('error', error?.response?.data?.error || t('admin.tmdbSaveFailed'))
    } finally {
      setTmdbSaving(false)
    }
  }

  const clearTmdbKey = async () => {
    const ok = await dialog.confirm({
      title: t('admin.tmdbClearConfirm'),
      confirmText: t('admin.confirm') || '确定',
      variant: 'danger',
    })
    if (!ok) return
    try {
      const response = await adminApi.clearTMDbConfig()
      setTmdbConfig(response.data.data)
      setTmdbKeyInput('')
      setTmdbEditing(false)
      showTmdbMessage('success', t('admin.tmdbClearSuccess'))
    } catch {
      showTmdbMessage('error', t('admin.tmdbClearFailed'))
    }
  }

  const testTmdbKey = async () => {
    const inputKey = tmdbKeyInput.trim()
    const useInput = tmdbEditing && inputKey.length > 0
    if (!useInput && !tmdbConfig?.configured) {
      showTmdbMessage('error', t('admin.tmdbTestNoKey'))
      return
    }

    setTmdbTesting(true)
    showTmdbMessage('info', t('admin.tmdbTesting'))
    try {
      const response = useInput ? await adminApi.testTMDbKey(inputKey) : await adminApi.validateTMDbConfig()
      const { valid, message } = response.data.data
      showTmdbMessage(valid ? 'success' : 'error', message || (valid ? t('admin.tmdbTestOK') : t('admin.tmdbTestFailed')))
    } catch (error: any) {
      showTmdbMessage('error', error?.response?.data?.error || t('admin.tmdbTestFailed'))
    } finally {
      setTmdbTesting(false)
    }
  }

  const saveDoubanCookie = async () => {
    const cookie = doubanCookieInput.trim()
    if (!cookie) return
    setDoubanSaving(true)
    try {
      const response = await adminApi.updateDoubanConfig(cookie)
      setDoubanConfig(response.data.data)
      setDoubanCookieInput('')
      setDoubanEditing(false)
      setDoubanShowCookie(false)
      showDoubanMessage('success', '豆瓣 Cookie 已保存')
    } catch (error: any) {
      showDoubanMessage('error', error?.response?.data?.error || '保存失败，请稍后重试')
    } finally {
      setDoubanSaving(false)
    }
  }

  const clearDoubanCookie = async () => {
    const ok = await dialog.confirm({
      title: '清除豆瓣 Cookie',
      message: '确定要清除豆瓣 Cookie 吗？清除后豆瓣刮削将回退到匿名模式（成功率较低）。',
      confirmText: '清除',
      variant: 'danger',
    })
    if (!ok) return
    try {
      const response = await adminApi.clearDoubanConfig()
      setDoubanConfig(response.data.data)
      setDoubanCookieInput('')
      setDoubanEditing(false)
      showDoubanMessage('success', '豆瓣 Cookie 已清除')
    } catch {
      showDoubanMessage('error', '清除失败，请稍后重试')
    }
  }

  const validateDoubanCookie = async () => {
    setDoubanValidating(true)
    try {
      const response = await adminApi.validateDoubanConfig()
      const { valid, message } = response.data.data
      showDoubanMessage(valid ? 'success' : 'error', message)
    } catch (error: any) {
      showDoubanMessage('error', error?.response?.data?.error || '校验失败')
    } finally {
      setDoubanValidating(false)
    }
  }

  return (
    <div className="space-y-6">
      <AdminPanel
        title={t('admin.metadataConfig')}
        description="配置 TMDb API Key，以及 API、图片和网络出口代理。"
        icon={<Film size={18} />}
      >
        <div className="space-y-5">
          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
            <p className="text-sm leading-6 text-[var(--nv-text-secondary)]">
              {t('admin.metadataConfigDesc').split('TMDb')[0]}
              <span className="font-semibold text-[var(--nv-text-primary)]">TMDb（The Movie Database）</span>
              {t('admin.metadataConfigDesc').split('TMDb（The Movie Database）')[1] || t('admin.metadataConfigDesc').split('TMDb (The Movie Database)')[1]}
            </p>
            <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noopener noreferrer" className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-[var(--nv-action-primary)] hover:underline">
              <ExternalLink size={14} />{t('admin.applyTMDbKey')}
            </a>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]">
              <Key size={18} />
            </div>
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-semibold text-[var(--nv-text-primary)]">{tmdbConfig?.configured ? t('admin.tmdbConfigured') : t('admin.tmdbNotConfigured')}</p>
                <Tag tone={tmdbConfig?.configured ? 'success' : 'neutral'}>{tmdbConfig?.configured ? '已启用' : '未配置'}</Tag>
              </div>
              {tmdbConfig?.configured && tmdbConfig.masked_key && (
                <div className="mt-1 flex items-center gap-2 text-xs font-mono text-[var(--nv-text-tertiary)]">
                  <span>{tmdbShowKey ? tmdbConfig.masked_key : '••••••••••••••••••••'}</span>
                  <button type="button" onClick={() => setTmdbShowKey((value) => !value)} className="text-[var(--nv-text-tertiary)] hover:text-[var(--nv-text-primary)]" aria-label={tmdbShowKey ? t('admin.tmdbHideKey') : t('admin.tmdbShowKey')}>
                    {tmdbShowKey ? <EyeOff size={13} /> : <Eye size={13} />}
                  </button>
                </div>
              )}
            </div>
          </div>

          {tmdbMessage && <FeedbackBanner feedback={tmdbMessage} />}

          {tmdbEditing ? (
            <div className="space-y-3">
              <div>
                <label className="mb-1.5 block text-sm font-medium text-[var(--nv-text-secondary)]">{t('admin.tmdbInputLabel')}</label>
                <Input
                  type="text"
                  value={tmdbKeyInput}
                  onChange={(event) => setTmdbKeyInput(event.target.value)}
                  placeholder={t('admin.tmdbInputPlaceholder')}
                  className="font-mono"
                  autoFocus
                  onKeyDown={(event) => event.key === 'Enter' && void saveTmdbKey()}
                />
                <p className="mt-1.5 text-xs text-[var(--nv-text-tertiary)]">{t('admin.tmdbInputHint')}</p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="primary" onClick={() => void saveTmdbKey()} disabled={!tmdbKeyInput.trim() || tmdbTesting} loading={tmdbSaving}>
                  {tmdbSaving ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}{tmdbSaving ? t('admin.saving') : t('common.save')}
                </Button>
                <Button onClick={() => void testTmdbKey()} disabled={!tmdbKeyInput.trim() || tmdbSaving} loading={tmdbTesting} title={t('admin.tmdbTestInputHint')}>
                  {tmdbTesting ? <Loader2 size={14} className="animate-spin" /> : <Wifi size={14} />}{tmdbTesting ? t('admin.tmdbTesting') : t('admin.tmdbTestBtn')}
                </Button>
                <Button variant="ghost" onClick={() => { setTmdbEditing(false); setTmdbKeyInput('') }}>{t('common.cancel')}</Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-wrap gap-2">
              <Button variant="primary" onClick={() => setTmdbEditing(true)}><Key size={14} />{tmdbConfig?.configured ? t('admin.modifyApiKey') : t('admin.configApiKey')}</Button>
              {tmdbConfig?.configured && (
                <Button onClick={() => void testTmdbKey()} loading={tmdbTesting} title={t('admin.tmdbTestSavedHint')}>
                  {tmdbTesting ? <Loader2 size={14} className="animate-spin" /> : <Wifi size={14} />}{tmdbTesting ? t('admin.tmdbTesting') : t('admin.tmdbTestConnection')}
                </Button>
              )}
              {tmdbConfig?.configured && <Button variant="danger" onClick={() => void clearTmdbKey()}><Trash2 size={14} />{t('admin.clearKey')}</Button>}
            </div>
          )}

          <TMDbProxySettings config={tmdbConfig} onConfigChange={setTmdbConfig} />

          <div className="border-t border-[var(--nv-border-subtle)] pt-4">
            <p className="mb-2 text-xs font-semibold text-[var(--nv-text-secondary)]">{t('admin.configFeatures')}</p>
            <ul className="space-y-1.5 text-xs text-[var(--nv-text-tertiary)]">
              {[t('admin.feature1'), t('admin.feature2'), t('admin.feature3')].map((feature) => (
                <li key={feature} className="flex items-center gap-2">
                  <span className={`h-1.5 w-1.5 rounded-full ${tmdbConfig?.configured ? 'bg-[var(--nv-status-success)]' : 'bg-[var(--nv-border-strong)]'}`} />
                  {feature}
                </li>
              ))}
            </ul>
          </div>
        </div>
      </AdminPanel>

      <AdminPanel
        title="豆瓣刮削登录配置"
        description="可选配置登录 Cookie 以提升豆瓣刮削成功率；不配置时继续使用匿名模式。"
        icon={<Key size={18} />}
      >
        <div className="space-y-5">
          <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
            <p className="text-sm leading-6 text-[var(--nv-text-secondary)]">Cookie 等同于豆瓣登录凭证，仅供个人刮削使用。建议通过浏览器插件导出完整 Header String，并确认其中包含 <code className="font-mono text-[var(--nv-text-primary)]">dbcl2</code>。</p>
            <a href="https://www.douban.com/" target="_blank" rel="noopener noreferrer" className="mt-3 inline-flex items-center gap-1.5 text-sm font-medium text-[var(--nv-action-primary)] hover:underline"><ExternalLink size={14} />打开豆瓣登录</a>
          </div>

          <div className="flex flex-wrap items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] text-[var(--nv-action-primary)]"><Key size={18} /></div>
            <div className="min-w-0 flex-1">
              <div className="flex flex-wrap items-center gap-2">
                <p className="text-sm font-semibold text-[var(--nv-text-primary)]">{doubanConfig?.configured ? 'Cookie 已配置' : 'Cookie 未配置（匿名模式）'}</p>
                <Tag tone={doubanConfig?.configured ? 'success' : 'neutral'}>{doubanConfig?.configured ? '登录刮削' : '匿名刮削'}</Tag>
              </div>
              {doubanConfig?.configured && doubanConfig.masked_cookie && (
                <div className="mt-1 flex min-w-0 items-center gap-2 text-xs font-mono text-[var(--nv-text-tertiary)]">
                  <span className="truncate">{doubanShowCookie ? doubanConfig.masked_cookie : '••••••••••••••••••••'}</span>
                  <button type="button" onClick={() => setDoubanShowCookie((value) => !value)} className="shrink-0 text-[var(--nv-text-tertiary)] hover:text-[var(--nv-text-primary)]" aria-label={doubanShowCookie ? '隐藏' : '显示掩码'}>{doubanShowCookie ? <EyeOff size={13} /> : <Eye size={13} />}</button>
                </div>
              )}
            </div>
          </div>

          {doubanMessage && <FeedbackBanner feedback={doubanMessage} />}

          {doubanEditing ? (
            <div className="space-y-3">
              <div>
                <label className="mb-1.5 block text-sm font-medium text-[var(--nv-text-secondary)]">豆瓣 Cookie 字符串</label>
                <textarea
                  value={doubanCookieInput}
                  onChange={(event) => setDoubanCookieInput(event.target.value)}
                  className="min-h-32 w-full resize-y rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] px-3 py-2.5 font-mono text-xs text-[var(--nv-text-primary)] outline-none transition-colors placeholder:text-[var(--nv-text-tertiary)] hover:border-[var(--nv-border-hover)] focus:border-[var(--nv-action-primary)] focus:shadow-[var(--nv-shadow-focus)]"
                  placeholder='示例：bid=...; dbcl2="..."; ck=...; ...'
                  autoFocus
                />
                <p className="mt-1.5 text-xs text-[var(--nv-text-tertiary)]">Cookie 有效期可能变化；失效后重新导出并保存即可。</p>
              </div>
              <div className="flex flex-wrap gap-2">
                <Button variant="primary" onClick={() => void saveDoubanCookie()} disabled={!doubanCookieInput.trim()} loading={doubanSaving}>{doubanSaving ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}{doubanSaving ? '保存中...' : '保存'}</Button>
                <Button variant="ghost" onClick={() => { setDoubanEditing(false); setDoubanCookieInput('') }}>取消</Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-wrap gap-2">
              <Button variant="primary" onClick={() => setImportOpen(true)}><Key size={14} />快速导入 Cookie</Button>
              <Button onClick={() => setDoubanEditing(true)}><Key size={14} />{doubanConfig?.configured ? '手动修改 Cookie' : '手动配置 Cookie'}</Button>
              {doubanConfig?.configured && <Button onClick={() => void validateDoubanCookie()} loading={doubanValidating}>{doubanValidating ? <Loader2 size={14} className="animate-spin" /> : <Check size={14} />}测试连接</Button>}
              {doubanConfig?.configured && <Button variant="danger" onClick={() => void clearDoubanCookie()}><Trash2 size={14} />清除 Cookie</Button>}
            </div>
          )}

          <div className="flex items-start gap-2 border-t border-[var(--nv-border-subtle)] pt-4 text-xs leading-5 text-[var(--nv-text-tertiary)]">
            <ShieldAlert size={15} className="mt-0.5 shrink-0 text-[var(--nv-status-warning)]" />
            <span>Cookie 属于敏感登录凭证，请勿分享或用于公共服务；出现风控时可清除 Cookie 回退到匿名模式。</span>
          </div>
        </div>
      </AdminPanel>

      <DoubanCookieImportModal
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onConfigChange={setDoubanConfig}
        showMessage={showDoubanMessage}
      />
    </div>
  )
}
