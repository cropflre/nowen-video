package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Collections
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Favorite
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Logout
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
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
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.data.SocialCatalogRepository
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.MediaPosterCard
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.designsystem.NowenPage
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
        get() = unavailableSections.takeIf(List<String>::isNotEmpty)
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
    onMediaClick: (String) -> Unit,
    onPersonClick: (String) -> Unit,
    onCollectionClick: (String) -> Unit,
    viewModel: SearchViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.store.snapshot.collectAsState()

    NowenPage(modifier, PaddingValues(horizontal = 20.dp, vertical = 20.dp)) {
        Text("搜索", style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(16.dp))
        OutlinedTextField(
            value = state.query,
            onValueChange = viewModel::query,
            modifier = Modifier.fillMaxWidth(),
            leadingIcon = { Icon(Icons.Default.Search, null) },
            placeholder = { Text("电影、剧集、演员或合集") },
            singleLine = true,
        )
        Spacer(Modifier.height(18.dp))
        when {
            state.loading -> LinearProgressIndicator(Modifier.fillMaxWidth())
            state.error != null -> MessagePanel("搜索失败", state.error!!)
            state.query.isBlank() -> MessagePanel("开始探索", "输入关键词即可搜索当前服务器。")
            !state.hasResults -> MessagePanel("没有找到结果", "换一个关键词试试。")
            else -> {
                state.unavailableMessage?.let { warning ->
                    Text(
                        warning,
                        style = MaterialTheme.typography.bodyMedium,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    Spacer(Modifier.height(12.dp))
                }
                LazyColumn(
                    modifier = Modifier.weight(1f),
                    verticalArrangement = Arrangement.spacedBy(14.dp),
                    contentPadding = PaddingValues(bottom = 20.dp),
                ) {
                    if (state.mediaResults.isNotEmpty()) {
                        item { SearchSectionHeader("影视", state.mediaResults.size) }
                        item {
                            LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                                items(state.mediaResults, key = { "media-${it.resolvedId}" }) { media ->
                                    MediaPosterCard(
                                        title = media.displayTitle,
                                        subtitle = media.year?.toString(),
                                        imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                                        progress = media.normalizedProgress,
                                        onClick = { onMediaClick(media.resolvedId) },
                                    )
                                }
                            }
                        }
                    }

                    if (state.peopleResults.isNotEmpty()) {
                        item { SearchSectionHeader("人物", state.peopleResults.size) }
                        item {
                            LazyRow(horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                items(state.peopleResults, key = { "person-${it.id}" }) { person ->
                                    SearchPersonCard(
                                        person = person,
                                        imageUrl = personProfileUrl(session.activeServer?.baseUrl, person.id),
                                        onClick = { onPersonClick(person.id) },
                                    )
                                }
                            }
                        }
                    }

                    if (state.collectionResults.isNotEmpty()) {
                        item { SearchSectionHeader("电影合集", state.collectionResults.size) }
                        item {
                            LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                                items(state.collectionResults, key = { "collection-${it.id}" }) { collection ->
                                    MediaPosterCard(
                                        title = collection.name,
                                        subtitle = listOfNotNull(
                                            collection.yearRange.takeIf(String::isNotBlank),
                                            collection.mediaCount.takeIf { it > 0 }?.let { "$it 部" },
                                        ).joinToString(" · ").ifBlank { "电影合集" },
                                        imageUrl = collectionPosterUrl(session.activeServer?.baseUrl, collection.id),
                                        progress = 0f,
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
private fun SearchPersonCard(
    person: Person,
    imageUrl: String?,
    onClick: () -> Unit,
) {
    ElevatedPanel(
        Modifier
            .width(220.dp)
            .clickable(onClick = onClick),
    ) {
        Row(verticalAlignment = Alignment.CenterVertically) {
            AsyncImage(
                model = imageUrl,
                contentDescription = person.name,
                contentScale = ContentScale.Crop,
                modifier = Modifier
                    .size(64.dp)
                    .clip(MaterialTheme.shapes.large)
                    .background(MaterialTheme.colorScheme.surfaceVariant),
            )
            Spacer(Modifier.width(12.dp))
            Column(Modifier.weight(1f)) {
                Text(
                    person.name,
                    style = MaterialTheme.typography.titleMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    person.originalName.ifBlank { "演职人员" },
                    style = MaterialTheme.typography.bodyMedium,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }
            Icon(Icons.Default.Person, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        }
    }
}

@Composable
fun ProfileScreen(
    modifier: Modifier = Modifier,
    sessionStore: ServerSessionStore,
    onFavorites: () -> Unit,
    onHistory: () -> Unit,
    onCollections: () -> Unit,
    onLogout: () -> Unit,
) {
    val session by sessionStore.snapshot.collectAsState()

    NowenPage(modifier, PaddingValues(horizontal = 20.dp, vertical = 20.dp)) {
        Text("我的", style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(20.dp))
        ElevatedPanel(Modifier.fillMaxWidth()) {
            Text(
                session.user?.nickname?.ifBlank { session.user?.username } ?: "用户",
                style = MaterialTheme.typography.titleLarge,
            )
            Spacer(Modifier.height(4.dp))
            Text(
                "${session.user?.role ?: "user"} · ${session.activeServer?.name ?: "Nowen Video"}",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
        Spacer(Modifier.height(14.dp))
        ElevatedPanel(Modifier.fillMaxWidth()) {
            Row {
                Icon(Icons.Default.Dns, null, tint = MaterialTheme.colorScheme.primary)
                Spacer(Modifier.width(12.dp))
                Column {
                    Text("当前服务器", style = MaterialTheme.typography.titleMedium)
                    Text(
                        session.activeServer?.baseUrl ?: "未连接",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }
        Spacer(Modifier.height(22.dp))
        Text("我的内容", style = MaterialTheme.typography.titleLarge)
        Spacer(Modifier.height(10.dp))
        ProfileDestinationRow(
            icon = Icons.Default.Favorite,
            title = "我的收藏",
            subtitle = "集中查看喜欢的电影和单集",
            onClick = onFavorites,
        )
        Spacer(Modifier.height(10.dp))
        ProfileDestinationRow(
            icon = Icons.Default.History,
            title = "观看历史",
            subtitle = "继续播放或管理已看记录",
            onClick = onHistory,
        )
        Spacer(Modifier.height(10.dp))
        ProfileDestinationRow(
            icon = Icons.Default.Collections,
            title = "系列合集",
            subtitle = "按电影系列浏览馆藏内容",
            onClick = onCollections,
        )
        Spacer(Modifier.height(22.dp))
        OutlinedButton(onClick = onLogout, modifier = Modifier.fillMaxWidth()) {
            Icon(Icons.Default.Logout, null)
            Spacer(Modifier.width(8.dp))
            Text("退出当前服务器账号")
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
    ElevatedPanel(
        Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
    ) {
        Row {
            Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
            Spacer(Modifier.width(14.dp))
            Column {
                Text(title, style = MaterialTheme.typography.titleMedium)
                Spacer(Modifier.height(3.dp))
                Text(subtitle, color = MaterialTheme.colorScheme.onSurfaceVariant)
            }
        }
    }
}
