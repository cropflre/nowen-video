package com.nowen.video.ui.screen.history

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
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
import com.nowen.video.data.model.WatchHistory
import com.nowen.video.data.repository.MediaRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HistoryScreen(onMediaClick: (String) -> Unit, onSeriesClick: (String) -> Unit = {}, onBack: () -> Unit, viewModel: HistoryViewModel = hiltViewModel()) {
    val uiState by viewModel.uiState.collectAsState()
    var showClearDialog by remember { mutableStateOf(false) }
    val colorScheme = MaterialTheme.colorScheme
    LaunchedEffect(Unit) { viewModel.loadHistory() }
    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(containerColor = Color.Transparent, topBar = {
            TopAppBar(title = { Text("观看历史", color = colorScheme.secondary, style = MaterialTheme.typography.titleLarge.copy(letterSpacing = 1.sp)) },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary) } },
                actions = { if (uiState.history.isNotEmpty()) IconButton({ showClearDialog = true }) { Icon(Icons.Default.DeleteSweep, "清空历史", tint = colorScheme.error.copy(alpha = 0.7f)) } },
colors = TopAppBarDefaults.topAppBarColors(containerColor = colorScheme.surface.copy(alpha = 0.95f)))
        }) { padding ->
            if (uiState.loading) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) { CircularProgressIndicator(color = colorScheme.primary, trackColor = colorScheme.surfaceContainerHigh) }
            } else if (uiState.history.isEmpty()) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(Icons.Default.History, null, Modifier.size(64.dp), tint = colorScheme.secondary.copy(alpha = 0.4f))
                        Spacer(Modifier.height(16.dp)); Text("暂无观看记录", style = MaterialTheme.typography.bodyLarge, color = colorScheme.onSurfaceVariant)
                    }
                }
            } else {
                LazyColumn(Modifier.fillMaxSize().padding(padding), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(10.dp)) {
                    items(uiState.history) { history ->
                        CyberHistoryItem(history, uiState.serverUrl, uiState.token,
                            onClick = { history.media?.let { m -> if (m.mediaType == "episode" && m.seriesId.isNotBlank()) onSeriesClick(m.seriesId) else onMediaClick(m.id) } },
                            onDelete = { viewModel.deleteHistory(history.mediaId) })
                    }
                }
            }
        }
    }
    if (showClearDialog) {
        AlertDialog(onDismissRequest = { showClearDialog = false }, title = { Text("清空观看历史", color = colorScheme.onSurface) }, text = { Text("确定要清空所有观看记录吗？此操作不可撤销。", color = colorScheme.onSurfaceVariant) },
            containerColor = colorScheme.surface,
            confirmButton = { TextButton({ showClearDialog = false; viewModel.clearHistory() }) { Text("确定", color = colorScheme.error) } },
            dismissButton = { TextButton({ showClearDialog = false }) { Text("取消", color = colorScheme.primary) } })
    }
}

@Composable
private fun CyberHistoryItem(history: WatchHistory, serverUrl: String, token: String, onClick: () -> Unit, onDelete: () -> Unit) {
    val media = history.media ?: return
    val progress = if (history.duration > 0) (history.position / history.duration).toFloat() else 0f
    val colorScheme = MaterialTheme.colorScheme
    Box(Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp)).glassMorphism(cornerRadius = 14.dp).clickable(onClick = onClick)) {
        Row(Modifier.padding(12.dp), Arrangement.spacedBy(12.dp)) {
            Box(Modifier.width(80.dp).height(120.dp).clip(RoundedCornerShape(8.dp)).border(1.dp, colorScheme.primary.copy(alpha = 0.15f), RoundedCornerShape(8.dp))) {
                AsyncImage("$serverUrl/api/media/${media.id}/poster?token=$token", media.title, Modifier.fillMaxSize(), contentScale = ContentScale.Crop)
if (history.completed) Surface(Modifier.align(Alignment.TopEnd).padding(4.dp), RoundedCornerShape(4.dp), colorScheme.tertiary) {
                    Icon(Icons.Default.Check, "已看完", Modifier.size(14.dp).padding(1.dp), tint = Color.White)
                }
            }
            Column(Modifier.weight(1f)) {
                Text(media.displayTitle(), style = MaterialTheme.typography.titleSmall, color = colorScheme.onSurface, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (media.mediaType == "episode" && media.episodeTitle.isNotBlank()) Text(media.episodeTitle, style = MaterialTheme.typography.labelSmall, color = colorScheme.outline, maxLines = 1, overflow = TextOverflow.Ellipsis)
                if (media.year > 0) Text("${media.year}", style = MaterialTheme.typography.bodySmall, color = colorScheme.outline)
                Spacer(Modifier.height(8.dp))
                // 霓虹进度条
                Box(Modifier.fillMaxWidth().height(3.dp).clip(RoundedCornerShape(2.dp)).background(colorScheme.surfaceVariant)) {
Box(Modifier.fillMaxWidth(progress.coerceIn(0f, 1f)).fillMaxHeight().background(Brush.horizontalGradient(listOf(colorScheme.primary, colorScheme.tertiary))))
                }
                Text(if (history.completed) "已看完" else "${formatDuration(history.position)} / ${formatDuration(history.duration)}",
                    style = MaterialTheme.typography.labelSmall, color = colorScheme.outline, modifier = Modifier.padding(top = 4.dp))
            }
            IconButton(onDelete, Modifier.align(Alignment.Top)) { Icon(Icons.Default.Close, "删除记录", Modifier.size(18.dp), tint = colorScheme.error.copy(alpha = 0.6f)) }
        }
    }
}

private fun formatDuration(seconds: Double): String { val t = seconds.toInt(); val h = t / 3600; val m = (t % 3600) / 60; val s = t % 60; return if (h > 0) String.format("%d:%02d:%02d", h, m, s) else String.format("%d:%02d", m, s) }

data class HistoryUiState(val loading: Boolean = true, val history: List<WatchHistory> = emptyList(), val serverUrl: String = "", val token: String = "")
@HiltViewModel
class HistoryViewModel @Inject constructor(private val mediaRepository: MediaRepository, private val tokenManager: TokenManager) : ViewModel() {
    private val _uiState = MutableStateFlow(HistoryUiState()); val uiState = _uiState.asStateFlow()
    fun loadHistory() { viewModelScope.launch {
        _uiState.value = _uiState.value.copy(loading = true); val s = tokenManager.getServerUrl() ?: ""; val t = tokenManager.getToken() ?: ""
        _uiState.value = _uiState.value.copy(serverUrl = s, token = t)
        mediaRepository.getHistory().onSuccess { _uiState.value = _uiState.value.copy(loading = false, history = it) }.onFailure { _uiState.value = _uiState.value.copy(loading = false) }
    } }
    fun deleteHistory(mediaId: String) { viewModelScope.launch { mediaRepository.deleteHistory(mediaId).onSuccess { _uiState.value = _uiState.value.copy(history = _uiState.value.history.filter { it.mediaId != mediaId }) } } }
    fun clearHistory() { viewModelScope.launch { mediaRepository.clearHistory().onSuccess { _uiState.value = _uiState.value.copy(history = emptyList()) } } }
}
