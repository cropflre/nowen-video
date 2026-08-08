import { useEffect, useState } from 'react'
import { AlertTriangle, Bookmark, Check, ClipboardPaste, Copy, ExternalLink, Key, Loader2, RefreshCw, X } from 'lucide-react'
import { adminApi } from '@/api'
import type { DoubanConfigStatus, DoubanImportTokenInfo, DoubanImportTokenStatus } from '@/types'
import { useDialog } from '@/components/Dialog'
import { Button, Modal, ModalBody, ModalFooter, ModalHeader, Tag } from '@/components/design-system'
import { AdminStatus } from './AdminPrimitives'

type MessageHandler = (type: 'success' | 'error' | 'info', text: string) => void

interface DoubanCookieImportModalProps {
  open: boolean
  onClose: () => void
  onConfigChange: (config: DoubanConfigStatus) => void
  showMessage: MessageHandler
}

export default function DoubanCookieImportModal({ open, onClose, onConfigChange, showMessage }: DoubanCookieImportModalProps) {
  const dialog = useDialog()
  const [info, setInfo] = useState<DoubanImportTokenInfo | null>(null)
  const [status, setStatus] = useState<DoubanImportTokenStatus | null>(null)
  const [loading, setLoading] = useState(false)
  const [pasting, setPasting] = useState(false)
  const [copied, setCopied] = useState<'script' | 'bookmarklet' | null>(null)

  const generateImportToken = async () => {
    setInfo(null)
    setStatus(null)
    setCopied(null)
    setLoading(true)
    try {
      const response = await adminApi.createDoubanImportToken()
      setInfo(response.data.data)
    } catch (error: any) {
      showMessage('error', error?.response?.data?.error || '生成导入链接失败，请稍后重试')
      onClose()
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    if (!open) {
      setInfo(null)
      setStatus(null)
      setCopied(null)
      return
    }
    void generateImportToken()
  }, [open])

  useEffect(() => {
    if (!open || !info) return
    if (status?.status === 'success' || status?.status === 'expired') return

    const timer = window.setInterval(async () => {
      try {
        const response = await adminApi.getDoubanImportTokenStatus(info.token)
        const next = response.data.data
        setStatus(next)
        if (next.status === 'success') {
          const configResponse = await adminApi.getDoubanConfig()
          onConfigChange(configResponse.data.data)
          showMessage('success', next.message || '豆瓣 Cookie 已导入')
        }
      } catch {
        // 保持旧行为：轮询失败静默处理，下一轮继续重试。
      }
    }, 2000)

    return () => window.clearInterval(timer)
  }, [info, onConfigChange, open, showMessage, status?.status])

  const copyText = async (text: string, kind: 'script' | 'bookmarklet') => {
    try {
      await navigator.clipboard.writeText(text)
    } catch {
      const textarea = document.createElement('textarea')
      textarea.value = text
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
    }
    setCopied(kind)
    window.setTimeout(() => setCopied(null), 2000)
  }

  const pasteAndImport = async () => {
    setPasting(true)
    try {
      let cookie = ''
      if (navigator.clipboard?.readText) {
        try {
          cookie = (await navigator.clipboard.readText()) || ''
        } catch {
          cookie = ''
        }
      }

      if (!cookie) {
        const input = await dialog.prompt({
          title: '手动粘贴豆瓣 Cookie',
          message: '无法自动读取剪贴板，请在下方手动粘贴豆瓣 Cookie：',
          placeholder: '粘贴完整 Cookie 字符串',
          inputType: 'textarea',
        })
        cookie = (input || '').trim()
      }

      cookie = cookie.trim()
      if (!cookie) {
        showMessage('error', '未读取到 Cookie，请先在豆瓣页面执行脚本')
        return
      }
      if (cookie.length < 20) {
        showMessage('error', 'Cookie 内容过短，请确认已在豆瓣页面成功执行脚本')
        return
      }
      if (!cookie.includes('dbcl2=')) {
        showMessage('error', '剪贴板内容中缺少登录凭证 dbcl2（HttpOnly 无法被 JS 读取），请改用 Cookie 浏览器插件导出完整 Cookie')
        return
      }

      const response = await adminApi.updateDoubanConfig(cookie)
      onConfigChange(response.data.data)
      showMessage('success', '豆瓣 Cookie 已导入，正在校验登录态...')
      try {
        const validation = await adminApi.validateDoubanConfig()
        const { valid, message } = validation.data.data
        showMessage(valid ? 'success' : 'error', message)
      } catch {
        // 校验失败不影响已完成的导入结果。
      }
      onClose()
    } catch (error: any) {
      showMessage('error', error?.response?.data?.error || '导入失败，请确认 Cookie 格式正确')
    } finally {
      setPasting(false)
    }
  }

  const renderState = () => {
    if (status?.status === 'success') {
      return (
        <div className="rounded-[var(--nv-radius-control)] border border-[color-mix(in_srgb,var(--nv-status-success)_28%,transparent)] bg-[color-mix(in_srgb,var(--nv-status-success)_8%,transparent)] p-4">
          <AdminStatus tone="success"><Check size={13} />导入成功</AdminStatus>
          {status.message && <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{status.message}</p>}
        </div>
      )
    }
    if (status?.status === 'expired' || status?.status === 'failed') {
      return (
        <div className="rounded-[var(--nv-radius-control)] border border-[color-mix(in_srgb,var(--nv-status-danger)_28%,transparent)] bg-[color-mix(in_srgb,var(--nv-status-danger)_8%,transparent)] p-4">
          <AdminStatus tone="danger"><X size={13} />{status.status === 'expired' ? '链接已过期' : '导入失败'}</AdminStatus>
          {status.message && <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">{status.message}</p>}
          <Button size="sm" className="mt-3" onClick={() => void generateImportToken()}>
            <RefreshCw size={14} />重新生成
          </Button>
        </div>
      )
    }
    return (
      <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-surface-soft)] p-4">
        <AdminStatus tone="active"><ClipboardPaste size={13} />等待导入</AdminStatus>
        <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">在豆瓣页面完成操作后，可从剪贴板读取 Cookie 并通过同域接口导入。</p>
        <Button size="sm" className="mt-3" onClick={() => void pasteAndImport()} loading={pasting}>
          {pasting ? <Loader2 size={14} className="animate-spin" /> : <ClipboardPaste size={14} />}
          {pasting ? '正在导入...' : '从剪贴板粘贴并导入'}
        </Button>
      </div>
    )
  }

  return (
    <Modal open={open} onClose={onClose} size="md" ariaLabel="豆瓣 Cookie 导入">
      <ModalHeader
        title="豆瓣 Cookie 快速导入"
        description="通过剪贴板中转导入；Cookie 仅保存到当前服务器。"
        icon={<Key size={18} />}
        onClose={onClose}
      />
      <ModalBody className="space-y-5">
        {loading && (
          <div className="flex min-h-40 items-center justify-center gap-3 text-sm text-[var(--nv-text-tertiary)]">
            <Loader2 size={20} className="animate-spin text-[var(--nv-action-primary)]" />
            正在生成一次性导入链接...
          </div>
        )}

        {info && !loading && (
          <>
            {renderState()}

            <div className="rounded-[var(--nv-radius-control)] border border-[color-mix(in_srgb,var(--nv-status-warning)_28%,transparent)] bg-[color-mix(in_srgb,var(--nv-status-warning)_7%,transparent)] p-4">
              <div className="flex items-center gap-2 text-sm font-semibold text-[var(--nv-status-warning)]">
                <AlertTriangle size={16} />推荐使用 Cookie 浏览器插件
              </div>
              <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                豆瓣的 <code className="font-mono text-[var(--nv-text-primary)]">dbcl2</code> 为 HttpOnly，现代浏览器中的 Bookmarklet / Console JS 通常无法读取完整登录态。Cookie-Editor 等插件导出的 Header String 更可靠。
              </p>
            </div>

            <div className="space-y-3">
              <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] p-4">
                <div className="flex flex-wrap items-center gap-2">
                  <ExternalLink size={15} className="text-[var(--nv-action-primary)]" />
                  <h4 className="text-sm font-semibold text-[var(--nv-text-primary)]">方式 3：Cookie 浏览器插件</h4>
                  <Tag tone="success">推荐</Tag>
                </div>
                <ol className="mt-3 list-decimal space-y-1 pl-5 text-xs leading-5 text-[var(--nv-text-tertiary)]">
                  <li>安装 Cookie-Editor 或同类 Cookie 导出插件。</li>
                  <li><a href={info.douban_url} target="_blank" rel="noopener noreferrer" className="text-[var(--nv-action-primary)] hover:underline">打开豆瓣</a> 并确认已登录。</li>
                  <li>导出 Header String，确保包含 dbcl2。</li>
                  <li>回到此处点击“从剪贴板粘贴并导入”。</li>
                </ol>
              </div>

              <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] p-4 opacity-90">
                <div className="flex flex-wrap items-center gap-2">
                  <Bookmark size={15} className="text-[var(--nv-text-tertiary)]" />
                  <h4 className="text-sm font-semibold text-[var(--nv-text-primary)]">方式 1：Bookmarklet</h4>
                  <Tag tone="warning">兼容方案</Tag>
                </div>
                <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">仅在目标 Cookie 未启用 HttpOnly 时有效。可拖动书签到书签栏，或复制地址。</p>
                <div className="mt-3 flex flex-wrap gap-2">
                  <a
                    href={info.bookmarklet}
                    onClick={(event) => event.preventDefault()}
                    draggable
                    className="nv-button nv-button--secondary nv-button--sm cursor-grab"
                    title="拖到浏览器书签栏"
                  >
                    <Bookmark size={13} />豆瓣一键登录
                  </a>
                  <Button size="sm" variant="ghost" onClick={() => void copyText(info.bookmarklet, 'bookmarklet')}>
                    {copied === 'bookmarklet' ? <Check size={13} /> : <Copy size={13} />}
                    {copied === 'bookmarklet' ? '已复制' : '复制书签地址'}
                  </Button>
                </div>
              </div>

              <div className="rounded-[var(--nv-radius-control)] border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] p-4 opacity-90">
                <div className="flex flex-wrap items-center gap-2">
                  <Key size={15} className="text-[var(--nv-text-tertiary)]" />
                  <h4 className="text-sm font-semibold text-[var(--nv-text-primary)]">方式 2：浏览器控制台</h4>
                  <Tag tone="warning">兼容方案</Tag>
                </div>
                <p className="mt-2 text-xs leading-5 text-[var(--nv-text-tertiary)]">在豆瓣页面 Console 中执行下面脚本；如果缺少 dbcl2，请改用插件导出。</p>
                <div className="relative mt-3">
                  <pre className="max-h-36 overflow-auto whitespace-pre-wrap break-all rounded-[var(--nv-radius-control)] border border-[var(--nv-border-subtle)] bg-[var(--nv-bg-elevated)] p-3 pr-24 text-[11px] text-[var(--nv-text-secondary)]">{info.script.trim()}</pre>
                  <Button size="sm" variant="ghost" className="absolute right-2 top-2" onClick={() => void copyText(info.script, 'script')}>
                    {copied === 'script' ? <Check size={12} /> : <Copy size={12} />}
                    {copied === 'script' ? '已复制' : '复制'}
                  </Button>
                </div>
              </div>
            </div>
          </>
        )}
      </ModalBody>
      <ModalFooter>
        <Button variant="secondary" onClick={onClose}>关闭</Button>
      </ModalFooter>
    </Modal>
  )
}
