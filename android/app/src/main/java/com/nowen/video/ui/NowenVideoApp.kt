package com.nowen.video.ui

import androidx.compose.animation.*
import androidx.compose.animation.core.*
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.*
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.alpha
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.StrokeCap
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
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
import com.nowen.video.ui.theme.*
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
import kotlin.math.cos
import kotlin.math.sin
import kotlin.random.Random

/**
 * 应用根 Composable — 赛博朋克版
 * 包含科幻启动动画 + 主题管理 + 导航
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
                (fadeIn(animationSpec = tween(600)) +
                        scaleIn(initialScale = 0.95f, animationSpec = tween(600))) togetherWith
                        (fadeOut(animationSpec = tween(400)) +
                                scaleOut(targetScale = 1.05f, animationSpec = tween(400)))
            },
            label = "splash_transition"
        ) { isSplash ->
            if (isSplash) {
                CyberSplashScreen()
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

// ==================== 赛博朋克启动画面 ====================

/**
 * 赛博朋克风格 Splash 启动屏
 * - 深空粒子背景
 * - DNA 双螺旋加载动画
 * - 霓虹 Logo 呈现
 * - 扫描线效果
 */
@Composable
private fun CyberSplashScreen() {
    val colorScheme = MaterialTheme.colorScheme
    // 动画进度
    val infiniteTransition = rememberInfiniteTransition(label = "splash")

    // Logo 淡入
    var logoVisible by remember { mutableStateOf(false) }
    LaunchedEffect(Unit) {
        delay(300)
        logoVisible = true
    }

    // 粒子系统
    val particles = remember {
        List(60) {
            SplashParticle(
                x = Random.nextFloat(),
                y = Random.nextFloat(),
                speed = Random.nextFloat() * 0.003f + 0.001f,
                size = Random.nextFloat() * 3f + 1f,
                alpha = Random.nextFloat() * 0.6f + 0.2f
            )
        }
    }

    // 扫描线位置
    val scanLineY by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 1f,
        animationSpec = infiniteRepeatable(
            animation = tween(3000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "scan_line"
    )

    // DNA 螺旋旋转
    val dnaRotation by infiniteTransition.animateFloat(
        initialValue = 0f,
        targetValue = 360f,
        animationSpec = infiniteRepeatable(
            animation = tween(4000, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "dna_rotation"
    )

    // 呼吸光晕
    val glowAlpha by infiniteTransition.animateFloat(
        initialValue = 0.2f,
        targetValue = 0.6f,
        animationSpec = infiniteRepeatable(
            animation = tween(1500, easing = EaseInOutCubic),
            repeatMode = RepeatMode.Reverse
        ),
        label = "glow_alpha"
    )

    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(
                brush = Brush.radialGradient(
                    colors = listOf(
                        colorScheme.surfaceDim,
                        colorScheme.scrim
                    ),
                    radius = 1200f
                )
            ),
        contentAlignment = Alignment.Center
    ) {
        // 粒子背景
        Canvas(modifier = Modifier.fillMaxSize()) {
            particles.forEach { particle ->
                val px = particle.x * size.width
                val py = ((particle.y + scanLineY * particle.speed * 100) % 1f) * size.height
                drawCircle(
                    color = colorScheme.primary.copy(alpha = particle.alpha * 0.5f),
                    radius = particle.size,
                    center = Offset(px, py)
                )
            }

            // 扫描线
            val lineY = scanLineY * size.height
            drawLine(
                brush = Brush.horizontalGradient(
                    colors = listOf(
                        Color.Transparent,
                        colorScheme.primary.copy(alpha = 0.3f),
                        colorScheme.primary.copy(alpha = 0.5f),
                        colorScheme.primary.copy(alpha = 0.3f),
                        Color.Transparent
                    )
                ),
                start = Offset(0f, lineY),
                end = Offset(size.width, lineY),
                strokeWidth = 2f
            )
        }

        // DNA 双螺旋加载动画
        Canvas(
            modifier = Modifier
                .size(120.dp)
                .alpha(0.6f)
        ) {
            val centerX = size.width / 2
            val centerY = size.height / 2
            val radius = size.width * 0.35f
            val nodeCount = 12

            for (i in 0 until nodeCount) {
                val angle = (dnaRotation + i * 30f) * (Math.PI / 180f)
                val x1 = centerX + radius * cos(angle).toFloat()
                val y1 = centerY + (i - nodeCount / 2f) * (size.height / nodeCount)
                val x2 = centerX - radius * cos(angle).toFloat()

                // 链接线
                drawLine(
                    color = colorScheme.primary.copy(alpha = 0.2f + 0.1f * cos(angle).toFloat()),
                    start = Offset(x1, y1),
                    end = Offset(x2, y1),
                    strokeWidth = 1.5f,
                    cap = StrokeCap.Round
                )

                // 节点
                drawCircle(
                    color = colorScheme.primary.copy(alpha = 0.6f + 0.3f * sin(angle).toFloat()),
                    radius = 4f,
                    center = Offset(x1, y1)
                )
                drawCircle(
                    color = colorScheme.secondary.copy(alpha = 0.6f + 0.3f * sin(angle).toFloat()),
                    radius = 4f,
                    center = Offset(x2, y1)
                )
            }
        }

        // Logo 区域
        Column(
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            // 中心光晕
            Canvas(modifier = Modifier.size(80.dp)) {
                drawCircle(
                    brush = Brush.radialGradient(
                        colors = listOf(
                            colorScheme.primary.copy(alpha = glowAlpha),
                            colorScheme.primary.copy(alpha = glowAlpha * 0.3f),
                            Color.Transparent
                        )
                    ),
                    radius = size.width / 2
                )
            }

            Spacer(modifier = Modifier.height(16.dp))

            // 主标题
            AnimatedVisibility(
                visible = logoVisible,
                enter = fadeIn(animationSpec = tween(800)) +
                        slideInVertically(
                            initialOffsetY = { it / 4 },
                            animationSpec = tween(800, easing = EaseOutCubic)
                        )
            ) {
                Text(
                    text = "NOWEN VIDEO",
                    style = MaterialTheme.typography.headlineLarge.copy(
                        fontWeight = FontWeight.Black,
                        fontSize = 32.sp,
                        letterSpacing = 6.sp
                    ),
                    color = colorScheme.primary
                )
            }

            Spacer(modifier = Modifier.height(8.dp))

            // 副标题
            AnimatedVisibility(
                visible = logoVisible,
                enter = fadeIn(animationSpec = tween(800, delayMillis = 300)) +
                        slideInVertically(
                            initialOffsetY = { it / 2 },
                            animationSpec = tween(800, delayMillis = 300, easing = EaseOutCubic)
                        )
            ) {
                Text(
                    text = "YOUR PRIVATE MEDIA CENTER",
                    style = MaterialTheme.typography.labelMedium.copy(
                        letterSpacing = 3.sp
                    ),
                    color = colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

private data class SplashParticle(
    val x: Float,
    val y: Float,
    val speed: Float,
    val size: Float,
    val alpha: Float
)

// ==================== ViewModel ====================

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

            // Splash 展示 1.5 秒后消失（给动画更多时间呈现）
            delay(1500)
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
