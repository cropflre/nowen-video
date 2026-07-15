package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.DeleteSweep
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Movie
import androidx.compose.material.icons.filled.Person
import androidx.compose.material3.*
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
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.CollectionWithMedia
import com.nowen.video.v2.core.model.FavoriteRecord
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MovieCollection
import com.nowen.video.v2.core.model.Person
import com.nowen.video.v2.core.model.PersonMediaResponse
import com.nowen.video.v2.core.model.WatchHistoryRecord
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.async
import kotlinx.coroutines.coroutineScope
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

private data class ListUiState<T>(
    val loading: Boolean = true,
    val items: List<T> = emptyList(),
    val actionId: String? = null,
    val error: String? = null,
)

@HiltViewModel
class FavoritesViewModel @Inject constructor(
    private val repository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(ListUiState<FavoriteRecord>())
    val state: StateFlow<ListUiState<FavoriteRecord>> = _state

    init { load() }

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            repository.favorites()
                .onSuccess { page -> _state.value = ListUiState(loading = false, items = page.data) }
                .onFailure { error ->
                    _state.update { it.copy(loading = false, error = error.message ?: "收藏加载失败") }
                }
        }
    }

    fun remove(mediaId: String) {
        viewModelScope.launch {
            _state.update { it.copy(actionId = mediaId, error = null) }
            repository.setFavorite(mediaId, false)
                .onSuccess {
                    _state.update { current ->
                        current.copy(
                            actionId = null,
                            items = current.items.filterNot { it.mediaId == mediaId || it.media.resolvedId == mediaId },
                        )
                    }
                }
                .onFailure { error ->
                    _state.update { it.copy(actionId = null, error = error.message ?: "取消收藏失败") }
                }
        }
    }
}

@Composable
fun FavoritesScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    viewModel: FavoritesViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    CatalogListScaffold(
        title = "我的收藏",
        icon = { Icon(Icons.Default.Favorite, contentDescription = null) },
        onBack = onBack,
    ) {
        when {
            state.loading -> item { LoadingPanel("正在同步收藏") }
            state.error != null -> item { MessagePanel("收藏加载失败", state.error!!, "重试", viewModel::load) }
            state.items.isEmpty() -> item { MessagePanel("还没有收藏", "在媒体详情页点击收藏，喜欢的内容会出现在这里。") }
            else -> items(state.items, key = { it.id.ifBlank { it.mediaId } }) { favorite ->
                val media = favorite.media
                MediaActionRow(
                    media = media,
                    imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                    subtitle = listOfNotNull(media.year?.toString(), "已收藏").joinToString(" · "),
                    onClick = { onMediaClick(media.resolvedId.ifBlank { favorite.mediaId }) },
                    action = {
                        IconButton(
                            onClick = { viewModel.remove(favorite.mediaId.ifBlank { media.resolvedId }) },
                            enabled = state.actionId != favorite.mediaId,
                        ) {
                            if (state.actionId == favorite.mediaId) {
                                CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                            } else {
                                Icon(Icons.Default.Delete, contentDescription = "取消收藏")
                            }
                        }
                    },
                )
            }
        }
    }
}

@HiltViewModel
class HistoryViewModel @Inject constructor(
    private val repository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(ListUiState<WatchHistoryRecord>())
    val state: StateFlow<ListUiState<WatchHistoryRecord>> = _state

    init { load() }

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            repository.history()
                .onSuccess { page -> _state.value = ListUiState(loading = false, items = page.data) }
                .onFailure { error ->
                    _state.update { it.copy(loading = false, error = error.message ?: "历史加载失败") }
                }
        }
    }

    fun delete(mediaId: String) {
        viewModelScope.launch {
            _state.update { it.copy(actionId = mediaId, error = null) }
            repository.deleteHistory(mediaId)
                .onSuccess {
                    _state.update { current ->
                        current.copy(
                            actionId = null,
                            items = current.items.filterNot { it.mediaId == mediaId },
                        )
                    }
                }
                .onFailure { error ->
                    _state.update { it.copy(actionId = null, error = error.message ?: "删除观看记录失败") }
                }
        }
    }

    fun clear() {
        viewModelScope.launch {
            _state.update { it.copy(actionId = CLEAR_ALL_ACTION, error = null) }
            repository.clearHistory()
                .onSuccess { _state.value = ListUiState(loading = false) }
                .onFailure { error ->
                    _state.update { it.copy(actionId = null, error = error.message ?: "清空观看历史失败") }
                }
        }
    }

    private companion object { const val CLEAR_ALL_ACTION = "__clear_all__" }
}

