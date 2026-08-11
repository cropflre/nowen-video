import { useState, useEffect, useRef, useCallback } from 'react'
import { Search, X, Clock, ChevronRight, Subtitles } from 'lucide-react'
import clsx from 'clsx'

interface SubtitleCue {
  startTime: number
  endTime: number
  text: string
}

interface SubtitleContentSearchProps {
  videoRef: React.RefObject<HTMLVideoElement | null>
  onClose: () => void
  hasActiveSubtitle: boolean
}

const HISTORY_KEY = 'subtitle-search-history'
const MAX_HISTORY = 10

function formatTime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  return `${m}:${String(s).padStart(2, '0')}`
}

function highlightText(text: string, keyword: string): React.ReactNode {
  if (!keyword) return text
  const escaped = keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const parts = text.split(new RegExp(`(${escaped})`, 'gi'))
  return parts.map((part, index) =>
    part.toLowerCase() === keyword.toLowerCase() ? (
      <span key={index} className="rounded bg-[var(--nv-player-accent-soft)] px-0.5 font-semibold text-[var(--nv-player-accent)]">{part}</span>
    ) : (
      <span key={index}>{part}</span>
    )
  )
}

function getSearchHistory(): string[] {
  try {
    const raw = localStorage.getItem(HISTORY_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

function saveSearchHistory(history: string[]) {
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history.slice(0, MAX_HISTORY)))
  } catch {}
}

