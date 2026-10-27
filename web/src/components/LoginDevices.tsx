import { useCallback, useEffect, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { authApi, type LoginDevice } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useDialog } from '@/components/Dialog'
import { useToast } from '@/components/Toast'
import { Button, Section } from '@/components/design-system'

export default function LoginDevices() {
  const [devices, setDevices] = useState<LoginDevice[]>([])
  const [currentID, setCurrentID] = useState('')
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)
  const [busy, setBusy] = useState(false)
  const token = useAuthStore(state => state.token)
  const request = useRef(0)
  const pending = useRef(false)
  const dialog = useDialog()
  const toast = useToast()
  const load = useCallback(async () => {
    const generation = ++request.current
    setLoading(true); setFailed(false)
    try { const { data } = await authApi.devices(); if (generation === request.current) { setDevices(data.data || []); setCurrentID(data.current_id || '') } }
    catch { if (generation === request.current) setFailed(true) }
    finally { if (generation === request.current) setLoading(false) }
  }, [])
  useEffect(() => { void load(); return () => { request.current++ } }, [load, token])
  const revoke = async (device?: LoginDevice) => {
    if (pending.current) return
    pending.current = true
    try {
      if (!await dialog.confirm({ title: device ? '退出这台设备？' : '退出全部设备？', message: device ? `“${device.name}”将需要重新登录，不会删除影片或观看记录。` : '包括当前设备和旧版登录凭据，全部需要重新登录。', confirmText: '确认退出', variant: 'warning' })) return
      setBusy(true)
      if (device) await authApi.revokeDevice(device.id)
      else await authApi.revokeAllDevices()
      if (!device || device.id === currentID) useAuthStore.getState().logout()
      else { toast.success('设备已退出'); await load() }
    } catch { toast.error('设备未能退出，请重试') }
    finally { pending.current = false; setBusy(false) }
  }
  return <Section title="登录设备" description="可以单独退出不用的设备，其他设备继续使用。">
    <div className="space-y-4">
      <Link to="/pair" className="nv-button" data-variant="primary">授权新设备</Link>
      {loading ? <p>正在读取设备…</p> : failed ? <div role="alert">暂时无法读取登录设备。<Button onClick={() => void load()}>重试</Button></div> : <>
        {!currentID && <p className="text-sm text-[var(--nv-text-tertiary)]">当前是旧版登录，新登录会记录为设备；要撤销旧凭据，请使用“退出全部设备”。</p>}
        <ul className="divide-y divide-[var(--nv-border-subtle)]">
          {devices.map(device => <li key={device.id} className="flex flex-wrap items-center justify-between gap-3 py-3">
            <div><strong>{device.name}</strong>{device.id === currentID && <span className="ml-2 text-sm">此设备</span>}<p className="text-sm text-[var(--nv-text-tertiary)]">最近使用 {new Date(device.last_seen_at).toLocaleString()}</p></div>
            <Button disabled={busy} onClick={() => void revoke(device)} aria-label={`退出 ${device.name}`}>退出设备</Button>
          </li>)}
        </ul>
        <Button disabled={busy} onClick={() => void revoke()}>退出全部设备</Button>
      </>}
    </div>
  </Section>
}
