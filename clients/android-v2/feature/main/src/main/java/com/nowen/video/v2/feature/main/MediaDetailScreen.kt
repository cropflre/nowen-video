package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Pause
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.OfflineDownloadRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.OfflineDownloadRecord
import com.nowen.video.v2.core.model.OfflineDownloadStatus
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class MediaDetailUiState(
    val loading: Boolean = true,
    val media: MediaDetail? = null,
    val download: OfflineDownloadRecord? = null,
    val downloadActionRunning: Boolean = false,
    val downloadMessage: String? = null,
    val error: String? = null,
)

@HiltViewModel
class MediaDetailViewModel @Inject constructor(
    private val repository: CatalogRepository,
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
            repository.detail(id)
                .onSuccess { media ->
                    _state.update { it.copy(loading = false, media = media, error = null) }
                }
                .onFailure { error ->
                    _state.update { it.copy(loading = false, error = error.message ?: "详情加载失败") }
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

@Composable
fun MediaDetailScreen(
    mediaId: String,
    onBack: () -> Unit,
    onPlay: (String) -> Unit,
    viewModel: MediaDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    LaunchedEffect(mediaId) { viewModel.load(mediaId) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        when {
            state.loading -> CircularProgressIndicator(Modifier.align(androidx.compose.ui.Alignment.Center))
            state.error != null -> MessagePanel(
                title = "无法打开详情",
                message = state.error!!,
                actionLabel = "返回",
                onAction = onBack,
                modifier = Modifier
                    .align(androidx.compose.ui.Alignment.Center)
                    .padding(20.dp),
            )
            state.media != null -> {
                val media = state.media!!
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState()),
                ) {
                    Box(Modifier.fillMaxWidth().height(280.dp)) {
                        AsyncImage(
                            model = resolveImage(session.activeServer?.baseUrl, media.backdropPath),
                            contentDescription = media.displayTitle,
                            contentScale = ContentScale.Crop,
                            modifier = Modifier.fillMaxSize(),
                        )
                        Box(
                            Modifier
                                .fillMaxSize()
                                .background(
                                    Brush.verticalGradient(
                                        listOf(Color.Transparent, MaterialTheme.colorScheme.background),
                                    ),
                                ),
                        )
                    }
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 20.dp)
                            .offset(y = (-52).dp),
                    ) {
                        AsyncImage(
                            model = resolveImage(session.activeServer?.baseUrl, media.posterPath),
                            contentDescription = media.displayTitle,
                            contentScale = ContentScale.Crop,
                            modifier = Modifier
                                .width(116.dp)
                                .aspectRatio(2f / 3f)
                                .clip(MaterialTheme.shapes.large)
                                .background(MaterialTheme.colorScheme.surfaceVariant),
                        )
                        Spacer(Modifier.width(18.dp))
                        Column(Modifier.weight(1f).padding(top = 42.dp)) {
                            Text(media.displayTitle, style = MaterialTheme.typography.headlineMedium)
                            Spacer(Modifier.height(8.dp))
                            Text(
                                listOfNotNull(
                                    media.year.takeIf { it > 0 }?.toString(),
                                    media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
                                    media.resolution.takeIf { it.isNotBlank() },
                                    media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
                                ).joinToString(" · "),
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 20.dp)
                            .offset(y = (-32).dp),
                    ) {
                        Button(
                            onClick = { onPlay(media.id) },
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Icon(Icons.Default.PlayArrow, null)
                            Spacer(Modifier.width(8.dp))
                            Text("立即播放")
                        }
                        Spacer(Modifier.height(10.dp))
                        FilledTonalButton(
                            onClick = viewModel::toggleDownload,
                            enabled = !state.downloadActionRunning &&
                                state.download?.status != OfflineDownloadStatus.Completed,
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            if (state.downloadActionRunning) {
                                CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                            } else {
                                Icon(downloadActionIcon(state.download?.status), contentDescription = null)
                            }
                            Spacer(Modifier.width(8.dp))
                            Text(downloadActionLabel(state.download?.status))
                        }
                        state.download?.let { download ->
                            if (download.status != OfflineDownloadStatus.Completed) {
                                Spacer(Modifier.height(8.dp))
                                LinearProgressIndicator(
                                    progress = { download.progress },
                                    modifier = Modifier.fillMaxWidth(),
                                )
                                Text(
                                    "${downloadStatusLabel(download.status)} · ${(download.progress * 100).toInt()}%",
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    style = MaterialTheme.typography.bodyMedium,
                                    modifier = Modifier.padding(top = 4.dp),
                                )
                            }
                        }
                        state.downloadMessage?.let { message ->
                            Text(
                                message,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                style = MaterialTheme.typography.bodyMedium,
                                modifier = Modifier.padding(top = 8.dp),
                            )
                        }
                        if (media.genres.isNotBlank()) {
                            Spacer(Modifier.height(18.dp))
                            Text(media.genres, color = MaterialTheme.colorScheme.primary)
                        }
                        Spacer(Modifier.height(16.dp))
                        Text("简介", style = MaterialTheme.typography.titleLarge)
                        Spacer(Modifier.height(8.dp))
                        Text(
                            media.overview.ifBlank { "暂无简介" },
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        Spacer(Modifier.height(36.dp))
                    }
                }
            }
        }

        IconButton(
            onClick = onBack,
            modifier = Modifier
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(8.dp)
                .background(MaterialTheme.colorScheme.surface.copy(alpha = 0.72f), MaterialTheme.shapes.large),
        ) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
        }
    }
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

private fun downloadActionIcon(status: OfflineDownloadStatus?) = when (status) {
    null -> Icons.Default.CloudDownload
    OfflineDownloadStatus.Queued,
    OfflineDownloadStatus.Downloading,
    -> Icons.Default.Pause
    OfflineDownloadStatus.Paused -> Icons.Default.PlayArrow
    OfflineDownloadStatus.Failed -> Icons.Default.Refresh
    OfflineDownloadStatus.Completed -> Icons.Default.CheckCircle
}
