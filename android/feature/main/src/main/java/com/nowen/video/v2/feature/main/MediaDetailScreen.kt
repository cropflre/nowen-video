package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
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
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.ExpandLess
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Star
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
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
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.CollectionWithMedia
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.MediaHighlight
import com.nowen.video.v2.core.model.MediaPerson
import com.nowen.video.v2.core.model.OfflineDownloadRecord
import com.nowen.video.v2.core.model.OfflineDownloadStatus
import com.nowen.video.v2.core.model.SubtitleTracksResponse
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
    val resumePositionSeconds: Double = 0.0,
    val favorite: Boolean = false,
    val favoriteActionRunning: Boolean = false,
    val favoriteMessage: String? = null,
    val download: OfflineDownloadRecord? = null,
    val downloadActionRunning: Boolean = false,
    val downloadMessage: String? = null,
    val error: String? = null,
) {
    val hasResumeProgress: Boolean
        get() = resumePositionSeconds >= 5.0
}

@HiltViewModel
class MediaDetailViewModel @Inject constructor(
    private val repository: CatalogRepository,
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
                    val resume = async {
                        val media = mediaDeferred.await()
                        val duration = media.duration.takeIf { it > 0.0 }
                            ?: media.runtime.takeIf { it > 0 }?.times(60.0)
                            ?: 0.0
                        progressRepository.restorePosition(id, duration)
                    }
                    RelatedMediaDetail(
                        media = mediaDeferred.await(),
                        favorite = favorite.await(),
                        persons = persons.await(),
                        collection = collection.await(),
                        subtitles = subtitles.await(),
                        highlights = highlights.await(),
                        recommendations = recommendations.await(),
                        resumePositionSeconds = resume.await(),
                    )
                }
            }.onSuccess { result ->
                _state.update {
                    it.copy(
                        loading = false,
                        media = result.media,
                        favorite = result.favorite,
                        persons = result.persons,
                        collection = result.collection,
                        subtitles = result.subtitles,
                        highlights = result.highlights,
                        recommendations = result.recommendations,
                        resumePositionSeconds = result.resumePositionSeconds,
                        error = null,
                    )
                }
            }.onFailure { error ->
                _state.update { it.copy(loading = false, error = error.message ?: "详情加载失败") }
            }
        }
    }

    fun toggleFavorite() {
        val mediaId = loadedId ?: return
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

private data class RelatedMediaDetail(
    val media: MediaDetail,
    val favorite: Boolean,
    val persons: List<MediaPerson>,
    val collection: CollectionWithMedia?,
    val subtitles: SubtitleTracksResponse,
    val highlights: List<MediaHighlight>,
    val recommendations: List<MediaCard>,
    val resumePositionSeconds: Double,
)

/** 与 Web 移动端详情页保持相同的信息流与展开交互。 */
@Composable
fun MediaDetailScreen(
    mediaId: String,
    onBack: () -> Unit,
    onPlay: (String) -> Unit,
    onPersonClick: (String) -> Unit,
    onCollectionClick: (String) -> Unit,
    onMediaClick: (String) -> Unit = onPlay,
    viewModel: MediaDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var highlightsExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    var castExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    var subtitlesExpanded by rememberSaveable(mediaId) { mutableStateOf(false) }
    LaunchedEffect(mediaId) { viewModel.load(mediaId) }

    Box(modifier = Modifier.fillMaxSize()) {
        when {
            state.loading -> CircularProgressIndicator(Modifier.align(Alignment.Center))
            state.error != null -> MessagePanel(
                title = "无法打开详情",
                message = state.error!!,
                actionLabel = "返回",
                onAction = onBack,
                modifier = Modifier
                    .align(Alignment.Center)
                    .padding(20.dp),
            )
            state.media != null -> {
                val media = state.media!!
                val baseUrl = session.activeServer?.baseUrl
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 28.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    item {
                        MobileDetailHero(
                            title = media.displayTitle,
                            originalTitle = media.originalTitle,
                            metadata = mediaMetadataLabel(media),
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
                        ) {
                            FilledTonalButton(
                                onClick = viewModel::toggleFavorite,
                                enabled = !state.favoriteActionRunning,
                                modifier = Modifier.weight(1f).height(42.dp),
                                shape = RoundedCornerShape(10.dp),
                                contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 8.dp),
                            ) {
                                if (state.favoriteActionRunning) {
                                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                                } else {
                                    Icon(
                                        if (state.favorite) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                                        contentDescription = null,
                                        modifier = Modifier.size(19.dp),
                                    )
                                }
                                Spacer(Modifier.width(4.dp))
                                Text(if (state.favorite) "已收藏" else "收藏", fontSize = 12.sp)
                            }
                            FilledTonalButton(
                                onClick = { subtitlesExpanded = !subtitlesExpanded },
                                modifier = Modifier.weight(1f).height(42.dp),
                                shape = RoundedCornerShape(10.dp),
                                contentPadding = androidx.compose.foundation.layout.PaddingValues(horizontal = 8.dp),
                            ) {
                                Icon(Icons.Default.Subtitles, contentDescription = null, modifier = Modifier.size(19.dp))
                                Spacer(Modifier.width(4.dp))
                                Text("字幕", fontSize = 12.sp)
                            }
                        }
                    }

                    item {
                        DetailFeedCard(title = "影片简介") {
                            Text(
                                media.overview.ifBlank { "暂无简介" },
                                style = MaterialTheme.typography.bodyLarge,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                lineHeight = 24.sp,
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
                                    onPlay = { onPlay(media.id) },
                                )
                            }
                        }
                    }

                    if (state.persons.isNotEmpty()) {
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
                        DetailFeedCard(title = "技术规格") {
                            val rows = mediaTechnicalRows(media)
                            if (rows.isEmpty()) {
                                Text("暂无技术信息", color = MaterialTheme.colorScheme.onSurfaceVariant)
                            } else {
                                DetailInfoPanel(rows)
                            }
                        }
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
                                        FilledTonalButton(
                                            onClick = viewModel::toggleDownload,
                                            enabled = !state.downloadActionRunning && download.status != OfflineDownloadStatus.Completed,
                                        ) {
                                            Text(downloadCompactLabel(download.status))
                                        }
                                    }
                                    if (download.status != OfflineDownloadStatus.Completed) {
                                        Spacer(Modifier.height(10.dp))
                                        LinearProgressIndicator(
                                            progress = { download.progress },
                                            modifier = Modifier.fillMaxWidth(),
                                        )
                                    }
                                } ?: FilledTonalButton(
                                    onClick = viewModel::toggleDownload,
                                    enabled = !state.downloadActionRunning,
                                    modifier = Modifier.fillMaxWidth(),
                                ) {
                                    Icon(Icons.Default.CloudDownload, null)
                                    Spacer(Modifier.width(7.dp))
                                    Text("下载到本机")
                                }
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

@Composable
private fun DetailFeedCard(
    title: String,
    count: Int? = null,
    actionLabel: String? = null,
    onAction: (() -> Unit)? = null,
    content: @Composable () -> Unit,
) {
    ElevatedPanel(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 14.dp),
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
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.42f),
        border = androidx.compose.foundation.BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
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

private fun mediaMetadataLabel(media: MediaDetail): String = listOfNotNull(
    media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
    media.year.takeIf { it > 0 }?.toString(),
    media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
    media.resolution.takeIf(String::isNotBlank),
    splitGenres(media.genres).take(2).joinToString(" · ").takeIf(String::isNotBlank),
).joinToString(" · ")

private fun mediaTechnicalRows(media: MediaDetail): List<Pair<String, String>> = listOfNotNull(
    media.resolution.takeIf(String::isNotBlank)?.let { "分辨率" to it },
    media.videoCodec.takeIf(String::isNotBlank)?.let { "视频编码" to it },
    media.audioCodec.takeIf(String::isNotBlank)?.let { "音频编码" to it },
    media.runtime.takeIf { it > 0 }?.let { "片长" to "$it 分钟" },
    media.duration.takeIf { it > 0.0 }?.let { "媒体时长" to formatResumeTime(it) },
)

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
