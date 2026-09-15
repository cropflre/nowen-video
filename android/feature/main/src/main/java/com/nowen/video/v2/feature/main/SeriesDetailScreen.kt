@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SuggestionChip
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SeriesRepository
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.HillsPressable
import com.nowen.video.v2.core.designsystem.HillsPrimaryAction
import com.nowen.video.v2.core.designsystem.HillsState
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.SeasonInfo
import com.nowen.video.v2.core.model.SeriesBundle
import com.nowen.video.v2.core.model.WatchHistoryRecord
import com.nowen.video.v2.core.model.episodeDisplayName
import com.nowen.video.v2.core.model.userEpisodeTitle
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class SeriesDetailUiState(
    val loading: Boolean = true,
    val bundle: SeriesBundle? = null,
    val history: List<WatchHistoryRecord> = emptyList(),
    val error: String? = null,
) {
    val orderedEpisodes: List<MediaDetail>
        get() = bundle?.seasons.orEmpty()
            .sortedWith(compareBy<SeasonInfo> { it.seasonNumber == 0 }.thenBy { it.seasonNumber })
            .flatMap(SeasonInfo::episodes)

    val firstEpisode: MediaDetail?
        get() = orderedEpisodes.firstOrNull()

    val historyByMediaId: Map<String, WatchHistoryRecord>
        get() = history.associateBy { it.resolvedMediaId }

    val continueEpisode: MediaDetail?
        get() {
            val episodes = orderedEpisodes
            if (episodes.isEmpty()) return null
            val ids = episodes.mapTo(hashSetOf()) { it.id }
            val inProgressId = history.firstOrNull {
                it.resolvedMediaId in ids && !it.completed && it.normalizedProgress in 0.01f..0.949f
            }?.resolvedMediaId
            if (inProgressId != null) return episodes.firstOrNull { it.id == inProgressId }
            val byId = historyByMediaId
            return episodes.firstOrNull { byId[it.id]?.completed != true } ?: episodes.first()
        }

    val watchedCount: Int
        get() = orderedEpisodes.count { historyByMediaId[it.id]?.completed == true }

    val inProgressCount: Int
        get() = orderedEpisodes.count {
            val progress = historyByMediaId[it.id]
            progress != null && !progress.completed && progress.normalizedProgress > 0.01f
        }

    val continueActionLabel: String
        get() {
            val episode = continueEpisode ?: return "暂无可播放单集"
            val progress = historyByMediaId[episode.id]
            return when {
                progress != null && !progress.completed && progress.normalizedProgress > 0.01f ->
                    "继续播放 ${episodeDisplayName(episode.seasonNumber, episode.episodeNumber)}"
                watchedCount >= orderedEpisodes.size && orderedEpisodes.isNotEmpty() ->
                    "重新播放 ${episodeDisplayName(episode.seasonNumber, episode.episodeNumber)}"
                else -> "播放 ${episodeDisplayName(episode.seasonNumber, episode.episodeNumber)}"
            }
        }
}

private val WatchHistoryRecord.resolvedMediaId: String
    get() = mediaId.ifBlank { media.resolvedId }

internal fun ensureDefaultSeason(bundle: SeriesBundle): SeriesBundle =
    bundle.copy(
        seasons = bundle.seasons
            .distinctBy(SeasonInfo::seasonNumber)
            .sortedWith(compareBy<SeasonInfo> { it.seasonNumber == 0 }.thenBy { it.seasonNumber }),
    )

internal fun seriesHeroMetadata(
    series: com.nowen.video.v2.core.model.SeriesInfo,
    selectedSeason: SeasonInfo? = null,
    actualSeasonCount: Int = series.seasonCount,
): String =
    listOfNotNull(
        series.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
        series.year.takeIf { it > 0 }?.toString(),
        series.genreList.takeIf { it.isNotEmpty() }?.joinToString("、"),
        selectedSeason?.let { "${it.label} · ${it.episodes.size} 集" },
        actualSeasonCount.takeIf { it > 0 }?.let { "共 $it 季" },
        if (selectedSeason == null) series.episodeCount.takeIf { it > 0 }?.let { "$it 集" } else null,
    ).joinToString(" · ")

private suspend fun loadAllHistory(repository: SocialCatalogRepository): List<WatchHistoryRecord> {
    val pageSize = 50
    val first = repository.history(page = 1, size = pageSize).getOrElse { return emptyList() }
    val totalPages = ((first.total + pageSize - 1) / pageSize).coerceAtLeast(1)
    if (totalPages == 1) return first.data
    val remaining = (2..totalPages).mapNotNull { page ->
        repository.history(page = page, size = pageSize).getOrNull()?.data
    }
    return buildList {
        addAll(first.data)
        remaining.forEach(::addAll)
    }
}

