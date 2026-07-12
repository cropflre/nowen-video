package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
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
import com.nowen.video.v2.core.model.HomeContent
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class HomeUiState(
    val loading: Boolean = true,
    val refreshing: Boolean = false,
    val content: HomeContent = HomeContent(),
    val error: String? = null,
)

@HiltViewModel
class HomeViewModel @Inject constructor(
    private val repository: NowenRepository,
    val store: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(HomeUiState())
    val state: StateFlow<HomeUiState> = _state

    init { load(false) }
    fun refresh() = load(true)

    private fun load(refresh: Boolean) {
        viewModelScope.launch {
            _state.update { it.copy(loading = !refresh, refreshing = refresh, error = null) }
            repository.loadHome()
                .onSuccess { content -> _state.value = HomeUiState(loading = false, content = content) }
                .onFailure { error ->
                    _state.update {
                        it.copy(loading = false, refreshing = false, error = error.message ?: "首页加载失败")
                    }
                }
        }
    }
}

@Composable
fun HomeScreen(
    modifier: Modifier = Modifier,
    viewModel: HomeViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.store.snapshot.collectAsState()

    LazyColumn(
        modifier = modifier.fillMaxSize(),
        contentPadding = PaddingValues(horizontal = 20.dp, vertical = 20.dp),
        verticalArrangement = Arrangement.spacedBy(26.dp),
    ) {
        item {
            Row(Modifier.fillMaxWidth()) {
                Column(Modifier.weight(1f)) {
                    Text(
                        "晚上好，${session.user?.nickname?.ifBlank { session.user?.username } ?: "探索者"}",
                        style = MaterialTheme.typography.headlineMedium,
                    )
                    Text(
                        session.activeServer?.name ?: "Nowen Video",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
                IconButton(onClick = viewModel::refresh, enabled = !state.refreshing) {
                    Icon(Icons.Default.Refresh, "刷新")
                }
            }
        }

        if (state.loading) {
            item {
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    LinearProgressIndicator(Modifier.fillMaxWidth())
                    Spacer(Modifier.height(14.dp))
                    Text("正在同步你的媒体空间")
                }
            }
        }

        state.error?.let { message ->
            item {
                MessagePanel("暂时无法加载", message, "重试", viewModel::refresh)
            }
        }

        if (state.content.continueWatching.isNotEmpty()) {
            item { SectionTitle("继续观看", "从上次离开的地方继续") }
            item {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                    items(state.content.continueWatching, key = { it.resolvedId }) { media ->
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

        if (state.content.libraries.isNotEmpty()) {
            item { SectionTitle("我的媒体库", "${state.content.libraries.size} 个资料库") }
            items(state.content.libraries, key = { it.id }) { library ->
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    Text(library.name, style = MaterialTheme.typography.titleLarge)
                    Spacer(Modifier.height(4.dp))
                    Text(
                        listOfNotNull(
                            library.type.takeIf { it.isNotBlank() },
                            library.mediaCount.takeIf { it > 0 }?.let { "$it 项" },
                        ).joinToString(" · ").ifBlank { "媒体库" },
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }
        }

        if (state.content.recent.isNotEmpty()) {
            item { SectionTitle("最近添加", "刚刚进入你的媒体空间") }
            item {
                LazyRow(horizontalArrangement = Arrangement.spacedBy(14.dp)) {
                    items(state.content.recent, key = { it.resolvedId }) { media ->
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

        item { Spacer(Modifier.height(12.dp)) }
    }
}

internal fun resolveImage(baseUrl: String?, path: String?): String? {
    if (path.isNullOrBlank()) return null
    if (path.startsWith("http://") || path.startsWith("https://")) return path
    return baseUrl?.trimEnd('/') + "/" + path.trimStart('/')
}
