package com.nowen.video.v2.feature.main

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.BorderStroke
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.BoxWithConstraints
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ColumnScope
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
import androidx.compose.material.icons.filled.Replay
import androidx.compose.material.icons.filled.Star
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.Icon
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
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import coil.compose.AsyncImage
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaCard
import kotlinx.coroutines.delay
import kotlin.math.roundToInt

private const val WEB_MOBILE_HERO_INTERVAL_MS = 7_000L
private const val WEB_MOBILE_HERO_SWIPE_THRESHOLD = 54f
private val WEB_MOBILE_HOME_GENRES = listOf("动画", "喜剧", "冒险", "家庭")

/** Android 首页与当前 Web 移动端共享同一视觉层级。 */
@Composable
fun WebMobileHomeScreen(
    modifier: Modifier = Modifier,
    onMediaClick: (String) -> Unit,
    onPlay: (String) -> Unit,
    onRestart: (String) -> Unit = onPlay,
    onLibraryClick: () -> Unit,
    onHistoryClick: () -> Unit = {},
    onFavoritesClick: () -> Unit = {},
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.store.snapshot.collectAsState()
    val baseUrl = session.activeServer?.baseUrl

    LazyColumn(
        modifier = modifier.fillMaxSize().background(MaterialTheme.colorScheme.background),
        contentPadding = PaddingValues(start = 10.dp, end = 10.dp, top = 10.dp, bottom = 20.dp),
        verticalArrangement = Arrangement.spacedBy(18.dp),
    ) {
        if (state.loading) {
            item {
                Surface(
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(18.dp),
                    color = MaterialTheme.colorScheme.surface,
                    border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant),
                ) {
                    Column(Modifier.padding(16.dp)) {
                        LinearProgressIndicator(Modifier.fillMaxWidth())
                        Spacer(Modifier.height(10.dp))
                        Text("正在同步首页内容", color = MaterialTheme.colorScheme.onSurfaceVariant, fontSize = 12.sp)
                    }
                }
            }
        }

        state.error?.let { message ->
            item { MessagePanel("暂时无法加载", message, "重试", viewModel::refresh) }
        }

        if (state.content.heroItems.isNotEmpty()) {
            item {
                WebMobileHero(
                    items = state.content.heroItems,
                    baseUrl = baseUrl,
                    onPlay = onPlay,
                    onRestart = onRestart,
                    onFavoritesClick = onFavoritesClick,
                )
            }
        }

        if (state.content.continueWatching.isNotEmpty()) {
            item {
                WebMobileSectionSurface {
                    WebMobileSectionHeader(
                        title = "继续观看",
                        action = "查看全部",
                        leading = { Icon(Icons.Default.History, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(16.dp)) },
                        onAction = onHistoryClick,
                    )
                    Spacer(Modifier.height(10.dp))
                    BoxWithConstraints(Modifier.fillMaxWidth()) {
                        val cardWidth = twoAcrossWidth(maxWidth)
                        LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            items(state.content.continueWatching, key = { "continue-${it.resolvedId}" }) { media ->
                                WebMobileLandscapeCard(
                                    media = media,
                                    imageUrl = webMobileArtwork(baseUrl, media, preferBackdrop = true),
                                    width = cardWidth,
                                    progress = media.normalizedProgress,
                                    onClick = { onPlay(media.resolvedId) },
                                )
                            }
                        }
                    }
                }
            }
        }

        if (state.content.recommendations.isNotEmpty()) {
            item {
                WebMobileSectionSurface {
                    WebMobileSectionHeader(title = "为你推荐", action = "查看全部", onAction = onLibraryClick)
                    Spacer(Modifier.height(10.dp))
                    BoxWithConstraints(Modifier.fillMaxWidth()) {
                        val cardWidth = twoAcrossWidth(maxWidth)
                        LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                            items(state.content.recommendations, key = { "recommend-${it.resolvedId}" }) { media ->
                                WebMobileLandscapeCard(
                                    media = media,
                                    imageUrl = webMobileArtwork(baseUrl, media, preferBackdrop = true),
                                    width = cardWidth,
                                    onClick = { onMediaClick(media.resolvedId) },
                                )
                            }
                        }
                    }
                }
            }
        }

        if (state.content.recent.isNotEmpty()) {
            item {
                WebMobilePosterShelf(
                    title = "最近添加",
                    items = state.content.recent,
                    baseUrl = baseUrl,
                    onMore = onLibraryClick,
                    onMediaClick = onMediaClick,
                )
            }
        }

        WEB_MOBILE_HOME_GENRES.forEach { genre ->
            val media = state.content.genreShelves[genre].orEmpty()
            if (media.isNotEmpty()) {
                item(key = "web-mobile-genre-$genre") {
                    WebMobilePosterShelf(
                        title = genre,
                        items = media,
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
private fun WebMobileHero(
    items: List<MediaCard>,
    baseUrl: String?,
    onPlay: (String) -> Unit,
    onRestart: (String) -> Unit,
    onFavoritesClick: () -> Unit,
) {
    var index by remember(items.map(MediaCard::resolvedId)) { mutableIntStateOf(0) }
    var dragDistance by remember { mutableStateOf(0f) }
    val safeIndex = index.coerceIn(0, items.lastIndex)
    val media = items[safeIndex]

    LaunchedEffect(items.map(MediaCard::resolvedId), safeIndex) {
        if (items.size <= 1) return@LaunchedEffect
        delay(WEB_MOBILE_HERO_INTERVAL_MS)
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
                        if (items.size > 1 && dragDistance > WEB_MOBILE_HERO_SWIPE_THRESHOLD) {
                            index = (safeIndex - 1 + items.size) % items.size
                        } else if (items.size > 1 && dragDistance < -WEB_MOBILE_HERO_SWIPE_THRESHOLD) {
                            index = (safeIndex + 1) % items.size
                        }
                        dragDistance = 0f
                    },
                )
            },
        shape = RoundedCornerShape(18.dp),
        color = Color(0xFF080B16),
        shadowElevation = 2.dp,
        tonalElevation = 0.dp,
    ) {
        AnimatedContent(
            targetState = media,
            transitionSpec = { fadeIn(tween(220)) togetherWith fadeOut(tween(180)) },
            label = "webMobileHero",
        ) { item ->
            val backdrop = resolveImage(baseUrl, item.resolvedBackdrop)
            val poster = resolveImage(baseUrl, item.resolvedPoster)
            var backdropFailed by remember(item.resolvedId, backdrop) { mutableStateOf(false) }
            val posterFallback = backdrop == null || backdropFailed
            val artwork = if (posterFallback) poster else backdrop

            Box(Modifier.fillMaxSize()) {
                if (artwork != null) {
                    AsyncImage(
                        model = artwork,
                        contentDescription = null,
                        contentScale = ContentScale.Crop,
                        onError = { if (!posterFallback && poster != null) backdropFailed = true },
                        modifier = Modifier
                            .fillMaxSize()
                            .then(
                                if (posterFallback) Modifier.graphicsLayer {
                                    scaleX = 1.18f
                                    scaleY = 1.18f
                                }.blur(18.dp) else Modifier,
                            ),
                    )
                }

                Box(
                    Modifier
                        .fillMaxSize()
                        .background(
                            Brush.verticalGradient(
                                0f to Color(0x14050812),
                                0.28f to Color(0x08050812),
                                0.50f to Color(0x29070A16),
                                0.72f to Color(0x8A070A16),
                                1f to Color(0xF0070A16),
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
                        fontSize = 28.sp,
                        lineHeight = 31.sp,
                        fontWeight = FontWeight.Bold,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    if (item.originalTitle.isNotBlank() && item.originalTitle != item.displayTitle) {
                        Spacer(Modifier.height(5.dp))
                        Text(
                            item.originalTitle,
                            color = Color.White.copy(alpha = 0.76f),
                            fontSize = 13.sp,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }
                    Spacer(Modifier.height(9.dp))
                    WebMobileHeroMetadata(item)

                    val overview = webMobileHeroOverview(item)
                    if (overview.isNotBlank()) {
                        Spacer(Modifier.height(9.dp))
                        Text(
                            overview,
                            color = Color.White.copy(alpha = 0.78f),
                            fontSize = 13.sp,
                            lineHeight = 19.sp,
                            maxLines = 2,
                            overflow = TextOverflow.Ellipsis,
                        )
                    }

                    Spacer(Modifier.height(14.dp))
                    Row(Modifier.fillMaxWidth(), horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        Button(
                            onClick = { onPlay(item.resolvedId) },
                            modifier = Modifier.weight(1f).height(46.dp),
                            shape = RoundedCornerShape(11.dp),
                            contentPadding = PaddingValues(horizontal = 8.dp),
                        ) {
                            Icon(Icons.Default.PlayArrow, null, modifier = Modifier.size(20.dp))
                            Spacer(Modifier.width(4.dp))
                            Text(if (item.normalizedProgress > 0f) "继续播放" else "立即播放", fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        }
                        OutlinedButton(
                            onClick = { onRestart(item.resolvedId) },
                            modifier = Modifier.weight(1f).height(46.dp),
                            shape = RoundedCornerShape(11.dp),
                            contentPadding = PaddingValues(horizontal = 8.dp),
                            colors = ButtonDefaults.outlinedButtonColors(
                                containerColor = Color.White.copy(alpha = 0.94f),
                                contentColor = Color(0xFF343842),
                            ),
                            border = BorderStroke(1.dp, Color.White.copy(alpha = 0.70f)),
                        ) {
                            Icon(Icons.Default.Replay, null, modifier = Modifier.size(19.dp))
                            Spacer(Modifier.width(4.dp))
                            Text("从头播放", fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                        }
                    }
                    Spacer(Modifier.height(8.dp))
                    OutlinedButton(
                        onClick = onFavoritesClick,
                        modifier = Modifier.fillMaxWidth().height(44.dp),
                        shape = RoundedCornerShape(11.dp),
                        colors = ButtonDefaults.outlinedButtonColors(
                            containerColor = Color.White.copy(alpha = 0.94f),
                            contentColor = Color(0xFF4A4E58),
                        ),
                        border = BorderStroke(1.dp, Color.White.copy(alpha = 0.68f)),
                    ) {
                        Icon(Icons.Default.FavoriteBorder, null, modifier = Modifier.size(19.dp))
                        Spacer(Modifier.width(5.dp))
                        Text("收藏", fontSize = 13.sp, fontWeight = FontWeight.SemiBold)
                    }
                }

                if (items.size > 1) {
                    Row(
                        modifier = Modifier.align(Alignment.BottomCenter).padding(bottom = 14.dp),
                        horizontalArrangement = Arrangement.spacedBy(5.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        items.indices.forEach { dot ->
                            Box(
                                Modifier
                                    .width(if (dot == safeIndex) 18.dp else 5.dp)
                                    .height(5.dp)
                                    .clip(CircleShape)
                                    .background(
                                        if (dot == safeIndex) MaterialTheme.colorScheme.primary
                                        else Color.White.copy(alpha = 0.28f),
                                    ),
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun WebMobileHeroMetadata(media: MediaCard) {
    Row(
        horizontalArrangement = Arrangement.spacedBy(7.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        if (media.rating > 0) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Icon(Icons.Default.Star, null, tint = Color(0xFFFFC84A), modifier = Modifier.size(14.dp))
                Spacer(Modifier.width(2.dp))
                Text("%.1f".format(media.rating), color = Color(0xFFFFC84A), fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
            }
        }
        media.year?.takeIf { it > 0 }?.let { Text(it.toString(), color = Color.White.copy(alpha = 0.82f), fontSize = 11.sp) }
        if (media.runtime > 0) Text(webMobileRuntime(media.runtime), color = Color.White.copy(alpha = 0.82f), fontSize = 11.sp)
        media.resolution.takeIf(String::isNotBlank)?.let { WebMobileHeroChip(it) }
        media.genres.split(',').map(String::trim).firstOrNull(String::isNotBlank)?.let { WebMobileHeroChip(it) }
    }
}

@Composable
private fun WebMobileHeroChip(label: String) {
    Surface(
        shape = RoundedCornerShape(6.dp),
        color = Color.Black.copy(alpha = 0.32f),
        border = BorderStroke(1.dp, Color.White.copy(alpha = 0.10f)),
    ) {
        Text(label, modifier = Modifier.padding(horizontal = 6.dp, vertical = 3.dp), color = Color.White.copy(alpha = 0.90f), fontSize = 9.sp)
    }
}

@Composable
private fun WebMobileSectionSurface(content: @Composable ColumnScope.() -> Unit) {
    Surface(
        modifier = Modifier.fillMaxWidth(),
        shape = RoundedCornerShape(18.dp),
        color = MaterialTheme.colorScheme.surface,
        tonalElevation = 0.dp,
        shadowElevation = 0.dp,
        border = BorderStroke(1.dp, MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.72f)),
    ) {
        Column(Modifier.padding(horizontal = 14.dp, vertical = 15.dp), content = content)
    }
}

@Composable
private fun WebMobileSectionHeader(
    title: String,
    action: String,
    onAction: () -> Unit,
    leading: (@Composable () -> Unit)? = null,
) {
    Row(Modifier.fillMaxWidth(), verticalAlignment = Alignment.CenterVertically) {
        Box(
            Modifier
                .width(4.dp)
                .height(22.dp)
                .clip(RoundedCornerShape(999.dp))
                .background(MaterialTheme.colorScheme.primary),
        )
        Spacer(Modifier.width(9.dp))
        if (leading != null) {
            leading()
            Spacer(Modifier.width(5.dp))
        }
        Text(
            title,
            modifier = Modifier.weight(1f),
            fontSize = 16.sp,
            lineHeight = 20.sp,
            fontWeight = FontWeight.Bold,
            color = MaterialTheme.colorScheme.onSurface,
        )
        TextButton(onClick = onAction, contentPadding = PaddingValues(horizontal = 3.dp, vertical = 0.dp)) {
            Text(action, fontSize = 11.sp, fontWeight = FontWeight.SemiBold)
            Icon(Icons.Default.ChevronRight, null, modifier = Modifier.size(15.dp))
        }
    }
}

@Composable
private fun WebMobileLandscapeCard(
    media: MediaCard,
    imageUrl: String?,
    width: Dp,
    progress: Float? = null,
    onClick: () -> Unit,
) {
    Column(modifier = Modifier.width(width)) {
        Surface(
            onClick = onClick,
            modifier = Modifier.fillMaxWidth().aspectRatio(16f / 9f),
            shape = RoundedCornerShape(10.dp),
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
                Surface(
                    modifier = Modifier.align(Alignment.Center),
                    shape = CircleShape,
                    color = Color.Black.copy(alpha = 0.52f),
                    contentColor = Color.White,
                ) {
                    Icon(Icons.Default.PlayArrow, null, modifier = Modifier.padding(8.dp).size(17.dp))
                }
                progress?.takeIf { it > 0f }?.let { value ->
                    LinearProgressIndicator(
                        progress = { value },
                        modifier = Modifier.align(Alignment.BottomCenter).fillMaxWidth().height(3.dp),
                        color = MaterialTheme.colorScheme.primary,
                        trackColor = Color.White.copy(alpha = 0.18f),
                    )
                }
            }
        }
        Spacer(Modifier.height(6.dp))
        Text(media.displayTitle, maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 12.sp, lineHeight = 16.sp, fontWeight = FontWeight.SemiBold)
        val meta = if (progress != null) {
            "已观看 ${(media.normalizedProgress * 100).roundToInt()}%"
        } else {
            listOfNotNull(
                media.year?.takeIf { it > 0 }?.toString(),
                media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
            ).joinToString(" · ")
        }
        if (meta.isNotBlank()) {
            Text(meta, maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 10.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

@Composable
private fun WebMobilePosterShelf(
    title: String,
    items: List<MediaCard>,
    baseUrl: String?,
    onMore: () -> Unit,
    onMediaClick: (String) -> Unit,
) {
    WebMobileSectionSurface {
        WebMobileSectionHeader(title = title, action = "更多", onAction = onMore)
        Spacer(Modifier.height(10.dp))
        BoxWithConstraints(Modifier.fillMaxWidth()) {
            val cardWidth = threeAcrossWidth(maxWidth)
            LazyRow(horizontalArrangement = Arrangement.spacedBy(10.dp)) {
                items(items.take(16), key = { "$title-${it.resolvedId}" }) { media ->
                    WebMobilePosterCard(
                        media = media,
                        imageUrl = webMobileArtwork(baseUrl, media, preferBackdrop = false),
                        width = cardWidth,
                        onClick = { onMediaClick(media.resolvedId) },
                    )
                }
            }
        }
    }
}

@Composable
private fun WebMobilePosterCard(
    media: MediaCard,
    imageUrl: String?,
    width: Dp,
    onClick: () -> Unit,
) {
    Column(modifier = Modifier.width(width)) {
        Surface(
            onClick = onClick,
            modifier = Modifier.fillMaxWidth().aspectRatio(2f / 3f),
            shape = RoundedCornerShape(9.dp),
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
                if (media.resolution.isNotBlank()) {
                    Surface(
                        modifier = Modifier.align(Alignment.TopEnd).padding(5.dp),
                        shape = RoundedCornerShape(6.dp),
                        color = Color.Black.copy(alpha = 0.60f),
                        contentColor = Color.White,
                    ) {
                        Text(media.resolution, modifier = Modifier.padding(horizontal = 5.dp, vertical = 2.dp), fontSize = 9.sp)
                    }
                }
            }
        }
        Spacer(Modifier.height(5.dp))
        Text(media.displayTitle, maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 11.sp, lineHeight = 14.sp, fontWeight = FontWeight.SemiBold)
        val meta = listOfNotNull(
            media.year?.takeIf { it > 0 }?.toString(),
            media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
        ).joinToString(" · ")
        if (meta.isNotBlank()) {
            Text(meta, maxLines = 1, overflow = TextOverflow.Ellipsis, fontSize = 9.sp, lineHeight = 12.sp, color = MaterialTheme.colorScheme.onSurfaceVariant)
        }
    }
}

private fun twoAcrossWidth(available: Dp): Dp = (available - 10.dp) / 2
private fun threeAcrossWidth(available: Dp): Dp = (available - 20.dp) / 3

private fun webMobileArtwork(baseUrl: String?, media: MediaCard, preferBackdrop: Boolean): String? {
    val path = if (preferBackdrop) media.resolvedBackdrop ?: media.resolvedPoster else media.resolvedPoster
    return resolveImage(baseUrl, path)
}

private fun webMobileHeroOverview(media: MediaCard): String = when {
    media.overview.isNotBlank() -> media.overview
    media.episodeTitle.isNotBlank() -> media.episodeTitle
    media.genres.isNotBlank() -> "${media.genres.replace(",", " · ")} · 在 Nowen Video 中继续探索这部作品。"
    else -> "在 Nowen Video 中继续探索这部作品。"
}

private fun webMobileRuntime(minutes: Int): String = if (minutes >= 60) {
    val hours = minutes / 60
    val rest = minutes % 60
    if (rest == 0) "${hours}小时" else "${hours}小时${rest}分钟"
} else {
    "${minutes}分钟"
}
