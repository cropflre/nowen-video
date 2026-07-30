import { useEffect } from 'react'
import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useLocation } from 'react-router-dom'
import { useServerProfileStore } from '@/stores/serverProfile'

const HIDDEN_ATTRIBUTE = 'data-nowen-capability-hidden'

function hideUnsupportedPreprocessSetting() {
  const headings = Array.from(document.querySelectorAll<HTMLHeadingElement>('h4'))
  const heading = headings.find((item) => item.textContent?.trim() === '扫描后自动预处理')
  const row = heading?.closest<HTMLElement>('.flex.items-start.justify-between.gap-4')
  if (!row || row.getAttribute(HIDDEN_ATTRIBUTE) === 'preprocess') return

  row.setAttribute(HIDDEN_ATTRIBUTE, 'preprocess')
  row.style.display = 'none'
}

function restoreCapabilityHiddenControls() {
  document.querySelectorAll<HTMLElement>(`[${HIDDEN_ATTRIBUTE}]`).forEach((element) => {
    element.style.removeProperty('display')
    element.removeAttribute(HIDDEN_ATTRIBUTE)
  })
}

/**
 * Transitional capability guard for the legacy Admin components.
 *
 * DashboardTab and AITab are still large shared components used by Lite and
 * Full. Until those panels are split into capability-scoped sections, this
 * component keeps unavailable controls out of Lite and surfaces process-level
 * restart requirements in one predictable place.
 */
export default function CapabilityAdminGuard() {
  const location = useLocation()
  const manifest = useServerProfileStore((state) => state.manifest)
  const loading = useServerProfileStore((state) => state.loading)
  const load = useServerProfileStore((state) => state.load)

  const isAdmin = location.pathname === '/admin'
  const isLite = manifest?.profile === 'lite'
  const ai = manifest?.capabilities.ai

  useEffect(() => {
    if (!isAdmin || !isLite) {
      restoreCapabilityHiddenControls()
      return
    }

    const apply = () => hideUnsupportedPreprocessSetting()
    apply()

    const root = document.querySelector('.nv-app-body') || document.body
    const observer = new MutationObserver(apply)
    observer.observe(root, { childList: true, subtree: true })

    return () => {
      observer.disconnect()
      restoreCapabilityHiddenControls()
    }
  }, [isAdmin, isLite, location.hash])

  if (!isAdmin || location.hash !== '#ai' || !ai?.pending_restart) return null

  const enabling = ai.configured && !ai.enabled
  const message = enabling
    ? 'AI 配置已经开启，但运行路由、数据表和后台组件需要重启服务后才会装配。当前可以继续编辑配置和测试连接。'
    : 'AI 配置已经关闭，客户端已停止把 AI 视为可用；重启服务后将完成运行组件回收。'

  return (
    <div className="fixed left-1/2 top-14 z-[120] w-[min(92vw,760px)] -translate-x-1/2">
      <div
        className="flex items-start gap-3 rounded-xl border px-4 py-3 shadow-2xl backdrop-blur-xl"
        style={{
          background: 'color-mix(in srgb, var(--bg-surface) 94%, transparent)',
          borderColor: 'rgba(245, 158, 11, 0.35)',
        }}
        role="status"
        aria-live="polite"
      >
        <AlertTriangle size={18} className="mt-0.5 shrink-0 text-amber-400" />
        <div className="min-w-0 flex-1">
          <p className="text-sm font-semibold text-theme-primary">
            AI 配置等待服务重启
          </p>
          <p className="mt-1 text-xs leading-relaxed text-theme-muted">{message}</p>
        </div>
        <button
          type="button"
          onClick={() => void load(true)}
          disabled={loading}
          className="btn-secondary shrink-0 gap-1.5 px-3 py-1.5 text-xs"
        >
          <RefreshCw size={13} className={loading ? 'animate-spin' : ''} />
          重新检测
        </button>
      </div>
    </div>
  )
}
