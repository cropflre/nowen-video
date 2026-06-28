package com.nowen.video.ui.screen.mobile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.ui.component.mobile.EmptyState
import com.nowen.video.ui.component.mobile.LibraryItem
import com.nowen.video.ui.component.mobile.MediaPosterCard
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.component.mobile.PageHeaderAction
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileSpacing
import com.nowen.video.ui.screen.home.HomeViewModel

/**
 * 服务器首页
 * 展示 Hero、继续观看、媒体库、最近添加
 */
@Composable
fun ServerHomeScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onLibraryClick: (String) -> Unit,
    onPlayerClick: (String) -> Unit,
    onSearchClick: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()
    val serverUrl = uiState.serverUrl

    Column(
        modifier = modifier.fillMaxSize(),
    ) {
        // 页面标题
        MobilePageHeader(
            title = "Nowen Video",
            onBack = onBack,
            actions = listOf(
                PageHeaderAction(
                    icon = Icons.Default.Search,
                    contentDescription = "搜索",
                    onClick = onSearchClick,
                ),
            ),
        )

        when {
            uiState.loading -> {
                Box(
                    modifier = Modifier.fillMaxSize(),
                    contentAlignment = Alignment.Center,
                ) {
                    CircularProgressIndicator()
                }
            }
            uiState.error != null -> {
                EmptyState(
                    icon = Icons.Default.PlayArrow,
                    title = "加载失败",
                    subtitle = uiState.error,
                )
            }
            else -> {
                LazyColumn(
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(bottom = MobileSpacing.xl),
                ) {
                    // 继续观看
                    if (uiState.continueWatching.isNotEmpty()) {
                        item {
                            SectionTitle(
                                title = "继续观看",
                                modifier = Modifier.padding(horizontal = MobileSpacing.xl),
                            )
                        }
                        item {
                            LazyRow(
                                contentPadding = PaddingValues(horizontal = MobileSpacing.xl),
                                horizontalArrangement = Arrangement.spacedBy(MobileSpacing.md),
                            ) {
                                items(uiState.continueWatching) { history ->
                                    val media = history.media
                                    MediaPosterCard(
                                        title = media?.title ?: "未知",
                                        year = media?.year,
                                        imageUrl = if (media?.posterPath?.isNotBlank() == true) {
                                            "$serverUrl/api/media/${media.id}/poster"
                                        } else {
                                            null
                                        },
                                        progress = if (history.duration > 0) {
                                            (history.position / history.duration).toFloat().coerceIn(0f, 1f)
                                        } else {
                                            null
                                        },
                                        onClick = { onPlayerClick(history.mediaId) },
                                        modifier = Modifier.width(200.dp),
                                    )
                                }
                            }
                        }
                    }

                    // 媒体库
                    if (uiState.libraries.isNotEmpty()) {
                        item {
                            SectionTitle(
                                title = "媒体库",
                                modifier = Modifier.padding(horizontal = MobileSpacing.xl),
                            )
                        }
                        items(uiState.libraries) { library ->
                            LibraryItem(
                                library = library,
                                onClick = { onLibraryClick(library.id) },
                                modifier = Modifier.padding(horizontal = MobileSpacing.xl),
                            )
                        }
                    }

                    // 最近添加
                    if (uiState.recentMixed.isNotEmpty()) {
                        item {
                            SectionTitle(
                                title = "最近添加",
                                modifier = Modifier.padding(horizontal = MobileSpacing.xl),
                            )
                        }
                        item {
                            LazyRow(
                                contentPadding = PaddingValues(horizontal = MobileSpacing.xl),
                                horizontalArrangement = Arrangement.spacedBy(MobileSpacing.md),
                            ) {
                                items(uiState.recentMixed) { mixedItem ->
                                    val media = mixedItem.media
                                    val series = mixedItem.series
                                    val title = media?.title ?: series?.title ?: "未知"
                                    val year = media?.year ?: series?.year
                                    val posterPath = media?.posterPath ?: series?.posterPath ?: ""
                                    val id = media?.id ?: series?.id ?: ""

                                    MediaPosterCard(
                                        title = title,
                                        year = year,
                                        imageUrl = if (posterPath.isNotBlank()) {
                                            "$serverUrl/api/media/$id/poster"
                                        } else {
                                            null
                                        },
                                        onClick = {
                                            if (mixedItem.type == "series") {
                                                onSeriesClick(id)
                                            } else {
                                                onMediaClick(id)
                                            }
                                        },
                                        modifier = Modifier.width(140.dp),
                                    )
                                }
                            }
                        }
                    }

                    // 空状态
                    if (uiState.continueWatching.isEmpty() &&
                        uiState.libraries.isEmpty() &&
                        uiState.recentMixed.isEmpty()
                    ) {
                        item {
                            EmptyState(
                                icon = Icons.Default.PlayArrow,
                                title = "暂无内容",
                                subtitle = "请先添加媒体库和媒体文件",
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SectionTitle(
    title: String,
    modifier: Modifier = Modifier,
) {
    Text(
        text = title,
        color = MobileColors.Text,
        fontSize = MobileFontSize.xl,
        fontWeight = FontWeight.SemiBold,
        modifier = modifier.padding(vertical = MobileSpacing.md),
    )
}
