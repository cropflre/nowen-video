package com.nowen.video.ui.screen.auth

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.discovery.DiscoveredServer
import com.nowen.video.data.discovery.DiscoverySource
import com.nowen.video.data.discovery.DiscoveryState
import com.nowen.video.data.discovery.ServerDiscoveryManager
import com.nowen.video.data.local.TokenManager
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject
import kotlin.random.Random

/**
 * 服务器配置页面 — 赛博朋克风格
 * 支持协议选择（http/https）+ 服务器地址 + 端口号
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServerSetupScreen(
    onServerConfigured: () -> Unit,
    viewModel: ServerSetupViewModel = hiltViewModel()
) {
    var protocol by remember { mutableStateOf("http") }
    var serverHost by remember { mutableStateOf("") }
    var serverPort by remember { mutableStateOf("8080") }
    var protocolExpanded by remember { mutableStateOf(false) }
    val uiState by viewModel.uiState.collectAsState()

    val protocols = listOf("http", "https")

    // 粒子动画
    val infiniteTransition = rememberInfiniteTransition(label = "server_bg")
    val glowAlpha by infiniteTransition.animateFloat(
        initialValue = 0.3f, targetValue = 0.7f,
        animationSpec = infiniteRepeatable(
            animation = tween(2000, easing = EaseInOutCubic),
            repeatMode = RepeatMode.Reverse
        ), label = "glow"
    )

    // 构建完整URL的辅助函数
    fun buildFullUrl(): String {
        val host = serverHost.trim()
        val port = serverPort.trim()
        return if (port.isNotBlank()) {
            "$protocol://$host:$port"
        } else {
            "$protocol://$host"
        }
    }

    val colorScheme = MaterialTheme.colorScheme
    Box(
        modifier = Modifier
            .fillMaxSize()
            .spaceBackground()
    ) {
        // 背景网格线
        Canvas(modifier = Modifier.fillMaxSize()) {
            val gridSize = 40.dp.toPx()
            var x = 0f
            while (x < size.width) {
                drawLine(
                    color = colorScheme.primary.copy(alpha = 0.03f),
                    start = Offset(x, 0f), end = Offset(x, size.height), strokeWidth = 1f
                )
                x += gridSize
            }
            var y = 0f
            while (y < size.height) {
                drawLine(
                    color = colorScheme.primary.copy(alpha = 0.03f),
                    start = Offset(0f, y), end = Offset(size.width, y), strokeWidth = 1f
                )
                y += gridSize
            }
        }

        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            // Logo 区域 — 带光晕
            Box(contentAlignment = Alignment.Center) {
                Canvas(modifier = Modifier.size(120.dp)) {
                    drawCircle(
                        brush = Brush.radialGradient(
                            colors = listOf(
                                colorScheme.primary.copy(alpha = glowAlpha * 0.4f),
                                Color.Transparent
                            )
                        ),
                        radius = size.width / 2
                    )
                }
                Icon(
                    imageVector = Icons.Default.Dns,
                    contentDescription = null,
                    modifier = Modifier.size(64.dp),
                    tint = colorScheme.primary
                )
            }

            Spacer(modifier = Modifier.height(20.dp))

            Text(
                text = "NOWEN VIDEO",
                style = MaterialTheme.typography.headlineLarge.copy(
                    fontWeight = FontWeight.Black,
                    letterSpacing = 4.sp
                ),
                color = colorScheme.onSurface
            )

            Spacer(modifier = Modifier.height(4.dp))

            Text(
                text = "连接到你的媒体服务器",
                style = MaterialTheme.typography.bodyLarge,
                color = colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(32.dp))

            // 🔍 扫描局域网服务器按钮
            val discoveryState by viewModel.discoveryState.collectAsState()
            var showDiscoverySheet by remember { mutableStateOf(false) }

            OutlinedButton(
                onClick = {
                    showDiscoverySheet = true
                    viewModel.startDiscovery()
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(48.dp),
                shape = CyberButtonShape,
                colors = ButtonDefaults.outlinedButtonColors(
                    contentColor = colorScheme.primary
                ),
                border = ButtonDefaults.outlinedButtonBorder(enabled = true).copy(
                    brush = Brush.horizontalGradient(
                        colors = listOf(
                            colorScheme.primary.copy(alpha = 0.6f),
                            colorScheme.secondary.copy(alpha = 0.4f)
                        )
                    )
                )
            ) {
                Icon(
                    imageVector = Icons.Default.Wifi,
                    contentDescription = null,
                    modifier = Modifier.size(20.dp)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(
                    "扫描局域网服务器",
                    style = MaterialTheme.typography.labelLarge.copy(
                        fontWeight = FontWeight.Medium,
                        letterSpacing = 0.5.sp
                    )
                )
            }

            // 发现结果 BottomSheet
            if (showDiscoverySheet) {
                DiscoveryBottomSheet(
                    discoveryState = discoveryState,
                    onServerSelected = { server ->
                        // 自动填充表单
                        val url = java.net.URL(server.url)
                        protocol = url.protocol
                        serverHost = server.host
                        serverPort = server.port.toString()
                        showDiscoverySheet = false
                        viewModel.stopDiscovery()
                    },
                    onDismiss = {
                        showDiscoverySheet = false
                        viewModel.stopDiscovery()
                    },
                    onRefresh = {
                        viewModel.startDiscovery()
                    }
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            // 分割线
            Row(
                modifier = Modifier.fillMaxWidth(),
                verticalAlignment = Alignment.CenterVertically
            ) {
                HorizontalDivider(
                    modifier = Modifier.weight(1f),
                    color = colorScheme.outline.copy(alpha = 0.3f)
                )
                Text(
                    text = "或手动输入",
                    style = MaterialTheme.typography.bodySmall,
                    color = colorScheme.outline,
                    modifier = Modifier.padding(horizontal = 16.dp)
                )
                HorizontalDivider(
                    modifier = Modifier.weight(1f),
                    color = colorScheme.outline.copy(alpha = 0.3f)
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            // 协议选择 + 服务器地址（同一行）
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
                verticalAlignment = Alignment.Top
            ) {
                // 协议选择下拉框
                ExposedDropdownMenuBox(
                    expanded = protocolExpanded,
                    onExpandedChange = { protocolExpanded = it },
                    modifier = Modifier.width(130.dp)
                ) {
                    OutlinedTextField(
                        value = "$protocol://",
                        onValueChange = {},
                        readOnly = true,
                        singleLine = true,
                        label = { Text("协议") },
                        trailingIcon = {
                            Icon(
                                Icons.Default.ArrowDropDown,
                                contentDescription = "选择协议",
                                tint = colorScheme.primary
                            )
                        },
                        modifier = Modifier.menuAnchor(),
                        colors = OutlinedTextFieldDefaults.colors(
                            focusedBorderColor = colorScheme.primary,
                            unfocusedBorderColor = colorScheme.outline,
                            cursorColor = colorScheme.primary,
                            focusedLabelColor = colorScheme.primary,
                            focusedTextColor = colorScheme.onSurface,
                            unfocusedTextColor = colorScheme.onSurface,
                        ),
                        shape = RoundedCornerShape(12.dp)
                    )
                    ExposedDropdownMenu(
                        expanded = protocolExpanded,
                        onDismissRequest = { protocolExpanded = false },
                        containerColor = colorScheme.surface
                    ) {
                        protocols.forEach { proto ->
                            DropdownMenuItem(
                                text = {
                                    Text(
                                        "$proto://",
                                        color = if (proto == protocol) colorScheme.primary else colorScheme.onSurface
                                    )
                                },
                                onClick = {
                                    protocol = proto
                                    protocolExpanded = false
                                }
                            )
                        }
                    }
                }

                // 服务器地址输入
                OutlinedTextField(
                    value = serverHost,
                    onValueChange = { serverHost = it },
                    label = { Text("服务器地址") },
                    placeholder = { Text("192.168.1.100") },
                    singleLine = true,
                    modifier = Modifier.weight(1f),
                    keyboardOptions = KeyboardOptions(
                        keyboardType = KeyboardType.Uri,
                        imeAction = ImeAction.Next
                    ),
                    isError = uiState.error != null,
                    colors = OutlinedTextFieldDefaults.colors(
                        focusedBorderColor = colorScheme.primary,
                        unfocusedBorderColor = colorScheme.outline,
                        cursorColor = colorScheme.primary,
                        focusedLabelColor = colorScheme.primary,
                        focusedTextColor = colorScheme.onSurface,
                        unfocusedTextColor = colorScheme.onSurface,
                    ),
                    shape = RoundedCornerShape(12.dp)
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            // 端口号输入
            OutlinedTextField(
                value = serverPort,
                onValueChange = { newValue ->
                    // 只允许输入数字，最多5位
                    if (newValue.all { it.isDigit() } && newValue.length <= 5) {
                        serverPort = newValue
                    }
                },
                label = { Text("端口号") },
                placeholder = { Text("8080") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Number,
                    imeAction = ImeAction.Done
                ),
                keyboardActions = KeyboardActions(
                    onDone = {
                        viewModel.saveServerUrl(buildFullUrl(), onServerConfigured)
                    }
                ),
                isError = uiState.error != null,
                colors = OutlinedTextFieldDefaults.colors(
                    focusedBorderColor = colorScheme.primary,
                    unfocusedBorderColor = colorScheme.outline,
                    cursorColor = colorScheme.primary,
                    focusedLabelColor = colorScheme.primary,
                    focusedTextColor = colorScheme.onSurface,
                    unfocusedTextColor = colorScheme.onSurface,
                ),
                shape = RoundedCornerShape(12.dp),
                supportingText = {
                    Text(
                        "留空则使用默认端口（HTTP:80 / HTTPS:443）",
                        color = colorScheme.outline,
                        style = MaterialTheme.typography.bodySmall
                    )
                }
            )

            if (uiState.error != null) {
                Text(
                    text = uiState.error!!,
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(top = 4.dp)
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            // 连接按钮 — 霓虹渐变
            Button(
                onClick = { viewModel.saveServerUrl(buildFullUrl(), onServerConfigured) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp),
                enabled = serverHost.isNotBlank() && !uiState.loading,
                shape = CyberButtonShape,
                colors = ButtonDefaults.buttonColors(
                    containerColor = colorScheme.primary,
                    contentColor = colorScheme.scrim
                )
            ) {
                if (uiState.loading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        color = colorScheme.scrim,
                        strokeWidth = 2.dp
                    )
                } else {
                    Text(
                        "连接服务器",
                        style = MaterialTheme.typography.labelLarge.copy(
                            fontWeight = FontWeight.Bold,
                            letterSpacing = 1.sp
                        )
                    )
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            // 当前将要连接的完整地址预览
            if (serverHost.isNotBlank()) {
                Text(
                    text = "将连接到: ${buildFullUrl()}",
                    style = MaterialTheme.typography.bodySmall,
                    color = colorScheme.primary.copy(alpha = 0.7f),
                    modifier = Modifier.padding(horizontal = 16.dp)
                )
            } else {
                Text(
                    text = "请输入服务器地址和端口号",
                    style = MaterialTheme.typography.bodySmall,
                    color = colorScheme.outline,
                    modifier = Modifier.padding(horizontal = 16.dp)
                )
            }
        }
    }
}

/**
 * 发现结果 BottomSheet
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun DiscoveryBottomSheet(
    discoveryState: DiscoveryState,
    onServerSelected: (DiscoveredServer) -> Unit,
    onDismiss: () -> Unit,
    onRefresh: () -> Unit
) {
    val sheetState = rememberModalBottomSheetState(skipPartiallyExpanded = true)
    val colorScheme = MaterialTheme.colorScheme

    ModalBottomSheet(
        onDismissRequest = onDismiss,
        sheetState = sheetState,
        containerColor = colorScheme.surfaceContainer,
        shape = RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp)
    ) {
        Column(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 24.dp)
                .padding(bottom = 32.dp)
        ) {
            // 标题栏
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Icon(
                        imageVector = Icons.Default.Sensors,
                        contentDescription = null,
                        tint = colorScheme.primary,
                        modifier = Modifier.size(24.dp)
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    Text(
                        text = "发现的服务器",
                        style = MaterialTheme.typography.titleMedium.copy(
                            fontWeight = FontWeight.Bold
                        ),
                        color = colorScheme.onSurface
                    )
                }

                // 刷新按钮
                IconButton(
                    onClick = onRefresh,
                    enabled = !discoveryState.isScanning
                ) {
                    Icon(
                        imageVector = Icons.Default.Refresh,
                        contentDescription = "重新扫描",
                        tint = if (discoveryState.isScanning) colorScheme.outline else colorScheme.primary
                    )
                }
            }

            Spacer(modifier = Modifier.height(8.dp))

            // 扫描状态提示
            if (discoveryState.isScanning) {
                Row(
                    verticalAlignment = Alignment.CenterVertically,
                    modifier = Modifier.padding(vertical = 4.dp)
                ) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(14.dp),
                        color = colorScheme.primary,
                        strokeWidth = 2.dp
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                    Text(
                        text = "正在扫描局域网... 已发现 ${discoveryState.servers.size} 台服务器",
                        style = MaterialTheme.typography.bodySmall,
                        color = colorScheme.onSurfaceVariant
                    )
                }
                Spacer(modifier = Modifier.height(4.dp))
                LinearProgressIndicator(
                    modifier = Modifier
                        .fillMaxWidth()
                        .height(2.dp)
                        .clip(RoundedCornerShape(1.dp)),
                    color = colorScheme.primary,
                    trackColor = colorScheme.surfaceContainerHighest
                )
            }

            Spacer(modifier = Modifier.height(12.dp))

            // 错误提示
            if (discoveryState.error != null) {
                Text(
                    text = discoveryState.error,
                    style = MaterialTheme.typography.bodySmall,
                    color = colorScheme.error,
                    modifier = Modifier.padding(bottom = 8.dp)
                )
            }

            // 服务器列表
            if (discoveryState.servers.isEmpty() && !discoveryState.isScanning) {
                // 空状态
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(vertical = 32.dp),
                    horizontalAlignment = Alignment.CenterHorizontally
                ) {
                    Icon(
                        imageVector = Icons.Default.SearchOff,
                        contentDescription = null,
                        modifier = Modifier.size(48.dp),
                        tint = colorScheme.outline
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    Text(
                        text = "未发现局域网内的服务器",
                        style = MaterialTheme.typography.bodyMedium,
                        color = colorScheme.outline
                    )
                    Spacer(modifier = Modifier.height(4.dp))
                    Text(
                        text = "请确保服务器已启动且与手机在同一网络",
                        style = MaterialTheme.typography.bodySmall,
                        color = colorScheme.outline.copy(alpha = 0.7f)
                    )
                }
            } else {
                LazyColumn(
                    modifier = Modifier
                        .fillMaxWidth()
                        .heightIn(max = 300.dp),
                    verticalArrangement = Arrangement.spacedBy(8.dp)
                ) {
                    items(discoveryState.servers) { server ->
                        DiscoveredServerItem(
                            server = server,
                            onClick = { onServerSelected(server) }
                        )
                    }
                }
            }
        }
    }
}

/**
 * 单个发现的服务器条目
 */
