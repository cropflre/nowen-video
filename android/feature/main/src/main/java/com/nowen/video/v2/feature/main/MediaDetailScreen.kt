package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.DeleteOutline
import androidx.compose.material.icons.filled.ExpandLess
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.Message
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Send
import androidx.compose.material.icons.filled.Star
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.MobileWebParityRepository
import com.nowen.video.v2.core.data.OfflineDownloadRepository
import com.nowen.video.v2.core.data.ProgressRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SeriesRepository
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.HillsPrimaryAction
import com.nowen.video.v2.core.designsystem.HillsSecondaryAction
import com.nowen.video.v2.core.designsystem.HillsState
import com.nowen.video.v2.core.designsystem.HillsTextField
import com.nowen.video.v2.core.model.CollectionWithMedia
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaComment
import com.nowen.video.v2.core.model.MediaCommentList
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.MediaHighlight
import com.nowen.video.v2.core.model.MediaPerson
import com.nowen.video.v2.core.model.OfflineDownloadRecord
import com.nowen.video.v2.core.model.OfflineDownloadStatus
import com.nowen.video.v2.core.model.SeasonInfo
import com.nowen.video.v2.core.model.SeriesInfo
import com.nowen.video.v2.core.model.SubtitleTracksResponse
import com.nowen.video.v2.core.model.episodeDisplayName
import com.nowen.video.v2.core.model.userEpisodeTitle
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class MediaDetailUiState(
    val loading: Boolean = true,
    val media: MediaDetail? = null,
    val persons: List<MediaPerson> = emptyList(),
    val collection: CollectionWithMedia? = null,
    val subtitles: SubtitleTracksResponse = SubtitleTracksResponse(),
    val highlights: List<MediaHighlight> = emptyList(),
    val recommendations: List<MediaCard> = emptyList(),
    val comments: MediaCommentList = MediaCommentList(),
    val commentsLoading: Boolean = false,
    val commentActionRunning: Boolean = false,
    val commentMessage: String? = null,
    val resumePositionSeconds: Double = 0.0,
    val favorite: Boolean = false,
    val favoriteActionRunning: Boolean = false,
    val favoriteMessage: String? = null,
    val download: OfflineDownloadRecord? = null,
    val downloadActionRunning: Boolean = false,
    val downloadMessage: String? = null,
    val episodeSeries: SeriesInfo? = null,
    val episodeSeason: SeasonInfo? = null,
    val error: String? = null,
) {
    val hasResumeProgress: Boolean
        get() = resumePositionSeconds >= 5.0
}

@HiltViewModel
class MediaDetailViewModel @Inject constructor(
    private val repository: CatalogRepository,
    private val seriesRepository: SeriesRepository,
    private val socialRepository: SocialCatalogRepository,
    private val parityRepository: MobileWebParityRepository,
    private val progressRepository: ProgressRepository,
    private val offlineDownloads: OfflineDownloadRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(MediaDetailUiState())
    val state: StateFlow<MediaDetailUiState> = _state
    private var loadedId: String? = null

    init {
        viewModelScope.launch {
            offlineDownloads.downloads.collectLatest { downloads ->
                val mediaId = loadedId
                _state.update { current ->
                    current.copy(download = downloads.firstOrNull { it.mediaId == mediaId })
                }
            }
        }
    }

    fun load(id: String) {
        if (loadedId == id && _state.value.media != null) return
        loadedId = id
        viewModelScope.launch {
            val currentDownload = _state.value.download
            _state.value = MediaDetailUiState(loading = true, download = currentDownload)
            runCatching {
                coroutineScope {
                    val mediaDeferred = async { repository.detail(id).getOrThrow() }
                    val favorite = async { socialRepository.favoriteStatus(id).getOrDefault(false) }
                    val persons = async { socialRepository.mediaPersons(id).getOrDefault(emptyList()) }
                    val collection = async { socialRepository.mediaCollection(id).getOrNull() }
                    val subtitles = async { repository.subtitles(id).getOrDefault(SubtitleTracksResponse()) }
                    val highlights = async {
                        parityRepository.highlights(id).getOrNull()?.highlights.orEmpty()
                    }
                    val recommendations = async {
                        parityRepository.similar(id, 12).getOrDefault(emptyList())
                    }
                    val comments = async {
                        parityRepository.comments(id, page = 1, size = 10).getOrDefault(MediaCommentList())
                    }
                    val resume = async {
                        val media = mediaDeferred.await()
                        val duration = media.duration.takeIf { it > 0.0 }
                            ?: media.runtime.takeIf { it > 0 }?.times(60.0)
                            ?: 0.0
                        progressRepository.restorePosition(id, duration)
                    }
                    val media = mediaDeferred.await()
                    val episodeBundle = if (media.mediaType.equals("episode", ignoreCase = true) && media.seriesId.isNotBlank()) {
                        seriesRepository.load(media.seriesId).getOrNull()
                    } else {
                        null
                    }
                    val directPersons = persons.await()
                    RelatedMediaDetail(
                        media = media,
                        episodeSeries = episodeBundle?.series,
                        episodeSeason = episodeBundle?.seasons?.firstOrNull { it.seasonNumber == media.seasonNumber },
                        favorite = favorite.await(),
                        persons = selectDetailPersons(directPersons, episodeBundle?.persons.orEmpty()),
                        collection = collection.await(),
                        subtitles = subtitles.await(),
                        highlights = highlights.await(),
                        recommendations = recommendations.await(),
                        comments = comments.await(),
                        resumePositionSeconds = resume.await(),
                    )
                }
            }.onSuccess { result ->
                _state.update {
                    it.copy(
                        loading = false,
                        media = result.media,
                        episodeSeries = result.episodeSeries,
                        episodeSeason = result.episodeSeason,
                        favorite = result.favorite,
                        persons = result.persons,
                        collection = result.collection,
                        subtitles = result.subtitles,
                        highlights = result.highlights,
                        recommendations = result.recommendations,
                        comments = result.comments,
                        resumePositionSeconds = result.resumePositionSeconds,
                        error = null,
                    )
                }
            }.onFailure { error ->
                _state.update { it.copy(loading = false, error = error.message ?: "详情加载失败") }
            }
        }
    }

    fun refreshComments() {
        val mediaId = loadedId ?: return
        viewModelScope.launch {
            _state.update { it.copy(commentsLoading = true, commentMessage = null) }
            parityRepository.comments(mediaId, page = 1, size = 10)
                .onSuccess { comments ->
                    _state.update { it.copy(comments = comments, commentsLoading = false) }
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(
                            commentsLoading = false,
                            commentMessage = error.message ?: "评价加载失败",
                        )
                    }
                }
        }
    }

    fun createComment(content: String, rating: Int?) {
        val mediaId = loadedId ?: return
        if (content.isBlank() || _state.value.commentActionRunning) return
        viewModelScope.launch {
            _state.update { it.copy(commentActionRunning = true, commentMessage = null) }
            parityRepository.createComment(mediaId, content, rating)
                .onSuccess {
                    parityRepository.comments(mediaId, page = 1, size = 10)
                        .onSuccess { comments ->
                            _state.update {
                                it.copy(
                                    comments = comments,
                                    commentsLoading = false,
                                    commentActionRunning = false,
                                    commentMessage = "评价已发表",
                                )
                            }
                        }
                        .onFailure {
                            _state.update {
                                it.copy(commentActionRunning = false, commentMessage = "评价已发表，刷新后可查看")
                            }
                        }
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(
                            commentActionRunning = false,
                            commentMessage = error.message ?: "发表评价失败",
                        )
                    }
                }
        }
    }

    fun deleteComment(commentId: String) {
        val mediaId = loadedId ?: return
        if (commentId.isBlank()) return
        viewModelScope.launch {
            _state.update { it.copy(commentActionRunning = true, commentMessage = null) }
            parityRepository.deleteComment(commentId)
                .onSuccess {
                    parityRepository.comments(mediaId, page = 1, size = 10)
                        .onSuccess { comments ->
                            _state.update {
                                it.copy(
                                    comments = comments,
                                    commentActionRunning = false,
                                    commentMessage = "评价已删除",
                                )
                            }
                        }
                        .onFailure {
                            _state.update { it.copy(commentActionRunning = false, commentMessage = "评价已删除") }
                        }
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(
                            commentActionRunning = false,
                            commentMessage = error.message ?: "删除评价失败",
                        )
                    }
                }
        }
    }

    fun toggleFavorite() {
        val mediaId = loadedId ?: return
        if (_state.value.favoriteActionRunning) return
        val desired = !_state.value.favorite
        viewModelScope.launch {
            _state.update { it.copy(favoriteActionRunning = true, favoriteMessage = null) }
            socialRepository.setFavorite(mediaId, desired)
                .onSuccess {
                    _state.update {
                        it.copy(
                            favorite = desired,
                            favoriteActionRunning = false,
                            favoriteMessage = if (desired) "已加入收藏" else "已取消收藏",
                        )
                    }
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(
                            favoriteActionRunning = false,
                            favoriteMessage = error.message ?: "收藏操作失败",
                        )
                    }
                }
        }
    }

    fun toggleDownload() {
        val mediaId = loadedId ?: return
        if (_state.value.downloadActionRunning) return
        val current = _state.value.download
        viewModelScope.launch {
            _state.update { it.copy(downloadActionRunning = true, downloadMessage = null) }
            val result: Result<Unit> = when (current?.status) {
                null -> offlineDownloads.enqueue(mediaId).map { Unit }
                OfflineDownloadStatus.Queued,
                OfflineDownloadStatus.Downloading,
                -> offlineDownloads.pause(current.id)
                OfflineDownloadStatus.Paused -> offlineDownloads.resume(current.id)
                OfflineDownloadStatus.Failed -> offlineDownloads.retry(current.id)
                OfflineDownloadStatus.Completed -> Result.success(Unit)
            }
            result
                .onSuccess {
                    _state.update {
                        it.copy(
                            downloadActionRunning = false,
                            downloadMessage = downloadActionMessage(current?.status),
                        )
                    }
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(
                            downloadActionRunning = false,
                            downloadMessage = error.message ?: "下载操作失败",
                        )
                    }
                }
        }
    }
}

