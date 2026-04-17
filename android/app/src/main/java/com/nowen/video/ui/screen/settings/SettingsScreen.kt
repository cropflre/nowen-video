package com.nowen.video.ui.screen.settings

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.ThemeMode
import com.nowen.video.data.local.ThemePreferences
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.remote.WSConnectionState
import com.nowen.video.data.remote.WebSocketManager
import com.nowen.video.data.repository.AuthRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 设置页面 — 赛博朋克风格
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
    var showThemeDialog by remember { mutableStateOf(false) }

    LaunchedEffect(Unit) { viewModel.loadInfo() }

    Box(Modifier.fillMaxSize().spaceBackground()) {
        Scaffold(
            containerColor = Color.Transparent,
            topBar = {
                TopAppBar(
                    title = {
                        Text(
                            "设置",
                            color = MaterialTheme.colorScheme.primary,
                            style = MaterialTheme.typography.titleLarge.copy(
                                letterSpacing = 1.sp,
                                fontWeight = FontWeight.Bold
                            )
                        )
                    },
                    navigationIcon = {
                        IconButton(onClick = onBack) {
                            Icon(Icons.AutoMirrored.Filled.ArrowBack, "返回", tint = MaterialTheme.colorScheme.primary)
                        }
                    },
                    colors = TopAppBarDefaults.topAppBarColors(
                        containerColor = MaterialTheme.colorScheme.surfaceDim.copy(alpha = 0.85f)
                    )
                )
            }
        ) { padding ->
            Column(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(padding)
                    .verticalScroll(rememberScrollState())
                    .padding(16.dp),
                verticalArrangement = Arrangement.spacedBy(16.dp)
            ) {
                // ==================== 用户信息卡片 ====================
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .clip(RoundedCornerShape(16.dp))
                        .background(
                            Brush.horizontalGradient(
                                listOf(
                                    MaterialTheme.colorScheme.primary.copy(alpha = 0.08f),
                                    MaterialTheme.colorScheme.secondary.copy(alpha = 0.06f)
                                )
                            )
                        )
                        .border(
                            1.dp,
                            Brush.horizontalGradient(
                                listOf(
                                    MaterialTheme.colorScheme.primary.copy(alpha = 0.25f),
                                    MaterialTheme.colorScheme.secondary.copy(alpha = 0.15f)
                                )
                            ),
                            RoundedCornerShape(16.dp)
                        )
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth().padding(20.dp),
                        verticalAlignment = Alignment.CenterVertically,
                        horizontalArrangement = Arrangement.spacedBy(14.dp)
                    ) {
                        // 用户头像 — 霓虹光圈
                        Box(
                            modifier = Modifier
                                .size(52.dp)
                                .clip(CircleShape)
                                .background(MaterialTheme.colorScheme.primary.copy(alpha = 0.1f))
                                .border(
                                    2.dp,
                                    Brush.sweepGradient(listOf(MaterialTheme.colorScheme.primary, MaterialTheme.colorScheme.secondary, MaterialTheme.colorScheme.primary)),
                                    CircleShape
                                ),
                            contentAlignment = Alignment.Center
                        ) {
                            Icon(
                                Icons.Default.Person,
                                contentDescription = null,
                                modifier = Modifier.size(28.dp),
                                tint = MaterialTheme.colorScheme.primary
                            )
                        }
                        Column {
                            Text(
                                text = uiState.username.ifBlank { "未登录" },
                                style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.SemiBold),
                                color = MaterialTheme.colorScheme.onSurface
                            )
                            Text(
                                text = if (uiState.role == "admin") "管理员" else "普通用户",
                                style = MaterialTheme.typography.bodySmall,
                                color = if (uiState.role == "admin") ElectricGreen else MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        }
                    }
                }

                // ==================== 功能设置区域 ====================
                Text(
                    "功能",
                    style = MaterialTheme.typography.labelMedium.copy(
                        letterSpacing = 2.sp,
                        fontWeight = FontWeight.Bold
                    ),
                    color = MaterialTheme.colorScheme.primary.copy(alpha = 0.7f)
                )

                // 设置项卡片容器
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .glassMorphism(cornerRadius = 14.dp)
                ) {
                    Column {
                        // 服务器管理
                        CyberSettingsItem(
                            icon = Icons.Default.Dns,
                            iconColor = ElectricGreen,
                            title = "服务器管理",
                            subtitle = uiState.serverUrl.ifBlank { "未配置" },
                            onClick = onServerManage
                        )

                        CyberDivider()

                        // 播放器设置
                        CyberSettingsItem(
                            icon = Icons.Default.PlayCircle,
                            iconColor = MaterialTheme.colorScheme.primary,
                            title = "播放器设置",
                            subtitle = "倍速、画面比例、解码器、字幕、手势",
                            onClick = onPlayerSettings
                        )

                        CyberDivider()

                        // 后台任务
                        val wsColor = when (uiState.wsState) {
                            WSConnectionState.CONNECTED -> ElectricGreen
                            WSConnectionState.CONNECTING, WSConnectionState.RECONNECTING -> AmberGold
                            WSConnectionState.DISCONNECTED -> MaterialTheme.colorScheme.error
                        }
                        val wsText = when (uiState.wsState) {
                            WSConnectionState.CONNECTED -> "已连接 · 实时接收通知"
                            WSConnectionState.CONNECTING -> "连接中..."
                            WSConnectionState.RECONNECTING -> "重连中..."
                            WSConnectionState.DISCONNECTED -> "未连接"
                        }
                        CyberSettingsItem(
                            icon = Icons.Default.Notifications,
                            iconColor = MaterialTheme.colorScheme.secondary,
                            title = "后台任务",
                            subtitle = wsText,
                            subtitleColor = wsColor.copy(alpha = 0.8f),
                            onClick = onNotifications,
                            trailing = {
                                Box(
                                    modifier = Modifier
                                        .size(8.dp)
                                        .clip(CircleShape)
                                        .background(wsColor)
                                )
                            }
                        )
                    }
                }

                // ==================== 外观设置 ====================
                Text(
                    "外观",
                    style = MaterialTheme.typography.labelMedium.copy(
                        letterSpacing = 2.sp,
                        fontWeight = FontWeight.Bold
                    ),
                    color = MaterialTheme.colorScheme.primary.copy(alpha = 0.7f)
                )

                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .glassMorphism(cornerRadius = 14.dp)
                ) {
                    Column {
                        val themeModeLabel = when (uiState.themeMode) {
                            ThemeMode.SYSTEM -> "跟随系统"
                            ThemeMode.LIGHT -> "浅色模式"
                            ThemeMode.DARK -> "深色模式"
                        }
                        val themeIcon = when (uiState.themeMode) {
                            ThemeMode.SYSTEM -> Icons.Default.BrightnessAuto
                            ThemeMode.LIGHT -> Icons.Default.LightMode
                            ThemeMode.DARK -> Icons.Default.DarkMode
                        }
                        CyberSettingsItem(
                            icon = themeIcon,
                            iconColor = AmberGold,
                            title = "主题模式",
                            subtitle = themeModeLabel,
                            onClick = { showThemeDialog = true }
                        )

                        CyberDivider()

                        // 应用版本
                        CyberSettingsItem(
                            icon = Icons.Default.Info,
                            iconColor = MaterialTheme.colorScheme.outline,
                            title = "应用版本",
                            subtitle = "1.0.0",
                            showChevron = false
                        )
                    }
                }

                Spacer(Modifier.height(8.dp))

                // ==================== 退出登录 ====================
                OutlinedButton(
                    onClick = { showLogoutDialog = true },
                    modifier = Modifier.fillMaxWidth(),
                    shape = RoundedCornerShape(12.dp),
                    border = ButtonDefaults.outlinedButtonBorder(enabled = true).copy(
                        brush = Brush.horizontalGradient(
                            listOf(MaterialTheme.colorScheme.error.copy(alpha = 0.5f), MaterialTheme.colorScheme.error.copy(alpha = 0.2f))
                        )
                    ),
                    colors = ButtonDefaults.outlinedButtonColors(
                        contentColor = MaterialTheme.colorScheme.error
                    )
                ) {
                    Icon(Icons.AutoMirrored.Filled.Logout, contentDescription = null, tint = MaterialTheme.colorScheme.error)
                    Spacer(Modifier.width(8.dp))
                    Text("退出登录", fontWeight = FontWeight.Medium)
                }

                Spacer(Modifier.height(16.dp))
            }
        }
    }

    // 主题选择对话框
    if (showThemeDialog) {
        AlertDialog(
            onDismissRequest = { showThemeDialog = false },
            title = { Text("选择主题", color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Bold) },
            containerColor = MaterialTheme.colorScheme.surfaceContainer,
            shape = CyberDialogShape,
            text = {
                Column(verticalArrangement = Arrangement.spacedBy(4.dp)) {
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
                        val isSelected = uiState.themeMode == mode
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clip(RoundedCornerShape(10.dp))
                                .then(
                                    if (isSelected) Modifier.background(MaterialTheme.colorScheme.primary.copy(alpha = 0.08f))
                                        .border(1.dp, MaterialTheme.colorScheme.primary.copy(alpha = 0.2f), RoundedCornerShape(10.dp))
                                    else Modifier
                                )
                                .clickable {
                                    viewModel.setThemeMode(mode)
                                    showThemeDialog = false
                                }
                                .padding(12.dp),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp)
                        ) {
                            Icon(icon, null, tint = if (isSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant)
                            Text(label, color = if (isSelected) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurface)
                            Spacer(Modifier.weight(1f))
                            if (isSelected) {
                                Icon(Icons.Default.CheckCircle, null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(20.dp))
                            }
                        }
                    }
                }
            },
            confirmButton = {
                TextButton(onClick = { showThemeDialog = false }) {
                    Text("取消", color = MaterialTheme.colorScheme.primary)
                }
            }
        )
    }

    // 退出确认对话框
    if (showLogoutDialog) {
        AlertDialog(
            onDismissRequest = { showLogoutDialog = false },
            title = { Text("退出登录", color = MaterialTheme.colorScheme.onSurface, fontWeight = FontWeight.Bold) },
            text = { Text("确定要退出当前账号吗？", color = MaterialTheme.colorScheme.onSurfaceVariant) },
            containerColor = MaterialTheme.colorScheme.surfaceContainer,
            shape = CyberDialogShape,
            confirmButton = {
                TextButton(onClick = { showLogoutDialog = false; viewModel.logout(onLogout) }) {
                    Text("确定", color = MaterialTheme.colorScheme.error, fontWeight = FontWeight.Bold)
                }
            },
            dismissButton = {
                TextButton(onClick = { showLogoutDialog = false }) {
                    Text("取消", color = MaterialTheme.colorScheme.primary)
                }
            }
        )
    }
}

