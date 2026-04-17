package com.nowen.video.ui.screen.auth

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowDropDown
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
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

            Spacer(modifier = Modifier.height(48.dp))

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

data class ServerSetupUiState(
    val loading: Boolean = false,
    val error: String? = null
)

@HiltViewModel
class ServerSetupViewModel @Inject constructor(
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(ServerSetupUiState())
    val uiState = _uiState.asStateFlow()

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
