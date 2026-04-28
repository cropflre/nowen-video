package com.nowen.video.ui.screen.auth

import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
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
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.repository.AuthRepository
import com.nowen.video.ui.theme.*
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 登录/注册页面 — 赛博朋克风格
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

    LaunchedEffect(Unit) {
        viewModel.loadServerUrl()
        viewModel.checkInitStatus()
    }

    // 光晕动画
    val infiniteTransition = rememberInfiniteTransition(label = "login_bg")
    val glowAlpha by infiniteTransition.animateFloat(
        initialValue = 0.2f, targetValue = 0.5f,
        animationSpec = infiniteRepeatable(
            animation = tween(2500, easing = EaseInOutCubic),
            repeatMode = RepeatMode.Reverse
        ), label = "glow"
    )

    val colorScheme = MaterialTheme.colorScheme

    Box(
        modifier = Modifier
            .fillMaxSize()
            .spaceBackground()
    ) {
        // 装饰性圆环
        Canvas(modifier = Modifier.fillMaxSize()) {
            // 顶部光晕
            drawCircle(
                brush = Brush.radialGradient(
                    colors = listOf(
                        colorScheme.secondary.copy(alpha = glowAlpha * 0.15f),
                        Color.Transparent
                    ),
                    center = Offset(size.width * 0.8f, size.height * 0.1f),
                    radius = size.width * 0.5f
                ),
                center = Offset(size.width * 0.8f, size.height * 0.1f),
                radius = size.width * 0.5f
            )
            // 底部光晕
            drawCircle(
                brush = Brush.radialGradient(
                    colors = listOf(
                        colorScheme.primary.copy(alpha = glowAlpha * 0.1f),
                        Color.Transparent
                    ),
                    center = Offset(size.width * 0.2f, size.height * 0.9f),
                    radius = size.width * 0.4f
                ),
                center = Offset(size.width * 0.2f, size.height * 0.9f),
                radius = size.width * 0.4f
            )
        }

        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(32.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            // Logo — 霓虹发光
            Box(contentAlignment = Alignment.Center) {
                Canvas(modifier = Modifier.size(100.dp)) {
                    drawCircle(
                        brush = Brush.radialGradient(
                            colors = listOf(
                                colorScheme.primary.copy(alpha = glowAlpha * 0.5f),
                                Color.Transparent
                            )
                        ),
                        radius = size.width / 2
                    )
                }
                Icon(
                    imageVector = Icons.Default.PlayCircle,
                    contentDescription = null,
                    modifier = Modifier.size(56.dp),
                    tint = colorScheme.primary
                )
            }

            Spacer(modifier = Modifier.height(16.dp))

            Text(
                text = "NOWEN VIDEO",
                style = MaterialTheme.typography.headlineMedium.copy(
                    fontWeight = FontWeight.Bold,
                    letterSpacing = 3.sp
                ),
                color = colorScheme.onSurface
            )

            Text(
                text = if (uiState.isRegisterMode) "创建管理员账号" else "登录到你的媒体库",
                style = MaterialTheme.typography.bodyMedium,
                color = colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(40.dp))

            // 用户名输入框
            OutlinedTextField(
                value = username,
                onValueChange = { username = it },
                label = { Text("用户名") },
                singleLine = true,
                modifier = Modifier.fillMaxWidth(),
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Next),
                keyboardActions = KeyboardActions(
                    onNext = { focusManager.moveFocus(FocusDirection.Down) }
                ),
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

            Spacer(modifier = Modifier.height(12.dp))

            // 密码输入框
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
                            contentDescription = if (passwordVisible) "隐藏密码" else "显示密码",
                            tint = colorScheme.onSurfaceVariant
                        )
                    }
                },
                keyboardOptions = KeyboardOptions(imeAction = ImeAction.Done),
                keyboardActions = KeyboardActions(
                    onDone = {
                        focusManager.clearFocus()
                        viewModel.submit(username, password, onLoginSuccess)
                    }
                ),
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

            // 错误提示
            if (uiState.error != null) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = uiState.error!!,
                    color = colorScheme.error,
                    style = MaterialTheme.typography.bodySmall
                )
            }

            Spacer(modifier = Modifier.height(24.dp))

            // 登录按钮 — 赛博蓝紫渐变
            Button(
                onClick = { viewModel.submit(username, password, onLoginSuccess) },
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp),
                enabled = username.isNotBlank() && password.isNotBlank() && !uiState.loading,
                shape = CyberButtonShape,
                colors = ButtonDefaults.buttonColors(
                    containerColor = Color.Transparent,
                    contentColor = Color.White,
                    disabledContainerColor = colorScheme.surfaceVariant,
                    disabledContentColor = colorScheme.outline
                ),
                contentPadding = PaddingValues()
            ) {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .background(
                            brush = if (username.isNotBlank() && password.isNotBlank() && !uiState.loading)
                                PrimaryButtonGradient
                            else
                                Brush.horizontalGradient(listOf(colorScheme.surfaceVariant, colorScheme.surfaceVariant)),
                            shape = CyberButtonShape
                        ),
                    contentAlignment = Alignment.Center
                ) {
                    if (uiState.loading) {
                        CircularProgressIndicator(
                            modifier = Modifier.size(20.dp),
                            color = Color.White,
                            strokeWidth = 2.dp
                        )
                    } else {
                        Text(
                            text = if (uiState.isRegisterMode) "创建账号" else "登  录",
                            style = MaterialTheme.typography.labelLarge.copy(
                                fontWeight = FontWeight.Bold,
                                letterSpacing = 1.sp
                            )
                        )
                    }
                }
            }

            // 切换模式
            if (uiState.registrationOpen) {
                Spacer(modifier = Modifier.height(12.dp))
                TextButton(onClick = { viewModel.toggleMode() }) {
                    Text(
                        text = if (uiState.isRegisterMode) "已有账号？去登录" else "没有账号？注册",
                        style = MaterialTheme.typography.bodyMedium,
                        color = colorScheme.primary
                    )
                }
            }

            // 更换服务器
            Spacer(modifier = Modifier.height(8.dp))
            TextButton(onClick = onChangeServer) {
                Text(
                    text = "更换服务器" + if (serverUrl.isNotBlank()) "（$serverUrl）" else "",
                    style = MaterialTheme.typography.bodySmall,
                    color = colorScheme.outline
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
                    isRegisterMode = !initialized
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
