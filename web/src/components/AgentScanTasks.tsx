import { useCallback, useEffect, useRef, useState } from 'react'
import { viewerAgentApi, type AgentRun } from '@/api/viewerAgent'
import { useAuthStore } from '@/stores/auth'
import { Button } from '@/components/design-system'

function message(error: unknown) {
  return (error as { response?: { data?: { error?: string } } }).response?.data?.error || '暂时无法处理任务，请稍后重试'
}

export function useAgentScanTasks(open: boolean, enabled: boolean, owner: string | undefined) {
  const [runs, setRuns] = useState<AgentRun[]>([])
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [expanded, setExpanded] = useState(false)
  const activeRuns = runs.filter(run => ['draft', 'running'].includes(run.status)).map(run => run.id).join(':')
  useEffect(() => { if (activeRuns) setExpanded(true) }, [activeRuns])
  const alive = useRef(true)
  const writing = useRef(false)
  const revision = useRef(0)
  const active = useCallback(() => alive.current && useAuthStore.getState().user?.id === owner && useAuthStore.getState().user?.role === 'admin' && !useAuthStore.getState().user?.child_mode, [owner])
  useEffect(() => { alive.current = true; return () => { alive.current = false } }, [])
  useEffect(() => {
    if (!open || !enabled) return
    let stopped = false
    let fetching = false
    const refresh = async () => {
      if (fetching || !active()) return
      fetching = true
      const current = revision.current
      try {
        const reply = (await viewerAgentApi.runs()).data.data
        if (!Array.isArray(reply)) throw new Error('Invalid task list')
        if (!stopped && active() && current === revision.current) { setRuns(reply); setError('') }
      } catch { if (!stopped && active()) setError('任务记录暂时无法读取，请稍后重新打开助手') }
      finally { fetching = false }
    }
    void refresh()
    const timer = window.setInterval(() => void refresh(), 3000)
    return () => { stopped = true; window.clearInterval(timer) }
  }, [open, enabled, active])
  const write = async (operation: () => ReturnType<typeof viewerAgentApi.prepareScan>) => {
    if (!active() || writing.current) throw new Error('请等待当前任务操作完成')
    writing.current = true; setPending(true); revision.current++
    try {
      const run = (await operation()).data.data
      if (!run?.id) throw new Error('Missing task receipt')
      if (active()) { setRuns(previous => [run, ...previous.filter(item => item.id !== run.id)].slice(0, 20)); setError('') }
      return run
    } catch (cause) { if (active()) setError(message(cause)); throw cause }
    finally { writing.current = false; revision.current++; if (active()) setPending(false) }
  }
  return { runs, pending, error, expanded, setExpanded,
    prepare: (id: string) => write(() => viewerAgentApi.prepareScan(id)),
    prepareMetadata: (id: string) => write(() => viewerAgentApi.prepareMetadata(id)),
    undoMetadata: (run: AgentRun) => write(() => viewerAgentApi.undoMetadata(run)),
    confirm: (run: AgentRun) => write(() => viewerAgentApi.confirmScan(run)),
    cancel: (run: AgentRun) => write(() => viewerAgentApi.cancelScan(run)),
  }
}

