import { useEffect, useRef, useState } from 'react'
import { Link, Navigate } from 'react-router-dom'
import { authApi } from '@/api/auth'
import { useAuthStore } from '@/stores/auth'
import { Button, Input, Section } from '@/components/design-system'

export default function DeviceApprovalPage() {
  const user = useAuthStore(state => state.user)
  const [initialCode] = useState(() => new URLSearchParams(window.location.hash.slice(1)).get('code') || '')
  const [code, setCode] = useState(initialCode)
  const [review, setReview] = useState<{ device_name: string; expires_at: string }>()
  const [reviewedCode, setReviewedCode] = useState('')
  const [busy, setBusy] = useState(false)
  const [done, setDone] = useState(false)
  const [error, setError] = useState('')
  const pending = useRef(false)
  useEffect(() => { if (window.location.hash && !user?.must_change_pwd) window.history.replaceState(window.history.state, '', '/pair') }, [])
  const inspect = async () => {
    if (pending.current) return
    pending.current = true; setBusy(true); setError('')
    try { const { data } = await authApi.inspectPairing(code); setReview(data.data); setReviewedCode(code) }
    catch (cause) { setError((cause as { response?: { data?: { error?: string } } }).response?.data?.error || '暂时无法查看配对请求，请重试') }
    finally { pending.current = false; setBusy(false) }
  }
  const approve = async () => {
    if (!review || pending.current) return
    pending.current = true; setBusy(true); setError('')
    try { await authApi.approvePairing(reviewedCode); setDone(true) }
    catch (cause) { setError((cause as { response?: { data?: { error?: string } } }).response?.data?.error || '授权未能完成，请重试') }
    finally { pending.current = false; setBusy(false) }
  }
  useEffect(() => {
    if (!initialCode || !user || user.must_change_pwd) return
    const timer = window.setTimeout(() => void inspect(), 0)
    return () => window.clearTimeout(timer)
  }, [initialCode, user?.id, user?.must_change_pwd])
  if (user?.must_change_pwd) return <Navigate to="/force-change-password" state={{ pairCode: code }} replace />
  if (!user) return <Section title="确认新设备登录">
    <p className="mb-4">请先登录你的家庭账号，再确认新设备。</p>
    <Link to={'/login#' + new URLSearchParams({ pair: code }).toString()} className="nv-button" data-variant="primary">登录后确认</Link>
  </Section>
  return <Section title="确认新设备登录" description="只确认你正在使用、且配对码一致的新设备。">
    <div className="max-w-xl space-y-4">
      {done ? <><p role="status">已确认，请回到新设备。配对只可使用一次。</p><Link to="/my" className="nv-button">返回我的</Link></> : <>
        <p>将使用账号 <strong>{user.nickname || user.username}</strong> 登录新设备。</p>
        {user.child_mode && <p>新设备继续使用这个儿童账号的内容范围和观看时间。</p>}
        {!review ? <form className="space-y-3" onSubmit={event => { event.preventDefault(); void inspect() }}>
          <label className="block text-sm">新设备上的配对码<Input aria-label="新设备上的配对码" value={code} onChange={event => setCode(event.target.value.toUpperCase())} disabled={busy} autoComplete="off" maxLength={32} required /></label>
          <Button type="submit" variant="primary" disabled={busy || !code.trim()}>查看配对设备</Button>
        </form> : <div className="space-y-4 rounded-lg border border-[var(--nv-border-default)] p-4">
          <p>新设备：<strong>{review.device_name}</strong></p>
          <p className="font-mono text-xl tracking-widest">{reviewedCode}</p>
          <p className="text-sm">{new Date(review.expires_at).toLocaleTimeString()} 前有效。请与新设备屏幕上的名称和配对码核对。</p>
          {user.role === 'admin' && <p className="text-sm">这会让新设备拥有管理员权限。给家人使用时，请先登录对应家人的账号。</p>}
          <div className="flex flex-wrap gap-3"><Button variant="primary" disabled={busy} onClick={() => void approve()}>确认授权此账号</Button><Button disabled={busy} onClick={() => { setReview(undefined); setError('') }}>重新输入</Button></div>
        </div>}
        {error && <p role="alert">{error}</p>}
        <Link to="/my" className="nv-button">取消并返回</Link>
      </>}
    </div>
  </Section>
}
