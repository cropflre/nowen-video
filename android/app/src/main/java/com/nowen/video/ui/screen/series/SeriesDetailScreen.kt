package com.nowen.video.ui.screen.series

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.layout.offset
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
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.Season
import com.nowen.video.data.model.Series
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
fun SeriesDetailScreen(
    seriesId: String,
    onEpisodeClick: (String) -> Unit,
    onSearchClick: (String) -> Unit = {},
    onBack: () -> Unit,
    viewModel: SeriesDetailViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    LaunchedEffect(seriesId) { viewModel.loadSeries(seriesId) }

    val colorScheme = MaterialTheme.colorScheme
    Box(modifier = Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    title = { Text(uiState.series?.title ?: "剧集详情", color = colorScheme.primary, style = MaterialTheme.typography.titleLarge.copy(letterSpacing = 1.sp)) },
                    navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary) } },
colors = TopAppBarDefaults.topAppBarColors(containerColor = colorScheme.surface.copy(alpha = 0.95f))
                )
            }
        ) { padding ->
            if (uiState.loading) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) {
                    CircularProgressIndicator(color = colorScheme.primary, trackColor = colorScheme.surfaceContainerHigh)
                }
            } else {
                val series = uiState.series ?: return@Scaffold
                LazyColumn(Modifier.fillMaxSize().padding(padding), contentPadding = PaddingValues(bottom = 16.dp)) {
                    // 剧集海报和信息
                    item {
                        Row(Modifier.fillMaxWidth().padding(16.dp), Arrangement.spacedBy(16.dp)) {
                            val posterUrl = "${uiState.serverUrl}/api/series/${series.id}/poster?token=${uiState.token}"
                            Box(
                                Modifier.width(120.dp).height(180.dp).clip(RoundedCornerShape(12.dp))
                                    .border(1.dp, colorScheme.primary.copy(alpha = 0.2f), RoundedCornerShape(12.dp))
                            ) {
                                AsyncImage(posterUrl, series.title, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                            }
                            Column(Modifier.weight(1f)) {
                                Text(series.title, style = MaterialTheme.typography.titleLarge.copy(fontWeight = FontWeight.Bold), color = colorScheme.onSurface)
                                if (series.year > 0) Text("${series.year}", style = MaterialTheme.typography.bodyMedium, color = colorScheme.outline)
                                if (series.rating > 0) {
                                    Row(verticalAlignment = Alignment.CenterVertically, modifier = Modifier.padding(top = 4.dp)) {
                                        Icon(Icons.Default.Star, null, Modifier.size(16.dp), tint = AmberGold)
                                        Text(String.format(" %.1f", series.rating), style = MaterialTheme.typography.bodyMedium.copy(fontWeight = FontWeight.Bold), color = AmberGold)
                                    }
                                }
                                Text("${series.seasonCount} 季 · ${series.episodeCount} 集", style = MaterialTheme.typography.bodySmall, color = colorScheme.primary.copy(alpha = 0.8f), modifier = Modifier.padding(top = 4.dp))
                            }
                        }
                    }
                    // 类型标签
                    if (series.genres.isNotBlank()) {
                        item {
                            FlowRow(Modifier.padding(horizontal = 16.dp, vertical = 8.dp), Arrangement.spacedBy(8.dp), Arrangement.spacedBy(8.dp)) {
                                series.genres.split(",").forEach { g ->
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
                    }
                    // 简介
                    if (series.overview.isNotBlank()) {
                        item {
                            Text(series.overview, style = MaterialTheme.typography.bodyMedium, color = colorScheme.onSurfaceVariant, modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp), maxLines = 4, overflow = TextOverflow.Ellipsis, lineHeight = 22.sp)
                        }
                    }
                    // 季选择器 — 赛博朋克风格
                    if (uiState.seasons.isNotEmpty()) {
                        item {
                            ScrollableTabRow(
                                selectedTabIndex = uiState.selectedSeasonIndex,
                                modifier = Modifier.padding(top = 8.dp),
                                containerColor = Color.Transparent,
                                contentColor = colorScheme.primary,
                                edgePadding = 16.dp,
                                indicator = @Composable { tabPositions ->
                                    if (uiState.selectedSeasonIndex < tabPositions.size) {
                                        val currentTabPosition = tabPositions[uiState.selectedSeasonIndex]
                                        TabRowDefaults.SecondaryIndicator(
                                            Modifier
                                                .fillMaxWidth()
                                                .wrapContentSize(Alignment.BottomStart)
                                                .offset(x = currentTabPosition.left)
                                                .width(currentTabPosition.width),
                                            color = colorScheme.primary
                                        )
                                    }
                                }
                            ) {
                                uiState.seasons.forEachIndexed { index, season ->
                                    Tab(
                                        selected = index == uiState.selectedSeasonIndex,
                                        onClick = { viewModel.selectSeason(index) },
                                        text = {
                                            Text(
                                                "第 ${season.seasonNum} 季",
                                                color = if (index == uiState.selectedSeasonIndex) colorScheme.primary else colorScheme.outline,
                                                fontWeight = if (index == uiState.selectedSeasonIndex) FontWeight.Bold else FontWeight.Normal
                                            )
                                        }
                                    )
                                }
                            }
                        }
                    }
                    item { Spacer(Modifier.height(8.dp)) }
                    // 剧集列表
                    items(uiState.episodes) { episode ->
                        CyberEpisodeItem(episode = episode, onClick = { onEpisodeClick(episode.id) })
                    }
                }
            }
        }
    }
}

