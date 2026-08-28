@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
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
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.SeasonInfo
import com.nowen.video.v2.core.model.episodeDisplayName
import com.nowen.video.v2.core.model.userEpisodeTitle
import com.nowen.video.v2.core.designsystem.HillsState

@Composable
fun SeasonEpisodesScreen(
    seriesId: String,
    seasonNumber: Int,
    onBack: () -> Unit,
    onEpisodeClick: (String) -> Unit,
    onPlayEpisode: (String) -> Unit,
    viewModel: SeriesDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    LaunchedEffect(seriesId) { viewModel.load(seriesId) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        when {
            state.loading -> CircularProgressIndicator(Modifier.align(Alignment.Center))
            state.error != null -> HillsState(
                title = "无法打开季列表",
                message = state.error!!,
                actionLabel = "返回",
                onAction = onBack,
                modifier = Modifier.align(Alignment.Center),
            )
            state.bundle != null -> {
                val bundle = state.bundle!!
                val series = bundle.series
                val season = bundle.seasons.firstOrNull { it.seasonNumber == seasonNumber }
                val episodes = season?.episodes.orEmpty()
                val firstEpisode = episodes.firstOrNull()
                val actionEpisode = state.continueEpisode?.takeIf { episode -> episodes.any { it.id == episode.id } }
                    ?: firstEpisode
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(bottom = 36.dp),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                ) {
                    item {
                        SeriesDetailHero(
                            title = series.displayTitle,
                            originalTitle = series.originalTitle,
                            metadata = "",
                            backdropUrl = seriesBackdropUrl(session.activeServer?.baseUrl, series.id),
                            posterUrl = seriesPosterUrl(session.activeServer?.baseUrl, series.id),
                            logoUrl = seriesLogoUrl(session.activeServer?.baseUrl, series.id),
                            primaryActionLabel = actionEpisode?.let {
                                "播放 ${episodeDisplayName(it.seasonNumber, it.episodeNumber)}"
                            } ?: "暂无可播放单集",
                            onPrimaryAction = { actionEpisode?.let { onPlayEpisode(it.id) } },
                            onBack = onBack,
                        )
                    }
                    if (series.overview.isNotBlank() || bundle.persons.any { it.role.equals("director", ignoreCase = true) }) {
                        item {
                            DetailSection("简介") {
                                if (series.overview.isNotBlank()) {
                                    Text(
                                        series.overview,
                                        style = MaterialTheme.typography.bodyLarge,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        maxLines = 6,
                                        overflow = TextOverflow.Ellipsis,
                                    )
                                }
                                val directors = bundle.persons
                                    .filter { it.role.equals("director", ignoreCase = true) }
                                    .map { it.person.name }
                                    .filter(String::isNotBlank)
                                    .distinct()
                                if (directors.isNotEmpty()) {
                                    Spacer(Modifier.height(14.dp))
                                    Text(
                                        "导演：${directors.joinToString("、")}",
                                        style = MaterialTheme.typography.bodyLarge,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    )
                                }
                            }
                        }
                    }
                    item {
                        DetailSection(
                            title = season?.label ?: "季列表",
                            subtitle = season?.let { "${it.episodes.size} 集" } ?: "服务器未返回这一季",
                        ) {}
                    }
                    if (season == null) {
                        item {
                            HillsState(
                                title = "暂无${seasonDisplayNameFallback(seasonNumber)}",
                                message = "当前服务器没有返回这一季的真实数据。",
                                modifier = Modifier.padding(top = 36.dp),
                            )
                        }
                    } else if (episodes.isEmpty()) {
                        item {
                            HillsState(
                                title = "本季暂无单集",
                                message = "当前服务器没有返回可播放的单集。",
                                modifier = Modifier.padding(top = 36.dp),
                            )
                        }
                    } else {
                        items(episodes, key = MediaDetail::id) { episode ->
                            SeasonEpisodeRow(
                                episode = episode,
                                baseUrl = session.activeServer?.baseUrl,
                                history = state.historyByMediaId[episode.id],
                                onOpen = { onEpisodeClick(episode.id) },
                                onPlay = { onPlayEpisode(episode.id) },
                                modifier = Modifier.padding(horizontal = 20.dp),
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SeasonEpisodeRow(
    episode: MediaDetail,
    baseUrl: String?,
    history: com.nowen.video.v2.core.model.WatchHistoryRecord?,
    onOpen: () -> Unit,
    onPlay: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val progress = history?.let { if (it.completed) 1f else it.normalizedProgress } ?: 0f
    var imageFailed by remember(episode.id) { mutableStateOf(false) }
    val imageUrl = if (imageFailed) mediaPosterUrl(baseUrl, episode.id) else mediaBackdropUrl(baseUrl, episode.id)
    Surface(
        onClick = onOpen,
        modifier = modifier.fillMaxWidth(),
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.72f),
        shape = RoundedCornerShape(10.dp),
    ) {
        Row(
            modifier = Modifier.padding(10.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Box(
                modifier = Modifier
                    .width(176.dp)
                    .aspectRatio(16f / 9f)
                    .clip(RoundedCornerShape(8.dp))
                    .background(MaterialTheme.colorScheme.surfaceVariant),
            ) {
                androidx.compose.foundation.Image(
                    painter = coil.compose.rememberAsyncImagePainter(
                        model = imageUrl,
                        onError = { imageFailed = true },
                    ),
                    contentDescription = episode.userEpisodeTitle,
                    contentScale = ContentScale.Crop,
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
            Column(Modifier.weight(1f)) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        episodeDisplayName(episode.seasonNumber, episode.episodeNumber),
                        style = MaterialTheme.typography.titleMedium,
                        fontWeight = FontWeight.SemiBold,
                        modifier = Modifier.weight(1f),
                    )
                    if (history?.completed == true) {
                        Icon(
                            Icons.Default.CheckCircle,
                            contentDescription = "已看完",
                            tint = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.size(18.dp),
                        )
                    }
                }
                Spacer(Modifier.height(3.dp))
                Text(
                    episode.userEpisodeTitle,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                if (episode.duration > 0.0) {
                    Spacer(Modifier.height(4.dp))
                    Text(
                        "${(episode.duration / 60).toInt().coerceAtLeast(1)} 分钟",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        style = MaterialTheme.typography.bodySmall,
                    )
                }
                IconButton(onClick = onPlay, modifier = Modifier.align(Alignment.End).size(40.dp)) {
                    Icon(Icons.Default.PlayArrow, contentDescription = "播放${episodeDisplayName(episode.seasonNumber, episode.episodeNumber)}")
                }
            }
        }
    }
}

private fun seasonDisplayNameFallback(seasonNumber: Int): String =
    if (seasonNumber == 0) "特别篇" else "第${seasonNumber}季"
