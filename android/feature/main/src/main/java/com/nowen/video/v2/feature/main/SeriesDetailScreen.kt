@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Star
import androidx.compose.material.icons.filled.Tv
import androidx.compose.material3.*
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SeriesRepository
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.SeasonInfo
import com.nowen.video.v2.core.model.SeriesBundle
import com.nowen.video.v2.core.model.seriesEpisodeLabel
import com.nowen.video.v2.core.model.seriesEpisodeSubtitle
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class SeriesDetailUiState(
    val loading: Boolean = true,
    val bundle: SeriesBundle? = null,
    val selectedSeasonNumber: Int? = null,
    val error: String? = null,
) {
    val selectedSeason: SeasonInfo?
        get() = bundle?.seasons?.firstOrNull { it.seasonNumber == selectedSeasonNumber }
            ?: bundle?.seasons?.firstOrNull()

    val firstEpisode: MediaDetail?
        get() = bundle?.firstEpisode
}

@HiltViewModel
class SeriesDetailViewModel @Inject constructor(
    private val repository: SeriesRepository,
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
            repository.load(id)
                .onSuccess { bundle ->
                    _state.value = SeriesDetailUiState(
                        loading = false,
                        bundle = bundle,
                        selectedSeasonNumber = initialSeasonNumber(bundle.seasons),
                    )
                }
                .onFailure { error ->
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
    LaunchedEffect(seriesId) { viewModel.load(seriesId) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Text(
                        state.bundle?.series?.displayTitle ?: "剧集详情",
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                },
            )
        },
    ) { padding ->
        Box(
            modifier = Modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background)
                .padding(padding),
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
                    LazyColumn(
                        modifier = Modifier.fillMaxSize(),
                        contentPadding = PaddingValues(bottom = 36.dp),
                        verticalArrangement = Arrangement.spacedBy(0.dp),
                    ) {
                        item {
                            Box(Modifier.fillMaxWidth().height(250.dp)) {
                                AsyncImage(
                                    model = seriesBackdropUrl(session.activeServer?.baseUrl, series.id),
                                    contentDescription = series.displayTitle,
                                    contentScale = ContentScale.Crop,
                                    modifier = Modifier.fillMaxSize(),
                                )
                                Box(
                                    Modifier
                                        .fillMaxSize()
                                        .background(
                                            Brush.verticalGradient(
                                                listOf(Color.Transparent, MaterialTheme.colorScheme.background),
                                            ),
                                        ),
                                )
                            }
                        }
                        item {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 20.dp)
                                    .offset(y = (-52).dp),
                                verticalAlignment = Alignment.Bottom,
                            ) {
                                AsyncImage(
                                    model = seriesPosterUrl(session.activeServer?.baseUrl, series.id),
                                    contentDescription = series.displayTitle,
                                    contentScale = ContentScale.Crop,
                                    modifier = Modifier
                                        .width(116.dp)
                                        .aspectRatio(2f / 3f)
                                        .clip(MaterialTheme.shapes.large)
                                        .background(MaterialTheme.colorScheme.surfaceVariant),
                                )
                                Spacer(Modifier.width(18.dp))
                                Column(
                                    modifier = Modifier
                                        .weight(1f)
                                        .padding(bottom = 10.dp),
                                ) {
                                    Text(series.displayTitle, style = MaterialTheme.typography.headlineMedium)
                                    if (series.originalTitle.isNotBlank() && series.originalTitle != series.title) {
                                        Spacer(Modifier.height(4.dp))
                                        Text(
                                            series.originalTitle,
                                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                                            maxLines = 2,
                                            overflow = TextOverflow.Ellipsis,
                                        )
                                    }
                                    if (series.metadataLabel.isNotBlank()) {
                                        Spacer(Modifier.height(8.dp))
                                        Text(series.metadataLabel, color = MaterialTheme.colorScheme.primary)
                                    }
                                }
                            }
                        }
                        item {
                            Column(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 20.dp)
                                    .offset(y = (-30).dp),
                            ) {
                                state.firstEpisode?.let { firstEpisode ->
                                    Button(
                                        onClick = { onPlayEpisode(firstEpisode.id) },
                                        modifier = Modifier.fillMaxWidth(),
                                    ) {
                                        Icon(Icons.Default.PlayArrow, contentDescription = null)
                                        Spacer(Modifier.width(8.dp))
                                        Text("从第一集播放")
                                    }
                                    Spacer(Modifier.height(18.dp))
                                }

                                if (series.genreList.isNotEmpty()) {
                                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                        items(series.genreList, key = { it }) { genre ->
                                            SuggestionChip(onClick = {}, label = { Text(genre) })
                                        }
                                    }
                                    Spacer(Modifier.height(18.dp))
                                }

                                Text("简介", style = MaterialTheme.typography.titleLarge)
                                Spacer(Modifier.height(8.dp))
                                Text(
                                    series.overview.ifBlank { "暂无简介" },
                                    style = MaterialTheme.typography.bodyLarge,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )

                                val production = listOfNotNull(
                                    series.country.takeIf(String::isNotBlank),
                                    series.language.takeIf(String::isNotBlank),
                                    series.studio.takeIf(String::isNotBlank),
                                ).joinToString(" · ")
                                if (production.isNotBlank()) {
                                    Spacer(Modifier.height(12.dp))
                                    Text(production, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                            }
                        }

                        if (bundle.persons.isNotEmpty()) {
                            item {
                                Column(Modifier.padding(horizontal = 20.dp)) {
                                    Text("演职人员", style = MaterialTheme.typography.titleLarge)
                                    Spacer(Modifier.height(12.dp))
                                    LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                        items(bundle.persons.take(20), key = { it.id }) { credit ->
                                            Column(
                                                modifier = Modifier
                                                    .width(96.dp)
                                                    .clickable { onPersonClick(credit.person.id) },
                                                horizontalAlignment = Alignment.CenterHorizontally,
                                            ) {
                                                AsyncImage(
                                                    model = personProfileUrl(
                                                        session.activeServer?.baseUrl,
                                                        credit.person.id,
                                                    ),
                                                    contentDescription = credit.person.name,
                                                    contentScale = ContentScale.Crop,
                                                    modifier = Modifier
                                                        .size(82.dp)
                                                        .clip(MaterialTheme.shapes.large)
                                                        .background(MaterialTheme.colorScheme.surfaceVariant),
                                                )
                                                Spacer(Modifier.height(8.dp))
                                                Text(
                                                    credit.person.name,
                                                    maxLines = 1,
                                                    overflow = TextOverflow.Ellipsis,
                                                    style = MaterialTheme.typography.titleSmall,
                                                )
                                                Text(
                                                    credit.roleLabel,
                                                    maxLines = 1,
                                                    overflow = TextOverflow.Ellipsis,
                                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                                    style = MaterialTheme.typography.bodySmall,
                                                )
                                            }
                                        }
                                    }
                                    Spacer(Modifier.height(24.dp))
                                }
                            }
                        }

                        item {
                            Column(Modifier.padding(horizontal = 20.dp)) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Icon(Icons.Default.Tv, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
                                    Spacer(Modifier.width(10.dp))
                                    Text("选集", style = MaterialTheme.typography.titleLarge)
                                    selectedSeason?.let {
                                        Spacer(Modifier.width(8.dp))
                                        Text(
                                            "${it.episodes.size} 集",
                                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        )
                                    }
                                }
                                Spacer(Modifier.height(12.dp))
                                if (bundle.seasons.isEmpty()) {
                                    MessagePanel("暂无单集", "服务器中还没有可播放的单集。")
                                } else {
                                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                        items(bundle.seasons, key = SeasonInfo::seasonNumber) { season ->
                                            FilterChip(
                                                selected = season.seasonNumber == selectedSeason?.seasonNumber,
                                                onClick = { viewModel.selectSeason(season.seasonNumber) },
                                                label = { Text(season.label) },
                                            )
                                        }
                                    }
                                }
                                Spacer(Modifier.height(12.dp))
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
                                EpisodeRow(
                                    episode = episode,
                                    imageUrl = mediaPosterUrl(session.activeServer?.baseUrl, episode.id),
                                    onOpen = { onEpisodeClick(episode.id) },
                                    onPlay = { onPlayEpisode(episode.id) },
                                    modifier = Modifier.padding(horizontal = 20.dp, vertical = 6.dp),
                                )
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun EpisodeRow(
    episode: MediaDetail,
    imageUrl: String?,
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
                    .width(108.dp)
                    .aspectRatio(16f / 9f)
                    .clip(MaterialTheme.shapes.medium)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
            )
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    episode.seriesEpisodeLabel,
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                if (episode.seriesEpisodeSubtitle.isNotBlank()) {
                    Spacer(Modifier.height(5.dp))
                    Text(
                        episode.seriesEpisodeSubtitle,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                }
                if (episode.rating > 0) {
                    Spacer(Modifier.height(5.dp))
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Icon(
                            Icons.Default.Star,
                            contentDescription = null,
                            modifier = Modifier.size(15.dp),
                            tint = MaterialTheme.colorScheme.primary,
                        )
                        Spacer(Modifier.width(4.dp))
                        Text("%.1f".format(episode.rating), style = MaterialTheme.typography.bodySmall)
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
