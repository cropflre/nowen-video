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
  DetailSectionTitle,
  HeroSection,
  MediaDetailSidebar,
  MediaDetailTechOverview,
  MediaInfoSection,
  MediaTechSpecs,
  RecommendationCarousel,
  TrailerModal,
} from '@/components/media'
import MediaHighlightsPanel from '@/components/media/MediaHighlightsPanel'
import MetadataMatchModal, { type MetadataMatchSource } from '@/components/media/MetadataMatchModal'
import CommentSection from '@/components/CommentSection'
import EditMetadataModal from '@/components/EditMetadataModal'
import SubtitleManager from '@/components/SubtitleManager'
import ConfirmDialog from '@/components/design-system/ConfirmDialog'
import { Button, EmptyState } from '@/components/design-system'
import { DetailTabs } from '@/ui'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import { useTranslation } from '@/i18n'
import { formatErrMsg } from '@/utils/error'
import { parseDirectMatchId } from '@/utils/parseDirectMatchId'
import { invalidateMediaListCaches } from '@/utils/invalidateMediaCaches'
import { AnimatePresence, motion } from 'framer-motion'
import { durations, easeSmooth } from '@/lib/motion'
import { ArrowLeft, Captions, Pencil, RefreshCw } from 'lucide-react'

type DetailTab = 'overview' | 'cast' | 'highlights' | 'tech' | 'subtitles' | 'related'

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
  const [activeTab, setActiveTab] = useState<DetailTab>('overview')
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
    setActiveTab('overview')

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
          className="nv-detail-loading space-y-5"
        >
          <div className="skeleton h-12 rounded-[var(--nv-radius-control)]" />
          <div className="skeleton h-[400px] rounded-[var(--nv-radius-hero)]" />
          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_320px]">
            <div className="space-y-4">
              <div className="skeleton h-14 rounded-[var(--nv-radius-card)]" />
              <div className="skeleton h-64 rounded-[var(--nv-radius-card)]" />
              <div className="skeleton h-40 rounded-[var(--nv-radius-card)]" />
            </div>
            <div className="space-y-4">
              <div className="skeleton h-52 rounded-[var(--nv-radius-card)]" />
              <div className="skeleton h-48 rounded-[var(--nv-radius-card)]" />
            </div>
          </div>
        </motion.div>
      </AnimatePresence>
    )
  }

  const isAdmin = user?.role === 'admin'
  const breadcrumbLabel = media.num || media.title
  const embeddedSubtitleCount = (techSpecs?.streams || []).filter((stream) => stream.codec_type === 'subtitle').length
  const externalSubtitleCount = (() => {
    const raw = media.subtitle_paths?.trim()
    if (!raw) return 0
    if (raw.startsWith('[')) {
      try {
        const parsed = JSON.parse(raw) as unknown
        if (Array.isArray(parsed)) return parsed.map(String).filter(Boolean).length
      } catch {
        // Fall back to delimiter parsing for legacy values.
      }
    }
    return raw.split(/[\n,;|]+/).map((item) => item.trim()).filter(Boolean).length
  })()
  const hasSubtitleData = embeddedSubtitleCount + externalSubtitleCount > 0

  return (
    <motion.div
      initial={{ opacity: 0, y: 8 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: durations.page, ease: easeSmooth as unknown as [number, number, number, number] }}
      className="nv-media-detail-page relative -mx-4 -mt-6 sm:-mx-6 lg:-mx-8"
    >
      <div className="nv-detail-local-toolbar">
        <Button
          type="button"
          variant="ghost"
          size="sm"
          iconOnly
          className="nv-detail-back-button"
          onClick={() => {
            if (window.history.length > 1) navigate(-1)
            else navigate('/')
          }}
          title="返回"
          aria-label="返回"
        >
          <ArrowLeft size={17} aria-hidden="true" />
        </Button>

        <div className="nv-detail-breadcrumb" title={`影视库 / ${breadcrumbLabel}`}>
          <span>影视库</span>
          <span aria-hidden="true">/</span>
          <strong>{breadcrumbLabel}</strong>
        </div>

        <div className="nv-detail-toolbar-spacer" />

        {isAdmin && (
          <div className="nv-detail-admin-actions">
            <Button type="button" variant="ghost" size="sm" onClick={handleEditMetadata}>
              <Pencil size={14} aria-hidden="true" />
              编辑元数据
            </Button>
            <Button type="button" variant="ghost" size="sm" onClick={handleRefreshMetadata} disabled={scraping}>
              <RefreshCw size={14} className={scraping ? 'animate-spin' : undefined} aria-hidden="true" />
              {scraping ? '重新刮削中' : '重新刮削'}
            </Button>
          </div>
        )}
      </div>

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
      />

      <div className="nv-detail-content-shell mx-auto w-full max-w-[var(--nv-content-max)] px-[var(--nv-page-gutter)] py-6">
        <div className="nv-detail-body-grid">
          <main className="nv-detail-main-column min-w-0">
            <DetailTabs
              value={activeTab}
              onChange={setActiveTab}
              ariaLabel="详情页章节导航"
              items={[
                { value: 'overview', label: '简介', panelId: 'detail-overview', tabId: 'detail-tab-overview' },
                { value: 'cast', label: '演职人员', panelId: 'detail-cast', tabId: 'detail-tab-cast' },
                { value: 'highlights', label: '片段', panelId: 'detail-highlights', tabId: 'detail-tab-highlights' },
                { value: 'tech', label: '技术规格', panelId: 'detail-tech', tabId: 'detail-tab-tech' },
                { value: 'subtitles', label: '字幕', panelId: 'detail-subtitles', tabId: 'detail-tab-subtitles' },
                { value: 'related', label: '相关推荐', panelId: 'detail-related', tabId: 'detail-tab-related' },
              ]}
            />

            <section id="detail-overview" className="nv-detail-content-section nv-detail-tab-panel" role="tabpanel" aria-labelledby="detail-tab-overview" hidden={activeTab !== 'overview'}>
              <MediaInfoSection media={media} playInfo={playInfo} persons={persons} />
              {id && <div className="nv-detail-tab-secondary-section"><CommentSection mediaId={id} /></div>}
            </section>

            <section id="detail-cast" className="nv-detail-content-section nv-detail-tab-panel" role="tabpanel" aria-labelledby="detail-tab-cast" hidden={activeTab !== 'cast'}>
              <CastGrid persons={persons} />
            </section>

            <section id="detail-highlights" className="nv-detail-content-section nv-detail-tab-panel" role="tabpanel" aria-labelledby="detail-tab-highlights" hidden={activeTab !== 'highlights'}>
              <MediaHighlightsPanel mediaId={media.id} isAdmin={isAdmin} />
            </section>

            <section id="detail-tech" className="nv-detail-content-section nv-detail-tab-panel" role="tabpanel" aria-labelledby="detail-tab-tech" hidden={activeTab !== 'tech'}>
              <MediaDetailTechOverview media={media} techSpecs={techSpecs} fileInfo={fileInfo} />
              <details className="nv-detail-tech-details">
                <summary>查看完整技术信息</summary>
                <div className="nv-detail-tech-details-body">
                  <MediaTechSpecs media={media} techSpecs={techSpecs} fileInfo={fileInfo} library={libraryInfo} playbackStats={playbackStats} loading={enhancedLoading} isAdmin={isAdmin} />
                </div>
              </details>
            </section>

            <section id="detail-subtitles" className="nv-detail-content-section nv-detail-tab-panel" role="tabpanel" aria-labelledby="detail-tab-subtitles" hidden={activeTab !== 'subtitles'}>
              {enhancedLoading && !hasSubtitleData ? (
                <div className="skeleton h-[220px] rounded-[var(--nv-radius-container)]" aria-label="正在加载字幕信息" />
              ) : hasSubtitleData ? (
                <div className="nv-detail-subtitle-tab">
                  <div className="nv-detail-subtitle-tab-copy">
                    <span className="nv-detail-subtitle-tab-eyebrow">SUBTITLES</span>
                    <DetailSectionTitle icon={<Captions size={17} />}>字幕</DetailSectionTitle>
                    <p>已检测到 {embeddedSubtitleCount + externalSubtitleCount} 个字幕来源，可查看内嵌与外挂字幕并管理文本字幕提取。</p>
                  </div>
                  <Button type="button" variant="secondary" size="sm" onClick={() => setShowSubtitleManager(true)}>管理字幕</Button>
                </div>
              ) : (
                <EmptyState className="nv-detail-tab-empty-state" icon={<Captions size={23} aria-hidden="true" />} title="暂无字幕" description="当前媒体暂未检测到内嵌字幕轨道或外挂字幕文件。" />
              )}
            </section>

            <section id="detail-related" className="nv-detail-content-section nv-detail-tab-panel" role="tabpanel" aria-labelledby="detail-tab-related" hidden={activeTab !== 'related'}>
              {media.media_type === 'movie' && id && (
                <div className="nv-detail-tab-secondary-section nv-detail-tab-secondary-section-first"><CollectionCarousel mediaId={id} /></div>
              )}
              <RecommendationCarousel recommendations={recommendations} />
            </section>
          </main>

          <MediaDetailSidebar media={media} playInfo={playInfo} techSpecs={techSpecs} fileInfo={fileInfo} playbackStats={playbackStats} isAdmin={isAdmin} onManageSubtitles={() => setShowSubtitleManager(true)} />
        </div>
      </div>

      {showTrailer && media.trailer_url && <TrailerModal trailerUrl={media.trailer_url} onClose={() => setShowTrailer(false)} />}

      {showMatchModal && (
        <MetadataMatchModal
          source={matchSource}
          onSourceChange={(source) => { setMatchSource(source); setMatchResults([]); setMatchSelectedId(null) }}
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

      {showSubtitleManager && <SubtitleManager mediaId={id!} mediaTitle={media.title} onClose={() => setShowSubtitleManager(false)} />}

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
