import { useState, useEffect, useCallback } from 'react'
import { motion } from 'framer-motion'
import {
  MobilePageHeader,
  MobileSearchBar,
  MediaPosterCard,
} from '@/components/mobile'
import { mobileTokens } from '@/styles/mobile-tokens'
import { mediaApi } from '@/api'
import { useNavigate } from 'react-router-dom'

interface MobileServerSearchPageProps {
  onBack: () => void
}

/**
 * 移动端服务器搜索页
 */
export default function MobileServerSearchPage({ onBack }: MobileServerSearchPageProps) {
  const navigate = useNavigate()
  const [keyword, setKeyword] = useState('')
  const [results, setResults] = useState<any[]>([])
  const [loading, setLoading] = useState(false)
  const [searchHistory, setSearchHistory] = useState<string[]>([])

  // 加载搜索历史
  useEffect(() => {
    const history = localStorage.getItem('search_history')
    if (history) {
      try {
        setSearchHistory(JSON.parse(history))
      } catch (e) {
        // 忽略解析错误
      }
    }
  }, [])

  // 保存搜索历史
  const saveHistory = (term: string) => {
    if (!term.trim()) return
    const newHistory = [term, ...searchHistory.filter(h => h !== term)].slice(0, 20)
    setSearchHistory(newHistory)
    localStorage.setItem('search_history', JSON.stringify(newHistory))
  }

  // 搜索
  const handleSearch = useCallback((term: string) => {
    if (!term.trim()) {
      setResults([])
      return
    }

    setLoading(true)
    saveHistory(term)

    mediaApi.search(term, 1, 20)
      .then(res => {
        if (res?.data) {
          setResults(Array.isArray(res.data) ? res.data : [])
        }
      })
      .catch(() => {
        setResults([])
      })
      .finally(() => {
        setLoading(false)
      })
  }, [searchHistory])

  // 防抖搜索
  useEffect(() => {
    const timer = setTimeout(() => {
      if (keyword.trim()) {
        handleSearch(keyword)
      } else {
        setResults([])
      }
    }, 300)

    return () => clearTimeout(timer)
  }, [keyword, handleSearch])

  // 清除搜索历史
  const clearHistory = () => {
    setSearchHistory([])
    localStorage.removeItem('search_history')
  }

  return (
    <>
      {/* 页面标题 */}
      <MobilePageHeader
        title="搜索"
        onBack={onBack}
      />

      {/* 搜索框 */}
      <MobileSearchBar
        value={keyword}
        onChange={setKeyword}
        onSubmit={handleSearch}
        placeholder="输入搜索内容"
        autoFocus
      />

      {/* 搜索建议 / 历史 */}
      {!keyword && (
        <div className="px-8 mt-6">
          {/* 搜索历史 */}
          {searchHistory.length > 0 && (
            <div>
              <div className="flex items-center justify-between mb-3">
                <h3
                  className="font-semibold"
                  style={{
                    fontSize: mobileTokens.fontSize.lg,
                    color: mobileTokens.text,
                  }}
                >
                  搜索历史
                </h3>
                <button
                  onClick={clearHistory}
                  style={{
                    fontSize: mobileTokens.fontSize.sm,
                    color: mobileTokens.textMuted,
                  }}
                >
                  清除
                </button>
              </div>
              <div className="flex flex-wrap gap-2">
                {searchHistory.map((item, index) => (
                  <motion.button
                    key={index}
                    whileTap={{ scale: 0.95 }}
                    onClick={() => setKeyword(item)}
                    className="px-4 py-2"
                    style={{
                      borderRadius: mobileTokens.radius.full,
                      background: mobileTokens.card,
                      border: `1px solid ${mobileTokens.cardBorder}`,
                      fontSize: mobileTokens.fontSize.sm,
                      color: mobileTokens.text,
                    }}
                  >
                    {item}
                  </motion.button>
                ))}
              </div>
            </div>
          )}

          {/* 空状态 */}
          {searchHistory.length === 0 && (
            <div
              className="flex flex-col items-center justify-center py-12"
              style={{ color: mobileTokens.textMuted }}
            >
              <svg
                width="48"
                height="48"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.5"
                style={{ opacity: 0.5, marginBottom: '16px' }}
              >
                <circle cx="11" cy="11" r="8" />
                <path d="m21 21-4.35-4.35" />
              </svg>
              <p className="text-center" style={{ fontSize: mobileTokens.fontSize.md }}>
                输入关键词搜索媒体
              </p>
            </div>
          )}
        </div>
      )}

      {/* 搜索结果 */}
      {keyword && (
        <div className="px-8 mt-6">
          {loading ? (
            <div className="py-12 text-center" style={{ color: mobileTokens.textMuted }}>
              搜索中...
            </div>
          ) : results.length > 0 ? (
            <div
              className="grid gap-4"
              style={{
                gridTemplateColumns: 'repeat(2, 1fr)',
              }}
            >
              {results.map((item: any) => (
                <MediaPosterCard
                  key={item.id}
                  title={item.title || '未知'}
                  year={item.year}
                  imageUrl={item.poster_path ? `/api/media/${item.id}/poster` : undefined}
                  onClick={() => {
                    navigate(`/media/${item.id}`)
                  }}
                />
              ))}
            </div>
          ) : (
            <div
              className="flex flex-col items-center justify-center py-12"
              style={{ color: mobileTokens.textMuted }}
            >
              <p className="text-center font-medium" style={{ fontSize: mobileTokens.fontSize.lg }}>
                没有找到相关内容
              </p>
              <p className="text-center mt-2" style={{ fontSize: mobileTokens.fontSize.sm }}>
                换个关键词试试
              </p>
            </div>
          )}
        </div>
      )}
    </>
  )
}
