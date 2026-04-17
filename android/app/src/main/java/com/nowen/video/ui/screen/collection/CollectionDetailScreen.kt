package com.nowen.video.ui.screen.collection

import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
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
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.MovieCollection
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.screen.home.MediaPosterCard
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun CollectionDetailScreen(collectionId: String, onMediaClick: (String) -> Unit, onBack: () -> Unit, viewModel: CollectionDetailViewModel = hiltViewModel()) {
    val uiState by viewModel.uiState.collectAsState()
    val colorScheme = MaterialTheme.colorScheme
    LaunchedEffect(collectionId) { viewModel.loadCollection(collectionId) }
    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(containerColor = Color.Transparent, topBar = {
            TopAppBar(title = { Text(uiState.collection?.name ?: "合集详情", color = colorScheme.secondary, style = MaterialTheme.typography.titleLarge.copy(letterSpacing = 1.sp)) },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary) } },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = colorScheme.scrim.copy(alpha = 0.6f)))
        }) { padding ->
            if (uiState.loading) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) { CircularProgressIndicator(color = colorScheme.primary, trackColor = colorScheme.surfaceContainerHigh) }
            } else {
                val collection = uiState.collection ?: return@Scaffold
                LazyColumn(Modifier.fillMaxSize().padding(padding), contentPadding = PaddingValues(bottom = 16.dp)) {
                    item {
                        Box(Modifier.fillMaxWidth().height(220.dp)) {
                            AsyncImage("${uiState.serverUrl}/api/collections/${collection.id}/poster?token=${uiState.token}", null, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
                            Box(Modifier.fillMaxSize().background(Brush.verticalGradient(listOf(Color.Transparent, colorScheme.scrim.copy(alpha = 0.3f), colorScheme.scrim.copy(alpha = 0.9f), colorScheme.scrim))))
                            Box(Modifier.fillMaxWidth().height(2.dp).align(Alignment.BottomCenter).background(Brush.horizontalGradient(listOf(Color.Transparent, colorScheme.secondary.copy(alpha = 0.5f), colorScheme.secondary.copy(alpha = 0.8f), colorScheme.secondary.copy(alpha = 0.5f), Color.Transparent))))
                        }
                    }
                    item {
                        Column(Modifier.padding(16.dp)) {
                            Text(collection.name, style = MaterialTheme.typography.headlineMedium.copy(fontWeight = FontWeight.Bold), color = colorScheme.onSurface)
                            Text("${collection.mediaCount} 部影片", style = MaterialTheme.typography.bodyMedium, color = colorScheme.secondary, modifier = Modifier.padding(top = 4.dp))
                            if (collection.overview.isNotBlank()) Text(collection.overview, style = MaterialTheme.typography.bodyMedium, color = colorScheme.onSurfaceVariant, modifier = Modifier.padding(top = 12.dp), maxLines = 4, overflow = TextOverflow.Ellipsis)
                        }
                    }
                    item { Text("系列影片", style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.SemiBold, letterSpacing = 1.sp), color = colorScheme.primary, modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp)) }
                    val movies = collection.media ?: emptyList()
                    item {
                        LazyRow(contentPadding = PaddingValues(horizontal = 16.dp), horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                            items(movies) { media -> MediaPosterCard(media, uiState.serverUrl, uiState.token) { onMediaClick(media.id) } }
                        }
                    }
                }
            }
        }
    }
}

data class CollectionDetailUiState(val loading: Boolean = true, val collection: MovieCollection? = null, val serverUrl: String = "", val token: String = "")
@HiltViewModel
class CollectionDetailViewModel @Inject constructor(private val mediaRepository: MediaRepository, private val tokenManager: TokenManager) : ViewModel() {
    private val _uiState = MutableStateFlow(CollectionDetailUiState()); val uiState = _uiState.asStateFlow()
    fun loadCollection(collectionId: String) { viewModelScope.launch {
        _uiState.value = _uiState.value.copy(loading = true); val s = tokenManager.getServerUrl() ?: ""; val t = tokenManager.getToken() ?: ""
        _uiState.value = _uiState.value.copy(serverUrl = s, token = t)
        mediaRepository.getCollectionDetail(collectionId).onSuccess { _uiState.value = _uiState.value.copy(loading = false, collection = it) }.onFailure { _uiState.value = _uiState.value.copy(loading = false) }
    } }
}
