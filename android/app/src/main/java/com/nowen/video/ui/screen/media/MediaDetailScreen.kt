package com.nowen.video.ui.screen.media

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.Movie
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Star
import androidx.compose.material.icons.filled.Tag
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
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
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.CollectionMediaItem
import com.nowen.video.data.model.CollectionWithMedia
import com.nowen.video.data.model.Media
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing
import com.nowen.video.ui.util.buildBackdropUrl
import com.nowen.video.ui.util.buildPosterUrl
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 媒体详情页面 — Hills Pro 风格
 */
@OptIn(ExperimentalLayoutApi::class)
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
    LaunchedEffect(mediaId) { viewModel.loadDetail(mediaId) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MobileColors.Bg),
    ) {
        val media = uiState.media
        if (uiState.loading) {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator(color = MobileColors.Primary)
            }
        } else if (media != null) {
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .verticalScroll(rememberScrollState()),
            ) {
                // 背景海报
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(300.dp),
                ) {
                    val backdropUrl = if (media.backdropPath.isNotBlank()) {
                        buildBackdropUrl(uiState.serverUrl, media.id, uiState.token)
                    } else if (media.posterPath.isNotBlank()) {
                        buildPosterUrl(uiState.serverUrl, media.id, "media", uiState.token)
                    } else {
                        null
                    }

                    if (backdropUrl != null) {
                        AsyncImage(
                            backdropUrl,
                            null,
                            Modifier.fillMaxSize(),
                            contentScale = ContentScale.Crop,
                        )
                    } else {
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .background(
                                    Brush.linearGradient(
                                        colors = listOf(MobileColors.PrimarySoft, MobileColors.BgAlt),
                                    ),
                                ),
                        )
                    }

                    // 渐变遮罩
                    Box(
                        modifier = Modifier
                            .fillMaxSize()
                            .background(
                                Brush.verticalGradient(
                                    colors = listOf(
                                        Color.Transparent,
                                        MobileColors.Bg.copy(alpha = 0.3f),
                                        MobileColors.Bg.copy(alpha = 0.85f),
                                        MobileColors.Bg,
                                    ),
                                ),
                            ),
                    )

                    // 返回按钮
                    IconButton(
                        onClick = onBack,
                        modifier = Modifier
                            .padding(16.dp)
                            .align(Alignment.TopStart),
                    ) {
                        Icon(
                            Icons.AutoMirrored.Filled.ArrowBack,
                            contentDescription = "返回",
                            tint = MobileColors.Text,
                        )
                    }

                    // 收藏按钮
                    IconButton(
                        onClick = { viewModel.toggleFavorite(mediaId) },
                        modifier = Modifier
                            .padding(16.dp)
                            .align(Alignment.TopEnd),
                    ) {
                        Icon(
                            if (uiState.isFavorited) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                            contentDescription = null,
                            tint = if (uiState.isFavorited) MobileColors.Error else MobileColors.Text,
                        )
                    }
                }

                // 媒体信息
                Column(
                    modifier = Modifier.padding(horizontal = MobileSpacing.xl),
                ) {
                    // 标题
                    Text(
                        text = media.displayTitle(),
                        fontSize = 24.sp,
                        fontWeight = FontWeight.Bold,
                        color = MobileColors.Text,
                    )

                    // 剧集标题
                    if (media.mediaType == "episode" && media.episodeTitle.isNotBlank()) {
                        Text(
                            text = media.episodeTitle,
                            fontSize = 16.sp,
                            color = MobileColors.Primary,
                            modifier = Modifier.padding(top = 4.dp),
                        )
                    }

                    // 元信息
                    Row(
                        modifier = Modifier.padding(vertical = 12.dp),
                        horizontalArrangement = Arrangement.spacedBy(12.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        if (media.year > 0) {
                            InfoChip(text = "${media.year}")
                        }
                        if (media.runtime > 0) {
                            InfoChip(text = "${media.runtime} 分钟")
                        }
                        if (media.rating > 0) {
                            Row(verticalAlignment = Alignment.CenterVertically) {
                                Icon(
                                    Icons.Default.Star,
                                    null,
                                    Modifier.size(16.dp),
                                    tint = MobileColors.Warning,
                                )
                                Spacer(Modifier.width(4.dp))
                                Text(
                                    String.format("%.1f", media.rating),
                                    fontSize = 14.sp,
                                    fontWeight = FontWeight.Bold,
                                    color = MobileColors.Warning,
                                )
                            }
                        }
                        if (media.resolution.isNotBlank()) {
                            InfoChip(text = media.resolution)
                        }
                    }

                    // 类型标签
                    if (media.genres.isNotBlank()) {
                        FlowRow(
                            modifier = Modifier.padding(bottom = 16.dp),
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                            verticalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            media.genres.split(",").forEach { g ->
                                val genre = g.trim()
                                if (genre.isNotBlank()) {
                                    GenreChip(
                                        genre = genre,
                                        onClick = { onSearchClick(genre) },
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
                            .height(54.dp),
                        shape = RoundedCornerShape(MobileRadius.lg),
                        colors = ButtonDefaults.buttonColors(
                            containerColor = MobileColors.Primary,
                        ),
                    ) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp),
                        ) {
                            Icon(
                                Icons.Default.PlayArrow,
                                null,
                                tint = Color.White,
                            )
                            Text(
                                "播 放",
                                fontSize = 16.sp,
                                fontWeight = FontWeight.Bold,
                                color = Color.White,
                            )
                        }
                    }

                    Spacer(Modifier.height(24.dp))

                    // 合集
                    val cd = uiState.collectionWithMedia
                    if (cd != null && cd.media.size > 1) {
                        CollectionSection(
                            collectionData = cd,
                            serverUrl = uiState.serverUrl,
                            token = uiState.token,
                            onCollectionClick = { onCollectionClick(cd.collection.id) },
                            onMediaClick = { cid -> if (cid != mediaId) onMediaNavigate(cid) },
                        )
                        Spacer(Modifier.height(24.dp))
                    }

                    // 简介
                    if (media.overview.isNotBlank()) {
                        Text(
                            text = "简介",
                            fontSize = 18.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = MobileColors.Text,
                            modifier = Modifier.padding(bottom = 8.dp),
                        )
                        Text(
                            text = media.overview,
                            fontSize = 14.sp,
                            color = MobileColors.Muted,
                            lineHeight = 22.sp,
                        )
                        Spacer(Modifier.height(24.dp))
                    }

                    // 相似推荐
                    if (uiState.similarMedia.isNotEmpty()) {
                        Text(
                            text = "相似推荐",
                            fontSize = 18.sp,
                            fontWeight = FontWeight.SemiBold,
                            color = MobileColors.Text,
                            modifier = Modifier.padding(bottom = 8.dp),
                        )
                        Row(
                            modifier = Modifier.horizontalScroll(rememberScrollState()),
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            uiState.similarMedia.take(4).forEach { s ->
                                Box(
                                    modifier = Modifier
                                        .width(100.dp)
                                        .clip(RoundedCornerShape(MobileRadius.md))
                                        .background(MobileColors.BgAlt)
                                        .clickable { onMediaNavigate(s.id) },
                                ) {
                                    AsyncImage(
                                        buildPosterUrl(uiState.serverUrl, s.id, "media", uiState.token),
                                        s.title,
                                        Modifier
                                            .fillMaxWidth()
                                            .height(150.dp),
                                        contentScale = ContentScale.Crop,
                                    )
                                }
                            }
                        }
                        Spacer(Modifier.height(24.dp))
                    }

                    // 技术信息
                    if (media.videoCodec.isNotBlank() || media.audioCodec.isNotBlank() || media.fileSize > 0) {
                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(MobileRadius.lg))
                                .background(MobileColors.Card)
                                .padding(16.dp),
                        ) {
                            Text(
                                text = "技术信息",
                                fontSize = 18.sp,
                                fontWeight = FontWeight.SemiBold,
                                color = MobileColors.Text,
                                modifier = Modifier.padding(bottom = 12.dp),
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
                        }
                    }

                    Spacer(Modifier.height(32.dp))
                }
            }
        }
    }
}