internal fun selectDetailPersons(
    directPersons: List<MediaPerson>,
    seriesPersons: List<MediaPerson>,
): List<MediaPerson> = directPersons.ifEmpty { seriesPersons }

private data class RelatedMediaDetail(
    val media: MediaDetail,
    val episodeSeries: SeriesInfo?,
    val episodeSeason: SeasonInfo?,
    val favorite: Boolean,
    val persons: List<MediaPerson>,
    val collection: CollectionWithMedia?,
    val subtitles: SubtitleTracksResponse,
    val highlights: List<MediaHighlight>,
    val recommendations: List<MediaCard>,
    val comments: MediaCommentList,
    val resumePositionSeconds: Double,
)

/** 与 Web 移动端详情页保持相同的信息流与展开交互。 */
@Composable
fun MediaDetailScreen(
    mediaId: String,
    onBack: () -> Unit,
    onPlay: (String) -> Unit,
    onHighlightPlay: (String, Double) -> Unit = { id, _ -> onPlay(id) },
    onPersonClick: (String) -> Unit,
    onCollectionClick: (String) -> Unit,
    onMediaClick: (String) -> Unit = onPlay,
    onSeriesClick: (String) -> Unit = {},
    viewModel: MediaDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var highlightsExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    var castExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    var episodeMoreExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    var subtitlesExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    var commentsExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    var commentText by rememberSaveable(mediaId) { mutableStateOf("") }
    var commentRating by rememberSaveable(mediaId) { mutableIntStateOf(0) }
    LaunchedEffect(mediaId) { viewModel.load(mediaId) }

    Box(modifier = Modifier.fillMaxSize()) {
        when {
            state.loading -> CircularProgressIndicator(Modifier.align(Alignment.Center))
            state.error != null -> HillsState(
                title = "无法打开详情",
                message = state.error!!,
                actionLabel = "返回",
                onAction = onBack,
                modifier = Modifier.align(Alignment.Center),
            )
            state.media != null -> {
                val media = state.media!!
                val baseUrl = session.activeServer?.baseUrl
                LazyColumn(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(
                            if (media.mediaType.equals("episode", ignoreCase = true)) {
                                MaterialTheme.colorScheme.background
                            } else {
                                MoviePageBackground
                            },
                        ),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 28.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    val isEpisode = media.mediaType.equals("episode", ignoreCase = true)
                    val episodeSeries = state.episodeSeries
                    val heroTitle = if (isEpisode) episodeSeries?.displayTitle ?: media.title else media.displayTitle
                    val heroOriginalTitle = if (isEpisode) media.episodeTitle else media.originalTitle
                    val heroMetadata = if (isEpisode) {
                        listOfNotNull(
                            episodeCode(media),
                            media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
                            media.year.takeIf { it > 0 }?.toString(),
                            media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
                        ).joinToString(" · ")
                    } else {
                        mediaMetadataLabel(media)
                    }
                    item {
                        if (isEpisode) {
                            MobileDetailHero(
                                title = heroTitle,
                                originalTitle = heroOriginalTitle,
                                metadata = heroMetadata,
                                overview = media.overview,
                                backdropUrl = resolveImage(baseUrl, media.backdropPath),
                                posterUrl = resolveImage(baseUrl, media.posterPath),
                                primaryActionLabel = if (state.hasResumeProgress) {
                                    "继续播放 · ${formatResumeTime(state.resumePositionSeconds)}"
                                } else {
                                    "播放"
                                },
                                onPrimaryAction = { onPlay(media.id) },
                                onBack = onBack,
                                logoUrl = episodeSeries?.let { seriesLogoUrl(baseUrl, it.id) },
                            ) {
                                if (episodeSeries?.id?.isNotBlank() == true) {
                                    HillsSecondaryAction(
                                        label = "剧集详情",
                                        onClick = { onSeriesClick(episodeSeries.id) },
                                        modifier = Modifier.weight(1f),
                                        icon = Icons.Default.Collections,
                                    )
                                }
                                HillsSecondaryAction(
                                    label = if (episodeMoreExpanded) "收起" else "更多",
                                    onClick = { episodeMoreExpanded = !episodeMoreExpanded },
                                    modifier = Modifier.weight(1f),
                                    icon = Icons.Default.ChevronRight,
                                )
                            }
                        } else {
                            MovieDetailHeader(
                                media = media,
                                baseUrl = baseUrl,
                                hasResumeProgress = state.hasResumeProgress,
                                resumePositionSeconds = state.resumePositionSeconds,
                                favorite = state.favorite,
                                onBack = onBack,
                                onFavorite = viewModel::toggleFavorite,
                                onPlay = { onPlay(media.id) },
                            )
                        }
                    }

                    if (isEpisode) {
                        item {
                            EpisodePlaybackSpecPanel(
                                media = media,
                                subtitles = state.subtitles,
                            )
                        }
                    }

                    if (isEpisode && episodeMoreExpanded) {
                        item {
                            EpisodeMoreActions(
                                favorite = state.favorite,
                                downloadLabel = downloadCompactLabel(state.download?.status),
                                onFavorite = viewModel::toggleFavorite,
                                onSubtitles = { subtitlesExpanded = !subtitlesExpanded },
                                onDownload = viewModel::toggleDownload,
                            )
                        }
                    }

                    item {
                        if (isEpisode) {
                            DetailFeedCard(title = "本集简介") {
                                Text(
                                    media.overview.ifBlank { "暂无简介" },
                                    style = MaterialTheme.typography.bodyLarge,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    lineHeight = 24.sp,
                                )
                            }
                        } else {
                            MovieOverviewSection(media = media, persons = state.persons)
                        }
                    }

                    if (!isEpisode && state.persons.isNotEmpty()) {
                        item {
                            DetailCastShelf(
                                persons = state.persons,
                                baseUrl = baseUrl,
                                onPersonClick = onPersonClick,
                            )
                        }
                    }

                    val episodeSeason = state.episodeSeason
                    if (isEpisode && episodeSeries != null && episodeSeason?.episodes?.isNotEmpty() == true) {
                        item {
                            EpisodeSeasonShelf(
                                series = episodeSeries,
                                season = episodeSeason,
                                currentMediaId = media.id,
                                baseUrl = baseUrl,
                                onEpisodeClick = onMediaClick,
                            )
                        }
                    }

                    if (subtitlesExpanded) {
                        item {
                            val tracks = state.subtitles.embedded.map { "内嵌" to it.displayLabel } +
                                state.subtitles.external.map { "外挂" to it.displayLabel }
                            DetailFeedCard(
                                title = "字幕",
                                actionLabel = "收起",
                                onAction = { subtitlesExpanded = false },
                            ) {
                                if (tracks.isEmpty()) {
                                    Text("暂无可用字幕", color = MaterialTheme.colorScheme.onSurfaceVariant)
                                } else {
                                    DetailInfoPanel(tracks)
                                }
                            }
                        }
                    }

                    if (state.highlights.isNotEmpty()) {
                        item {
                            if (isEpisode) {
                                DetailFeedCard(
                                    title = "精彩片段",
                                    count = state.highlights.size,
                                    actionLabel = if (highlightsExpanded) "收起" else "查看更多",
                                    onAction = { highlightsExpanded = !highlightsExpanded },
                                ) {
                                    HighlightPreviewGrid(
                                        highlights = state.highlights,
                                        expanded = highlightsExpanded,
                                        baseUrl = baseUrl,
                                        onPlay = { highlight -> onHighlightPlay(media.id, highlight.startTime) },
                                    )
                                }
                            } else {
                                MovieGalleryShelf(
                                    highlights = state.highlights,
                                    baseUrl = baseUrl,
                                    onPlay = { highlight -> onHighlightPlay(media.id, highlight.startTime) },
                                )
                            }
                        }
                    }

                    if (isEpisode && state.persons.isNotEmpty()) {
                        item {
                            DetailFeedCard(
                                title = "演职人员",
                                count = state.persons.size,
                                actionLabel = if (castExpanded) "收起" else "查看更多",
                                onAction = { castExpanded = !castExpanded },
                            ) {
                                CastPreviewGrid(
                                    persons = state.persons,
                                    expanded = castExpanded,
                                    baseUrl = baseUrl,
                                    onPersonClick = onPersonClick,
                                )
                            }
                        }
                    }

                    if (state.recommendations.isNotEmpty()) {
                        item {
                            if (isEpisode) {
                                DetailFeedCard(title = "相似推荐") {
                                    LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                        items(state.recommendations, key = { "similar-${it.resolvedId}" }) { item ->
                                            SimilarPosterCard(
                                                media = item,
                                                imageUrl = resolveImage(baseUrl, item.resolvedPoster),
                                                onClick = { onMediaClick(item.resolvedId) },
                                            )
                                        }
                                    }
                                }
                            } else {
                                MovieRecommendationShelf(
                                    recommendations = state.recommendations,
                                    baseUrl = baseUrl,
                                    onMediaClick = onMediaClick,
                                )
                            }
                        }
                    }

                    state.collection?.collection?.takeIf { it.id.isNotBlank() }?.let { collection ->
                        item {
                            DetailFeedCard(title = "所属合集") {
                                Surface(
                                    onClick = { onCollectionClick(collection.id) },
                                    shape = RoundedCornerShape(14.dp),
                                    color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.58f),
                                ) {
                                    Row(
                                        modifier = Modifier.fillMaxWidth().padding(14.dp),
                                        verticalAlignment = Alignment.CenterVertically,
                                    ) {
                                        Icon(Icons.Default.Collections, null, tint = MaterialTheme.colorScheme.primary)
                                        Spacer(Modifier.width(12.dp))
                                        Column(Modifier.weight(1f)) {
                                            Text(collection.name, style = MaterialTheme.typography.titleMedium)
                                            Text(
                                                listOfNotNull(
                                                    collection.yearRange.takeIf(String::isNotBlank),
                                                    state.collection?.media?.size?.takeIf { it > 0 }?.let { "$it 部作品" },
                                                ).joinToString(" · ").ifBlank { "电影合集" },
                                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                            )
                                        }
                                        Icon(Icons.Default.ChevronRight, null)
                                    }
                                }
                            }
                        }
                    }

                    item {
                        CommentSummarySection(
                            comments = state.comments,
                            expanded = commentsExpanded,
                            loading = state.commentsLoading,
                            actionRunning = state.commentActionRunning,
                            message = state.commentMessage,
                            currentUserId = session.user?.id.orEmpty(),
                            isAdmin = session.user?.role == "admin",
                            text = commentText,
                            rating = commentRating,
                            onToggle = {
                                commentsExpanded = !commentsExpanded
                                if (commentsExpanded && state.comments.data.isEmpty() && state.comments.total == 0) {
                                    viewModel.refreshComments()
                                }
                            },
                            onTextChange = { commentText = it },
                            onRatingChange = { commentRating = it },
                            onSubmit = {
                                viewModel.createComment(commentText, commentRating.takeIf { it > 0 })
                                commentText = ""
                                commentRating = 0
                            },
                            onDelete = viewModel::deleteComment,
                            onRefresh = viewModel::refreshComments,
                        )
                    }

                    item {
                        val messages = listOfNotNull(state.favoriteMessage, state.downloadMessage)
                        if (state.download != null || messages.isNotEmpty()) {
                            DetailFeedCard(title = "本机状态") {
                                state.download?.let { download ->
                                    Row(
                                        modifier = Modifier.fillMaxWidth(),
                                        verticalAlignment = Alignment.CenterVertically,
                                    ) {
                                        Icon(downloadActionIcon(download.status), null, tint = MaterialTheme.colorScheme.primary)
                                        Spacer(Modifier.width(10.dp))
                                        Text(downloadActionLabel(download.status), modifier = Modifier.weight(1f))
                                        HillsSecondaryAction(
                                            label = downloadCompactLabel(download.status),
                                            onClick = viewModel::toggleDownload,
                                            enabled = !state.downloadActionRunning && download.status != OfflineDownloadStatus.Completed,
                                            modifier = Modifier.width(112.dp),
                                        )
                                    }
                                    if (download.status != OfflineDownloadStatus.Completed) {
                                        Spacer(Modifier.height(10.dp))
                                        LinearProgressIndicator(
                                            progress = { download.progress },
                                            modifier = Modifier.fillMaxWidth(),
                                        )
                                    }
                                } ?: HillsSecondaryAction(
                                    label = "下载到本机",
                                    icon = Icons.Default.CloudDownload,
                                    onClick = viewModel::toggleDownload,
                                    enabled = !state.downloadActionRunning,
                                    modifier = Modifier.fillMaxWidth(),
                                )
                                messages.forEach { message ->
                                    Spacer(Modifier.height(6.dp))
                                    Text(message, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

internal fun mediaLogoUrl(baseUrl: String?, mediaId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/media/$mediaId/logo" }

private val MoviePageBackground = Color(0xFF203847)
private val MovieSpecSurface = Color(0xE5172834)
private val MoviePlayColor = Color(0xFF9AC9ED)

@Composable
private fun MovieDetailHeader(
    media: MediaDetail,
    baseUrl: String?,
    hasResumeProgress: Boolean,
    resumePositionSeconds: Double,
    favorite: Boolean,
    onBack: () -> Unit,
    onFavorite: () -> Unit,
    onPlay: () -> Unit,
) {
    val usePosterAsBackground = media.backdropPath.isBlank()
    val backdrop = resolveImage(baseUrl, if (usePosterAsBackground) media.posterPath else media.backdropPath)
    val logo = mediaLogoUrl(baseUrl, media.id)
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(650.dp)
            .background(MoviePageBackground),
    ) {
        AsyncImage(
            model = backdrop,
            contentDescription = media.displayTitle,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .fillMaxSize()
                .then(
                    if (usePosterAsBackground) {
                        Modifier
                            .graphicsLayer { scaleX = 1.12f; scaleY = 1.12f }
                            .blur(18.dp)
                    } else Modifier,
                ),
        )
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(
                    Brush.verticalGradient(
                        0f to Color.Black.copy(alpha = 0.16f),
                        0.40f to MoviePageBackground.copy(alpha = 0.18f),
                        0.74f to MoviePageBackground.copy(alpha = 0.72f),
                        1f to MoviePageBackground,
                    ),
                ),
        )
        IconButton(
            onClick = onBack,
            modifier = Modifier
                .align(Alignment.TopStart)
                .padding(start = 16.dp, top = 28.dp)
                .size(44.dp)
                .clip(CircleShape)
                .background(Color.Black.copy(alpha = 0.34f))
                .border(1.dp, Color.White.copy(alpha = 0.12f), CircleShape),
        ) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回", tint = Color.White)
        }
        IconButton(
            onClick = onFavorite,
            modifier = Modifier
                .align(Alignment.TopEnd)
                .padding(end = 16.dp, top = 28.dp)
                .size(44.dp)
                .clip(CircleShape)
                .background(Color.Black.copy(alpha = 0.34f))
                .border(1.dp, Color.White.copy(alpha = 0.12f), CircleShape),
        ) {
            Icon(
                if (favorite) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                contentDescription = if (favorite) "取消收藏" else "收藏",
                tint = Color.White,
            )
        }
        Column(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 22.dp),
        ) {
            DetailTitleArtwork(
                logoUrl = logo,
                fallbackTitle = media.displayTitle,
                maxWidth = 240.dp,
                maxHeight = 74.dp,
                fallbackStyle = MaterialTheme.typography.headlineLarge,
            )
            if (media.originalTitle.isNotBlank() && media.originalTitle != media.displayTitle) {
                Spacer(Modifier.height(3.dp))
                Text(media.originalTitle, color = Color.White.copy(alpha = 0.72f), style = MaterialTheme.typography.bodyLarge)
            }
            Spacer(Modifier.height(8.dp))
            Text(movieMetadataLabel(media), color = Color.White.copy(alpha = 0.92f), style = MaterialTheme.typography.bodyLarge)
            Spacer(Modifier.height(12.dp))
            MovieSpecPanel(media)
            Spacer(Modifier.height(14.dp))
            HillsPrimaryAction(
                label = if (hasResumeProgress) "继续播放 ${formatResumeTime(resumePositionSeconds)}" else "播放",
                icon = Icons.Default.PlayArrow,
                onClick = onPlay,
                modifier = Modifier.fillMaxWidth(),
            )
        }
    }
}

@Composable
private fun MovieSpecPanel(media: MediaDetail) {
    val videoLine = listOfNotNull(
        media.resolution.takeIf(String::isNotBlank),
        media.videoCodec.takeIf(String::isNotBlank)?.uppercase(),
        media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
    ).joinToString(" · ")
    val audioLine = media.audioCodec.takeIf(String::isNotBlank)?.uppercase().orEmpty()
    if (videoLine.isBlank() && audioLine.isBlank()) return
    Surface(
        modifier = Modifier.fillMaxWidth(),
        color = MovieSpecSurface.copy(alpha = 0.90f),
        shape = RoundedCornerShape(10.dp),
        border = androidx.compose.foundation.BorderStroke(1.dp, Color.White.copy(alpha = 0.12f)),
        shadowElevation = 2.dp,
    ) {
        Column(Modifier.padding(horizontal = 16.dp, vertical = 6.dp)) {
            if (videoLine.isNotBlank()) {
                MovieSpecRow("视频", videoLine)
            }
            if (videoLine.isNotBlank() && audioLine.isNotBlank()) {
                androidx.compose.material3.HorizontalDivider(color = Color.White.copy(alpha = 0.12f))
            }
            if (audioLine.isNotBlank()) {
                MovieSpecRow("音频", audioLine)
            }
        }
    }
}

@Composable
private fun MovieSpecRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, color = Color.White.copy(alpha = 0.64f), style = MaterialTheme.typography.bodyMedium, modifier = Modifier.width(76.dp))
        Text(value, color = Color.White, style = MaterialTheme.typography.bodyMedium, fontWeight = FontWeight.SemiBold)
    }
}

@Composable
private fun MovieOverviewSection(
    media: MediaDetail,
    persons: List<MediaPerson>,
) {
    val directors = persons.filter { it.role.equals("director", ignoreCase = true) }
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 12.dp),
    ) {
        Text("影片简介", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(12.dp))
        Text(
            media.overview.ifBlank { "暂无简介" },
            style = MaterialTheme.typography.bodyLarge,
            color = Color(0xFFD3DEE5),
            lineHeight = 27.sp,
        )
        if (directors.isNotEmpty()) {
            Spacer(Modifier.height(14.dp))
            Text(
                "导演：${directors.joinToString("、") { it.person.name.ifBlank { it.roleLabel } }}",
                color = Color(0xFFB8C7D0),
                style = MaterialTheme.typography.bodyMedium,
            )
        }
    }
}

