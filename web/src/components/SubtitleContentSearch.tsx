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
  /** 是否有已加载的字幕 */
  hasActiveSubtitle: boolean
}

const HISTORY_KEY = 'subtitle-search-history'
const MAX_HISTORY = 10

/** 格式化秒数为 HH:MM:SS 或 MM:SS */
function formatTime(seconds: number): string {
  const h = Math.floor(seconds / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = Math.floor(seconds % 60)
  if (h > 0) {
    return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
  }
  return `${m}:${String(s).padStart(2, '0')}`
}

/** 高亮关键词 */
function highlightText(text: string, keyword: string): React.ReactNode {
  if (!keyword) return text
  const escaped = keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')
  const parts = text.split(new RegExp(`(${escaped})`, 'gi'))
  return parts.map((part, i) =>
    part.toLowerCase() === keyword.toLowerCase() ? (
      <span key={i} className="text-neon-blue font-semibold bg-neon-blue/10 rounded px-0.5">{part}</span>
    ) : (
      <span key={i}>{part}</span>
    )
  )
}

/** 从 localStorage 读取搜索历史 */
function getSearchHistory(): string[] {
  try {
    const raw = localStorage.getItem(HISTORY_KEY)
    return raw ? JSON.parse(raw) : []
  } catch {
    return []
  }
}

/** 保存搜索历史到 localStorage */
function saveSearchHistory(history: string[]) {
  try {
    localStorage.setItem(HISTORY_KEY, JSON.stringify(history.slice(0, MAX_HISTORY)))
  } catch {}
}

export default function SubtitleContentSearch({
  videoRef,
  onClose,
  hasActiveSubtitle,
}: SubtitleContentSearchProps) {
  const [keyword, setKeyword] = useState('')
  const [results, setResults] = useState<SubtitleCue[]>([])
  const [searched, setSearched] = useState(false)
  const [history, setHistory] = useState<string[]>(getSearchHistory)
  const [showHistory, setShowHistory] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const inputRef = useRef<HTMLInputElement>(null)
  const resultsRef = useRef<HTMLDivElement>(null)

  // 自动聚焦输入框
  useEffect(() => {
    setTimeout(() => inputRef.current?.focus(), 100)
  }, [])

  /** 从 video 元素获取当前字幕的所有 cues */
  const getCues = useCallback((): SubtitleCue[] => {
    const video = videoRef.current
    if (!video) return []

    const cues: SubtitleCue[] = []
    for (let i = 0; i < video.textTracks.length; i++) {
      const track = video.textTracks[i]
      if (track.mode === 'showing' && track.cues) {
        for (let j = 0; j < track.cues.length; j++) {
          const cue = track.cues[j] as VTTCue
          cues.push({
            startTime: cue.startTime,
            endTime: cue.endTime,
            text: cue.text.replace(/<[^>]*>/g, ''), // 去除 HTML 标签
          })
        }
      }
    }
    return cues
  }, [videoRef])

  /** 执行搜索 */
  const doSearch = useCallback((searchKeyword: string) => {
    const trimmed = searchKeyword.trim()
    if (!trimmed) {
      setResults([])
      setSearched(false)
      return
    }

    const cues = getCues()
    const matched = cues.filter(cue =>
      cue.text.toLowerCase().includes(trimmed.toLowerCase())
    )
    setResults(matched)
    setSearched(true)
    setActiveIndex(-1)

    // 保存到搜索历史
    const newHistory = [trimmed, ...history.filter(h => h !== trimmed)].slice(0, MAX_HISTORY)
    setHistory(newHistory)
    saveSearchHistory(newHistory)
    setShowHistory(false)
  }, [getCues, history])

  /** 跳转到指定时间点 */
  const jumpTo = useCallback((cue: SubtitleCue) => {
    const video = videoRef.current
    if (!video) return
    video.currentTime = cue.startTime
    if (video.paused) {
      video.play().catch(() => {})
    }
  }, [videoRef])

  /** 键盘导航 */
  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      e.preventDefault()
      if (activeIndex >= 0 && activeIndex < results.length) {
        jumpTo(results[activeIndex])
      } else {
        doSearch(keyword)
      }
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (results.length > 0) {
        const next = activeIndex < results.length - 1 ? activeIndex + 1 : 0
        setActiveIndex(next)
        // 滚动到可见
        const el = resultsRef.current?.children[next] as HTMLElement
        el?.scrollIntoView({ block: 'nearest' })
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      if (results.length > 0) {
        const prev = activeIndex > 0 ? activeIndex - 1 : results.length - 1
        setActiveIndex(prev)
        const el = resultsRef.current?.children[prev] as HTMLElement
        el?.scrollIntoView({ block: 'nearest' })
      }
    } else if (e.key === 'Escape') {
      e.preventDefault()
      onClose()
    }
  }, [keyword, results, activeIndex, doSearch, jumpTo, onClose])

  /** 删除单条历史 */
  const removeHistory = (item: string) => {
    const newHistory = history.filter(h => h !== item)
    setHistory(newHistory)
    saveSearchHistory(newHistory)
  }

  /** 清空全部历史 */
  const clearHistory = () => {
    setHistory([])
    saveSearchHistory([])
  }

  // 无字幕时的提示
  if (!hasActiveSubtitle) {
    return (
      <div
        className="absolute bottom-full right-0 mb-2 w-[340px] rounded-xl p-4 shadow-2xl"
        style={{
          background: 'rgba(11, 17, 32, 0.95)',
          border: '1px solid rgba(0, 240, 255, 0.1)',
          backdropFilter: 'blur(20px)',
        }}
      >
        <div className="flex items-center justify-between mb-3">
          <div className="flex items-center gap-2 text-sm font-medium" style={{ color: '#ffffff' }}>
            <Search size={14} className="text-neon-blue" />
            字幕搜索
          </div>
          <button onClick={onClose} className="hover:text-white transition-colors" style={{ color: '#627d98' }}>
            <X size={14} />
          </button>
        </div>
        <div className="text-center py-6">
          <Subtitles className="h-8 w-8 mx-auto mb-2" style={{ color: '#486581' }} />
          <p className="text-sm" style={{ color: '#829ab1' }}>请先选择一个字幕轨道</p>
          <p className="text-xs mt-1" style={{ color: '#486581' }}>加载字幕后即可搜索字幕内容</p>
        </div>
      </div>
    )
  }

  return (
    <div
      className="absolute bottom-full right-0 mb-2 w-[380px] rounded-xl shadow-2xl"
      style={{
        background: 'rgba(11, 17, 32, 0.95)',
        border: '1px solid rgba(0, 240, 255, 0.1)',
        backdropFilter: 'blur(20px)',
      }}
      onClick={(e) => e.stopPropagation()}
    >
      {/* 标题栏 */}
      <div className="flex items-center justify-between px-4 pt-3 pb-2">
        <div className="flex items-center gap-2 text-sm font-medium" style={{ color: '#ffffff' }}>
          <Search size={14} className="text-neon-blue" />
          字幕搜索
        </div>
        <div className="flex items-center gap-1">
          <span className="text-[10px] mr-1" style={{ color: '#486581' }}>
            <kbd className="px-1 py-0.5 rounded text-[10px]" style={{ background: 'rgba(18, 26, 39, 0.8)', color: '#829ab1' }}>↑↓</kbd> 导航
            <kbd className="ml-1 px-1 py-0.5 rounded text-[10px]" style={{ background: 'rgba(18, 26, 39, 0.8)', color: '#829ab1' }}>↵</kbd> 跳转
          </span>
          <button onClick={onClose} className="hover:text-white transition-colors p-1" style={{ color: '#627d98' }}>
            <X size={14} />
          </button>
        </div>
      </div>

      {/* 搜索输入框 */}
      <div className="px-4 pb-2">
        <div className="relative">
          <input
            ref={inputRef}
            type="text"
            value={keyword}
            onChange={(e) => {
              setKeyword(e.target.value)
              if (!e.target.value.trim()) {
                setResults([])
                setSearched(false)
              }
            }}
            onKeyDown={handleKeyDown}
            onFocus={() => { if (!keyword && history.length > 0) setShowHistory(true) }}
            placeholder="输入关键词搜索字幕内容..."
            className="w-full rounded-lg px-3 py-2 pl-9 pr-20 text-sm outline-none ring-1 ring-transparent focus:ring-neon-blue/30 transition-all"
            style={{ background: 'rgba(18, 26, 39, 0.8)', color: '#ffffff' }}
          />
          <Search size={14} className="absolute left-3 top-1/2 -translate-y-1/2" style={{ color: '#627d98' }} />
          <div className="absolute right-2 top-1/2 -translate-y-1/2 flex items-center gap-1">
            {keyword && (
              <button
                onClick={() => { setKeyword(''); setResults([]); setSearched(false); inputRef.current?.focus() }}
                className="hover:text-white p-0.5 transition-colors"
                style={{ color: '#627d98' }}
              >
                <X size={12} />
              </button>
            )}
            <button
              onClick={() => doSearch(keyword)}
              disabled={!keyword.trim()}
              className={clsx(
                'rounded-md px-2.5 py-1 text-xs font-medium transition-all',
                keyword.trim()
                  ? 'bg-neon-blue/20 text-neon-blue hover:bg-neon-blue/30'
                  : 'cursor-not-allowed'
              )}
            style={!keyword.trim() ? { background: 'rgba(18, 26, 39, 0.5)', color: '#486581' } : {}}
            >
              搜索
            </button>
          </div>
        </div>
      </div>

      {/* 搜索历史 */}
      {showHistory && !searched && history.length > 0 && (
        <div className="px-4 pb-2">
          <div className="flex items-center justify-between mb-1.5">
            <span className="text-[10px] font-bold uppercase tracking-widest" style={{ color: '#627d98' }}>
              <Clock size={10} className="inline mr-1" />搜索历史
            </span>
            <button onClick={clearHistory} className="text-[10px] hover:text-red-400 transition-colors" style={{ color: '#486581' }}>
              清空
            </button>
          </div>
          <div className="space-y-0.5">
            {history.map((item) => (
              <div key={item} className="flex items-center group">
                <button
                  onClick={() => { setKeyword(item); doSearch(item) }}
                  className="flex-1 text-left text-xs hover:text-white px-2 py-1.5 rounded-md transition-colors truncate"
                  style={{ color: '#829ab1' }}
                  onMouseEnter={e => (e.currentTarget.style.background = 'rgba(18, 26, 39, 0.6)')}
                  onMouseLeave={e => (e.currentTarget.style.background = 'transparent')}
                >
                  <Clock size={10} className="inline mr-1.5" style={{ color: '#486581' }} />
                  {item}
                </button>
                <button
                  onClick={() => removeHistory(item)}
                  className="opacity-0 group-hover:opacity-100 hover:text-red-400 p-1 transition-all"
                  style={{ color: '#486581' }}
                >
                  <X size={10} />
                </button>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* 搜索结果 */}
      {searched && (
        <div className="border-t border-neon-blue/5">
          <div className="px-4 py-2 flex items-center justify-between">
            <span className="text-xs" style={{ color: '#627d98' }}>
              {results.length > 0 ? (
                <>找到 <span className="text-neon-blue font-medium">{results.length}</span> 条结果</>
              ) : (
                '未找到匹配结果'
              )}
            </span>
            {results.length > 0 && activeIndex >= 0 && (
              <span className="text-[10px]" style={{ color: '#486581' }}>
                {activeIndex + 1} / {results.length}
              </span>
            )}
          </div>

          <div
            ref={resultsRef}
            className="max-h-[280px] overflow-y-auto px-2 pb-2 space-y-0.5"
            style={{ scrollbarWidth: 'thin', scrollbarColor: 'rgba(55, 65, 81, 0.5) transparent' }}
          >
            {results.map((cue, index) => (
              <button
                key={`${cue.startTime}-${index}`}
                onClick={() => { setActiveIndex(index); jumpTo(cue) }}
                className={clsx(
                  'w-full text-left rounded-lg px-3 py-2.5 transition-all group/item',
                    activeIndex === index
                    ? 'bg-neon-blue/10 ring-1 ring-neon-blue/20'
                    : ''
                )}
              >
                <div className="flex items-center gap-2 mb-0.5">
                  <span className={clsx(
                    'text-[11px] font-mono tabular-nums',
                    activeIndex === index ? 'text-neon-blue' : ''
                  )} style={activeIndex !== index ? { color: '#627d98' } : {}}>
                    {formatTime(cue.startTime)}
                  </span>
                  <span style={{ color: '#334155' }}>—</span>
                  <span className="text-[11px] font-mono tabular-nums" style={{ color: '#486581' }}>
                    {formatTime(cue.endTime)}
                  </span>
                  <ChevronRight size={10} className={clsx(
                    'ml-auto transition-transform',
                    activeIndex === index ? 'text-neon-blue translate-x-0' : '-translate-x-1 opacity-0 group-hover/item:opacity-100 group-hover/item:translate-x-0'
                  )} style={activeIndex !== index ? { color: '#334155' } : {}} />
                </div>
                <p className={clsx(
                  'text-sm leading-relaxed',
                  activeIndex === index ? '' : ''
                )} style={{ color: activeIndex === index ? '#ffffff' : '#bcccdc' }}>
                  {highlightText(cue.text, keyword)}
                </p>
              </button>
            ))}
          </div>

          {results.length === 0 && (
            <div className="text-center py-6 px-4">
              <Search className="h-6 w-6 mx-auto mb-2" style={{ color: '#334155' }} />
              <p className="text-xs" style={{ color: '#627d98' }}>未在当前字幕中找到「{keyword}」</p>
              <p className="text-[10px] mt-1" style={{ color: '#486581' }}>请尝试其他关键词</p>
            </div>
          )}
        </div>
      )}

      {/* 空状态 */}
      {!searched && !showHistory && (
        <div className="text-center py-4 px-4 border-t border-neon-blue/5">
          <p className="text-xs" style={{ color: '#627d98' }}>输入关键词搜索当前字幕内容</p>
          <p className="text-[10px] mt-0.5" style={{ color: '#486581' }}>点击结果可跳转到对应时间点</p>
        </div>
      )}
    </div>
  )
}
