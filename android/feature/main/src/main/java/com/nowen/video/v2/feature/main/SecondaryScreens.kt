package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Logout
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.HillsPoster
import com.nowen.video.v2.core.designsystem.HillsScreen
import com.nowen.video.v2.core.designsystem.HillsSecondaryAction
import com.nowen.video.v2.core.designsystem.HillsState
import com.nowen.video.v2.core.designsystem.HillsTextField
import com.nowen.video.v2.core.designsystem.HillsTopBar
import com.nowen.video.v2.core.model.MediaCard
import com.nowen.video.v2.core.model.MovieCollection
import com.nowen.video.v2.core.model.Person
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.Job
import kotlinx.coroutines.async
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

private enum class SearchMediaKind(val label: String) {
    Movie("电影"),
    Series("剧集"),
    Episode("单集"),
}

data class SearchUiState(
    val query: String = "",
    val loading: Boolean = false,
    val mediaResults: List<MediaCard> = emptyList(),
    val peopleResults: List<Person> = emptyList(),
    val collectionResults: List<MovieCollection> = emptyList(),
    val unavailableSections: List<String> = emptyList(),
    val error: String? = null,
) {
    val hasResults: Boolean
        get() = mediaResults.isNotEmpty() || peopleResults.isNotEmpty() || collectionResults.isNotEmpty()

    val unavailableMessage: String?
        get() = unavailableSections.takeIf { it.isNotEmpty() }
            ?.joinToString(prefix = "部分结果暂不可用：", separator = "、")
}

@HiltViewModel
class SearchViewModel @Inject constructor(
    private val repository: NowenRepository,
    private val socialRepository: SocialCatalogRepository,
    val store: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(SearchUiState())
    val state: StateFlow<SearchUiState> = _state
    private var searchJob: Job? = null

    fun query(value: String) {
        _state.update { it.copy(query = value, error = null) }
        searchJob?.cancel()
        if (value.isBlank()) {
            _state.value = SearchUiState(query = value)
            return
        }
        searchJob = viewModelScope.launch {
            delay(280)
            val keyword = value.trim()
            _state.update { it.copy(loading = true, error = null, unavailableSections = emptyList()) }

            val mediaDeferred = async { repository.search(keyword) }
            val peopleDeferred = async { socialRepository.searchPeople(keyword) }
            val collectionsDeferred = async { socialRepository.searchCollections(keyword) }

            val media = mediaDeferred.await()
            val people = peopleDeferred.await()
            val collections = collectionsDeferred.await()
            val unavailable = buildList {
                if (media.isFailure) add("影视")
                if (people.isFailure) add("人物")
                if (collections.isFailure) add("合集")
            }

            if (unavailable.size == 3) {
                val error = media.exceptionOrNull()
                    ?: people.exceptionOrNull()
                    ?: collections.exceptionOrNull()
                _state.update {
                    it.copy(
                        loading = false,
                        mediaResults = emptyList(),
                        peopleResults = emptyList(),
                        collectionResults = emptyList(),
                        unavailableSections = emptyList(),
                        error = error?.message ?: "搜索失败",
                    )
                }
            } else {
                _state.update {
                    it.copy(
                        loading = false,
                        mediaResults = media.getOrDefault(emptyList()),
                        peopleResults = people.getOrDefault(emptyList()),
                        collectionResults = collections.getOrDefault(emptyList()),
                        unavailableSections = unavailable,
                        error = null,
                    )
                }
            }
        }
    }
}