@Composable
private fun DiscoveredServerItem(
    server: DiscoveredServer,
    onClick: () -> Unit
) {
    val colorScheme = MaterialTheme.colorScheme

    Card(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick),
        shape = RoundedCornerShape(12.dp),
        colors = CardDefaults.cardColors(
            containerColor = colorScheme.surfaceContainerHigh
        ),
        elevation = CardDefaults.cardElevation(defaultElevation = 0.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(16.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            // 服务器图标
            Box(
                modifier = Modifier
                    .size(44.dp)
                    .clip(RoundedCornerShape(10.dp))
                    .background(
                        brush = Brush.linearGradient(
                            colors = listOf(
                                colorScheme.primary.copy(alpha = 0.15f),
                                colorScheme.secondary.copy(alpha = 0.1f)
                            )
                        )
                    ),
                contentAlignment = Alignment.Center
            ) {
                Icon(
                    imageVector = Icons.Default.Dns,
                    contentDescription = null,
                    tint = colorScheme.primary,
                    modifier = Modifier.size(22.dp)
                )
            }

            Spacer(modifier = Modifier.width(14.dp))

            // 服务器信息
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = server.name,
                    style = MaterialTheme.typography.bodyLarge.copy(
                        fontWeight = FontWeight.SemiBold
                    ),
                    color = colorScheme.onSurface,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                Spacer(modifier = Modifier.height(2.dp))
                Row(verticalAlignment = Alignment.CenterVertically) {
                    Text(
                        text = "${server.host}:${server.port}",
                        style = MaterialTheme.typography.bodySmall,
                        color = colorScheme.onSurfaceVariant
                    )
                    if (server.version.isNotBlank()) {
                        Text(
                            text = " · v${server.version}",
                            style = MaterialTheme.typography.bodySmall,
                            color = colorScheme.outline
                        )
                    }
                }
            }

            // 发现来源标签
            Surface(
                shape = RoundedCornerShape(6.dp),
                color = when (server.source) {
                    DiscoverySource.MDNS -> colorScheme.primaryContainer
                    DiscoverySource.HTTP_SWEEP -> colorScheme.secondaryContainer
                }
            ) {
                Text(
                    text = when (server.source) {
                        DiscoverySource.MDNS -> "mDNS"
                        DiscoverySource.HTTP_SWEEP -> "扫描"
                    },
                    style = MaterialTheme.typography.labelSmall,
                    color = when (server.source) {
                        DiscoverySource.MDNS -> colorScheme.onPrimaryContainer
                        DiscoverySource.HTTP_SWEEP -> colorScheme.onSecondaryContainer
                    },
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 3.dp)
                )
            }

            Spacer(modifier = Modifier.width(8.dp))

            Icon(
                imageVector = Icons.Default.ChevronRight,
                contentDescription = "选择",
                tint = colorScheme.outline,
                modifier = Modifier.size(20.dp)
            )
        }
    }
}