@Composable
fun HistoryScreen(
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    viewModel: HistoryViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var confirmClear by remember { mutableStateOf(false) }

    CatalogListScaffold(
        title = "观看历史",
        icon = { Icon(Icons.Default.History, contentDescription = null) },
        onBack = onBack,
        topAction = if (state.items.isEmpty()) null else {
            {
                IconButton(onClick = { confirmClear = true }, enabled = state.actionId == null) {
                    Icon(Icons.Default.DeleteSweep, contentDescription = "清空历史")
                }
            }
        },
    ) {
        when {
            state.loading -> item { LoadingPanel("正在同步观看历史") }
            state.error != null -> item { MessagePanel("历史加载失败", state.error!!, "重试", viewModel::load) }
            state.items.isEmpty() -> item { MessagePanel("暂无观看历史", "开始播放后，观看进度会自动记录在这里。") }
            else -> items(state.items, key = { it.id.ifBlank { it.mediaId } }) { history ->
                val media = history.media
                MediaActionRow(
                    media = media,
                    imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                    subtitle = history.progressLabel,
                    progress = history.normalizedProgress,
                    onClick = { onMediaClick(media.resolvedId.ifBlank { history.mediaId }) },
                    action = {
                        IconButton(
                            onClick = { viewModel.delete(history.mediaId) },
                            enabled = state.actionId != history.mediaId,
                        ) {
                            if (state.actionId == history.mediaId) {
                                CircularProgressIndicator(Modifier.size(20.dp), strokeWidth = 2.dp)
                            } else {
                                Icon(Icons.Default.Delete, contentDescription = "删除观看记录")
                            }
                        }
                    },
                )
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
                    viewModel.clear()
                }) { Text("确认清空") }
            },
            dismissButton = { TextButton(onClick = { confirmClear = false }) { Text("取消") } },
        )
    }
}

@HiltViewModel
class CollectionsViewModel @Inject constructor(
    private val repository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(ListUiState<MovieCollection>())
    val state: StateFlow<ListUiState<MovieCollection>> = _state

    init { load() }

    fun load() {
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            repository.collections()
                .onSuccess { page -> _state.value = ListUiState(loading = false, items = page.data) }
                .onFailure { error ->
                    _state.update { it.copy(loading = false, error = error.message ?: "合集加载失败") }
                }
        }
    }
}