@Composable
fun SearchScreen(
    modifier: Modifier = Modifier,
    initialQuery: String = "",
    onMediaClick: (MediaCard) -> Unit,
    onPersonClick: (String) -> Unit,
    onCollectionClick: (String) -> Unit,
    viewModel: SearchViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.store.snapshot.collectAsState()
    var selectedKinds by rememberSaveable { mutableStateOf(SearchMediaKind.entries.toSet()) }

    LaunchedEffect(initialQuery) {
        if (initialQuery.isNotBlank() && state.query != initialQuery) {
            viewModel.query(initialQuery)
        }
    }
    LaunchedEffect(state.query) {
        selectedKinds = SearchMediaKind.entries.toSet()
    }

    HillsScreen(modifier) { topPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(topPadding)
                .statusBarsPadding()
                .padding(horizontal = 20.dp, vertical = 12.dp),
        ) {
            HillsTextField(
                value = state.query,
                onValueChange = viewModel::query,
                placeholder = "输入搜索内容",
            )
            if (state.query.isNotBlank()) {
                val availableKinds = state.mediaResults.mapNotNull { media ->
                    when (media.type.lowercase()) {
                        "movie" -> SearchMediaKind.Movie
                        "series" -> SearchMediaKind.Series
                        "episode" -> SearchMediaKind.Episode
                        else -> null
                    }
                }.toSet()
                if (!state.loading && availableKinds.isNotEmpty()) {
                    Spacer(Modifier.height(12.dp))
                    SearchMediaKindRail(
                        availableKinds = availableKinds,
                        selectedKinds = selectedKinds,
                        onToggle = { kind ->
                            selectedKinds = selectedKinds.toMutableSet().apply {
                                if (!add(kind)) remove(kind)
                            }
                        },
                    )
                    Spacer(Modifier.height(16.dp))
                } else {
                    Spacer(Modifier.height(16.dp))
                }
                val effectiveKinds = selectedKinds.intersect(availableKinds)
                val filteredMedia = state.mediaResults.filter { media ->
                    when (media.type.lowercase()) {
                        "movie" -> SearchMediaKind.Movie in effectiveKinds
                        "series" -> SearchMediaKind.Series in effectiveKinds
                        "episode" -> SearchMediaKind.Episode in effectiveKinds
                        else -> false
                    }
                }
                when {
                    state.loading -> HillsState("正在搜索", "正在从当前服务器检索内容")
                    state.error != null -> HillsState("搜索失败", state.error!!)
                    filteredMedia.isEmpty() && state.peopleResults.isEmpty() && state.collectionResults.isEmpty() ->
                        HillsState("没有找到结果", "换一个关键词试试。")
                    else -> {
                        LazyColumn(
                            modifier = Modifier.weight(1f),
                            verticalArrangement = Arrangement.spacedBy(14.dp),
                            contentPadding = PaddingValues(bottom = 24.dp),
                        ) {
                            state.unavailableMessage?.let { warning ->
                                item { Text(warning, color = MaterialTheme.colorScheme.onSurfaceVariant) }
                            }
                            if (filteredMedia.isNotEmpty()) {
                                item { SearchSectionHeader("影视", filteredMedia.size) }
                                items(filteredMedia, key = { "media-${it.resolvedId}" }) { media ->
                                    SearchMediaRow(
                                        media = media,
                                        imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                                        onClick = { onMediaClick(media) },
                                    )
                                }
                            }
                            if (state.peopleResults.isNotEmpty()) {
                                item { SearchSectionHeader("人物", state.peopleResults.size) }
                                items(state.peopleResults, key = { "person-${it.id}" }) { person ->
                                    SearchPersonRow(
                                        person = person,
                                        imageUrl = personProfileUrl(session.activeServer?.baseUrl, person.id),
                                        onClick = { onPersonClick(person.id) },
                                    )
                                }
                            }
                            if (state.collectionResults.isNotEmpty()) {
                                item { SearchSectionHeader("电影合集", state.collectionResults.size) }
                                items(state.collectionResults, key = { "collection-${it.id}" }) { collection ->
                                    SearchCollectionRow(
                                        collection = collection,
                                        imageUrl = collectionPosterUrl(session.activeServer?.baseUrl, collection.id),
                                        onClick = { onCollectionClick(collection.id) },
                                    )
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun SearchMediaKindRail(
    availableKinds: Set<SearchMediaKind>,
    selectedKinds: Set<SearchMediaKind>,
    onToggle: (SearchMediaKind) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(androidx.compose.foundation.rememberScrollState()),
        horizontalArrangement = Arrangement.spacedBy(22.dp),
    ) {
        SearchMediaKind.entries.filter { it in availableKinds }.forEach { kind ->
            val selected = kind in selectedKinds
            Text(
                kind.label,
                color = if (selected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = if (selected) FontWeight.SemiBold else FontWeight.Normal,
                modifier = Modifier
                    .clip(RoundedCornerShape(8.dp))
                    .clickable { onToggle(kind) }
                    .padding(horizontal = 2.dp, vertical = 6.dp),
            )
        }
    }
}

@Composable
private fun SearchSectionHeader(title: String, count: Int) {
    Row(
        modifier = Modifier.fillMaxWidth(),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.SpaceBetween,
    ) {
        Text(title, style = MaterialTheme.typography.titleLarge)
        Text("$count 项", color = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun SearchMediaRow(
    media: MediaCard,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .clickable(onClick = onClick)
            .padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        AsyncImage(
            model = imageUrl,
            contentDescription = media.displayTitle,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .width(118.dp)
                .aspectRatio(16f / 9f)
                .clip(RoundedCornerShape(10.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
        )
        Spacer(Modifier.width(14.dp))
        Column(Modifier.weight(1f)) {
            Text(media.displayTitle, style = MaterialTheme.typography.titleMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
            val subtitle = listOfNotNull(
                media.year?.toString(),
                media.episodeTitle.takeIf { it.isNotBlank() },
                media.type.takeIf { it.isNotBlank() }?.let { if (it.equals("series", true)) "剧集" else null },
            ).joinToString(" · ")
            if (subtitle.isNotBlank()) {
                Spacer(Modifier.height(4.dp))
                Text(subtitle, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        Icon(Icons.Default.ChevronRight, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun SearchPersonRow(
    person: Person,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .clickable(onClick = onClick)
            .padding(vertical = 8.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        AsyncImage(
            model = imageUrl,
            contentDescription = person.name,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .size(56.dp)
                .clip(RoundedCornerShape(28.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
        )
        Spacer(Modifier.width(14.dp))
        Column(Modifier.weight(1f)) {
            Text(person.name, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
            person.originalName.takeIf { it.isNotBlank() }?.let { originalName ->
                Text(originalName, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
            }
        }
        Icon(Icons.Default.ChevronRight, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun SearchCollectionRow(
    collection: MovieCollection,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(10.dp))
            .clickable(onClick = onClick)
            .padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        AsyncImage(
            model = imageUrl,
            contentDescription = collection.name,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .width(72.dp)
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(10.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
        )
        Spacer(Modifier.width(14.dp))
        Column(Modifier.weight(1f)) {
            Text(collection.name, style = MaterialTheme.typography.titleMedium, maxLines = 2, overflow = TextOverflow.Ellipsis)
            val subtitle = listOfNotNull(
                collection.yearRange.takeIf { it.isNotBlank() },
                collection.mediaCount.takeIf { it > 0 }?.let { "$it 部" },
            ).joinToString(" · ")
            if (subtitle.isNotBlank()) {
                Spacer(Modifier.height(4.dp))
                Text(subtitle, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
        Icon(Icons.Default.ChevronRight, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}

@Composable
private fun SearchPersonCard(
    person: Person,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    Column(
        modifier = Modifier
            .width(92.dp)
            .clickable(onClick = onClick),
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        AsyncImage(
            model = imageUrl,
            contentDescription = person.name,
            contentScale = ContentScale.Crop,
            modifier = Modifier
                .size(76.dp)
                .clip(RoundedCornerShape(38.dp))
                .background(MaterialTheme.colorScheme.surfaceVariant),
        )
        Spacer(Modifier.height(8.dp))
        Text(person.name, style = MaterialTheme.typography.titleMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
        Text(person.originalName.ifBlank { "演职人员" }, style = MaterialTheme.typography.bodyMedium, color = MaterialTheme.colorScheme.onSurfaceVariant, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
fun ProfileScreen(
    modifier: Modifier = Modifier,
    sessionStore: ServerSessionStore,
    onFavorites: () -> Unit,
    onHistory: () -> Unit,
    onDownloads: () -> Unit,
    onCollections: () -> Unit,
    onSettings: () -> Unit,
    onLogout: () -> Unit,
) {
    val session by sessionStore.snapshot.collectAsState()

    HillsScreen(modifier, top = { HillsTopBar(title = "我的") }) { topPadding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(topPadding)
                .padding(horizontal = 20.dp),
        ) {
        Spacer(Modifier.height(12.dp))
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Default.Person, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(42.dp))
            Spacer(Modifier.width(14.dp))
            Column {
                Text(
                    session.user?.nickname?.ifBlank { session.user?.username } ?: "用户",
                    style = MaterialTheme.typography.titleLarge,
                )
                Text(
                    session.activeServer?.name ?: "未连接服务器",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }
        }
        Spacer(Modifier.height(28.dp))
        Text("媒体", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.primary)
        ProfileDestinationRow(
            icon = Icons.Default.Favorite,
            title = "我的收藏",
            subtitle = "喜欢的电影和剧集",
            onClick = onFavorites,
        )
        ProfileDestinationRow(
            icon = Icons.Default.History,
            title = "观看历史",
            subtitle = "继续播放或管理记录",
            onClick = onHistory,
        )
        ProfileDestinationRow(
            icon = Icons.Default.CloudDownload,
            title = "下载",
            subtitle = "管理离线媒体与空间",
            onClick = onDownloads,
        )
        ProfileDestinationRow(
            icon = Icons.Default.Collections,
            title = "系列合集",
            subtitle = "按电影系列浏览内容",
            onClick = onCollections,
        )
        Spacer(Modifier.height(24.dp))
        Text("客户端", style = MaterialTheme.typography.titleLarge, color = MaterialTheme.colorScheme.primary)
        ProfileDestinationRow(
            icon = Icons.Default.Settings,
            title = "设置",
            subtitle = "服务器、播放、下载与通知",
            onClick = onSettings,
        )
        Spacer(Modifier.height(18.dp))
        HillsSecondaryAction(
            label = "退出当前服务器账号",
            icon = Icons.Default.Logout,
            onClick = onLogout,
            modifier = Modifier.fillMaxWidth(),
        )
        }
    }
}

@Composable
private fun ProfileDestinationRow(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
    subtitle: String,
    onClick: () -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.onSurface)
        Spacer(Modifier.width(18.dp))
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleLarge)
            Spacer(Modifier.height(2.dp))
            Text(subtitle, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodyMedium)
        }
        Icon(Icons.Default.ChevronRight, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant)
    }
}
