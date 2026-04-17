package com.nowen.video.ui.screen.media

import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

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
    LaunchedEffect(mediaId) { viewModel.loadDetail(mediaId) }

    val colorScheme = MaterialTheme.colorScheme

    Box(modifier = Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    title = { },
                    navigationIcon = {
                        IconButton(onClick = onBack) {
                            Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary)
                        }
                    },
                    actions = {
                        IconButton(onClick = { viewModel.toggleFavorite(mediaId) }) {
                            Icon(
                                if (uiState.isFavorited) Icons.Default.Favorite else Icons.Default.FavoriteBorder,
                                contentDescription = null,
                                tint = if (uiState.isFavorited) colorScheme.error else colorScheme.primary.copy(alpha = 0.7f)
                            )
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(containerColor = colorScheme.scrim.copy(alpha = 0.6f))
                )
            }
        ) { padding ->
            val media = uiState.media
            if (uiState.loading) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) {
                    CircularProgressIndicator(color = colorScheme.primary, trackColor = colorScheme.surfaceContainerHigh)
                }
            } else if (media != null) {
                Column(Modifier.fillMaxSize().padding(padding).verticalScroll(rememberScrollState())) {
                    // 背景海报
                    Box(Modifier.fillMaxWidth().height(300.dp)) {
                        val backdropUrl = if (media.backdropPath.isNotBlank())
                            "${uiState.serverUrl}/api/media/${media.id}/poster?token=${uiState.token}" else null
                        if (backdropUrl != null) {
                            AsyncImage(backdropUrl, null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                        } else {
                            Surface(Modifier.fillMaxSize(), color = colorScheme.surfaceVariant) {}
                        }
                        Box(Modifier.fillMaxSize().background(Brush.verticalGradient(
                            listOf(Color.Transparent, colorScheme.scrim.copy(alpha = 0.3f), colorScheme.scrim.copy(alpha = 0.85f), colorScheme.scrim)
                        )))
                        Box(Modifier.fillMaxWidth().height(2.dp).align(Alignment.BottomCenter).background(
                            Brush.horizontalGradient(listOf(Color.Transparent, colorScheme.primary.copy(alpha = 0.5f), colorScheme.primary.copy(alpha = 0.8f), colorScheme.primary.copy(alpha = 0.5f), Color.Transparent))
                        ))
                    }
                    Column(Modifier.padding(16.dp)) {
                        Text(media.displayTitle(), style = MaterialTheme.typography.headlineMedium.copy(fontWeight = FontWeight.Bold, letterSpacing = 1.sp), color = colorScheme.onSurface)
                        if (media.mediaType == "episode" && media.episodeTitle.isNotBlank()) {
                            Text(media.episodeTitle, style = MaterialTheme.typography.titleSmall, color = colorScheme.primary.copy(alpha = 0.8f), modifier = Modifier.padding(top = 4.dp))
                        }
                        // 元信息
                        Row(Modifier.padding(vertical = 10.dp), Arrangement.spacedBy(12.dp), Alignment.CenterVertically) {
                            if (media.year > 0) CyberInfoTag("${media.year}")
                            if (media.runtime > 0) CyberInfoTag("${media.runtime} 分钟")
                            if (media.rating > 0) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Icon(Icons.Default.Star, null, Modifier.size(16.dp), tint = AmberGold)
                                    Spacer(Modifier.width(3.dp))
                                    Text(String.format("%.1f", media.rating), style = MaterialTheme.typography.bodyMedium.copy(fontWeight = FontWeight.Bold), color = AmberGold)
                                }
                            }
                            if (media.resolution.isNotBlank()) {
                                Surface(shape = RoundedCornerShape(6.dp), color = ElectricGreen.copy(alpha = 0.15f),
                                    border = androidx.compose.foundation.BorderStroke(1.dp, ElectricGreen.copy(alpha = 0.4f))) {
                                    Text(media.resolution, style = MaterialTheme.typography.labelSmall.copy(fontWeight = FontWeight.Bold), color = ElectricGreen, modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp))
                                }
                            }
                        }
                        // 类型标签
                        if (media.genres.isNotBlank()) {
                            FlowRow(Modifier.padding(bottom = 14.dp), Arrangement.spacedBy(8.dp), Arrangement.spacedBy(8.dp)) {
                                media.genres.split(",").forEach { g ->
                                    val genre = g.trim()
                                    if (genre.isNotBlank()) {
                                        Surface(onClick = { onSearchClick(genre) }, shape = CyberChipShape, color = colorScheme.secondary.copy(alpha = 0.1f),
                                            border = androidx.compose.foundation.BorderStroke(1.dp, colorScheme.secondary.copy(alpha = 0.3f))) {
                                            Row(Modifier.padding(horizontal = 12.dp, vertical = 6.dp), verticalAlignment = Alignment.CenterVertically) {
                                                Icon(Icons.Default.Tag, null, Modifier.size(12.dp), tint = colorScheme.secondary)
                                                Spacer(Modifier.width(4.dp))
                                                Text(genre, style = MaterialTheme.typography.labelMedium, color = colorScheme.secondary)
                                            }
                                        }
                                    }
                                }
                            }
                        }
                        // 播放按钮
                        Button(onClick = { onPlayClick(media.id) }, Modifier.fillMaxWidth().height(54.dp), shape = CyberButtonShape,
                            colors = ButtonDefaults.buttonColors(containerColor = Color.Transparent), contentPadding = PaddingValues()) {
                            Box(Modifier.fillMaxSize().background(Brush.horizontalGradient(listOf(colorScheme.primary, colorScheme.secondary)), CyberButtonShape), Alignment.Center) {
                                Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                    Icon(Icons.Default.PlayArrow, null, tint = Color.White)
                                    Text("播 放", style = MaterialTheme.typography.labelLarge.copy(fontWeight = FontWeight.Bold, letterSpacing = 4.sp), color = Color.White)
                                }
                            }
                        }
                        Spacer(Modifier.height(20.dp))
                        // 合集
                        val cd = uiState.collectionWithMedia
                        if (cd != null && cd.media.size > 1) {
                            CyberCollectionSection(cd, uiState.serverUrl, uiState.token, { onCollectionClick(cd.collection.id) }) { cid -> if (cid != mediaId) onMediaNavigate(cid) }
                            Spacer(Modifier.height(20.dp))
                        }
                        // 简介
                        if (media.overview.isNotBlank()) {
                            Text("简介", style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.SemiBold, letterSpacing = 1.sp), color = colorScheme.primary, modifier = Modifier.padding(bottom = 8.dp))
                            Text(media.overview, style = MaterialTheme.typography.bodyMedium, color = colorScheme.onSurfaceVariant, lineHeight = 22.sp)
                        }
                        Spacer(Modifier.height(20.dp))
                        // 相似推荐
                        if (uiState.similarMedia.isNotEmpty()) {
                            Text("相似推荐", style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.SemiBold, letterSpacing = 1.sp), color = colorScheme.secondary, modifier = Modifier.padding(bottom = 8.dp))
                            Row(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                                uiState.similarMedia.take(4).forEach { s ->
                                    Box(Modifier.width(90.dp).clip(RoundedCornerShape(10.dp)).border(1.dp, colorScheme.primary.copy(alpha = 0.15f), RoundedCornerShape(10.dp)).clickable { onMediaNavigate(s.id) }) {
                                        AsyncImage("${uiState.serverUrl}/api/media/${s.id}/poster?token=${uiState.token}", s.title, Modifier.fillMaxWidth().height(135.dp), contentScale = ContentScale.Crop)
                                    }
                                }
                            }
                            Spacer(Modifier.height(20.dp))
                        }
                        // 技术信息
                        Box(Modifier.fillMaxWidth().glassMorphism(cornerRadius = 14.dp).padding(16.dp)) {
                            Column {
                                Text("技术信息", style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.SemiBold, letterSpacing = 1.sp), color = ElectricGreen, modifier = Modifier.padding(bottom = 10.dp))
                                if (media.videoCodec.isNotBlank()) CyberInfoRow("视频编码", media.videoCodec)
                                if (media.audioCodec.isNotBlank()) CyberInfoRow("音频编码", media.audioCodec)
                                if (media.fileSize > 0) CyberInfoRow("文件大小", formatFileSize(media.fileSize))
                            }
                        }
                        Spacer(Modifier.height(32.dp))
                    }
                }
            }
        }
    }
}

