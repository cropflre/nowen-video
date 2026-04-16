package com.nowen.video.ui.screen.media

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Movie
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
import com.nowen.video.data.model.MixedItem
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 媒体列表页 — 按媒体库展示电影/剧集合集混合网格
 * 使用 /api/media/mixed 接口，将剧集聚合为合集展示，避免每集重复显示
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun MediaListScreen(
    libraryId: String,
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onBack: () -> Unit,
    viewModel: MediaListViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(libraryId) {
        viewModel.loadMedia(libraryId)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(uiState.libraryName.ifBlank { "媒体列表" }) },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { padding ->
        if (uiState.loading) {
            Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator()
            }
        } else {
            LazyVerticalGrid(
                columns = GridCells.Adaptive(minSize = 130.dp),
                modifier = Modifier.fillMaxSize().padding(padding),
                contentPadding = PaddingValues(12.dp),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                items(uiState.mixedList) { item ->
                    MixedGridItem(
                        item = item,
                        serverUrl = uiState.serverUrl,
                        token = uiState.token,
                        onClick = {
                            if (item.type == "series" && item.series != null) {
                                onSeriesClick(item.series.id)
                            } else if (item.media != null) {
                                onMediaClick(item.media.id)
                            }
                        }
                    )
                }
            }
        }
    }
}

@Composable
private fun MixedGridItem(
    item: MixedItem,
    serverUrl: String,
    token: String,
    onClick: () -> Unit
) {
    // 根据类型获取展示信息
    val title: String
    val year: Int
    val posterUrl: String
    val resolution: String
    val badgeText: String? // 额外角标（如集数）

    if (item.type == "series" && item.series != null) {
        val series = item.series
        title = series.title
        year = series.year
        posterUrl = "$serverUrl/api/series/${series.id}/poster?token=$token"
        resolution = ""
        badgeText = if (series.episodeCount > 0) "${series.episodeCount} 集" else null
    } else if (item.media != null) {
        val media = item.media
        title = media.title
        year = media.year
        posterUrl = "$serverUrl/api/media/${media.id}/poster?token=$token"
        resolution = media.resolution
        badgeText = null
    } else {
        return
    }

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        elevation = CardDefaults.cardElevation(defaultElevation = 2.dp)
    ) {
        Column {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .aspectRatio(2f / 3f)
                    .clip(RoundedCornerShape(topStart = 12.dp, topEnd = 12.dp))
            ) {
                AsyncImage(
                    model = posterUrl,
                    contentDescription = title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )

                // 分辨率标签（仅电影显示）
                if (resolution.isNotBlank()) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.BottomStart)
                            .padding(6.dp),
                        shape = RoundedCornerShape(4.dp),
                        color = MaterialTheme.colorScheme.secondaryContainer.copy(alpha = 0.9f)
                    ) {
                        Text(
                            text = resolution,
                            style = MaterialTheme.typography.labelSmall,
                            modifier = Modifier.padding(horizontal = 4.dp, vertical = 1.dp)
                        )
                    }
                }

                // 集数角标（仅剧集合集显示）
                if (badgeText != null) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.BottomEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(4.dp),
                        color = MaterialTheme.colorScheme.primary.copy(alpha = 0.9f)
                    ) {
                        Text(
                            text = badgeText,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }
            }

            Column(modifier = Modifier.padding(8.dp)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.bodySmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (year > 0) {
                    Text(
                        text = "$year",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

// ==================== ViewModel ====================

data class MediaListUiState(
    val loading: Boolean = true,
    val libraryName: String = "",
    val mixedList: List<MixedItem> = emptyList(),
    val serverUrl: String = "",
    val token: String = "",
    val error: String? = null
)

@HiltViewModel
class MediaListViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(MediaListUiState())
    val uiState = _uiState.asStateFlow()

    fun loadMedia(libraryId: String) {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true)

            val serverUrl = tokenManager.getServerUrl() ?: ""
            val token = tokenManager.getToken() ?: ""
            _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)

            mediaRepository.getMediaMixed(libraryId = libraryId, limit = 100).onSuccess { response ->
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    mixedList = response.data
                )
            }.onFailure { e ->
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    error = e.message
                )
            }
        }
    }
}
