package com.nowen.video.v2.feature.main

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SuggestionChip
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.OfflineDownloadRepository
import com.nowen.video.v2.core.data.ProgressRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.CollectionWithMedia
import com.nowen.video.v2.core.model.MediaDetail
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
    val resumePositionSeconds: Double,
)

private enum class MediaDetailTab(val label: String) {
    Overview("概览"),
    Cast("演职人员"),
    Technical("媒体信息"),
    Subtitles("字幕"),
}

@Composable
fun MediaDetailScreen(
    mediaId: String,
    onBack: () -> Unit,
    onPlay: (String) -> Unit,
    onPersonClick: (String) -> Unit,
    onCollectionClick: (String) -> Unit,
    viewModel: MediaDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var selectedTab by rememberSaveable(mediaId) { mutableIntStateOf(0) }
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
                val tabs = MediaDetailTab.entries
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = androidx.compose.foundation.layout.PaddingValues(bottom = 36.dp),
                ) {
                    item {
                        MobileDetailHero(
                            title = media.displayTitle,
                            originalTitle = media.originalTitle,
                            metadata = mediaMetadataLabel(media),
                            overview = media.overview,
                            backdropUrl = resolveImage(session.activeServer?.baseUrl, media.backdropPath),
                            posterUrl = resolveImage(session.activeServer?.baseUrl, media.posterPath),
                            primaryActionLabel = if (state.hasResumeProgress) {
                                "继续播放 · ${formatResumeTime(state.resumePositionSeconds)}"
                            } else {
                                "立即播放"
                            },
                            onPrimaryAction = { onPlay(media.id) },
                            onBack = onBack,
                        ) {
                            FilledTonalButton(
                                onClick = viewModel::toggleFavorite,
                                enabled = !state.favoriteActionRunning,
                                modifier = Modifier.weight(1f),
                            ) {
                                if (state.favoriteActionRunning) {
                                    CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                                } else {
                                    Icon(
                                        if (state.favorite) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                                        contentDescription = null,
                                    )
                                }
                                Spacer(Modifier.size(8.dp))
                                Text(if (state.favorite) "已收藏" else "收藏")
                            }
                            FilledTonalButton(
                                onClick = viewModel::toggleDownload,
                                enabled = !state.downloadActionRunning &&
                                    state.download?.status != OfflineDownloadStatus.Completed,
                                modifier = Modifier.weight(1f),
                            ) {
                                if (state.downloadActionRunning) {
                                    CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                                } else {
                                    Icon(downloadActionIcon(state.download?.status), contentDescription = null)
                                }
                                Spacer(Modifier.size(8.dp))
                                Text(downloadCompactLabel(state.download?.status))
                            }
                        }
                    }

                    item {
                        DetailTabStrip(
                            labels = tabs.map(MediaDetailTab::label),
                            selectedIndex = selectedTab,
                            onSelected = { selectedTab = it },
                        )
                    }

                    when (tabs[selectedTab]) {
                        MediaDetailTab.Overview -> {
                            item {
                                DetailSection("剧情简介") {
                                    Text(
                                        media.overview.ifBlank { "暂无简介" },
                                        style = MaterialTheme.typography.bodyLarge,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                    val genres = splitGenres(media.genres)
                                    if (genres.isNotEmpty()) {
                                        Spacer(Modifier.height(14.dp))
                                        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                            items(genres, key = { it }) { genre ->
                                                SuggestionChip(onClick = {}, label = { Text(genre) })
                                            }
                                        }
                                    }
                                }
                            }
                            state.collection?.collection?.takeIf { it.id.isNotBlank() }?.let { collection ->
                                item {
                                    DetailSection("所属合集") {
                                        ElevatedPanel(
                                            Modifier
                                                .fillMaxWidth()
                                                .clickable { onCollectionClick(collection.id) },
                                        ) {
                                            Row(verticalAlignment = Alignment.CenterVertically) {
                                                Icon(
                                                    Icons.Default.Collections,
                                                    contentDescription = null,
                                                    tint = MaterialTheme.colorScheme.primary,
                                                )
                                                Spacer(Modifier.size(14.dp))
                                                androidx.compose.foundation.layout.Column(Modifier.weight(1f)) {
                                                    Text(collection.name, style = MaterialTheme.typography.titleMedium)
                                                    Text(
                                                        listOfNotNull(
                                                            collection.yearRange.takeIf(String::isNotBlank),
                                                            state.collection?.media?.size?.takeIf { it > 0 }?.let { "$it 部作品" },
                                                        ).joinToString(" · ").ifBlank { "电影合集" },
                                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                                    )
                                                }
                                            }
                                        }
                                    }
                                }
                            }
                            item {
                                val messages = listOfNotNull(
                                    state.favoriteMessage,
                                    state.downloadMessage,
                                )
                                if (messages.isNotEmpty() || state.download != null) {
                                    DetailSection("本机状态") {
                                        state.download?.let { download ->
                                            Text(
                                                downloadActionLabel(download.status),
                                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                            )
                                            if (download.status != OfflineDownloadStatus.Completed) {
                                                Spacer(Modifier.height(8.dp))
                                                LinearProgressIndicator(
                                                    progress = { download.progress },
                                                    modifier = Modifier.fillMaxWidth(),
                                                )
                                            }
                                        }
                                        messages.forEach { message ->
                                            Spacer(Modifier.height(6.dp))
                                            Text(message, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                        }
                                    }
                                }
                            }
                        }

                        MediaDetailTab.Cast -> item {
                            DetailSection(
                                title = "演职人员",
                                subtitle = if (state.persons.isEmpty()) "当前媒体暂无演职人员信息" else "点击人物查看相关作品",
                            ) {
                                if (state.persons.isEmpty()) {
                                    MessagePanel("暂无演职人员", "服务器暂未返回该媒体的演职人员信息。")
                                } else {
                                    LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                                        items(state.persons.take(24), key = { it.id }) { credit ->
                                            DetailCreditCard(
                                                name = credit.person.name,
                                                role = credit.roleLabel,
                                                imageUrl = personProfileUrl(session.activeServer?.baseUrl, credit.person.id),
                                                onClick = { onPersonClick(credit.person.id) },
                                            )
                                        }
                                    }
                                }
                            }
                        }

                        MediaDetailTab.Technical -> item {
                            DetailSection("媒体信息", "只展示当前文件真实存在的技术信息") {
                                DetailInfoPanel(mediaTechnicalRows(media))
                            }
                        }

                        MediaDetailTab.Subtitles -> item {
                            val embedded = state.subtitles.embedded
                            val external = state.subtitles.external
                            DetailSection(
                                title = "字幕",
                                subtitle = "内嵌 ${embedded.size} · 外挂 ${external.size}",
                            ) {
                                if (embedded.isEmpty() && external.isEmpty()) {
                                    MessagePanel("暂无字幕", "服务器没有检测到可用的内嵌或外挂字幕。")
                                } else {
                                    DetailInfoPanel(
                                        (embedded.map { "内嵌" to it.displayLabel } +
                                            external.map { "外挂" to it.displayLabel }),
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

private fun mediaMetadataLabel(media: MediaDetail): String = listOfNotNull(
    media.year.takeIf { it > 0 }?.toString(),
    media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
    media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
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