@Composable
private fun CyberInfoTag(text: String) {
    val colorScheme = MaterialTheme.colorScheme
    Surface(shape = RoundedCornerShape(6.dp), color = colorScheme.surfaceContainerHigh.copy(alpha = 0.8f),
        border = androidx.compose.foundation.BorderStroke(1.dp, colorScheme.primary.copy(alpha = 0.2f))) {
        Text(text, style = MaterialTheme.typography.bodyMedium, color = colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp))
    }
}

@Composable
private fun CyberCollectionSection(collectionData: CollectionWithMedia, serverUrl: String, token: String, onCollectionClick: () -> Unit, onMediaClick: (String) -> Unit) {
    val collection = collectionData.collection; val mediaList = collectionData.media
    var expanded by remember { mutableStateOf(false) }
    val currentIndex = mediaList.indexOfFirst { it.isCurrent }
    val colorScheme = MaterialTheme.colorScheme
    Box(Modifier.fillMaxWidth().glassMorphism(cornerRadius = 16.dp).padding(14.dp)) {
        Column {
            Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.Default.Collections, null, Modifier.size(18.dp), tint = colorScheme.secondary)
                Spacer(Modifier.width(8.dp))
                Text("系列合集", style = MaterialTheme.typography.titleSmall.copy(fontWeight = FontWeight.SemiBold), color = colorScheme.secondary)
                Spacer(Modifier.width(8.dp))
                Surface(Modifier.clickable(onClick = onCollectionClick), RoundedCornerShape(12.dp), colorScheme.secondary.copy(alpha = 0.1f),
                    border = androidx.compose.foundation.BorderStroke(1.dp, colorScheme.secondary.copy(alpha = 0.3f))) {
                    Row(Modifier.padding(horizontal = 8.dp, vertical = 4.dp), verticalAlignment = Alignment.CenterVertically) {
                        Text("${collection.name} · ${mediaList.size}部", style = MaterialTheme.typography.labelSmall, color = colorScheme.secondary)
                        Icon(Icons.Default.ChevronRight, null, Modifier.size(12.dp), tint = colorScheme.secondary)
                    }
                }
                Spacer(Modifier.weight(1f))
                if (currentIndex >= 0 && !expanded) { Text("第 ${currentIndex + 1}/${mediaList.size} 部", style = MaterialTheme.typography.labelSmall, color = colorScheme.outline); Spacer(Modifier.width(4.dp)) }
                IconButton({ expanded = !expanded }, Modifier.size(28.dp)) { Icon(if (expanded) Icons.Default.ExpandLess else Icons.Default.ExpandMore, null, Modifier.size(18.dp), tint = colorScheme.onSurfaceVariant) }
            }
            Spacer(Modifier.height(10.dp))
            if (!expanded) {
                Row(Modifier.fillMaxWidth().horizontalScroll(rememberScrollState()), Arrangement.spacedBy(10.dp)) {
                    mediaList.forEach { item -> CyberCollCardItem(item, serverUrl, token) { onMediaClick(item.id) } }
                }
            } else {
                Column(verticalArrangement = Arrangement.spacedBy(6.dp)) {
                    mediaList.forEachIndexed { i, item -> CyberCollListItem(item, i + 1, serverUrl, token) { onMediaClick(item.id) } }
                }
            }
        }
    }
}

