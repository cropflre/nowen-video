@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.SuggestionChip
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
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
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SeriesRepository
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.SeasonInfo
import com.nowen.video.v2.core.model.SeriesBundle
import com.nowen.video.v2.core.model.WatchHistoryRecord
import com.nowen.video.v2.core.model.seriesEpisodeLabel
import com.nowen.video.v2.core.model.seriesEpisodeSubtitle
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
    val selectedSeasonNumber: Int? = null,
    val history: List<WatchHistoryRecord> = emptyList(),
    val error: String? = null,
) {
    val selectedSeason: SeasonInfo?
        get() = bundle?.seasons?.firstOrNull { it.seasonNumber == selectedSeasonNumber }
            ?: bundle?.seasons?.firstOrNull()

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
                    "继续播放 ${episode.seasonEpisodeCode}"
                watchedCount >= orderedEpisodes.size && orderedEpisodes.isNotEmpty() ->
                    "重新播放 ${episode.seasonEpisodeCode}"
                else -> "播放 ${episode.seasonEpisodeCode}"
            }
        }
}

private val WatchHistoryRecord.resolvedMediaId: String
    get() = mediaId.ifBlank { media.resolvedId }

private val MediaDetail.seasonEpisodeCode: String
    get() = if (seasonNumber == 0) {
        "SP${episodeNumber.toString().padStart(2, '0')}"
    } else {
        "S${seasonNumber.toString().padStart(2, '0')}E${episodeNumber.toString().padStart(2, '0')}"
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
                    val historyDeferred = async {
                        socialRepository.history(page = 1, size = 100).getOrNull()?.data.orEmpty()
                    }
                    val bundle = bundleDeferred.await()
                    val history = historyDeferred.await()
                    val provisional = SeriesDetailUiState(
                        loading = false,
                        bundle = bundle,
                        history = history,
                    )
                    provisional.copy(
                        selectedSeasonNumber = provisional.continueEpisode?.seasonNumber
                            ?: initialSeasonNumber(bundle.seasons),
                    )
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

    fun selectSeason(seasonNumber: Int) {
        if (_state.value.bundle?.seasons?.none { it.seasonNumber == seasonNumber } != false) return
        _state.update { it.copy(selectedSeasonNumber = seasonNumber) }
    }
}

private enum class SeriesDetailTab(val label: String) {
    Episodes("剧集"),
    Overview("简介"),
    Cast("演职人员"),
}

