import { useEffect } from 'react'
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
 * Transitional capability guard for legacy Admin controls that have not yet
 * been migrated into capability-aware sections. AITab now owns its configured,
 * runtime and pending-restart states directly from the server profile store.
 */
export default function CapabilityAdminGuard() {
  const location = useLocation()
  const manifest = useServerProfileStore((state) => state.manifest)

  const isAdmin = location.pathname === '/admin'
  const isLite = manifest?.profile === 'lite'

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

  return null
}
