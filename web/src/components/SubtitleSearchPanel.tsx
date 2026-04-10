import { useState } from 'react'
import { subtitleSearchApi } from '@/api'
import type { SubtitleSearchResult } from '@/types'
import { Search, Download, Loader2, Subtitles, Star, Globe, X } from 'lucide-react'
import clsx from 'clsx'

interface SubtitleSearchPanelProps {
  mediaId: string
  title?: string
  year?: number
  type?: string
  onClose: () => void
  onDownloaded?: () => void
}

export default function SubtitleSearchPanel({
  mediaId, title, year, type, onClose, onDownloaded,
}: SubtitleSearchPanelProps) {
  const [language, setLanguage] = useState('zh-cn,en')
  const [results, setResults] = useState<SubtitleSearchResult[]>([])
  const [searching, setSearching] = useState(false)
  const [downloading, setDownloading] = useState<string | null>(null)
  const [message, setMessage] = useState<{ type: 'success' | 'error'; text: string } | null>(null)

  const handleSearch = async () => {
    setSearching(true)
    setMessage(null)
    try {
      const res = await subtitleSearchApi.search(mediaId, { language, title, year, type })
      setResults(res.data.data || [])
      if (!res.data.data?.length) {
        setMessage({ type: 'error', text: '未找到匹配的字幕' })
      }
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.error || '搜索失败' })
    } finally {
      setSearching(false)
    }
  }

  const handleDownload = async (sub: SubtitleSearchResult) => {
    setDownloading(sub.id)
    try {
      await subtitleSearchApi.download(mediaId, sub.id)
      setMessage({ type: 'success', text: `字幕 "${sub.file_name}" 下载成功` })
      onDownloaded?.()
    } catch (err: any) {
      setMessage({ type: 'error', text: err.response?.data?.error || '下载失败' })
    } finally {
      setDownloading(null)
      setTimeout(() => setMessage(null), 3000)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm">
      <div className="w-full max-w-2xl max-h-[80vh] flex flex-col rounded-2xl" style={{
        background: 'rgba(11, 17, 32, 0.92)',
        border: '1px solid rgba(0, 240, 255, 0.15)',
        backdropFilter: 'blur(20px)',
        boxShadow: '0 8px 32px rgba(0, 0, 0, 0.5)',
      }}>
        {/* 标题 */}
        <div className="flex items-center justify-between p-6 pb-0">
          <div className="flex items-center gap-3">
            <Subtitles className="h-5 w-5 text-neon-blue" />
            <div>
              <h3 className="font-display text-lg font-semibold" style={{ color: '#ffffff' }}>在线字幕搜索</h3>
              {title && <p className="text-xs" style={{ color: '#829ab1' }}>{title} {year ? `(${year})` : ''}</p>}
            </div>
          </div>
          <button onClick={onClose} className="hover:text-white" style={{ color: '#829ab1' }}>
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* 搜索栏 */}
        <div className="flex items-center gap-3 p-6">
          <select value={language} onChange={e => setLanguage(e.target.value)}
            className="rounded-lg px-3 py-2 text-sm outline-none"
            style={{
              background: 'rgba(255, 255, 255, 0.06)',
              border: '1px solid rgba(0, 240, 255, 0.1)',
              color: '#e2e8f0',
            }}>
            <option value="zh-cn,en" style={{ background: '#0b1120', color: '#e2e8f0' }}>中文 + 英文</option>
            <option value="zh-cn" style={{ background: '#0b1120', color: '#e2e8f0' }}>简体中文</option>
            <option value="zh-tw" style={{ background: '#0b1120', color: '#e2e8f0' }}>繁体中文</option>
            <option value="en" style={{ background: '#0b1120', color: '#e2e8f0' }}>English</option>
            <option value="ja" style={{ background: '#0b1120', color: '#e2e8f0' }}>日本語</option>
            <option value="ko" style={{ background: '#0b1120', color: '#e2e8f0' }}>한국어</option>
          </select>
          <button onClick={handleSearch} disabled={searching}
            className="btn-neon rounded-lg px-5 py-2 text-sm font-medium flex items-center gap-2"
            style={{ color: '#ffffff' }}>
            {searching ? <Loader2 className="h-4 w-4 animate-spin" /> : <Search className="h-4 w-4" />}
            搜索字幕
          </button>
        </div>

        {/* 消息提示 */}
        {message && (
          <div className={clsx(
            'mx-6 rounded-xl px-4 py-2 text-sm font-medium',
            message.type === 'success' ? 'bg-green-500/10 text-green-400 border border-green-500/20' : 'bg-red-500/10 text-red-400 border border-red-500/20'
          )}>
            {message.text}
          </div>
        )}

        {/* 搜索结果 */}
        <div className="flex-1 overflow-y-auto p-6 pt-3 space-y-2">
          {results.map(sub => (
            <div key={sub.id} className="flex items-center gap-3 rounded-xl p-3 transition-colors"
              style={{
                background: 'rgba(18, 26, 39, 0.5)',
              }}
              onMouseEnter={e => (e.currentTarget.style.background = 'rgba(18, 26, 39, 0.8)')}
              onMouseLeave={e => (e.currentTarget.style.background = 'rgba(18, 26, 39, 0.5)')}
            >
              <div className="flex-1 min-w-0">
                <p className="text-sm truncate" style={{ color: '#ffffff' }}>{sub.file_name}</p>
                <div className="flex items-center gap-3 mt-1 text-xs" style={{ color: '#829ab1' }}>
                  <span className="flex items-center gap-1">
                    <Globe className="h-3 w-3" />
                    {sub.language_name || sub.language}
                  </span>
                  <span className="flex items-center gap-1">
                    <Star className="h-3 w-3" />
                    {sub.rating.toFixed(1)}
                  </span>
                  <span>下载 {sub.download_count}</span>
                  <span className={clsx(
                    'rounded-full px-2 py-0.5 text-[10px] font-medium',
                    sub.match_type === 'hash' ? 'bg-green-500/20 text-green-400' : 'bg-blue-500/20 text-blue-400'
                  )}>
                    {sub.match_type === 'hash' ? '精确匹配' : '标题匹配'}
                  </span>
                  <span className="uppercase" style={{ color: '#627d98' }}>{sub.format}</span>
                </div>
              </div>
              <button
                onClick={() => handleDownload(sub)}
                disabled={downloading === sub.id}
                className="shrink-0 rounded-lg px-3 py-1.5 text-xs flex items-center gap-1 transition-all duration-300"
                style={{
                  color: '#9fb3c8',
                  background: 'rgba(0, 240, 255, 0.05)',
                  border: '1px solid rgba(0, 240, 255, 0.1)',
                }}
                onMouseEnter={e => { e.currentTarget.style.color = '#ffffff'; e.currentTarget.style.background = 'rgba(0, 240, 255, 0.1)' }}
                onMouseLeave={e => { e.currentTarget.style.color = '#9fb3c8'; e.currentTarget.style.background = 'rgba(0, 240, 255, 0.05)' }}
              >
                {downloading === sub.id ? (
                  <Loader2 className="h-3 w-3 animate-spin" />
                ) : (
                  <Download className="h-3 w-3" />
                )}
                下载
              </button>
            </div>
          ))}

          {results.length === 0 && !searching && (
            <div className="text-center py-12" style={{ color: '#627d98' }}>
              <Subtitles className="h-12 w-12 mx-auto mb-3 opacity-30" />
              <p className="text-sm">点击「搜索字幕」开始搜索</p>
              <p className="text-xs mt-1" style={{ color: '#627d98' }}>数据源: OpenSubtitles</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
