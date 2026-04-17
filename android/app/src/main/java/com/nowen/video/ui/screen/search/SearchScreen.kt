package com.nowen.video.ui.screen.search

import androidx.compose.foundation.background
import androidx.compose.foundation.border
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
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.Series
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.Job
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 搜索页面 — 赛博朋克风格：深空背景 + 霓虹搜索框 + 玻璃拟态结果卡片
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
    val colorScheme = MaterialTheme.colorScheme

    LaunchedEffect(initialQuery) {
        if (initialQuery.isNotBlank()) {
            viewModel.search(initialQuery)
        }
    }

    Box(
        modifier = Modifier
            .fillMaxSize()
            .spaceBackground()
    ) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    title = {
                        // 赛博朋克搜索输入框
                        OutlinedTextField(
                            value = query,
                            onValueChange = {
                                query = it
                                viewModel.search(it)
                            },
                            placeholder = {
                                Text(
                                    "搜索电影、剧集...",
                                    color = colorScheme.outline,
                                    letterSpacing = 1.sp
                                )
                            },
                            singleLine = true,
                            modifier = Modifier.fillMaxWidth(),
                            keyboardOptions = KeyboardOptions(imeAction = ImeAction.Search),
                            keyboardActions = KeyboardActions(
                                onSearch = { viewModel.search(query) }
                            ),
                            colors = OutlinedTextFieldDefaults.colors(
                                focusedBorderColor = colorScheme.primary,
                                unfocusedBorderColor = colorScheme.primary.copy(alpha = 0.3f),
                                cursorColor = colorScheme.primary,
                                focusedTextColor = colorScheme.onSurface,
                                unfocusedTextColor = colorScheme.onSurface,
                                focusedContainerColor = colorScheme.surface.copy(alpha = 0.6f),
                                unfocusedContainerColor = colorScheme.surface.copy(alpha = 0.4f)
                            ),
                            shape = RoundedCornerShape(12.dp)
                        )
                    },
                    navigationIcon = {
                        IconButton(onClick = onBack) {
                            Icon(
                                Icons.AutoMirrored.Filled.ArrowBack,
                                contentDescription = "返回",
                                tint = colorScheme.primary
                            )
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(
                        containerColor = colorScheme.scrim.copy(alpha = 0.85f)
                    )
                )
            }
        ) { padding ->
            if (uiState.loading) {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        CircularProgressIndicator(
                            color = colorScheme.primary,
                            trackColor = colorScheme.surfaceContainerHigh
                        )
                        Spacer(modifier = Modifier.height(12.dp))
                        Text(
                            "搜索中...",
                            color = colorScheme.primary.copy(alpha = 0.7f),
                            style = MaterialTheme.typography.bodySmall,
                            letterSpacing = 2.sp
                        )
                    }
                }
            } else if (uiState.media.isEmpty() && uiState.series.isEmpty() && query.isNotBlank()) {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(
                            Icons.Default.SearchOff,
                            contentDescription = null,
                            modifier = Modifier.size(48.dp),
                            tint = colorScheme.secondary.copy(alpha = 0.5f)
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            "未找到相关内容",
                            style = MaterialTheme.typography.bodyLarge,
                            color = colorScheme.onSurfaceVariant
                        )
                    }
                }
            } else {
                LazyColumn(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding),
                    contentPadding = PaddingValues(16.dp),
                    verticalArrangement = Arrangement.spacedBy(10.dp)
                ) {
                    // 电影结果
                    if (uiState.media.isNotEmpty()) {
                        item {
                            CyberResultSectionTitle("电影", "${uiState.media.size} 个结果")
                        }
                        items(uiState.media) { media ->
                            CyberSearchResultItem(
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
                            CyberResultSectionTitle("剧集", "${uiState.series.size} 个结果")
                        }
                        items(uiState.series) { series ->
                            CyberSearchResultItem(
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
}

@Composable
private fun CyberResultSectionTitle(title: String, count: String) {
    val colorScheme = MaterialTheme.colorScheme
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        Text(
            title,
            style = MaterialTheme.typography.titleMedium.copy(
                fontWeight = FontWeight.SemiBold,
                letterSpacing = 1.sp
            ),
            color = colorScheme.primary
        )
        Spacer(modifier = Modifier.width(8.dp))
        Text(
            count,
            style = MaterialTheme.typography.labelSmall,
            color = colorScheme.outline
        )
    }
}

@Composable
private fun CyberSearchResultItem(
    title: String,
    subtitle: String,
    posterUrl: String,
    rating: Double,
    onClick: () -> Unit
) {
    val colorScheme = MaterialTheme.colorScheme
    Box(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(14.dp))
            .glassMorphism(cornerRadius = 14.dp)
            .clickable(onClick = onClick)
    ) {
        Row(
            modifier = Modifier.padding(12.dp),
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // 海报
            Box(
                modifier = Modifier
                    .width(60.dp)
                    .height(90.dp)
                    .clip(RoundedCornerShape(8.dp))
                    .border(
                        1.dp,
                        colorScheme.primary.copy(alpha = 0.15f),
                        RoundedCornerShape(8.dp)
                    )
            ) {
                AsyncImage(
                    model = posterUrl,
                    contentDescription = title,
                    modifier = Modifier.fillMaxSize(),
                    contentScale = ContentScale.Crop
                )
            }

            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.titleSmall,
                    color = colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Text(
                    text = subtitle,
                    style = MaterialTheme.typography.bodySmall,
                    color = colorScheme.outline
                )
                if (rating > 0) {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        modifier = Modifier.padding(top = 6.dp)
                    ) {
                        Icon(
                            Icons.Default.Star,
                            contentDescription = null,
                            modifier = Modifier.size(14.dp),
                            tint = AmberGold
                        )
                        Text(
                            text = String.format(" %.1f", rating),
                            style = MaterialTheme.typography.labelSmall.copy(
                                fontWeight = FontWeight.Bold
                            ),
                            color = AmberGold
                        )
                    }
                }
            }

            // 箭头指示
            Icon(
                Icons.Default.ChevronRight,
                contentDescription = null,
                modifier = Modifier
                    .size(20.dp)
                    .align(Alignment.CenterVertically),
                tint = colorScheme.primary.copy(alpha = 0.5f)
            )
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