@Composable
private fun MovieGalleryShelf(
    highlights: List<MediaHighlight>,
    baseUrl: String?,
    onPlay: (MediaHighlight) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 10.dp),
    ) {
        Text("精彩片段", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(12.dp))
        LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            items(highlights.take(12), key = MediaHighlight::id) { highlight ->
                Box(
                    modifier = Modifier
                        .width(330.dp)
                        .aspectRatio(16f / 9f)
                        .clip(RoundedCornerShape(10.dp))
                        .background(MovieSpecSurface)
                        .border(1.dp, Color.White.copy(alpha = 0.10f), RoundedCornerShape(10.dp))
                        .clickable(onClick = { onPlay(highlight) }),
                ) {
                    AsyncImage(
                        model = resolveImage(baseUrl, highlight.thumbnailUrl),
                        contentDescription = highlight.title.ifBlank { "精彩片段" },
                        contentScale = ContentScale.Crop,
                        modifier = Modifier.fillMaxSize(),
                    )
                    Icon(
                        Icons.Default.PlayArrow,
                        contentDescription = "播放片段",
                        tint = Color.White,
                        modifier = Modifier
                            .align(Alignment.Center)
                            .size(34.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun MovieRecommendationShelf(
    recommendations: List<MediaCard>,
    baseUrl: String?,
    onMediaClick: (String) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 10.dp),
    ) {
        Text("更多类似", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
        Spacer(Modifier.height(12.dp))
        LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            items(recommendations, key = { "movie-similar-${it.resolvedId}" }) { item ->
                MovieRecommendationCard(
                    media = item,
                    imageUrl = resolveImage(baseUrl, item.resolvedPoster),
                    onClick = { onMediaClick(item.resolvedId) },
                )
            }
        }
    }
}

@Composable
private fun MovieRecommendationCard(
    media: MediaCard,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(modifier = Modifier.width(128.dp).clickable(onClick = onClick)) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(10.dp))
                .background(MovieSpecSurface)
                .border(1.dp, Color.White.copy(alpha = 0.10f), RoundedCornerShape(10.dp)),
        ) {
            AsyncImage(
                model = imageUrl,
                contentDescription = media.displayTitle,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
            if (media.rating > 0) {
                Text(
                    "%.1f".format(media.rating),
                    color = Color.White,
                    style = MaterialTheme.typography.labelMedium,
                    modifier = Modifier
                        .align(Alignment.BottomEnd)
                        .padding(6.dp)
                        .clip(RoundedCornerShape(6.dp))
                        .background(Color.Black.copy(alpha = 0.64f))
                        .border(1.dp, Color.White.copy(alpha = 0.10f), RoundedCornerShape(6.dp))
                        .padding(horizontal = 5.dp, vertical = 2.dp),
                )
            }
        }
        Spacer(Modifier.height(7.dp))
        Text(media.displayTitle, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.titleMedium)
        media.year?.let { Text(it.toString(), color = Color(0xFFB8C7D0), style = MaterialTheme.typography.bodySmall) }
    }
}

@Composable
private fun EpisodeMoreActions(
    favorite: Boolean,
    downloadLabel: String,
    onFavorite: () -> Unit,
    onSubtitles: () -> Unit,
    onDownload: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 2.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        HillsSecondaryAction(
            label = if (favorite) "已收藏" else "收藏",
            onClick = onFavorite,
            modifier = Modifier.weight(1f),
            icon = if (favorite) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
        )
        HillsSecondaryAction(
            label = "字幕",
            onClick = onSubtitles,
            modifier = Modifier.weight(1f),
            icon = Icons.Default.Subtitles,
        )
        HillsSecondaryAction(
            label = downloadLabel,
            onClick = onDownload,
            modifier = Modifier.weight(1f),
            icon = Icons.Default.CloudDownload,
        )
    }
}

