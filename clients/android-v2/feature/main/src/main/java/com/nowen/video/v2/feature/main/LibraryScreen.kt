package com.nowen.video.v2.feature.main

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.MediaPosterCard
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaCard
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class LibraryUiState(
    val loading: Boolean = true,
    val items: List<MediaCard> = emptyList(),
    val error: String? = null,
)

@HiltViewModel
class LibraryViewModel @Inject constructor(
    private val repository: CatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(LibraryUiState())
    val state: StateFlow<LibraryUiState> = _state

    init {
        refresh()
    }

    fun refresh() {
        viewModelScope.launch {
            _state.update { it.copy(loading = true, error = null) }
            repository.media(size = 120)
                .onSuccess { page ->
                    _state.value = LibraryUiState(
                        loading = false,
                        items = page.data.filter { it.resolvedId.isNotBlank() },
                    )
                }
                .onFailure { error ->
                    _state.update {
                        it.copy(loading = false, error = error.message ?: "媒体库加载失败")
                    }
                }
        }
    }
}

@Composable
fun LibraryScreen(
    modifier: Modifier = Modifier,
    onMediaClick: (String) -> Unit,
    viewModel: LibraryViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()

    Box(modifier = modifier.fillMaxSize()) {
        when {
            state.loading && state.items.isEmpty() -> {
                CircularProgressIndicator(Modifier.align(Alignment.Center))
            }
            state.error != null && state.items.isEmpty() -> {
                MessagePanel(
                    title = "媒体库加载失败",
                    message = state.error!!,
                    actionLabel = "重试",
                    onAction = viewModel::refresh,
                    modifier = Modifier
                        .align(Alignment.Center)
                        .padding(20.dp),
                )
            }
            state.items.isEmpty() -> {
                MessagePanel(
                    title = "媒体库还是空的",
                    message = "请先在服务器端添加媒体库并完成扫描。",
                    modifier = Modifier
                        .align(Alignment.Center)
                        .padding(20.dp),
                )
            }
            else -> {
                LazyVerticalGrid(
                    columns = GridCells.Adaptive(minSize = 132.dp),
                    modifier = Modifier.fillMaxSize(),
                    contentPadding = PaddingValues(start = 20.dp, end = 20.dp, top = 72.dp, bottom = 24.dp),
                    horizontalArrangement = Arrangement.spacedBy(14.dp),
                    verticalArrangement = Arrangement.spacedBy(18.dp),
                ) {
                    items(state.items, key = { it.resolvedId }) { media ->
                        MediaPosterCard(
                            title = media.displayTitle,
                            subtitle = media.year?.toString(),
                            imageUrl = resolveImage(session.activeServer?.baseUrl, media.resolvedPoster),
                            progress = media.normalizedProgress,
                            onClick = { onMediaClick(media.resolvedId) },
                            modifier = Modifier.fillMaxSize(),
                        )
                    }
                }
            }
        }

        Text(
            text = "媒体库",
            style = MaterialTheme.typography.headlineMedium,
            modifier = Modifier
                .align(Alignment.TopStart)
                .padding(start = 20.dp, top = 20.dp),
        )
        IconButton(
            onClick = viewModel::refresh,
            enabled = !state.loading,
            modifier = Modifier
                .align(Alignment.TopEnd)
                .padding(end = 10.dp, top = 10.dp),
        ) {
            Icon(Icons.Default.Refresh, contentDescription = "刷新媒体库")
        }
    }
}