@Composable
fun SeriesDetailScreen(
    seriesId: String,
    onBack: () -> Unit,
    onEpisodeClick: (String) -> Unit,
    onPlayEpisode: (String) -> Unit,
    onPersonClick: (String) -> Unit,
    viewModel: SeriesDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var selectedTab by rememberSaveable(seriesId) { mutableIntStateOf(0) }
    LaunchedEffect(seriesId) { viewModel.load(seriesId) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        when {
            state.loading -> CircularProgressIndicator(Modifier.align(Alignment.Center))
            state.error != null -> MessagePanel(
                title = "无法打开剧集",
                message = state.error!!,
                actionLabel = "重试",
                onAction = { viewModel.load(seriesId) },
                modifier = Modifier
                    .align(Alignment.Center)
                    .padding(20.dp),
            )
            state.bundle != null -> {
                val bundle = state.bundle!!
                val series = bundle.series
                val selectedSeason = state.selectedSeason
                val tabs = SeriesDetailTab.entries
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(bottom = 36.dp),
                ) {
                    item {
                        MobileDetailHero(
                            title = series.displayTitle,
                            originalTitle = series.originalTitle,
                            metadata = series.metadataLabel,
                            overview = series.overview,
                            backdropUrl = seriesBackdropUrl(session.activeServer?.baseUrl, series.id),
                            posterUrl = seriesPosterUrl(session.activeServer?.baseUrl, series.id),
                            primaryActionLabel = state.continueActionLabel,
                            onPrimaryAction = {
                                state.continueEpisode?.let { onPlayEpisode(it.id) }
                            },
                            onBack = onBack,
                        )
                    }

                    item {
                        DetailTabStrip(
                            labels = tabs.map(SeriesDetailTab::label),
                            selectedIndex = selectedTab,
                            onSelected = { selectedTab = it },
                        )
                    }

                    when (tabs[selectedTab]) {
                        SeriesDetailTab.Episodes -> {
                            item {
                                DetailSection(
                                    title = "观看进度",
                                    subtitle = "回到上次播放位置，或继续下一集",
                                ) {
                                    DetailInfoPanel(
                                        listOfNotNull(
                                            "观看进度" to "${state.watchedCount} / ${state.orderedEpisodes.size} 集",
                                            state.inProgressCount.takeIf { it > 0 }?.let { "进行中" to "$it 集" },
                                            state.continueEpisode?.let { "当前" to it.seasonEpisodeCode },
                                        ),
                                    )
                                }
                            }
                            item {
                                DetailSection("选择季") {
                                    if (bundle.seasons.isEmpty()) {
                                        MessagePanel("暂无单集", "服务器中还没有可播放的单集。")
                                    } else {
                                        LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                            items(bundle.seasons, key = SeasonInfo::seasonNumber) { season ->
                                                FilterChip(
                                                    selected = season.seasonNumber == selectedSeason?.seasonNumber,
                                                    onClick = { viewModel.selectSeason(season.seasonNumber) },
                                                    label = { Text("${season.label} · ${season.episodes.size}") },
                                                )
                                            }
                                        }
                                    }
                                }
                            }

                            if (selectedSeason != null && selectedSeason.episodes.isEmpty()) {
                                item {
                                    MessagePanel(
                                        title = "本季暂无单集",
                                        message = "扫描或整理完成后，单集会显示在这里。",
                                        modifier = Modifier.padding(horizontal = 20.dp),
                                    )
                                }
                            } else {
                                items(selectedSeason?.episodes.orEmpty(), key = MediaDetail::id) { episode ->
                                    val history = state.historyByMediaId[episode.id]
                                    EpisodeWorkspaceCard(
                                        episode = episode,
                                        imageUrl = mediaBackdropUrl(session.activeServer?.baseUrl, episode.id),
                                        history = history,
                                        highlighted = episode.id == state.continueEpisode?.id,
                                        onOpen = { onEpisodeClick(episode.id) },
                                        onPlay = { onPlayEpisode(episode.id) },
                                        modifier = Modifier.padding(horizontal = 20.dp, vertical = 6.dp),
                                    )
                                }
                            }
                        }

                        SeriesDetailTab.Overview -> item {
                            DetailSection("剧情简介") {
                                Text(
                                    series.overview.ifBlank { "暂无简介" },
                                    style = MaterialTheme.typography.bodyLarge,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                                if (series.genreList.isNotEmpty()) {
                                    Spacer(Modifier.height(14.dp))
                                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                        items(series.genreList, key = { it }) { genre ->
                                            SuggestionChip(onClick = {}, label = { Text(genre) })
                                        }
                                    }
                                }
                                Spacer(Modifier.height(16.dp))
                                DetailInfoPanel(
                                    listOfNotNull(
                                        series.seasonCount.takeIf { it > 0 }?.let { "季数" to "$it 季" },
                                        series.episodeCount.takeIf { it > 0 }?.let { "集数" to "$it 集" },
                                        series.year.takeIf { it > 0 }?.let { "年份" to it.toString() },
                                        series.country.takeIf(String::isNotBlank)?.let { "地区" to it },
                                        series.language.takeIf(String::isNotBlank)?.let { "语言" to it },
                                        series.studio.takeIf(String::isNotBlank)?.let { "制作" to it },
                                    ),
                                )
                            }
                        }

                        SeriesDetailTab.Cast -> item {
                            DetailSection(
                                title = "演职人员",
                                subtitle = if (bundle.persons.isEmpty()) "当前剧集暂无演职人员信息" else "点击人物查看相关作品",
                            ) {
                                if (bundle.persons.isEmpty()) {
                                    MessagePanel("暂无演职人员", "服务器暂未返回该剧集的演职人员信息。")
                                } else {
                                    LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                                        items(bundle.persons.take(24), key = { it.id }) { credit ->
                                            DetailCreditCard(
                                                name = credit.person.name,
                                                role = credit.roleLabel,
                                                imageUrl = personProfileUrl(
                                                    session.activeServer?.baseUrl,
                                                    credit.person.id,
                                                ),
                                                onClick = { onPersonClick(credit.person.id) },
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun EpisodeWorkspaceCard(
    episode: MediaDetail,
    imageUrl: String?,
    history: WatchHistoryRecord?,
    highlighted: Boolean,
    onOpen: () -> Unit,
    onPlay: () -> Unit,
    modifier: Modifier = Modifier,
) {
    ElevatedPanel(
        modifier
            .fillMaxWidth()
            .clickable(onClick = onOpen),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            AsyncImage(
                model = imageUrl,
                contentDescription = episode.seriesEpisodeLabel,
                contentScale = ContentScale.Crop,
                modifier = Modifier
                    .width(118.dp)
                    .aspectRatio(16f / 9f)
                    .clip(MaterialTheme.shapes.medium)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
            )
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        episode.seasonEpisodeCode,
                        style = MaterialTheme.typography.titleMedium,
                        color = if (highlighted) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface,
                    )
                    if (history?.completed == true) {
                        Spacer(Modifier.width(6.dp))
                        Icon(
                            Icons.Default.CheckCircle,
                            contentDescription = "已看完",
                            modifier = Modifier.size(16.dp),
                            tint = MaterialTheme.colorScheme.primary,
                        )
                    }
                }
                Text(
                    episode.episodeTitle.ifBlank { episode.seriesEpisodeLabel },
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    style = MaterialTheme.typography.bodyMedium,
                )
                if (episode.seriesEpisodeSubtitle.isNotBlank()) {
                    Spacer(Modifier.height(4.dp))
                    Text(
                        episode.seriesEpisodeSubtitle,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                        style = MaterialTheme.typography.bodyMedium,
                    )
                }
                history?.let { record ->
                    val progress = if (record.completed) 1f else record.normalizedProgress
                    if (progress > 0f) {
                        Spacer(Modifier.height(8.dp))
                        LinearProgressIndicator(
                            progress = { progress },
                            modifier = Modifier.fillMaxWidth(),
                        )
                        Spacer(Modifier.height(4.dp))
                        Text(
                            if (record.completed) "已看完" else "已观看 ${(progress * 100).toInt()}%",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                    }
                }
            }
            IconButton(onClick = onPlay) {
                Icon(Icons.Default.PlayArrow, contentDescription = "播放${episode.seriesEpisodeLabel}")
            }
            Icon(
                Icons.Default.ChevronRight,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

internal fun initialSeasonNumber(seasons: List<SeasonInfo>): Int? =
    seasons.firstOrNull { it.seasonNumber > 0 }?.seasonNumber
        ?: seasons.firstOrNull()?.seasonNumber

internal fun seriesPosterUrl(baseUrl: String?, seriesId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/series/$seriesId/poster" }

internal fun seriesBackdropUrl(baseUrl: String?, seriesId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/series/$seriesId/backdrop" }

internal fun mediaPosterUrl(baseUrl: String?, mediaId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/media/$mediaId/poster" }

internal fun mediaBackdropUrl(baseUrl: String?, mediaId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/media/$mediaId/backdrop" }
