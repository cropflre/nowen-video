package com.nowen.video.ui.screen.home

import androidx.compose.animation.*
import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
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
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 首页 — 赛博朋克风格：深空背景 + 霓虹标题 + 玻璃拟态卡片
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
    val colorScheme = MaterialTheme.colorScheme

    LaunchedEffect(Unit) {
        viewModel.loadData()
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .spaceBackground()
    ) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                // 赛博朋克顶部栏
                TopAppBar(
                    title = {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            // 发光圆点装饰
                            Box(
                                modifier = Modifier
                                    .size(8.dp)
                                    .background(colorScheme.primary, RoundedCornerShape(4.dp))
                            )
                            Spacer(modifier = Modifier.width(10.dp))
                            Text(
                                "NOWEN VIDEO",
                                style = MaterialTheme.typography.titleLarge.copy(
                                    fontWeight = FontWeight.Bold,
                                    letterSpacing = 2.sp
                                ),
                                color = colorScheme.primary
                            )
                        }
                    },
                    actions = {
                        IconButton(onClick = onSearchClick) {
                            Icon(
                                Icons.Default.Search,
                                contentDescription = "搜索",
                                tint = colorScheme.primary.copy(alpha = 0.8f)
                            )
                        }
                        IconButton(onClick = onSettingsClick) {
                            Icon(
                                Icons.Default.Settings,
                                contentDescription = "设置",
                                tint = colorScheme.secondary.copy(alpha = 0.8f)
                            )
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(
                        containerColor = colorScheme.scrim.copy(alpha = 0.85f)
                    )
                )
            }
        ) { padding ->
            if (uiState.loading && uiState.libraries.isEmpty()) {
                // 赛博朋克加载动画
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    CyberLoadingIndicator()
                }
            } else {
                @OptIn(ExperimentalMaterial3Api::class)
                PullToRefreshBox(
                    isRefreshing = uiState.refreshing,
                    onRefresh = { viewModel.refresh() },
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding)
                ) {
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        contentPadding = PaddingValues(bottom = 24.dp)
                    ) {
                        // ===== 媒体库入口 =====
                        if (uiState.libraries.isNotEmpty()) {
                            item {
                                CyberSectionTitle("媒体库", colorScheme.primary)
                                LazyRow(
                                    contentPadding = PaddingValues(horizontal = 16.dp),
                                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                                ) {
                                    items(uiState.libraries) { library ->
                                        CyberLibraryChip(
                                            library = library,
                                            onClick = { onLibraryClick(library.id) }
                                        )
                                    }
                                }
                                Spacer(modifier = Modifier.height(12.dp))
                            }
                        }

                        // ===== 快捷入口 =====
                        item {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 16.dp, vertical = 8.dp),
                                horizontalArrangement = Arrangement.spacedBy(12.dp)
                            ) {
                                CyberQuickEntryCard(
                                    icon = Icons.Default.Favorite,
                                    label = "我的收藏",
                                    glowColor = colorScheme.error,
                                    onClick = onFavoritesClick,
                                    modifier = Modifier.weight(1f)
                                )
                                CyberQuickEntryCard(
                                    icon = Icons.Default.History,
                                    label = "观看历史",
                                    glowColor = colorScheme.secondary,
                                    onClick = onHistoryClick,
                                    modifier = Modifier.weight(1f)
                                )
                                CyberQuickEntryCard(
                                    icon = Icons.Default.Collections,
                                    label = "影视合集",
                                    glowColor = ElectricGreen,
                                    onClick = onCollectionsClick,
                                    modifier = Modifier.weight(1f)
                                )
                            }
                            Spacer(modifier = Modifier.height(8.dp))
                        }

                        // ===== 继续观看 =====
                        if (uiState.continueWatching.isNotEmpty()) {
                            item {
                                CyberSectionTitle("继续观看", ElectricGreen)
                                LazyRow(
                                    contentPadding = PaddingValues(horizontal = 16.dp),
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    items(uiState.continueWatching) { history ->
                                        history.media?.let { media ->
                                            CyberContinueWatchingCard(
                                                media = media,
                                                serverUrl = uiState.serverUrl,
                                                token = uiState.token,
                                                progress = if (history.duration > 0) (history.position / history.duration).toFloat() else 0f,
                                                onClick = {
                                                    if (media.mediaType == "episode" && media.seriesId.isNotBlank()) {
                                                        onSeriesClick(media.seriesId)
                                                    } else {
                                                        onMediaClick(media.id)
                                                    }
                                                }
                                            )
                                        }
                                    }
                                }
                                Spacer(modifier = Modifier.height(16.dp))
                            }
                        }

                        // ===== 最近添加 =====
                        if (uiState.recentMixed.isNotEmpty()) {
                            item {
                                CyberSectionTitle("最近添加", colorScheme.secondary)
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

                        // ===== 空状态 =====
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
                                            tint = colorScheme.primary.copy(alpha = 0.4f)
                                        )
                                        Spacer(modifier = Modifier.height(16.dp))
                                        Text(
                                            "暂无媒体内容",
                                            style = MaterialTheme.typography.bodyLarge,
                                            color = colorScheme.onSurfaceVariant
                                        )
                                        Text(
                                            "请在 Web 管理后台添加媒体库",
                                            style = MaterialTheme.typography.bodySmall,
                                            color = colorScheme.outline
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
}

// ==================== 赛博朋克子组件 ====================

/**
 * 赛博朋克加载指示器
 */
@Composable
private fun CyberLoadingIndicator() {
    val colorScheme = MaterialTheme.colorScheme
    val infiniteTransition = rememberInfiniteTransition(label = "loading")
    val rotation by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(2000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "rotation"
    )

    Column(horizontalAlignment = Alignment.CenterHorizontally) {
        CircularProgressIndicator(
            color = colorScheme.primary,
            trackColor = colorScheme.surfaceContainerHigh,
            strokeWidth = 3.dp
        )
        Spacer(modifier = Modifier.height(16.dp))
        Text(
            "数据加载中...",
            color = colorScheme.primary.copy(alpha = 0.7f),
            style = MaterialTheme.typography.bodySmall,
            letterSpacing = 2.sp
        )
    }
}

/**
 * 赛博朋克章节标题
 */
@Composable
private fun CyberSectionTitle(title: String, accentColor: Color) {
    Row(
        modifier = Modifier.padding(horizontal = 16.dp, vertical = 10.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        // 发光线条装饰
        Box(
            modifier = Modifier
                .width(3.dp)
                .height(18.dp)
                .background(
                    brush = Brush.verticalGradient(
                        colors = listOf(accentColor, accentColor.copy(alpha = 0.3f))
                    ),
                    shape = RoundedCornerShape(2.dp)
                )
        )
        Spacer(modifier = Modifier.width(10.dp))
        Text(
            text = title,
            style = MaterialTheme.typography.titleMedium.copy(
                fontWeight = FontWeight.SemiBold,
                letterSpacing = 1.sp
            ),
            color = accentColor
        )
    }
}

/**
 * 赛博朋克快捷入口卡片
 */
@Composable
private fun CyberQuickEntryCard(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    label: String,
    glowColor: Color,
    onClick: () -> Unit,
    modifier: Modifier = Modifier
) {
    val colorScheme = MaterialTheme.colorScheme
    Box(
        modifier = modifier
            .clip(RoundedCornerShape(14.dp))
            .background(
                brush = Brush.verticalGradient(
                    colors = listOf(
                        glowColor.copy(alpha = 0.08f),
                        colorScheme.surfaceContainerHigh.copy(alpha = 0.9f)
                    )
                )
            )
            .border(
                width = 1.dp,
                brush = Brush.verticalGradient(
                    colors = listOf(
                        glowColor.copy(alpha = 0.25f),
                        glowColor.copy(alpha = 0.05f)
                    )
                ),
                shape = RoundedCornerShape(14.dp)
            )
            .clickable(onClick = onClick)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(vertical = 14.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Icon(
                icon,
                contentDescription = null,
                modifier = Modifier.size(22.dp),
                tint = glowColor
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = colorScheme.onSurfaceVariant,
                letterSpacing = 0.5.sp
            )
        }
    }
}

/**
 * 赛博朋克媒体库芯片
 */
@Composable
private fun CyberLibraryChip(library: Library, onClick: () -> Unit) {
    val colorScheme = MaterialTheme.colorScheme
    val icon = when (library.type) {
        "movie" -> Icons.Default.Movie
        "tvshow" -> Icons.Default.Tv
        else -> Icons.Default.VideoLibrary
    }

    Surface(
        onClick = onClick,
        shape = RoundedCornerShape(20.dp),
        color = colorScheme.surfaceContainerHigh.copy(alpha = 0.8f),
        border = androidx.compose.foundation.BorderStroke(
            1.dp,
            colorScheme.primary.copy(alpha = 0.2f)
        )
    ) {
        Row(
            modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(6.dp)
        ) {
            Icon(
                icon,
                contentDescription = null,
                modifier = Modifier.size(16.dp),
                tint = colorScheme.primary
            )
            Text(
                library.name,
                style = MaterialTheme.typography.labelMedium,
                color = colorScheme.onSurface
            )
        }
    }
}

/**
 * 赛博朋克继续观看卡片
 */
@Composable
private fun CyberContinueWatchingCard(
    media: Media,
    serverUrl: String,
    token: String,
    progress: Float,
    onClick: () -> Unit
) {
    val colorScheme = MaterialTheme.colorScheme
    val posterUrl = if (media.posterPath.isNotBlank()) {
        "$serverUrl/api/media/${media.id}/poster?token=$token"
    } else null

    Box(
        modifier = Modifier
            .width(200.dp)
            .clip(RoundedCornerShape(14.dp))
            .cyberCard(cornerRadius = 14.dp, glowColor = ElectricGreen)
            .clickable(onClick = onClick)
    ) {
        Column {
            // 缩略图 + 海报
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(112.dp)
                    .clip(RoundedCornerShape(topStart = 14.dp, topEnd = 14.dp))
            ) {
                if (posterUrl != null) {
                    AsyncImage(
                        model = posterUrl,
                        contentDescription = media.displayTitle(),
                        modifier = Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop
                    )
                } else {
                    Surface(
                        modifier = Modifier.fillMaxSize(),
                        color = colorScheme.surfaceVariant
                    ) {}
                }

                // 半透明遮罩让播放按钮更显眼
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(colorScheme.scrim.copy(alpha = 0.3f))
                )

                // 播放图标
                Icon(
                    Icons.Default.PlayCircle,
                    contentDescription = null,
                    modifier = Modifier
                        .size(40.dp)
                        .align(Alignment.Center),
                    tint = colorScheme.primary.copy(alpha = 0.85f)
                )
            }

            // 霓虹进度条
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(3.dp)
                    .background(colorScheme.surfaceVariant)
            ) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth(progress.coerceIn(0f, 1f))
                        .fillMaxHeight()
                        .background(
                            brush = Brush.horizontalGradient(
                                colors = listOf(colorScheme.primary, ElectricGreen)
                            )
                        )
                )
            }

            // 标题
            Column(modifier = Modifier.padding(horizontal = 12.dp, vertical = 8.dp)) {
                Text(
                    text = media.displayTitle(),
                    style = MaterialTheme.typography.bodyMedium,
                    color = colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (media.mediaType == "episode" && media.episodeTitle.isNotBlank()) {
                    Text(
                        text = media.episodeTitle,
                        style = MaterialTheme.typography.labelSmall,
                        color = colorScheme.outline,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }
        }
    }
}

