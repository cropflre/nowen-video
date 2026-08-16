import { useCallback, useEffect, useRef, useState } from 'react'
import { subtitleApi, subtitleSearchApi } from '@/api'
import type { SubtitleDownloadResult, SubtitleSearchResult } from '@/types'
import { Search, Download, Loader2, Subtitles, Globe, X, Database, Languages } from 'lucide-react'
import clsx from 'clsx'

interface SubtitleSearchPanelProps {
  mediaId: string
  title?: string
  year?: number
  type?: string
  onClose: () => void
  onDownloaded?: (subtitle?: SubtitleDownloadResult) => void
}

type OnlineSubtitleResult = SubtitleSearchResult & {
  file_size?: string
  match_score?: number
  source_url?: string
  available_languages?: string[]
}

const languageName = (code: string) => {
  switch (code.toLowerCase()) {
    case 'zh-cn': return '简中'
    case 'zh-tw': return '繁中'
    case 'en': return 'English'
    case 'ja': return '日本語'
    case 'ko': return '한국어'
    default: return code
  }
}

const providerName = (source: string) => {
  if (source === 'subtitlecat') return 'SubtitleCat'
  if (source === 'opensubtitles') return 'OpenSubtitles'
  return source || 'Online'
}

// 媒体标题经常包含长描述（例如「CAND-181 女体育大学生 ...」）。
// 在线字幕搜索优先提取片号/番号作为可编辑初始值，避免把整段展示标题传给 Provider。
const initialSearchTitle = (title?: string) => {
  const value = title?.trim() || ''
  if (!value) return ''

  const catalogCode = value.match(/(?:^|[\s[(])([A-Za-z]{2,12})[-_\s]?(\d{2,6})(?=$|[\s)\]._\-])/)
  if (catalogCode) {
    return `${catalogCode[1].toUpperCase()}-${catalogCode[2]}`
  }

  return value
}

async function activateDownloadedSubtitle(result: SubtitleDownloadResult) {
  const video = document.querySelector<HTMLVideoElement>('.group\\/player video') || document.querySelector<HTMLVideoElement>('video')
  if (!video || !result.file_path) return

  const subtitleURL = subtitleApi.getExternalUrl(result.file_path)
  const response = await fetch(subtitleURL)
  if (!response.ok) throw new Error(`字幕加载失败: HTTP ${response.status}`)
  const vttText = await response.text()
  if (!vttText.trim()) throw new Error('字幕加载失败: 空字幕')

  video.querySelectorAll('track').forEach(track => track.remove())
  for (let i = 0; i < video.textTracks.length; i += 1) {
    video.textTracks[i].mode = 'disabled'
  }

  const blobURL = URL.createObjectURL(new Blob([vttText], { type: 'text/vtt' }))
  const track = document.createElement('track')
  track.kind = 'subtitles'
  track.label = result.language ? languageName(result.language) : '在线字幕'
  track.srclang = result.language || 'und'
  track.src = blobURL

  await new Promise<void>((resolve, reject) => {
    let settled = false
    const cleanup = () => {
      window.clearTimeout(timer)
      track.removeEventListener('load', onTrackLoad)
      track.removeEventListener('error', onTrackError)
    }
    const finishWithError = (error: Error) => {
      if (settled) return
      settled = true
      cleanup()
      if (track.parentNode) track.parentNode.removeChild(track)
      URL.revokeObjectURL(blobURL)
      reject(error)
    }
    const onTrackLoad = () => {
      if (settled) return
      settled = true
      cleanup()
      for (let i = 0; i < video.textTracks.length; i += 1) {
        const textTrack = video.textTracks[i]
        textTrack.mode = textTrack === track.track ? 'showing' : 'disabled'
      }
      URL.revokeObjectURL(blobURL)
      resolve()
    }
    const onTrackError = () => finishWithError(new Error('浏览器无法解析字幕轨道'))
    const timer = window.setTimeout(() => finishWithError(new Error('字幕轨道加载超时')), 8000)

    track.addEventListener('load', onTrackLoad)
    track.addEventListener('error', onTrackError)
    video.appendChild(track)

    // HTMLTrackElement 新建时默认 mode=disabled。先切 showing 触发 WebVTT 加载，
    // load 事件只负责确认解析完成与资源清理。
    track.track.mode = 'showing'
  })
}