/**
 * 信息标签
 */
@Composable
private fun InfoChip(text: String) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(MobileRadius.sm))
            .background(MobileColors.BgAlt)
            .padding(horizontal = 8.dp, vertical = 4.dp),
    ) {
        Text(
            text = text,
            fontSize = 14.sp,
            color = MobileColors.Text,
        )
    }
}

/**
 * 类型标签
 */
@Composable
private fun GenreChip(genre: String, onClick: () -> Unit) {
    Box(
        modifier = Modifier
            .clip(RoundedCornerShape(MobileRadius.full))
            .background(MobileColors.PrimarySoft)
            .clickable(onClick = onClick)
            .padding(horizontal = 12.dp, vertical = 6.dp),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(4.dp),
        ) {
            Icon(
                Icons.Default.Tag,
                null,
                Modifier.size(12.dp),
                tint = MobileColors.Primary,
            )
            Text(
                genre,
                fontSize = 14.sp,
                color = MobileColors.Primary,
            )
        }
    }
}

/**
 * 合集区域
 */
@Composable
private fun CollectionSection(
    collectionData: CollectionWithMedia,
    serverUrl: String,
    token: String,
    onCollectionClick: () -> Unit,
    onMediaClick: (String) -> Unit,
) {
    val collection = collectionData.collection
    val mediaList = collectionData.media

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(MobileRadius.lg))
            .background(MobileColors.Card)
            .padding(16.dp),
    ) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                Icons.Default.Collections,
                null,
                Modifier.size(18.dp),
                tint = MobileColors.Primary,
            )
            Spacer(Modifier.width(8.dp))
            Text(
                "系列合集",
                fontSize = 16.sp,
                fontWeight = FontWeight.SemiBold,
                color = MobileColors.Text,
            )
            Spacer(Modifier.width(8.dp))
            Box(
                modifier = Modifier
                    .clip(RoundedCornerShape(MobileRadius.full))
                    .background(MobileColors.PrimarySoft)
                    .clickable(onClick = onCollectionClick)
                    .padding(horizontal = 8.dp, vertical = 4.dp),
            ) {
                Text(
                    "${collection.name} · ${mediaList.size}部",
                    fontSize = 12.sp,
                    color = MobileColors.Primary,
                )
            }
        }

        Spacer(Modifier.height(12.dp))

        // 合集列表
        Row(
            modifier = Modifier.horizontalScroll(rememberScrollState()),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            mediaList.forEach { item ->
                CollectionCardItem(
                    item = item,
                    serverUrl = serverUrl,
                    token = token,
                    onClick = { onMediaClick(item.id) },
                )
            }
        }
    }
}