/**
 * 赛博朋克混合海报卡片
 */
@Composable
fun MixedPosterCard(
    item: MixedItem,
    serverUrl: String,
    token: String,
    onClick: () -> Unit
) {
    val colorScheme = MaterialTheme.colorScheme
    val title: String
    val year: Int
    val rating: Double
    val posterUrl: String?
    val badgeText: String?

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

    Box(
        modifier = Modifier
            .width(130.dp)
            .clip(RoundedCornerShape(14.dp))
            .cyberCard(cornerRadius = 14.dp)
            .clickable(onClick = onClick)
    ) {
        Column {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(195.dp)
                    .clip(RoundedCornerShape(topStart = 14.dp, topEnd = 14.dp))
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
                        color = colorScheme.surfaceVariant
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Movie,
                                contentDescription = null,
                                modifier = Modifier.size(40.dp),
                                tint = colorScheme.outline
                            )
                        }
                    }
                }

                // 底部渐变遮罩
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(60.dp)
                        .align(Alignment.BottomCenter)
                        .gradientScrim()
                )

                // 评分角标 — 霓虹风格
                if (rating > 0) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.TopEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(6.dp),
                        color = colorScheme.scrim.copy(alpha = 0.7f)
                    ) {
                        Text(
                            text = String.format("%.1f", rating),
                            style = MaterialTheme.typography.labelSmall,
                            color = AmberGold,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }

                // 集数角标
                if (badgeText != null) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.BottomEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(4.dp),
                        color = colorScheme.secondary.copy(alpha = 0.85f)
                    ) {
                        Text(
                            text = badgeText,
                            style = MaterialTheme.typography.labelSmall,
                            color = Color.White,
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
                    color = colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (year > 0) {
                    Text(
                        text = "$year",
                        style = MaterialTheme.typography.labelSmall,
                        color = colorScheme.outline
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
    val colorScheme = MaterialTheme.colorScheme
    Box(
        modifier = Modifier
            .width(130.dp)
            .clip(RoundedCornerShape(14.dp))
            .cyberCard(cornerRadius = 14.dp)
            .clickable(onClick = onClick)
    ) {
        Column {
            val posterUrl = if (media.posterPath.isNotBlank()) {
                "$serverUrl/api/media/${media.id}/poster?token=$token"
            } else null

            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(195.dp)
                    .clip(RoundedCornerShape(topStart = 14.dp, topEnd = 14.dp))
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
                        color = colorScheme.surfaceVariant
                    ) {
                        Box(contentAlignment = Alignment.Center) {
                            Icon(
                                Icons.Default.Movie,
                                contentDescription = null,
                                modifier = Modifier.size(40.dp),
                                tint = colorScheme.outline
                            )
                        }
                    }
                }

                // 底部渐变遮罩
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(60.dp)
                        .align(Alignment.BottomCenter)
                        .gradientScrim()
                )

                // 评分角标
                if (media.rating > 0) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.TopEnd)
                            .padding(6.dp),
                        shape = RoundedCornerShape(6.dp),
                        color = colorScheme.scrim.copy(alpha = 0.7f)
                    ) {
                        Text(
                            text = String.format("%.1f", media.rating),
                            style = MaterialTheme.typography.labelSmall,
                            color = AmberGold,
                            fontWeight = FontWeight.Bold,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }
            }

            Column(modifier = Modifier.padding(8.dp)) {
                Text(
                    text = media.title,
                    style = MaterialTheme.typography.bodySmall,
                    color = colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (media.year > 0) {
                    Text(
                        text = "${media.year}",
                        style = MaterialTheme.typography.labelSmall,
                        color = colorScheme.outline
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
