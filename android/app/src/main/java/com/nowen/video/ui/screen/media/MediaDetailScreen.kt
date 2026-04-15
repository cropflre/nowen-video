package com.nowen.video.ui.screen.media

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.MovieCollection
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 电影详情页 — 展示电影完整信息、收藏、合集入口和播放入口
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MediaDetailScreen(
    mediaId: String,
    onPlayClick: (String) -> Unit,
    onCollectionClick: (String) -> Unit = {},
    onBack: () -> Unit,
    viewModel: MediaDetailViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(mediaId) {
        viewModel.loadDetail(mediaId)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    // 收藏按钮
                    IconButton(onClick = { viewModel.toggleFavorite(mediaId) }) {
                        Icon(
                            imageVector = if (uiState.isFavorited) Icons.Default.Favorite
                                else Icons.Default.FavoriteBorder,
                            contentDescription = if (uiState.isFavorited) "取消收藏" else "收藏",
                            tint = if (uiState.isFavorited) MaterialTheme.colorScheme.error
                                else MaterialTheme.colorScheme.onSurface
                        )
                    }
                },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.background.copy(alpha = 0.9f)
                )
            )
        }
    ) { padding ->
        val media = uiState.media

        if (uiState.loading) {
            Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator()
            }
        } else if (media != null) {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .verticalScroll(rememberScrollState())
            ) {
                // 背景海报区域
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(280.dp)
                ) {
                    val backdropUrl = if (media.backdropPath.isNotBlank()) {
                        "${uiState.serverUrl}/api/media/${media.id}/poster?token=${uiState.token}"
                    } else null

                    if (backdropUrl != null) {
                        AsyncImage(
                            model = backdropUrl,
                            contentDescription = null,
                            modifier = Modifier.fillMaxSize(),
                            contentScale = ContentScale.Crop
                        )
                    } else {
                        Surface(
                            modifier = Modifier.fillMaxSize(),
                            color = MaterialTheme.colorScheme.surfaceVariant
                        ) {}
                    }
                }

                // 信息区域
                Column(modifier = Modifier.padding(16.dp)) {
                    // 标题
                    Text(
                        text = media.title,
                        style = MaterialTheme.typography.headlineMedium
                    )

                    // 元信息行
                    Row(
                        modifier = Modifier.padding(vertical = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        if (media.year > 0) {
                            Text(
                                text = "${media.year}",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                        if (media.runtime > 0) {
                            Text(
                                text = "${media.runtime} 分钟",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                        if (media.rating > 0) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(
                                    Icons.Default.Star,
                                    contentDescription = null,
                                    modifier = Modifier.size(16.dp),
                                    tint = MaterialTheme.colorScheme.tertiary
                                )
                                Spacer(modifier = Modifier.width(2.dp))
                                Text(
                                    text = String.format("%.1f", media.rating),
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.tertiary
                                )
                            }
                        }
                        if (media.resolution.isNotBlank()) {
                            SuggestionChip(
                                onClick = {},
                                label = {
                                    Text(
                                        media.resolution,
                                        style = MaterialTheme.typography.labelSmall
                                    )
                                }
                            )
                        }
                    }

                    // 类型标签
                    if (media.genres.isNotBlank()) {
                        Row(
                            modifier = Modifier.padding(bottom = 12.dp),
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            media.genres.split(",").take(4).forEach { genre ->
                                SuggestionChip(
                                    onClick = {},
                                    label = { Text(genre.trim(), style = MaterialTheme.typography.labelSmall) }
                                )
                            }
                        }
                    }

                    // 播放按钮
                    Button(
                        onClick = { onPlayClick(media.id) },
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(52.dp),
                        shape = RoundedCornerShape(12.dp)
                    ) {
                        Icon(Icons.Default.PlayArrow, contentDescription = null)
                        Spacer(modifier = Modifier.width(8.dp))
                        Text("播放", style = MaterialTheme.typography.labelLarge)
                    }

                    Spacer(modifier = Modifier.height(16.dp))

                    // 合集入口
                    if (uiState.collection != null) {
                        val collection = uiState.collection!!
                        Card(
                            modifier = Modifier.fillMaxWidth(),
                            shape = RoundedCornerShape(12.dp),
                            onClick = { onCollectionClick(collection.id) }
                        ) {
                            Row(
                                modifier = Modifier.padding(12.dp),
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(12.dp)
                            ) {
                                Icon(
                                    Icons.Default.Collections,
                                    contentDescription = null,
                                    tint = MaterialTheme.colorScheme.primary
                                )
                                Column(modifier = Modifier.weight(1f)) {
                                    Text(
                                        text = collection.name,
                                        style = MaterialTheme.typography.titleSmall
                                    )
                                    Text(
                                        text = "共 ${collection.mediaCount} 部影片",
                                        style = MaterialTheme.typography.bodySmall,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant
                                    )
                                }
                                Icon(
                                    Icons.Default.ChevronRight,
                                    contentDescription = null,
                                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                        Spacer(modifier = Modifier.height(16.dp))
                    }

                    // 简介
                    if (media.overview.isNotBlank()) {
                        Text(
                            text = "简介",
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.padding(bottom = 8.dp)
                        )
                        Text(
                            text = media.overview,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }

                    Spacer(modifier = Modifier.height(16.dp))

                    // 相似推荐
                    if (uiState.similarMedia.isNotEmpty()) {
                        Text(
                            text = "相似推荐",
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.padding(bottom = 8.dp)
                        )
                        // 注意：这里不能用 LazyRow（因为外层是 verticalScroll），用 Row 代替
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            uiState.similarMedia.take(4).forEach { similar ->
                                val posterUrl = "${uiState.serverUrl}/api/media/${similar.id}/poster?token=${uiState.token}"
                                Card(
                                    modifier = Modifier.width(90.dp),
                                    shape = RoundedCornerShape(8.dp),
                                    onClick = { onPlayClick(similar.id) }
                                ) {
                                    AsyncImage(
                                        model = posterUrl,
                                        contentDescription = similar.title,
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .height(135.dp),
                                        contentScale = ContentScale.Crop
                                    )
                                }
                            }
                        }
                        Spacer(modifier = Modifier.height(16.dp))
                    }

                    // 技术信息
                    Text(
                        text = "技术信息",
                        style = MaterialTheme.typography.titleMedium,
                        modifier = Modifier.padding(bottom = 8.dp)
                    )

                    if (media.videoCodec.isNotBlank()) {
                        InfoRow("视频编码", media.videoCodec)
                    }
                    if (media.audioCodec.isNotBlank()) {
                        InfoRow("音频编码", media.audioCodec)
                    }
                    if (media.fileSize > 0) {
                        InfoRow("文件大小", formatFileSize(media.fileSize))
                    }

                    Spacer(modifier = Modifier.height(32.dp))
                }
            }
        }
    }
}

@Composable
private fun InfoRow(label: String, value: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
        horizontalArrangement = Arrangement.SpaceBetween
    ) {
        Text(
            text = label,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Text(
            text = value,
            style = MaterialTheme.typography.bodyMedium
        )
    }
}

private fun formatFileSize(bytes: Long): String {
    return when {
        bytes >= 1_073_741_824 -> String.format("%.2f GB", bytes / 1_073_741_824.0)
        bytes >= 1_048_576 -> String.format("%.1f MB", bytes / 1_048_576.0)
        else -> String.format("%.0f KB", bytes / 1024.0)
    }
}

// ==================== ViewModel ====================

data class MediaDetailUiState(
    val loading: Boolean = true,
    val media: Media? = null,
    val isFavorited: Boolean = false,
    val collection: MovieCollection? = null,
    val similarMedia: List<Media> = emptyList(),
    val serverUrl: String = "",
    val token: String = "",
    val error: String? = null
)

@HiltViewModel
class MediaDetailViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(MediaDetailUiState())
    val uiState = _uiState.asStateFlow()

    fun loadDetail(mediaId: String) {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true)

            val serverUrl = tokenManager.getServerUrl() ?: ""
            val token = tokenManager.getToken() ?: ""
            _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)

            // 并行加载详情、收藏状态、合集、相似推荐
            launch {
                mediaRepository.getMediaDetail(mediaId).onSuccess { media ->
                    _uiState.value = _uiState.value.copy(media = media)
                }
            }

            launch {
                mediaRepository.checkFavorite(mediaId).onSuccess { favorited ->
                    _uiState.value = _uiState.value.copy(isFavorited = favorited)
                }
            }

            launch {
                mediaRepository.getMediaCollection(mediaId).onSuccess { collection ->
                    _uiState.value = _uiState.value.copy(collection = collection)
                }
            }

            launch {
                mediaRepository.getSimilarMedia(mediaId).onSuccess { similar ->
                    _uiState.value = _uiState.value.copy(similarMedia = similar)
                }
            }

            _uiState.value = _uiState.value.copy(loading = false)
        }
    }

    fun toggleFavorite(mediaId: String) {
        viewModelScope.launch {
            val currentFavorited = _uiState.value.isFavorited
            // 乐观更新 UI
            _uiState.value = _uiState.value.copy(isFavorited = !currentFavorited)

            val result = if (currentFavorited) {
                mediaRepository.removeFavorite(mediaId)
            } else {
                mediaRepository.addFavorite(mediaId)
            }

            result.onFailure {
                // 回滚
                _uiState.value = _uiState.value.copy(isFavorited = currentFavorited)
            }
        }
    }
}
