import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { mediaApi, userApi, streamApi, playlistApi, recommendApi, adminApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import type { Media, MediaPlayInfo, Playlist, RecommendedMedia, MediaPerson, WatchHistory, TechSpecs, FileDetail, LibraryInfo, PlaybackStatsInfo } from '@/types'
import { HeroSection, MediaInfoSection, MediaTechSpecs, RecommendationCarousel, TrailerModal, CastGrid } from '@/components/media'
import CommentSection from '@/components/CommentSection'
import EditMetadataModal from '@/components/EditMetadataModal'
import SubtitleManager from '@/components/SubtitleManager'
import { useTranslation } from '@/i18n'
import { motion, AnimatePresence } from 'framer-motion'
import { easeSmooth, durations } from '@/lib/motion'

export default function MediaDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const user = useAuthStore((s) => s.user)
  const toast = useToast()
  const { t } = useTranslation()

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

  // 增强详情数据
  const [techSpecs, setTechSpecs] = useState<TechSpecs | null>(null)
  const [fileInfo, setFileInfo] = useState<FileDetail | null>(null)
  const [libraryInfo, setLibraryInfo] = useState<LibraryInfo | null>(null)
  const [playbackStats, setPlaybackStats] = useState<PlaybackStatsInfo | null>(null)
  const [enhancedLoading, setEnhancedLoading] = useState(false)

  // UI 状态
  const [scraping, setScraping] = useState(false)
  const [showTrailer, setShowTrailer] = useState(false)

  // 管理功能状态
  const [showMatchModal, setShowMatchModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showUnmatchConfirm, setShowUnmatchConfirm] = useState(false)
  const [showSubtitleManager, setShowSubtitleManager] = useState(false)
  const [matchQuery, setMatchQuery] = useState('')
  const [matchResults, setMatchResults] = useState<any[]>([])
  const [matchSearching, setMatchSearching] = useState(false)
  const [matchSource, setMatchSource] = useState<'tmdb' | 'bangumi' | 'douban' | 'thetvdb'>('tmdb')
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

        // 增强详情（分块加载，不阻塞首屏）
        setEnhancedLoading(true)
        mediaApi.detailEnhanced(mediaData.id)
          .then((res) => {
            if (abortController.signal.aborted) return
            const data = res.data.data
            setTechSpecs(data.tech_specs)
            setFileInfo(data.file_info)
            setLibraryInfo(data.library)
            setPlaybackStats(data.playback_stats)
          })
          .catch(() => {})
          .finally(() => { if (!abortController.signal.aborted) setEnhancedLoading(false) })
      })
      .catch(() => {
        if (abortController.signal.aborted) return
        toast.error(t('mediaDetail.loadFailed'))
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
      toast.error(t('mediaDetail.favoriteFailed'))
    }
  }

  const handleScrape = async () => {
    if (!id) return
    setScraping(true)
    try {
      await mediaApi.scrape(id)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      toast.success(t('mediaDetail.scrapeSuccess'))
    } catch {
      toast.error(t('mediaDetail.scrapeFailed'))
    } finally {
      setScraping(false)
    }
  }

  const handleAddToPlaylist = async (playlistId: string) => {
    if (!id) return
    try {
      await playlistApi.addItem(playlistId, id)
      toast.success(t('mediaDetail.addToPlaylistSuccess'))
    } catch {
      toast.error(t('mediaDetail.addToPlaylistFailed'))
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
          toast.info(t('mediaDetail.tmdbNoResult'))
        }
      } else if (matchSource === 'douban') {
        const res = await adminApi.searchDouban(matchQuery, media?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) {
          toast.info(t('mediaDetail.doubanNoResult'))
        }
      } else if (matchSource === 'thetvdb') {
        const res = await adminApi.searchTheTVDB(matchQuery, media?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) {
          toast.info(t('mediaDetail.thetvdbNoResult'))
        }
      } else {
        // Bangumi 搜索：2=动画, 6=三次元
        const subjectType = (media?.genres || '').includes('动画') ? 2 : 6
        const res = await adminApi.searchBangumi(matchQuery, subjectType, media?.year || undefined)
        setMatchResults(res.data.data || [])
        if ((res.data.data || []).length === 0) {
          toast.info(t('mediaDetail.bangumiNoResult'))
        }
      }
    } catch {
      const errorMap: Record<string, string> = {
        tmdb: t('mediaDetail.tmdbSearchFailed'),
        douban: t('mediaDetail.doubanSearchFailed'),
        thetvdb: t('mediaDetail.thetvdbSearchFailed'),
        bangumi: t('mediaDetail.bangumiSearchFailed'),
      }
      toast.error(errorMap[matchSource] || t('mediaDetail.matchFailed'))
    } finally {
      setMatchSearching(false)
    }
  }

  const handleMatchSelect = async (resultId: number | string) => {
    if (!id) return
    try {
      const sourceNameMap: Record<string, string> = { tmdb: 'TMDb', bangumi: 'Bangumi', douban: '豆瓣', thetvdb: 'TheTVDB' }
      if (matchSource === 'tmdb') {
        await adminApi.matchMetadata(id, resultId as number)
      } else if (matchSource === 'douban') {
        await adminApi.matchMediaDouban(id, resultId as string)
      } else if (matchSource === 'thetvdb') {
        // TheTVDB 主要用于剧集，但媒体也可以尝试
        toast.info('TheTVDB 主要用于剧集匹配')
        return
      } else {
        await adminApi.matchMediaBangumi(id, resultId as number)
      }
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      setShowMatchModal(false)
      toast.success(t('mediaDetail.matchSuccess', { source: sourceNameMap[matchSource] }))
    } catch {
      toast.error(t('mediaDetail.matchFailed'))
    }
  }

  const handleUnmatch = async () => {
    if (!id) return
    try {
      await adminApi.unmatchMetadata(id)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      setShowUnmatchConfirm(false)
      toast.success(t('mediaDetail.unmatchSuccess'))
    } catch {
      toast.error(t('mediaDetail.unmatchFailed'))
    }
  }

  const handleRefreshMetadata = async () => {
    if (!id) return
    setScraping(true)
    try {
      await mediaApi.scrape(id)
      const res = await mediaApi.detail(id)
      setMedia(res.data.data)
      toast.success(t('mediaDetail.refreshSuccess'))
    } catch {
      toast.error(t('mediaDetail.refreshFailed'))
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
      toast.success(t('mediaDetail.editSuccess'))
    } catch {
      toast.error(t('mediaDetail.editFailed'))
    }
  }

  const handleDelete = async () => {
    if (!id) return
    try {
      await adminApi.deleteMedia(id)
      toast.success(t('mediaDetail.deleteSuccess'))
      navigate(-1)
    } catch {
      toast.error(t('mediaDetail.deleteFailed'))
    }
  }

  // ==================== 骨架屏 / 内容 — AnimatePresence 平滑过渡 ====================
  if (loading || !media) {
    return (
      <AnimatePresence mode="wait">
        <motion.div
          key="skeleton"
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          exit={{ opacity: 0 }}
          transition={{ duration: durations.fast }}
          className="space-y-6"
        >
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
        </motion.div>
      </AnimatePresence>
    )
  }

  // ==================== 渲染 ====================
  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: durations.page, ease: easeSmooth as unknown as [number, number, number, number] }}
      className="-mx-4 -mt-6 sm:-mx-6 lg:-mx-8"
    >
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
        {/* 媒体信息（简介 + 类型 + 演职） */}
        <MediaInfoSection
          media={media}
          playInfo={playInfo}
          persons={persons}
        />

        {/* 演职人员 */}
        <CastGrid persons={persons} />

        {/* 文件信息与技术规格（统一展示区域） */}
        <MediaTechSpecs
          media={media}
          techSpecs={techSpecs}
          fileInfo={fileInfo}
          library={libraryInfo}
          playbackStats={playbackStats}
          loading={enhancedLoading}
          isAdmin={user?.role === 'admin'}
        />

        {/* 字幕管理入口（管理员可见） */}
        {user?.role === 'admin' && (
          <section>
            <button
              onClick={() => setShowSubtitleManager(true)}
              className="flex items-center gap-2 rounded-xl px-5 py-3 text-sm font-medium transition-all hover:opacity-90"
              style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
            >
              <span>🎬</span>
              <span>字幕管理</span>
              <span className="text-xs" style={{ color: 'var(--text-muted)' }}>查看内嵌/外挂字幕 · 批量提取导出</span>
            </button>
          </section>
        )}

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
            <h3 className="mb-4 text-lg font-bold" style={{ color: 'var(--text-primary)' }}>{t('mediaDetail.manualMatch')}</h3>
            {/* 数据源切换 */}
            <div className="mb-4 flex flex-wrap gap-2">
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
                onClick={() => { setMatchSource('douban'); setMatchResults([]) }}
                className="rounded-lg px-4 py-1.5 text-sm font-medium transition-all"
                style={{
                  background: matchSource === 'douban' ? 'linear-gradient(135deg, #00b414, #009910)' : 'var(--bg-surface)',
                  color: matchSource === 'douban' ? '#fff' : 'var(--text-secondary)',
                  border: matchSource === 'douban' ? 'none' : '1px solid var(--border-default)',
                }}
              >
                🎯 {t('mediaDetail.doubanLabel')}
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
              <button
                onClick={() => { setMatchSource('thetvdb'); setMatchResults([]) }}
                className="rounded-lg px-4 py-1.5 text-sm font-medium transition-all"
                style={{
                  background: matchSource === 'thetvdb' ? 'linear-gradient(135deg, #6dc849, #4fa82d)' : 'var(--bg-surface)',
                  color: matchSource === 'thetvdb' ? '#fff' : 'var(--text-secondary)',
                  border: matchSource === 'thetvdb' ? 'none' : '1px solid var(--border-default)',
                }}
              >
                📡 TheTVDB
              </button>
            </div>
            <p className="mb-3 text-xs" style={{ color: 'var(--text-muted)' }}>
              {{
                tmdb: t('mediaDetail.tmdbDesc'),
                douban: t('mediaDetail.doubanDesc'),
                bangumi: t('mediaDetail.bangumiDesc'),
                thetvdb: t('mediaDetail.thetvdbDesc'),
              }[matchSource]}
            </p>
            <div className="mb-4 flex gap-2">
              <input
                value={matchQuery}
                onChange={(e) => setMatchQuery(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleMatchSearch()}
                placeholder={t('mediaDetail.searchPlaceholder')}
                className="flex-1 rounded-xl px-4 py-2.5 text-sm outline-none"
                style={{ background: 'var(--bg-surface)', border: '1px solid var(--border-default)', color: 'var(--text-primary)' }}
                autoFocus
              />
              <button
                onClick={handleMatchSearch}
                disabled={matchSearching}
                className="rounded-xl px-5 py-2.5 text-sm font-semibold text-white transition-all hover:opacity-90 disabled:opacity-50"
                style={{ background: { tmdb: 'linear-gradient(135deg, var(--neon-blue), var(--neon-blue-mid))', douban: 'linear-gradient(135deg, #00b414, #009910)', bangumi: 'linear-gradient(135deg, #f09199, #e8788a)', thetvdb: 'linear-gradient(135deg, #6dc849, #4fa82d)' }[matchSource] }}
              >
                {matchSearching ? t('mediaDetail.searching') : t('mediaDetail.searchBtn')}
              </button>
            </div>
            <div className="max-h-80 space-y-2 overflow-y-auto pr-1">
              {matchResults.map((result: any) => {
                // 多数据源结果的统一展示
                let displayTitle = '', displayOrigTitle = '', displayYear = '', displayOverview = '', posterUrl: string | null = null
                let displayRating = 0, resultKey: string | number = result.id

                if (matchSource === 'tmdb') {
                  displayTitle = result.title || result.name
                  displayOrigTitle = result.original_title || result.original_name
                  displayYear = (result.release_date || result.first_air_date)?.split('-')[0] || ''
                  displayRating = result.vote_average || 0
                  displayOverview = result.overview || ''
                  posterUrl = result.poster_path ? `https://image.tmdb.org/t/p/w92${result.poster_path}` : null
                } else if (matchSource === 'douban') {
                  displayTitle = result.title
                  displayYear = result.year > 0 ? String(result.year) : ''
                  displayRating = result.rating || 0
                  displayOverview = result.overview || ''
                  posterUrl = result.cover || null
                  resultKey = result.id
                } else if (matchSource === 'thetvdb') {
                  displayTitle = result.name || result.seriesName
                  displayOrigTitle = result.originalName || ''
                  displayYear = result.year || (result.firstAired?.split('-')[0]) || ''
                  displayOverview = result.overview || ''
                  posterUrl = result.image || result.poster || null
                  if (posterUrl && !posterUrl.startsWith('http')) posterUrl = 'https://artworks.thetvdb.com' + posterUrl
                } else {
                  // Bangumi
                  displayTitle = result.name_cn || result.name
                  displayOrigTitle = result.name
                  displayYear = result.air_date?.split('-')[0] || ''
                  displayRating = result.rating?.score || 0
                  displayOverview = result.summary || ''
                  posterUrl = result.images?.common || result.images?.medium || null
                }

                return (
                  <button
                    key={resultKey}
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
                        {matchSource === 'bangumi' && (
                          <span className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(240,145,153,0.15)', color: '#f09199' }}>
                            {result.type === 2 ? '动画' : result.type === 6 ? '三次元' : 'BGM'}
                          </span>
                        )}
                        {matchSource === 'douban' && result.genres && (
                          <span className="shrink-0 rounded px-1.5 py-0.5 text-[10px] font-medium" style={{ background: 'rgba(0,180,20,0.12)', color: '#00b414' }}>
                            {result.genres.split(',')[0]}
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
                        {matchSource === 'bangumi' && result.eps > 0 && (
                          <span>{result.eps}{t('mediaDetail.episodes')}</span>
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
                  {t('mediaDetail.searchHint', { source: ' ' + { tmdb: 'TMDb', douban: '豆瓣', bangumi: 'Bangumi', thetvdb: 'TheTVDB' }[matchSource] })}
                </div>
              )}
            </div>
            <div className="mt-4 flex justify-end">
              <button
                onClick={() => setShowMatchModal(false)}
                className="rounded-xl px-5 py-2 text-sm font-medium transition-colors hover:opacity-80"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                {t('common.cancel')}
              </button>
            </div>
          </div>
        </div>
      )}

      {showUnmatchConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-md rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-2 text-lg font-bold" style={{ color: 'var(--text-primary)' }}>{t('mediaDetail.unmatchTitle')}</h3>
            <p className="mb-6 text-sm" style={{ color: 'var(--text-secondary)' }}>
              {t('mediaDetail.unmatchDesc')}
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowUnmatchConfirm(false)}
                className="rounded-xl px-5 py-2.5 text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={handleUnmatch}
                className="rounded-xl bg-orange-600 px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-orange-500"
              >
                {t('mediaDetail.unmatchConfirm')}
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

      {/* ==================== 字幕管理弹窗 ==================== */}
      {showSubtitleManager && (
        <SubtitleManager
          mediaId={id!}
          mediaTitle={media.title}
          onClose={() => setShowSubtitleManager(false)}
        />
      )}

      {/* ==================== 删除确认弹窗 ==================== */}
      {showDeleteConfirm && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4" style={{ background: 'rgba(0,0,0,0.7)' }}>
          <div className="w-full max-w-md rounded-2xl p-6 shadow-2xl" style={{ background: 'var(--bg-elevated)', border: '1px solid var(--glass-border)' }}>
            <h3 className="mb-2 text-lg font-bold text-red-400">{t('mediaDetail.deleteTitle')}</h3>
            <p className="mb-2 text-sm" style={{ color: 'var(--text-secondary)' }}>
              {t('mediaDetail.deleteDesc')}
            </p>
            <p className="mb-6 text-xs" style={{ color: 'var(--text-muted)' }}>
              {t('mediaDetail.deleteHint')}
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowDeleteConfirm(false)}
                className="rounded-xl px-5 py-2.5 text-sm font-medium transition-colors"
                style={{ color: 'var(--text-secondary)', background: 'var(--bg-surface)', border: '1px solid var(--border-default)' }}
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={handleDelete}
                className="rounded-xl bg-red-600 px-5 py-2.5 text-sm font-semibold text-white transition-all hover:bg-red-500"
              >
                {t('mediaDetail.deleteConfirm')}
              </button>
            </div>
          </div>
        </div>
      )}
    </motion.div>
  )
}