export default function SubtitleContentSearch({ videoRef, onClose, hasActiveSubtitle }: SubtitleContentSearchProps) {
  const [keyword, setKeyword] = useState('')
  const [results, setResults] = useState<SubtitleCue[]>([])
  const [searched, setSearched] = useState(false)
  const [history, setHistory] = useState<string[]>(getSearchHistory)
  const [showHistory, setShowHistory] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)
  const resultsRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const timer = window.setTimeout(() => inputRef.current?.focus(), 100)
    return () => window.clearTimeout(timer)
  }, [])

  const getCues = useCallback((): SubtitleCue[] => {
    const video = videoRef.current
    if (!video) return []
    const cues: SubtitleCue[] = []
    for (let i = 0; i < video.textTracks.length; i++) {
      const track = video.textTracks[i]
      if (track.mode === 'showing' && track.cues) {
        for (let j = 0; j < track.cues.length; j++) {
          const cue = track.cues[j] as VTTCue
          cues.push({ startTime: cue.startTime, endTime: cue.endTime, text: cue.text.replace(/<[^>]*>/g, '') })
        }
      }
    }
    return cues
  }, [videoRef])

  const doSearch = useCallback((searchKeyword: string) => {
    const trimmed = searchKeyword.trim()
    if (!trimmed) {
      setResults([])
      setSearched(false)
      return
    }
    const matched = getCues().filter(cue => cue.text.toLowerCase().includes(trimmed.toLowerCase()))
    setResults(matched)
    setSearched(true)
    setActiveIndex(-1)
    const nextHistory = [trimmed, ...history.filter(item => item !== trimmed)].slice(0, MAX_HISTORY)
    setHistory(nextHistory)
    saveSearchHistory(nextHistory)
    setShowHistory(false)
  }, [getCues, history])

  const jumpTo = useCallback((cue: SubtitleCue) => {
    const video = videoRef.current
    if (!video) return
    video.currentTime = cue.startTime
    if (video.paused) video.play().catch(() => {})
  }, [videoRef])

  const handleKeyDown = useCallback((event: React.KeyboardEvent) => {
    if (event.key === 'Enter') {
      event.preventDefault()
      if (activeIndex >= 0 && activeIndex < results.length) jumpTo(results[activeIndex])
      else doSearch(keyword)
      return
    }
    if (event.key === 'ArrowDown') {
      event.preventDefault()
      if (results.length > 0) {
        const next = activeIndex < results.length - 1 ? activeIndex + 1 : 0
        setActiveIndex(next)
        ;(resultsRef.current?.children[next] as HTMLElement | undefined)?.scrollIntoView({ block: 'nearest' })
      }
      return
    }
    if (event.key === 'ArrowUp') {
      event.preventDefault()
      if (results.length > 0) {
        const prev = activeIndex > 0 ? activeIndex - 1 : results.length - 1
        setActiveIndex(prev)
        ;(resultsRef.current?.children[prev] as HTMLElement | undefined)?.scrollIntoView({ block: 'nearest' })
      }
      return
    }
    if (event.key === 'Escape') {
      event.preventDefault()
      onClose()
    }
  }, [activeIndex, doSearch, jumpTo, keyword, onClose, results])

  const removeHistory = (item: string) => {
    const nextHistory = history.filter(historyItem => historyItem !== item)
    setHistory(nextHistory)
    saveSearchHistory(nextHistory)
  }

  const clearHistory = () => {
    setHistory([])
    saveSearchHistory([])
  }

  if (!hasActiveSubtitle) {
    return (
      <div className="player-overlay-panel absolute bottom-full right-0 mb-3 w-[360px] max-w-[calc(100vw-24px)]" onClick={(event) => event.stopPropagation()}>
        <div className="player-overlay-panel-header">
          <div className="player-overlay-panel-heading">
            <div className="player-overlay-panel-title"><Search size={16} aria-hidden="true" /><span>字幕搜索</span></div>
            <div className="player-overlay-panel-subtitle">在当前字幕轨道中快速定位对白</div>
          </div>
          <button type="button" onClick={onClose} className="player-overlay-close" title="关闭" aria-label="关闭字幕搜索"><X size={16} aria-hidden="true" /></button>
        </div>
        <div className="player-overlay-body">
          <div className="player-overlay-empty">
            <div className="player-overlay-empty-inner">
              <div className="player-overlay-empty-icon"><Subtitles size={24} aria-hidden="true" /></div>
              <div className="player-overlay-empty-title">请先选择一个字幕轨道</div>
              <div className="player-overlay-empty-desc">加载字幕后即可搜索文本并跳转到对应时间点</div>
            </div>
          </div>
        </div>
      </div>
    )
  }

  return (
    <div className="player-overlay-panel absolute bottom-full right-0 mb-3 w-[440px] max-w-[calc(100vw-24px)]" onClick={(event) => event.stopPropagation()}>
      <div className="player-overlay-panel-header">
        <div className="player-overlay-panel-heading">
          <div className="player-overlay-panel-title"><Search size={16} aria-hidden="true" /><span>字幕搜索</span></div>
          <div className="player-overlay-panel-subtitle">搜索当前字幕内容并快速跳转</div>
        </div>
        <div className="player-overlay-inline-actions">
          <span className="player-overlay-chip"><kbd>↑↓</kbd> 导航</span>
          <span className="player-overlay-chip"><kbd>↵</kbd> 跳转</span>
          <button type="button" onClick={onClose} className="player-overlay-close" title="关闭" aria-label="关闭字幕搜索"><X size={16} aria-hidden="true" /></button>
        </div>
      </div>

      <div className="player-overlay-body">
        <div className="player-overlay-input-wrap">
          <Search size={15} aria-hidden="true" />
          <input
            ref={inputRef}
            type="text"
            value={keyword}
            onChange={(event) => {
              setKeyword(event.target.value)
              if (!event.target.value.trim()) {
                setResults([])
                setSearched(false)
              }
            }}
            onKeyDown={handleKeyDown}
            onFocus={() => { if (!keyword && history.length > 0) setShowHistory(true) }}
            placeholder="输入关键词搜索字幕内容..."
          />
          {keyword && (
            <button type="button" onClick={() => { setKeyword(''); setResults([]); setSearched(false); setShowHistory(history.length > 0); inputRef.current?.focus() }} className="player-overlay-input-clear" title="清空" aria-label="清空关键词"><X size={13} aria-hidden="true" /></button>
          )}
          <button type="button" onClick={() => doSearch(keyword)} disabled={!keyword.trim()} className="player-overlay-input-action">搜索</button>
        </div>

        {showHistory && !searched && history.length > 0 && (
          <div className="mt-4">
            <div className="player-overlay-section-label">
              <span className="inline-flex items-center gap-1.5"><Clock size={11} aria-hidden="true" />搜索历史</span>
              <button type="button" onClick={clearHistory} className="normal-case tracking-normal text-[var(--nv-player-text-faint)] transition-colors hover:text-[var(--nv-player-danger)]">清空</button>
            </div>
            <div className="player-overlay-list player-overlay-scroll max-h-[220px] overflow-y-auto pr-1">
              {history.map((item) => (
                <div key={item} className="group flex items-center gap-1">
                  <button type="button" onClick={() => { setKeyword(item); doSearch(item) }} className="player-overlay-item flex-1">
                    <div className="player-overlay-item-primary"><Clock size={13} className="shrink-0 text-[var(--nv-player-text-faint)]" aria-hidden="true" /><div className="player-overlay-item-title">{item}</div></div>
                  </button>
                  <button type="button" onClick={() => removeHistory(item)} className="player-overlay-close opacity-0 transition-opacity group-hover:opacity-100" title="删除记录" aria-label={`删除搜索记录 ${item}`}><X size={13} aria-hidden="true" /></button>
                </div>
              ))}
            </div>
          </div>
        )}

        {searched && (
          <div className="mt-4">
            <div className="player-overlay-section-label">
              <span>{results.length > 0 ? `搜索结果 · ${results.length}` : '搜索结果'}</span>
              {results.length > 0 && activeIndex >= 0 && <span className="normal-case tracking-normal">{activeIndex + 1} / {results.length}</span>}
            </div>
            {results.length > 0 ? (
              <div ref={resultsRef} className="player-overlay-list player-overlay-scroll max-h-[300px] overflow-y-auto pr-1">
                {results.map((cue, index) => (
                  <button
                    key={`${cue.startTime}-${index}`}
                    type="button"
                    onClick={() => { setActiveIndex(index); jumpTo(cue) }}
                    className={clsx('player-overlay-item group/item', activeIndex === index && 'is-active')}
                  >
                    <div className="min-w-0 flex-1">
                      <div className="mb-1.5 flex items-center gap-2">
                        <span className="font-mono text-[10px] tabular-nums text-[var(--nv-player-text-tertiary)]">{formatTime(cue.startTime)}</span>
                        <span className="text-[var(--nv-player-text-faint)]">—</span>
                        <span className="font-mono text-[10px] tabular-nums text-[var(--nv-player-text-faint)]">{formatTime(cue.endTime)}</span>
                      </div>
                      <div className={clsx('text-[12px] leading-relaxed', activeIndex === index ? 'text-[var(--nv-player-text-primary)]' : 'text-[var(--nv-player-text-secondary)]')}>{highlightText(cue.text, keyword)}</div>
                    </div>
                    <ChevronRight
                      size={13}
                      className={clsx(
                        'shrink-0 transition-[opacity,transform,color]',
                        activeIndex === index
                          ? 'translate-x-0 text-[var(--nv-player-accent)]'
                          : '-translate-x-1 text-[var(--nv-player-text-faint)] opacity-0 group-hover/item:translate-x-0 group-hover/item:opacity-100',
                      )}
                      aria-hidden="true"
                    />
                  </button>
                ))}
              </div>
            ) : (
              <div className="player-overlay-empty min-h-[150px]">
                <div className="player-overlay-empty-inner">
                  <div className="player-overlay-empty-icon"><Search size={22} aria-hidden="true" /></div>
                  <div className="player-overlay-empty-title">未找到「{keyword}」</div>
                  <div className="player-overlay-empty-desc">请尝试更短的关键词或其他表达</div>
                </div>
              </div>
            )}
          </div>
        )}

        {!searched && !showHistory && <div className="player-overlay-helper">输入关键词搜索当前字幕内容<br />点击结果即可跳转到对应时间点</div>}
      </div>
    </div>
  )
}