/**
 * 合集卡片项
 */
@Composable
private fun CollectionCardItem(
    item: CollectionMediaItem,
    serverUrl: String,
    token: String,
    onClick: () -> Unit,
) {
    val isCurrent = item.isCurrent
    val url = if (item.posterPath.isNotBlank()) {
        buildPosterUrl(serverUrl, item.id, "media", token)
    } else {
        null
    }

    Box(
        modifier = Modifier
            .width(100.dp)
            .clip(RoundedCornerShape(MobileRadius.md))
            .background(
                if (isCurrent) MobileColors.PrimarySoft else MobileColors.Card,
            )
            .clickable(enabled = !isCurrent, onClick = onClick),
    ) {
        Column {
            // 海报
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .height(140.dp)
                    .clip(RoundedCornerShape(topStart = MobileRadius.md, topEnd = MobileRadius.md)),
            ) {
                if (url != null) {
                    AsyncImage(
                        url,
                        item.title,
                        Modifier.fillMaxSize(),
                        contentScale = ContentScale.Crop,
                    )
                } else {
                    Box(
                        modifier = Modifier
                            .fillMaxSize()
                            .background(MobileColors.BgAlt),
                        contentAlignment = Alignment.Center,
                    ) {
                        Icon(
                            Icons.Default.Movie,
                            null,
                            Modifier.size(28.dp),
                            tint = MobileColors.Muted,
                        )
                    }
                }

                // 当前标识
                if (isCurrent) {
                    Box(
                        modifier = Modifier
                            .padding(4.dp)
                            .align(Alignment.TopStart)
                            .clip(RoundedCornerShape(MobileRadius.xs))
                            .background(MobileColors.Primary)
                            .padding(horizontal = 6.dp, vertical = 2.dp),
                    ) {
                        Text(
                            "当前",
                            fontSize = 12.sp,
                            color = Color.White,
                        )
                    }
                }

                // 评分
                if (item.rating > 0 && !isCurrent) {
                    Box(
                        modifier = Modifier
                            .padding(4.dp)
                            .align(Alignment.TopEnd)
                            .clip(RoundedCornerShape(MobileRadius.xs))
                            .background(Color.Black.copy(alpha = 0.7f))
                            .padding(horizontal = 4.dp, vertical = 1.dp),
                    ) {
                        Text(
                            "★${String.format("%.1f", item.rating)}",
                            fontSize = 12.sp,
                            color = MobileColors.Warning,
                        )
                    }
                }
            }

            // 标题和年份
            Column(
                modifier = Modifier.padding(6.dp),
            ) {
                Text(
                    item.title,
                    fontSize = 12.sp,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    fontWeight = if (isCurrent) FontWeight.Bold else FontWeight.Normal,
                    color = if (isCurrent) MobileColors.Primary else MobileColors.Text,
                )
                if (item.year > 0) {
                    Text(
                        "${item.year}",
                        fontSize = 12.sp,
                        color = MobileColors.Muted,
                    )
                }
            }
        }
    }
}

