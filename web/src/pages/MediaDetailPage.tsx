import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, userApi, streamApi, playlistApi, recommendApi, adminApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import type { Media, MediaPlayInfo, Playlist, RecommendedMedia, MediaPerson, WatchHistory } from '@/types'
import { HeroSection, MediaInfoSection, RecommendationCarousel, TrailerModal } from '@/components/media'
import CommentSection from '@/components/CommentSection'
import EditMetadataModal from '@/components/EditMetadataModal'

export default function MediaDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const toast = useToast()

  // 核心数据
  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)

  // 用户相关
  const [isFavorited, setIsFavorited] = useState(false)
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [watchProgress, setWatchProgress] = useState<WatchHistory | null>(null)

  // 附加数据
  const [recommendations, setRecommendations] = useState<RecommendedMedia[]>([])
  const [persons, setPersons] = useState<MediaPerson[]>([])

  // UI 状态
  const [scraping, setScraping] = useState(false)
  const [showTrailer, setShowTrailer] = useState(false)

  // 管理功能状态
  const [showMatchModal, setShowMatchModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showUnmatchConfirm, setShowUnmatchConfirm] = useState(false)
  const [matchQuery, setMatchQuery] = useState('')
  const [matchResults, setMatchResults] = useState<any[]>([])
  const [matchSearching, setMatchSearching] = useState(false)
  const [matchSource, setMatchSource] = useState<'tmdb' | 'bangumi'>('tmdb')
  const [editForm, setEditForm] = useState<{
    title: string; orig_title: string; year: number; overview: string;
    rating: number; genres: string; country: string; language: string;
    tagline: string; studio: string
  }>({ title: '', orig_title: '', year: 0, overview: '', rating: 0, genres: '', country: '', language: '', tagline: '', studio: '' })

  // ==================== 数据加载 ====================
  useEffect(() => {
    if (!id) return
    const abortController = new AbortController()
    setLoading(true)
    setPersons([])
    setWatchProgress(null)

    Promise.all([
      mediaApi.detail(id),
      streamApi.getPlayInfo(id),
      playlistApi.list(),
    ])
      .then(([mediaRes, playInfoRes, playlistRes]) => {
        if (abortController.signal.aborted) return
        const mediaData = mediaRes.data.data
        setMedia(mediaData)
        setPlayInfo(playInfoRes.data.data)
        setPlaylists(playlistRes.data.data || [])

        // 非首屏请求：收藏状态、相关推荐、演职人员、观看进度
        userApi.checkFavorite(mediaData.id)
          .then((res) => { if (!abortController.signal.aborted) setIsFavorited(res.data.data) })
          .catch(() => {})
        recommendApi.getSimilarMedia(mediaData.id, 12)
          .then((res) => { if (!abortController.signal.aborted) setRecommendations(res.data.data || []) })
          .catch(() => {})
        mediaApi.getPersons(mediaData.id)
          .then((res) => { if (!abortController.signal.aborted) setPersons(res.data.data || []) })
          .catch(() => {})
        userApi.getProgress(mediaData.id)
          .then((res) => { if (!abortController.signal.aborted) setWatchProgress(res.data.data) })
          .catch(() => {})
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        toast.error('加载媒体详情失败')
        navigate('/')
      })
      .finally(() => { if (!abortController.signal.aborted) setLoading(false) })

    return () => abortController.abort()
  }, [id, navigate])

  // ==================== 事件处理 ====================
  const handleFavorite = async () => {
    if (!id) return
    try {
      if (isFavorited) {
        await userApi.removeFavorite(id)
        setIsFavorited(false)
      } else {
        await userApi.addFavorite(id)
        setIsFavorited(true)
      }
    } catch {
      toast.error('收藏操作失败')
    }
  }

  const handleScrape = async () => {
    if (!id) return
    setScraping(true)
    try {
      await mediaApi.scrape(id)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      toast.success('元数据刮削成功')
    } catch {
      toast.error('元数据刮削失败，请检查TMDb API Key配置')
    } finally {
      setScraping(false)
    }
  }

  const handleAddToPlaylist = async (playlistId: string) => {
    if (!id) return
    try {
      await playlistApi.addItem(playlistId, id)
      toast.success('已添加到播放列表')
    } catch {
      toast.error('添加到播放列表失败')
    }
  }

  // ==================== 管理功能事件处理 ====================
  const handleManualMatch = () => {
    if (!media) return
    setMatchQuery(media.title)
    setMatchResults([])
    setMatchSource('tmdb')
    setShowMatchModal(true)
  }

  const handleMatchSearch = async () => {
    if (!matchQuery.trim()) return
    setMatchSearching(true)
    try {
      if (matchSource === 'tmdb') {
        const mediaType = media?.media_type === 'episode' ? 'tv' : 'movie'
        const res = await adminApi.searchMetadata(matchQuery, mediaType, media?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) {
          toast.info('TMDb 未找到匹配结果，请尝试其他关键词或切换到 Bangumi 数据源')
        }
      } else {
        // Bangumi 搜索：2=动画, 6=三次元
        const subjectType = (media?.genres || '').includes('动画') ? 2 : 6
        const res = await adminApi.searchBangumi(matchQuery, subjectType, media?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) {
          toast.info('Bangumi 未找到匹配结果，可尝试切换类型（动画/三次元）或更换关键词')
        }
      }
    } catch {
      toast.error(matchSource === 'tmdb' ? '搜索失败，请检查 TMDb API Key 配置' : 'Bangumi 搜索失败')
    } finally {
      setMatchSearching(false)
    }
  }

  const handleMatchSelect = async (resultId: number) => {
    if (!id) return
    try {
      if (matchSource === 'tmdb') {
        await adminApi.matchMetadata(id, resultId)
      } else {
        await adminApi.matchMediaBangumi(id, resultId)
      }
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      setShowMatchModal(false)
      toast.success(`影片匹配成功（来源：${matchSource === 'tmdb' ? 'TMDb' : 'Bangumi'}）`)
    } catch {
      toast.error('匹配失败')
    }
  }

  const handleUnmatch = async () => {
    if (!id) return
    try {
      await adminApi.unmatchMetadata(id)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      setShowUnmatchConfirm(false)
      toast.success('已解除匹配')
    } catch {
      toast.error('解除匹配失败')
    }
  }

  const handleRefreshMetadata = async () => {
    if (!id) return
    setScraping(true)
    try {
      await mediaApi.scrape(id)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      toast.success('元数据刷新成功')
    } catch {
      toast.error('元数据刷新失败，请检查 TMDb API Key 配置')
    } finally {
      setScraping(false)
    }
  }

  const handleEditMetadata = () => {
    if (!media) return
    setEditForm({
      title: media.title || '',
      orig_title: media.orig_title || '',
      year: media.year || 0,
      overview: media.overview || '',
      rating: media.rating || 0,
      genres: media.genres || '',
      country: media.country || '',
      language: media.language || '',
      tagline: media.tagline || '',
      studio: media.studio || '',
    })
    setShowEditModal(true)
  }

  const handleEditSave = async () => {
    if (!id) return
    try {
      await adminApi.updateMediaMetadata(id, editForm)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      setShowEditModal(false)
      toast.success('元数据已更新')
    } catch {
      toast.error('更新元数据失败')
    }
  }

  const handleDelete = async () => {
    if (!id) return
    try {
      await adminApi.deleteMedia(id)
      toast.success('影片已删除')
      navigate(-1)
    } catch {
      toast.error('删除影片失败')
    }
  }

  // ==================== 骨架屏 ====================
  if (loading || !media) {
    return (
      <div className="space-y-6 animate-fade-in">
        <div className="skeleton h-[420px] rounded-2xl" />
        <div className="flex gap-6 pt-4">
          <div className="skeleton hidden h-72 w-48 rounded-xl sm:block" />
          <div className="flex-1 space-y-4">
            <div className="skeleton h-10 w-2/3 rounded-lg" />
            <div className="skeleton h-5 w-1/3 rounded-lg" />
            <div className="flex gap-3">
              <div className="skeleton h-12 w-28 rounded-xl" />
              <div className="skeleton h-12 w-24 rounded-xl" />
              <div className="skeleton h-12 w-28 rounded-xl" />
            </div>
            <div className="skeleton h-20 w-full rounded-xl" />
          </div>
        </div>
      </div>
    )
  }

  // ==================== 渲染 ====================
  return (
    <div className="animate-fade-in -mx-4 -mt-6 sm:-mx-6 lg:-mx-8">
      {/* 英雄区 */}
      <HeroSection
        media={media}
        playInfo={playInfo}
        isFavorited={isFavorited}
        watchProgress={watchProgress}
        playlists={playlists}
        scraping={scraping}
        isAdmin={user?.role === 'admin'}
        onFavorite={handleFavorite}
        onScrape={handleScrape}
        onAddToPlaylist={handleAddToPlaylist}
        onShowTrailer={media.trailer_url ? () => setShowTrailer(true) : undefined}
        onManualMatch={handleManualMatch}
        onUnmatch={() => setShowUnmatchConfirm(true)}
        onRefreshMetadata={handleRefreshMetadata}
        onEditMetadata={handleEditMetadata}
        onDelete={() => setShowDeleteConfirm(true)}
      />

      {/* 内容区 */}
      <div className="mx-auto max-w-7xl space-y-8 px-4 pt-6 sm:px-6 lg:px-8">
        {/* 媒体信息（简介 + 类型 + 演职 + 文件 + 视频） */}
        <MediaInfoSection
          media={media}
          playInfo={playInfo}
          persons={persons}
          isAdmin={user?.role === 'admin'}
        />

        {/* 相关推荐 */}
        <RecommendationCarousel recommendations={recommendations} />

        {/* 评论区 */}
        {id && <CommentSection mediaId={id} />}
      </div>

      {/* 预告片弹窗 */}
      {showTrailer && media.trailer_url && (
        <TrailerModal
          trailerUrl={media.trailer_url}
          onClose={() => setShowTrailer(false)}
        />
      )}

      {/* ==================== 手动匹配弹窗 ==================== */}
      {showMatchModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-2xl rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-4 text-lg font-bold" style={{ color: 'var(--text-primary)' }}>手动匹配影片</h3>
            {/* 数据源切换 */}
            <div className="mb-4 flex gap-2">
              <button
                onClick={() => { setMatchSource('tmdb'); setMatchResults([]) }}
                className="rounded-lg px-4 py-1.5 text-sm font-medium transition-all"
                style={{
                  background: matchSource === 'tmdb' ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))' : 'var(--bg-surface)',
                  color: matchSource === 'tmdb' ? '#fff' : 'var(--text-secondary)',
                  border: matchSource === 'tmdb' ? 'none' : '1px solid var(--border-default)',
                }}
              >
                🎬 TMDb
              </button>
              <button
                onClick={() => { setMatchSource('bangumi'); setMatchResults([]) }}
                className="rounded-lg px-4 py-1.5 text-sm font-medium transition-all"
                style={{
                  background: matchSource === 'bangumi' ? 'linear-gradient(135deg, #f09199, #e8788a)' : 'var(--bg-surface)',
                  color: matchSource === 'bangumi' ? '#fff' : 'var(--text-secondary)',
                  border: matchSource === 'bangumi' ? 'none' : '1px solid var(--border-default)',
                }}
              >
                📺 Bangumi
              </button>
            </div>
            <p className="mb-3 text-xs" style={{ color: 'var(--text-muted)' }}>
              {matchSource === 'tmdb'
                ? '搜索 TMDb 数据库，适合欧美电影/电视剧。'
                : '搜索 Bangumi (bgm.tv) 数据库，适合日本动画/日剧。'
              }
            </p>
            <div className="mb-4 flex gap-2">
              <input
                value={matchQuery}
                onChange={(e) => setMatchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleMatchSearch()}
                placeholder="输入影片名称搜索..."
                className="flex-1 rounded-xl px-4 py-2.5 text-sm outline-none"
                style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
                autoFocus
              />
              <button
                onClick={handleMatchSearch}
                disabled={matchSearching}
                className="rounded-xl px-5 py-2.5 text-sm font-semibold text-white transition-all hover:opacity-90 disabled:opacity-50"
                style={{ background: matchSource === 'tmdb' ? 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))' : 'linear-gradient(135deg, #f09199, #e8788a)' }}
              >
                {matchSearching ? '搜索中...' : '搜索'}
              </button>
            </div>
            <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
              {matchResults.map((result: any) => {
                // TMDb 和 Bangumi 结果的统一展示
                const isBangumi = matchSource === 'bangumi'
                const displayTitle = isBangumi ? (result.name_cn || result.name) : (result.title || result.name)
                const displayOrigTitle = isBangumi ? result.name : (result.original_title || result.original_name)
                const displayYear = isBangumi ? result.air_date?.split('-')[0] : (result.release_date || result.first_air_date)?.split('-')[0]
                const displayRating = isBangumi ? result.rating?.score : result.vote_average
                const displayOverview = isBangumi ? result.summary : result.overview
                const posterUrl = isBangumi
                  ? result.images?.common || result.images?.medium
                  : (result.poster_path ? `https://image.tmdb.org/t/p/w92${result.poster_path}` : null)

                return (
                  <button
                    key={result.id}
                    onClick={() => handleMatchSelect(result.id)}
                    className="flex w-full items-center gap-3 rounded-xl p-3 text-left transition-all hover:scale-[1.01]"
                    style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
                  >
                    {posterUrl ? (
                      <img src={posterUrl} alt="" className="h-16 w-11 rounded-lg object-cover" />
                    ) : (
                      <div className="flex h-16 w-11 items-center justify-center rounded-lg" style={{ background: 'var(--bg-card)', color: 'var(--text-muted)' }}>
                        <span className="text-xs">N/A</span>
                      </div>
                    )}
                    <div className="min-w-0 flex-1">
                      <div className="flex items-center gap-2">
                        <div className="truncate text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                          {displayTitle}
                        </div>
                        {isBangumi && (
                          <span className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(240,145,153,0.15)', color: '#f09199' }}>
                            {result.type === 2 ? '动画' : result.type === 6 ? '三次元' : 'BGM'}
                          </span>
                        )}
                      </div>
                      {displayOrigTitle && displayOrigTitle !== displayTitle && (
                        <div className="truncate text-xs" style={{ color: 'var(--text-tertiary)' }}>{displayOrigTitle}</div>
                      )}
                      <div className="mt-0.5 flex items-center gap-2 text-xs" style={{ color: 'var(--text-muted)' }}>
                        {displayYear && <span>{displayYear}</span>}
                        {displayRating > 0 && (
                          <span className="text-yellow-400">★ {displayRating.toFixed(1)}</span>
                        )}
                        {isBangumi && result.eps > 0 && (
                          <span>{result.eps}话</span>
                        )}
                      </div>
                      {displayOverview && (
                        <p className="mt-1 line-clamp-2 text-xs" style={{ color: 'var(--text-tertiary)' }}>{displayOverview}</p>
                      )}
                    </div>
                  </button>
                )
              })}
              {matchResults.length === 0 && !matchSearching && (
                <div className="py-8 text-center text-sm" style={{ color: 'var(--text-muted)' }}>
                  输入关键词搜索{matchSource === 'tmdb' ? ' TMDb' : ' Bangumi'} 数据库
                </div>
              )}
            </div>
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => setShowMatchModal(false)}
                className="rounded-xl px-5 py-2 text-sm font-medium transition-colors hover:opacity-80"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ==================== 解除匹配确认弹窗 ==================== */}
      {showUnmatchConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-md rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-2 text-lg font-bold" style={{ color: 'var(--text-primary)' }}>解除匹配影片</h3>
            <p className="mb-6 text-sm" style={{ color: 'var(--text-secondary)' }}>
              确定要解除此影片的元数据匹配吗？这将清除所有从 TMDb/豆瓣获取的信息（简介、海报、评分等），但保留文件扫描的原始信息。
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowUnmatchConfirm(false)}
                className="rounded-xl px-5 py-2.5 text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={handleUnmatch}
                className="rounded-xl bg-orange-600 px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-orange-500"
              >
                确认解除
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ==================== 编辑元数据弹窗 ==================== */}
      {showEditModal && (
        <EditMetadataModal
          type="media"
          id={id!}
          tmdbId={media.tmdb_id}
          mediaType={media.media_type === 'episode' ? 'tv' : 'movie'}
          editForm={editForm}
          setEditForm={setEditForm}
          currentPoster={streamApi.getPosterUrl(media.id)}
          hasPoster={!!media.poster_path}
          hasBackdrop={!!media.backdrop_path}
          onSave={handleEditSave}
          onClose={() => setShowEditModal(false)}
          hasTagline
        />
      )}

      {/* ==================== 删除确认弹窗 ==================== */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-md rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-2 text-lg font-bold text-red-400">删除影片</h3>
            <p className="mb-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
              确定要删除此影片记录吗？
            </p>
            <p className="mb-6 text-xs" style={{ color: 'var(--text-muted)' }}>
              此操作仅从数据库中移除记录，不会删除磁盘上的视频文件。重新扫描媒体库后可恢复。
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="rounded-xl px-5 py-2.5 text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                取消
              </button>
              <button
                onClick={handleDelete}
                className="rounded-xl bg-red-600 px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-red-500"
              >
                确认删除
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
