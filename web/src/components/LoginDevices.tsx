import { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { ChevronDown, ChevronUp } from 'lucide-react'
import { authApi, type LoginDevice } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useDialog } from '@/components/Dialog'
import { useToast } from '@/components/Toast'
import { Button, Section, Tag } from '@/components/design-system'

const DEFAULT_VISIBLE_DEVICES = 3

export default function LoginDevices() {
  const [devices, setDevices] = useState<LoginDevice[]>([])
  const [currentID, setCurrentID] = useState('')
  const [loading, setLoading] = useState(true)
  const [failed, setFailed] = useState(false)
  const [busy, setBusy] = useState(false)
  const [expanded, setExpanded] = useState(false)
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

  // 当前设备置顶，其余按原有顺序排列，默认只展示前 3 台避免列表过长
  const sortedDevices = useMemo(() => {
    if (!currentID) return devices
    const current = devices.filter(d => d.id === currentID)
    const others = devices.filter(d => d.id !== currentID)
    return [...current, ...others]
  }, [devices, currentID])

  const hiddenCount = Math.max(0, sortedDevices.length - DEFAULT_VISIBLE_DEVICES)
  const visibleDevices = expanded ? sortedDevices : sortedDevices.slice(0, DEFAULT_VISIBLE_DEVICES)

  return (
    <Section title="登录设备" description="可以单独退出不用的设备，其他设备继续使用。">
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <Link to="/pair" className="nv-button nv-button--sm" data-variant="primary">授权新设备</Link>
          {!loading && !failed && sortedDevices.length > 0 && (
            <Button size="sm" variant="secondary" disabled={busy} onClick={() => void revoke()}>
              退出全部设备{sortedDevices.length > 1 ? ` (${sortedDevices.length})` : ''}
            </Button>
          )}
        </div>

        {loading ? (
          <p className="text-sm text-[var(--nv-text-tertiary)]">正在读取设备…</p>
        ) : failed ? (
          <div role="alert" className="flex items-center gap-2 text-sm">
            暂时无法读取登录设备。
            <Button size="sm" onClick={() => void load()}>重试</Button>
          </div>
        ) : (
          <>
            {!currentID && (
              <p className="text-sm text-[var(--nv-text-tertiary)]">
                当前是旧版登录，新登录会记录为设备；要撤销旧凭据，请使用“退出全部设备”。
              </p>
            )}
            <ul className="divide-y divide-[var(--nv-border-subtle)]">
              {visibleDevices.map(device => {
                const isCurrent = device.id === currentID
                return (
                  <li key={device.id} className="flex flex-wrap items-center justify-between gap-3 py-2.5">
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <strong className="text-sm font-medium text-[var(--nv-text-primary)]">{device.name}</strong>
                        {isCurrent && <Tag tone="brand">当前设备</Tag>}
                      </div>
                      <p className="text-xs text-[var(--nv-text-tertiary)]">
                        最近使用 {new Date(device.last_seen_at).toLocaleString()}
                      </p>
                    </div>
                    <Button size="sm" variant="ghost" disabled={busy} onClick={() => void revoke(device)} aria-label={`退出 ${device.name}`}>
                      退出设备
                    </Button>
                  </li>
                )
              })}
            </ul>
            {hiddenCount > 0 && (
              <Button type="button" variant="ghost" size="sm" onClick={() => setExpanded(v => !v)}>
                {expanded ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
                {expanded ? '收起其余设备' : `查看其余 ${hiddenCount} 台设备`}
              </Button>
            )}
          </>
        )}
      </div>
    </Section>
  )
}