@Composable
fun CollectionsScreen(
    onBack: () -> Unit,
    onCollectionClick: (String) -> Unit,
    viewModel: CollectionsViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    CatalogListScaffold(
        title = "系列合集",
        icon = { Icon(Icons.Default.Collections, contentDescription = null) },
        onBack = onBack,
    ) {
        when {
            state.loading -> item { LoadingPanel("正在整理系列合集") }
            state.error != null -> item { MessagePanel("合集加载失败", state.error!!, "重试", viewModel::load) }
            state.items.isEmpty() -> item { MessagePanel("暂无系列合集", "服务器完成合集匹配后会显示在这里。") }
            else -> items(state.items, key = MovieCollection::id) { collection ->
                ElevatedPanel(
                    Modifier
                        .fillMaxWidth()
                        .clickable { onCollectionClick(collection.id) },
                ) {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        AsyncImage(
                            model = collectionPosterUrl(session.activeServer?.baseUrl, collection.id),
                            contentDescription = collection.name,
                            contentScale = ContentScale.Crop,
                            modifier = Modifier
                                .width(76.dp)
                                .aspectRatio(2f / 3f)
                                .clip(MaterialTheme.shapes.medium)
                                .background(MaterialTheme.colorScheme.surfaceVariant),
                        )
                        Spacer(Modifier.width(16.dp))
                        Column(Modifier.weight(1f)) {
                            Text(collection.name, style = MaterialTheme.typography.titleLarge)
                            Spacer(Modifier.height(6.dp))
                            Text(
                                listOfNotNull(
                                    collection.yearRange.takeIf(String::isNotBlank),
                                    collection.mediaCount.takeIf { it > 0 }?.let { "$it 部电影" },
                                ).joinToString(" · ").ifBlank { "系列合集" },
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                            if (collection.overview.isNotBlank()) {
                                Spacer(Modifier.height(6.dp))
                                Text(
                                    collection.overview,
                                    maxLines = 2,
                                    overflow = TextOverflow.Ellipsis,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        }
                    }
                }
                Spacer(Modifier.height(12.dp))
            }
        }
    }
}

private data class CollectionDetailUiState(
    val loading: Boolean = true,
    val detail: CollectionWithMedia? = null,
    val error: String? = null,
)

@HiltViewModel
class CollectionDetailViewModel @Inject constructor(
    private val repository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(CollectionDetailUiState())
    val state: StateFlow<CollectionDetailUiState> = _state
    private var loadedId: String? = null

    fun load(id: String) {
        if (id == loadedId && _state.value.detail != null) return
        loadedId = id
        viewModelScope.launch {
            _state.value = CollectionDetailUiState(loading = true)
            repository.collection(id)
                .onSuccess { _state.value = CollectionDetailUiState(loading = false, detail = it) }
                .onFailure { error ->
                    _state.value = CollectionDetailUiState(loading = false, error = error.message ?: "合集详情加载失败")
                }
        }
    }
}

@Composable
fun CollectionDetailScreen(
    collectionId: String,
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    viewModel: CollectionDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    LaunchedEffect(collectionId) { viewModel.load(collectionId) }

    CatalogListScaffold(
        title = state.detail?.collection?.name ?: "合集详情",
        icon = { Icon(Icons.Default.Collections, contentDescription = null) },
        onBack = onBack,
    ) {
        when {
            state.loading -> item { LoadingPanel("正在加载合集详情") }
            state.error != null -> item { MessagePanel("无法打开合集", state.error!!, "重试", { viewModel.load(collectionId) }) }
            state.detail != null -> {
                val detail = state.detail!!
                item {
                    ElevatedPanel(Modifier.fillMaxWidth()) {
                        Row(verticalAlignment = Alignment.Top) {
                            AsyncImage(
                                model = collectionPosterUrl(session.activeServer?.baseUrl, detail.collection.id),
                                contentDescription = detail.collection.name,
                                contentScale = ContentScale.Crop,
                                modifier = Modifier
                                    .width(112.dp)
                                    .aspectRatio(2f / 3f)
                                    .clip(MaterialTheme.shapes.large)
                                    .background(MaterialTheme.colorScheme.surfaceVariant),
                            )
                            Spacer(Modifier.width(18.dp))
                            Column(Modifier.weight(1f)) {
                                Text(detail.collection.name, style = MaterialTheme.typography.headlineSmall)
                                Spacer(Modifier.height(8.dp))
                                Text(
                                    listOfNotNull(
                                        detail.collection.yearRange.takeIf(String::isNotBlank),
                                        detail.media.size.takeIf { it > 0 }?.let { "$it 部作品" },
                                    ).joinToString(" · "),
                                    color = MaterialTheme.colorScheme.primary,
                                )
                                Spacer(Modifier.height(12.dp))
                                Text(
                                    detail.collection.overview.ifBlank { "暂无合集简介" },
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                )
                            }
                        }
                    }
                    Spacer(Modifier.height(22.dp))
                    Text("合集作品", style = MaterialTheme.typography.titleLarge)
                    Spacer(Modifier.height(12.dp))
                }
                items(detail.media, key = { it.id }) { media ->
                    val card = MediaCard(
                        id = media.id,
                        title = media.title,
                        year = media.year,
                        posterPath = media.posterPath,
                    )
                    MediaActionRow(
                        media = card,
                        imageUrl = resolveImage(session.activeServer?.baseUrl, media.posterPath),
                        subtitle = listOfNotNull(
                            media.year.takeIf { it > 0 }?.toString(),
                            media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
                            media.resolution.takeIf(String::isNotBlank),
                        ).joinToString(" · "),
                        onClick = { onMediaClick(media.id) },
                    )
                }
            }
        }
    }
}

private data class PersonDetailUiState(
    val loading: Boolean = true,
    val person: Person? = null,
    val works: PersonMediaResponse = PersonMediaResponse(),
    val error: String? = null,
)

@HiltViewModel
class PersonDetailViewModel @Inject constructor(
    private val repository: SocialCatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(PersonDetailUiState())
    val state: StateFlow<PersonDetailUiState> = _state
    private var loadedId: String? = null

    fun load(id: String) {
        if (id == loadedId && _state.value.person != null) return
        loadedId = id
        viewModelScope.launch {
            _state.value = PersonDetailUiState(loading = true)
            runCatching {
                coroutineScope {
                    val person = async { repository.person(id).getOrThrow() }
                    val works = async { repository.personMedia(id).getOrThrow() }
                    person.await() to works.await()
                }
            }.onSuccess { (person, works) ->
                _state.value = PersonDetailUiState(loading = false, person = person, works = works)
            }.onFailure { error ->
                _state.value = PersonDetailUiState(loading = false, error = error.message ?: "人物详情加载失败")
            }
        }
    }
}

@Composable
fun PersonDetailScreen(
    personId: String,
    onBack: () -> Unit,
    onMediaClick: (String) -> Unit,
    viewModel: PersonDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    LaunchedEffect(personId) { viewModel.load(personId) }

    CatalogListScaffold(
        title = state.person?.name ?: "人物详情",
        icon = { Icon(Icons.Default.Person, contentDescription = null) },
        onBack = onBack,
    ) {
        when {
            state.loading -> item { LoadingPanel("正在加载人物资料") }
            state.error != null -> item { MessagePanel("无法打开人物详情", state.error!!, "重试", { viewModel.load(personId) }) }
            state.person != null -> {
                val person = state.person!!
                item {
                    ElevatedPanel(Modifier.fillMaxWidth()) {
                        Row(verticalAlignment = Alignment.CenterVertically) {
                            AsyncImage(
                                model = personProfileUrl(session.activeServer?.baseUrl, person.id),
                                contentDescription = person.name,
                                contentScale = ContentScale.Crop,
                                modifier = Modifier
                                    .size(112.dp)
                                    .clip(MaterialTheme.shapes.large)
                                    .background(MaterialTheme.colorScheme.surfaceVariant),
                            )
                            Spacer(Modifier.width(18.dp))
                            Column(Modifier.weight(1f)) {
                                Text(person.name, style = MaterialTheme.typography.headlineSmall)
                                person.originalName.takeIf(String::isNotBlank)?.let {
                                    Spacer(Modifier.height(6.dp))
                                    Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant)
                                }
                                val total = state.works.media.size + state.works.series.size
                                if (total > 0) {
                                    Spacer(Modifier.height(10.dp))
                                    Text("$total 部馆藏作品", color = MaterialTheme.colorScheme.primary)
                                }
                            }
                        }
                    }
                }
                if (state.works.media.isNotEmpty()) {
                    item {
                        Spacer(Modifier.height(22.dp))
                        Text("电影与单集", style = MaterialTheme.typography.titleLarge)
                        Spacer(Modifier.height(12.dp))
                    }
                    items(state.works.media, key = { "media-${it.resolvedId}" }) { media ->
                        MediaActionRow(
                            media = media,
                            imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                            subtitle = media.year?.toString().orEmpty(),
                            onClick = { onMediaClick(media.resolvedId) },
                        )
                    }
                }
                if (state.works.series.isNotEmpty()) {
                    item {
                        Spacer(Modifier.height(22.dp))
                        Text("剧集作品", style = MaterialTheme.typography.titleLarge)
                        Spacer(Modifier.height(4.dp))
                        Text(
                            "剧集完整季集导航将在后续系列详情阶段开放。",
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        Spacer(Modifier.height(12.dp))
                    }
                    items(state.works.series, key = { "series-${it.resolvedId}" }) { series ->
                        MediaActionRow(
                            media = series,
                            imageUrl = resolveImage(session.activeServer?.baseUrl, series.resolvedPoster),
                            subtitle = listOfNotNull(series.year?.toString(), "剧集").joinToString(" · "),
                            onClick = null,
                        )
                    }
                }
                if (state.works.media.isEmpty() && state.works.series.isEmpty()) {
                    item { MessagePanel("暂无馆藏作品", "当前服务器中还没有这个人物关联的影视内容。") }
                }
            }
        }
    }
}

@Composable
private fun CatalogListScaffold(
    title: String,
    icon: @Composable () -> Unit,
    onBack: () -> Unit,
    topAction: (@Composable () -> Unit)? = null,
    content: androidx.compose.foundation.lazy.LazyListScope.() -> Unit,
) {
    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        icon()
                        Spacer(Modifier.width(10.dp))
                        Text(title, maxLines = 1, overflow = TextOverflow.Ellipsis)
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
            content = content,
        )
    }
}