@Composable
private fun EpisodePlaybackSpecPanel(
    media: MediaDetail,
    subtitles: SubtitleTracksResponse,
) {
    val rows = listOfNotNull(
        listOfNotNull(
            media.resolution.takeIf(String::isNotBlank),
            media.videoCodec.takeIf(String::isNotBlank)?.uppercase(),
            media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
        ).joinToString(" · ").takeIf(String::isNotBlank)?.let { "视频" to it },
        media.audioCodec.takeIf(String::isNotBlank)?.uppercase()?.let { "音频" to it },
        (subtitles.embedded + subtitles.external)
            .takeIf { it.isNotEmpty() }
            ?.let { "字幕" to "${it.size} 条可用字幕" },
    )
    if (rows.isEmpty()) return
    DetailInfoPanel(
        rows = rows,
        modifier = Modifier.padding(horizontal = 20.dp, vertical = 4.dp),
    )
}

@Composable
private fun EpisodeSeasonShelf(
    series: SeriesInfo,
    season: SeasonInfo,
    currentMediaId: String,
    baseUrl: String?,
    onEpisodeClick: (String) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 6.dp),
    ) {
        Text(
            "更多来自 ${series.displayTitle} ${season.label}",
            style = MaterialTheme.typography.titleLarge,
            fontWeight = FontWeight.SemiBold,
        )
        Spacer(Modifier.height(12.dp))
        LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            items(season.episodes, key = MediaDetail::id) { episode ->
                var imageFailed by rememberSaveable(episode.id) { mutableStateOf(false) }
                val episodeArtwork = if (imageFailed) {
                    mediaPosterUrl(baseUrl, episode.id)
                } else {
                    mediaBackdropUrl(baseUrl, episode.id)
                }
                Column(
                    modifier = Modifier
                        .width(220.dp)
                        .clickable(enabled = episode.id != currentMediaId) { onEpisodeClick(episode.id) },
                ) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .aspectRatio(16f / 9f)
                            .clip(RoundedCornerShape(12.dp))
                            .background(MaterialTheme.colorScheme.surfaceVariant),
                    ) {
                        AsyncImage(
                            model = episodeArtwork,
                            contentDescription = episode.episodeTitle.ifBlank { episodeCode(episode) },
                            contentScale = ContentScale.Crop,
                            onError = { imageFailed = true },
                            modifier = Modifier.fillMaxSize(),
                        )
                        if (episode.id == currentMediaId) {
                            Box(
                                modifier = Modifier
                                    .fillMaxSize()
                                    .background(Color.Black.copy(alpha = 0.48f)),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text("当前播放项", color = Color.White, style = MaterialTheme.typography.labelLarge)
                            }
                        }
                    }
                    Spacer(Modifier.height(7.dp))
                    Text(
                        "${episodeCode(episode)} · ${episode.userEpisodeTitle}",
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        style = MaterialTheme.typography.titleMedium,
                    )
                    if (episode.duration > 0.0) {
                        Text(
                            formatResumeTime(episode.duration),
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            style = MaterialTheme.typography.bodySmall,
                        )
                    }
                }
            }
        }
    }
}

