package com.nowen.video.ui.screen.auth

import androidx.compose.foundation.layout.*
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.PlayCircle
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.repository.AuthRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 登录/注册页面
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun LoginScreen(
    onLoginSuccess: () -> Unit,
    onChangeServer: () -> Unit = {},
    viewModel: LoginViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    val focusManager = LocalFocusManager.current

    var username by remember { mutableStateOf("") }
    var password by remember { mutableStateOf("") }
    var passwordVisible by remember { mutableStateOf(false) }

    val serverUrl by viewModel.serverUrl.collectAsState()

    // 检查系统初始化状态
    LaunchedEffect(Unit) {
        viewModel.loadServerUrl()
        viewModel.checkInitStatus()
    }

    Scaffold { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            // Logo
            Icon(
                imageVector = Icons.Default.PlayCircle,
                contentDescription = null,
                modifier = Modifier.size(72.dp),
                tint = MaterialTheme.colorScheme.primary
            )

            Spacer(modifier = Modifier.height(12.dp))

            Text(
                text = "Nowen Video",
                style = MaterialTheme.typography.headlineMedium,
                color = MaterialTheme.colorScheme.onBackground
            )

            Text(
                text = if (uiState.isRegisterMode) "创建管理员账号" else "登录到你的媒体库",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(40.dp))

            // 用户名
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { Text("用户名") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next),
                keyboardActions = KeyboardActions(
                    onNext = { focusManager.moveFocus(FocusDirection.Down) }
                )
            )

            Spacer(modifier = Modifier.height(12.dp))

            // 密码
            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text("密码") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                visualTransformation = if (passwordVisible) VisualTransformation.None
                    else PasswordVisualTransformation(),
                trailingIcon = {
                    IconButton(onClick = { passwordVisible = !passwordVisible }) {
                        Icon(
                            imageVector = if (passwordVisible) Icons.Default.VisibilityOff
                                else Icons.Default.Visibility,
                            contentDescription = if (passwordVisible) "隐藏密码" else "显示密码"
                        )
                    }
                },
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(
                    onDone = {
                        focusManager.clearFocus()
                        viewModel.submit(username, password, onLoginSuccess)
                    }
                )
            )

            // 错误提示
            if (uiState.error != null) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = uiState.error!!,
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodySmall
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            // 登录/注册按钮
            Button(
                onClick = { viewModel.submit(username, password, onLoginSuccess) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp),
                enabled = username.isNotBlank() && password.isNotBlank() && !uiState.loading
            ) {
                if (uiState.loading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        color = MaterialTheme.colorScheme.onPrimary,
                        strokeWidth = 2.dp
                    )
                } else {
                    Text(
                        text = if (uiState.isRegisterMode) "创建账号" else "登录",
                        style = MaterialTheme.typography.labelLarge
                    )
                }
            }

            // 切换登录/注册模式
            if (uiState.registrationOpen) {
                Spacer(modifier = Modifier.height(12.dp))
                TextButton(
                    onClick = { viewModel.toggleMode() }
                ) {
                    Text(
                        text = if (uiState.isRegisterMode) "已有账号？去登录" else "没有账号？注册",
                        style = MaterialTheme.typography.bodyMedium
                    )
                }
            }

            // 更换服务器按钮
            Spacer(modifier = Modifier.height(8.dp))
            TextButton(
                onClick = onChangeServer
            ) {
                Text(
                    text = "更换服务器" + if (serverUrl.isNotBlank()) "（$serverUrl）" else "",
                    style = MaterialTheme.typography.bodySmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

data class LoginUiState(
    val loading: Boolean = false,
    val error: String? = null,
    val isRegisterMode: Boolean = false,
    val registrationOpen: Boolean = false,
    val initialized: Boolean = true
)

@HiltViewModel
class LoginViewModel @Inject constructor(
    private val authRepository: AuthRepository,
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(LoginUiState())
    val uiState = _uiState.asStateFlow()

    private val _serverUrl = MutableStateFlow("")
    val serverUrl = _serverUrl.asStateFlow()

    fun loadServerUrl() {
        viewModelScope.launch {
            _serverUrl.value = tokenManager.getServerUrl() ?: ""
        }
    }

    fun checkInitStatus() {
        viewModelScope.launch {
            authRepository.getInitStatus().onSuccess { (initialized, registrationOpen) ->
                _uiState.value = _uiState.value.copy(
                    initialized = initialized,
                    registrationOpen = registrationOpen,
                    isRegisterMode = !initialized // 未初始化时自动切换到注册模式
                )
            }
        }
    }

    fun toggleMode() {
        _uiState.value = _uiState.value.copy(
            isRegisterMode = !_uiState.value.isRegisterMode,
            error = null
        )
    }

    fun submit(username: String, password: String, onSuccess: () -> Unit) {
        if (username.isBlank() || password.isBlank()) {
            _uiState.value = _uiState.value.copy(error = "请输入用户名和密码")
            return
        }

        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true, error = null)

            val result = if (_uiState.value.isRegisterMode) {
                authRepository.register(username, password)
            } else {
                authRepository.login(username, password)
            }

            result.onSuccess {
                _uiState.value = _uiState.value.copy(loading = false)
                onSuccess()
            }.onFailure { e ->
                _uiState.value = _uiState.value.copy(
                    loading = false,
                    error = e.message ?: "操作失败"
                )
            }
        }
    }
}
