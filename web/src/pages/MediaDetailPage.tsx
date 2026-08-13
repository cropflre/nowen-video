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
import { Button } from '@/components/design-system'
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
    setCWatchProgress(null)

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
          .then((response) => { if (!abortController.signal.aborted) setRecommendations(response.data.data || []))
          .catch(() => {})
        mediaApi.getPersons(mediaData.id)
          .then((response) => { if (!abortController.signal.aborted) setPersons(response.data.data || []) })
          .catch(() => {})
        userApi.getProgress(mediaData.id)
          .then((response) => { if (!abortController.signal.aborted) setWatchProgress(response.data.data)
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
      // åŒ¹é…å¾×æˆåŠŸæ—¶ï¼Œè¯¦æƒ…äºŒæ¬¡åˆ·æ–°å¤±è´¥ä¸é˜»æ–­ä¸»æµç¨‹ã€‚

    }
  }

  const handleMatchSearch = async () => {
    if (!matchQuery.trim()) return
    const direct = parseDirectMatchId(matchQuery, matchSource)
    if (direct) {
      if (direct.source !== matchSource) {
        setMatchSource(direct.source)
        toast.info(`e·²è®°åˆ«ä¸º ${{ tmdb: 'TMDb', douban: 'è±†ç“£', bangumi: 'Bangumi', thetvdb: 'TheTVDB' }[direct.source]} é“¾æ¥ï¼Œå·²è‡ªåŠ¨åˆ‡æ¢æ•°æ®æº`)
      }
      const idForApply: number | string = direct.source === 'douban' ? direct.id : Number(direct.id)
      setMatchResults([{
        id: idForApply,
        title: `ç›´è¾“ ç”¨æˆ·å ${direct.id}`,
        original_title: '',
        name: `ç›´è¾“ ç”¨æˆ·å ${direct.id}`,
        original_name: '',
        overview: `ç‚¹å‡»"åº”ç”¨æˆ·å" å°†æœ¬æ¡ç›®ç»‘å®šåˆ° ${{ tmdb: 'TMDb', douban: 'è‚†åŠ¢': ç”¨æˆ·å ${direct.id}ã€‚à,
        release_date: '',
        first_air_date: '',
        vote_average: 0,
        poster_path: '',
      }])
      setMatchSelectedId(idForApply)
      toast.success(`å·²è¯†åˆ« ${{ tmdb: 'TMDb', douban: 'è‚†åŠ¢': ç”¨æˆ·å ${direct.id}ï¼Œç‚¹å‡»"åº”ç”¨"å³å¯ç»‘å®šâ€¦&
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
        const subjectType = (media?.genres || '').includes('åŠ¨ç”»') ? 2 : 6
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
      toast.info('TheTVDB ä¸»è¦ç”¨äºå‰¦é›†é›…è½¦åŒ¹	ÊBˆ™]\›‚ˆBˆÙ]X]ÚÙ[XİYY
™\İ[Y
BˆB‚ˆÛÛœİ[™SX]Ú\HH\Ş[˜È

HOˆÂˆYˆ
ZYX]ÚÙ[XİYYOOH[
H™]\›‚ˆÙ]X]Ú\Z[™ÊYJBˆHÂˆÛÛœİÛİ\˜ÙS˜[YSX\ˆ™XÛÜ™İš[™Ëİš[™ÏˆHÂˆYˆ	ÕQ‰Ëˆ˜[™İ[ZNˆ	Ğ˜[™İ[ZIËˆİX˜[ˆ	ú ¡¹b¨‰Ëˆ]™ˆ	ÕU‘‰ËˆBˆYˆ
X]ÚÛİ\˜ÙHOOH	İY‰ÊH]ØZ]YZ[\K›X]ÚY]Y]JYX]ÚÙ[XİYY\È[X™\ŠBˆ[ÙHYˆ
X]ÚÛİ\˜ÙHOOH	ÙİX˜[‰ÊH]ØZ]YZ[\K›X]ÚYYXQİX˜[ŠYX]ÚÙ[XİYY\Èİš[™ÊBˆ[ÙHYˆ
X]ÚÛİ\˜ÙHOOH	Ø˜[™İ[ZIÊH]ØZ]YZ[\K›X]ÚYYXP˜[™İ[ZJYX]ÚÙ[XİYY\È[X™\ŠBˆ[ÙHÂˆØ\İš[™›Ê	ÕU‘ˆ9..ú) yå*9.£¹biúfá¹£k¹c.IÊBˆ™]\›‚ˆB‚ˆ]ØZ]™Yœ™\ÚYYXQ]Z[
Y
BˆÙ]Üİ\•™\œÚ[ÛŠ]K››İÊ
JBˆ[\Üİ\•™\œÚ[ÛŠ
Bˆ[˜[Y]SYYXS\İØXÚ\Ê
BˆÙ]ÚİÓX]Ú[Ù[
˜[ÙJBˆÙ]X]ÚÙ[XİYY
[
BˆØ\İœİXØÙ\ÜÊ
	ÛYYXQ]Z[›X]ÚİXØÙ\ÜÉËÈÛİ\˜ÙNˆÛİ\˜ÙS˜[YSX\ÛX]ÚÛİ\˜ÙWHJJBˆHØ]Ú
\œ›ÜŠHÂˆØ\İ™\œ›ÜŠ›Ü›X]\œ“\ÙÊ\œ›Ü‹
	ÛYYXQ]Z[›X]Ú˜Z[Y	ÊJJBˆHš[˜[HÂˆÙ]X]Ú\Z[™Ê˜[ÙJBˆBˆB‚ˆÛÛœİ[™U[›X]ÚH\Ş[˜È

HOˆÂˆYˆ
ZY
H™]\›‚ˆHÂˆ]ØZ]YZ[\K[›X]ÚY]Y]JY
BˆÛÛœİ™\ÜÛœÙHH]ØZ]YYXP\K™]Z[
Y
BˆÙ]YYXJ™\ÜÛœÙK™]K™]JBˆÙ]Üİ\•™\œÚ[ÛŠ]K››İÊ
JBˆ[˜[Y]SYYXS\İØXÚ\Ê
BˆÙ]ÚİÕ[›X]ÚÛÛ™š\›J˜[ÙJBˆØ\İœİXØÙ\ÜÊ
	ÛYYXQ]Z[[›X]ÚİXØÙ\ÜÉÊJBˆHØ]ÚÂˆØ\İ™\œ›ÜŠ
	ÛYYXQ]Z[[›X]Ú˜Z[Y	ÊJBˆBˆB‚ˆÛÛœİ[™T™Yœ™\ÚY]Y]HH\Ş[˜È

HOˆÂˆYˆ
ZY
H™]\›‚ˆÙ]ØÜ˜\[™ÊYJBˆHÂˆ]ØZ]YYXP\KœØÜ˜\JY
BˆÛÛœİ™\ÜÛœÙHH]ØZ]YYXP\K™]Z[
Y
BˆÙ]YYXJ™\ÜÛœÙK™]K™]JBˆÙ]Üİ\•™\œÚ[ÛŠ]K››İÊ
JBˆ[˜[Y]SYYXS\İØXÚ\Ê
BˆØ\İœİXØÙ\ÜÊ
	ÛYYXQ]Z[œ™Yœ™\ÚİXØÙ\ÜÉÊJBˆHØ]Ú
\œ›ÜŠHÂˆØ\İ™\œ›ÜŠ›Ü›X]\œ“\ÙÊ\œ›Ü‹
	ÛYYXQ]Z[œ™Yœ™\Ú˜Z[Y	ÊJJBˆHš[˜[HÂˆÙ]ØÜ˜\[™Ê˜[ÙJBˆBˆB‚ˆÛÛœİ[™QY]Y]Y]HH

HOˆÂˆYˆ
[YYXJH™]\›‚ˆÙ]Y]›Ü›JÂˆ]NˆYYXK]H	ÉËˆÜšY×İ]NˆYYXK›ÜšY×İ]H	ÉËˆYX\ˆYYXKYX\ˆˆİ™\šY]ÎˆYYXK›İ™\šY]È	ÉËˆ˜][™ÎˆYYXKœ˜][™ÈˆÙ[œ™\ÎˆYYXK™Ù[œ™\È	ÉËˆÛİ[NˆYYXK˜Ûİ[H	ÉËˆ[™İXYÙNˆYYXK›[™İXYÙH	ÉËˆYÛ[™NˆYYXKYÛ[™H	ÉËˆİY[ÎˆYYXKœİY[È	ÉËˆJBˆÙ]ÚİÑY][Ù[
YJBˆB‚ˆÛÛœİ[™QY]Ø]™HH\Ş[˜È

HOˆÂˆYˆ
ZY
H™]\›‚ˆHÂˆ]ØZ]YZ[\K\]SYYXSY]Y]JYY]›Ü›JBˆÛÛœİ™\ÜÛœÙHH]ØZ]YYXP\K™]Z[
Y
BˆÙ]YYXJ™\ÜÛœÙK™]K™]JBˆÙ]Üİ\•™\œÚ[ÛŠ]K››İÊ
JBˆ[˜[Y]SYYXS\İØXÚ\Ê
BˆÙ]ÚİÑY][Ù[
˜[ÙJBˆØ\İœİXØÙ\ÜÊ
	ÛYYXQ]Z[™Y]İXØÙ\ÜÉÊJBˆHØ]ÚÂˆØ\İ™\œ›ÜŠ
	ÛYYXQ]Z[™Y]˜Z[Y	ÊJBˆBˆB‚ˆÛÛœİ[™Q[]HH\Ş[˜È

HOˆÂˆYˆ
ZY
H™]\›‚ˆHÂˆ]ØZ]YZ[\K™[]SYYXJY
Bˆ[˜[Y]SYYXS\İØXÚ\Ê
BˆØ\İœİXØÙ\ÜÊ
	ÛYYXQ]Z[™[]TİXØÙ\ÜÉÊJBˆ˜]šYØ]JLJBˆHØ]ÚÂˆØ\İ™\œ›ÜŠ
	ÛYYXQ]Z[™[]Q˜Z[Y	ÊJBˆBˆB‚ˆYˆ
ØY[™È[YYXJHÂˆ™]\›ˆ
ˆ]ˆÛ\ÜÓ˜[YOHœÜXÙK^KMˆˆ\šXK[X™[H¹j¤¹/dú+é¹ áyb¨:/oy.+H‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆVÍŒH›İ[™YVİ˜\ŠK[‹\˜Y]\ËZ\›ÊWHˆÏ‚ˆ]ˆÛ\ÜÓ˜[YOH›^X]]È›^ËY[X^]ËVİ˜\ŠK[‹XÛÛ[[X^
WHØ\MˆVİ˜\ŠK[‹\YÙKYİ]\ŠWHM‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆY[ˆMÌˆËM›İ[™YVİ˜\ŠK[‹\˜Y]\ËXØ\™
WHÛN˜›ØÚÈˆÏ‚ˆ]ˆÛ\ÜÓ˜[YOH™›^LHÜXÙK^KM‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆLLËL‹ÌÈ›İ[™YVİ˜\ŠK[‹\˜Y]\ËXÛÛ›Û
WHˆÏ‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆMHËLKÌÈ›İ[™YVİ˜\ŠK[‹\˜Y]\ËXÛÛ›Û
WHˆÏ‚ˆ]ˆÛ\ÜÓ˜[YOH™›^Ø\LÈ‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆLLËL›İ[™YVİ˜\ŠK[‹\˜Y]\ËXÛÛ›Û
WHˆÏ‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆLLËL›İ[™YVİ˜\ŠK[‹\˜Y]\ËXÛÛ›Û
WHˆÏ‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆLLËL›İ[™YVİ˜\ŠK[‹\˜Y]\ËXÛÛ›Û
WHˆÏ‚ˆÙ]‚ˆ]ˆÛ\ÜÓ˜[YOHœÚÙ[]ÛˆLŒËY[›İ[™YVİ˜\ŠK[‹\˜Y]\ËXÛÛ›Û
WHˆÏ‚ˆÙ]‚ˆÙ]‚ˆÙ]‚ˆ
BˆB‚ˆÛÛœİ\ĞYZ[ˆH\Ù\Ëœ›ÛHOOH	ØYZ[‰Â‚ˆ™]\›ˆ
ˆ]ˆÛ\ÜÓ˜[YOHœ™[]]™H‚ˆ]Û‚ˆ\OH˜]Ûˆ‚ˆ˜\šX[H™ÚÜİ‚ˆÚ^™OHœÛH‚ˆXÛÛ“Û›BˆÛÛXÚÏ^Ê
HOˆÂˆYˆ
Ú[™İËš\İÜK›[™İˆJH˜]šYØ]JLJBˆ[ÙH˜]šYØ]J	ËÉÊBˆ_Bˆ\šXK[X™[Hº/å9fïH‚ˆ]OHº/å9fçˆ‚ˆÛ\ÜÓ˜[YOH˜XœÛÛ]HYVİ˜\ŠK[‹\YÙKYİ]\ŠWHÜM‹LÌ›Ü™\ˆ›Ü™\‹Vİ˜\ŠK[‹X›Ü™\‹\İXJWH™ËVİ˜\ŠK[‹X™Ë\İ\™˜XÙK\ÛÙ
WH^Vİ˜\ŠK[‹]^\ÙXÛÛ™\JWHÚYİËVİ˜\ŠK[‹\ÚYİËXØ\™
WH‚ˆ‚ˆ\œ›İÓYÚ^™O^ÌNH\šXKZY[HYHˆÏ‚ˆĞ]Û‚‚ˆ\›ÔÙXİ[Û‚ˆYYXO^ÛYYX_Bˆ^R[™›Ï^Ü^R[™›ßBˆ\Ñ˜]›Üš]Y^Ú\Ñ˜]›Üš]YBˆØ]Ú›ÙÜ™\ÜÏ^İØ]Ú›ÙÜ™\ÜßBˆ^[\İÏ^Ü^[\İßBˆØÜ˜\[™Ï^ÜØÜ˜\[™ßBˆ\ĞYZ[^Ú\ĞYZ[ŸBˆÜİ\•™\œÚ[Û^ÜÜİ\•™\œÚ[ÛŸBˆÛ‘˜]›Üš]O^Ú[™Q˜]›Üš]_BˆÛ”ØÜ˜\O^Ú[™TØÜ˜\_BˆÛYÔ^[\İ^Ú[™PYÔ^[\İBˆÛ”ÚİÕ˜Z[\^ÛYYXK˜Z[\—İ\›È

HOˆÙ]ÚİÕ˜Z[\ŠYJHˆ[™Yš[™YBˆÛ“X[X[X]Ú^Ú[™SX[X[X]ÚBˆÛ•[›X]Ú^Ê
HOˆÙ]ÚİÕ[›X]ÚÛÛ™š\›JYJ_BˆÛ”™Yœ™\ÚY]Y]O^Ú[™T™Yœ™\ÚY]Y]_BˆÛ‘Y]Y]Y]O^Ú[™QY]Y]Y]_BˆÛ‘[]O^Ê
HOˆÙ]ÚİÑ[]PÛÛ™š\›JYJ_BˆÛ”™\›ØÙ\ÜÏ^Ê
HOˆÂˆYZ[\KœİX›Z]™\›ØÙ\ÜÊYJK[Š

HOˆØ\İœİXØÙ\ÜÊ	ùmì¹£ä9.¢:h¡9i!9ä!¹.îùb¨IÊJK˜Ø]Ú


HOˆØ\İ™\œ›ÜŠ	ù¤ä9.©:h¡9i!9ä!¹i,yb¨yi,z-)IÊJBˆ_BˆÛ•˜[œØÛÙO^Ê
HOˆÂˆYZ[\KœİX›Z]˜[œØÛÙJYJK[Š

HOˆØ\İœİXØÙ\ÜÊ	ùmìù£ä9.©ùo.¹b-º/k9è ù.îùb¨IÊJK˜Ø]Ú


HOˆØ\İ™\œ›ÜŠ	ù£ä9.©:/k9è yi,z-)IÊJBˆ_BˆÏ‚‚ˆ]ˆÛ\ÜÓ˜[YOH›^X]]ÈËY[X^]ËVİ˜\ŠK[‹XÛÛ[[X^
WHÜXÙK^KNVİ˜\ŠK[‹\YÙKYİ]\ŠWHKN‚ˆYYXR[™›ÔÙXİ[ÛˆYYXO^ÛYYX_H^R[™›Ï^Ü^R[™›ßH\œÛÛœÏ^Ü\œÛÛœßHÏ‚ˆØ\İÜšY\œÛÛœÏ^Ü\œÛÛœßHÏ‚‚ˆÛYYXK›YYXWİ\HOOH	Û[İšYIÈ	‰ˆY	‰ˆÛÛXİ[ÛØ\›İ\Ù[YYXRY^ÚYHÏŸB‚ˆYYXUXÚÜXÜÂˆYYXO^ÛYYX_BˆXÚÜXÜÏ^İXÚÜXÜßBˆš[R[™›Ï^Ùš[R[™›ßBˆXœ˜\O^ÛXœ˜\R[™›ßBˆ^X˜XÚÔİ]Ï^Ü^X˜XÚÔİ]ßBˆØY[™Ï^Ù[š[˜ÙYØY[™ßBˆ\ĞYZ[^Ú\ĞYZ[ŸBˆÏ‚‚ˆÚ\ĞYZ[ˆ	‰ˆ
ˆÙXİ[ÛˆÛ\ÜÓ˜[YOH™›^›^XÛÛØ\LÈ›Ü™\‹^H›Ü™\‹Vİ˜\ŠK[‹X›Ü™\‹\İXJWHKMÛN™›^\›İÈÛNš][\ËXÙ[\ˆÛNš\İYKX™]ÙY[ˆ‚ˆ]ˆÛ\ÜÓ˜[YOH™›^Z[‹]ËL][\Ë\İ\Ø\LÈ‚ˆ]ˆÛ\ÜÓ˜[YOH™ÜšYNËNÚš[šËLXÙKZ][\ËXÙ[\ˆ›İ[™YVİ˜\ŠK[‹\˜Y]\ËXÛÛ›Û
WH™ËVİ˜\ŠK[‹Yš[Zİ™\ŠWH^Vİ˜\ŠK[‹]^]\X\JWH‚ˆØ\[ÛœÈÚ^™O^ÌMŸH\šXKZY[HYHˆÏ‚ˆÙ]‚ˆ]ˆÛ\ÜÓ˜[YOH›Z[‹]ËL‚ˆÈÛ\ÜÓ˜[YOH^\ÛH›Û[YY][H^Vİ˜\ŠK[‹]^\š[X\JWH¹keùney¥l9£k¹ë¨yä!ÚÏ‚ˆÛ\ÜÓ˜[YOH›]LH^^ÈXY[™ËMH^Vİ˜\ŠK[‹]^]\X\JWH¹§éyàçùa¡ymeÈ9i%¹c+9keùne{ï#9¢nyi#y£ä9cåùnm¹kï9aî¹¥¡ù§+9keùnexà ¸ .OÜ‚ˆÙ]‚ˆÙ]‚ˆ]Ûˆ\OH˜]Ûˆˆ˜\šX[H™ÚÜİˆÛÛXÚÏ^Ê
HOˆÙ]ÚİÔİX]SX[˜YÙ\ŠYJ_O‚ˆØ\[ÛœÈÚ^™O^ÌM_H\šXKZY[HYHˆÏˆ9ë¨yä!¹keùneBˆĞ]Û‚ˆÜÙXİ[Û‚ˆ
_B‚ˆ™XÛÛ[Y[™][ÛØ\›İ\Ù[™XÛÛ[Y[™][ÛœÏ^Ü™XÛÛ[Y[™][ÛœßHÏ‚ˆÚY	‰ˆÛÛ[Y[ÙXİ[ÛˆYYXRY^ÚYHÏŸBˆÙ]‚‚ˆÜÚİÕ˜Z[\ˆ	‰ˆYYXK˜Z[\—İ\›	‰ˆ
ˆ˜Z[\“[Ù[˜Z[\•\›^ÛYYXK˜Z[\—İ\›HÛÛÜÙO^Ê
HOˆÙ]ÚİÕ˜Z[\Š˜[ÙJ_HÏ‚ˆ
_B‚ˆÜÚİÓX]Ú[Ù[	‰ˆ
ˆY]Y]SX]Ú[Ù[ˆÛİ\˜ÙO^ÛX]ÚÛİ\˜Ù_BˆÛ”Ûİ\˜ÙPÚ[™ÙO^ÊÛİ\˜ÙJHOˆÂˆÙ]X]ÚÛİ\˜ÙJÛİ\˜ÙJBˆÙ]Y]Ú™\İ[Ê×JBˆÙ]X]ÚÙ[XİYY
[
Bˆ_Bˆ]Y\O^ÛX]Ú]Y\_BˆÙ]]Y\O^ÜÙ]X]Ú]Y\_Bˆ™\İ[Ï^ÛX]Ú™\İ[ßBˆÙX\˜Ú[™Ï^ÛX]ÚÙX\˜Ú[™ßBˆÙ[XİYY^ÛX]ÚÙ[XİYYBˆ\Z[™Ï^ÛX]Ú\Z[™ßBˆÛ”ÙX\˜Ú^Ú[™SX]ÚÙX\˜ÚBˆÛ”Ù[Xİ^Ú[™SX]ÚÙ[XİBˆÛ\O^Ú[™SX]Ú\_BˆÛÛÜÙO^Ê
HOˆÙ]ÚİÓX]Ú[Ù[
˜[ÙJ_BˆÏ‚ˆ
_B‚ˆÜÚİÕ[›X]ÚÛÛ™š\›H	‰ˆ
ˆÛÛ™š\›QX[ÙÂˆ]O^İ
	ÛYYXQ]Z[[›X]Ú]IÊ_Bˆ\ØÜš\[Û^İ
	ÛYYXQ]Z[[›X]Ú\ØÉÊ_BˆÛÛ™š\›SX™[^İ
	ÛYYXQ]Z[[›X]ÚÛÛ™š\›IÊ_BˆØ[˜Ù[X™[^İ
	ØÛÛ[[Û‹˜Ø[˜Ù[	Ê_BˆÛ™OHØ\›š[™È‚ˆÛÛÛ™š\›O^Ú[™U[›X]ÚBˆÛÛÜÙO^Ê
HOˆÙ]ÚİÕ[›X]ÚÛÛ™š\›J˜[ÙJ_BˆÏ‚ˆ
_B‚ˆÜÚİÑY][Ù[	‰ˆ
ˆY]Y]Y]S[Ù[ˆ\OH›YYXH‚ˆY^ÚY_BˆY’Y^ÛYYXKY—ÚYBˆYYXU\O^ÛYYXK›YYXWİ\HOOH	Ù\\ÛÙIÈÈ	İ‰Èˆ	Û[İšYIßBˆY]›Ü›O^ÙY]›Ü›_BˆÙ]Y]›Ü›O^ÜÙ]Y]›Ü›_Bˆİ\œ™[Üİ\^Üİ™X[P\K™Ù]Üİ\•\›
YYXKšYÜİ\•™\œÚ[ÛŠ_Bˆ\ÔÜİ\^ÈH[YYXKœÜİ\—Ü]Bˆ\Ğ˜XÚÙ›Ü^ÈH[YYXK˜˜XÚÙ›ÜÜ]BˆÛ”Ø]™O^Ú[™QY]Ø]™_BˆÛÛÜÙO^Ê
HOˆÙ]ÚİÑY][Ù[
˜[ÙJ_Bˆ\ÕYÛ[™BˆÏ‚ˆ
_B‚ˆÜÚİÔİX]SX[˜YÙ\ˆ	‰ˆ
ˆİX]SX[˜YÙ\ˆYYXRY^ÚY_HYYXU]O^ÛYYXK]_HÛÛÜÙO^Ê
HOˆÙ]ÚİÔİX]SX[˜YÙ\Š˜[ÙJ_HÏ‚ˆ
_B‚ˆÜÚİÑ[]PÛÛ™š\›H	‰ˆ
ˆÛÛ™š\›QX[ÙÂˆ]O^İ
	ÛYYXQ]Z[™[]U]IÊ_Bˆ\ØÜš\[Û^İ
	ÛYYXQ]Z[™[]Q\ØÉÊ_Bˆ[^İ
	ÛYYXQ]Z[™[]R[	Ê_BˆÛÛ™š\›SX™[^İ
	ÛYYXQ]Z[™[]PÛÛ™š\›IÊ_BˆØ[˜Ù[X™[^İ
	ØÛÛ[[Û‹˜Ø[˜Ù[	Ê_BˆÛ™OH™[™Ù\ˆ‚ˆÛÛÛ™š\›O^Ú[™Q[]_BˆÛÛÜÙO^Ê
HOˆÙ]ÚİÑ[]PÛÛ™š\›J˜[ÙJ_BˆÏ‚ˆ
_BˆÙ]‚ˆ
BŸB