data class ServerSetupUiState(
    val loading: Boolean = false,
    val error: String? = null
)

@HiltViewModel
class ServerSetupViewModel @Inject constructor(
    private val tokenManager: TokenManager,
    private val discoveryManager: ServerDiscoveryManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(ServerSetupUiState())
    val uiState = _uiState.asStateFlow()

    /** 发现状态（来自 ServerDiscoveryManager） */
    val discoveryState = discoveryManager.discoveryState

    /** 开始局域网设备发现 */
    fun startDiscovery() {
        discoveryManager.startDiscovery(viewModelScope)
    }

    /** 停止局域网设备发现 */
    fun stopDiscovery() {
        discoveryManager.stopDiscovery()
    }

    fun saveServerUrl(url: String, onSuccess: () -> Unit) {
        val trimmedUrl = url.replace(Regex("[^\\x20-\\x7E]"), "").trim().trimEnd('/')
        if (trimmedUrl.isBlank() || (!trimmedUrl.startsWith("http://") && !trimmedUrl.startsWith("https://"))) {
            _uiState.value = _uiState.value.copy(error = "请输入有效的服务器地址（以 http:// 或 https:// 开头）")
            return
        }

        // 验证地址中是否包含有效的 host 部分
        val hostPart = trimmedUrl.removePrefix("http://").removePrefix("https://").split(":").firstOrNull()
        if (hostPart.isNullOrBlank()) {
            _uiState.value = _uiState.value.copy(error = "服务器地址不能为空")
            return
        }

        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, error = null)
            try {
                tokenManager.saveServerUrl(trimmedUrl)
                onSuccess()
            } catch (e: Exception) {
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    error = "保存失败: ${e.message}"
                )
            }
        }
    }
}