private fun episodeCode(media: MediaDetail): String = when {
    media.episodeNumber > 0 -> episodeDisplayName(media.seasonNumber, media.episodeNumber)
    else -> "单集"
}

@Composable
private fun CommentSummarySection(
    comments: MediaCommentList,
    expanded: Boolean,
    loading: Boolean,
    actionRunning: Boolean,
    message: String?,
    currentUserId: String,
    isAdmin: Boolean,
    text: String,
    rating: Int,
    onToggle: () -> Unit,
    onTextChange: (String) -> Unit,
    onRatingChange: (Int) -> Unit,
    onSubmit: () -> Unit,
    onDelete: (String) -> Unit,
    onRefresh: () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 6.dp),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .clickable(onClick = onToggle)
                .padding(vertical = 2.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Surface(
                shape = RoundedCornerShape(10.dp),
                color = MaterialTheme.colorScheme.primaryContainer,
                contentColor = MaterialTheme.colorScheme.primary,
            ) {
                Icon(Icons.Default.Message, null, modifier = Modifier.padding(8.dp).size(18.dp))
            }
            Spacer(Modifier.width(10.dp))
            Column(Modifier.weight(1f)) {
                Text("评价", style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
                val summary = when {
                    loading -> "正在加载评价…"
                    comments.total > 0 && comments.ratingCount > 0 -> "${comments.total} 条评价 · ${"%.1f".format(comments.averageRating)} 分"
                    comments.total > 0 -> "${comments.total} 条评价"
                    else -> "暂无评价 · 可展开评分或留言"
                }
                Text(summary, color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 12.sp)
            }
            if (comments.ratingCount > 0) {
                Surface(
                    shape = RoundedCornerShape(999.dp),
                    color = MaterialTheme.colorScheme.surfaceVariant,
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 5.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Icon(Icons.Default.Star, null, tint = Color(0xFFFFC53D), modifier = Modifier.size(13.dp))
                        Spacer(Modifier.width(3.dp))
                        Text("%.1f".format(comments.averageRating), fontSize = 11.sp)
                    }
                }
                Spacer(Modifier.width(6.dp))
            }
            Text(if (expanded) "收起" else "查看评价", color = MaterialTheme.colorScheme.primary, fontSize = 12.sp)
            Icon(
                if (expanded) Icons.Default.ExpandLess else Icons.Default.ChevronRight,
                null,
                tint = MaterialTheme.colorScheme.primary,
                modifier = Modifier.size(18.dp),
            )
        }

        if (expanded) {
            Spacer(Modifier.height(14.dp))
            Text("给这部作品评分", style = MaterialTheme.typography.titleMedium)
            Spacer(Modifier.height(7.dp))
            LazyRow(horizontalArrangement = Arrangement.spacedBy(2.dp)) {
                items((1..10).toList(), key = { it }) { value ->
                    IconButton(onClick = { onRatingChange(if (rating == value) 0 else value) }, modifier = Modifier.size(30.dp)) {
                        Icon(
                            Icons.Default.Star,
                            contentDescription = "$value/10",
                            tint = if (value <= rating) Color(0xFFFFC53D) else MaterialTheme.colorScheme.outlineVariant,
                            modifier = Modifier.size(19.dp),
                        )
                    }
                }
            }
            Spacer(Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                HillsTextField(
                    value = text,
                    onValueChange = onTextChange,
                    placeholder = "分享你的观影感受…",
                    modifier = Modifier.weight(1f),
                )
                HillsPrimaryAction(
                    label = if (actionRunning) "提交中" else "发表",
                    onClick = onSubmit,
                    modifier = Modifier.width(94.dp),
                    icon = Icons.Default.Send,
                    enabled = text.isNotBlank() && !actionRunning,
                )
            }

            message?.let {
                Spacer(Modifier.height(7.dp))
                Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 12.sp)
            }

            Spacer(Modifier.height(12.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text("最近评价", style = MaterialTheme.typography.titleMedium)
                HillsSecondaryAction(
                    label = "刷新",
                    onClick = onRefresh,
                    enabled = !loading,
                    modifier = Modifier.width(84.dp),
                )
            }

            if (loading) {
                LinearProgressIndicator(Modifier.fillMaxWidth())
            } else if (comments.data.isEmpty()) {
                Text("还没有评价，成为第一个发表评论的人。", color = MaterialTheme.colorScheme.onSurfaceVariant)
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(9.dp)) {
                    comments.data.take(10).forEach { comment ->
                        CommentCard(
                            comment = comment,
                            canDelete = isAdmin || (currentUserId.isNotBlank() && comment.userId == currentUserId),
                            actionRunning = actionRunning,
                            onDelete = { onDelete(comment.id) },
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun CommentCard(
    comment: MediaComment,
    canDelete: Boolean,
    actionRunning: Boolean,
    onDelete: () -> Unit,
) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.40f),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth().padding(11.dp),
            verticalAlignment = Alignment.Top,
        ) {
            Surface(
                modifier = Modifier.size(34.dp),
                shape = CircleShape,
                color = MaterialTheme.colorScheme.primaryContainer,
                contentColor = MaterialTheme.colorScheme.primary,
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        (comment.nickname.ifBlank { comment.username.ifBlank { "U" } }).take(1).uppercase(),
                        fontWeight = FontWeight.Bold,
                    )
                }
            }
            Spacer(Modifier.width(9.dp))
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        comment.nickname.ifBlank { comment.username.ifBlank { "用户" } },
                        fontWeight = FontWeight.SemiBold,
                        fontSize = 12.sp,
                    )
                    if (comment.rating > 0) {
                        Spacer(Modifier.width(6.dp))
                        Icon(Icons.Default.Star, null, tint = Color(0xFFFFC53D), modifier = Modifier.size(12.dp))
                        Text(comment.rating.toString(), color = Color(0xFFFFC53D), fontSize = 11.sp)
                    }
                }
                Spacer(Modifier.height(4.dp))
                Text(comment.content, style = MaterialTheme.typography.bodyMedium)
            }
            if (canDelete) {
                IconButton(onClick = onDelete, enabled = !actionRunning, modifier = Modifier.size(34.dp)) {
                    Icon(Icons.Default.DeleteOutline, contentDescription = "删除评价", modifier = Modifier.size(18.dp))
                }
            }
        }
    }
}

