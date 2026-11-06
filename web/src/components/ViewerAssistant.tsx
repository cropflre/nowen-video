import { useCallback, useEffect, useLayoutEffect, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { useLocation, useNavigate } from 'react-router-dom'
import { MessageCircle, Send } from 'lucide-react'
import { viewerAgentApi, type AgentAction, type AgentCard, type AgentSearch, type AgentManagement } from '@/api/viewerAgent'
import { userApi } from '@/api/user'
import { taskCenterApi } from '@/api/tasks'
import { useServerProfileStore } from '@/stores/serverProfile'
import { useAuthStore } from '@/stores/auth'
import { usePlayerStore } from '@/stores/player'
import { useAssistantWindowStore, assistantViewport } from '@/stores/assistantWindow'
import { clampAssistantLauncher, isAssistantPlayerPath, shouldHideForRoute, type AssistantPoint, type AssistantVisibility } from '@/utils/assistantWindow'
import { prepareChildViewing } from '@/hooks/useChildViewing'
import { Button } from '@/components/design-system'
import { useToast } from '@/components/Toast'
import AssistantWindow from '@/components/AssistantWindow'
import type { ViewingPreferences } from '@/types'
import AgentScanTasks, { useAgentScanTasks } from '@/components/AgentScanTasks'

interface Turn { role: 'user' | 'assistant'; text: string; cards?: AgentCard[]; search?: AgentSearch; management?: AgentManagement; metadataCandidates?: boolean; matchCandidates?: boolean; scanLibraries?: { id: string; name: string }[] }

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
  const toast = useToast()
  const userId = useAuthStore(state => state.user?.id) || ''
  const preferences = useAssistantWindowStore(state => state.preferences)
  const hydrate = useAssistantWindowStore(state => state.hydrate)
  const setVisibility = useAssistantWindowStore(state => state.setVisibility)
  const setPosition = useAssistantWindowStore(state => state.setPosition)
  const setSize = useAssistantWindowStore(state => state.setSize)
  const setLauncher = useAssistantWindowStore(state => state.setLauncher)
  const resetWindow = useAssistantWindowStore(state => state.resetWindow)
  const syncViewport = useAssistantWindowStore(state => state.syncViewport)
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
  const launcherRef = useRef<HTMLButtonElement>(null)
  const routeRef = useRef('')
  const location = useLocation()
  const navigate = useNavigate()
  routeRef.current = location.pathname
  const owner = useAuthStore.getState().user?.id
  const admin = useAuthStore(state => state.user?.role === 'admin' && !state.user.child_mode)
  const open = preferences.visibility === 'expanded'
  const scanTasks = useAgentScanTasks(open, admin, owner)

  // The saved state is read before paint so the window never jumps on load.
  useLayoutEffect(() => { hydrate(userId) }, [hydrate, userId])
  useEffect(() => {
    const onViewportChange = () => syncViewport()
    window.addEventListener('resize', onViewportChange)
    window.addEventListener('orientationchange', onViewportChange)
    return () => {
      window.removeEventListener('resize', onViewportChange)
      window.removeEventListener('orientationchange', onViewportChange)
    }
  }, [syncViewport])

  // Watching a movie should be nothing but the movie: entering the player hides
  // the panel and its launcher, and leaving restores whatever was there before.
  const playerRoute = isAssistantPlayerPath(location.pathname)
  const playerRouteRef = useRef(false)
  // Persisted per account so a crash or reload on the player still restores the
  // panel next time; it only stores which state to bring back, never content.
  const playbackKey = `nowen-assistant-window:hidden-for-playback:${userId}`
  const rememberPlaybackVisibility = (previous: AssistantVisibility | null) => {
    try {
      if (previous) localStorage.setItem(playbackKey, previous)
      else localStorage.removeItem(playbackKey)
    } catch {
      // Storage can be unavailable; the panel still hides.
    }
  }
  const previousPlaybackVisibility = (): AssistantVisibility | null => {
    try {
      const saved = localStorage.getItem(playbackKey)
      return saved === 'expanded' || saved === 'minimized' ? saved : null
    } catch {
      return null
    }
  }
  useLayoutEffect(() => {
    const enteringPlayer = playerRoute && !playerRouteRef.current
    // Read the live value: on a direct load of /play the hydration effect above
    // has just written the saved state without a re-render yet.
    const visibilityNow = useAssistantWindowStore.getState().preferences.visibility
    if (enteringPlayer && shouldHideForRoute(location.pathname, visibilityNow)) {
      rememberPlaybackVisibility(visibilityNow)
      setVisibility('hidden')
    } else if (!playerRoute) {
      // Works for an in-app navigation and for one that reloads the document.
      const previous = previousPlaybackVisibility()
      if (previous) {
        if (visibilityNow === 'hidden') setVisibility(previous)
        rememberPlaybackVisibility(null)
      }
    }
    playerRouteRef.current = playerRoute
  }, [location.pathname, playerRoute, preferences.visibility, setVisibility, playbackKey])

  useEffect(() => { alive.current = true; return () => { alive.current = false; abortRef.current?.abort() } }, [])
  useEffect(() => { if (open) endRef.current?.scrollIntoView({ block: 'nearest' }) }, [turns, busy, open])

  // The pill is content sized: measure it and pull it back inside the viewport
  // before paint, otherwise a wide label overflows the right edge.
  useLayoutEffect(() => {
    if (preferences.visibility !== 'minimized') return
    const element = launcherRef.current
    if (!element) return
    const rect = element.getBoundingClientRect()
    if (!rect.width || !rect.height) return
    const size = { width: rect.width, height: rect.height }
    const next = clampAssistantLauncher(preferences.launcher, assistantViewport(), size)
    if (next.x !== preferences.launcher.x || next.y !== preferences.launcher.y) setLauncher(next, size)
  }, [preferences.launcher, preferences.visibility, setLauncher])

  // Keep the launcher keyboard reachable after minimising.
  const previousVisibility = useRef(preferences.visibility)
  useEffect(() => {
    if (previousVisibility.current === 'expanded' && preferences.visibility === 'minimized') launcherRef.current?.focus()
    previousVisibility.current = preferences.visibility
  }, [preferences.visibility])

  const launcherDrag = useRef<{ pointerId: number; startX: number; startY: number; origin: AssistantPoint; latest: AssistantPoint; size: { width: number; height: number }; moved: boolean } | null>(null)
  const suppressLauncherClick = useRef(false)
  const startLauncherDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    if (event.pointerType === 'mouse' && event.button !== 0) return
    suppressLauncherClick.current = false
    event.currentTarget.setPointerCapture(event.pointerId)
    const rect = event.currentTarget.getBoundingClientRect()
    launcherDrag.current = { pointerId: event.pointerId, startX: event.clientX, startY: event.clientY, origin: preferences.launcher, latest: preferences.launcher, size: { width: rect.width, height: rect.height }, moved: false }
  }
  const moveLauncher = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const current = launcherDrag.current
    if (!current || current.pointerId !== event.pointerId) return
    const dx = event.clientX - current.startX
    const dy = event.clientY - current.startY
    if (!current.moved && Math.abs(dx) < 3 && Math.abs(dy) < 3) return
    current.moved = true
    event.currentTarget.dataset.dragging = 'true'
    current.latest = clampAssistantLauncher({ x: current.origin.x + dx, y: current.origin.y + dy }, assistantViewport(), current.size)
    event.currentTarget.style.left = `${current.latest.x}px`
    event.currentTarget.style.top = `${current.latest.y}px`
  }
  const finishLauncherDrag = (event: ReactPointerEvent<HTMLButtonElement>) => {
    const current = launcherDrag.current
    launcherDrag.current = null
    delete event.currentTarget.dataset.dragging
    if (!current || current.pointerId !== event.pointerId || !current.moved) return
    suppressLauncherClick.current = true
    setLauncher(current.latest, current.size)
  }
  const expand = () => {
    if (suppressLauncherClick.current) { suppressLauncherClick.current = false; return }
    setVisibility('expanded')
  }
  const minimize = useCallback(() => setVisibility('minimized'), [setVisibility])
  const hideAssistant = () => {
    setVisibility('hidden')
    toast.info('问问助手已隐藏。可以在「我的 → 个人设置」重新开启。')
  }

  const active = () => alive.current && useAuthStore.getState().isAuthenticated && useAuthStore.getState().user?.id === owner
  const add = (turn: Turn) => { if (active()) setTurns(previous => [...previous, turn].slice(-30)) }
  const currentID = () => /^\/play\/([^/]+)$/.exec(routeRef.current)?.[1] || ''

  // Acknowledgements come from the authenticated API or the actual video node,
  // never from a model's assertion. Route changes invalidate player commands.
  const execute = async (action: AgentAction): Promise<string> => {
    if (!active()) throw new Error('账号已切换，请重新操作')
    const target = action.target
    if (action.kind === 'prepare_match') {
      if (!target || target.kind !== 'media') throw new Error('请先选择电影')
      await scanTasks.prepareMatch(target.id)
      return '已搜索影片候选。请核对片名、年份和简介，选择候选后再明确确认匹配。'
    }
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
      // The player route hides the assistant by itself; series keeps the launcher.
      if (target.kind === 'series') setVisibility('minimized')
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
      const nextPreferences: ViewingPreferences = { large_text: previous?.large_text ?? false, subtitle_language: previous?.subtitle_language || 'auto', auto_play_next: previous?.auto_play_next ?? true }
      if (action.kind === 'large_text_on' || action.kind === 'large_text_off') nextPreferences.large_text = action.kind === 'large_text_on'
      else {
        const language = action.kind.slice('subtitle_'.length)
        if (!['zh', 'en', 'ja', 'auto', 'off'].includes(language)) throw new Error('暂不支持这个字幕设置')
        nextPreferences.subtitle_language = language as ViewingPreferences['subtitle_language']
      }
      const saved = await userApi.updateProfile({ viewing_preferences: nextPreferences })
      if (!active() || saved.data.data?.id !== owner) throw new Error('无法确认设置结果，请重新打开我的页面')
      useAuthStore.getState().updateUser(saved.data.data)
      if (action.kind.startsWith('large_text_')) return saved.data.data.viewing_preferences?.large_text ? '已开启大字模式。' : '已恢复正常字体。'
      if (saved.data.data.viewing_preferences?.subtitle_language !== nextPreferences.subtitle_language) throw new Error('字幕偏好未能保存，请在我的页面重试')
      const labels = { zh: '中文', en: '英文', ja: '日文', auto: '自动', off: '关闭' }
      return `字幕偏好已保存为${labels[nextPreferences.subtitle_language]}。${nextPreferences.subtitle_language === 'off' ? '' : '实际显示取决于影片是否有可用字幕。'}`
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
      const metadataTarget = (message.includes('补全') && message.includes('资料')) || message.startsWith('匹配') ? /^\/media\/([^/]+)$/.exec(routeRef.current)?.[1] || startingMedia : startingMedia
      const { data } = await viewerAgentApi.chat(message, cardsRef.current.map(({ id, kind }) => ({ id, kind })), metadataTarget, searchRef.current, abort.signal)
      if (!active()) return
      const reply = data.data
      if (!reply || !Array.isArray(reply.cards)) throw new Error('暂时无法读取结果，请使用找片页面')
      setMode(reply.mode)
      if (reply.search) searchRef.current = reply.search
      if (reply.search || reply.cards.length) cardsRef.current = reply.cards
      let text = reply.message
      if (reply.action) text = await execute(reply.action)
      add({ role: 'assistant', text, cards: reply.cards, search: reply.search, management: reply.management, scanLibraries: reply.scan_libraries, metadataCandidates: reply.metadata_candidates, matchCandidates: reply.match_candidates })
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
    {preferences.visibility === 'minimized' && <button
      ref={launcherRef}
      type="button"
      onClick={expand}
      onPointerDown={startLauncherDrag}
      onPointerMove={moveLauncher}
      onPointerUp={finishLauncherDrag}
      onPointerCancel={finishLauncherDrag}
      className="nv-assistant-launcher"
      style={{ left: preferences.launcher.x, top: preferences.launcher.y }}
      aria-label="打开观影助手"
      aria-haspopup="dialog"
    ><MessageCircle size={20} aria-hidden="true" /><span>问问助手</span>{busy && <span className="nv-assistant-launcher__dot" aria-hidden="true" />}</button>}
    <AssistantWindow
      open={open}
      position={preferences.position}
      size={preferences.size}
      onMove={setPosition}
      onResize={setSize}
      onMinimize={minimize}
      onHide={hideAssistant}
      onReset={resetWindow}
    >
      <div className="flex min-h-0 flex-1 flex-col">
        {!turns.length && <div className="shrink-0">{!scanTasks.runs.length && <p className="mb-3 text-sm text-[var(--nv-text-secondary)]">说说想看什么，也可以让我帮你续播、加入想看或调整这个网页的播放。</p>}
        <div className="mb-3 flex flex-wrap gap-2">{(admin && scanTasks.runs.length ? ['帮我找一部喜剧', '查找新影片', '匹配影片', '补全资料'] : ['帮我找一部喜剧', '继续看', '我的想看', '字体大一点', ...(admin ? ['检查影视库', '检查任务', '查找新影片', '匹配影片', '补全资料'] : []), ...(currentID() ? ['暂停', '继续播放', '中文字幕'] : [])]).map(text => <Button key={text} disabled={busy || scanTasks.pending} onClick={() => void send(text)}>{text}</Button>)}</div></div>}
        {mode === 'fallback' && <p className="mb-2 shrink-0 text-sm text-[var(--nv-text-tertiary)]">智能理解暂时不可用，已使用基础找片。可以简化条件或输入片名。</p>}
        <div role="log" aria-label="观影对话" aria-live="polite" className="min-h-0 flex-1 space-y-4 overflow-x-hidden overflow-y-auto overscroll-contain pr-1">
          {turns.length === 0 && <div className="flex h-full min-h-[120px] flex-col items-center justify-center gap-2 px-3 text-center text-sm text-[var(--nv-text-tertiary)]"><MessageCircle size={22} aria-hidden="true" /><p className="w-full break-words">试试：“找一部 90 分钟以内、没看过的喜剧”。</p></div>}
          {turns.map((turn, index) => <div key={index} className="space-y-2 rounded-xl bg-[var(--nv-bg-surface-soft)] p-3">
            <p className="text-xs text-[var(--nv-text-tertiary)]">{turn.role === 'user' ? '你' : '助手'}</p>
            <p className="whitespace-pre-wrap break-words leading-relaxed">{turn.text}</p>
            {turn.scanLibraries?.map(library => <Button key={library.id} disabled={busy || scanTasks.pending} onClick={() => { void scanTasks.prepare(library.id).catch(reportFailure) }}>预览：{library.name}</Button>)}
            {turn.search && describeSearch(turn.search) && <p className="text-sm text-[var(--nv-text-secondary)]">条件：{describeSearch(turn.search)}</p>}
            {turn.management && <div className="space-y-2">
              <p>{turn.management.library_count} 个影视库 · {turn.management.media_count} 部影片</p>
              <p>资料待补：{turn.management.needs_metadata} 部 · 分级待审核：{turn.management.needs_rating} 部</p>
              {turn.management.libraries.map(library => <Button key={library.id} disabled={busy} onClick={() => { navigate(`/library/${encodeURIComponent(library.id)}`); setVisibility('minimized') }}>{library.name}（{library.media_count} 部）</Button>)}
              {turn.management.library_count > turn.management.libraries.length && <p className="text-sm text-[var(--nv-text-tertiary)]">这里显示前 {turn.management.libraries.length} 个库，其余可在家庭管理中查看。</p>}
            </div>}
            {turn.cards?.map((card, i) => <div key={`${card.kind}:${card.id}`} className="rounded-lg border border-[var(--nv-border-default)] p-3">
              <p className="font-medium">{i + 1}. {card.title}</p>
              <p className="my-2 text-sm text-[var(--nv-text-secondary)]">{[card.kind === 'series' ? '剧集' : '影片', card.year || '', card.rating > 0 ? `${card.rating} 分` : '', card.duration ? `${Math.ceil(card.duration / 60)} 分钟` : '', card.position ? `已看到 ${Math.floor(card.position / 60)} 分钟` : ''].filter(Boolean).join(' · ')}</p>
              <div className="flex flex-wrap gap-2">{admin && turn.matchCandidates && card.kind === 'media' && <Button disabled={busy || scanTasks.pending} onClick={() => { void scanTasks.prepareMatch(card.id).catch(reportFailure) }}>查找影片候选</Button>}{admin && turn.metadataCandidates && card.kind === 'media' && <Button disabled={busy || scanTasks.pending} onClick={() => { void scanTasks.prepareMetadata(card.id).catch(reportFailure) }}>预览资料补全</Button>}<Button disabled={busy} onClick={() => void act(card, 'play')}>{card.kind === 'series' ? '选集播放' : card.position ? '接着看' : '播放'}</Button>{card.kind === 'media' && <Button disabled={busy} onClick={() => void act(card, 'favorite')}>加入想看</Button>}</div>
            </div>)}
          </div>)}
          {busy && <p role="status" className="py-2 text-sm">正在处理，请稍等…</p>}
          <div ref={endRef} />
        </div>
        {admin && <div className="mt-3 max-h-[38%] shrink-0 overflow-y-auto overscroll-contain pr-1"><AgentScanTasks tasks={scanTasks} /></div>}
        <form className="mt-3 flex shrink-0 gap-2 border-t border-[var(--nv-border-subtle)] pt-3" onSubmit={event => { event.preventDefault(); void send(input.trim()) }}>
          <input aria-label="告诉助手你想做什么" placeholder="想看什么，直接说" maxLength={500} value={input} disabled={busy} onChange={event => setInput(event.target.value)} className="min-h-11 min-w-0 flex-1 rounded-lg border border-[var(--nv-border-default)] bg-[var(--nv-bg-surface)] px-3" />
          <Button type="submit" disabled={busy || !input.trim()} aria-label="发送消息"><Send size={18} aria-hidden="true" />发送</Button>
        </form>
        <div className="mt-2 flex shrink-0 flex-col items-start gap-2 text-sm sm:flex-row sm:flex-wrap sm:items-center sm:justify-between"><span className="min-w-0 break-words text-[var(--nv-text-tertiary)]">{admin && scanTasks.runs.length ? '任务记录会保留。' : '对话仅保留在本次页面会话，切换账号后清空。'}</span><Button disabled={busy || !turns.length} onClick={() => { setTurns([]); cardsRef.current = []; searchRef.current = undefined; setMode('') }}>清空对话</Button></div>
      </div>
    </AssistantWindow>
  </>
}
