import { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import qrcode from 'qrcode-generator'
import { authApi, type PairingRequest } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { useDesktop } from '@/desktop'
import { Button, Input } from '@/components/design-system'

export default function DevicePairingRequest({ onBack }: { onBack: () => void }) {
  const [name, setName] = useState('家里的设备')
  const [request, setRequest] = useState<PairingRequest>()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState('')
  const [expired, setExpired] = useState(false)
  const [seconds, setSeconds] = useState(0)
  const [retry, setRetry] = useState(0)
  const deadline = useRef(0)
  const creating = useRef(false)
  const navigate = useNavigate()
  const runtime = useDesktop()
  const approvalURL = useMemo(() => {
    if (!request) return ''
    const origin = runtime.isDesktop ? runtime.serverBaseUrl : window.location.origin
    if (!origin || !/^https?:\/\//.test(origin)) return ''
    const url = new URL('/pair', origin)
    url.hash = new URLSearchParams({ code: request.code }).toString()
    return url.toString()
  }, [request, runtime.isDesktop, runtime.serverBaseUrl])
  const qr = useMemo(() => {
    if (!approvalURL) return ''
    const code = qrcode(0, 'M'); code.addData(approvalURL); code.make()
    return 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(code.createSvgTag({ scalable: true }))
  }, [approvalURL])
  const generate = async () => {
    if (creating.current) return
    creating.current = true; setBusy(true); setError('')
    try {
      if (request) await authApi.cancelPairing(request.id, request.secret)
      setRequest(undefined)
      const started = performance.now()
      const { data } = await authApi.createPairing(name.trim() || '家里的设备')
      if (!data.data.id || !data.data.secret || !data.data.code) throw new Error('Invalid pairing response')
      deadline.current = started + Math.min(300000, data.data.expires_in_ms)
      setExpired(false); setRequest(data.data)
    } catch { setError('暂时无法生成配对码，请重试') }
    finally { creating.current = false; setBusy(false) }
  }
  useEffect(() => {
    if (!request) return
    let disposed = false
    let timer = 0
    let failures = 0
    const tick = () => {
      const remaining = Math.max(0, Math.ceil((deadline.current - performance.now()) / 1000))
      setSeconds(remaining)
      if (!remaining) { setExpired(true); window.clearTimeout(timer) }
    }
    const poll = async () => {
      if (disposed || performance.now() >= deadline.current) return
      try {
        const { data } = await authApi.pollPairing(request.id, request.secret)
        if (disposed) return
        failures = 0; setError('')
        if (data.data.status === 'authorized') {
          const authorization = data.data.authorization
          if (!authorization?.token || !authorization.user) throw new Error('Missing authorization')
          useAuthStore.getState().setAuth(authorization.token, authorization.user)
          navigate(authorization.user.must_change_pwd ? '/force-change-password' : '/', { replace: true })
          return
        }
        timer = window.setTimeout(() => void poll(), 2000)
      } catch (cause) {
        if (disposed) return
        const status = (cause as { response?: { status?: number } }).response?.status
        if ([404, 410].includes(status || 0)) { setExpired(true); setError('配对码已失效，请重新生成'); return }
        failures += 1
        if (failures >= 3) { setError('连接暂时不可用，请重试'); return }
        timer = window.setTimeout(() => void poll(), 2000)
      }
    }
    tick(); void poll()
    const clock = window.setInterval(tick, 1000)
    return () => { disposed = true; window.clearTimeout(timer); window.clearInterval(clock) }
  }, [request, retry, navigate])
  useEffect(() => () => {
    if (request) void authApi.cancelPairing(request.id, request.secret).catch(() => {})
  }, [request])
  return <section className="space-y-4" aria-label="设备配对">
    <p className="text-sm">用已经登录的手机扫码，或在“我的 → 登录设备”输入下方配对码。</p>
    {!request && <label className="block text-sm">设备名称<Input aria-label="设备名称" maxLength={80} value={name} onChange={event => setName(event.target.value)} /></label>}
    {request && !expired && <div className="space-y-3 text-center">
      {qr && <img src={qr} alt="用已登录手机扫描的设备配对二维码" width={208} height={208} className="mx-auto rounded-lg bg-white p-2" />}
      <p className="font-mono text-3xl font-semibold tracking-widest" aria-label="设备配对码">{request.code}</p>
      <p className="text-sm">{request.device_name} · {Math.floor(seconds / 60)}:{String(seconds % 60).padStart(2, '0')} 后失效</p>
      <p className="text-xs text-[var(--nv-text-tertiary)]">手机需要能访问同一个家庭地址。新设备只会登录你确认的那个账号。</p>
    </div>}
    {expired && <p role="status">配对码已失效，请重新生成</p>}
    {error && <p role="alert">{error}</p>}
    <div className="flex flex-wrap gap-3">
      {(!request || expired) && <Button disabled={busy} onClick={() => void generate()} variant="primary">{busy ? '正在生成…' : request ? '重新生成配对码' : '生成配对码'}</Button>}
      {request && error && !expired && <Button onClick={() => setRetry(value => value + 1)}>重试连接</Button>}
      <Button disabled={busy} onClick={onBack}>返回账号登录</Button>
    </div>
  </section>
}