@HiltViewModel
class SeriesDetailViewModel @Inject constructor(
    private val repository: SeriesRepository,
    private val socialRepository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(SeriesDetailUiState())
    val state: StateFlow<SeriesDetailUiState> = _state
    private var loadedId: String? = null

    fun load(id: String) {
        if (id.isBlank()) return
        if (loadedId == id && _state.value.bundle != null) return
        loadedId = id
        viewModelScope.launch {
            _state.value = SeriesDetailUiState(loading = true)
            runCatching {
                coroutineScope {
                    val bundleDeferred = async { repository.load(id).getOrThrow() }
                    val historyDeferred = async { loadAllHistory(socialRepository) }
                    val bundle = ensureDefaultSeason(bundleDeferred.await())
                    val history = historyDeferred.await()
                    val provisional = SeriesDetailUiState(
                        loading = false,
                        bundle = bundle,
                        history = history,
                    )
                    provisional
                }
            }.onSuccess { next ->
                _state.value = next
            }.onFailure { error ->
                _state.value = SeriesDetailUiState(
                    loading = false,
                    error = error.message ?: "剧集详情加载失败",
                )
            }
        }
    }

}

@Composable
fun SeriesDetailScreen(
    seriesId: String,
    onBack: () -> Unit,
    onEpisodeClick: (String) -> Unit,
    onPlayEpisode: (String) -> Unit,
    onGenreClick: (String) -> Unit = {},
    onPersonClick: (String) -> Unit,
    onSeasonClick: (Int) -> Unit,
    viewModel: SeriesDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    LaunchedEffect(seriesId) { viewModel.load(seriesId) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color(0xFF3A281E)),
    ) {
        when {
            state.loading -> CircularProgressIndicator(Modifier.align(Alignment.Center))
            state.error != null -> HillsState(
                title = "无法打开剧集",
                message = state.error!!,
                actionLabel = "重试",
                onAction = { viewModel.load(seriesId) },
                modifier = Modifier.align(Alignment.Center),
            )
            state.bundle != null -> {
                val bundle = state.bundle!!
                val series = bundle.series
                val displaySeasons = bundle.seasons.sortedWith(
                    compareBy<SeasonInfo> { it.seasonNumber == 0 }.thenBy { it.seasonNumber },
                )
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(bottom = 36.dp),
                ) {
                    item {
                        SeriesDetailHero(
                            title = series.displayTitle,
                            originalTitle = series.originalTitle,
                            metadata = seriesHeroMetadata(series, null, displaySeasons.size),
                            backdropUrl = seriesBackdropUrl(session.activeServer?.baseUrl, series.id),
                            posterUrl = seriesPosterUrl(session.activeServer?.baseUrl, series.id),
                            primaryActionLabel = state.continueEpisode?.let { state.continueActionLabel }.orEmpty(),
                            onPrimaryAction = { state.continueEpisode?.let { onPlayEpisode(it.id) } },
                            onBack = onBack,
                            logoUrl = seriesLogoUrl(session.activeServer?.baseUrl, series.id),
                        )
                    }
                    if (series.overview.isNotBlank() || series.genreList.isNotEmpty() || bundle.persons.any { it.role.equals("director", ignoreCase = true) }) {
                        item {
                            SeriesOverviewSynopsis(
                                overview = series.overview.ifBlank { state.continueEpisode?.overview.orEmpty() },
                                episode = state.continueEpisode,
                                genres = series.genreList,
                                directors = bundle.persons
                                    .filter { it.role.equals("director", ignoreCase = true) }
                                    .map { it.person.name }
                                    .filter(String::isNotBlank)
                                    .distinct(),
                                onGenreClick = onGenreClick,
                            )
                        }
                    }
                    if (state.continueEpisode != null) {
                        item {
                            DetailSection("继续观看") {
                                ContinueEpisodeCard(
                                    episode = state.continueEpisode!!,
                                    baseUrl = session.activeServer?.baseUrl,
                                    progress = state.historyByMediaId[state.continueEpisode!!.id]?.normalizedProgress ?: 0f,
                                    onOpen = { onEpisodeClick(state.continueEpisode!!.id) },
                                )
                            }
                        }
                    }
                    item {
                        DetailSection(
                            title = "季",
                            subtitle = if (displaySeasons.isEmpty()) "服务器未返回季信息" else "选择要浏览的季",
                        ) {
                            if (displaySeasons.isEmpty()) {
                                HillsState("暂无季信息", "当前服务器没有返回可浏览的季。")
                            } else {
                                SeasonEntryRail(
                                    seasons = displaySeasons,
                                    baseUrl = session.activeServer?.baseUrl,
                                    onSeasonClick = onSeasonClick,
                                )
                            }
                        }
                    }
                    if (bundle.persons.isNotEmpty()) {
                        item {
                            DetailCastShelf(
                                persons = bundle.persons,
                                baseUrl = session.activeServer?.baseUrl,
                                onPersonClick = onPersonClick,
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
internal fun SeriesDetailHero(
    title: String,
    originalTitle: String,
    metadata: String,
    backdropUrl: String?,
    posterUrl: String?,
    logoUrl: String?,
    primaryActionLabel: String,
    onPrimaryAction: () -> Unit,
    onBack: () -> Unit,
) {
    var backdropFailed by remember(title, backdropUrl) { mutableStateOf(backdropUrl.isNullOrBlank()) }
    val artwork = if (backdropFailed) posterUrl else backdropUrl
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .height(590.dp)
            .background(Color(0xFF3A281E)),
    ) {
        artwork?.let { image ->
            AsyncImage(
                model = image,
                contentDescription = null,
                contentScale = ContentScale.Crop,
                onError = { backdropFailed = true },
                modifier = Modifier
                    .fillMaxSize()
                    .then(if (backdropFailed) Modifier else Modifier),
            )
        }
        Box(
            Modifier
                .fillMaxSize()
                .background(
                    Brush.verticalGradient(
                        0f to Color.Black.copy(alpha = 0.10f),
                        0.46f to Color.Black.copy(alpha = 0.22f),
                        0.72f to Color(0xFF3A281E).copy(alpha = 0.80f),
                        1f to Color(0xFF3A281E),
                    ),
                ),
        )
        Surface(
            onClick = onBack,
            modifier = Modifier
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(start = 12.dp, top = 8.dp)
                .size(48.dp),
            shape = androidx.compose.foundation.shape.CircleShape,
            color = Color.Black.copy(alpha = 0.42f),
            contentColor = Color.White,
        ) {
            Icon(
                Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "返回",
                modifier = Modifier.padding(12.dp),
            )
        }
        Column(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .fillMaxWidth()
                .padding(horizontal = 20.dp, vertical = 24.dp),
        ) {
            DetailTitleArtwork(
                logoUrl = logoUrl,
                fallbackTitle = title,
                maxWidth = 360.dp,
                maxHeight = 112.dp,
                fallbackStyle = MaterialTheme.typography.displaySmall,
            )
            if (originalTitle.isNotBlank() && originalTitle != title) {
                Spacer(Modifier.height(6.dp))
                Text(
                    text = originalTitle,
                    color = Color.White.copy(alpha = 0.82f),
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Spacer(Modifier.height(10.dp))
            Text(
                text = metadata,
                color = Color.White,
                style = MaterialTheme.typography.titleMedium,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            Spacer(Modifier.height(14.dp))
            if (primaryActionLabel.isNotBlank()) {
                HillsPrimaryAction(
                    label = primaryActionLabel,
                    icon = Icons.Default.PlayArrow,
                    onClick = onPrimaryAction,
                    modifier = Modifier.fillMaxWidth(),
                    containerColor = Color(0xFFF5B36B),
                    contentColor = Color(0xFF4B2E1E),
                )
            }
        }
    }
}

@Composable
private fun SeriesOverviewSynopsis(
    overview: String,
    episode: MediaDetail?,
    genres: List<String>,
    directors: List<String>,
    onGenreClick: (String) -> Unit,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp, vertical = 18.dp),
    ) {
        if (episode != null) {
            Text(
                text = episodeDisplayName(episode.seasonNumber, episode.episodeNumber),
                color = Color(0xFFF3E6DC),
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
            )
            Spacer(Modifier.height(6.dp))
        }
        if (overview.isNotBlank()) {
            Text(
                text = overview,
                color = Color(0xFFF3E6DC),
                style = MaterialTheme.typography.bodyLarge,
                lineHeight = 24.sp,
                maxLines = 5,
                overflow = TextOverflow.Ellipsis,
            )
        }
        if (directors.isNotEmpty()) {
            Spacer(Modifier.height(12.dp))
            Text(
                text = "导演：${directors.joinToString("、")}",
                color = Color(0xFFF3E6DC),
                style = MaterialTheme.typography.bodyLarge,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
        }
        if (genres.isNotEmpty()) {
            Spacer(Modifier.height(12.dp))
            LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                items(genres, key = { it }) { genre ->
                    SuggestionChip(
                        onClick = { onGenreClick(genre) },
                        label = { Text(genre) },
                    )
                }
            }
        }
    }
}

@Composable
private fun ContinueEpisodeCard(
    episode: MediaDetail,
    baseUrl: String?,
    progress: Float,
    onOpen: () -> Unit,
) {
    var imageFailed by remember(episode.id) { mutableStateOf(false) }
    val imageUrl = if (imageFailed) mediaPosterUrl(baseUrl, episode.id) else mediaBackdropUrl(baseUrl, episode.id)
    val episodeLabel = episodeDisplayName(episode.seasonNumber, episode.episodeNumber)
    HillsPressable(onClick = onOpen, modifier = Modifier.fillMaxWidth()) {
        Column {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(12.dp),
                verticalAlignment = Alignment.CenterVertically,
            ) {
                Box(
                    modifier = Modifier
                        .width(168.dp)
                        .aspectRatio(16f / 9f)
                        .clip(RoundedCornerShape(10.dp))
                        .background(MaterialTheme.colorScheme.surfaceVariant),
                ) {
                    AsyncImage(
                        model = imageUrl,
                        contentDescription = episode.userEpisodeTitle,
                        contentScale = ContentScale.Crop,
                        onError = { imageFailed = true },
                        modifier = Modifier.fillMaxSize(),
                    )
                    if (progress > 0f) {
                        LinearProgressIndicator(
                            progress = { progress },
                            modifier = Modifier.align(Alignment.BottomStart).fillMaxWidth(),
                            color = MaterialTheme.colorScheme.primary,
                            trackColor = Color.Transparent,
                        )
                    }
                }
                Text(
                    episodeLabel,
                    style = MaterialTheme.typography.titleMedium,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.weight(1f),
                )
            }
            episode.userEpisodeTitle
                .takeIf { it.isNotBlank() && it != episodeLabel }
                ?.let { title ->
                    Text(
                        title,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        modifier = Modifier.padding(top = 8.dp),
                    )
                }
        }
    }
}

@Composable
private fun SeasonEntryRail(
    seasons: List<SeasonInfo>,
    baseUrl: String?,
    onSeasonClick: (Int) -> Unit,
) {
    LazyRow(
        contentPadding = PaddingValues(end = 20.dp),
        horizontalArrangement = Arrangement.spacedBy(12.dp),
    ) {
        items(seasons, key = SeasonInfo::seasonNumber) { season ->
            val preview = season.episodes.firstOrNull()
            var imageFailed by remember(season.seasonNumber, preview?.id) { mutableStateOf(false) }
            val imageUrl = if (imageFailed) {
                preview?.let { mediaBackdropUrl(baseUrl, it.id) }
            } else {
                preview?.let { mediaPosterUrl(baseUrl, it.id) }
            }
            HillsPressable(
                onClick = { onSeasonClick(season.seasonNumber) },
                modifier = Modifier.width(112.dp),
            ) {
                Column {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .aspectRatio(2f / 3f)
                            .clip(RoundedCornerShape(10.dp))
                            .background(MaterialTheme.colorScheme.surfaceVariant),
                    ) {
                        if (imageUrl != null) {
                            AsyncImage(
                                model = imageUrl,
                                contentDescription = season.label,
                                contentScale = ContentScale.Crop,
                                onError = { imageFailed = true },
                                modifier = Modifier.fillMaxSize(),
                            )
                        }
                        val episodeCount = season.episodeCount.takeIf { it > 0 } ?: season.episodes.size
                        if (episodeCount > 0) {
                            Box(
                                modifier = Modifier
                                    .align(Alignment.TopEnd)
                                    .padding(8.dp)
                                    .size(36.dp)
                                    .clip(androidx.compose.foundation.shape.CircleShape)
                                    .background(Color(0xCC25201D)),
                                contentAlignment = Alignment.Center,
                            ) {
                                Text(
                                    episodeCount.toString(),
                                    color = Color.White,
                                    style = MaterialTheme.typography.labelLarge,
                                    fontWeight = FontWeight.SemiBold,
                                )
                            }
                        }
                    }
                    Spacer(Modifier.height(8.dp))
                    Text(season.label, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
        }
    }
}

internal fun initialSeasonNumber(seasons: List<SeasonInfo>): Int? =
    seasons.firstOrNull { it.seasonNumber > 0 && it.episodes.isNotEmpty() }?.seasonNumber
        ?: seasons.firstOrNull { it.seasonNumber > 0 }?.seasonNumber
        ?: seasons.firstOrNull { it.episodes.isNotEmpty() }?.seasonNumber
        ?: seasons.firstOrNull()?.seasonNumber

internal fun seriesPosterUrl(baseUrl: String?, seriesId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/series/$seriesId/poster" }

internal fun seriesBackdropUrl(baseUrl: String?, seriesId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/series/$seriesId/backdrop" }

internal fun seriesLogoUrl(baseUrl: String?, seriesId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/series/$seriesId/logo" }

internal fun mediaPosterUrl(baseUrl: String?, mediaId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/media/$mediaId/poster" }

internal fun mediaBackdropUrl(baseUrl: String?, mediaId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/media/$mediaId/backdrop" }
