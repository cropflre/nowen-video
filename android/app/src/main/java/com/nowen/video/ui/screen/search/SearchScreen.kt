package com.nowen.video.ui.screen.search

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.Series
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 搜索页面 — 支持电影和剧集混合搜索
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SearchScreen(
    initialQuery: String = "",
    onMediaClick: (String) -> Unit,
    onSeriesClick: (String) -> Unit,
    onBack: () -> Unit,
    viewModel: SearchViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    var query by remember { mutableStateOf(initialQuery) }

    // 如果有初始搜索关键词，自动触发搜索
    LaunchedEffect(initialQuery) {
        if (initialQuery.isNotBlank()) {
            viewModel.search(initialQuery)
        }
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = {
                    OutlinedTextField(
                        value = query,
                        onValueChange = {
                            query = it
                            viewModel.search(it)
                        },
                        placeholder = { Text("搜索电影、剧集...") },
                        singleLine = true,
                        modifier = Modifier.fillMaxWidth(),
                        keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                        keyboardActions = KeyboardActions(
                            onSearch = { viewModel.search(query) }
                        ),
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = MaterialTheme.colorScheme.primary,
                            unfocusedBorderColor = MaterialTheme.colorScheme.outline.copy(alpha = 0.5f)
                        )
                    )
                },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { padding ->
        if (uiState.loading) {
            Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                CircularProgressIndicator()
            }
        } else if (uiState.media.isEmpty() && uiState.series.isEmpty() && query.isNotBlank()) {
            Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Icon(
                        Icons.Default.SearchOff,
                        contentDescription = null,
                        modifier = Modifier.size(48.dp),
                        tint = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(
                        "未找到相关内容",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                // 电影结果
                if (uiState.media.isNotEmpty()) {
                    item {
                        Text(
                            "电影",
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.padding(vertical = 4.dp)
                        )
                    }
                    items(uiState.media) { media ->
                        SearchResultItem(
                            title = media.title,
                            subtitle = buildString {
                                if (media.year > 0) append("${media.year}")
                                if (media.genres.isNotBlank()) append(" · ${media.genres.split(",").first()}")
                            },
                            posterUrl = "${uiState.serverUrl}/api/media/${media.id}/poster?token=${uiState.token}",
                            rating = media.rating,
                            onClick = { onMediaClick(media.id) }
                        )
                    }
                }

                // 剧集结果
                if (uiState.series.isNotEmpty()) {
                    item {
                        Text(
                            "剧集",
                            style = MaterialTheme.typography.titleMedium,
                            modifier = Modifier.padding(vertical = 4.dp)
                        )
                    }
                    items(uiState.series) { series ->
                        SearchResultItem(
                            title = series.title,
                            subtitle = buildString {
                                if (series.year > 0) append("${series.year}")
                                append(" · ${series.seasonCount} 季")
                            },
                            posterUrl = "${uiState.serverUrl}/api/series/${series.id}/poster?token=${uiState.token}",
                            rating = series.rating,
                            onClick = { onSeriesClick(series.id) }
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun SearchResultItem(
    title: String,
    subtitle: String,
    posterUrl: String,
    rating: Double,
    onClick: () -> Unit
) {
    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp)
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            AsyncImage(
                model = posterUrl,
                contentDescription = title,
                modifier = Modifier
                    .width(60.dp)
                    .height(90.dp)
                    .clip(RoundedCornerShape(8.dp)),
                contentScale = ContentScale.Crop
            )

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleSmall,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
                if (rating > 0) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.padding(top = 4.dp)
                    ) {
                        Icon(
                            Icons.Default.Star,
                            contentDescription = null,
                            modifier = Modifier.size(14.dp),
                            tint = MaterialTheme.colorScheme.tertiary
                        )
                        Text(
                            text = String.format(" %.1f", rating),
                            style = MaterialTheme.typography.labelSmall,
                            color = MaterialTheme.colorScheme.tertiary
                        )
                    }
                }
            }
        }
    }
}

// ==================== ViewModel ====================

data class SearchUiState(
    val loading: Boolean = false,
    val media: List<Media> = emptyList(),
    val series: List<Series> = emptyList(),
    val serverUrl: String = "",
    val token: String = ""
)

@HiltViewModel
class SearchViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(SearchUiState())
    val uiState = _uiState.asStateFlow()

    private var searchJob: Job? = null

    init {
        viewModelScope.launch {
            val serverUrl = tokenManager.getServerUrl() ?: ""
            val token = tokenManager.getToken() ?: ""
            _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)
        }
    }

    fun search(query: String) {
        searchJob?.cancel()
        if (query.isBlank()) {
            _uiState.value = _uiState.value.copy(media = emptyList(), series = emptyList())
            return
        }

        searchJob = viewModelScope.launch {
            delay(300) // 防抖
            _uiState.value = _uiState.value.copy(loading = true)

            mediaRepository.searchMixed(query).onSuccess { result ->
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    media = result.media,
                    series = result.series
                )
            }.onFailure {
                _uiState.value = _uiState.value.copy(loading = false)
            }
        }
    }
}