@Composable
private fun CyberEpisodeItem(episode: Media, onClick: () -> Unit) {
    val colorScheme = MaterialTheme.colorScheme
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 16.dp, vertical = 3.dp)
            .clip(RoundedCornerShape(12.dp))
            .background(colorScheme.surfaceContainerHigh.copy(alpha = 0.5f))
            .border(1.dp, colorScheme.primary.copy(alpha = 0.08f), RoundedCornerShape(12.dp))
            .clickable(onClick = onClick)
    ) {
        Row(
            Modifier.padding(14.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // 播放图标
            Icon(Icons.Default.PlayCircle, null, Modifier.size(28.dp), tint = colorScheme.primary)
            Column(Modifier.weight(1f)) {
                Text(
                    text = "第 ${episode.episodeNum} 集" + if (episode.episodeTitle.isNotBlank()) " · ${episode.episodeTitle}" else "",
                    style = MaterialTheme.typography.bodyMedium.copy(fontWeight = FontWeight.Medium),
                    color = colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (episode.duration > 0) {
                    Text("${(episode.duration / 60).toInt()} 分钟", style = MaterialTheme.typography.labelSmall, color = colorScheme.outline)
                }
            }
            Icon(Icons.Default.ChevronRight, null, Modifier.size(18.dp), tint = colorScheme.primary.copy(alpha = 0.4f))
        }
    }
}

// ==================== ViewModel ====================

data class SeriesDetailUiState(val loading: Boolean = true, val series: Series? = null, val seasons: List<Season> = emptyList(), val episodes: List<Media> = emptyList(), val selectedSeasonIndex: Int = 0, val serverUrl: String = "", val token: String = "", val error: String? = null)

@HiltViewModel
class SeriesDetailViewModel @Inject constructor(private val mediaRepository: MediaRepository, private val tokenManager: TokenManager) : ViewModel() {
    private val _uiState = MutableStateFlow(SeriesDetailUiState()); val uiState = _uiState.asStateFlow()
    private var seriesId: String = ""
    fun loadSeries(id: String) { seriesId = id; viewModelScope.launch {
        _uiState.value = _uiState.value.copy(loading = true)
        val serverUrl = tokenManager.getServerUrl() ?: ""; val token = tokenManager.getToken() ?: ""
        _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)
        mediaRepository.getSeriesDetail(id).onSuccess { _uiState.value = _uiState.value.copy(series = it) }
        mediaRepository.getSeasons(id).onSuccess { seasons ->
            _uiState.value = _uiState.value.copy(loading = false, seasons = seasons)
            if (seasons.isNotEmpty()) loadEpisodes(seasons.first().seasonNum)
        }.onFailure { _uiState.value = _uiState.value.copy(loading = false) }
    } }
    fun selectSeason(index: Int) { val season = _uiState.value.seasons.getOrNull(index) ?: return; _uiState.value = _uiState.value.copy(selectedSeasonIndex = index); loadEpisodes(season.seasonNum) }
    private fun loadEpisodes(seasonNum: Int) { viewModelScope.launch { mediaRepository.getSeasonEpisodes(seriesId, seasonNum).onSuccess { _uiState.value = _uiState.value.copy(episodes = it) } } }
}