@Composable
private fun CyberCollCardItem(item: CollectionMediaItem, serverUrl: String, token: String, onClick: () -> Unit) {
    val cur = item.isCurrent; val url = if (item.posterPath.isNotBlank()) "$serverUrl/api/media/${item.id}/poster?token=$token" else null
    val colorScheme = MaterialTheme.colorScheme
    Box(Modifier.width(100.dp).clip(RoundedCornerShape(10.dp))
        .then(if (cur) Modifier.border(2.dp, Brush.verticalGradient(listOf(colorScheme.primary, colorScheme.secondary)), RoundedCornerShape(10.dp)) else Modifier.border(1.dp, colorScheme.primary.copy(alpha = 0.1f), RoundedCornerShape(10.dp)))
        .background(if (cur) colorScheme.primary.copy(alpha = 0.06f) else colorScheme.surfaceContainerHigh.copy(alpha = 0.6f)).clickable(enabled = !cur, onClick = onClick)) {
        Column {
            Box(Modifier.fillMaxWidth().height(140.dp).clip(RoundedCornerShape(topStart = 10.dp, topEnd = 10.dp))) {
                if (url != null) AsyncImage(url, item.title, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                else Surface(Modifier.fillMaxSize(), color = colorScheme.surfaceVariant) { Box(contentAlignment = Alignment.Center) { Icon(Icons.Default.Movie, null, Modifier.size(28.dp), tint = colorScheme.outline) } }
                if (cur) Surface(Modifier.align(Alignment.TopStart).padding(4.dp), RoundedCornerShape(4.dp), colorScheme.primary) { Text("当前", style = MaterialTheme.typography.labelSmall, color = Color.White, modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)) }
                if (item.rating > 0 && !cur) Surface(Modifier.align(Alignment.TopEnd).padding(4.dp), RoundedCornerShape(4.dp), colorScheme.scrim.copy(alpha = 0.7f)) { Text("★${String.format("%.1f", item.rating)}", style = MaterialTheme.typography.labelSmall, color = AmberGold, modifier = Modifier.padding(horizontal = 4.dp, vertical = 1.dp)) }
            }
            Column(Modifier.padding(6.dp)) {
                Text(item.title, style = MaterialTheme.typography.labelSmall, maxLines = 1, overflow = TextOverflow.Ellipsis, fontWeight = if (cur) FontWeight.Bold else FontWeight.Normal, color = if (cur) colorScheme.primary else colorScheme.onSurface)
                if (item.year > 0) Text("${item.year}", style = MaterialTheme.typography.labelSmall, color = colorScheme.outline)
            }
        }
    }
}

