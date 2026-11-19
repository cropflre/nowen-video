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
  const limitSeconds = status?.daily_limit_seconds ?? 0
  const usedPercent = limitSeconds > 0 && status ? Math.min(100, Math.round((status.used_seconds / limitSeconds) * 100)) : 0
  const lowRemaining = Boolean(status && limitSeconds > 0 && status.remaining_seconds > 0 && status.remaining_seconds <= 300)
  return <Section title="今天的观看时间" description="由家长设置，按服务器当天累计。">
    {failed ? <div role="alert">暂时无法读取观看时间。<Button onClick={() => { load() }}>重试</Button></div> : !status ? <p>正在读取…</p> : <div className="space-y-2 text-sm">
      <p>{limitSeconds > 0 ? `今天还可看 ${duration(status.remaining_seconds)}` : '家长暂未设置每日时间限制'}</p>
      {limitSeconds > 0 && (
        <div
          data-testid="family-viewing-progress"
          role="progressbar"
          aria-label="今日观看进度"
          aria-valuemin={0}
          aria-valuemax={100}
          aria-valuenow={usedPercent}
          aria-valuetext={`今天已看 ${duration(status.used_seconds)}，还可看 ${duration(status.remaining_seconds)}`}
          className="h-2 w-full overflow-hidden rounded-full bg-[var(--nv-fill-hover)]"
        >
          <span className="block h-full rounded-full bg-[var(--nv-action-primary)] transition-[width]" style={{ width: `${usedPercent}%` }} />
        </div>
      )}
      {lowRemaining && <p className="text-[var(--nv-status-warning)]">还剩不到 5 分钟，请合理安排，时间到会自动暂停。</p>}
      <p className="text-[var(--nv-text-tertiary)]">今天已计入 {duration(status.used_seconds)} · {status.day}（{status.timezone}）</p>
    </div>}
  </Section>
}
