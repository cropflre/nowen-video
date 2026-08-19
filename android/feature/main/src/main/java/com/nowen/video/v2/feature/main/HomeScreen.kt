package com.nowen.video.v2.feature.main

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
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
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Replay
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.blur
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.MobileWebParityRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MobileWebHomeContent
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlin.math.roundToInt
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

private const val HOME_HERO_INTERVAL_MS = 7_000L
private const val HOME_HERO_SWIPE_THRESHOLD = 64f
private val HOME_GENRES = listOf("动画", "喜剧", "冒险", "家庭")

data class HomeUiState(
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val content: MobileWebHomeContent = MobileWebHomeContent(),
    val error: String? = null,
)

@HiltViewModel
class HomeViewModel @Inject constructor(
    private val parityRepository: MobileWebParityRepository,
    val store: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(HomeUiState())
    val state: StateFlow<HomeUiState> = _state

    init {
        load(false)
    }

    fun refresh() = load(true)

    private fun load(refresh: Boolean) {
        viewModelScope.launch {
            _state.update {
                it.copy(
                    loading = !refresh && it.content.isEmpty,
                    refreshing = refresh,
                    error = null,
                )
            }
            parityRepository.home()
                .onSuccess { content ->
                    _state.value = HomeUiState(
                        loading = false,
                        refreshing = false,
                        content = content,
                    )
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(
                            loading = false,
                            refreshing = false,
                            error = error.message ?: "首页加载失败",
                        )
                    }
                }
        }
    }
}

/**
 * Android 首页严格按照 Web 移动端的信息架构组织：
 * Hero -> 继续观看 -> 为你推荐 -> 最近添加 / 分类货架。
 * 横向内容全部采用手势滚动；Hero 支持自动轮播、圆点切换和左右滑动。
 */
@Composable
fun HomeScreen(
    modifier: Modifier = Modifier,
    onMediaClick: (String) -> Unit,
    onPlay: (String) -> Unit,
    onLibraryClick: () -> Unit,
    onHistoryClick: () -> Unit = {},
    onFavoritesClick: () -> Unit = {},
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.store.snapshot.collectAsState()
    val baseUrl = session.activeServer?.baseUrl

    LazyColumn(
        modifier = modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
        contentPadding = PaddingValues(start = 14.dp, end = 14.dp, top = 8.dp, bottom = 26.dp),
        verticalArrangement = Arrangement.spacedBy(24.dp),
    ) {
        item {
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Column(Modifier.weight(1f)) {
                    Text("首页", style = MaterialTheme.typography.headlineMedium)
                    Text(
                        "精选推荐 · 精彩不断",
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                IconButton(onClick = viewModel::refresh, enabled = !state.refreshing) {
                    if (state.refreshing) {
                        CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                    } else {
                        Icon(Icons.Default.Refresh, contentDescription = "刷新首页")
                    }
                }
            }
        }

        if (state.loading) {
            item {
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    LinearProgressIndicator(Modifier.fillMaxWidth())
                    Spacer(Modifier.height(12.dp))
                    Text("正在同步首页内容")
                }
            }
        }

        state.error?.let { message ->
            item { MessagePanel("暂时无法加载", message, "重试", viewModel::refresh) }
        }

        if (state.content.heroItems.isNotEmpty()) {
            item {
                HomeHeroCarousel(
                    items = state.content.heroItems,
                    baseUrl = baseUrl,
                    onOpen = onMediaClick,
                    onPlay = onPlay,
                    onFavoritesClick = onFavoritesClick,
                )
            }
        }

        if (state.content.continueWatching.isNotEmpty()) {
            item {
                HomeSectionHeader(
                    title = "继续观看",
                    action = "查看全部",
                    leading = { Icon(Icons.Default.History, null, tint = MaterialTheme.colorScheme.primary) },
                    onAction = onHistoryClick,
                )
                Spacer(Modifier.height(10.dp))
                LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(state.content.continueWatching, key = { "continue-${it.resolvedId}" }) { media ->
                        ContinueWatchingCard(
                            media = media,
                            imageUrl = resolveHomeArtwork(baseUrl, media, preferBackdrop = true),
                            onClick = { onPlay(media.resolvedId) },
                        )
                    }
                }
            }
        }

        if (state.content.recommendations.isNotEmpty()) {
            item {
                HomeSectionHeader(
                    title = "为你推荐",
                    action = "查看全部",
                    onAction = onLibraryClick,
                )
                Spacer(Modifier.height(10.dp))
                LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(state.content.recommendations, key = { "recommend-${it.resolvedId}" }) { media ->
                        LandscapeMediaCard(
                            media = media,
                            imageUrl = resolveHomeArtwork(baseUrl, media, preferBackdrop = true),
                            onClick = { onMediaClick(media.resolvedId) },
                        )
                    }
                }
            }
        }

        if (state.content.recent.isNotEmpty()) {
            item {
                PosterShelf(
                    title = "最近添加",
                    items = state.content.recent,
                    baseUrl = baseUrl,
                    onMore = onLibraryClick,
                    onMediaClick = onMediaClick,
                )
            }
        }

        HOME_GENRES.forEach { genre ->
            val items = state.content.genreShelves[genre].orEmpty()
            if (items.isNotEmpty()) {
                item(key = "genre-$genre") {
                    PosterShelf(
                        title = genre,
                        items = items,
                        baseUrl = baseUrl,
                        onMore = onLibraryClick,
                        onMediaClick = onMediaClick,
                    )
                }
            }
        }

        if (!state.loading && state.error == null && state.content.isEmpty) {
            item { MessagePanel("暂无内容", "添加媒体后，这里会自动生成与你的 Web 移动端一致的首页。") }
        }
    }
}