@Composable
private fun LoadingPanel(label: String) {
    ElevatedPanel(Modifier.fillMaxWidth()) {
        LinearProgressIndicator(Modifier.fillMaxWidth())
        Spacer(Modifier.height(12.dp))
        Text(label)
    }
}

@Composable
private fun MediaActionRow(
    media: MediaCard,
    imageUrl: String?,
    subtitle: String,
    onClick: (() -> Unit)?,
    progress: Float = 0f,
    action: (@Composable () -> Unit)? = null,
) {
    ElevatedPanel(
        Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier),
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
                Text(media.displayTitle, style = MaterialTheme.typography.titleMedium, maxLines = 2)
                if (subtitle.isNotBlank()) {
                    Spacer(Modifier.height(5.dp))
                    Text(subtitle, color = MaterialTheme.colorScheme.onSurfaceVariant)
                }
                if (progress > 0f) {
                    Spacer(Modifier.height(9.dp))
                    LinearProgressIndicator(progress = { progress }, modifier = Modifier.fillMaxWidth())
                }
            }
            action?.let {
                Spacer(Modifier.width(8.dp))
                it()
            } ?: Icon(Icons.Default.Movie, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        }
    }
}

internal fun collectionPosterUrl(baseUrl: String?, collectionId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/collections/$collectionId/poster" }

internal fun personProfileUrl(baseUrl: String?, personId: String): String? =
    baseUrl?.trimEnd('/')?.let { "$it/api/persons/$personId/profile" }
