import { useState } from 'react'
import type { AgentRun } from '@/api/viewerAgent'
import type { useAgentScanTasks } from '@/components/AgentScanTasks'
import { Button } from '@/components/design-system'

export default function AgentMovieMatchTask({ run, tasks }: { run: AgentRun; tasks: ReturnType<typeof useAgentScanTasks> }) {
  const [query, setQuery] = useState(run.match_query || '')
  const [year, setYear] = useState(run.match_year ? String(run.match_year) : '')
  const [editing, setEditing] = useState(false)
  const dirty = query.trim() !== (run.match_query || '') || (Number(year) || 0) !== (run.match_year || 0)
  const operate = (work: Promise<unknown>) => { void work.catch(() => {}) }
  const candidates = run.status === 'draft' ? run.match_candidates || [] : run.selected_match ? [run.selected_match] : []
  return <div className="space-y-3 text-sm">
    <p className="break-words">本地影片：{run.media_title} · {run.media_year || '年份未知'}</p>
    {run.status === 'draft' && !editing && <div className="space-y-1"><p className="break-words">搜索：{run.match_query} · {run.match_year || '不限年份'}</p><Button disabled={tasks.pending} onClick={() => setEditing(true)}>修改片名或年份</Button></div>}
    {run.status === 'draft' && editing && <form className="space-y-2" onSubmit={event => { event.preventDefault(); if (run.media_id) operate(tasks.prepareMatch(run.media_id, query.trim(), Number(year) || 0)) }}>
      <label className="block">影片名称<input aria-label="影片匹配关键词" value={query} onChange={event => setQuery(event.target.value)} required maxLength={120} disabled={tasks.pending} className="mt-1 min-h-11 w-full min-w-0 rounded-lg border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] px-3" /></label>
      <label className="block">年份（可留空）<input aria-label="影片匹配年份" type="number" inputMode="numeric" min={1888} max={2100} value={year} onChange={event => setYear(event.target.value)} disabled={tasks.pending} className="mt-1 min-h-11 w-full min-w-0 rounded-lg border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] px-3" /></label>
      <div className="flex flex-wrap gap-2"><Button type="submit" disabled={tasks.pending || !query.trim()}>重新搜索候选</Button><Button disabled={tasks.pending} onClick={() => { setQuery(run.match_query || ''); setYear(run.match_year ? String(run.match_year) : ''); setEditing(false) }}>取消修改</Button></div>
      {dirty && <p role="status">搜索条件已更改，请先重新搜索，再选择候选。</p>}
    </form>}
    {candidates.map(candidate => <div key={candidate.id} className="space-y-2 rounded-lg border border-[var(--nv-border-default)] p-3 break-words" aria-label={`匹配候选 ${candidate.title} ${candidate.year || '年份未知'}`}>
      <p className="font-medium">{candidate.title} · {candidate.year || '年份未知'}</p>
      {candidate.original_title && <p>原名：{candidate.original_title}</p>}
      <p>{candidate.overview || '资料源暂无简介，请打开来源核对。'}</p>
      <ul className="space-y-1 text-[var(--nv-text-secondary)]">{candidate.evidence.map(reason => <li key={reason}>{reason}</li>)}</ul>
      <a href={candidate.source_url} target="_blank" rel="noopener noreferrer" className="underline">打开 TMDb 来源核对</a>
      {run.status === 'draft' && <div><Button aria-pressed={run.selected_match?.id === candidate.id} disabled={tasks.pending || dirty} onClick={() => operate(tasks.selectMatch(run, candidate.id))}>{run.selected_match?.id === candidate.id ? '已选择此候选' : '选择此候选'}</Button></div>}
    </div>)}
    {run.status === 'draft' && <>
      <p>{run.effect}</p>
      <p>选择候选后仍需明确确认。有效期至 {new Date(run.expires_at).toLocaleTimeString()}。</p>
      {run.selected_match && <p className="font-medium">即将关联：{run.media_title} → {run.selected_match.title}（{run.selected_match.year || '年份未知'}）</p>}
      <div className="flex flex-wrap gap-2">{run.selected_match && <Button disabled={tasks.pending || dirty} onClick={() => operate(tasks.confirm(run))}>确认匹配此影片</Button>}<Button disabled={tasks.pending} onClick={() => operate(tasks.cancel(run))}>取消匹配预览</Button></div>
    </>}
    {run.status === 'completed' && <>
      <div className="flex flex-wrap gap-2"><Button disabled={tasks.pending} onClick={() => { if (run.media_id) operate(tasks.prepareMetadata(run.media_id)) }}>预览资料补全</Button><Button disabled={tasks.pending} onClick={() => operate(tasks.undoMetadata(run))}>撤销本次匹配</Button></div>
      <p className="text-[var(--nv-text-secondary)]">已补全的资料需要先撤销补全，才能安全撤销本次匹配。</p>
    </>}
    {run.status === 'reverted' && <Button disabled={tasks.pending} onClick={() => { if (run.media_id) operate(tasks.prepareMatch(run.media_id,run.match_query,run.match_year)) }}>重新搜索匹配</Button>}
  </div>
}