@Composable
private fun HomeHeroCarousel(
    items: List<MediaCard>,
    baseUrl: String?,
    onOpen: (String) -> Unit,
    onPlay: (String) -> Unit,
    onFavoritesClick: () -> Unit,
) {
    var index by remember(items.map(MediaCard::resolvedId)) { mutableIntStateOf(0) }
    var dragDistance by remember { mutableStateOf(0f) }
    val safeIndex = index.coerceIn(0, items.lastIndex)
    val media = items[safeIndex]

    LaunchedEffect(items.map(MediaCard::resolvedId), safeIndex) {
        if (items.size <= 1) return@LaunchedEffect
        delay(HOME_HERO_INTERVAL_MS)
        index = (safeIndex + 1) % items.size
    }

    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .height(500.dp)
            .pointerInput(items.size, safeIndex) {
                detectHorizontalDragGestures(
                    onDragStart = { dragDistance = 0f },
                    onHorizontalDrag = { _, amount -> dragDistance += amount },
                    onDragEnd = {
                        if (dragDistance > HOME_HERO_SWIPE_THRESHOLD && items.size > 1) {
                            index = (safeIndex - 1 + items.size) % items.size
                        } else if (dragDistance < -HOME_HERO_SWIPE_THRESHOLD && items.size > 1) {
                            index = (safeIndex + 1) % items.size
                        }
                        dragDistance = 0f
                    },
                )
            },
        shape = RoundedCornerShape(18.dp),
        color = Color(0xFF080B16),
        tonalElevation = 0.dp,
        shadowElevation = 6.dp,
    ) {
        AnimatedContent(
            targetState = media,
            transitionSpec = { androidx.compose.animation.fadeIn(tween(220)) togetherWith androidx.compose.animation.fadeOut(tween(180)) },
            label = "homeHero",
        ) { item ->
            val backdrop = resolveImage(baseUrl, item.resolvedBackdrop)
            val poster = resolveImage(baseUrl, item.resolvedPoster)
            val artwork = backdrop ?: poster
            val posterFallback = backdrop == null && poster != null

            Box(Modifier.fillMaxSize()) {
                if (artwork != null) {
                    AsyncImage(
                        model = artwork,
                        contentDescription = null,
                        contentScale = ContentScale.Crop,
                        modifier = Modifier
                            .fillMaxSize()
                            .then(
                                if (posterFallback) {
                                    Modifier
                                        .graphicsLayer { scaleX = 1.16f; scaleY = 1.16f }
                                        .blur(18.dp)
                                } else Modifier,
                            ),
                    )
                }

                if (posterFallback) {
                    Box(Modifier.fillMaxSize().background(Color.Black.copy(alpha = 0.28f)))
                }
                Box(
                    Modifier
                        .fillMaxSize()
                        .background(
                            Brush.verticalGradient(
                                0f to Color(0x44060810),
                                0.34f to Color(0x12060810),
                                0.58f to Color(0x77070A16),
                                1f to Color(0xFC070A16),
                            ),
                        ),
                )

                Column(
                    modifier = Modifier
                        .align(Alignment.BottomStart)
                        .fillMaxWidth()
                        .padding(start = 18.dp, end = 18.dp, bottom = 42.dp),
                ) {
                    Text(
                        item.displayTitle,
                        color = Color.White,
                        fontSize = 29.sp,
                        lineHeight = 32.sp,
                        fontWeight = FontWeight.Bold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.fillMaxWidth(),
                    )
                    if (item.originalTitle.isNotBlank() && item.originalTitle != item.displayTitle) {
                        Spacer(Modifier.height(6.dp))
                        Text(
                            item.originalTitle,
                            color = Color.White.copy(alpha = 0.76f),
                            style = MaterialTheme.typography.bodyMedium,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }

                    Spacer(Modifier.height(10.dp))
                    HeroMetadata(item)

                    val overview = heroOverview(item)
                    if (overview.isNotBlank()) {
                        Spacer(Modifier.height(10.dp))
                        Text(
                            overview,
                            color = Color.White.copy(alpha = 0.76f),
                            style = MaterialTheme.typography.bodyMedium,
                            lineHeight = 20.sp,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }

                    Spacer(Modifier.height(14.dp))
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        horizontalArrangement = Arrangement.spacedBy(8.dp),
                    ) {
                        Button(
                            onClick = { onPlay(item.resolvedId) },
                            modifier = Modifier.weight(1f).height(44.dp),
                            shape = RoundedCornerShape(10.dp),
                        ) {
                            Icon(Icons.Default.PlayArrow, null)
                            Spacer(Modifier.width(5.dp))
                            Text(if (item.normalizedProgress > 0f) "继续播放" else "立即播放")
                        }
                        OutlinedButton(
                            onClick = { onPlay(item.resolvedId) },
                            modifier = Modifier.weight(1f).height(44.dp),
                            shape = RoundedCornerShape(10.dp),
                            colors = ButtonDefaults.outlinedButtonColors(contentColor = Color.White),
                            border = androidx.compose.foundation.BorderStroke(1.dp, Color.White.copy(alpha = 0.28f)),
                        ) {
                            Icon(Icons.Default.Replay, null)
                            Spacer(Modifier.width(5.dp))
                            Text("从头播放")
                        }
                    }
                    Spacer(Modifier.height(8.dp))
                    OutlinedButton(
                        onClick = onFavoritesClick,
                        modifier = Modifier.fillMaxWidth().height(42.dp),
                        shape = RoundedCornerShape(10.dp),
                        colors = ButtonDefaults.outlinedButtonColors(contentColor = Color.White),
                        border = androidx.compose.foundation.BorderStroke(1.dp, Color.White.copy(alpha = 0.22f)),
                    ) {
                        Icon(Icons.Default.FavoriteBorder, null)
                        Spacer(Modifier.width(6.dp))
                        Text("收藏")
                    }
                }

                if (items.size > 1) {
                    Row(
                        modifier = Modifier
                            .align(Alignment.BottomCenter)
                            .padding(bottom = 14.dp),
                        horizontalArrangement = Arrangement.spacedBy(6.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        items.indices.forEach { dot ->
                            Box(
                                Modifier
                                    .width(if (dot == safeIndex) 20.dp else 6.dp)
                                    .height(6.dp)
                                    .clip(CircleShape)
                                    .background(
                                        if (dot == safeIndex) MaterialTheme.colorScheme.primary
                                        else Color.White.copy(alpha = 0.24f),
                                    ),
                            )
                        }
                    }
                }

                // 整个画面保持可打开详情，但按钮区域仍优先处理自己的点击。
                Surface(
                    onClick = { onOpen(item.resolvedId) },
                    modifier = Modifier
                        .align(Alignment.TopEnd)
                        .padding(12.dp),
                    shape = CircleShape,
                    color = Color.Black.copy(alpha = 0.28f),
                    contentColor = Color.White,
                ) {
                    Icon(
                        Icons.Default.ChevronRight,
                        contentDescription = "查看详情",
                        modifier = Modifier.padding(8.dp).size(18.dp),
                    )
                }
            }
        }
    }
}

@Composable
private fun HeroMetadata(media: MediaCard) {
    Row(
        horizontalArrangement = Arrangement.spacedBy(9.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (media.rating > 0) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.Default.Star, null, tint = Color(0xFFFFC53D), modifier = Modifier.size(15.dp))
                Spacer(Modifier.width(3.dp))
                Text("%.1f".format(media.rating), color = Color(0xFFFFC53D), fontWeight = FontWeight.SemiBold)
            }
        }
        media.year?.takeIf { it > 0 }?.let { Text(it.toString(), color = Color.White.copy(alpha = 0.82f)) }
        if (media.runtime > 0) Text(formatRuntime(media.runtime), color = Color.White.copy(alpha = 0.82f))
        media.resolution.takeIf(String::isNotBlank)?.let {
            HeroChip(it)
        }
        media.genres.split(',').map(String::trim).firstOrNull(String::isNotBlank)?.let {
            HeroChip(it)
        }
    }
}

