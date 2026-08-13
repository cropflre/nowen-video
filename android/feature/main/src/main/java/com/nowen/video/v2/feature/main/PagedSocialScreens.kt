@file:OptIn(androidx.compose.material3.ExperimentalMaterial3Api::class)

package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Movie
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
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
import androidx.paging.PagingData
import androidx.paging.cachedIn
import androidx.paging.compose.LazyPagingItems
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
    val items = viewModel.favorites.collectAsLazyPagingItems()
    val action by viewModel.action.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()

    PagedSocialScaffold(
        title = "我的收藏",
        icon = { Icon(Icons.Default.Favorite, contentDescription = null) },
        onBack = onBack,
        items = items,
        emptyTitle = "还没有收藏",
        emptyMessage = "在媒体详情页点击收藏，喜欢的内容会出现在这里。",
        actionError = action.error,
    ) { favorite ->
        val media = favorite.media
        val mediaId = favorite.mediaId.ifBlank { media.resolvedId }
        PagedMediaRow(
            media = media,
            imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
            subtitle = listOfNotNull(media.year?.toString(), "已收藏").joinToString(" · "),
            onClick = { onMediaClick(mediaId) },
            action = {
                IconButton(
                    onClick = { viewModel.remove(mediaId, items::refresh) },
                    enabled = action.runningId != mediaId,
                ) {
                    if (action.runningId == mediaId) {
                        CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                    } else {
                        Icon(Icons.Default.Delete, contentDescription = "取消收藏")
                    }
                }
            },
        )
    }
}

@Composable
fun PagedHistoryScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    viewModel: PagedHistoryViewModel = hiltViewModel(),
) {
    val items = viewModel.history.collectAsLazyPagingItems()
    val action by viewModel.action.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var confirmClear by remember { mutableStateOf(false) }

    PagedSocialScaffold(
        title = "观看历史",
        icon = { Icon(Icons.Default.History, contentDescription = null) },
        onBack = onBack,
        items = items,
        emptyTitle = "暂无观看历史",
        emptyMessage = "开始播放后，观看进度会自动记录在这里。",
        actionError = action.error,
        topAction = if (items.itemCount == 0) null else {
            {
                IconButton(onClick = { confirmClear = true }, enabled = !action.clearing) {
                    if (action.clearing) {
                        CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                    } else {
                        Icon(Icons.Default.DeleteSweep, contentDescription = "清空历史")
                    }
                }
            }
        },
    ) { history ->
        val media = history.media
        val mediaId = history.mediaId.ifBlank { media.resolvedId }
        PagedMediaRow(
            media = media,
            imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
            subtitle = history.progressLabel,
            progress = history.normalizedProgress,
            onClick = { onMediaClick(mediaId) },
            action = {
                IconButton(
                    onClick = { viewModel.delete(mediaId, items::refresh) },
                    enabled = action.runningId != mediaId,
                ) {
                    if (action.runningId == mediaId) {
                        CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                    } else {
                        Icon(Icons.Default.Delete, contentDescription = "删除观看记录")
                    }
                }
            },
        )
    }

    if (confirmClear) {
        AlertDialog(
            onDismissRequest = { confirmClear = false },
            title = { Text("清空观看历史？") },
            text = { Text("这会删除当前服务器账号的全部观看记录，已下载文件不会受影响。") },
            confirmButton = {
                TextButton(onClick = {
                    confirmClear = false
                    viewModel.clear(items::refresh)
                }) { Text("确认清空") }
            },
            dismissButton = { TextButton(onClick = { confirmClear = false }) { Text("取消") } },
        )
    }
}

@Composable
private fun <T : Any> PagedSocialScaffold(
    title: String,
    icon: @Composable () -> Unit,
    onBack: () -> Unit,
    items: LazyPagingItems<T>,
    emptyTitle: String,
    emptyMessage: String,
    actionError: String?,
    topAction: (@Composable () -> Unit)? = null,
    row: @Composable (T) -> Unit,
) {
    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        icon()
                        Spacer(Modifier.width(10.dp))
                        Text(title)
                    }
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                },
                actions = { topAction?.invoke() },
            )
        },
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .background(MaterialTheme.colorScheme.background)
                .padding(padding),
            contentPadding = PaddingValues(horizontal = 20.dp, vertical = 18.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            actionError?.let { message -> item { MessagePanel("操作失败", message) } }
            when {
                items.loadState.refresh is androidx.paging.LoadState.Loading -> item {
                    ElevatedPanel(Modifier.fillMaxWidth()) {
                        LinearProgressIndicator(Modifier.fillMaxWidth())
                        Spacer(Modifier.height(12.dp))
                        Text("正在同步$title")
                    }
                }
                items.loadState.refresh is androidx.paging.LoadState.Error -> item {
                    val error = (items.loadState.refresh as androidx.paging.LoadState.Error).error
                    MessagePanel("加载失败", error.message ?: "网络请求失败", "重试", items::retry)
                }
                items.itemCount == 0 -> item { MessagePanel(emptyTitle, emptyMessage) }
                else -> items(items.itemCount) { index ->
                    val value = items[index]
                    if (value != null) row(value)
                }
            }
            when (val append = items.loadState.append) {
                is androidx.paging.LoadState.Loading -> item {
                    LinearProgressIndicator(Modifier.fillMaxWidth())
                }
                is androidx.paging.LoadState.Error -> item {
                    MessagePanel("加载更多失败", append.error.message ?: "网络请求失败", "重试", items::retry)
                }
                else -> Unit
            }
        }
    }
}

@Composable
private fun PagedMediaRow(
    media: MediaCard,
    imageUrl: String?,
    subtitle: String,
    onClick: () -> Unit,
    progress: Float = 0f,
    action: @Composable () -> Unit,
) {
    ElevatedPanel(
        Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            AsyncImage(
                model = imageUrl,
                contentDescription = media.displayTitle,
                contentScale = ContentScale.Crop,
                modifier = Modifier
                    .width(68.dp)
                    .aspectRatio(2f / 3f)
                    .clip(MaterialTheme.shapes.medium)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
            )
            Spacer(Modifier.width(14.dp))
            Column(Modifier.weight(1f)) {
                Text(media.displayTitle, style = MaterialTheme.typography.titleMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
                if (subtitle.isNotBlank()) {
                    Spacer(Modifier.height(5.dp))
                    Text(subtitle, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                if (progress > 0f) {
                    Spacer(Modifier.height(9.dp))
                    LinearProgressIndicator(progress = { progress }, modifier = Modifier.fillMaxWidth())
                }
            }
            Spacer(Modifier.width(8.dp))
            action()
            Icon(Icons.Default.Movie, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        }
    }
}
