import DetailStatusStrip from '@/components/media/DetailStatusStrip'
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
  MediaDetailsSection,
  MediaDetailTechOverview,
  MediaSynopsisSection,
  MediaTechSpecs,
  RecommendationCarousel,
  TrailerModal,
} from '@/components/media'
import MediaHighlightsPanel from '@/components/media/MediaHighlightsPanel'
import MetadataMatchModal, { type MetadataMatchSource } from '@/components/media/MetadataMatchModal'
import CommentSection from '@/components/CommentSection'
import EditMetadataModal from '@/components/EditMetadataModal'
import SubtitleManager from '@/components/SubtitleManager'
import SubtitleSearchPanel from '@/components/SubtitleSearchPanel'
import ConfirmDialog from '@/components/design-system/ConfirmDialog'
import { Button, EmptyState } from '@/components/design-system'
import { bumpPosterVersion } from '@/stores/mediaRefresh'
import { useTranslation } from '@/i18n'
import { formatErrMsg } from '@/utils/error'
import { parseDirectMatchId } from '@/utils/parseDirectMatchId'
import { invalidateMediaListCaches } from '@/utils/invalidateMediaCaches'
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
  const [showSubtitleSearch, setShowSubtitleSearch] = useState(false)
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
      <div className="nv-media-detail-page nv-detail-loading" role="status" aria-label="影片详情加载中">
        <div className="skeleton nv-detail-toolbar-skeleton" aria-hidden="true" />
        <div className="skeleton nv-detail-hero-skeleton" aria-hidden="true" />
        <div className="nv-detail-content-shell space-y-4 px-[var(--nv-page-gutter)]" aria-hidden="true">
          <div className="skeleton h-44 rounded-[var(--nv-radius-card)]" />
          <div className="skeleton h-24 rounded-[var(--nv-radius-card)]" />
        </div>
      </div>
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
    <div className="nv-media-detail-page nv-detail-content-ready relative">
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

        <div className="nv-detail-breadcrumb" title={`找片 / ${breadcrumbLabel}`}>
          <span>找片</span>
          <span aria-hidden="true">/</span>
          <strong>{breadcrumbLabel}</strong>
        </div>

        <div className="nv-detail-toolbar-spacer" />


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

      <DetailStatusStrip media={media} watchProgress={watchProgress} isAdmin={isAdmin} />

      <div className="nv-detail-content-shell mx-auto w-full max-w-[var(--nv-content-max)] px-[var(--nv-page-gutter)] py-6">
        <div className="nv-detail-body-grid">
          <main className="nv-detail-main-column min-w-0">
            <MediaSynopsisSection media={media} />

            <section id="cast" className="nv-detail-content-section">
              <CastGrid persons={persons} />
            </section>

            <MediaDetailsSection media={media} persons={persons} />

            <section id="subtitles" className="nv-detail-content-section">
              {enhancedLoading && !hasSubtitleData ? (
                <div className="skeleton h-[220px] rounded-[var(--nv-radius-container)]" role="status" aria-label="正在加载字幕信息" />
              ) : hasSubtitleData ? (
                <div className="nv-detail-subtitle-tab">
                  <div className="nv-detail-subtitle-tab-copy">

                    <DetailSectionTitle icon={<Captions size={17} />}>字幕</DetailSectionTitle>
                    <p>已检测到 {embeddedSubtitleCount + externalSubtitleCount} 个字幕来源，播放时可以选择或关闭字幕。</p>
                  </div>
                  {isAdmin && <Button type="button" variant="secondary" size="sm" onClick={() => setShowSubtitleManager(true)}>管理字幕</Button>}
                </div>
              ) : (
                <EmptyState
                  className="nv-detail-tab-empty-state"
                  icon={<Captions size={23} aria-hidden="true" />}
                  title="暂无字幕"
                  description="当前媒体暂未检测到内嵌字幕轨道或外挂字幕文件。"
                  action={(
                    <div className="flex flex-wrap items-center justify-center gap-2">
                      <Button type="button" variant="primary" size="sm" onClick={() => setShowSubtitleSearch(true)}>{t('subtitle.search')}</Button>
                      {isAdmin && <Button type="button" variant="secondary" size="sm" onClick={() => setShowSubtitleManager(true)}>管理字幕</Button>}
                    </div>
                  )}
                />
              )}
            </section>

            <section id="recommendations" className="nv-detail-content-section space-y-6">
              {media.media_type === 'movie' && id && (
                <CollectionCarousel mediaId={id} />
              )}
              <RecommendationCarousel recommendations={recommendations} />
            </section>

            {id && (
              <section id="reviews" className="nv-detail-content-section">
                <CommentSection mediaId={id} />
              </section>
            )}
            <details className="nv-detail-tech-details mt-6">
              <summary>更多信息与片段</summary>
              <div className="nv-detail-tech-details-body space-y-6">
                <MediaHighlightsPanel mediaId={media.id} isAdmin={isAdmin} />
                <details className="nv-detail-tech-details">
                  <summary>技术信息</summary>
                  <div className="nv-detail-tech-details-body">
                    <MediaDetailTechOverview media={media} techSpecs={techSpecs} fileInfo={fileInfo} />
                    <MediaTechSpecs media={media} techSpecs={techSpecs} fileInfo={fileInfo} library={libraryInfo} playbackStats={playbackStats} loading={enhancedLoading} isAdmin={isAdmin} />
                  </div>
                </details>
              </div>
            </details>
          </main>

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

      {showSubtitleSearch && id && (
        <SubtitleSearchPanel
          mediaId={id}
          title={media.title}
          year={media.year}
          type={media.media_type}
          onClose={() => setShowSubtitleSearch(false)}
          onDownloaded={() => { void refreshMediaDetail(id) }}
        />
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
    </div>
  )
}
