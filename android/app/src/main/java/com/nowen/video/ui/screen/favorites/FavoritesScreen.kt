package com.nowen.video.ui.screen.favorites

import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.Media
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun FavoritesScreen(onMediaClick: (String) -> Unit, onBack: () -> Unit, viewModel: FavoritesViewModel = hiltViewModel()) {
    val uiState by viewModel.uiState.collectAsState()
    val colorScheme = MaterialTheme.colorScheme
    LaunchedEffect(Unit) { viewModel.loadFavorites() }
    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(containerColor = Color.Transparent, topBar = {
            TopAppBar(title = { Text("我的收藏", color = colorScheme.error, style = MaterialTheme.typography.titleLarge.copy(letterSpacing = 1.sp)) },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary) } },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = colorScheme.scrim.copy(alpha = 0.85f)))
        }) { padding ->
            if (uiState.loading) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) { CircularProgressIndicator(color = colorScheme.primary, trackColor = colorScheme.surfaceContainerHigh) }
            } else if (uiState.favorites.isEmpty()) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(Icons.Default.FavoriteBorder, null, Modifier.size(64.dp), tint = colorScheme.error.copy(alpha = 0.4f))
                        Spacer(Modifier.height(16.dp)); Text("暂无收藏", style = MaterialTheme.typography.bodyLarge, color = colorScheme.onSurfaceVariant)
                        Text("在电影详情页点击收藏按钮添加", style = MaterialTheme.typography.bodySmall, color = colorScheme.outline)
                    }
                }
            } else {
                LazyVerticalGrid(GridCells.Adaptive(130.dp), Modifier.fillMaxSize().padding(padding), contentPadding = PaddingValues(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(uiState.favorites) { media ->
                        Box(Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).cyberCard(cornerRadius = 14.dp).clickable { onMediaClick(media.id) }) {
                            Column {
                                Box(Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(topStart = 14.dp, topEnd = 14.dp))) {
                                    AsyncImage("${uiState.serverUrl}/api/media/${media.id}/poster?token=${uiState.token}", media.title, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                                    Box(Modifier.fillMaxWidth().height(50.dp).align(Alignment.BottomCenter).gradientScrim())
                                    Icon(Icons.Default.Favorite, null, Modifier.align(Alignment.TopEnd).padding(6.dp).size(20.dp), tint = colorScheme.error)
                                }
                                Column(Modifier.padding(8.dp)) {
                                    Text(media.title, style = MaterialTheme.typography.bodySmall, color = colorScheme.onSurface, maxLines = 1, overflow = TextOverflow.Ellipsis)
                                    if (media.year > 0) Text("${media.year}", style = MaterialTheme.typography.labelSmall, color = colorScheme.outline)
                                }
                            }
                        }
                    }
                }
            }
        }
    }
}

data class FavoritesUiState(val loading: Boolean = true, val favorites: List<Media> = emptyList(), val serverUrl: String = "", val token: String = "")
@HiltViewModel
class FavoritesViewModel @Inject constructor(private val mediaRepository: MediaRepository, private val tokenManager: TokenManager) : ViewModel() {
    private val _uiState = MutableStateFlow(FavoritesUiState()); val uiState = _uiState.asStateFlow()
    fun loadFavorites() { viewModelScope.launch {
        _uiState.value = _uiState.value.copy(loading = true)
        val serverUrl = tokenManager.getServerUrl() ?: ""; val token = tokenManager.getToken() ?: ""
        _uiState.value = _uiState.value.copy(serverUrl = serverUrl, token = token)
        mediaRepository.getFavorites().onSuccess { _uiState.value = _uiState.value.copy(loading = false, favorites = it) }.onFailure { _uiState.value = _uiState.value.copy(loading = false) }
    } }
}
