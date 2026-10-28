import { useEffect, useRef, useState } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { MessageCircle, Send } from 'lucide-react'
import { viewerAgentApi, type AgentAction, type AgentCard, type AgentSearch, type AgentManagement } from '@/api/viewerAgent'
import { userApi } from '@/api/user'
import { taskCenterApi } from '@/api/tasks'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useAuthStore } from '@/stores/auth'
import { usePlayerStore } from '@/stores/player'
import { prepareChildViewing } from '@/hooks/useChildViewing'
import { Button, Modal, ModalBody, ModalHeader } from '@/components/design-system'
import type { ViewingPreferences } from '@/types'
import AgentScanTasks, { useAgentScanTasks } from '@/components/AgentScanTasks'

interface Turn { role: 'user' | 'assistant'; text: string; cards?: AgentCard[]; search?: AgentSearch; management?: AgentManagement; metadataCandidates?: boolean; scanLibraries?: { id: string; name: string }[] }

function describeSearch(search: AgentSearch) {
  const years = search.year_min ? search.year_max === search.year_min ? `${search.year_min}年` : search.year_max ? `${search.year_min}–${search.year_max}年` : `${search.year_min}年起` : search.year_max ? `${search.year_max}年以前` : ''
  return [search.query, search.genre, search.type === 'movie' ? '电影' : search.type === 'series' ? '剧集' : '', years, search.min_rating ? `${search.min_rating}分以上` : '', search.max_minutes ? `${search.max_minutes}分钟以内` : '', search.unwatched ? '没看过' : ''].filter(Boolean).join(' · ')
}

export default function ViewerAssistant() {
  const user = useAuthStore(state => state.user)
  const authenticated = useAuthStore(state => state.isAuthenticated)
  const location = useLocation()
  if (!authenticated || !user || user.must_change_pwd || ['/login', '/pair', '/force-change-password'].includes(location.pathname)) return null
  return <Assistant key={`${user.id}:${user.role}:${user.child_mode}`} />
}

