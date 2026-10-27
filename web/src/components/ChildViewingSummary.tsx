import { useCallback, useEffect, useRef, useState } from 'react'
import { userApi, type FamilyViewingStatus } from '@/api/user'
import { Button, Section } from '@/components/design-system'

function duration(seconds: number) {
  const minutes = Math.floor(seconds / 60)
  return minutes > 0 ? `${minutes} 分 ${Math.floor(seconds % 60)} 秒` : `${Math.floor(seconds)} 秒`
}

export default function ChildViewingSummary() {
  const [status, setStatus] = useState<FamilyViewingStatus>()
  const [failed, setFailed] = useState(false)
  const request = useRef(0)
  const load = useCallback(() => {
    const generation = ++request.current
    void userApi.familyControls().then(({ data }) => { if (generation === request.current) { setStatus(data.data); setFailed(false) } }).catch(() => { if (generation === request.current) setFailed(true) })
  }, [])
  useEffect(() => {
    load()
    const focus = () => { load() }
    window.addEventListener('focus', focus)
    return () => { request.current++; window.removeEventListener('focus', focus) }
  }, [load])
  return <Section title="今天的观看时间" description="由家长设置，按服务器当天累计。">
    {failed ? <div role="alert">暂时无法读取观看时间。<Button onClick={() => { load() }}>重试</Button></div> : !status ? <p>正在读取…</p> : <div className="space-y-2 text-sm">
      <p>{status.daily_limit_seconds > 0 ? `今天还可看 ${duration(status.remaining_seconds)}` : '家长暂未设置每日时间限制'}</p>
      <p className="text-[var(--nv-text-tertiary)]">今天已计入 {duration(status.used_seconds)} · {status.day}（{status.timezone}）</p>
    </div>}
  </Section>
}
