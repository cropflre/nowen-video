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
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.GridItemSpan
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LinearProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.paging.LoadState
import androidx.paging.PagingData
import androidx.paging.cachedIn
import androidx.paging.compose.collectAsLazyPagingItems
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.FavoriteRecord
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.WatchHistoryRecord
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

@HiltViewModel
class PagedFavoritesViewModel @Inject constructor(
    private val repository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    val favorites: Flow<PagingData<FavoriteRecord>> = repository.pagedFavorites().cachedIn(viewModelScope)
    private val _action = MutableStateFlow(PagedSocialAction())
    val action: StateFlow<PagedSocialAction> = _action

    fun remove(mediaId: String, onSuccess: () -> Unit) {
        viewModelScope.launch {
            _action.value = PagedSocialAction(runningId = mediaId)
            repository.setFavorite(mediaId, false)
                .onSuccess {
                    _action.value = PagedSocialAction()
                    onSuccess()
                }
                .onFailure { error ->
                    _action.value = PagedSocialAction(error = error.message ?: "取消收藏失败")
                }
        }
    }
}

@HiltViewModel
class PagedHistoryViewModel @Inject constructor(
    private val repository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    val history: Flow<PagingData<WatchHistoryRecord>> = repository.pagedHistory().cachedIn(viewModelScope)
    private val _action = MutableStateFlow(PagedSocialAction())
    val action: StateFlow<PagedSocialAction> = _action

    fun delete(mediaId: String, onSuccess: () -> Unit) {
        viewModelScope.launch {
            _action.value = PagedSocialAction(runningId = mediaId)
            repository.deleteHistory(mediaId)
                .onSuccess {
                    _action.value = PagedSocialAction()
                    onSuccess()
                }
                .onFailure { error ->
                    _action.value = PagedSocialAction(error = error.message ?: "删除观看记录失败")
                }
        }
    }

    fun clear(onSuccess: () -> Unit) {
        viewModelScope.launch {
            _action.value = PagedSocialAction(clearing = true)
            repository.clearHistory()
                .onSuccess {
                    _action.value = PagedSocialAction()
                    onSuccess()
                }
                .onFailure { error ->
                    _action.value = PagedSocialAction(error = error.message ?: "清空观看历史失败")
                }
        }
    }
}

data class PagedSocialAction(
    val runningId: String? = null,
    val clearing: Boolean = false,
    val error: String? = null,
)

@Composable
fun PagedFavoritesScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    viewModel: PagedFavoritesViewModel = hiltViewModel(),
) {
    val favorites = viewModel.favorites.collectAsLazyPagingItems()
    val action by viewModel.action.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("我的收藏") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                },
            )
        },
    ) { padding ->
        LazyVerticalGrid(
            columns = GridCells.Adaptive(minSize = 146.dp),
            modifier = Modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background)
                .padding(padding),
            contentPadding = PaddingValues(horizontal = 18.dp, vertical = 18.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp),
            verticalArrangement = Arrangement.spacedBy(18.dp),
        ) {
            item(span = { GridItemSpan(maxLineSpan) }) {
                PersonalWorkspaceHeader(
                    icon = { Icon(Icons.Default.Favorite, contentDescription = null) },
                    eyebrow = "MY LIBRARY",
                    title = "收藏内容",
                    subtitle = "把喜欢的电影与单集留在一个更容易再次找到的位置。",
                    count = favorites.itemCount,
                )
            }

            action.error?.let { message ->
                item(span = { GridItemSpan(maxLineSpan) }) { MessagePanel("操作失败", message) }
            }

            when (val refresh = favorites.loadState.refresh) {
                is LoadState.Loading -> item(span = { GridItemSpan(maxLineSpan) }) {
                    ElevatedPanel(Modifier.fillMaxWidth()) {
                        LinearProgressIndicator(Modifier.fillMaxWidth())
                        Spacer(Modifier.height(10.dp))
                        Text("正在同步收藏内容")
                    }
                }
                is LoadState.Error -> item(span = { GridItemSpan(maxLineSpan) }) {
                    MessagePanel("加载失败", refresh.error.message ?: "网络请求失败", "重试", favorites::retry)
                }
                else -> if (favorites.itemCount == 0) {
                    item(span = { GridItemSpan(maxLineSpan) }) {
                        MessagePanel("还没有收藏", "在媒体详情页点击收藏，喜欢的内容会出现在这里。")
                    }
                }
            }

            items(favorites.itemCount) { index ->
                val favorite = favorites[index] ?: return@items
                val media = favorite.media
                val mediaId = favorite.mediaId.ifBlank { media.resolvedId }
                FavoriteWorkspaceCard(
                    media = media,
                    imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                    removing = action.runningId == mediaId,
                    onClick = { onMediaClick(mediaId) },
                    onRemove = { viewModel.remove(mediaId, favorites::refresh) },
                )
            }

            when (val append = favorites.loadState.append) {
                is LoadState.Loading -> item(span = { GridItemSpan(maxLineSpan) }) {
                    LinearProgressIndicator(Modifier.fillMaxWidth())
                }
                is LoadState.Error -> item(span = { GridItemSpan(maxLineSpan) }) {
                    MessagePanel("加载更多失败", append.error.message ?: "网络请求失败", "重试", favorites::retry)
                }
                else -> Unit
            }
        }
    }
}

