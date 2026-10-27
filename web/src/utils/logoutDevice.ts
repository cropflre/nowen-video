import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'

let pending: { token: string | null; promise: Promise<boolean> } | undefined

export function logoutDevice(): Promise<boolean> {
  const originalToken = useAuthStore.getState().token
  if (pending?.token === originalToken) return pending.promise
  const promise = (async () => {
    let confirmed = false
    try { confirmed = Boolean((await authApi.logout()).data.server_revoked) } catch { /* Local sign-out remains available offline. */ }
    if (useAuthStore.getState().token === originalToken) useAuthStore.getState().logout()
    return confirmed
  })()
  pending = { token: originalToken, promise }
  void promise.finally(() => { if (pending?.promise === promise) pending = undefined })
  return promise
}
