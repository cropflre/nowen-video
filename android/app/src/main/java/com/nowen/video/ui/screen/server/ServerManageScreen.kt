package com.nowen.video.ui.screen.server

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
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.ServerManager
import com.nowen.video.data.local.ServerProfile
import com.nowen.video.data.local.TokenManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 多服务器管理页面
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServerManageScreen(
    onBack: () -> Unit,
    onServerSwitch: () -> Unit,
    onAddServer: () -> Unit,
    viewModel: ServerManageViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    var showDeleteDialog by remember { mutableStateOf<ServerProfile?>(null) }

    LaunchedEffect(Unit) {
        viewModel.loadServers()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("服务器管理") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        },
        floatingActionButton = {
            FloatingActionButton(
                onClick = onAddServer,
                containerColor = MaterialTheme.colorScheme.primary
            ) {
                Icon(Icons.Default.Add, contentDescription = "添加服务器")
            }
        }
    ) { padding ->
        if (uiState.servers.isEmpty()) {
            Box(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentAlignment = Alignment.Center
            ) {
                Column(horizontalAlignment = Alignment.CenterHorizontally) {
                    Icon(
                        Icons.Default.Dns,
                        contentDescription = null,
                        modifier = Modifier.size(64.dp),
                        tint = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Spacer(modifier = Modifier.height(16.dp))
                    Text(
                        "暂无服务器",
                        style = MaterialTheme.typography.bodyLarge,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                    Text(
                        "点击右下角按钮添加服务器",
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        } else {
            LazyColumn(
                modifier = Modifier.fillMaxSize().padding(padding),
                contentPadding = PaddingValues(16.dp),
                verticalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                items(uiState.servers) { server ->
                    ServerCard(
                        server = server,
                        isActive = server.id == uiState.activeServerId,
                        onClick = {
                            if (server.id != uiState.activeServerId) {
                                viewModel.switchServer(server.id, onServerSwitch)
                            }
                        },
                        onDelete = { showDeleteDialog = server }
                    )
                }
            }
        }
    }

    // 删除确认对话框
    showDeleteDialog?.let { server ->
        AlertDialog(
            onDismissRequest = { showDeleteDialog = null },
            title = { Text("删除服务器") },
            text = { Text("确定要删除「${server.name}」吗？\n服务器地址: ${server.url}") },
            confirmButton = {
                TextButton(
                    onClick = {
                        viewModel.deleteServer(server.id)
                        showDeleteDialog = null
                    }
                ) {
                    Text("删除", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = {
                TextButton(onClick = { showDeleteDialog = null }) {
                    Text("取消")
                }
            }
        )
    }
}

@Composable
private fun ServerCard(
    server: ServerProfile,
    isActive: Boolean,
    onClick: () -> Unit,
    onDelete: () -> Unit
) {
    Card(
        modifier = Modifier.fillMaxWidth().clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        colors = if (isActive) {
            CardDefaults.cardColors(
                containerColor = MaterialTheme.colorScheme.primaryContainer
            )
        } else {
            CardDefaults.cardColors()
        },
        elevation = CardDefaults.cardElevation(
            defaultElevation = if (isActive) 4.dp else 1.dp
        )
    ) {
        Row(
            modifier = Modifier.padding(16.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(12.dp)
        ) {
            // 服务器图标
            Icon(
                if (isActive) Icons.Default.CheckCircle else Icons.Default.Dns,
                contentDescription = null,
                modifier = Modifier.size(40.dp),
                tint = if (isActive) MaterialTheme.colorScheme.primary
                else MaterialTheme.colorScheme.onSurfaceVariant
            )

            Column(modifier = Modifier.weight(1f)) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    Text(
                        text = server.name,
                        style = MaterialTheme.typography.titleMedium
                    )
                    if (isActive) {
                        Surface(
                            shape = RoundedCornerShape(4.dp),
                            color = MaterialTheme.colorScheme.primary
                        ) {
                            Text(
                                "当前",
                                style = MaterialTheme.typography.labelSmall,
                                color = MaterialTheme.colorScheme.onPrimary,
                                modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                            )
                        }
                    }
                }
                Text(
                    text = server.url,
                    style = MaterialTheme.typography.bodySmall,
                    color = if (isActive) MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
                    else MaterialTheme.colorScheme.onSurfaceVariant
                )
                if (server.username.isNotBlank()) {
                    Text(
                        text = "用户: ${server.username}",
                        style = MaterialTheme.typography.labelSmall,
                        color = if (isActive) MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.5f)
                        else MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.7f)
                    )
                }
            }

            // 删除按钮（非活跃服务器才显示）
            if (!isActive) {
                IconButton(onClick = onDelete) {
                    Icon(
                        Icons.Default.Delete,
                        contentDescription = "删除",
                        tint = MaterialTheme.colorScheme.error.copy(alpha = 0.7f)
                    )
                }
            }
        }
    }
}

// ==================== ViewModel ====================

data class ServerManageUiState(
    val servers: List<ServerProfile> = emptyList(),
    val activeServerId: String = ""
)

@HiltViewModel
class ServerManageViewModel @Inject constructor(
    private val serverManager: ServerManager,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(ServerManageUiState())
    val uiState = _uiState.asStateFlow()

    fun loadServers() {
        viewModelScope.launch {
            // 首次迁移
            serverManager.migrateFromTokenManager(tokenManager)

            val servers = serverManager.getServers()
            val active = serverManager.getActiveServer()
            _uiState.value = ServerManageUiState(
                servers = servers,
                activeServerId = active?.id ?: ""
            )
        }
    }

    fun switchServer(serverId: String, onComplete: () -> Unit) {
        viewModelScope.launch {
            val server = serverManager.getServers().find { it.id == serverId } ?: return@launch

            // 切换活跃服务器
            serverManager.setActiveServer(serverId)

            // 更新 TokenManager（兼容现有代码）
            tokenManager.saveServerUrl(server.url)
            tokenManager.saveToken(server.token)
            if (server.username.isNotBlank()) {
                tokenManager.saveUserInfo(server.userId, server.username, server.userRole)
            }

            onComplete()
        }
    }

    fun deleteServer(serverId: String) {
        viewModelScope.launch {
            serverManager.removeServer(serverId)
            loadServers()
        }
    }
}