export default function AgentScanTasks({ tasks }: { tasks: ReturnType<typeof useAgentScanTasks> }) {
  const [expanded, setExpanded] = useState(false)
  if (!tasks.runs.length && !tasks.error) return null
  const labels = { draft: '等待确认', running: '正在执行', completed: '已完成', failed: '未完成', cancelled: '已取消', expired: '预览已过期', interrupted: '执行中断', reverted: '已撤销' }
  const operate = (operation: Promise<unknown>) => { void operation.catch(() => {}) }
  return <section className="mt-3 space-y-2" aria-label="助手管理任务">
    <div className="flex flex-wrap items-center justify-between gap-2"><h3 className="font-medium">任务预览与回执</h3><Button onClick={() => tasks.setExpanded(!tasks.expanded)}>{tasks.expanded ? '收起任务记录' : '展开任务记录'}</Button></div>
    {tasks.error && <p role="alert" className="text-sm">{tasks.error}</p>}
    {tasks.expanded && tasks.runs.some(run => run.status === 'draft') && <p className="text-xs text-[var(--nv-text-secondary)]">向下核对操作范围和确认按钮。</p>}
    {tasks.expanded && <div className="max-h-[26dvh] space-y-3 overflow-y-auto overscroll-contain">
      {(expanded ? tasks.runs : tasks.runs.slice(0, 3)).map(run => <article key={run.id} className="space-y-2 rounded-lg border border-[var(--nv-border-default)] p-3" aria-label={run.kind === 'fill_movie_metadata' ? `${run.media_title}资料补全任务` : `${run.library_name}查找任务`}>
        <p className="font-medium">{run.kind === 'fill_movie_metadata' ? `补全资料 · ${run.media_title}` : `查找新影片 · ${run.library_name}`} · {labels[run.status]}</p>
        <p className="text-sm leading-relaxed" role={run.status === 'running' ? 'status' : undefined}>{run.message}</p>
        {run.kind === 'fill_movie_metadata' && <>
          <p className="text-sm break-words">资料来源：<a href={run.source_url} target="_blank" rel="noopener noreferrer" className="underline">TMDb · {run.source_title || run.media_title}</a></p>
          <dl className="space-y-2 text-sm">{run.changes?.map(change => <div key={change.field} className="break-words"><dt className="font-medium">{change.label}</dt><dd className="whitespace-pre-wrap">{change.before.trim() || '空缺'} → {change.after}</dd></div>)}</dl>
        </>}
        {run.status === 'draft' && run.kind === 'fill_movie_metadata' && <>
          <p className="text-sm leading-relaxed">{run.effect}</p>
          <p className="text-sm">确认有效期至 {new Date(run.expires_at).toLocaleTimeString()}。影片或影视库资料变化后需要重新预览。</p>
          <div className="flex flex-wrap gap-2"><Button disabled={tasks.pending} onClick={() => operate(tasks.confirm(run))}>确认补全资料</Button><Button disabled={tasks.pending} onClick={() => operate(tasks.cancel(run))}>取消预览</Button></div>
        </>}
        {run.status === 'completed' && run.kind === 'fill_movie_metadata' && <>
          <p className="text-sm">撤销将恢复本次补全前的空缺；如果这些字段已被修改，会保留现有资料并拒绝撤销。</p>
          <Button disabled={tasks.pending} onClick={() => operate(tasks.undoMetadata(run))}>撤销本次补全</Button>
        </>}
        {run.status === 'draft' && run.kind !== 'fill_movie_metadata' && <>
          <p className="text-sm leading-relaxed">{run.effect}</p>
          <p className="text-sm">开始后暂不支持中途取消；失败时可能已登记部分影片，回执会保留实际结果。</p>
          <ul className="space-y-1 text-sm">{run.paths.map(folder => <li key={folder} className="break-all">文件夹：{folder}</li>)}</ul>
          <p className="text-sm">{run.file_filter && run.min_file_size_mb > 0 ? `跳过小于 ${run.min_file_size_mb} MB 的本地视频。` : '按影视库当前文件过滤规则查找。'}确认有效期至 {new Date(run.expires_at).toLocaleTimeString()}。</p>
          <div className="flex flex-wrap gap-2"><Button disabled={tasks.pending} onClick={() => operate(tasks.confirm(run))}>确认查找新影片</Button><Button disabled={tasks.pending} onClick={() => operate(tasks.cancel(run))}>取消预览</Button></div>
        </>}
        {run.status === 'running' && <p className="text-sm text-[var(--nv-text-secondary)]">关闭页面后仍会继续，重新打开助手可查看回执。服务失联超过两分钟后会标记中断，不会自动重跑。</p>}
        {['failed', 'expired', 'interrupted'].includes(run.status) && <Button disabled={tasks.pending} onClick={() => operate(run.kind === 'fill_movie_metadata' && run.media_id ? tasks.prepareMetadata(run.media_id) : tasks.prepare(run.library_id))}>重新预览</Button>}
      </article>)}
    </div>}
    {tasks.expanded && tasks.runs.length > 3 && <Button onClick={() => setExpanded(!expanded)}>{expanded ? '仅显示最近三条' : `查看最近 ${tasks.runs.length} 条记录`}</Button>}
  </section>
}
