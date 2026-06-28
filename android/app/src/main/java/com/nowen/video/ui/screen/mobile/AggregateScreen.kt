package com.nowen.video.ui.screen.mobile

import androidx.compose.animation.AnimatedContent
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.togetherWith
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material.icons.filled.FolderOpen
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.ui.component.mobile.EmptyState
import com.nowen.video.ui.component.mobile.LibraryItem
import com.nowen.video.ui.component.mobile.MediaPosterCard
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.component.mobile.PageHeaderAction
import com.nowen.video.ui.component.mobile.SegmentedTabs
import com.nowen.video.ui.theme.MobileSpacing
import com.nowen.video.ui.screen.home.HomeViewModel

/**
 * 聚合视界内部 Tab
 */
enum class AggregateTab {
    ContinueWatching,
    Favorites,
    Libraries,
}

/**
 * 聚合视界页面
 * 展示继续观看、收藏、媒体库
 */
@Composable
fun AggregateScreen(
    onMediaClick: (String) -> Unit,
    onPlayerClick: (String) -> Unit,
    onLibraryClick: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    var selectedTab by remember { mutableStateOf(AggregateTab.ContinueWatching) }
    val uiState by viewModel.uiState.collectAsState()

    val tabs = listOf(
        AggregateTab.ContinueWatching to "继续观看",
        AggregateTab.Favorites to "收藏",
        AggregateTab.Libraries to "媒体库",
    )

    Column(
        modifier = modifier.fillMaxSize(),
    ) {
        // 页面标题
        MobilePageHeader(
            title = "聚合视界",
            actions = listOf(
                PageHeaderAction(
                    icon = Icons.Default.Search,
                    contentDescription = "搜索",
                    onClick = { /* TODO: 导航到搜索 */ },
                ),
            ),
        )

        // Tab 切换
        SegmentedTabs(
            tabs = tabs.map { it.first.name to it.second },
            selectedTab = selectedTab.name,
            onTabSelected = { selectedTab = AggregateTab.valueOf(it) },
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = MobileSpacing.xl),
        )

        // 内容区域
        AnimatedContent(
            targetState = selectedTab,
            transitionSpec = { fadeIn() togetherWith fadeOut() },
            label = "AggregateTabTransition",
        ) { tab ->
            when (tab) {
                AggregateTab.ContinueWatching -> ContinueWatchingContent(
                    viewModel = viewModel,
                    onMediaClick = onMediaClick,
                    onPlayerClick = onPlayerClick,
                )
                AggregateTab.Favorites -> FavoritesContent(
                    onMediaClick = onMediaClick,
                )
                AggregateTab.Libraries -> LibrariesContent(
                    viewModel = viewModel,
                    onLibraryClick = onLibraryClick,
                )
            }
        }
    }
}

@Composable
private fun ContinueWatchingContent(
    viewModel: HomeViewModel,
    onMediaClick: (String) -> Unit,
    onPlayerClick: (String) -> Unit,
) {
    val uiState by viewModel.uiState.collectAsState()

    when {
        uiState.loading -> {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator()
            }
        }
        uiState.continueWatching.isEmpty() -> {
            EmptyState(
                icon = Icons.Default.PlayArrow,
                title = "暂无继续观看",
                subtitle = "开始播放后会显示在这里",
            )
        }
        else -> {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(MobileSpacing.xl),
                verticalArrangement = Arrangement.spacedBy(MobileSpacing.md),
            ) {
                items(uiState.continueWatching) { history ->
                    val media = history.media
                    MediaPosterCard(
                        title = media?.title ?: "未知",
                        year = media?.year,
                        imageUrl = if (media?.posterPath?.isNotBlank() == true) {
                            "${uiState.serverUrl}/api/media/${media.id}/poster"
                        } else {
                            null
                        },
                        progress = if (history.duration > 0) {
                            (history.position / history.duration).toFloat().coerceIn(0f, 1f)
                        } else {
                            null
                        },
                        onClick = { onPlayerClick(history.mediaId) },
                    )
                }
            }
        }
    }
}

@Composable
private fun FavoritesContent(
    onMediaClick: (String) -> Unit,
) {
    // TODO: 接入收藏 API
    EmptyState(
        icon = Icons.Default.FavoriteBorder,
        title = "还没有收藏",
        subtitle = "点亮喜欢的影片后会出现在这里",
    )
}

@Composable
private fun LibrariesContent(
    viewModel: HomeViewModel,
    onLibraryClick: (String) -> Unit,
) {
    val uiState by viewModel.uiState.collectAsState()

    when {
        uiState.loading -> {
            Box(
                modifier = Modifier.fillMaxSize(),
                contentAlignment = Alignment.Center,
            ) {
                CircularProgressIndicator()
            }
        }
        uiState.libraries.isEmpty() -> {
            EmptyState(
                icon = Icons.Default.FolderOpen,
                title = "暂无媒体库",
                subtitle = "请先创建媒体库",
            )
        }
        else -> {
            LazyColumn(
                modifier = Modifier.fillMaxSize(),
                contentPadding = PaddingValues(MobileSpacing.xl),
                verticalArrangement = Arrangement.spacedBy(MobileSpacing.md),
            ) {
                items(uiState.libraries) { library ->
                    LibraryItem(
                        library = library,
                        onClick = { onLibraryClick(library.id) },
                    )
                }
            }
        }
    }
}