@Composable
private fun DetailFeedCard(
    title: String,
    count: Int? = null,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
    content: @Composable () -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 6.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                Modifier
                    .size(width = 5.dp, height = 24.dp)
                    .clip(RoundedCornerShape(999.dp))
                    .background(MaterialTheme.colorScheme.primary),
            )
            Spacer(Modifier.width(9.dp))
            Text(title, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.SemiBold)
            count?.let {
                Spacer(Modifier.width(6.dp))
                Text("$it 个", color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 12.sp)
            }
            Spacer(Modifier.weight(1f))
            if (!actionLabel.isNullOrBlank() && onAction != null) {
                TextButton(onClick = onAction) {
                    Text(actionLabel, fontSize = 12.sp)
                    Spacer(Modifier.width(2.dp))
                    Icon(
                        if (actionLabel == "收起") Icons.Default.ExpandLess else Icons.Default.ChevronRight,
                        null,
                        modifier = Modifier.size(18.dp),
                    )
                }
            }
        }
        Spacer(Modifier.height(12.dp))
        content()
    }
}

@Composable
private fun HighlightPreviewGrid(
    highlights: List<MediaHighlight>,
    expanded: Boolean,
    baseUrl: String?,
    onPlay: (MediaHighlight) -> Unit,
) {
    val visible = if (expanded) highlights else highlights.take(2)
    Column(verticalArrangement = Arrangement.spacedBy(10.dp)) {
        visible.chunked(2).forEach { rowItems ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(10.dp),
            ) {
                rowItems.forEach { highlight ->
                    HighlightCard(
                        highlight = highlight,
                        imageUrl = resolveImage(baseUrl, highlight.thumbnailUrl),
                        onClick = { onPlay(highlight) },
                        modifier = Modifier.weight(1f),
                    )
                }
                if (rowItems.size == 1) Spacer(Modifier.weight(1f))
            }
        }
    }
}

