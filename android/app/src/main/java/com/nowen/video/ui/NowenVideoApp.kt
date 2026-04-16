package com.nowen.video.ui

import androidx.compose.animation.*
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.navigation.compose.rememberNavController
import com.nowen.video.data.local.ThemeMode
import com.nowen.video.data.local.ThemePreferences
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.remote.WebSocketManager
import com.nowen.video.ui.navigation.NowenNavGraph
import com.nowen.video.ui.navigation.Screen
import com.nowen.video.ui.theme.NowenVideoTheme
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch
import javax.inject.Inject

/**
 * 应用根 Composable — 包含 Splash 启动屏 + 主题管理 + 导航
 */
@Composable
fun NowenVideoApp(
    viewModel: AppViewModel = hiltViewModel()
) {
    val startDestination by viewModel.startDestination.collectAsState()
    val themeMode by viewModel.themeMode.collectAsState()
    val showSplash by viewModel.showSplash.collectAsState()

    NowenVideoTheme(themeMode = themeMode) {
        AnimatedContent(
            targetState = showSplash,
            transitionSpec = {
                fadeIn(animationSpec = tween(500)) togetherWith
                        fadeOut(animationSpec = tween(500))
            },
            label = "splash_transition"
        ) { isSplash ->
            if (isSplash) {
                // Splash 启动屏
                SplashScreen()
            } else if (startDestination != null) {
                val navController = rememberNavController()

                // 监听认证失效事件，自动跳转到登录页
                LaunchedEffect(Unit) {
                    viewModel.authExpiredEvent.collect {
                        navController.navigate(Screen.Login.route) {
                            popUpTo(0) { inclusive = true }
                        }
                    }
                }

                NowenNavGraph(
                    navController = navController,
                    startDestination = startDestination!!
                )
            }
        }
    }
}

/**
 * Splash 启动屏 — 品牌展示
 */
@Composable
private fun SplashScreen() {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Text(
            text = "Nowen Video",
            style = MaterialTheme.typography.headlineLarge.copy(
                fontWeight = FontWeight.Bold,
                fontSize = 32.sp,
                letterSpacing = 2.sp
            ),
            color = MaterialTheme.colorScheme.primary
        )
    }
}

/**
 * 应用级 ViewModel — 判断启动路由 + 管理主题 + WebSocket 生命周期 + 认证状态监听
 */
@HiltViewModel
class AppViewModel @Inject constructor(
    private val tokenManager: TokenManager,
    private val themePreferences: ThemePreferences,
    private val webSocketManager: WebSocketManager
) : ViewModel() {

    private val _startDestination = MutableStateFlow<String?>(null)
    val startDestination = _startDestination.asStateFlow()

    private val _showSplash = MutableStateFlow(true)
    val showSplash = _showSplash.asStateFlow()

    /** 认证失效事件 — 当 Token 被拦截器清除时触发 */
    private val _authExpiredEvent = MutableSharedFlow<Unit>()
    val authExpiredEvent = _authExpiredEvent.asSharedFlow()

    /**
     * 主题模式 — 从 DataStore 实时读取
     */
    val themeMode = themePreferences.themeModeFlow.stateIn(
        scope = viewModelScope,
        started = SharingStarted.WhileSubscribed(5_000),
        initialValue = ThemeMode.SYSTEM
    )

    init {
        viewModelScope.launch {
            val serverUrl = tokenManager.getServerUrl()
            val token = tokenManager.getToken()

            _startDestination.value = when {
                serverUrl.isNullOrBlank() -> Screen.ServerSetup.route
                token.isNullOrBlank() -> Screen.Login.route
                else -> {
                    // 已登录，自动连接 WebSocket
                    webSocketManager.connect()
                    Screen.Home.route
                }
            }

            // Splash 展示 1 秒后消失
            delay(1000)
            _showSplash.value = false
        }

        // 监听 Token 变化：当 Token 被清除（401 拦截器触发）时，发送认证失效事件
        viewModelScope.launch {
            var wasLoggedIn = false
            tokenManager.isLoggedInFlow().collect { isLoggedIn ->
                if (wasLoggedIn && !isLoggedIn) {
                    // 从已登录变为未登录 → Token 失效
                    webSocketManager.disconnect()
                    _authExpiredEvent.emit(Unit)
                }
                wasLoggedIn = isLoggedIn
            }
        }
    }

    override fun onCleared() {
        super.onCleared()
        webSocketManager.disconnect()
    }
}
