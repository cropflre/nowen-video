package com.nowen.video.ui.screen.media

import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.CollectionMediaItem
import com.nowen.video.data.model.CollectionWithMedia
import com.nowen.video.data.model.Media
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 电影详情页 — 展示电影完整信息、标签、合集、收藏和播放入口
 */
@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun MediaDetailScreen(
    mediaId: String,
    onPlayClick: (String) -> Unit,
    onCollectionClick: (String) -> Unit = {},
    onSearchClick: (String) -> Unit = {},
    onMediaNavigate: (String) -> Unit = {},
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

                    // ==================== 类型标签（可点击搜索） ====================
                    if (media.genres.isNotBlank()) {
                        FlowRow(
                            modifier = Modifier.padding(bottom = 12.dp),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                            verticalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            media.genres.split(",").forEach { genre ->
                                val trimmedGenre = genre.trim()
                                if (trimmedGenre.isNotBlank()) {
                                    AssistChip(
                                        onClick = { onSearchClick(trimmedGenre) },
                                        label = {
                                            Text(
                                                trimmedGenre,
                                                style = MaterialTheme.typography.labelMedium
                                            )
                                        },
                                        leadingIcon = {
                                            Icon(
                                                Icons.Default.Tag,
                                                contentDescription = null,
                                                modifier = Modifier.size(14.dp)
                                            )
                                        },
                                        shape = RoundedCornerShape(20.dp)
                                    )
                                }
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

                    // ==================== 系列合集（内嵌卡片列表） ====================
                    val collectionData = uiState.collectionWithMedia
                    if (collectionData != null && collectionData.media.size > 1) {
                        CollectionSection(
                            collectionData = collectionData,
                            serverUrl = uiState.serverUrl,
                            token = uiState.token,
                            onCollectionClick = { onCollectionClick(collectionData.collection.id) },
                            onMediaClick = { clickedMediaId ->
                                if (clickedMediaId != mediaId) {
                                    onMediaNavigate(clickedMediaId)
                                }
                            }
                        )
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
                        Row(
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            uiState.similarMedia.take(4).forEach { similar ->
                                val posterUrl = "${uiState.serverUrl}/api/media/${similar.id}/poster?token=${uiState.token}"
                                Card(
                                    modifier = Modifier.width(90.dp),
                                    shape = RoundedCornerShape(8.dp),
                                    onClick = { onMediaNavigate(similar.id) }
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

// ==================== 合集区域组件 ====================

/**
 * 系列合集区域 — 参考 Web 端 CollectionCarousel 实现
 * 支持横向滚动卡片模式和展开列表模式
 */
@Composable
private fun CollectionSection(
    collectionData: CollectionWithMedia,
    serverUrl: String,
    token: String,
    onCollectionClick: () -> Unit,
    onMediaClick: (String) -> Unit
) {
    val collection = collectionData.collection
    val mediaList = collectionData.media
    var expanded by remember { mutableStateOf(false) }

    // 当前电影在合集中的位置
    val currentIndex = mediaList.indexOfFirst { it.isCurrent }

    Card(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f)
        )
    ) {
        Column(modifier = Modifier.padding(12.dp)) {
            // 标题栏
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                Icon(
                    Icons.Default.Collections,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp),
                    tint = MaterialTheme.colorScheme.primary
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    text = "系列合集",
                    style = MaterialTheme.typography.titleSmall,
                    fontWeight = FontWeight.SemiBold
                )

                // 合集名称标签（可点击跳转合集详情）
                Spacer(modifier = Modifier.width(8.dp))
                Surface(
                    modifier = Modifier.clickable(onClick = onCollectionClick),
                    shape = RoundedCornerShape(12.dp),
                    color = MaterialTheme.colorScheme.primary.copy(alpha = 0.1f)
                ) {
                    Row(
                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        Text(
                            text = "${collection.name} · ${mediaList.size}部",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.primary
                        )
                        Icon(
                            Icons.Default.ChevronRight,
                            contentDescription = null,
                            modifier = Modifier.size(12.dp),
                            tint = MaterialTheme.colorScheme.primary
                        )
                    }
                }

                Spacer(modifier = Modifier.weight(1f))

                // 系列进度指示
                if (currentIndex >= 0 && !expanded) {
                    Text(
                        text = "第 ${currentIndex + 1}/${mediaList.size} 部",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Spacer(modifier = Modifier.width(4.dp))
                }

                // 展开/收起按钮
                IconButton(
                    onClick = { expanded = !expanded },
                    modifier = Modifier.size(28.dp)
                ) {
                    Icon(
                        if (expanded) Icons.Default.ExpandLess else Icons.Default.ExpandMore,
                        contentDescription = if (expanded) "收起" else "展开",
                        modifier = Modifier.size(18.dp),
                        tint = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // 横向滚动卡片模式（默认）
            if (!expanded) {
                val scrollState = rememberScrollState()

                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .horizontalScroll(scrollState),
                    horizontalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    mediaList.forEach { item ->
                        CollectionCardItem(
                            item = item,
                            serverUrl = serverUrl,
                            token = token,
                            onClick = { onMediaClick(item.id) }
                        )
                    }
                }
            }

            // 展开的列表模式
            if (expanded) {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    mediaList.forEachIndexed { index, item ->
                        CollectionListItem(
                            item = item,
                            index = index + 1,
                            serverUrl = serverUrl,
                            token = token,
                            onClick = { onMediaClick(item.id) }
                        )
                    }
                }
            }
        }
    }
}

/**
 * 合集横向滚动卡片
 */
@Composable
private fun CollectionCardItem(
    item: CollectionMediaItem,
    serverUrl: String,
    token: String,
    onClick: () -> Unit
) {
    val isCurrent = item.isCurrent
    val posterUrl = if (item.posterPath.isNotBlank()) {
        "$serverUrl/api/media/${item.id}/poster?token=$token"
    } else null

    Card(
        modifier = Modifier
            .width(100.dp)
            .clickable(enabled = !isCurrent, onClick = onClick),
        shape = RoundedCornerShape(10.dp),
        border = if (isCurrent) CardDefaults.outlinedCardBorder().copy(
            width = 2.dp
        ) else null,
        colors = if (isCurrent) CardDefaults.cardColors(
            containerColor = MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
        ) else CardDefaults.cardColors()
    ) {
        Column {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(140.dp)
                    .clip(RoundedCornerShape(topStart = 10.dp, topEnd = 10.dp))
            ) {
                if (posterUrl != null) {
                    AsyncImage(
                        model = posterUrl,
                        contentDescription = item.title,
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
                                modifier = Modifier.size(28.dp),
                                tint = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }

                // "当前" 标识
                if (isCurrent) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.TopStart)
                            .padding(4.dp),
                        shape = RoundedCornerShape(4.dp),
                        color = MaterialTheme.colorScheme.primary
                    ) {
                        Text(
                            text = "当前",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onPrimary,
                            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                        )
                    }
                }

                // 评分
                if (item.rating > 0 && !isCurrent) {
                    Surface(
                        modifier = Modifier
                            .align(Alignment.TopEnd)
                            .padding(4.dp),
                        shape = RoundedCornerShape(4.dp),
                        color = Color.Black.copy(alpha = 0.6f)
                    ) {
                        Text(
                            text = "★${String.format("%.1f", item.rating)}",
                            style = MaterialTheme.typography.labelSmall,
                            color = Color(0xFFFFD700),
                            modifier = Modifier.padding(horizontal = 4.dp, vertical = 1.dp)
                        )
                    }
                }
            }

            // 标题和年份
            Column(modifier = Modifier.padding(6.dp)) {
                Text(
                    text = item.title,
                    style = MaterialTheme.typography.labelSmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    fontWeight = if (isCurrent) FontWeight.Bold else FontWeight.Normal,
                    color = if (isCurrent) MaterialTheme.colorScheme.primary
                        else MaterialTheme.colorScheme.onSurface
                )
                if (item.year > 0) {
                    Text(
                        text = "${item.year}",
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}

/**
 * 合集展开列表项
 */
@Composable
private fun CollectionListItem(
    item: CollectionMediaItem,
    index: Int,
    serverUrl: String,
    token: String,
    onClick: () -> Unit
) {
    val isCurrent = item.isCurrent
    val posterUrl = if (item.posterPath.isNotBlank()) {
        "$serverUrl/api/media/${item.id}/poster?token=$token"
    } else null

    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(enabled = !isCurrent, onClick = onClick),
        shape = RoundedCornerShape(10.dp),
        color = if (isCurrent) MaterialTheme.colorScheme.primaryContainer.copy(alpha = 0.3f)
            else MaterialTheme.colorScheme.surface,
        border = if (isCurrent) CardDefaults.outlinedCardBorder() else null
    ) {
        Row(
            modifier = Modifier.padding(8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(10.dp)
        ) {
            // 序号
            Surface(
                modifier = Modifier.size(28.dp),
                shape = RoundedCornerShape(6.dp),
                color = if (isCurrent) MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.surfaceVariant
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Text(
                        text = "$index",
                        style = MaterialTheme.typography.labelSmall,
                        fontWeight = FontWeight.Bold,
                        color = if (isCurrent) MaterialTheme.colorScheme.onPrimary
                            else MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }

            // 海报缩略图
            Box(
                modifier = Modifier
                    .width(36.dp)
                    .height(54.dp)
                    .clip(RoundedCornerShape(6.dp))
            ) {
                if (posterUrl != null) {
                    AsyncImage(
                        model = posterUrl,
                        contentDescription = item.title,
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
                                modifier = Modifier.size(16.dp),
                                tint = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }
            }

            // 信息
            Column(modifier = Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = item.title,
                        style = MaterialTheme.typography.bodySmall,
                        fontWeight = if (isCurrent) FontWeight.Bold else FontWeight.Normal,
                        color = if (isCurrent) MaterialTheme.colorScheme.primary
                            else MaterialTheme.colorScheme.onSurface,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.weight(1f, fill = false)
                    )
                    if (isCurrent) {
                        Spacer(modifier = Modifier.width(6.dp))
                        Surface(
                            shape = RoundedCornerShape(4.dp),
                            color = MaterialTheme.colorScheme.primary
                        ) {
                            Text(
                                text = "当前",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onPrimary,
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp)
                            )
                        }
                    }
                }
                Row(
                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    if (item.year > 0) {
                        Text(
                            text = "${item.year}",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                    if (item.rating > 0) {
                        Text(
                            text = "★${String.format("%.1f", item.rating)}",
                            style = MaterialTheme.typography.labelSmall,
                            color = Color(0xFFFFD700)
                        )
                    }
                    if (item.runtime > 0) {
                        Text(
                            text = "${item.runtime}分钟",
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
                if (item.overview.isNotBlank()) {
                    Text(
                        text = item.overview,
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                }
            }

            // 播放图标（非当前电影）
            if (!isCurrent) {
                Surface(
                    modifier = Modifier.size(28.dp),
                    shape = CircleShape,
                    color = MaterialTheme.colorScheme.primary.copy(alpha = 0.1f)
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(
                            Icons.Default.PlayArrow,
                            contentDescription = "播放",
                            modifier = Modifier.size(16.dp),
                            tint = MaterialTheme.colorScheme.primary
                        )
                    }
                }
            }
        }
    }
}

// ==================== 通用组件 ====================

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
    val collectionWithMedia: CollectionWithMedia? = null,
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
                mediaRepository.getMediaCollection(mediaId).onSuccess { collectionData ->
                    _uiState.value = _uiState.value.copy(collectionWithMedia = collectionData)
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
