import { useCallback, useEffect, useState } from 'react'
import { Check, ChevronDown, ChevronUp, ExternalLink, Eye, EyeOff, Film, Key, Loader2, ShieldAlert, Trash2, Wifi, X } from 'lucide-react'
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
  const [doubanSaving, setDoubanSaving] = useState(false)
  const [doubanValidating, setDoubanValidating] = useState(false)
  const [doubanMessage, setDoubanMessage] = useState<Feedback | null>(null)
  const [importOpen, setImportOpen] = useState(false)
  const [showAdvanced, setShowAdvanced] = useState(false)

  useEffect(() => {
    let cancelled = false

    const load = async () => {
      const [tmdbResult, doubanResult] = await Promise.allSettled([
        adminApi.getTMDbConfig(),
        adminApi.getDoubanConfig(),
      ])
      if (cancelled) return

      if (tmdbResult.status === 'fulfilled') {
        setTmdbConfig(tmdbResult.value.data.data)
      } else {
        setTmdbMessage({ type: 'error', text: 'TMDb 配置加载失败，可稍后重新进入本页重试' })
      }

      if (doubanResult.status === 'fulfilled') {
        setDoubanConfig(doubanResult.value.data.data)
      } else {
        setDoubanMessage({ type: 'error', text: '豆瓣配置加载失败，可稍后重新进入本页重试' })
      }
    }

    void load()
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

  // 傻瓜式一步到位：保存并立即测试连通性
  const saveAndTestTmdbKey = async () => {
    const key = tmdbKeyInput.trim()
    if (!key) return
    setTmdbSaving(true)
    try {
      const response = await adminApi.updateTMDbConfig(key)
      setTmdbConfig(response.data.data)
      setTmdbKeyInput('')
      setTmdbEditing(false)
      setTmdbShowKey(false)
      setTmdbTesting(true)
      try {
        const testRes = await adminApi.validateTMDbConfig()
        const { valid, message } = testRes.data.data
        showTmdbMessage(valid ? 'success' : 'error', valid ? '密钥已保存且连接正常' : (message || t('admin.tmdbTestFailed')))
      } catch {
        showTmdbMessage('success', t('admin.tmdbSaveSuccess'))
      } finally {
        setTmdbTesting(false)
      }
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
    if (!tmdbConfig?.configured) {
      showTmdbMessage('error', t('admin.tmdbTestNoKey'))
      return
    }
    setTmdbTesting(true)
    showTmdbMessage('info', t('admin.tmdbTesting'))
    try {
      const response = await adminApi.validateTMDbConfig()
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

  const showTmdbForm = tmdbEditing || (tmdbConfig !== null && !tmdbConfig.configured)

  return (
    <div className="space-y-4">
      <AdminPanel
        title="影片刮削（TMDb）"
        description="自动获取影片海报、简介、评分和演职员信息。"
        icon={<Film size={18} />}
        actions={
          <Tag tone={tmdbConfig?.configured ? 'success' : 'warning'}>
            {tmdbConfig?.configured ? '已就绪' : '待配置'}
          </Tag>
        }
      >
        <div className="space-y-4">
          {tmdbMessage && <FeedbackBanner feedback={tmdbMessage} />}

          {showTmdbForm ? (
            <div className="space-y-3">
              <div className="flex flex-wrap items-center justify-between gap-2">
                <label className="text-sm font-medium text-[var(--nv-text-secondary)]">TMDb API Key</label>
                <a href="https://www.themoviedb.org/settings/api" target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1 text-xs font-medium text-[var(--nv-action-primary)] hover:underline">
                  <ExternalLink size={12} />免费申请 Key
                </a>
              </div>
              <div className="flex flex-col gap-2 sm:flex-row">
                <Input
                  type="text"
                  value={tmdbKeyInput}
                  onChange={(event) => setTmdbKeyInput(event.target.value)}
                  placeholder="粘贴 32 位 TMDb API Key（v3 auth）"
                  className="flex-1 font-mono"
                  onKeyDown={(event) => event.key === 'Enter' && void saveAndTestTmdbKey()}
                />
                <div className="flex shrink-0 gap-2">
                  <Button variant="primary" onClick={() => void saveAndTestTmdbKey()} disabled={!tmdbKeyInput.trim()} loading={tmdbSaving || tmdbTesting}>
                    {!(tmdbSaving || tmdbTesting) && <Check size={14} />}
                    {tmdbSaving ? '保存中...' : tmdbTesting ? '测试中...' : '保存并测试'}
                  </Button>
                  {tmdbConfig?.configured && (
                    <Button variant="ghost" onClick={() => { setTmdbEditing(false); setTmdbKeyInput('') }}>{t('common.cancel')}</Button>
                  )}
                </div>
              </div>
            </div>
          ) : (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] px-4 py-3">
              <div className="flex items-center gap-2.5 text-sm">
                <Key size={15} className="text-[var(--nv-action-primary)]" />
                <span className="font-medium text-[var(--nv-text-primary)]">API 密钥</span>
                {tmdbConfig?.masked_key && (
                  <span className="flex items-center gap-1.5 font-mono text-xs text-[var(--nv-text-tertiary)]">
                    <span>{tmdbShowKey ? tmdbConfig.masked_key : '••••••••••••'}</span>
                    <button type="button" onClick={() => setTmdbShowKey((v) => !v)} className="hover:text-[var(--nv-text-primary)]" aria-label={tmdbShowKey ? '隐藏' : '显示'}>
                      {tmdbShowKey ? <EyeOff size={13} /> : <Eye size={13} />}
                    </button>
                  </span>
                )}
              </div>
              <div className="flex flex-wrap gap-2">
                <Button size="sm" variant="secondary" onClick={() => void testTmdbKey()} loading={tmdbTesting}>
                  {!tmdbTesting && <Wifi size={13} />}
                  {tmdbTesting ? '测试中...' : '测试连接'}
                </Button>
                <Button size="sm" variant="ghost" onClick={() => setTmdbEditing(true)}>修改密钥</Button>
                <Button size="sm" variant="ghost" onClick={() => void clearTmdbKey()} className="text-[var(--nv-status-danger)]" aria-label="清除密钥">
                  <Trash2 size={13} />
                </Button>
              </div>
            </div>
          )}

          <div>
            <Button type="button" variant="ghost" size="sm" onClick={() => setShowAdvanced((v) => !v)}>
              {showAdvanced ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
              高级设置（网络代理与豆瓣 Cookie）
            </Button>
          </div>

          {showAdvanced && (
            <div className="space-y-5 border-t border-[var(--nv-border-subtle)] pt-4">
              <TMDbProxySettings config={tmdbConfig} onConfigChange={setTmdbConfig} />

              <div className="space-y-3 border-t border-[var(--nv-border-subtle)] pt-4">
                <div className="flex flex-wrap items-center justify-between gap-2">
                  <div>
                    <p className="text-sm font-medium text-[var(--nv-text-primary)]">豆瓣 Cookie（可选）</p>
                    <p className="text-xs text-[var(--nv-text-tertiary)]">未配置时使用匿名模式；仅在需要提升豆瓣刮削成功率时填写。</p>
                  </div>
                  <Tag tone={doubanConfig?.configured ? 'success' : 'neutral'}>{doubanConfig?.configured ? '已配置' : '匿名模式'}</Tag>
                </div>

                {doubanMessage && <FeedbackBanner feedback={doubanMessage} />}

                {doubanEditing ? (
                  <div className="space-y-3">
                    <textarea
                      value={doubanCookieInput}
                      onChange={(event) => setDoubanCookieInput(event.target.value)}
                      className="min-h-24 w-full resize-y rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-control)] px-3 py-2 font-mono text-xs text-[var(--nv-text-primary)] outline-none focus:border-[var(--nv-action-primary)]"
                      placeholder='示例：bid=...; dbcl2="..."; ck=...;'
                    />
                    <div className="flex flex-wrap gap-2">
                      <Button size="sm" variant="primary" onClick={() => void saveDoubanCookie()} disabled={!doubanCookieInput.trim()} loading={doubanSaving}>保存</Button>
                      <Button size="sm" variant="ghost" onClick={() => { setDoubanEditing(false); setDoubanCookieInput('') }}>取消</Button>
                    </div>
                  </div>
                ) : (
                  <div className="flex flex-wrap items-center gap-2">
                    <Button size="sm" variant="secondary" onClick={() => setImportOpen(true)}><Key size={13} />快速导入 Cookie</Button>
                    <Button size="sm" variant="ghost" onClick={() => setDoubanEditing(true)}>{doubanConfig?.configured ? '手动修改' : '手动填写'}</Button>
                    {doubanConfig?.configured && <Button size="sm" variant="ghost" onClick={() => void validateDoubanCookie()} loading={doubanValidating}>测试连接</Button>}
                    {doubanConfig?.configured && <Button size="sm" variant="ghost" onClick={() => void clearDoubanCookie()} className="text-[var(--nv-status-danger)]">清除</Button>}
                  </div>
                )}

                <div className="flex items-start gap-2 text-xs text-[var(--nv-text-tertiary)]">
                  <ShieldAlert size={14} className="mt-0.5 shrink-0 text-[var(--nv-status-warning)]" />
                  <span>Cookie 属于敏感登录凭证，请勿分享；出现风控时可清除回退匿名模式。</span>
                </div>
              </div>
            </div>
          )}
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
