/**
 * UpdateBanner —— 桌面端自动更新提示横幅
 *
 * 应用启动 3 秒后后台自动检查更新，有新版本则触发 `update-available` 事件。
 * 本组件监听事件并在右下角弹出提示，用户点击后立即下载并安装。
 * 仅桌面端生效；浏览器环境不渲染。
 */

import { useEffect, useState } from 'react'
import { Download, Sparkles, X } from 'lucide-react'
import { desktop, UpdateInfo } from './bridge'
import { Button, Surface, Tag } from '@/components/design-system'

export default function UpdateBanner() {
  const [info, setInfo] = useState<UpdateInfo | null>(null)
  const [downloading, setDownloading] = useState(false)
  const [dismissed, setDismissed] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!desktop.isDesktop) return
    let cleanup: (() => void) | undefined

    ;(async () => {
      cleanup = await desktop.onUpdateAvailable((updateInfo) => {
        if (updateInfo.available) setInfo(updateInfo)
      })
    })()

    return () => {
      cleanup?.()
    }
  }, [])

  if (!desktop.isDesktop || !info || dismissed) return null

  const handleInstall = async () => {
    setDownloading(true)
    setError(null)
    try {
      const ok = await desktop.installUpdate()
      if (!ok) {
        setError('更新失败，请稍后重试')
        setDownloading(false)
      }
      // 成功时应用会自动重启。
    } catch (cause: unknown) {
      setError(cause instanceof Error ? cause.message : String(cause))
      setDownloading(false)
    }
  }

  return (
    <Surface className="fixed bottom-6 right-6 z-[var(--nv-z-toast)] w-[min(380px,calc(100vw-32px))] overflow-hidden border-[var(--nv-border-strong)] bg-[var(--nv-bg-elevated)] shadow-[var(--nv-shadow-elevated)] animate-fade-in">
      <div className="p-4">
        <div className="flex items-start gap-3">
          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] border border-[var(--nv-border-hover)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
            <Sparkles className="h-5 w-5" aria-hidden="true" />
          </div>
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <h3 className="font-semibold text-[var(--nv-text-primary)]">发现新版本 v{info.version}</h3>
              <Tag tone="neutral">当前 v{info.current_version}</Tag>
            </div>
            {info.notes && (
              <p className="mt-1 line-clamp-3 whitespace-pre-line text-xs leading-5 text-[var(--nv-text-secondary)]">
                {info.notes}
              </p>
            )}
            {error && <p className="mt-1.5 text-xs text-[var(--nv-status-danger)]" role="alert">⚠️ {error}</p>}
          </div>
          <Button type="button" variant="ghost" size="sm" iconOnly onClick={() => setDismissed(true)} aria-label="关闭更新提示">
            <X className="h-4 w-4" aria-hidden="true" />
          </Button>
        </div>

        <div className="mt-4 flex items-center gap-2">
          <Button type="button" variant="primary" className="flex-1" onClick={handleInstall} disabled={downloading} loading={downloading}>
            {!downloading && <Download className="h-4 w-4" aria-hidden="true" />}
            {downloading ? '下载中...' : '下载并安装'}
          </Button>
          <Button type="button" variant="secondary" onClick={() => setDismissed(true)}>
            稍后
          </Button>
        </div>
      </div>
    </Surface>
  )
}