@Composable
fun PagedHistoryScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    onPlay: (String) -> Unit,
    viewModel: PagedHistoryViewModel = hiltViewModel(),
) {
    val historyItems = viewModel.history.collectAsLazyPagingItems()
    val action by viewModel.action.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var confirmClear by remember { mutableStateOf(false) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("观看历史") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = {
                    if (historyItems.itemCount > 0) {
                        IconButton(onClick = { confirmClear = true }, enabled = !action.clearing) {
                            if (action.clearing) {
                                CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                            } else {
                                Icon(Icons.Default.DeleteSweep, contentDescription = "清空历史")
                            }
                        }
                    }
                },
            )
        },
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background)
                .padding(padding),
            contentPadding = PaddingValues(horizontal = 18.dp, vertical = 18.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            item {
                PersonalWorkspaceHeader(
                    icon = { Icon(Icons.Default.History, contentDescription = null) },
                    eyebrow = "CONTINUE WATCHING",
                    title = "最近观看",
                    subtitle = "从上次位置继续播放；需要更多信息时再进入详情。",
                    count = historyItems.itemCount,
                )
            }

            action.error?.let { message -> item { MessagePanel("操作失败", message) } }

            when (val refresh = historyItems.loadState.refresh) {
                is LoadState.Loading -> item {
                    ElevatedPanel(Modifier.fillMaxWidth()) {
                        LinearProgressIndicator(Modifier.fillMaxWidth())
                        Spacer(Modifier.height(10.dp))
                        Text("正在同步观看历史")
                    }
                }
                is LoadState.Error -> item {
                    MessagePanel("加载失败", refresh.error.message ?: "网络请求失败", "重试", historyItems::retry)
                }
                else -> if (historyItems.itemCount == 0) {
                    item { MessagePanel("暂无观看历史", "开始播放后，观看进度会自动记录在这里。") }
                }
            }

            items(historyItems.itemCount) { index ->
                val history = historyItems[index] ?: return@items
                val media = history.media
                val mediaId = history.mediaId.ifBlank { media.resolvedId }
                HistoryWorkspaceCard(
                    media = media,
                    history = history,
                    imageUrl = mediaBackdropUrl(session.activeServer?.baseUrl, mediaId),
                    deleting = action.runningId == mediaId,
                    onPlay = { onPlay(mediaId) },
                    onDetail = { onMediaClick(mediaId) },
                    onDelete = { viewModel.delete(mediaId, historyItems::refresh) },
                )
            }

            when (val append = historyItems.loadState.append) {
                is LoadState.Loading -> item { LinearProgressIndicator(Modifier.fillMaxWidth()) }
                is LoadState.Error -> item {
                    MessagePanel("加载更多失败", append.error.message ?: "网络请求失败", "重试", historyItems::retry)
                }
                else -> Unit
            }
        }
    }

    if (confirmClear) {
        AlertDialog(
            onDismissRequest = { confirmClear = false },
            title = { Text("清空观看历史？") },
            text = { Text("这会删除当前服务器账号的全部观看记录，已下载文件不会受影响。") },
            confirmButton = {
                TextButton(onClick = {
                    confirmClear = false
                    viewModel.clear(historyItems::refresh)
                }) { Text("确认清空") }
            },
            dismissButton = { TextButton(onClick = { confirmClear = false }) { Text("取消") } },
        )
    }
}

