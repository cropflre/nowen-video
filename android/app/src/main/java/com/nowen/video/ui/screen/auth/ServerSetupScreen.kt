package com.nowen.video.ui.screen.auth

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.TokenManager
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 服务器配置页面 — 首次启动时配置服务器地址
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServerSetupScreen(
    onServerConfigured: () -> Unit,
    viewModel: ServerSetupViewModel = hiltViewModel()
) {
    var serverUrl by remember { mutableStateOf("http://") }
    val uiState by viewModel.uiState.collectAsState()

    Scaffold { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            // Logo 区域
            Icon(
                imageVector = Icons.Default.Dns,
                contentDescription = null,
                modifier = Modifier.size(80.dp),
                tint = MaterialTheme.colorScheme.primary
            )

            Spacer(modifier = Modifier.height(16.dp))

            Text(
                text = "Nowen Video",
                style = MaterialTheme.typography.headlineLarge,
                color = MaterialTheme.colorScheme.onBackground
            )

            Text(
                text = "连接到你的媒体服务器",
                style = MaterialTheme.typography.bodyLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(48.dp))

            // 服务器地址输入
            OutlinedTextField(
                value = serverUrl,
                onValueChange = { serverUrl = it },
                label = { Text("服务器地址") },
                placeholder = { Text("http://192.168.1.100:8080") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Uri,
                    imeAction = ImeAction.Done
                ),
                keyboardActions = KeyboardActions(
                    onDone = {
                        viewModel.saveServerUrl(serverUrl, onServerConfigured)
                    }
                ),
                isError = uiState.error != null
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

            // 连接按钮
            Button(
                onClick = { viewModel.saveServerUrl(serverUrl, onServerConfigured) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp),
                enabled = serverUrl.isNotBlank() && !uiState.loading
            ) {
                if (uiState.loading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        color = MaterialTheme.colorScheme.onPrimary,
                        strokeWidth = 2.dp
                    )
                } else {
                    Text("连接服务器", style = MaterialTheme.typography.labelLarge)
                }
            }

            Spacer(modifier = Modifier.height(16.dp))

            Text(
                text = "请输入 Nowen Video 服务器的完整地址\n包含协议和端口号",
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.padding(horizontal = 16.dp)
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
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(ServerSetupUiState())
    val uiState = _uiState.asStateFlow()

    fun saveServerUrl(url: String, onSuccess: () -> Unit) {
        // 清理非 ASCII 可见字符（防止输入法注入不可见 Unicode 字符）
        val trimmedUrl = url.replace(Regex("[^\\x20-\\x7E]"), "").trim().trimEnd('/')
        if (trimmedUrl.isBlank() || (!trimmedUrl.startsWith("http://") && !trimmedUrl.startsWith("https://"))) {
            _uiState.value = _uiState.value.copy(error = "请输入有效的服务器地址（以 http:// 或 https:// 开头）")
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
