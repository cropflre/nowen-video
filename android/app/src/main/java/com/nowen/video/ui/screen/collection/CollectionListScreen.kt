package com.nowen.video.ui.screen.collection

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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import coil.compose.AsyncImage
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.MovieCollection
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CollectionListScreen(onCollectionClick: (String) -> Unit, onBack: () -> Unit, viewModel: CollectionListViewModel = hiltViewModel()) {
    val uiState by viewModel.uiState.collectAsState()
    val colorScheme = MaterialTheme.colorScheme
    LaunchedEffect(Unit) { viewModel.loadCollections() }
    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(containerColor = Color.Transparent, topBar = {
            TopAppBar(title = { Text("影视合集", color = colorScheme.secondary, style = MaterialTheme.typography.titleLarge.copy(letterSpacing = 1.sp)) },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary) } },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = colorScheme.scrim.copy(alpha = 0.85f)))
        }) { padding ->
            if (uiState.loading) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) { CircularProgressIndicator(color = colorScheme.primary, trackColor = colorScheme.surfaceContainerHigh) }
            } else if (uiState.collections.isEmpty()) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(Icons.Default.Collections, null, Modifier.size(64.dp), tint = colorScheme.secondary.copy(alpha = 0.4f))
                        Spacer(Modifier.height(16.dp)); Text("暂无合集", style = MaterialTheme.typography.bodyLarge, color = colorScheme.onSurfaceVariant)
                    }
                }
            } else {
                LazyVerticalGrid(GridCells.Adaptive(160.dp), Modifier.fillMaxSize().padding(padding), contentPadding = PaddingValues(12.dp), verticalArrangement = Arrangement.spacedBy(12.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(uiState.collections) { c ->
                        Box(Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).cyberCard(cornerRadius = 14.dp, glowColor = colorScheme.secondary).clickable { onCollectionClick(c.id) }) {
                            Column {
                                Box(Modifier.fillMaxWidth().aspectRatio(2f / 3f).clip(RoundedCornerShape(topStart = 14.dp, topEnd = 14.dp))) {
                                    AsyncImage("${uiState.serverUrl}/api/collections/${c.id}/poster?token=${uiState.token}", c.name, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                                    Box(Modifier.fillMaxWidth().height(50.dp).align(Alignment.BottomCenter).gradientScrim())
                                    Surface(Modifier.align(Alignment.BottomEnd).padding(6.dp), RoundedCornerShape(6.dp), colorScheme.secondary.copy(alpha = 0.85f)) {
                                        Text("${c.mediaCount} 部", style = MaterialTheme.typography.labelSmall, color = Color.White, modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp))
                                    }
                                }
                                Column(Modifier.padding(10.dp)) { Text(c.name, style = MaterialTheme.typography.bodyMedium, color = colorScheme.onSurface, maxLines = 2, overflow = TextOverflow.Ellipsis) }
                            }
                        }
                    }
                }
            }
        }
    }
}

data class CollectionListUiState(val loading: Boolean = true, val collections: List<MovieCollection> = emptyList(), val serverUrl: String = "", val token: String = "")
@HiltViewModel
class CollectionListViewModel @Inject constructor(private val mediaRepository: MediaRepository, private val tokenManager: TokenManager) : ViewModel() {
    private val _uiState = MutableStateFlow(CollectionListUiState()); val uiState = _uiState.asStateFlow()
    fun loadCollections() { viewModelScope.launch {
        _uiState.value = _uiState.value.copy(loading = true); val s = tokenManager.getServerUrl() ?: ""; val t = tokenManager.getToken() ?: ""
        _uiState.value = _uiState.value.copy(serverUrl = s, token = t)
        mediaRepository.getCollections().onSuccess { _uiState.value = _uiState.value.copy(loading = false, collections = it) }.onFailure { _uiState.value = _uiState.value.copy(loading = false) }
    } }
}