@Composable
private fun PersonalWorkspaceHeader(
    icon: @Composable () -> Unit,
    eyebrow: String,
    title: String,
    subtitle: String,
    count: Int,
) {
    ElevatedPanel(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Surface(
                modifier = Modifier.size(44.dp),
                shape = MaterialTheme.shapes.medium,
                color = MaterialTheme.colorScheme.primaryContainer,
                contentColor = MaterialTheme.colorScheme.onPrimaryContainer,
            ) {
                Box(contentAlignment = Alignment.Center) { icon() }
            }
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    eyebrow,
                    style = MaterialTheme.typography.labelLarge,
                    color = MaterialTheme.colorScheme.primary,
                )
                Text(title, style = MaterialTheme.typography.titleLarge)
                Spacer(Modifier.height(3.dp))
                Text(
                    subtitle,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
            if (count > 0) {
                Surface(
                    shape = MaterialTheme.shapes.extraLarge,
                    color = MaterialTheme.colorScheme.surfaceVariant,
                ) {
                    Text(
                        "$count",
                        modifier = Modifier.padding(horizontal = 11.dp, vertical = 6.dp),
                        style = MaterialTheme.typography.labelLarge,
                    )
                }
            }
        }
    }
}

@Composable
private fun FavoriteWorkspaceCard(
    media: MediaCard,
    imageUrl: String?,
    removing: Boolean,
    onClick: () -> Unit,
    onRemove: () -> Unit,
) {
    Column(modifier = Modifier.clickable(onClick = onClick)) {
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(MaterialTheme.shapes.large)
                .background(MaterialTheme.colorScheme.surfaceVariant),
        ) {
            AsyncImage(
                model = imageUrl,
                contentDescription = media.displayTitle,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxSize(),
            )
            Surface(
                modifier = Modifier
                    .align(Alignment.TopEnd)
                    .padding(8.dp),
                shape = MaterialTheme.shapes.extraLarge,
                color = Color.Black.copy(alpha = 0.58f),
                contentColor = Color.White,
            ) {
                IconButton(onClick = onRemove, enabled = !removing, modifier = Modifier.size(38.dp)) {
                    if (removing) {
                        CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp, color = Color.White)
                    } else {
                        Icon(Icons.Default.Delete, contentDescription = "取消收藏")
                    }
                }
            }
        }
        Spacer(Modifier.height(9.dp))
        Text(
            media.displayTitle,
            style = MaterialTheme.typography.titleMedium,
            maxLines = 2,
            overflow = TextOverflow.Ellipsis,
        )
        media.year?.let {
            Text(
                it.toString(),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun HistoryWorkspaceCard(
    media: MediaCard,
    history: WatchHistoryRecord,
    imageUrl: String?,
    deleting: Boolean,
    onPlay: () -> Unit,
    onDetail: () -> Unit,
    onDelete: () -> Unit,
) {
    ElevatedPanel(Modifier.fillMaxWidth()) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            Box(
                modifier = Modifier
                    .width(142.dp)
                    .aspectRatio(16f / 9f)
                    .clip(MaterialTheme.shapes.medium)
                    .background(MaterialTheme.colorScheme.surfaceVariant)
                    .clickable(onClick = onPlay),
            ) {
                AsyncImage(
                    model = imageUrl,
                    contentDescription = media.displayTitle,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                )
                Surface(
                    modifier = Modifier
                        .align(Alignment.Center)
                        .size(38.dp),
                    shape = MaterialTheme.shapes.extraLarge,
                    color = Color.Black.copy(alpha = 0.58f),
                    contentColor = Color.White,
                ) {
                    Box(contentAlignment = Alignment.Center) {
                        Icon(Icons.Default.PlayArrow, contentDescription = "继续播放")
                    }
                }
            }
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    media.displayTitle,
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )
                Spacer(Modifier.height(5.dp))
                Text(
                    history.progressLabel,
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                if (history.normalizedProgress > 0f) {
                    Spacer(Modifier.height(8.dp))
                    LinearProgressIndicator(
                        progress = { if (history.completed) 1f else history.normalizedProgress },
                        modifier = Modifier.fillMaxWidth(),
                    )
                }
            }
            IconButton(onClick = onDelete, enabled = !deleting) {
                if (deleting) {
                    CircularProgressIndicator(Modifier.size(18.dp), strokeWidth = 2.dp)
                } else {
                    Icon(Icons.Default.Delete, contentDescription = "删除观看记录")
                }
            }
        }
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(top = 12.dp),
            horizontalArrangement = Arrangement.End,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            TextButton(onClick = onDetail) { Text("查看详情") }
            Spacer(Modifier.width(8.dp))
            FilledTonalButton(onClick = onPlay) {
                Icon(Icons.Default.PlayArrow, contentDescription = null)
                Spacer(Modifier.width(6.dp))
                Text(if (history.completed) "重新播放" else "继续播放")
            }
        }
    }
}
