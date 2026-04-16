package com.nowen.video.ui.screen.home

import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.material3.pulltorefresh.PullToRefreshBox
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
import com.nowen.video.data.model.Library
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.MixedItem
import com.nowen.video.data.model.WatchHistory
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 首页 — 继续观看 + 最近添加 + 媒体库入口
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onSearchClick: () -> Unit,
    onSettingsClick: () -> Unit,
    onLibraryClick: (String) -> Unit,
    onFavoritesClick: () -> Unit = {},
    onHistoryClick: () -> Unit = {},
    onCollectionsClick: () -> Unit = {},
    viewModel: HomeViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(Unit) {
        viewModel.loadData()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        "Nowen Video",
                        style = MaterialTheme.typography.titleLarge
                    )
                },
                actions = {
                    IconButton(onClick = onSearchClick) {
                        Icon(Icons.Default.Search, contentDescription = "搜索")
                    }
                    IconButton(onClick = onSettingsClick) {
                        Icon(Icons.Default.Settings, contentDescription = "设置")
                    }
                }
            )
        }
    ) { padding ->
        if (uiState.loading && uiState.libraries.isEmpty()) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator()
            }
        } else {
            // 下拉刷新
            @OptIn(ExperimentalMaterial3Api::class)
            PullToRefreshBox(
                isRefreshing = uiState.refreshing,
                onRefresh = { viewModel.refresh() },
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
            ) {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize(),
                contentPadding = PaddingValues(bottom = 16.dp)
            ) {
                // 媒体库入口
                if (uiState.libraries.isNotEmpty()) {
                    item {
                        SectionTitle("媒体库")
                        LazyRow(
                            contentPadding = PaddingValues(horizontal = 16.dp),
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            items(uiState.libraries) { library ->
                                LibraryChip(
                                    library = library,
                                    onClick = { onLibraryClick(library.id) }
                                )
                            }
                        }
                        Spacer(modifier = Modifier.height(8.dp))
                    }
                }

                // 快捷入口行（收藏、历史、合集）
                item {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 16.dp, vertical = 8.dp),
                        horizontalArrangement = Arrangement.spacedBy(12.dp)
                    ) {
                        QuickEntryCard(
                            icon = Icons.Default.Favorite,
                            label = "我的收藏",
                            onClick = onFavoritesClick,
                            modifier = Modifier.weight(1f)
                        )
                        QuickEntryCard(
                            icon = Icons.Default.History,
                            label = "观看历史",
                            onClick = onHistoryClick,
                            modifier = Modifier.weight(1f)
                        )
                        QuickEntryCard(
                            icon = Icons.Default.Collections,
                            label = "影视合集",
                            onClick = onCollectionsClick,
                            modifier = Modifier.weight(1f)
                        )
                    }
                    Spacer(modifier = Modifier.height(8.dp))
                }

                // 继续观看
                if (uiState.continueWatching.isNotEmpty()) {
                    item {
                        SectionTitle("继续观看")
                        LazyRow(
                            contentPadding = PaddingValues(horizontal = 16.dp),
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            items(uiState.continueWatching) { history ->
                                history.media?.let { media ->
                                    ContinueWatchingCard(
                                        media = media,
                                        progress = if (history.duration > 0) (history.position / history.duration).toFloat() else 0f,
                                        onClick = { onMediaClick(media.id) }
                                    )
                                }
                            }
                        }
                        Spacer(modifier = Modifier.height(16.dp))
                    }
                }

                // 最近添加（使用混合列表，动漫按系列展示，电影按单部展示）
                if (uiState.recentMixed.isNotEmpty()) {
                    item {
                        SectionTitle("最近添加")
                    }
                    item {
                        LazyRow(
                            contentPadding = PaddingValues(horizontal = 16.dp),
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            items(uiState.recentMixed) { item ->
                                MixedPosterCard(
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

                // 空状态
                if (uiState.libraries.isEmpty() && uiState.recentMixed.isEmpty()) {
                    item {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(64.dp),
                            contentAlignment = Alignment.Center
                        ) {
                            Column(horizontalAlignment = Alignment.CenterHorizontally) {
                                Icon(
                                    Icons.Default.VideoLibrary,
                                    contentDescription = null,
                                    modifier = Modifier.size(64.dp),
                                    tint = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                                Spacer(modifier = Modifier.height(16.dp))
                                Text(
                                    "暂无媒体内容",
                                    style = MaterialTheme.typography.bodyLarge,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                                Text(
                                    "请在 Web 管理后台添加媒体库",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                        }
                    }
                }
            }
            } // PullToRefreshBox
        }
    }
}

// ==================== 子组件 ====================

@Composable
private fun QuickEntryCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    label: String,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    Card(
        modifier = modifier.clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant
        )
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(vertical = 12.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Icon(
                icon,
                contentDescription = null,
                modifier = Modifier.size(24.dp),
                tint = MaterialTheme.colorScheme.primary
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
    }
}

@Composable
private fun SectionTitle(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleMedium,
        modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)
    )
}

@Composable
private fun LibraryChip(library: Library, onClick: () -> Unit) {
    val icon = when (library.type) {
        "movie" -> Icons.Default.Movie
        "tvshow" -> Icons.Default.Tv
        else -> Icons.Default.VideoLibrary
    }

    AssistChip(
        onClick = onClick,
        label = { Text(library.name) },
        leadingIcon = {
            Icon(icon, contentDescription = null, modifier = Modifier.size(18.dp))
        }
    )
}

@Composable
private fun ContinueWatchingCard(
    media: Media,
    progress: Float,
    onClick: () -> Unit
) {
    Card(
        modifier = Modifier
            .width(200.dp)
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp)
    ) {
        Column {
            // 缩略图（使用 backdrop 或 poster）
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(112.dp)
                    .clip(RoundedCornerShape(topStart = 12.dp, topEnd = 12.dp))
            ) {
                // 占位色块
                Surface(
                    modifier = Modifier.fillMaxSize(),
                    color = MaterialTheme.colorScheme.surfaceVariant
                ) {}

                // 播放图标
                Icon(
                    Icons.Default.PlayCircle,
                    contentDescription = null,
                    modifier = Modifier
                        .size(40.dp)
                        .align(Alignment.Center),
                    tint = MaterialTheme.colorScheme.primary.copy(alpha = 0.8f)
                )
            }

            // 进度条
            LinearProgressIndicator(
                progress = { progress.coerceIn(0f, 1f) },
                modifier = Modifier.fillMaxWidth(),
                color = MaterialTheme.colorScheme.primary,
                trackColor = MaterialTheme.colorScheme.surfaceVariant,
            )

            // 标题（剧集显示系列名+集数，电影显示标题）
            Column(modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
                Text(
                    text = media.displayTitle(),
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                // 剧集单集标题（如有）
                if (media.mediaType == "episode" && media.episodeTitle.isNotBlank()) {
                    Text(
                        text = media.episodeTitle,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        }
    }
}

/**
 * 混合列表海报卡片 — 根据类型区分展示电影和剧集合集
 * 电影：显示电影海报、标题、年份、评分
 * 剧集合集：显示系列海报、标题、年份、评分、集数角标
 */
@Composable
fun MixedPosterCard(
    item: MixedItem,
    serverUrl: String,
    token: String,
    onClick: () -> Unit
) {
    // 根据类型提取展示信息
    val title: String
    val year: Int
    val rating: Double
    val posterUrl: String?
    val badgeText: String? // 集数角标（仅剧集合集）

    if (item.type == "series" && item.series != null) {
        val series = item.series
        title = series.title
        year = series.year
        rating = series.rating
        posterUrl = if (series.posterPath.isNotBlank()) {
            "$serverUrl/api/series/${series.id}/poster?token=$token"
        } else null
        badgeText = if (series.episodeCount > 0) "${series.episodeCount} 集" else null
    } else if (item.media != null) {
        val media = item.media
        title = media.title
        year = media.year
        rating = media.rating
        posterUrl = if (media.posterPath.isNotBlank()) {
            "$serverUrl/api/media/${media.id}/poster?token=$token"
        } else null
        badgeText = null
    } else {
        return
    }

    Card(
        modifier = Modifier
            .width(130.dp)
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
    ) {
        Column {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(195.dp)
                    .clip(RoundedCornerShape(topStart = 12.dp, topEnd = 12.dp))
            ) {
                if (posterUrl != null) {
                    AsyncImage(
                        model = posterUrl,
                        contentDescription = title,
                        modifier = Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop
                    )
                } else {
                    Surface(
                        modifier = Modifier.fillMaxSize(),
                        color = MaterialTheme.colorScheme.surfaceVariant
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Movie,
                                contentDescription = null,
                                modifier = Modifier.size(40.dp),
                                tint = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }

                // 评分角标
                if (rating > 0) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.TopEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(6.dp),
                        color = MaterialTheme.colorScheme.primary.copy(alpha = 0.9f)
                    ) {
                        Text(
                            text = String.format("%.1f", rating),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
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
                        color = MaterialTheme.colorScheme.tertiary.copy(alpha = 0.9f)
                    ) {
                        Text(
                            text = badgeText,
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onTertiary,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }
            }

            // 标题 + 年份
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

@Composable
fun MediaPosterCard(
    media: Media,
    serverUrl: String,
    token: String,
    onClick: () -> Unit
) {
    Card(
        modifier = Modifier
            .width(130.dp)
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        elevation = CardDefaults.cardElevation(defaultElevation = 4.dp)
    ) {
        Column {
            // 海报图片
            val posterUrl = if (media.posterPath.isNotBlank()) {
                "$serverUrl/api/media/${media.id}/poster?token=$token"
            } else null

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(195.dp)
                    .clip(RoundedCornerShape(topStart = 12.dp, topEnd = 12.dp))
            ) {
                if (posterUrl != null) {
                    AsyncImage(
                        model = posterUrl,
                        contentDescription = media.title,
                        modifier = Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop
                    )
                } else {
                    Surface(
                        modifier = Modifier.fillMaxSize(),
                        color = MaterialTheme.colorScheme.surfaceVariant
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Movie,
                                contentDescription = null,
                                modifier = Modifier.size(40.dp),
                                tint = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }

                // 评分角标
                if (media.rating > 0) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.TopEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(6.dp),
                        color = MaterialTheme.colorScheme.primary.copy(alpha = 0.9f)
                    ) {
                        Text(
                            text = String.format("%.1f", media.rating),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }
            }

            // 标题 + 年份
            Column(modifier = Modifier.padding(8.dp)) {
                Text(
                    text = media.title,
                    style = MaterialTheme.typography.bodySmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (media.year > 0) {
                    Text(
                        text = "${media.year}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

// ==================== ViewModel ====================

data class HomeUiState(
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val libraries: List<Library> = emptyList(),
    val continueWatching: List<WatchHistory> = emptyList(),
    val recentMixed: List<MixedItem> = emptyList(),
    val serverUrl: String = "",
    val token: String = "",
    val error: String? = null
)

@HiltViewModel
class HomeViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(HomeUiState())
    val uiState = _uiState.asStateFlow()

    fun loadData() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, error = null)

            val serverUrl = tokenManager.getServerUrl() ?: ""
            val token = tokenManager.getToken() ?: ""

            _uiState.value = _uiState.value.copy(
                serverUrl = serverUrl,
                token = token
            )

            // 并行加载数据
            launch {
                mediaRepository.getLibraries()
                    .onSuccess { libraries ->
                        _uiState.value = _uiState.value.copy(libraries = libraries)
                    }
                    .onFailure { e ->
                        _uiState.value = _uiState.value.copy(
                            error = "加载媒体库失败: ${e.message}"
                        )
                    }
            }

            launch {
                mediaRepository.getContinueWatching()
                    .onSuccess { history ->
                        _uiState.value = _uiState.value.copy(continueWatching = history)
                    }
            }

            launch {
                mediaRepository.getRecentMixed(20)
                    .onSuccess { items ->
                        _uiState.value = _uiState.value.copy(recentMixed = items)
                    }
                    .onFailure { e ->
                        _uiState.value = _uiState.value.copy(
                            error = "加载最近媒体失败: ${e.message}"
                        )
                    }
            }

            _uiState.value = _uiState.value.copy(loading = false)
        }
    }

    /**
     * 下拉刷新
     */
    fun refresh() {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(refreshing = true, error = null)

            launch {
                mediaRepository.getLibraries()
                    .onSuccess { libraries ->
                        _uiState.value = _uiState.value.copy(libraries = libraries)
                    }
                    .onFailure { e ->
                        _uiState.value = _uiState.value.copy(
                            error = "刷新失败: ${e.message}"
                        )
                    }
            }

            launch {
                mediaRepository.getContinueWatching().onSuccess { history ->
                    _uiState.value = _uiState.value.copy(continueWatching = history)
                }
            }

            launch {
                mediaRepository.getRecentMixed(20).onSuccess { items ->
                    _uiState.value = _uiState.value.copy(recentMixed = items)
                }
            }

            _uiState.value = _uiState.value.copy(refreshing = false)
        }
    }
}