function Assistant() {
  const [open, setOpen] = useState(false)
  const [input, setInput] = useState('')
  const [turns, setTurns] = useState<Turn[]>([])
  const [busy, setBusy] = useState(false)
  const [mode, setMode] = useState('')
  const cardsRef = useRef<AgentCard[]>([])
  const searchRef = useRef<AgentSearch>()
  const busyRef = useRef(false)
  const alive = useRef(true)
  const abortRef = useRef<AbortController>()
  const endRef = useRef<HTMLDivElement>(null)
  const routeRef = useRef('')
  const location = useLocation()
  const navigate = useNavigate()
  routeRef.current = location.pathname
  const owner = useAuthStore.getState().user?.id
  const admin = useAuthStore(state => state.user?.role === 'admin' && !state.user.child_mode)
  const scanTasks = useAgentScanTasks(open, admin, owner)
  useEffect(() => { alive.current = true; return () => { alive.current = false; abortRef.current?.abort() } }, [])
  useEffect(() => { if (open) endRef.current?.scrollIntoView({ block: 'nearest' }) }, [turns, busy, open])
  const active = () => alive.current && useAuthStore.getState().isAuthenticated && useAuthStore.getState().user?.id === owner
  const add = (turn: Turn) => { if (active()) setTurns(previous => [...previous, turn].slice(-30)) }
  const currentID = () => /^\/play\/([^/]+)$/.exec(routeRef.current)?.[1] || ''

  // Acknowledgements come from the authenticated API or the actual video node,
  // never from a model's assertion. Route changes invalidate player commands.
  const execute = async (action: AgentAction): Promise<string> => {
    if (!active()) throw new Error('账号已切换，请重新操作')
    const target = action.target
    if (action.kind === 'prepare_metadata') {
      if (!target || target.kind !== 'media') throw new Error('请先选择电影')
      await scanTasks.prepareMetadata(target.id)
      return '资料预览已生成。请核对影片、资料来源及每项变化，再点击“确认补全资料”。'
    }
    if (action.kind === 'prepare_scan') {
      if (!action.library_id) throw new Error('请先选择电影库')
      await scanTasks.prepare(action.library_id)
      return '任务预览已生成。请核对范围，再点击“确认查找新影片”。'
    }
    if (action.kind === 'inspect_tasks') {
      if (!useAuthStore.getState().user || useAuthStore.getState().user?.role !== 'admin' || useAuthStore.getState().user?.child_mode) throw new Error('只有管理员可以查看任务中心')
      const ownRuns = (await viewerAgentApi.runs()).data.data
      if (!Array.isArray(ownRuns)) throw new Error('暂时无法确认助手任务状态，请稍后重试')
      const ownSummary = `最近 ${ownRuns.length} 条助手任务：${ownRuns.filter(run => run.status === 'draft').length} 项等待确认，${ownRuns.filter(run => run.status === 'running').length} 项进行中，${ownRuns.filter(run => run.status === 'completed').length} 项已完成，${ownRuns.filter(run => ['failed', 'interrupted'].includes(run.status)).length} 项未完成。`
      if (!useServerProfileStore.getState().manifest?.capabilities.task_center?.available) return `任务检查完成。${ownSummary}`
      const snapshot = (await taskCenterApi.list({ limit: 50 })).data.data
      if (!snapshot || !Array.isArray(snapshot.tasks) || !snapshot.summary || !Number.isFinite(snapshot.summary.total) || !Number.isFinite(snapshot.summary.active)) throw new Error('暂时无法确认任务状态，请稍后重试')
      const failures = snapshot.tasks.filter(task => task.status === 'failed').slice(0, 6)
      return `任务检查完成：任务中心当前返回 ${snapshot.summary.total} 项任务，${snapshot.summary.active} 项进行中，${snapshot.summary.by_status?.failed || 0} 项失败。\n${ownSummary}${failures.length ? `\n失败项目：\n${failures.map(task => `${task.title}：${task.message || '请查看任务详情'}`).join('\n')}` : ''}`
    }
    if (action.kind === 'play') {
      if (!target || !['media', 'series'].includes(target.kind)) throw new Error('请先选择一部影片')
      navigate(`/${target.kind === 'series' ? 'series' : 'play'}/${encodeURIComponent(target.id)}`)
      setOpen(false)
      return target.kind === 'series' ? '已打开剧集详情，请选择一集播放。' : '已打开播放器；播放状态以播放器为准。'
    }
    if (action.kind === 'favorite' || action.kind === 'unfavorite') {
      if (!target || target.kind !== 'media') throw new Error('请选择单部影片或单集')
      if (action.kind === 'favorite') await userApi.addFavorite(target.id)
      else await userApi.removeFavorite(target.id)
      return action.kind === 'favorite' ? '已加入想看。' : '已移出想看。'
    }
    if (action.kind.startsWith('large_text_') || action.kind.startsWith('subtitle_')) {
      const response = await userApi.profile()
      if (!active() || response.data.data?.id !== owner) throw new Error('账号已切换，请重新操作')
      const previous = response.data.data.viewing_preferences
      const preferences: ViewingPreferences = { large_text: previous?.large_text ?? false, subtitle_language: previous?.subtitle_language || 'auto', auto_play_next: previous?.auto_play_next ?? true }
      if (action.kind === 'large_text_on' || action.kind === 'large_text_off') preferences.large_text = action.kind === 'large_text_on'
      else {
        const language = action.kind.slice('subtitle_'.length)
        if (!['zh', 'en', 'ja', 'auto', 'off'].includes(language)) throw new Error('暂不支持这个字幕设置')
        preferences.subtitle_language = language as ViewingPreferences['subtitle_language']
      }
      const saved = await userApi.updateProfile({ viewing_preferences: preferences })
      if (!active() || saved.data.data?.id !== owner) throw new Error('无法确认设置结果，请重新打开我的页面')
      useAuthStore.getState().updateUser(saved.data.data)
      if (action.kind.startsWith('large_text_')) return saved.data.data.viewing_preferences?.large_text ? '已开启大字模式。' : '已恢复正常字体。'
      if (saved.data.data.viewing_preferences?.subtitle_language !== preferences.subtitle_language) throw new Error('字幕偏好未能保存，请在我的页面重试')
      const labels = { zh: '中文', en: '英文', ja: '日文', auto: '自动', off: '关闭' }
      return `字幕偏好已保存为${labels[preferences.subtitle_language]}。${preferences.subtitle_language === 'off' ? '' : '实际显示取决于影片是否有可用字幕。'}`
    }
    const mediaID = currentID()
    const videos = document.querySelectorAll<HTMLVideoElement>('video')
    const video = videos.length === 1 ? videos[0] : null
    if (!target || target.kind !== 'media' || mediaID !== target.id || !video) throw new Error('播放器已变化或尚未准备好，请在当前影片重试')
    const stillCurrent = () => active() && currentID() === mediaID && video.isConnected
    if (action.kind === 'pause') {
      video.pause()
      if (!video.paused) throw new Error('未能暂停，请使用播放器按钮')
      return '已暂停当前网页的播放。'
    }
    if (action.kind === 'resume') {
      if (!await prepareChildViewing(mediaID)) throw new Error('暂时不能继续观看，请查看播放器的观看时间提示')
      if (!stillCurrent()) throw new Error('播放器已变化，请重新操作')
      let timeout = 0
      try {
        await Promise.race([video.play(), new Promise<never>((_, reject) => { timeout = window.setTimeout(() => reject(new Error('播放尚未开始，请使用播放器的播放按钮')), 5000) })])
      } catch (error) { video.pause(); throw error }
      finally { window.clearTimeout(timeout) }
      if (!stillCurrent() || video.paused) throw new Error('未能开始播放，请点击播放器的播放按钮')
      return '已继续当前网页的播放。'
    }
    const player = usePlayerStore.getState()
    if (action.kind === 'mute' || action.kind === 'unmute') {
      video.muted = action.kind === 'mute'
      player.setMuted(video.muted)
      return video.muted ? '当前网页已静音。' : '当前网页已取消静音。'
    }
    if (action.kind === 'volume_up' || action.kind === 'volume_down') {
      video.volume = Math.max(0, Math.min(1, video.volume + (action.kind === 'volume_up' ? 0.1 : -0.1)))
      video.muted = false
      player.setVolume(video.volume)
      player.setMuted(false)
      return `当前网页音量为 ${Math.round(video.volume * 100)}%。`
    }
    throw new Error('这个操作暂不支持，请使用页面上的按钮')
  }

  const reportFailure = (error: unknown) => {
    const response = error as { response?: { data?: { error?: string } }; message?: string; isAxiosError?: boolean }
    const reason = response.response?.data?.error || (response.isAxiosError ? '网络暂时不可用，请稍后重试' : response.message) || '请稍后重试'
    add({ role: 'assistant', text: `未能完成：${reason}。也可以使用找片、想看或播放器按钮。` })
  }
  const send = async (message: string) => {
    if (busyRef.current || !message.trim() || !active()) return
    busyRef.current = true; setBusy(true); setInput('')
    add({ role: 'user', text: message })
    const abort = new AbortController(); abortRef.current = abort
    const startingMedia = currentID()
    try {
      const metadataTarget = message.includes('补全') && message.includes('资料') ? /^\/media\/([^/]+)$/.exec(routeRef.current)?.[1] || startingMedia : startingMedia
      const { data } = await viewerAgentApi.chat(message, cardsRef.current.map(({ id, kind }) => ({ id, kind })), metadataTarget, searchRef.current, abort.signal)
      if (!active()) return
      const reply = data.data
      if (!reply || !Array.isArray(reply.cards)) throw new Error('暂时无法读取结果，请使用找片页面')
      setMode(reply.mode)
      if (reply.search) searchRef.current = reply.search
      if (reply.search || reply.cards.length) cardsRef.current = reply.cards
      let text = reply.message
      if (reply.action) text = await execute(reply.action)
      add({ role: 'assistant', text, cards: reply.cards, search: reply.search, management: reply.management, scanLibraries: reply.scan_libraries, metadataCandidates: reply.metadata_candidates })
    } catch (error) { if (!abort.signal.aborted) reportFailure(error) }
    finally { busyRef.current = false; if (active()) setBusy(false) }
  }
  const act = async (card: AgentCard, kind: 'play' | 'favorite' | 'unfavorite') => {
    if (busyRef.current || !active()) return
    busyRef.current = true; setBusy(true)
    try { add({ role: 'assistant', text: await execute({ kind, target: { id: card.id, kind: card.kind } }) }) }
    catch (error) { reportFailure(error) }
    finally { busyRef.current = false; if (active()) setBusy(false) }
  }
  return <>
    <button type="button" onClick={() => setOpen(true)} className="nv-viewer-assistant-launcher fixed z-40 flex min-h-11 items-center gap-2 rounded-full border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] px-4 py-3 text-[var(--nv-text-primary)] shadow-lg" style={{ right: 'max(16px, env(safe-area-inset-right))', bottom: 'max(88px, calc(env(safe-area-inset-bottom) + 76px))' }} aria-label="打开观影助手" aria-haspopup="dialog"><MessageCircle size={20} aria-hidden="true" /><span>问问助手</span></button>
    <Modal open={open} onClose={() => setOpen(false)} size="md" ariaLabel="观影助手">
      <ModalHeader title="观影助手" onClose={() => setOpen(false)} />
      <ModalBody>
        {!turns.length && <>{!scanTasks.runs.length && <p className="mb-3 text-sm text-[var(--nv-text-secondary)]">说说想看什么，也可以让我帮你续播、加入想看或调整这个网页的播放。</p>}
        <div className="mb-3 flex flex-wrap gap-2">{(admin && scanTasks.runs.length ? ['帮我找一部喜剧', '查找新影片', '补全资料'] : ['帮我找一部喜剧', '继续看', '我的想看', '字体大一点', ...(admin ? ['检查影视库', '检查任务', '查找新影片', '补全资料'] : []), ...(currentID() ? ['暂停', '继续播放', '中文字幕'] : [])]).map(text => <Button key={text} disabled={busy || scanTasks.pending} onClick={() => void send(text)}>{text}</Button>)}</div></>}
        {mode === 'fallback' && <p className="mb-2 text-sm text-[var(--nv-text-tertiary)]">智能理解暂时不可用，已使用基础找片。可以简化条件或输入片名。</p>}
        <div role="log" aria-label="观影对话" aria-live="polite" className={`${admin && scanTasks.expanded && scanTasks.runs.length ? 'max-h-[16dvh]' : 'max-h-[42dvh]'} space-y-4 overflow-y-auto overscroll-contain pr-1`}>
          {turns.length === 0 && <p className="py-4 text-[var(--nv-text-tertiary)]">试试：“找一部 90 分钟以内、没看过的喜剧”。</p>}
          {turns.map((turn, index) => <div key={index} className="space-y-2 rounded-xl bg-[var(--nv-bg-surface-soft)] p-3">
            <p className="text-xs text-[var(--nv-text-tertiary)]">{turn.role === 'user' ? '你' : '助手'}</p>
            <p className="whitespace-pre-wrap break-words leading-relaxed">{turn.text}</p>
            {turn.scanLibraries?.map(library => <Button key={library.id} disabled={busy || scanTasks.pending} onClick={() => { void scanTasks.prepare(library.id).catch(reportFailure) }}>预览：{library.name}</Button>)}
            {turn.search && describeSearch(turn.search) && <p className="text-sm text-[var(--nv-text-secondary)]">条件：{describeSearch(turn.search)}</p>}
            {turn.management && <div className="space-y-2">
              <p>{turn.management.library_count} 个影视库 · {turn.management.media_count} 部影片</p>
              <p>资料待补：{turn.management.needs_metadata} 部 · 分级待审核：{turn.management.needs_rating} 部</p>
              {turn.management.libraries.map(library => <Button key={library.id} disabled={busy} onClick={() => { navigate(`/library/${encodeURIComponent(library.id)}`); setOpen(false) }}>{library.name}（{library.media_count} 部）</Button>)}
              {turn.management.library_count > turn.management.libraries.length && <p className="text-sm text-[var(--nv-text-tertiary)]">这里显示前 {turn.management.libraries.length} 个库，其余可在家庭管理中查看。</p>}
            </div>}
            {turn.cards?.map((card, i) => <div key={`${card.kind}:${card.id}`} className="rounded-lg border border-[var(--nv-border-default)] p-3">
              <p className="font-medium">{i + 1}. {card.title}</p>
              <p className="my-2 text-sm text-[var(--nv-text-secondary)]">{[card.kind === 'series' ? '剧集' : '影片', card.year || '', card.rating > 0 ? `${card.rating} 分` : '', card.duration ? `${Math.ceil(card.duration / 60)} 分钟` : '', card.position ? `已看到 ${Math.floor(card.position / 60)} 分钟` : ''].filter(Boolean).join(' · ')}</p>
              <div className="flex flex-wrap gap-2">{admin && turn.metadataCandidates && card.kind === 'media' && <Button disabled={busy || scanTasks.pending} onClick={() => { void scanTasks.prepareMetadata(card.id).catch(reportFailure) }}>预览资料补全</Button>}<Button disabled={busy} onClick={() => void act(card, 'play')}>{card.kind === 'series' ? '选集播放' : card.position ? '接着看' : '播放'}</Button>{card.kind === 'media' && <Button disabled={busy} onClick={() => void act(card, 'favorite')}>加入想看</Button>}</div>
            </div>)}
          </div>)}
          {busy && <p role="status" className="py-2 text-sm">正在处理，请稍等…</p>}
          <div ref={endRef} />
        </div>
        {admin && <AgentScanTasks tasks={scanTasks} />}
        <form className="mt-4 flex gap-2" onSubmit={event => { event.preventDefault(); void send(input.trim()) }}>
          <input aria-label="告诉助手你想做什么" placeholder="想看什么，直接说" maxLength={500} value={input} disabled={busy} onChange={event => setInput(event.target.value)} className="min-h-11 min-w-0 flex-1 rounded-lg border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] px-3" />
          <Button type="submit" disabled={busy || !input.trim()} aria-label="发送消息"><Send size={18} aria-hidden="true" />发送</Button>
        </form>
        <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-sm"><span className="text-[var(--nv-text-tertiary)]">{admin && scanTasks.runs.length ? '任务记录会保留。' : '对话仅保留在本次页面会话，切换账号后清空。'}</span><Button disabled={busy || !turns.length} onClick={() => { setTurns([]); cardsRef.current = []; searchRef.current = undefined; setMode('') }}>清空对话</Button></div>
      </ModalBody>
    </Modal>
  </>
}