export default function SubtitleSearchPanel({
  mediaId, title, year, type, onClose, onDownloaded,
}: SubtitleSearchPanelProps) {
  const [searchTitle, setSearchTitle] = useState(() => initialSearchTitle(title))
  const [language, setLanguage] = useState('zh-CN,zh-TW,en')
  const [results, setResults] = useState<OnlineSubtitleResult[]>([])
  const [searching, setSearching] = useState(false)
  const [downloading, setDownloading] = useState<string | null>(null)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)
  const didAutoSearch = useRef(false)
  const inputRef = useRef<HTMLInputElement>(null)

  const handleSearch = useCallback(async () => {
    setSearching(true)
    setMessage(null)
    try {
      const keyword = searchTitle.trim()
      const synchronizedTitle = keyword || undefined
      const res = await subtitleSearchApi.search(mediaId, {
        language,
        title: synchronizedTitle,
        year,
        type,
        query: synchronizedTitle,
      })
      const data = (res.data.data || []) as OnlineSubtitleResult[]
      setResults(data)
      if (!data.length) {
        setMessage({
          type: 'error',
          text: keyword ? `未找到「${keyword}」的在线字幕` : '未找到与当前视频匹配的在线字幕',
        })
      }
    } catch (err: any) {
      setResults([])
      const errorText = err.response?.data?.error || err.message || '搜索失败'
      setMessage({ type: 'error', text: errorText.includes('暂时不可用') ? errorText : `字幕搜索失败：${errorText}` })
    } finally {
      setSearching(false)
    }
  }, [language, mediaId, searchTitle, type, year])

  useEffect(() => {
    if (didAutoSearch.current) return
    didAutoSearch.current = true
    void handleSearch()
  }, [handleSearch])

  useEffect(() => {
    const timer = window.setTimeout(() => inputRef.current?.focus(), 120)
    return () => window.clearTimeout(timer)
  }, [])

  const handleDownload = async (sub: OnlineSubtitleResult) => {
    setDownloading(sub.id)
    setMessage(null)
    try {
      const res = await subtitleSearchApi.download(mediaId, sub.id)
      const downloaded = res.data.data
      onDownloaded?.(downloaded)
      await activateDownloadedSubtitle(downloaded)
      setMessage({ type: 'success', text: `${sub.language_name || sub.language} 字幕已保存并加载` })
      window.setTimeout(onClose, 650)
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.error || err.message || '下载失败' })
    } finally {
      setDownloading(null)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center px-4 backdrop-blur-sm"
      style={{ background: 'color-mix(in srgb, var(--nv-player-canvas) 68%, transparent)' }}
    >
      <div
        className="flex max-h-[82vh] w-full max-w-3xl flex-col overflow-hidden rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface)] shadow-[var(--nv-player-shadow)] backdrop-blur-2xl"
        onClick={event => event.stopPropagation()}
      >
        <div className="flex items-center justify-between border-b border-[var(--nv-player-border-subtle)] px-6 py-5">
          <div className="flex min-w-0 items-center gap-3">
            <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] text-[var(--nv-player-accent)]">
              <Subtitles className="h-[18px] w-[18px]" aria-hidden="true" />
            </div>
            <div className="min-w-0">
              <h3 className="font-display text-base font-semibold tracking-tight text-[var(--nv-player-text-primary)]">在线字幕搜索</h3>
              <p className="mt-0.5 truncate text-[11px] text-[var(--nv-player-text-tertiary)]">
                SubtitleCat · {title || '当前视频'}{year ? ` · ${year}` : ''}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="flex h-8 w-8 items-center justify-center rounded-[var(--nv-player-radius-control)] text-[var(--nv-player-text-tertiary)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-primary)]"
            title="关闭"
            aria-label="关闭在线字幕搜索"
          >
            <X className="h-4 w-4" aria-hidden="true" />
          </button>
        </div>

        <div className="border-b border-[var(--nv-player-border-subtle)] px-6 py-4">
          <div className="mb-2 flex items-center justify-between gap-3">
            <span className="text-[10px] font-medium uppercase tracking-[0.12em] text-[var(--nv-player-text-faint)]">搜索标题</span>
            <span className="text-[10px] text-[var(--nv-player-text-faint)]">title / query 同步</span>
          </div>
          <div className="flex gap-2">
            <div className="relative flex min-w-0 flex-1 items-center">
              <Search className="pointer-events-none absolute left-3.5 h-4 w-4 text-[var(--nv-player-text-faint)]" aria-hidden="true" />
              <input
                ref={inputRef}
                value={searchTitle}
                onChange={event => setSearchTitle(event.target.value)}
                onKeyDown={event => {
                  if (event.key === 'Enter' && !searching) {
                    event.preventDefault()
                    void handleSearch()
                  }
                }}
                placeholder="输入片名 / 番号，例如 JUNY-146"
                className="h-11 w-full rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] pl-10 pr-9 text-sm text-[var(--nv-player-text-primary)] outline-none transition-[background-color,border-color,box-shadow] placeholder:text-[var(--nv-player-text-faint)] focus:border-[var(--nv-player-border-hover)] focus:bg-[var(--nv-player-surface)] focus:shadow-[0_0_0_3px_var(--nv-player-accent-soft)]"
              />
              {searchTitle && (
                <button
                  type="button"
                  onClick={() => {
                    setSearchTitle('')
                    inputRef.current?.focus()
                  }}
                  className="absolute right-3 flex h-6 w-6 items-center justify-center rounded-[var(--nv-radius-sm)] text-[var(--nv-player-text-faint)] transition-colors hover:bg-[var(--nv-player-surface-hover)] hover:text-[var(--nv-player-text-secondary)]"
                  title="清空"
                  aria-label="清空搜索标题"
                >
                  <X className="h-3.5 w-3.5" aria-hidden="true" />
                </button>
              )}
            </div>
            <button
              type="button"
              onClick={() => void handleSearch()}
              disabled={searching}
              className="flex h-11 shrink-0 items-center gap-2 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] px-4 text-xs font-medium text-[var(--nv-player-accent)] transition-[background-color,border-color] hover:bg-[var(--nv-player-accent-soft-hover)] hover:border-[var(--nv-player-border-hover)] disabled:cursor-not-allowed disabled:opacity-50"
            >
              {searching ? <Loader2 className="h-4 w-4 animate-spin" aria-hidden="true" /> : <Search className="h-4 w-4" aria-hidden="true" />}
              {searching ? '搜索中...' : '搜索'}
            </button>
          </div>
          <p className="mt-2 text-[10px] leading-relaxed text-[var(--nv-player-text-faint)]">
            可直接修改搜索标题。请求时会把当前输入值同时传给 title 和 query；清空后则回退到当前视频文件名自动匹配。
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3 border-b border-[var(--nv-player-border-subtle)] px-6 py-3">
          <select
            value={language}
            onChange={event => setLanguage(event.target.value)}
            className="h-9 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-border)] bg-[var(--nv-player-surface-soft)] px-3 text-xs text-[var(--nv-player-text-secondary)] outline-none transition-colors focus:border-[var(--nv-player-border-hover)]"
          >
            <option value="zh-CN,zh-TW,en">简中 + 繁中 + English</option>
            <option value="zh-CN">简体中文</option>
            <option value="zh-TW">繁体中文</option>
            <option value="en">English</option>
            <option value="ja">日本語</option>
            <option value="ko">한국어</option>
          </select>
          <div className="ml-auto flex items-center gap-1.5 rounded-[var(--nv-radius-pill)] border border-[var(--nv-player-border-subtle)] bg-[var(--nv-player-surface-subtle)] px-2.5 py-1 text-[10px] text-[var(--nv-player-text-tertiary)]">
            <Database className="h-3 w-3 text-[var(--nv-player-accent)]" aria-hidden="true" />
            SubtitleCat
          </div>
        </div>

        {message && (
          <div
            className={clsx('mx-6 mt-4 rounded-[var(--nv-player-radius-control)] border px-4 py-2.5 text-xs')}
            style={message.type === 'success'
              ? {
                  color: 'var(--nv-player-success)',
                  background: 'color-mix(in srgb, var(--nv-player-success) 8%, transparent)',
                  borderColor: 'color-mix(in srgb, var(--nv-player-success) 22%, transparent)',
                }
              : {
                  color: 'var(--nv-player-danger)',
                  background: 'var(--nv-player-danger-soft)',
                  borderColor: 'var(--nv-player-danger-border)',
                }}
          >
            {message.text}
          </div>
        )}

        <div className="flex-1 space-y-2 overflow-y-auto p-6 pt-4">
          {results.map(sub => {
            const available = sub.available_languages || []
            const visibleLanguages = available.slice(0, 3)
            return (
              <div
                key={`${sub.source}:${sub.id}`}
                className="flex items-center gap-4 rounded-[var(--nv-player-radius-panel)] border border-[var(--nv-player-border-subtle)] bg-[var(--nv-player-surface-subtle)] p-4 transition-[background-color,border-color] hover:border-[var(--nv-player-border)] hover:bg-[var(--nv-player-surface-hover)]"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <p className="truncate text-sm font-medium text-[var(--nv-player-text-primary)]">{sub.title || sub.file_name}</p>
                    <span className="shrink-0 rounded-[var(--nv-radius-pill)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] px-2 py-0.5 text-[9px] font-semibold text-[var(--nv-player-accent)]">
                      {providerName(sub.source)}
                    </span>
                  </div>
                  <p className="mt-1 truncate text-[11px] text-[var(--nv-player-text-faint)]">{sub.file_name}</p>
                  <div className="mt-2 flex flex-wrap items-center gap-x-3 gap-y-1.5 text-[10px] text-[var(--nv-player-text-tertiary)]">
                    <span className="inline-flex items-center gap-1 text-[var(--nv-player-text-secondary)]">
                      <Globe className="h-3 w-3 text-[var(--nv-player-accent)]" aria-hidden="true" />
                      {sub.language_name || languageName(sub.language)}
                    </span>
                    {sub.file_size && <span>{sub.file_size}</span>}
                    {typeof sub.match_score === 'number' && sub.match_score > 0 && <span>匹配度 {sub.match_score}%</span>}
                    {sub.download_count > 0 && <span>{sub.download_count} 下载</span>}
                    <span className="uppercase text-[var(--nv-player-text-faint)]">{sub.format}</span>
                    {available.length > 0 && (
                      <span className="inline-flex items-center gap-1">
                        <Languages className="h-3 w-3" aria-hidden="true" />
                        支持 {visibleLanguages.map(languageName).join(' / ')}
                        {available.length > visibleLanguages.length ? ` / +${available.length - visibleLanguages.length}` : ''}
                      </span>
                    )}
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => void handleDownload(sub)}
                  disabled={downloading === sub.id}
                  className="flex h-9 shrink-0 items-center gap-1.5 rounded-[var(--nv-player-radius-control)] border border-[var(--nv-player-accent-border)] bg-[var(--nv-player-accent-soft)] px-3 text-[11px] font-medium text-[var(--nv-player-text-secondary)] transition-[background-color,border-color,color] hover:bg-[var(--nv-player-accent-soft-hover)] hover:border-[var(--nv-player-border-hover)] hover:text-[var(--nv-player-text-primary)] disabled:cursor-not-allowed disabled:opacity-50"
                >
                  {downloading === sub.id ? <Loader2 className="h-3.5 w-3.5 animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" /> : <Download className="h-3.5 w-3.5 text-[var(--nv-player-accent)]" aria-hidden="true" />}
                  下载并使用
                </button>
              </div>
            )
          })}

          {searching && results.length === 0 && (
            <div className="flex min-h-[220px] flex-col items-center justify-center text-center">
              <Loader2 className="mb-3 h-7 w-7 animate-spin text-[var(--nv-player-accent)]" aria-hidden="true" />
              <p className="text-sm text-[var(--nv-player-text-secondary)]">正在请求在线字幕源</p>
              <p className="mt-1 text-[11px] text-[var(--nv-player-text-faint)]">优先查找简体中文、繁体中文和 English</p>
            </div>
          )}

          {!searching && results.length === 0 && !message && (
            <div className="flex min-h-[220px] flex-col items-center justify-center text-center text-[var(--nv-player-text-tertiary)]">
              <Subtitles className="mb-3 h-10 w-10 opacity-30" aria-hidden="true" />
              <p className="text-sm">没有可用的在线字幕</p>
              <p className="mt-1 text-[11px] text-[var(--nv-player-text-faint)]">可以输入更精确的片名或番号重新搜索</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
