
/**
 * UpdateBanner —— 桌面端自动更新提示横幅
 *
 * 应用启动 3 秒后后台自动检查更新，有新版本则触发 `update-available` 事件。
 * 本组件监听事件并在右下角弹出提示，用户点击后立即下载并安装。
 *
 * 仅桌面端生效；浏览器环境不渲染。
 */

import { useEffect, useState } from 'react'
import { Download, Sparkles, X } from 'lucide-react'
import { desktop, UpdateInfo } from './bridge'

export default function UpdateBanner() {
  const [info, setInfo] = useState<UpdateInfo | null>(null)
  const [downloading, setDownloading] = useState(false)
  const [dismissed, setDismissed] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!desktop.isDesktop) return
    let cleanup: (() => void) | undefined

    ;(async () => {
      cleanup = await desktop.onUpdateAvailable((i) => {
        if (i.available) setInfo(i)
      })
    })()

    return () => {
      cleanup?.()
    }
  }, [])

  if (!desktop.isDesktop) return null
  if (!info || dismissed) return null

  const handleInstall = async () => {
    setDownloading(true)
    setError(null)
    try {
      const ok = await desktop.installUpdate()
      if (!ok) {
        setError('更新失败，请稍后重试')
        setDownloading(false)
      }
      // 成功时应用会自动重启
    } catch (e: any) {
      setError(e?.message || String(e))
      setDownloading(false)
    }
  }

  return (
    <div
      className="fixed bottom-6 right-6 z-[9999] w-[380px] rounded-2xl shadow-2xl
                 border border-white/20 backdrop-blur-xl
                 bg-gradient-to-br from-purple-600/90 via-blue-600/90 to-cyan-500/90
                 text-white overflow-hidden animate-fade-in"
    >
      <div className="p-4">
        <div className="flex items-start gap-3">
          <div className="shrink-0 w-10 h-10 rounded-full bg-white/20 flex items-center justify-center">
            <Sparkles className="w-5 h-5" />
          </div>
          <div className="flex-1 min-w-0">
            <div className="flex items-center gap-2">
              <h3 className="font-semibold">发现新版本 v{info.version}</h3>
              <span className="text-xs opacity-70">当前 v{info.current_version}</span>
            </div>
            {info.notes && (
              <p className="mt-1 text-xs opacity-90 line-clamp-3 whitespace-pre-line">
                {info.notes}
              </p>
            )}
            {error && (
              <p className="mt-1.5 text-xs text-red-200">⚠️ {error}</p>
            )}
          </div>
          <button
            onClick={() => setDismissed(true)}
            className="shrink-0 w-6 h-6 rounded-full hover:bg-white/20 flex items-center justify-center"
            aria-label="关闭"
          >
            <X className="w-4 h-4" />
          </button>
        </div>

        <div className="mt-3 flex items-center gap-2">
          <button
            onClick={handleInstall}
            disabled={downloading}
            className="flex-1 px-3 py-2 rounded-lg bg-white text-purple-700 font-medium text-sm
                       hover:bg-white/90 disabled:opacity-60 transition flex items-center justify-center gap-1.5"
          >
            <Download className="w-4 h-4" />
            {downloading ? '下载中...' : '下载并安装'}
          </button>
          <button
            onClick={() => setDismissed(true)}
            className="px-3 py-2 rounded-lg bg-white/15 hover:bg-white/25 text-sm transition"
          >
            稍后
          </button>
        </div>
      </div>
    </div>
  )
}
