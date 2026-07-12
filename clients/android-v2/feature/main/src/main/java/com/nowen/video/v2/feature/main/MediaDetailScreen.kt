package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaDetail
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch

data class MediaDetailUiState(
    val loading: Boolean = true,
    val media: MediaDetail? = null,
    val error: String? = null,
)

@HiltViewModel
class MediaDetailViewModel @Inject constructor(
    private val repository: CatalogRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(MediaDetailUiState())
    val state: StateFlow<MediaDetailUiState> = _state
    private var loadedId: String? = null

    fun load(id: String) {
        if (loadedId == id && _state.value.media != null) return
        loadedId = id
        viewModelScope.launch {
            _state.value = MediaDetailUiState(loading = true)
            repository.detail(id)
                .onSuccess { media -> _state.value = MediaDetailUiState(loading = false, media = media) }
                .onFailure { error ->
                    _state.update { it.copy(loading = false, error = error.message ?: "详情加载失败") }
                }
        }
    }
}

@Composable
fun MediaDetailScreen(
    mediaId: String,
    onBack: () -> Unit,
    onPlay: (String) -> Unit,
    viewModel: MediaDetailViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    LaunchedEffect(mediaId) { viewModel.load(mediaId) }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(MaterialTheme.colorScheme.background),
    ) {
        when {
            state.loading -> CircularProgressIndicator(Modifier.align(androidx.compose.ui.Alignment.Center))
            state.error != null -> MessagePanel(
                title = "无法打开详情",
                message = state.error!!,
                actionLabel = "返回",
                onAction = onBack,
                modifier = Modifier
                    .align(androidx.compose.ui.Alignment.Center)
                    .padding(20.dp),
            )
            state.media != null -> {
                val media = state.media!!
                Column(
                    modifier = Modifier
                        .fillMaxSize()
                        .verticalScroll(rememberScrollState()),
                ) {
                    Box(Modifier.fillMaxWidth().height(280.dp)) {
                        AsyncImage(
                            model = resolveImage(session.activeServer?.baseUrl, media.backdropPath),
                            contentDescription = media.displayTitle,
                            contentScale = ContentScale.Crop,
                            modifier = Modifier.fillMaxSize(),
                        )
                        Box(
                            Modifier
                                .fillMaxSize()
                                .background(
                                    Brush.verticalGradient(
                                        listOf(Color.Transparent, MaterialTheme.colorScheme.background),
                                    ),
                                ),
                        )
                    }
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 20.dp)
                            .offset(y = (-52).dp),
                    ) {
                        AsyncImage(
                            model = resolveImage(session.activeServer?.baseUrl, media.posterPath),
                            contentDescription = media.displayTitle,
                            contentScale = ContentScale.Crop,
                            modifier = Modifier
                                .width(116.dp)
                                .aspectRatio(2f / 3f)
                                .clip(MaterialTheme.shapes.large)
                                .background(MaterialTheme.colorScheme.surfaceVariant),
                        )
                        Spacer(Modifier.width(18.dp))
                        Column(Modifier.weight(1f).padding(top = 42.dp)) {
                            Text(media.displayTitle, style = MaterialTheme.typography.headlineMedium)
                            Spacer(Modifier.height(8.dp))
                            Text(
                                listOfNotNull(
                                    media.year.takeIf { it > 0 }?.toString(),
                                    media.runtime.takeIf { it > 0 }?.let { "$it 分钟" },
                                    media.resolution.takeIf { it.isNotBlank() },
                                    media.rating.takeIf { it > 0 }?.let { "★ %.1f".format(it) },
                                ).joinToString(" · "),
                                color = MaterialTheme.colorScheme.onSurfaceVariant,
                            )
                        }
                    }
                    Column(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = 20.dp)
                            .offset(y = (-32).dp),
                    ) {
                        Button(
                            onClick = { onPlay(media.id) },
                            modifier = Modifier.fillMaxWidth(),
                        ) {
                            Icon(Icons.Default.PlayArrow, null)
                            Spacer(Modifier.width(8.dp))
                            Text("立即播放")
                        }
                        if (media.genres.isNotBlank()) {
                            Spacer(Modifier.height(18.dp))
                            Text(media.genres, color = MaterialTheme.colorScheme.primary)
                        }
                        Spacer(Modifier.height(16.dp))
                        Text("简介", style = MaterialTheme.typography.titleLarge)
                        Spacer(Modifier.height(8.dp))
                        Text(
                            media.overview.ifBlank { "暂无简介" },
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                        )
                        Spacer(Modifier.height(36.dp))
                    }
                }
            }
        }

        IconButton(
            onClick = onBack,
            modifier = Modifier
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(8.dp)
                .background(MaterialTheme.colorScheme.surface.copy(alpha = 0.72f), MaterialTheme.shapes.large),
        ) {
            Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
        }
    }
}
