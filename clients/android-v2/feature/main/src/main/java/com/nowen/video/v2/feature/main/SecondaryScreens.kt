package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Logout
import androidx.compose.material.icons.filled.Search
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.NowenRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.*
import com.nowen.video.v2.core.model.MediaCard
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

@Composable
fun LibraryScreen(modifier: Modifier = Modifier) {
    NowenPage(modifier, PaddingValues(horizontal = 20.dp, vertical = 20.dp)) {
        Text("媒体库", style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(8.dp))
        Text(
            "V2 已建立独立媒体库入口。后续在此接入 Paging 3、排序、筛选和双栏平板布局。",
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
        Spacer(Modifier.height(24.dp))
        MessagePanel(
            title = "清晰、克制、可扩展",
            message = "媒体库不会复制 Web 管理后台，而是围绕浏览、筛选和播放构建。",
        )
    }
}

data class SearchUiState(
    val query: String = "",
    val loading: Boolean = false,
    val results: List<MediaCard> = emptyList(),
    val error: String? = null,
)

@HiltViewModel
class SearchViewModel @Inject constructor(
    private val repository: NowenRepository,
    val store: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(SearchUiState())
    val state: StateFlow<SearchUiState> = _state
    private var searchJob: Job? = null

    fun query(value: String) {
        _state.update { it.copy(query = value, error = null) }
        searchJob?.cancel()
        if (value.isBlank()) {
            _state.update { it.copy(results = emptyList(), loading = false) }
            return
        }
        searchJob = viewModelScope.launch {
            delay(280)
            _state.update { it.copy(loading = true) }
            repository.search(value)
                .onSuccess { results -> _state.update { it.copy(loading = false, results = results) } }
                .onFailure { error ->
                    _state.update { it.copy(loading = false, error = error.message ?: "搜索失败") }
                }
        }
    }
}

@Composable
fun SearchScreen(
    modifier: Modifier = Modifier,
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
            state.results.isEmpty() -> MessagePanel("没有找到结果", "换一个关键词试试。")
            else -> LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                items(state.results, key = { it.resolvedId }) { media ->
                    MediaPosterCard(
                        media.displayTitle,
                        media.year?.toString(),
                        resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                        media.normalizedProgress,
                        onClick = {},
                    )
                }
            }
        }
    }
}

@Composable
fun DownloadsScreen(modifier: Modifier = Modifier) {
    NowenPage(modifier, PaddingValues(horizontal = 20.dp, vertical = 20.dp)) {
        Text("下载", style = MaterialTheme.typography.headlineLarge)
        Spacer(Modifier.height(18.dp))
        ElevatedPanel(Modifier.fillMaxWidth()) {
            Icon(Icons.Default.CloudDownload, null, tint = MaterialTheme.colorScheme.primary)
            Spacer(Modifier.height(12.dp))
            Text("离线播放基础已预留", style = MaterialTheme.typography.titleLarge)
            Spacer(Modifier.height(8.dp))
            Text(
                "下一阶段使用 WorkManager 和 Media3 下载服务，按服务器隔离任务并支持断点续传。",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
fun ProfileScreen(
    modifier: Modifier = Modifier,
    sessionStore: ServerSessionStore,
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
        Spacer(Modifier.height(18.dp))
        OutlinedButton(onClick = onLogout, modifier = Modifier.fillMaxWidth()) {
            Icon(Icons.Default.Logout, null)
            Spacer(Modifier.width(8.dp))
            Text("退出当前服务器账号")
        }
    }
}