/**
 * 信息行
 */
@Composable
private fun InfoRow(label: String, value: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 4.dp),
    ) {
        Text(
            text = label,
            fontSize = 14.sp,
            color = MobileColors.Muted,
            modifier = Modifier.width(80.dp),
        )
        Text(
            text = value,
            fontSize = 14.sp,
            color = MobileColors.Text,
        )
    }
}

/**
 * 格式化文件大小
 */
private fun formatFileSize(bytes: Long): String {
    return when {
        bytes >= 1024 * 1024 * 1024 -> String.format("%.2f GB", bytes / (1024.0 * 1024.0 * 1024.0))
        bytes >= 1024 * 1024 -> String.format("%.2f MB", bytes / (1024.0 * 1024.0))
        bytes >= 1024 -> String.format("%.2f KB", bytes / 1024.0)
        else -> "$bytes B"
    }
}

// ==================== ViewModel ====================

/**
 * 媒体详情 UI 状态
 */
data class MediaDetailUiState(
    val loading: Boolean = true,
    val media: Media? = null,
    val isFavorited: Boolean = false,
    val collectionWithMedia: CollectionWithMedia? = null,
    val similarMedia: List<Media> = emptyList(),
    val serverUrl: String = "",
    val token: String = "",
)

/**
 * 媒体详情 ViewModel
 */
@HiltViewModel
class MediaDetailViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager,
) : ViewModel() {
    private val _uiState = MutableStateFlow(MediaDetailUiState())
    val uiState = _uiState.asStateFlow()

    fun loadDetail(mediaId: String) {
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true)
            val serverUrl = tokenManager.getServerUrl() ?: ""
            val token = tokenManager.getToken() ?: ""
            _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)

            launch {
                mediaRepository.getMediaDetail(mediaId).onSuccess {
                    _uiState.value = _uiState.value.copy(media = it)
                }
            }
            launch {
                mediaRepository.checkFavorite(mediaId).onSuccess {
                    _uiState.value = _uiState.value.copy(isFavorited = it)
                }
            }
            launch {
                mediaRepository.getMediaCollection(mediaId).onSuccess {
                    _uiState.value = _uiState.value.copy(collectionWithMedia = it)
                }
            }
            launch {
                mediaRepository.getSimilarMedia(mediaId).onSuccess {
                    _uiState.value = _uiState.value.copy(similarMedia = it)
                }
            }
            _uiState.value = _uiState.value.copy(loading = false)
        }
    }

    fun toggleFavorite(mediaId: String) {
        viewModelScope.launch {
            val cur = _uiState.value.isFavorited
            _uiState.value = _uiState.value.copy(isFavorited = !cur)
            val r = if (cur) {
                mediaRepository.removeFavorite(mediaId)
            } else {
                mediaRepository.addFavorite(mediaId)
            }
            r.onFailure {
                _uiState.value = _uiState.value.copy(isFavorited = cur)
            }
        }
    }
}
