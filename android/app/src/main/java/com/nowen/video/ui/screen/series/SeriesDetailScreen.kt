package com.nowen.video.ui.screen.series

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
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
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.Season
import com.nowen.video.data.model.Series
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 剧集详情页 — 展示剧集信息和季/集列表
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SeriesDetailScreen(
    seriesId: String,
    onEpisodeClick: (String) -> Unit,
    onBack: () -> Unit,
    viewModel: SeriesDetailViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()

    LaunchedEffect(seriesId) {
        viewModel.loadSeries(seriesId)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(uiState.series?.title ?: "剧集详情") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { padding ->
        if (uiState.loading) {
            Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator()
            }
        } else {
            val series = uiState.series ?: return@Scaffold

            LazyColumn(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentPadding = PaddingValues(bottom = 16.dp)
            ) {
                // 剧集海报和信息
                item {
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(16.dp),
                        horizontalArrangement = Arrangement.spacedBy(16.dp)
                    ) {
                        // 海报
                        val posterUrl = "${uiState.serverUrl}/api/series/${series.id}/poster?token=${uiState.token}"
                        AsyncImage(
                            model = posterUrl,
                            contentDescription = series.title,
                            modifier = Modifier
                                .width(120.dp)
                                .height(180.dp)
                                .clip(RoundedCornerShape(12.dp)),
                            contentScale = ContentScale.Crop
                        )

                        // 信息
                        Column(modifier = Modifier.weight(1f)) {
                            Text(
                                text = series.title,
                                style = MaterialTheme.typography.titleLarge
                            )
                            if (series.year > 0) {
                                Text(
                                    text = "${series.year}",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            }
                            if (series.rating > 0) {
                                Row(
                                    verticalAlignment = Alignment.CenterVertically,
                                    modifier = Modifier.padding(top = 4.dp)
                                ) {
                                    Icon(
                                        Icons.Default.Star,
                                        contentDescription = null,
                                        modifier = Modifier.size(16.dp),
                                        tint = MaterialTheme.colorScheme.tertiary
                                    )
                                    Text(
                                        text = String.format(" %.1f", series.rating),
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = MaterialTheme.colorScheme.tertiary
                                    )
                                }
                            }
                            Text(
                                text = "${series.seasonCount} 季 · ${series.episodeCount} 集",
                                style = MaterialTheme.typography.bodySmall,
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                                modifier = Modifier.padding(top = 4.dp)
                            )
                        }
                    }
                }

                // 简介
                if (series.overview.isNotBlank()) {
                    item {
                        Text(
                            text = series.overview,
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp),
                            maxLines = 4,
                            overflow = TextOverflow.Ellipsis
                        )
                    }
                }

                // 季选择器
                if (uiState.seasons.isNotEmpty()) {
                    item {
                        ScrollableTabRow(
                            selectedTabIndex = uiState.selectedSeasonIndex,
                            modifier = Modifier.padding(top = 8.dp)
                        ) {
                            uiState.seasons.forEachIndexed { index, season ->
                                Tab(
                                    selected = index == uiState.selectedSeasonIndex,
                                    onClick = { viewModel.selectSeason(index) },
                                    text = { Text("第 ${season.seasonNum} 季") }
                                )
                            }
                        }
                    }
                }

                // 剧集列表
                item { Spacer(modifier = Modifier.height(8.dp)) }

                items(uiState.episodes) { episode ->
                    EpisodeItem(
                        episode = episode,
                        onClick = { onEpisodeClick(episode.id) }
                    )
                }
            }
        }
    }
}

@Composable
private fun EpisodeItem(
    episode: Media,
    onClick: () -> Unit
) {
    ListItem(
        headlineContent = {
            Text(
                text = "第 ${episode.episodeNum} 集" +
                    if (episode.episodeTitle.isNotBlank()) " · ${episode.episodeTitle}" else "",
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        },
        supportingContent = {
            if (episode.duration > 0) {
                Text("${(episode.duration / 60).toInt()} 分钟")
            }
        },
        leadingContent = {
            Icon(Icons.Default.PlayCircle, contentDescription = null)
        },
        modifier = Modifier.clickable(onClick = onClick)
    )
}

// ==================== ViewModel ====================

data class SeriesDetailUiState(
    val loading: Boolean = true,
    val series: Series? = null,
    val seasons: List<Season> = emptyList(),
    val episodes: List<Media> = emptyList(),
    val selectedSeasonIndex: Int = 0,
    val serverUrl: String = "",
    val token: String = "",
    val error: String? = null
)

@HiltViewModel
class SeriesDetailViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(SeriesDetailUiState())
    val uiState = _uiState.asStateFlow()

    private var seriesId: String = ""

    fun loadSeries(id: String) {
        seriesId = id
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true)

            val serverUrl = tokenManager.getServerUrl() ?: ""
            val token = tokenManager.getToken() ?: ""
            _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)

            // 加载剧集详情
            mediaRepository.getSeriesDetail(id).onSuccess { series ->
                _uiState.value = _uiState.value.copy(series = series)
            }

            // 加载季列表
            mediaRepository.getSeasons(id).onSuccess { seasons ->
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    seasons = seasons
                )
                // 自动加载第一季的剧集
                if (seasons.isNotEmpty()) {
                    loadEpisodes(seasons.first().seasonNum)
                }
            }.onFailure {
                _uiState.value = _uiState.value.copy(loading = false)
            }
        }
    }

    fun selectSeason(index: Int) {
        val season = _uiState.value.seasons.getOrNull(index) ?: return
        _uiState.value = _uiState.value.copy(selectedSeasonIndex = index)
        loadEpisodes(season.seasonNum)
    }

    private fun loadEpisodes(seasonNum: Int) {
        viewModelScope.launch {
            mediaRepository.getSeasonEpisodes(seriesId, seasonNum).onSuccess { episodes ->
                _uiState.value = _uiState.value.copy(episodes = episodes)
            }
        }
    }
}