/**
 * 赛博朋克设置项组件
 */
@Composable
private fun CyberSettingsItem(
    icon: ImageVector,
    iconColor: Color,
    title: String,
    subtitle: String,
    subtitleColor: Color = MaterialTheme.colorScheme.onSurfaceVariant,
    showChevron: Boolean = true,
    onClick: (() -> Unit)? = null,
    trailing: @Composable (() -> Unit)? = null
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .then(if (onClick != null) Modifier.clickable(onClick = onClick) else Modifier)
            .padding(horizontal = 16.dp, vertical = 14.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(14.dp)
    ) {
        // 图标容器
        Box(
            modifier = Modifier
                .size(36.dp)
                .clip(RoundedCornerShape(8.dp))
                .background(iconColor.copy(alpha = 0.1f)),
            contentAlignment = Alignment.Center
        ) {
            Icon(icon, null, tint = iconColor, modifier = Modifier.size(20.dp))
        }

        Column(modifier = Modifier.weight(1f)) {
            Text(
                title,
                style = MaterialTheme.typography.bodyLarge.copy(fontWeight = FontWeight.Medium),
                color = MaterialTheme.colorScheme.onSurface
            )
            Text(
                subtitle,
                style = MaterialTheme.typography.bodySmall,
                color = subtitleColor,
                maxLines = 1
            )
        }

        if (trailing != null) {
            trailing()
            Spacer(Modifier.width(4.dp))
        }

        if (showChevron && onClick != null) {
            Icon(Icons.Default.ChevronRight, null, tint = MaterialTheme.colorScheme.onSurfaceVariant, modifier = Modifier.size(20.dp))
        }
    }
}

/**
 * 赛博朋克分割线
 */
@Composable
private fun CyberDivider() {
    HorizontalDivider(
        modifier = Modifier.padding(horizontal = 16.dp),
        thickness = 0.5.dp,
        color = MaterialTheme.colorScheme.outlineVariant.copy(alpha = 0.5f)
    )
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
            _uiState.value = SettingsUiState(username = username, serverUrl = serverUrl)

            authRepository.getProfile().onSuccess { user ->
                _uiState.value = _uiState.value.copy(username = user.username, role = user.role)
            }
        }

        viewModelScope.launch {
            webSocketManager.connectionState.collect { state ->
                _uiState.value = _uiState.value.copy(wsState = state)
            }
        }

        webSocketManager.connect()

        viewModelScope.launch {
            themePreferences.themeModeFlow.collect { mode ->
                _uiState.value = _uiState.value.copy(themeMode = mode)
            }
        }
    }

    fun setThemeMode(mode: ThemeMode) {
        viewModelScope.launch { themePreferences.setThemeMode(mode) }
    }

    fun logout(onComplete: () -> Unit) {
        viewModelScope.launch {
            webSocketManager.disconnect()
            authRepository.logout()
            onComplete()
        }
    }
}
