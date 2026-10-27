import { useEffect } from 'react'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

export default function DeviceSessionBridge() {
  const token = useAuthStore(state => state.token)
  useEffect(() => {
    if (!token) return
    let pending = false
    const check = () => {
      if (pending || document.hidden) return
      pending = true
      void authApi.deviceStatus().catch(() => {}).finally(() => { pending = false })
    }
    check()
    const timer = window.setInterval(check, 30000)
    window.addEventListener('focus', check)
    return () => { window.clearInterval(timer); window.removeEventListener('focus', check) }
  }, [token])
  return null
}
