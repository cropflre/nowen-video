package com.nowen.video.ui.screen.mobile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.imePadding
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
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
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.ui.component.mobile.EmptyState
import com.nowen.video.ui.component.mobile.MediaPosterCard
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.component.mobile.SearchBarLarge
import com.nowen.video.ui.theme.MobileSpacing
import com.nowen.video.ui.screen.search.SearchViewModel

/**
 * 服务器搜索页
 * 大圆角搜索框 + 搜索结果
 */
@Composable
fun ServerSearchScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: SearchViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()
    var query by remember { mutableStateOf("") }

    Column(
        modifier = modifier
            .fillMaxSize()
            .imePadding(),
    ) {
        // 页面标题
        MobilePageHeader(
            title = "搜索",
            onBack = onBack,
        )

        // 搜索框
        SearchBarLarge(
            query = query,
            onQueryChange = { query = it },
            onSearch = { viewModel.search(it) },
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = MobileSpacing.xl),
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
            query.isBlank() -> {
                // 空输入状态
                EmptyState(
                    icon = Icons.Default.Search,
                    title = "输入搜索内容",
                    subtitle = "搜索你的媒体库",
                )
            }
            uiState.media.isEmpty() && !uiState.loading -> {
                // 无结果
                EmptyState(
                    icon = Icons.Default.Search,
                    title = "没有找到相关内容",
                    subtitle = "换个关键词试试",
                )
            }
            else -> {
                // 搜索结果
                LazyVerticalGrid(
                    columns = GridCells.Fixed(2),
                    contentPadding = PaddingValues(
                        start = MobileSpacing.xl,
                        end = MobileSpacing.xl,
                        top = MobileSpacing.md,
                        bottom = MobileSpacing.xl,
                    ),
                    horizontalArrangement = Arrangement.spacedBy(MobileSpacing.md),
                    verticalArrangement = Arrangement.spacedBy(MobileSpacing.md),
                ) {
                    items(uiState.media) { media ->
                        MediaPosterCard(
                            title = media.title,
                            year = media.year,
                            imageUrl = if (media.posterPath.isNotBlank()) {
                                "${uiState.serverUrl}/api/media/${media.id}/poster"
                            } else {
                                null
                            },
                            onClick = { onMediaClick(media.id) },
                        )
                    }
                }
            }
        }
    }
}