@Composable
private fun CyberCollListItem(item: CollectionMediaItem, index: Int, serverUrl: String, token: String, onClick: () -> Unit) {
    val cur = item.isCurrent; val url = if (item.posterPath.isNotBlank()) "$serverUrl/api/media/${item.id}/poster?token=$token" else null
    val colorScheme = MaterialTheme.colorScheme
    Surface(Modifier.fillMaxWidth().clickable(enabled = !cur, onClick = onClick), RoundedCornerShape(10.dp),
        if (cur) colorScheme.primary.copy(alpha = 0.08f) else Color.Transparent,
        border = if (cur) androidx.compose.foundation.BorderStroke(1.dp, colorScheme.primary.copy(alpha = 0.4f)) else null) {
        Row(Modifier.padding(8.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(10.dp)) {
            Surface(Modifier.size(28.dp), RoundedCornerShape(6.dp), if (cur) colorScheme.primary else colorScheme.surfaceVariant) { Box(contentAlignment = Alignment.Center) { Text("$index", style = MaterialTheme.typography.labelSmall, fontWeight = FontWeight.Bold, color = if (cur) Color.White else colorScheme.outline) } }
            Box(Modifier.width(36.dp).height(54.dp).clip(RoundedCornerShape(6.dp))) {
                if (url != null) AsyncImage(url, item.title, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                else Surface(Modifier.fillMaxSize(), color = colorScheme.surfaceVariant) { Box(contentAlignment = Alignment.Center) { Icon(Icons.Default.Movie, null, Modifier.size(16.dp), tint = colorScheme.outline) } }
            }
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(item.title, style = MaterialTheme.typography.bodySmall, fontWeight = if (cur) FontWeight.Bold else FontWeight.Normal, color = if (cur) colorScheme.primary else colorScheme.onSurface, maxLines = 1, overflow = TextOverflow.Ellipsis, modifier = Modifier.weight(1f, false))
                    if (cur) { Spacer(Modifier.width(6.dp)); Surface(shape = RoundedCornerShape(4.dp), color = colorScheme.primary) { Text("当前", style = MaterialTheme.typography.labelSmall, color = Color.White, modifier = Modifier.padding(horizontal = 6.dp, vertical = 1.dp)) } }
                }
                Row(horizontalArrangement = Arrangement.spacedBy(8.dp), verticalAlignment = Alignment.CenterVertically) {
                    if (item.year > 0) Text("${item.year}", style = MaterialTheme.typography.labelSmall, color = colorScheme.outline)
                    if (item.rating > 0) Text("★${String.format("%.1f", item.rating)}", style = MaterialTheme.typography.labelSmall, color = AmberGold)
                    if (item.runtime > 0) Text("${item.runtime}分钟", style = MaterialTheme.typography.labelSmall, color = colorScheme.outline)
                }
                if (item.overview.isNotBlank()) Text(item.overview, style = MaterialTheme.typography.labelSmall, color = colorScheme.outline, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
            if (!cur) Surface(Modifier.size(28.dp), CircleShape, colorScheme.primary.copy(alpha = 0.1f)) { Box(contentAlignment = Alignment.Center) { Icon(Icons.Default.PlayArrow, "播放", Modifier.size(16.dp), tint = colorScheme.primary) } }
        }
    }
}

@Composable
private fun CyberInfoRow(label: String, value: String) {
    val colorScheme = MaterialTheme.colorScheme
    Row(Modifier.fillMaxWidth().padding(vertical = 5.dp), Arrangement.SpaceBetween) {
        Text(label, style = MaterialTheme.typography.bodyMedium, color = colorScheme.outline)
        Text(value, style = MaterialTheme.typography.bodyMedium.copy(fontWeight = FontWeight.Medium), color = colorScheme.onSurface)
    }
}

private fun formatFileSize(bytes: Long): String = when {
    bytes >= 1_073_741_824 -> String.format("%.2f GB", bytes / 1_073_741_824.0)
    bytes >= 1_048_576 -> String.format("%.1f MB", bytes / 1_048_576.0)
    else -> String.format("%.0f KB", bytes / 1024.0)
}

data class MediaDetailUiState(val loading: Boolean = true, val media: Media? = null, val isFavorited: Boolean = false, val collectionWithMedia: CollectionWithMedia? = null, val similarMedia: List<Media> = emptyList(), val serverUrl: String = "", val token: String = "", val error: String? = null)

@HiltViewModel
class MediaDetailViewModel @Inject constructor(private val mediaRepository: MediaRepository, private val tokenManager: TokenManager) : ViewModel() {
    private val _uiState = MutableStateFlow(MediaDetailUiState()); val uiState = _uiState.asStateFlow()
    fun loadDetail(mediaId: String) { viewModelScope.launch {
        _uiState.value = _uiState.value.copy(loading = true)
        val serverUrl = tokenManager.getServerUrl() ?: ""; val token = tokenManager.getToken() ?: ""
        _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)
        launch { mediaRepository.getMediaDetail(mediaId).onSuccess { _uiState.value = _uiState.value.copy(media = it) } }
        launch { mediaRepository.checkFavorite(mediaId).onSuccess { _uiState.value = _uiState.value.copy(isFavorited = it) } }
        launch { mediaRepository.getMediaCollection(mediaId).onSuccess { _uiState.value = _uiState.value.copy(collectionWithMedia = it) } }
        launch { mediaRepository.getSimilarMedia(mediaId).onSuccess { _uiState.value = _uiState.value.copy(similarMedia = it) } }
        _uiState.value = _uiState.value.copy(loading = false)
    } }
    fun toggleFavorite(mediaId: String) { viewModelScope.launch {
        val cur = _uiState.value.isFavorited; _uiState.value = _uiState.value.copy(isFavorited = !cur)
        val r = if (cur) mediaRepository.removeFavorite(mediaId) else mediaRepository.addFavorite(mediaId)
        r.onFailure { _uiState.value = _uiState.value.copy(isFavorited = cur) }
    } }
}