@Composable
private fun HighlightCard(
    highlight: MediaHighlight,
    imageUrl: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Surface(
        onClick = onClick,
        modifier = modifier,
        shape = RoundedCornerShape(12.dp),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.52f),
        border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.82f)),
        shadowElevation = 2.dp,
    ) {
        Column {
            Box(Modifier.fillMaxWidth().aspectRatio(16f / 9f)) {
                AsyncImage(
                    model = imageUrl,
                    contentDescription = highlight.title,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
                Surface(
                    modifier = Modifier.align(Alignment.Center),
                    shape = CircleShape,
                    color = Color.Black.copy(alpha = 0.58f),
                    contentColor = Color.White,
                ) {
                    Icon(Icons.Default.PlayArrow, null, modifier = Modifier.padding(9.dp).size(20.dp))
                }
                Surface(
                    modifier = Modifier.align(Alignment.BottomEnd).padding(7.dp),
                    shape = RoundedCornerShape(8.dp),
                    color = Color.Black.copy(alpha = 0.65f),
                    contentColor = Color.White,
                ) {
                    Text("${highlight.durationSeconds} 秒", modifier = Modifier.padding(horizontal = 7.dp, vertical = 3.dp), fontSize = 10.sp)
                }
            }
            Row(
                modifier = Modifier.fillMaxWidth().padding(horizontal = 10.dp, vertical = 8.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Text(
                    highlight.title.ifBlank { "精彩片段" },
                    modifier = Modifier.weight(1f),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    fontWeight = FontWeight.SemiBold,
                    fontSize = 12.sp,
                )
                if (highlight.score > 0) {
                    Spacer(Modifier.width(6.dp))
                    Text("%.1f".format(highlight.score), fontSize = 11.sp)
                }
            }
        }
    }
}

@Composable
private fun CastPreviewGrid(
    persons: List<MediaPerson>,
    expanded: Boolean,
    baseUrl: String?,
    onPersonClick: (String) -> Unit,
) {
    val visible = if (expanded) persons else persons.take(4)
    Column(verticalArrangement = Arrangement.spacedBy(12.dp)) {
        visible.chunked(4).forEach { rowItems ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                rowItems.forEach { credit ->
                    CompactCreditCard(
                        credit = credit,
                        imageUrl = personProfileUrl(baseUrl, credit.person.id),
                        onClick = { onPersonClick(credit.person.id) },
                        modifier = Modifier.weight(1f),
                    )
                }
                repeat(4 - rowItems.size) { Spacer(Modifier.weight(1f)) }
            }
        }
    }
}

