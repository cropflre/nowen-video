package com.nowen.video.ui.screen.settings

import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.ThemeMode
import com.nowen.video.data.local.ThemePreferences
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.remote.WSConnectionState
import com.nowen.video.data.remote.WebSocketManager
import com.nowen.video.data.repository.AuthRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 设置页面 — Phase 4 增强版
 * 包含用户信息、主题切换、服务器管理、播放器设置、后台任务等入口
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    onLogout: () -> Unit,
    onPlayerSettings: () -> Unit = {},
    onServerManage: () -> Unit = {},
    onNotifications: () -> Unit = {},
    onBack: () -> Unit,
    viewModel: SettingsViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    var showLogoutDialog by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) {
        viewModel.loadInfo()
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("设置") },
                navigationIcon = {
                    IconButton(onClick = onBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(16.dp)
        ) {
            // 用户信息卡片
            Card(
                modifier = Modifier.fillMaxWidth(),
                colors = CardDefaults.cardColors(
                    containerColor = MaterialTheme.colorScheme.primaryContainer
                )
            ) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically,
                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                ) {
                    Icon(
                        Icons.Default.AccountCircle,
                        contentDescription = null,
                        modifier = Modifier.size(48.dp),
                        tint = MaterialTheme.colorScheme.onPrimaryContainer
                    )
                    Column {
                        Text(
                            text = uiState.username,
                            style = MaterialTheme.typography.titleMedium,
                            color = MaterialTheme.colorScheme.onPrimaryContainer
                        )
                        Text(
                            text = if (uiState.role == "admin") "管理员" else "普通用户",
                            style = MaterialTheme.typography.bodySmall,
                            color = MaterialTheme.colorScheme.onPrimaryContainer.copy(alpha = 0.7f)
                        )
                    }
                }
            }

            Spacer(modifier = Modifier.height(24.dp))

            // ==================== 功能设置 ====================

            // 服务器管理
            ListItem(
                headlineContent = { Text("服务器管理") },
                supportingContent = { Text(uiState.serverUrl) },
                leadingContent = {
                    Icon(Icons.Default.Dns, contentDescription = null)
                },
                trailingContent = {
                    Icon(Icons.Default.ChevronRight, contentDescription = null)
                },
                modifier = Modifier.clickable(onClick = onServerManage)
            )

            HorizontalDivider()

            // 播放器设置
            ListItem(
                headlineContent = { Text("播放器设置") },
                supportingContent = { Text("倍速、画面比例、解码器、字幕、手势") },
                leadingContent = {
                    Icon(Icons.Default.PlayCircle, contentDescription = null)
                },
                trailingContent = {
                    Icon(Icons.Default.ChevronRight, contentDescription = null)
                },
                modifier = Modifier.clickable(onClick = onPlayerSettings)
            )

            HorizontalDivider()

            // 后台任务
            ListItem(
                headlineContent = { Text("后台任务") },
                supportingContent = {
                    Text(
                        when (uiState.wsState) {
                            WSConnectionState.CONNECTED -> "已连接 · 实时接收通知"
                            WSConnectionState.CONNECTING -> "连接中..."
                            WSConnectionState.RECONNECTING -> "重连中..."
                            WSConnectionState.DISCONNECTED -> "未连接"
                        }
                    )
                },
                leadingContent = {
                    Icon(Icons.Default.Notifications, contentDescription = null)
                },
                trailingContent = {
                    Row(
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(4.dp)
                    ) {
                        // 连接状态指示灯
                        Surface(
                            modifier = Modifier.size(8.dp),
                            shape = MaterialTheme.shapes.small,
                            color = when (uiState.wsState) {
                                WSConnectionState.CONNECTED -> MaterialTheme.colorScheme.primary
                                WSConnectionState.CONNECTING, WSConnectionState.RECONNECTING ->
                                    MaterialTheme.colorScheme.tertiary
                                WSConnectionState.DISCONNECTED -> MaterialTheme.colorScheme.error
                            }
                        ) {}
                        Icon(Icons.Default.ChevronRight, contentDescription = null)
                    }
                },
                modifier = Modifier.clickable(onClick = onNotifications)
            )

            HorizontalDivider()

            // 主题切换
            var showThemeDialog by remember { mutableStateOf(false) }
            val themeModeLabel = when (uiState.themeMode) {
                ThemeMode.SYSTEM -> "跟随系统"
                ThemeMode.LIGHT -> "浅色模式"
                ThemeMode.DARK -> "深色模式"
            }
            ListItem(
                headlineContent = { Text("主题模式") },
                supportingContent = { Text(themeModeLabel) },
                leadingContent = {
                    Icon(
                        when (uiState.themeMode) {
                            ThemeMode.SYSTEM -> Icons.Default.BrightnessAuto
                            ThemeMode.LIGHT -> Icons.Default.LightMode
                            ThemeMode.DARK -> Icons.Default.DarkMode
                        },
                        contentDescription = null
                    )
                },
                trailingContent = {
                    Icon(Icons.Default.ChevronRight, contentDescription = null)
                },
                modifier = Modifier.clickable { showThemeDialog = true }
            )

            if (showThemeDialog) {
                AlertDialog(
                    onDismissRequest = { showThemeDialog = false },
                    title = { Text("选择主题") },
                    text = {
                        Column {
                            ThemeMode.entries.forEach { mode ->
                                val label = when (mode) {
                                    ThemeMode.SYSTEM -> "跟随系统"
                                    ThemeMode.LIGHT -> "浅色模式"
                                    ThemeMode.DARK -> "深色模式"
                                }
                                val icon = when (mode) {
                                    ThemeMode.SYSTEM -> Icons.Default.BrightnessAuto
                                    ThemeMode.LIGHT -> Icons.Default.LightMode
                                    ThemeMode.DARK -> Icons.Default.DarkMode
                                }
                                ListItem(
                                    headlineContent = { Text(label) },
                                    leadingContent = {
                                        RadioButton(
                                            selected = uiState.themeMode == mode,
                                            onClick = {
                                                viewModel.setThemeMode(mode)
                                                showThemeDialog = false
                                            }
                                        )
                                    },
                                    trailingContent = {
                                        Icon(icon, contentDescription = null)
                                    },
                                    modifier = Modifier.clickable {
                                        viewModel.setThemeMode(mode)
                                        showThemeDialog = false
                                    }
                                )
                            }
                        }
                    },
                    confirmButton = {
                        TextButton(onClick = { showThemeDialog = false }) { Text("取消") }
                    }
                )
            }

            HorizontalDivider()

            // 应用版本
            ListItem(
                headlineContent = { Text("应用版本") },
                supportingContent = { Text("1.0.0") },
                leadingContent = {
                    Icon(Icons.Default.Info, contentDescription = null)
                }
            )

            HorizontalDivider()

            Spacer(modifier = Modifier.weight(1f))

            // 退出登录按钮
            OutlinedButton(
                onClick = { showLogoutDialog = true },
                modifier = Modifier.fillMaxWidth(),
                colors = ButtonDefaults.outlinedButtonColors(
                    contentColor = MaterialTheme.colorScheme.error
                )
            ) {
                Icon(Icons.AutoMirrored.Filled.Logout, contentDescription = null)
                Spacer(modifier = Modifier.width(8.dp))
                Text("退出登录")
            }
        }
    }

    // 退出确认对话框
    if (showLogoutDialog) {
        AlertDialog(
            onDismissRequest = { showLogoutDialog = false },
            title = { Text("退出登录") },
            text = { Text("确定要退出当前账号吗？") },
            confirmButton = {
                TextButton(
                    onClick = {
                        showLogoutDialog = false
                        viewModel.logout(onLogout)
                    }
                ) {
                    Text("确定", color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = {
                TextButton(onClick = { showLogoutDialog = false }) {
                    Text("取消")
                }
            }
        )
    }
}

// ==================== ViewModel ====================

data class SettingsUiState(
    val username: String = "",
    val role: String = "",
    val serverUrl: String = "",
    val wsState: WSConnectionState = WSConnectionState.DISCONNECTED,
    val themeMode: ThemeMode = ThemeMode.SYSTEM
)

@HiltViewModel
class SettingsViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    private val tokenManager: TokenManager,
    private val webSocketManager: WebSocketManager,
    private val themePreferences: ThemePreferences
) : ViewModel() {

    private val _uiState = MutableStateFlow(SettingsUiState())
    val uiState = _uiState.asStateFlow()

    fun loadInfo() {
        viewModelScope.launch {
            val username = tokenManager.getUsername() ?: ""
            val serverUrl = tokenManager.getServerUrl() ?: ""
            _uiState.value = SettingsUiState(
                username = username,
                serverUrl = serverUrl
            )

            // 获取最新用户信息
            authRepository.getProfile().onSuccess { user ->
                _uiState.value = _uiState.value.copy(
                    username = user.username,
                    role = user.role
                )
            }
        }

        // 监听 WebSocket 连接状态
        viewModelScope.launch {
            webSocketManager.connectionState.collect { state ->
                _uiState.value = _uiState.value.copy(wsState = state)
            }
        }

        // 自动连接 WebSocket
        webSocketManager.connect()

        // 监听主题模式
        viewModelScope.launch {
            themePreferences.themeModeFlow.collect { mode ->
                _uiState.value = _uiState.value.copy(themeMode = mode)
            }
        }
    }

    fun setThemeMode(mode: ThemeMode) {
        viewModelScope.launch {
            themePreferences.setThemeMode(mode)
        }
    }

    fun logout(onComplete: () -> Unit) {
        viewModelScope.launch {
            webSocketManager.disconnect()
            authRepository.logout()
            onComplete()
        }
    }
}
