package com.nowen.video.ui.screen.server

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
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.ServerManager
import com.nowen.video.data.local.ServerProfile
import com.nowen.video.data.local.TokenManager
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServerManageScreen(onBack: () -> Unit, onServerSwitch: () -> Unit, onAddServer: () -> Unit, viewModel: ServerManageViewModel = hiltViewModel()) {
    val uiState by viewModel.uiState.collectAsState()
    var showDeleteDialog by remember { mutableStateOf<ServerProfile?>(null) }
    val colorScheme = MaterialTheme.colorScheme
    LaunchedEffect(Unit) { viewModel.loadServers() }
    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(containerColor = Color.Transparent, topBar = {
            TopAppBar(title = { Text("服务器管理", color = colorScheme.primary, style = MaterialTheme.typography.titleLarge.copy(letterSpacing = 1.sp)) },
                navigationIcon = { IconButton(onClick = onBack) { Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = colorScheme.primary) } },
                colors = TopAppBarDefaults.topAppBarColors(containerColor = colorScheme.surface.copy(alpha = 0.95f)))
        }, floatingActionButton = {
            FloatingActionButton(onClick = onAddServer, containerColor = colorScheme.primary) { Icon(Icons.Default.Add, "添加服务器", tint = Color.White) }
        }) { padding ->
            if (uiState.servers.isEmpty()) {
                Box(Modifier.fillMaxSize().padding(padding), Alignment.Center) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Icon(Icons.Default.Dns, null, Modifier.size(64.dp), tint = colorScheme.tertiary.copy(alpha = 0.4f))
                        Spacer(Modifier.height(16.dp)); Text("暂无服务器", style = MaterialTheme.typography.bodyLarge, color = colorScheme.onSurfaceVariant)
                        Text("点击右下角按钮添加服务器", style = MaterialTheme.typography.bodySmall, color = colorScheme.outline)
                    }
                }
            } else {
                LazyColumn(Modifier.fillMaxSize().padding(padding), contentPadding = PaddingValues(16.dp), verticalArrangement = Arrangement.spacedBy(12.dp)) {
                    items(uiState.servers) { server ->
                        val isActive = server.id == uiState.activeServerId
                        Box(Modifier.fillMaxWidth().clip(RoundedCornerShape(14.dp))
                            .then(if (isActive) Modifier.border(1.dp, Brush.horizontalGradient(listOf(colorScheme.primary, colorScheme.tertiary)), RoundedCornerShape(14.dp)).background(colorScheme.primary.copy(alpha = 0.06f))
                            else Modifier.glassMorphism(cornerRadius = 14.dp))
                            .clickable { if (!isActive) viewModel.switchServer(server.id, onServerSwitch) }) {
                            Row(Modifier.padding(16.dp), verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(12.dp)) {
                                Icon(if (isActive) Icons.Default.CheckCircle else Icons.Default.Dns, null, Modifier.size(40.dp), tint = if (isActive) colorScheme.tertiary else colorScheme.outline)
                                Column(Modifier.weight(1f)) {
                                    Row(verticalAlignment = Alignment.CenterVertically, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                        Text(server.name, style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.Medium), color = colorScheme.onSurface)
                                        if (isActive) Surface(shape = RoundedCornerShape(4.dp), color = colorScheme.tertiary) { Text("当前", style = MaterialTheme.typography.labelSmall, color = Color.White, modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)) }
                                    }
                                    Text(server.url, style = MaterialTheme.typography.bodySmall, color = if (isActive) colorScheme.primary.copy(alpha = 0.8f) else colorScheme.outline)
                                    if (server.username.isNotBlank()) Text("用户: ${server.username}", style = MaterialTheme.typography.labelSmall, color = colorScheme.outline)
                                }
                                if (!isActive) IconButton({ showDeleteDialog = server }) { Icon(Icons.Default.Delete, "删除", tint = colorScheme.error.copy(alpha = 0.7f)) }
                            }
                        }
                    }
                }
            }
        }
    }
    showDeleteDialog?.let { server ->
        AlertDialog(onDismissRequest = { showDeleteDialog = null }, title = { Text("删除服务器", color = colorScheme.onSurface) },
            text = { Text("确定要删除「${server.name}」吗？\n服务器地址: ${server.url}", color = colorScheme.onSurfaceVariant) },
            containerColor = colorScheme.surface,
            confirmButton = { TextButton({ viewModel.deleteServer(server.id); showDeleteDialog = null }) { Text("删除", color = colorScheme.error) } },
            dismissButton = { TextButton({ showDeleteDialog = null }) { Text("取消", color = colorScheme.primary) } })
    }
}

data class ServerManageUiState(val servers: List<ServerProfile> = emptyList(), val activeServerId: String = "")
@HiltViewModel
class ServerManageViewModel @Inject constructor(private val serverManager: ServerManager, private val tokenManager: TokenManager) : ViewModel() {
    private val _uiState = MutableStateFlow(ServerManageUiState()); val uiState = _uiState.asStateFlow()
    fun loadServers() { viewModelScope.launch { serverManager.migrateFromTokenManager(tokenManager); val s = serverManager.getServers(); val a = serverManager.getActiveServer(); _uiState.value = ServerManageUiState(s, a?.id ?: "") } }
    fun switchServer(serverId: String, onComplete: () -> Unit) { viewModelScope.launch { val s = serverManager.getServers().find { it.id == serverId } ?: return@launch; serverManager.setActiveServer(serverId); tokenManager.saveServerUrl(s.url); tokenManager.saveToken(s.token); if (s.username.isNotBlank()) tokenManager.saveUserInfo(s.userId, s.username, s.userRole); onComplete() } }
    fun deleteServer(serverId: String) { viewModelScope.launch { serverManager.removeServer(serverId); loadServers() } }
}
