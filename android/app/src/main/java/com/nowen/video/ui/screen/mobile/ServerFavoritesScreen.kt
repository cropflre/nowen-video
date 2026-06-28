package com.nowen.video.ui.screen.mobile

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.FavoriteBorder
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.ui.component.mobile.EmptyState
import com.nowen.video.ui.component.mobile.MediaPosterCard
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.theme.MobileSpacing
import com.nowen.video.ui.screen.favorites.FavoritesViewModel
import com.nowen.video.ui.util.buildPosterUrl

/**
 * 服务器收藏页
 * 展示已收藏的媒体
 */
@Composable
fun ServerFavoritesScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    modifier: Modifier = Modifier,
    viewModel: FavoritesViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()

    // 加载数据
    LaunchedEffect(Unit) {
        viewModel.loadFavorites()
    }

    Box(
        modifier = modifier.fillMaxSize(),
    ) {
        // 页面标题
        MobilePageHeader(
            title = "收藏",
            onBack = onBack,
        )

        when {
            uiState.loading -> {
                CircularProgressIndicator(
                    modifier = Modifier.align(Alignment.Center),
                )
            }
            uiState.error != null -> {
                EmptyState(
                    icon = Icons.Default.FavoriteBorder,
                    title = "加载失败",
                    subtitle = uiState.error,
                    modifier = Modifier.align(Alignment.Center),
                )
            }
            uiState.favorites.isEmpty() -> {
                EmptyState(
                    icon = Icons.Default.FavoriteBorder,
                    title = "还没有收藏",
                    subtitle = "点亮喜欢的影片后会出现在这里",
                    modifier = Modifier.align(Alignment.Center),
                )
            }
            else -> {
                LazyVerticalGrid(
                    columns = GridCells.Fixed(2),
                    contentPadding = PaddingValues(
                        start = MobileSpacing.xl,
                        end = MobileSpacing.xl,
                        top = 100.dp, // 为标题留空间
                        bottom = MobileSpacing.xl,
                    ),
                    horizontalArrangement = Arrangement.spacedBy(MobileSpacing.md),
                    verticalArrangement = Arrangement.spacedBy(MobileSpacing.md),
                ) {
                    items(uiState.favorites) { media ->
                        MediaPosterCard(
                            title = media.title,
                            year = media.year,
                            imageUrl = if (media.posterPath.isNotBlank()) {
                                buildPosterUrl(uiState.serverUrl, media.id, "media", uiState.token)
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