@Composable
private fun HeroChip(label: String) {
    Surface(
        shape = RoundedCornerShape(7.dp),
        color = Color(0x33202A45),
        border = androidx.compose.foundation.BorderStroke(1.dp, Color.White.copy(alpha = 0.12f)),
    ) {
        Text(
            label,
            modifier = Modifier.padding(horizontal = 7.dp, vertical = 3.dp),
            color = Color.White.copy(alpha = 0.88f),
            fontSize = 10.sp,
        )
    }
}

@Composable
private fun HomeSectionHeader(
    title: String,
    action: String,
    onAction: () -> Unit,
    leading: (@Composable () -> Unit)? = null,
) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (leading != null) {
            leading()
            Spacer(Modifier.width(7.dp))
        }
        Text(title, style = MaterialTheme.typography.titleLarge, modifier = Modifier.weight(1f))
        TextButton(onClick = onAction, contentPadding = PaddingValues(horizontal = 5.dp, vertical = 0.dp)) {
            Text(action, fontSize = 12.sp)
            Icon(Icons.Default.ChevronRight, null, modifier = Modifier.size(17.dp))
        }
    }
}

@Composable
private fun ContinueWatchingCard(
    media: MediaCard,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(modifier = Modifier.width(300.dp)) {
        Surface(
            onClick = onClick,
            modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f),
            shape = RoundedCornerShape(12.dp),
            color = MaterialTheme.colorScheme.surfaceVariant,
            tonalElevation = 0.dp,
        ) {
            Box {
                AsyncImage(
                    model = imageUrl,
                    contentDescription = media.displayTitle,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
                Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent, Color.Black.copy(alpha = 0.34f)))))
                Surface(
                    modifier = Modifier.align(Alignment.Center),
                    shape = CircleShape,
                    color = Color.Black.copy(alpha = 0.55f),
                    contentColor = Color.White,
                ) {
                    Icon(Icons.Default.PlayArrow, null, modifier = Modifier.padding(10.dp).size(20.dp))
                }
                val percent = (media.normalizedProgress * 100).roundToInt()
                Surface(
                    modifier = Modifier.align(Alignment.TopEnd).padding(8.dp),
                    shape = RoundedCornerShape(7.dp),
                    color = Color.Black.copy(alpha = 0.62f),
                    contentColor = Color.White,
                ) {
                    Text("$percent%", modifier = Modifier.padding(horizontal = 7.dp, vertical = 3.dp), fontSize = 10.sp)
                }
                LinearProgressIndicator(
                    progress = { media.normalizedProgress },
                    modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(3.dp),
                    color = MaterialTheme.colorScheme.primary,
                    trackColor = Color.White.copy(alpha = 0.18f),
                )
            }
        }
        Spacer(Modifier.height(7.dp))
        Text(media.displayTitle, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.titleMedium)
        val secondary = media.episodeTitle.takeIf(String::isNotBlank)
            ?: "已观看 ${(media.normalizedProgress * 100).roundToInt()}%"
        Text(
            secondary,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun LandscapeMediaCard(
    media: MediaCard,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(modifier = Modifier.width(190.dp)) {
        Surface(
            onClick = onClick,
            modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f),
            shape = RoundedCornerShape(11.dp),
            color = MaterialTheme.colorScheme.surfaceVariant,
        ) {
            Box {
                AsyncImage(
                    model = imageUrl,
                    contentDescription = media.displayTitle,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
                Surface(
                    modifier = Modifier.align(Alignment.Center),
                    shape = CircleShape,
                    color = Color.Black.copy(alpha = 0.48f),
                    contentColor = Color.White,
                ) {
                    Icon(Icons.Default.PlayArrow, null, modifier = Modifier.padding(8.dp).size(17.dp))
                }
            }
        }
        Spacer(Modifier.height(7.dp))
        Text(media.displayTitle, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.titleMedium)
        Text(
            listOfNotNull(
                media.year?.takeIf { it > 0 }?.toString(),
                media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
            ).joinToString(" · "),
            maxLines = 1,
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun PosterShelf(
    title: String,
    items: List<MediaCard>,
    baseUrl: String?,
    onMore: () -> Unit,
    onMediaClick: (String) -> Unit,
) {
    Column {
        HomeSectionHeader(title = title, action = "更多", onAction = onMore)
        Spacer(Modifier.height(10.dp))
        LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
            items(items.take(16), key = { "$title-${it.resolvedId}" }) { media ->
                PosterMediaCard(
                    media = media,
                    imageUrl = resolveHomeArtwork(baseUrl, media, preferBackdrop = false),
                    onClick = { onMediaClick(media.resolvedId) },
                )
            }
        }
    }
}

@Composable
private fun PosterMediaCard(
    media: MediaCard,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(modifier = Modifier.width(106.dp)) {
        Surface(
            onClick = onClick,
            modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f),
            shape = RoundedCornerShape(10.dp),
            color = MaterialTheme.colorScheme.surfaceVariant,
        ) {
            Box {
                AsyncImage(
                    model = imageUrl,
                    contentDescription = media.displayTitle,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
                if (media.resolution.isNotBlank()) {
                    Surface(
                        modifier = Modifier.align(Alignment.TopEnd).padding(5.dp),
                        shape = RoundedCornerShape(6.dp),
                        color = Color.Black.copy(alpha = 0.58f),
                        contentColor = Color.White,
                    ) {
                        Text(media.resolution, modifier = Modifier.padding(horizontal = 5.dp, vertical = 2.dp), fontSize = 9.sp)
                    }
                }
            }
        }
        Spacer(Modifier.height(6.dp))
        Text(media.displayTitle, maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
        val meta = listOfNotNull(
            media.year?.takeIf { it > 0 }?.toString(),
            media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
        ).joinToString(" · ")
        if (meta.isNotBlank()) {
            Text(meta, maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 9.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

private fun resolveHomeArtwork(baseUrl: String?, media: MediaCard, preferBackdrop: Boolean): String? {
    val relative = if (preferBackdrop) media.resolvedBackdrop ?: media.resolvedPoster else media.resolvedPoster
    return resolveImage(baseUrl, relative)
}

private fun heroOverview(media: MediaCard): String = when {
    media.episodeTitle.isNotBlank() -> media.episodeTitle
    media.genres.isNotBlank() -> "${media.genres.replace(',', ' · ')} · 在 Nowen Video 中继续探索这部作品。"
    else -> "在 Nowen Video 中继续探索这部作品。"
}

private fun formatRuntime(minutes: Int): String = if (minutes >= 60) {
    val hours = minutes / 60
    val rest = minutes % 60
    if (rest == 0) "${hours}小时" else "${hours}小时${rest}分钟"
} else {
    "${minutes}分钟"
}

internal fun resolveImage(baseUrl: String?, path: String?): String? {
    if (path.isNullOrBlank()) return null
    if (path.startsWith("http://") || path.startsWith("https://")) return path
    return baseUrl?.trimEnd('/') + "/" + path.trimStart('/')
}
