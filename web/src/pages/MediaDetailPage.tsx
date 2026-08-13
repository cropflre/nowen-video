import { useEffect, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { adminApi, mediaApi, playlistApi, recommendApi, streamApi, userApi } from '@/api'
import { useAuthStore } from '@/stores/auth'
import { useToast } from '@/components/Toast'
import type {
  FileDetail,
  LibraryInfo,
  Media,
  MediaPerson,
  MediaPlayInfo,
  PlaybackStatsInfo,
  Playlist,
  RecommendedMedia,
  TechSpecs,
  WatchHistory,
} from '@/types'
import {
  CastGrid,
  CollectionCarousel,
  HeroSection,
  MediaInfoSection,
  MediaTechSpecs,
  RecommendationCarousel,
  TrailerModal,
} from '@/components/media'
import MetadataMatchModal, { type MetadataMatchSource } from '@/components/media/MetadataMatchModal'
import CommentSection from '@/components/CommentSection'
import EditMetadataModal from '@/components/EditMetadataModal'
import SubtitleManager from '@/components/SubtitleManager'
import ConfirmDialog from '@/components/design-system/ConfirmDialog'
import { Button, Surface } from '@/components/design-system'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import { useTranslation } from '@/i18n'
import { formatErrMsg } from '@/utils/error'
import { parseDirectMatchId } from '@/utils/parseDirectMatchId'
import { invalidateMediaListCaches } from '@/utils/invalidateMediaCaches'
import { AnimatePresence, motion } from 'framer-motion'
import { durations, easeSmooth } from '@/lib/motion'
import { ArrowLeft, Captions } from 'lucide-react'

export default function MediaDetailPage() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const user = useAuthStore((state) => state.user)
  const toast = useToast()
  const { t } = useTranslation()

  const [media, setMedia] = useState<Media | null>(null)
  const [playInfo, setPlayInfo] = useState<MediaPlayInfo | null>(null)
  const [loading, setLoading] = useState(true)

  const [isFavorited, setIsFavorited] = useState(false)
  const [playlists, setPlaylists] = useState<Playlist[]>([])
  const [watchProgress, setWatchProgress] = useState<WatchHistory | null>(null)

  const [recommendations, setRecommendations] = useState<RecommendedMedia[]>([])
  const [persons, setPersons] = useState<MediaPerson[]>([])

  const [techSpecs, setTechSpecs] = useState<TechSpecs | null>(null)
  const [fileInfo, setFileInfo] = useState<FileDetail | null>(null)
  const [libraryInfo, setLibraryInfo] = useState<LibraryInfo | null>(null)
  const [playbackStats, setPlaybackStats] = useState<PlaybackStatsInfo | null>(null)
  const [enhancedLoading, setEnhancedLoading] = useState(false)

  const [scraping, setScraping] = useState(false)
  const [showTrailer, setShowTrailer] = useState(false)
  const [posterVersion, setPosterVersion] = useState<number>(() => Date.now())

  const [showMatchModal, setShowMatchModal] = useState(false)
  const [showEditModal, setShowEditModal] = useState(false)
  const [showDeleteConfirm, setShowDeleteConfirm] = useState(false)
  const [showUnmatchConfirm, setShowUnmatchConfirm] = useState(false)
  const [showSubtitleManager, setShowSubtitleManager] = useState(false)
  const [matchQuery, setMatchQuery] = useState('')
  const [matchResults, setMatchResults] = useState<any[]>([])
  const [matchSearching, setMatchSearching] = useState(false)
  const [matchSource, setMatchSource] = useState<MetadataMatchSource>('tmdb')
  const [matchSelectedId, setMatchSelectedId] = useState<number | string | null>(null)
  const [matchApplying, setMatchApplying] = useState(false)
  const [editForm, setEditForm] = useState<{
    title: string
    orig_title: string
    year: number
    overview: string
    rating: number
    genres: string
    country: string
    language: string
    tagline: string
    studio: string
  }>({
    title: '',
    orig_title: '',
    year: 0,
    overview: '',
    rating: 0,
    genres: '',
    country: '',
    language: '',
    tagline: '',
    studio: '',
  })

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
      .then(([mediaResponse, playInfoResponse, playlistResponse]) => {
        if (abortController.signal.aborted) return
        const mediaData = mediaResponse.data.data
        setMedia(mediaData)
        setPlayInfo(playInfoResponse.data.data)
        setPlaylists(playlistResponse.data.data || [])

        userApi.checkFavorite(mediaData.id)
          .then((response) => { if (!abortController.signal.aborted) setIsFavorited(response.data.data) })
          .catch(() => {})
        recommendApi.getSimilarMedia(mediaData.id, 12)
          .then((response) => { if (!abortController.signal.aborted) setRecommendations(response.data.data || []) })
          .catch(() => {})
        mediaApi.getPersons(mediaData.id)
          .then((response) => { if (!abortController.signal.aborted) setPersons(response.data.data || []) })
          .catch(() => {})
        userApi.getProgress(mediaData.id)
          .then((response) => { if (!abortController.signal.aborted) setWatchProgress(response.data.data) })
          .catch(() => {})

        setEnhancedLoading(true)
        mediaApi.detailEnhanced(mediaData.id)
          .then((response) => {
            if (abortController.signal.aborted) return
            const data = response.data.data
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
      const response = await mediaApi.detail(id)
      setMedia(response.data.data)
      invalidateMediaListCaches()
      toast.success(t('mediaDetail.scrapeSuccess'))
    } catch (error) {
      toast.error(formatErrMsg(error, t('mediaDetail.scrapeFailed')))
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

  const handleManualMatch = () => {
    if (!media) return
    setMatchQuery(media.title)
    setMatchResults([])
    setMatchSource('tmdb')
    setMatchSelectedId(null)
    setShowMatchModal(true)
  }

  const refreshMediaDetail = async (mediaId: string) => {
    try {
      const [detailResponse, enhancedResponse, personsResponse, recommendResponse] = await Promise.all([
        mediaApi.detail(mediaId),
        mediaApi.detailEnhanced(mediaId).catch(() => null),
        mediaApi.getPersons(mediaId).catch(() => null),
        recommendApi.getSimilarMedia(mediaId, 12).catch(() => null),
      ])
      setMedia(detailResponse.data.data)
      if (enhancedResponse) {
        const data = enhancedResponse.data.data
        setTechSpecs(data.tech_specs)
        setFileInfo(data.file_info)
        setLibraryInfo(data.library)
        setPlaybackStats(data.playback_stats)
      }
      if (personsResponse) setPersons(personsResponse.data.data || [])
      if (recommendResponse) setRecommendations(recommendResponse.data.data || [])
    } catch {
      // 匹配已经成功时，详情二次刷新失败不阻断主流程。
    }
  }

  const handleMatchSearch = async () => {
    if (!matchQuery.trim()) return
    const direct = parseDirectMatchId(matchQuery, matchSource)
    if (direct) {
      if (direct.source !== matchSource) {
        setMatchSource(direct.source)
        toast.info(`已识别为 ${{ tmdb: 'TMDb', douban: '豆瓣', bangumi: 'Bangumi', thetvdb: 'TheTVDB' }[direct.source]} 链接，已自动切换数据源`)
      }
      const idForApply: number | string = direct.source === 'douban' ? direct.id : Number(direct.id)
      setMatchResults([{
        id: idForApply,
        title: `直输 ID：${direct.id}`,
        original_title: '',
        name: `直输 ID：${direct.id}`,
        original_name: '',
        overview: `点击“应用”将本条目绑定到 ${{ tmdb: 'TMDb', douban: '豆瓣', bangumi: 'Bangumi', thetvdb: 'TheTVDB' }[direct.source]} ID = ${direct.id}。`,
        release_date: '',
        first_air_date: '',
        vote_average: 0,
        poster_path: '',
      }])
      setMatchSelectedId(idForApply)
      toast.success(`已识别 ${{ tmdb: 'TMDb', douban: '豆瓣', bangumi: 'Bangumi', thetvdb: 'TheTVDB' }[direct.source]} ID：${direct.id}，点击“应用”即可绑定`)
      return
    }

    setMatchSearching(true)
    try {
      if (matchSource === 'tmdb') {
        const mediaType = media?.media_type === 'episode' ? 'tv' : 'movie'
        const response = await adminApi.searchMetadata(matchQuery, mediaType, media?.year || undefined)
        setMatchResults(response.data.data || [])
        if ((response.data.data || []).length === 0) toast.info(t('mediaDetail.tmdbNoResult'))
      } else if (matchSource === 'douban') {
        const response = await adminApi.searchDouban(matchQuery, media?.year || undefined)
        setMatchResults(response.data.data || [])
        if ((response.data.data || []).length === 0) toast.info(t('mediaDetail.doubanNoResult'))
      } else if (matchSource === 'thetvdb') {
        const response = await adminApi.searchTheTVDB(matchQuery, media?.year || undefined)
        setMatchResults(response.data.data || [])
        if ((response.data.data || []).length === 0) toast.info(t('mediaDetail.thetvdbNoResult'))
      } else {
        const subjectType = (media?.genres || '').includes('动画') ? 2 : 6
        const response = await adminApi.searchBangumi(matchQuery, subjectType, media?.year || undefined)
        setMatchResults(response.data.data || [])
        if ((response.data.data || []).length === 0) toast.info(t('mediaDetail.bangumiNoResult'))
      }
    } catch (error) {
      const errorMap: Record<string, string> = {
        tmdb: t('mediaDetail.tmdbSearchFailed'),
        douban: t('mediaDetail.doubanSearchFailed'),
        thetvdb: t('mediaDetail.thetvdbSearchFailed'),
        bangumi: t('mediaDetail.bangumiSearchFailed'),
      }
      toast.error(formatErrMsg(error, errorMap[matchSource] || t('mediaDetail.matchFailed')))
    } finally {
      setMatchSearching(false)
    }
  }

  const handleMatchSelect = (resultId: number | string) => {
    if (matchSource === 'thetvdb') {
      toast.info('TheTVDB 主要用于剧集匹配')
      return
    }
    setMatchSelectedId(resultId)
  }

  const handleMatchApply = async () => {
    if (!id || matchSelectedId === null) return
    setMatchApplying(true)
    try {
      const sourceNameMap: Record<string, string> = {
        tmdb: 'TMDb',
        bangumi: 'Bangumi',
        douban: '豆瓣',
        thetvdb: 'TheTVDB',
      }
      if (matchSource === 'tmdb') await adminApi.matchMetadata(id, matchSelectedId as number)
      else if (matchSource === 'douban') await adminApi.matchMediaDouban(id, matchSelectedId as string)
      else if (matchSource === 'bangumi') await adminApi.matchMediaBangumi(id, matchSelectedId as number)
      else {
        toast.info('TheTVDB 主要用于剧集匹配')
        return
      }

      await refreshMediaDetail(id)
      setPosterVersion(Date.now())
      bumpPosterVersion()
      invalidateMediaListCaches()
      setShowMatchModal(false)
      setMatchSelectedId(null)
      toast.success(t('mediaDetail.matchSuccess', { source: sourceNameMap[matchSource] }))
    } catch (error) {
      toast.error(formatErrMsg(error, t('mediaDetail.matchFailed')))
    } finally {
      setMatchApplying(false)
    }
  }

  const handleUnmatch = async () => {
    if (!id) return
    try {
      await adminApi.unmatchMetadata(id)
      const response = await mediaApi.detail(id)
      setMedia(response.data.data)
      setPosterVersion(Date.now())
      invalidateMediaListCaches()
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
      const response = await mediaApi.detail(id)
      setMedia(response.data.data)
      setPosterVersion(Date.now())
      invalidateMediaListCaches()
      toast.success(t('mediaDetail.refreshSuccess'))
    } catch (error) {
      toast.error(formatErrMsg(error, t('mediaDetail.refreshFailed')))
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
      const response = await mediaApi.detail(id)
      setMedia(response.data.data)
      setPosterVersion(Date.now())
      invalidateMediaListCaches()
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
      invalidateMediaListCaches()
      toast.success(t('mediaDetail.deleteSuccess'))
      navigate(-1)
    } catch {
      toast.error(t('mediaDetail.deleteFailed'))
    }
  }

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
          <div className="skeleton h-[420px] rounded-[var(--nv-radius-hero)]" />
          <div className="flex gap-6 pt-4">
            <div className="skeleton hidden h-72 w-48 rounded-[var(--nv-radius-card)] sm:block" />
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

  const isAdmin = user?.role === 'admin'

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: durations.page, ease: easeSmooth as unknown as [number, number, number, number] }}
      className="relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8"
    >
      <button
        type="button"
        onClick={() => {
          if (window.history.length > 1) navigate(-1)
          else navigate('/')
        }}
        aria-label="返回"
        title="返回"
        className="absolute left-4 top-4 z-30 flex h-10 w-10 items-center justify-center rounded-full border border-white/15 bg-black/45 text-white shadow-lg backdrop-blur-md transition-[background-color,transform] hover:scale-[1.03] hover:bg-black/60"
      >
        <ArrowLeft size={18} aria-hidden="true" />
      </button>

      <HeroSection
        media={media}
        playInfo={playInfo}
        isFavorited={isFavorited}
        watchProgress={watchProgress}
        playlists={playlists}
        scraping={scraping}
        isAdmin={isAdmin}
        posterVersion={posterVersion}
        onFavorite={handleFavorite}
        onScrape={handleScrape}
        onAddToPlaylist={handleAddToPlaylist}
        onShowTrailer={media.trailer_url ? () => setShowTrailer(true) : undefined}
        onManualMatch={handleManualMatch}
        onUnmatch={() => setShowUnmatchConfirm(true)}
        onRefreshMetadata={handleRefreshMetadata}
        onEditMetadata={handleEditMetadata}
        onDelete={() => setShowDeleteConfirm(true)}
        onPreprocess={() => {
          adminApi.submitPreprocess(id!).then(() => toast.success('已提交预处理任务')).catch(() => toast.error('提交预处理失败'))
        }}
        onTranscode={() => {
          adminApi.submitTranscode(id!).then(() => toast.success('已提交强制转码任务')).catch(() => toast.error('提交转码失败'))
        }}
      />

      <div className="mx-auto w-full max-w-[var(--nv-content-max)] space-y-8 px-[var(--nv-page-gutter)] pt-6">
        <MediaInfoSection media={media} playInfo={playInfo} persons={persons} />
        <CastGrid persons={persons} />

        {media.media_type === 'movie' && id && <CollectionCarousel mediaId={id} />}

        <MediaTechSpecs
          media={media}
          techSpecs={techSpecs}
          fileInfo={fileInfo}
          library={libraryInfo}
          playbackStats={playbackStats}
          loading={enhancedLoading}
          isAdmin={isAdmin}
        />

        {isAdmin && (
          <Surface className="flex flex-col gap-4 p-4 sm:flex-row sm:items-center sm:justify-between sm:p-5">
            <div className="flex min-w-0 items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[var(--nv-radius-control)] bg-[var(--nv-bg-active)] text-[var(--nv-action-primary)]">
                <Captions size={18} aria-hidden="true" />
              </div>
              <div className="min-w-0">
                <h3 className="text-sm font-semibold text-[var(--nv-text-primary)]">字幕管理</h3>
                <p className="mt-1 text-xs leading-5 text-[var(--nv-text-tertiary)]">查看内嵌 / 外挂字幕，批量提取并导出文本字幕。</p>
              </div>
            </div>
            <Button type="button" variant="secondary" onClick={() => setShowSubtitleManager(true)}>
              <Captions size={15} aria-hidden="true" /> 管理字幕
            </Button>
          </Surface>
        )}

        <RecommendationCarousel recommendations={recommendations} />
        {id && <CommentSection mediaId={id} />}
      </div>

      {showTrailer && media.trailer_url && (
        <TrailerModal trailerUrl={media.trailer_url} onClose={() => setShowTrailer(false)} />
      )}

      {showMatchModal && (
        <MetadataMatchModal
          source={matchSource}
          onSourceChange={(source) => {
            setMatchSource(source)
            setMatchResults([])
            setMatchSelectedId(null)
          }}
          query={matchQuery}
          setQuery={setMatchQuery}
          results={matchResults}
          searching={matchSearching}
          selectedId={matchSelectedId}
          applying={matchApplying}
          onSearch={handleMatchSearch}
          onSelect={handleMatchSelect}
          onApply={handleMatchApply}
          onClose={() => setShowMatchModal(false)}
        />
      )}

      {showUnmatchConfirm && (
        <ConfirmDialog
          title={t('mediaDetail.unmatchTitle')}
          description={t('mediaDetail.unmatchDesc')}
          confirmLabel={t('mediaDetail.unmatchConfirm')}
          cancelLabel={t('common.cancel')}
          tone="warning"
          onConfirm={handleUnmatch}
          onClose={() => setShowUnmatchConfirm(false)}
        />
      )}

      {showEditModal && (
        <EditMetadataModal
          type="media"
          id={id!}
          tmdbId={media.tmdb_id}
          mediaType={media.media_type === 'episode' ? 'tv' : 'movie'}
          editForm={editForm}
          setEditForm={setEditForm}
          currentPoster={streamApi.getPosterUrl(media.id, posterVersion)}
          hasPoster={!!media.poster_path}
          hasBackdrop={!!media.backdrop_path}
          onSave={handleEditSave}
          onClose={() => setShowEditModal(false)}
          hasTagline
        />
      )}

      {showSubtitleManager && (
        <SubtitleManager mediaId={id!} mediaTitle={media.title} onClose={() => setShowSubtitleManager(false)} />
      )}

      {showDeleteConfirm && (
        <ConfirmDialog
          title={t('mediaDetail.deleteTitle')}
          description={t('mediaDetail.deleteDesc')}
          hint={t('mediaDetail.deleteHint')}
          confirmLabel={t('mediaDetail.deleteConfirm')}
          cancelLabel={t('common.cancel')}
          tone="danger"
          onConfirm={handleDelete}
          onClose={() => setShowDeleteConfirm(false)}
        />
      )}
    </motion.div>
  )
}