@Composable
private fun CompactCreditCard(
    credit: MediaPerson,
    imageUrl: String?,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier.clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Box {
            AsyncImage(
                model = imageUrl,
                contentDescription = credit.person.name,
                contentScale = ContentScale.Crop,
                modifier = Modifier
                    .size(62.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
            )
            if (credit.roleLabel.isNotBlank()) {
                Surface(
                    modifier = Modifier.align(Alignment.TopStart),
                    shape = RoundedCornerShape(999.dp),
                    color = Color.Black.copy(alpha = 0.68f),
                    contentColor = Color.White,
                ) {
                    Text(
                        credit.roleLabel,
                        modifier = Modifier.padding(horizontal = 5.dp, vertical = 2.dp),
                        maxLines = 1,
                        fontSize = 8.sp,
                    )
                }
            }
        }
        Spacer(Modifier.height(6.dp))
        Text(
            credit.person.name,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            fontSize = 11.sp,
            fontWeight = FontWeight.SemiBold,
        )
        Text(
            credit.roleLabel,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            fontSize = 9.sp,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun SimilarPosterCard(
    media: MediaCard,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(modifier = Modifier.width(112.dp).clickable(onClick = onClick)) {
        AsyncImage(
            model = imageUrl,
            contentDescription = media.displayTitle,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(11.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
        )
        Spacer(Modifier.height(6.dp))
        Text(media.displayTitle, maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
        Row(verticalAlignment = Alignment.CenterVertically) {
            media.year?.takeIf { it > 0 }?.let { Text(it.toString(), fontSize = 9.sp, color = MaterialTheme.colorScheme.onSurfaceVariant) }
            if (media.rating > 0) {
                Spacer(Modifier.width(5.dp))
                Icon(Icons.Default.Star, null, tint = Color(0xFFFFC53D), modifier = Modifier.size(10.dp))
                Text("%.1f".format(media.rating), fontSize = 9.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}

private fun movieMetadataLabel(media: MediaDetail): String = listOfNotNull(
    media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
    media.year.takeIf { it > 0 }?.toString(),
    media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
    media.resolution.takeIf(String::isNotBlank),
    splitGenres(media.genres).take(2).joinToString(" · ").takeIf(String::isNotBlank),
).joinToString(" · ")

private fun mediaMetadataLabel(media: MediaDetail): String = listOfNotNull(
    media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
    media.year.takeIf { it > 0 }?.toString(),
    media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
    media.resolution.takeIf(String::isNotBlank),
    splitGenres(media.genres).take(2).joinToString(" · ").takeIf(String::isNotBlank),
).joinToString(" · ")

private fun splitGenres(genres: String): List<String> = genres
    .split(',', '，', '/', '|')
    .map(String::trim)
    .filter(String::isNotBlank)
    .distinct()

private fun formatResumeTime(seconds: Double): String {
    val total = seconds.toLong().coerceAtLeast(0L)
    val hours = total / 3600
    val minutes = (total % 3600) / 60
    return if (hours > 0) "${hours}小时${minutes}分" else "${minutes.coerceAtLeast(1)}分钟"
}

private fun downloadActionMessage(status: OfflineDownloadStatus?): String = when (status) {
    null -> "已加入离线下载队列"
    OfflineDownloadStatus.Queued,
    OfflineDownloadStatus.Downloading,
    -> "下载已暂停，可从当前进度继续"
    OfflineDownloadStatus.Paused -> "下载已继续"
    OfflineDownloadStatus.Failed -> "已重新加入下载队列"
    OfflineDownloadStatus.Completed -> "该影片已下载到本机"
}

internal fun downloadActionLabel(status: OfflineDownloadStatus?): String = when (status) {
    null -> "下载到本机"
    OfflineDownloadStatus.Queued -> "等待下载 · 点击暂停"
    OfflineDownloadStatus.Downloading -> "暂停下载"
    OfflineDownloadStatus.Paused -> "继续下载"
    OfflineDownloadStatus.Failed -> "重新下载"
    OfflineDownloadStatus.Completed -> "已下载，可在下载页离线播放"
}

private fun downloadCompactLabel(status: OfflineDownloadStatus?): String = when (status) {
    null -> "下载"
    OfflineDownloadStatus.Queued -> "等待中"
    OfflineDownloadStatus.Downloading -> "暂停"
    OfflineDownloadStatus.Paused -> "继续"
    OfflineDownloadStatus.Failed -> "重试"
    OfflineDownloadStatus.Completed -> "已下载"
}

private fun downloadActionIcon(status: OfflineDownloadStatus?) = when (status) {
    null -> Icons.Default.CloudDownload
    OfflineDownloadStatus.Queued,
    OfflineDownloadStatus.Downloading,
    -> Icons.Default.Pause
    OfflineDownloadStatus.Paused -> Icons.Default.PlayArrow
    OfflineDownloadStatus.Failed -> Icons.Default.Refresh
    OfflineDownloadStatus.Completed -> Icons.Default.CheckCircle
}
