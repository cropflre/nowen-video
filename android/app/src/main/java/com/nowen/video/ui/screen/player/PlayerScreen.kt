package com.nowen.video.ui.screen.player

import android.app.Activity
import android.content.Context
import android.media.AudioManager
import android.media.MediaCodecList
import android.provider.Settings
import android.view.GestureDetector
import android.view.MotionEvent
import android.view.ViewGroup
import android.view.WindowManager
import android.widget.FrameLayout
import androidx.annotation.OptIn
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.tween
import androidx.compose.animation.fadeIn
import androidx.compose.animation.fadeOut
import androidx.compose.animation.slideInVertically
import androidx.compose.animation.slideOutVertically
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectTapGestures
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.util.UnstableApi
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.ui.AspectRatioFrameLayout
import androidx.media3.ui.PlayerView
import com.nowen.video.data.local.PlayerPreferences
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.Media
import com.nowen.video.data.model.StreamInfo
import com.nowen.video.data.model.SubtitleTrack
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import kotlinx.coroutines.runBlocking
import javax.inject.Inject
import android.util.Log
import kotlin.math.abs

private const val TAG = "PlayerDebug"

// ==================== 主题色常量 ====================
private val NeonBlue = Color(0xFF00D4FF)
private val NeonBlueDim = Color(0xFF00D4FF).copy(alpha = 0.6f)
private val ControlBg = Color(0xFF0B1120)
private val ControlBgAlpha = Color(0xFF0B1120).copy(alpha = 0.85f)

/**
 * 视频播放器页面
 * 参考 Web 端 Emby 风格设计，完全自定义控制 UI
 * 支持四种播放模式：Direct / Remux / HLS / Preprocessed HLS
 * 播放失败自动降级 + 字幕选择 + 手势控制 + 播放设置面板
 */
@kotlin.OptIn(ExperimentalMaterial3Api::class)
@OptIn(UnstableApi::class)
@Composable
fun PlayerScreen(
    mediaId: String,
    onBack: () -> Unit,
    viewModel: PlayerViewModel = hiltViewModel()
) {
    val uiState by viewModel.uiState.collectAsState()
    val context = LocalContext.current
    val activity = context as? Activity

    // ==================== 控制栏显示状态 ====================
    var showControls by remember { mutableStateOf(true) }
    var isLocked by remember { mutableStateOf(false) }

    // ==================== 播放状态 ====================
    var isPlaying by remember { mutableStateOf(false) }
    var currentPosition by remember { mutableLongStateOf(0L) }
    var totalDuration by remember { mutableLongStateOf(0L) }
    var bufferedPosition by remember { mutableLongStateOf(0L) }
    var isSeeking by remember { mutableStateOf(false) }
    var seekPosition by remember { mutableLongStateOf(0L) }

    // ==================== 手势状态 ====================
    var gestureInfo by remember { mutableStateOf<String?>(null) }
    var showSubtitlePicker by remember { mutableStateOf(false) }

    // ==================== 倍速播放状态 ====================
    var currentSpeed by remember { mutableFloatStateOf(1f) }
    val speedOptions = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f, 2.5f, 3f, 4f, 5f, 6f, 7f, 8f)

    // ==================== 播放设置面板状态 ====================
    var showSettingsPanel by remember { mutableStateOf(false) }
    var settingsCategory by remember { mutableStateOf<String?>(null) }

    // ==================== 画面比例状态 ====================
    var currentAspectRatio by remember { mutableIntStateOf(0) }
    val aspectRatioLabels = listOf("自适应", "填充屏幕", "16:9", "4:3", "原始")

    // ==================== 解码器状态 ====================
    var currentDecoder by remember { mutableIntStateOf(0) }
    val decoderLabels = listOf("自动", "硬件解码优先", "软件解码优先")

    // PlayerView 引用
    var playerViewRef by remember { mutableStateOf<PlayerView?>(null) }

    // 加载偏好设置
    LaunchedEffect(Unit) {
        currentSpeed = viewModel.getPlaybackSpeed()
        currentAspectRatio = viewModel.getAspectRatio()
        currentDecoder = viewModel.getDecoderPriority()
    }

    // 创建 ExoPlayer（注入 Authorization Header，解决流媒体请求 401 问题）
    val exoPlayer = remember {
        Log.d(TAG, "创建 ExoPlayer, token长度=${viewModel.token.length}, token前10=${viewModel.token.take(10)}")
        // 创建带认证头的 HttpDataSource
        val httpDataSourceFactory = DefaultHttpDataSource.Factory()
            .setDefaultRequestProperties(
                mapOf("Authorization" to "Bearer ${viewModel.token}")
            )
        val dataSourceFactory = DefaultDataSource.Factory(context, httpDataSourceFactory)
        val mediaSourceFactory = DefaultMediaSourceFactory(dataSourceFactory)

        ExoPlayer.Builder(context)
            .setMediaSourceFactory(mediaSourceFactory)
            .build()
            .apply {
                playWhenReady = true
            }
    }

    // 应用保存的播放速度
    LaunchedEffect(currentSpeed) {
        exoPlayer.setPlaybackSpeed(currentSpeed)
    }

    // 加载流信息和字幕
    LaunchedEffect(mediaId) {
        viewModel.loadStreamInfo(mediaId)
        viewModel.loadSubtitleTracks(mediaId)
    }

    // 设置播放源（含字幕）
    LaunchedEffect(uiState.playbackUrl) {
        val url = uiState.playbackUrl ?: return@LaunchedEffect

        val mediaItemBuilder = MediaItem.Builder().setUri(url)

        when (uiState.playbackMode) {
            PlaybackMode.HLS, PlaybackMode.PREPROCESSED_HLS -> {
                mediaItemBuilder.setMimeType(MimeTypes.APPLICATION_M3U8)
            }
            else -> {}
        }

        // 添加外挂字幕轨道
        val subtitleConfigs = uiState.subtitleTracks
            .filter { it.isExternal }
            .map { track ->
                val subtitleUrl = "${viewModel.serverUrl}/api/subtitle/external?path=${track.filePath}&token=${viewModel.token}"
                MediaItem.SubtitleConfiguration.Builder(android.net.Uri.parse(subtitleUrl))
                    .setMimeType(MimeTypes.APPLICATION_SUBRIP)
                    .setLanguage(track.language)
                    .setLabel(track.title.ifBlank { track.language })
                    .setSelectionFlags(if (track.isDefault) C.SELECTION_FLAG_DEFAULT else 0)
                    .build()
            }

        if (subtitleConfigs.isNotEmpty()) {
            mediaItemBuilder.setSubtitleConfigurations(subtitleConfigs)
        }

        exoPlayer.setMediaItem(mediaItemBuilder.build())
        exoPlayer.prepare()

        // 恢复播放进度
        if (uiState.resumePosition > 0) {
            exoPlayer.seekTo((uiState.resumePosition * 1000).toLong())
        }
    }

    // 监听播放错误 — 自动降级
    DisposableEffect(exoPlayer) {
        val listener = object : Player.Listener {
            override fun onPlayerError(error: PlaybackException) {
                viewModel.onPlaybackError(error)
            }
        }
        exoPlayer.addListener(listener)
        onDispose {
            exoPlayer.removeListener(listener)
        }
    }

    // 定期保存播放进度
    LaunchedEffect(exoPlayer) {
        while (true) {
            delay(10_000)
            if (exoPlayer.isPlaying) {
                val position = exoPlayer.currentPosition / 1000.0
                val duration = exoPlayer.duration / 1000.0
                if (duration > 0) {
                    viewModel.saveProgress(mediaId, position, duration)
                }
            }
        }
    }

    // 手势提示自动隐藏
    LaunchedEffect(gestureInfo) {
        if (gestureInfo != null) {
            delay(1500)
            gestureInfo = null
        }
    }

    // 监听播放状态变化（播放/暂停、进度、时长）
    DisposableEffect(exoPlayer) {
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(playing: Boolean) {
                isPlaying = playing
            }
            override fun onPlaybackStateChanged(playbackState: Int) {
                if (playbackState == Player.STATE_READY) {
                    totalDuration = exoPlayer.duration.coerceAtLeast(0L)
                }
            }
        }
        exoPlayer.addListener(listener)
        onDispose { exoPlayer.removeListener(listener) }
    }

    // 定期更新播放进度（UI 刷新用）
    LaunchedEffect(isPlaying) {
        while (true) {
            if (!isSeeking) {
                currentPosition = exoPlayer.currentPosition.coerceAtLeast(0L)
                bufferedPosition = exoPlayer.bufferedPosition.coerceAtLeast(0L)
                totalDuration = exoPlayer.duration.coerceAtLeast(0L)
            }
            delay(300)
        }
    }

    // 自动隐藏控制栏（播放中 3 秒后隐藏）
    LaunchedEffect(showControls, isPlaying) {
        if (showControls && isPlaying && !isLocked) {
            delay(4000)
            showControls = false
        }
    }

    // 释放播放器
    DisposableEffect(Unit) {
        onDispose {
            val position = exoPlayer.currentPosition / 1000.0
            val duration = exoPlayer.duration / 1000.0
            if (duration > 0) {
                viewModel.saveProgress(mediaId, position, duration)
            }
            exoPlayer.release()
        }
    }

    // 应用画面比例到 PlayerView
    fun applyAspectRatio(ratio: Int) {
        playerViewRef?.resizeMode = when (ratio) {
            0 -> AspectRatioFrameLayout.RESIZE_MODE_FIT
            1 -> AspectRatioFrameLayout.RESIZE_MODE_ZOOM
            2 -> AspectRatioFrameLayout.RESIZE_MODE_FIXED_WIDTH
            3 -> AspectRatioFrameLayout.RESIZE_MODE_FIXED_HEIGHT
            4 -> AspectRatioFrameLayout.RESIZE_MODE_FIT
            else -> AspectRatioFrameLayout.RESIZE_MODE_FIT
        }
    }

    // 进度条百分比
    val progress = if (totalDuration > 0) {
        if (isSeeking) seekPosition.toFloat() / totalDuration.toFloat()
        else currentPosition.toFloat() / totalDuration.toFloat()
    } else 0f
    val bufferedProgress = if (totalDuration > 0) bufferedPosition.toFloat() / totalDuration.toFloat() else 0f

    // ==================== UI ====================
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
            .systemBarsPadding()
    ) {
        // ===== ExoPlayer 视图（禁用自带控制器）=====
        AndroidView(
            factory = { ctx ->
                PlayerView(ctx).apply {
                    player = exoPlayer
                    useController = false  // 完全自定义控制 UI
                    layoutParams = FrameLayout.LayoutParams(
                        ViewGroup.LayoutParams.MATCH_PARENT,
                        ViewGroup.LayoutParams.MATCH_PARENT
                    )
                    resizeMode = when (currentAspectRatio) {
                        0 -> AspectRatioFrameLayout.RESIZE_MODE_FIT
                        1 -> AspectRatioFrameLayout.RESIZE_MODE_ZOOM
                        2 -> AspectRatioFrameLayout.RESIZE_MODE_FIXED_WIDTH
                        3 -> AspectRatioFrameLayout.RESIZE_MODE_FIXED_HEIGHT
                        4 -> AspectRatioFrameLayout.RESIZE_MODE_FIT
                        else -> AspectRatioFrameLayout.RESIZE_MODE_FIT
                    }
                    playerViewRef = this
                }
            },
            modifier = Modifier.fillMaxSize()
        )

        // ===== 手势层（覆盖在播放器上方）=====
        Box(
            modifier = Modifier
                .fillMaxSize()
                .pointerInput(isLocked) {
                    detectTapGestures(
                        onTap = {
                            if (isLocked) {
                                // 锁定时点击只切换锁定按钮显示
                                showControls = !showControls
                            } else {
                                showControls = !showControls
                            }
                        },
                        onDoubleTap = {
                            if (!isLocked) {
                                if (exoPlayer.isPlaying) exoPlayer.pause() else exoPlayer.play()
                            }
                        }
                    )
                }
                .then(
                    if (!isLocked) {
                        Modifier
                            .pointerInput(Unit) {
                                val screenWidth = size.width
                                val screenHeight = size.height
                                val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
                                val maxVolume = audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC)

                                detectVerticalDragGestures { change, dragAmount ->
                                    val x = change.position.x
                                    val sensitivity = 1.5f

                                    if (x < screenWidth / 2) {
                                        activity?.let { act ->
                                            val window = act.window
                                            val lp = window.attributes
                                            var brightness = lp.screenBrightness
                                            if (brightness < 0) {
                                                brightness = Settings.System.getFloat(
                                                    act.contentResolver,
                                                    Settings.System.SCREEN_BRIGHTNESS, 128f
                                                ) / 255f
                                            }
                                            brightness = (brightness - dragAmount / screenHeight * sensitivity).coerceIn(0.01f, 1f)
                                            lp.screenBrightness = brightness
                                            window.attributes = lp
                                            gestureInfo = "☀️ 亮度 ${(brightness * 100).toInt()}%"
                                        }
                                    } else {
                                        val curVol = audioManager.getStreamVolume(AudioManager.STREAM_MUSIC)
                                        val delta = (-dragAmount / screenHeight * maxVolume * sensitivity).toInt()
                                        val newVol = (curVol + delta).coerceIn(0, maxVolume)
                                        if (newVol != curVol) {
                                            audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, newVol, 0)
                                            gestureInfo = "🔊 音量 ${(newVol * 100 / maxVolume)}%"
                                        }
                                    }
                                }
                            }
                            .pointerInput(Unit) {
                                detectHorizontalDragGestures(
                                    onDragStart = {
                                        isSeeking = true
                                        seekPosition = exoPlayer.currentPosition
                                    },
                                    onDragEnd = {
                                        exoPlayer.seekTo(seekPosition)
                                        isSeeking = false
                                    }
                                ) { _, dragAmount ->
                                    if (abs(dragAmount) > 2) {
                                        val seekDelta = (dragAmount / 5).toLong() * 1000
                                        val dur = exoPlayer.duration
                                        seekPosition = (seekPosition + seekDelta).coerceIn(0, dur)
                                        gestureInfo = "⏩ ${formatTime(seekPosition / 1000)} / ${formatTime(dur / 1000)}"
                                    }
                                }
                            }
                    } else Modifier
                )
        )

        // ===== 锁定模式：只显示锁定按钮 =====
        if (isLocked) {
            AnimatedVisibility(
                visible = showControls,
                enter = fadeIn(tween(200)),
                exit = fadeOut(tween(200)),
                modifier = Modifier.align(Alignment.CenterStart)
            ) {
                IconButton(
                    onClick = { isLocked = false },
                    modifier = Modifier
                        .padding(start = 16.dp)
                        .size(48.dp)
                        .background(Color.Black.copy(alpha = 0.5f), CircleShape)
                ) {
                    Icon(Icons.Default.Lock, contentDescription = "解锁", tint = NeonBlue, modifier = Modifier.size(24.dp))
                }
            }
        }

        // ===== 非锁定模式：完整控制 UI =====
        if (!isLocked) {
            // ---------- 顶部渐变遮罩 + 信息栏 ----------
            AnimatedVisibility(
                visible = showControls,
                enter = fadeIn(tween(200)) + slideInVertically(tween(200)) { -it },
                exit = fadeOut(tween(200)) + slideOutVertically(tween(200)) { -it },
                modifier = Modifier.align(Alignment.TopCenter)
            ) {
                Box(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(
                            Brush.verticalGradient(
                                colors = listOf(Color.Black.copy(alpha = 0.7f), Color.Transparent)
                            )
                        )
                        .padding(horizontal = 8.dp, vertical = 8.dp)
                ) {
                    Row(
                        modifier = Modifier.fillMaxWidth(),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        // 返回按钮
                        IconButton(onClick = onBack, modifier = Modifier.size(40.dp)) {
                            Icon(
                                Icons.AutoMirrored.Filled.ArrowBack,
                                contentDescription = "返回",
                                tint = Color.White,
                                modifier = Modifier.size(22.dp)
                            )
                        }

                        // 标题
                        Text(
                            text = uiState.title ?: "",
                            style = MaterialTheme.typography.titleSmall,
                            color = Color.White,
                            maxLines = 1,
                            overflow = TextOverflow.Ellipsis,
                            modifier = Modifier
                                .weight(1f)
                                .padding(horizontal = 4.dp)
                        )

                        // 播放模式标签
                        if (uiState.playbackMode != null) {
                            Surface(
                                shape = RoundedCornerShape(4.dp),
                                color = NeonBlue.copy(alpha = 0.15f)
                            ) {
                                Text(
                                    text = when (uiState.playbackMode) {
                                        PlaybackMode.DIRECT -> "直接播放"
                                        PlaybackMode.REMUX -> "Remux"
                                        PlaybackMode.HLS -> "HLS转码"
                                        PlaybackMode.PREPROCESSED_HLS -> "预处理HLS"
                                        else -> ""
                                    },
                                    style = MaterialTheme.typography.labelSmall.copy(fontSize = 10.sp),
                                    color = NeonBlue,
                                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                                )
                            }
                        }

                        // 倍速标签
                        if (currentSpeed != 1f) {
                            Spacer(modifier = Modifier.width(4.dp))
                            Surface(
                                shape = RoundedCornerShape(4.dp),
                                color = NeonBlue.copy(alpha = 0.15f)
                            ) {
                                Text(
                                    text = "${currentSpeed}x",
                                    style = MaterialTheme.typography.labelSmall.copy(fontSize = 10.sp),
                                    color = NeonBlue,
                                    modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
                                )
                            }
                        }
                    }
                }
            }

            // ---------- 底部渐变遮罩 + 控制栏 ----------
            AnimatedVisibility(
                visible = showControls,
                enter = fadeIn(tween(200)) + slideInVertically(tween(200)) { it },
                exit = fadeOut(tween(200)) + slideOutVertically(tween(200)) { it },
                modifier = Modifier.align(Alignment.BottomCenter)
            ) {
                Column(
                    modifier = Modifier
                        .fillMaxWidth()
                        .background(
                            Brush.verticalGradient(
                                colors = listOf(Color.Transparent, Color.Black.copy(alpha = 0.8f))
                            )
                        )
                        .padding(horizontal = 12.dp)
                        .padding(bottom = 8.dp)
                ) {
                    // ===== 进度条 =====
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .height(32.dp)
                            .pointerInput(Unit) {
                                detectTapGestures { offset ->
                                    if (totalDuration > 0) {
                                        val fraction = (offset.x / size.width).coerceIn(0f, 1f)
                                        val newPos = (fraction * totalDuration).toLong()
                                        exoPlayer.seekTo(newPos)
                                        currentPosition = newPos
                                    }
                                }
                            }
                            .pointerInput(Unit) {
                                detectHorizontalDragGestures(
                                    onDragStart = { offset ->
                                        isSeeking = true
                                        val fraction = (offset.x / size.width).coerceIn(0f, 1f)
                                        seekPosition = (fraction * totalDuration).toLong()
                                    },
                                    onDragEnd = {
                                        exoPlayer.seekTo(seekPosition)
                                        currentPosition = seekPosition
                                        isSeeking = false
                                    }
                                ) { change, _ ->
                                    val fraction = (change.position.x / size.width).coerceIn(0f, 1f)
                                    seekPosition = (fraction * totalDuration).toLong()
                                }
                            },
                        contentAlignment = Alignment.CenterStart
                    ) {
                        // 背景轨道
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .height(3.dp)
                                .clip(RoundedCornerShape(1.5.dp))
                                .background(Color.White.copy(alpha = 0.15f))
                        )
                        // 缓冲进度
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(bufferedProgress)
                                .height(3.dp)
                                .clip(RoundedCornerShape(1.5.dp))
                                .background(Color.White.copy(alpha = 0.3f))
                        )
                        // 播放进度
                        Box(
                            modifier = Modifier
                                .fillMaxWidth(progress.coerceIn(0f, 1f))
                                .height(3.dp)
                                .clip(RoundedCornerShape(1.5.dp))
                                .background(NeonBlue)
                        )
                        // 进度条拖拽圆点
                        Box(
                            modifier = Modifier
                                .offset(x = ((progress.coerceIn(0f, 1f)) * (this@Box).run { 0 }).dp) // 占位，实际用 fraction
                        ) {
                            // 使用 fillMaxWidth(fraction) 的方式定位
                        }
                        Row(
                            modifier = Modifier.fillMaxWidth(progress.coerceIn(0f, 1f)),
                            horizontalArrangement = Arrangement.End
                        ) {
                            Box(
                                modifier = Modifier
                                    .size(if (isSeeking) 16.dp else 12.dp)
                                    .clip(CircleShape)
                                    .background(NeonBlue)
                            )
                        }
                    }

                    // ===== 底部按钮行 =====
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 2.dp),
                        verticalAlignment = Alignment.CenterVertically
                    ) {
                        // 播放/暂停
                        IconButton(
                            onClick = { if (exoPlayer.isPlaying) exoPlayer.pause() else exoPlayer.play() },
                            modifier = Modifier.size(40.dp)
                        ) {
                            Icon(
                                if (isPlaying) Icons.Default.Pause else Icons.Default.PlayArrow,
                                contentDescription = if (isPlaying) "暂停" else "播放",
                                tint = Color.White.copy(alpha = 0.9f),
                                modifier = Modifier.size(26.dp)
                            )
                        }

                        // 快退 10s
                        IconButton(
                            onClick = { exoPlayer.seekTo((exoPlayer.currentPosition - 10000).coerceAtLeast(0)) },
                            modifier = Modifier.size(36.dp)
                        ) {
                            Icon(
                                Icons.Default.Replay10,
                                contentDescription = "快退10秒",
                                tint = Color.White.copy(alpha = 0.7f),
                                modifier = Modifier.size(20.dp)
                            )
                        }

                        // 快进 10s
                        IconButton(
                            onClick = { exoPlayer.seekTo((exoPlayer.currentPosition + 10000).coerceAtMost(exoPlayer.duration)) },
                            modifier = Modifier.size(36.dp)
                        ) {
                            Icon(
                                Icons.Default.Forward10,
                                contentDescription = "快进10秒",
                                tint = Color.White.copy(alpha = 0.7f),
                                modifier = Modifier.size(20.dp)
                            )
                        }

                        // 时间显示
                        val displayPos = if (isSeeking) seekPosition else currentPosition
                        Text(
                            text = "${formatTime(displayPos / 1000)} / ${formatTime(totalDuration / 1000)}",
                            style = MaterialTheme.typography.labelSmall.copy(fontSize = 11.sp, letterSpacing = 0.5.sp),
                            color = Color.White.copy(alpha = 0.6f),
                            modifier = Modifier.padding(start = 4.dp)
                        )

                        Spacer(modifier = Modifier.weight(1f))

                        // ---- 右侧功能按钮 ----

                        // 倍速按钮
                        Surface(
                            modifier = Modifier
                                .clip(RoundedCornerShape(4.dp))
                                .clickable {
                                    showSettingsPanel = true
                                    settingsCategory = "speed"
                                },
                            color = Color.Transparent
                        ) {
                            Text(
                                text = if (currentSpeed != 1f) "${currentSpeed}x" else "倍速",
                                style = MaterialTheme.typography.labelSmall.copy(
                                    fontSize = 11.sp,
                                    fontWeight = if (currentSpeed != 1f) FontWeight.Bold else FontWeight.Normal
                                ),
                                color = if (currentSpeed != 1f) NeonBlue else Color.White.copy(alpha = 0.7f),
                                modifier = Modifier.padding(horizontal = 8.dp, vertical = 8.dp)
                            )
                        }

                        // 字幕按钮
                        if (uiState.subtitleTracks.isNotEmpty()) {
                            IconButton(
                                onClick = { showSubtitlePicker = true },
                                modifier = Modifier.size(36.dp)
                            ) {
                                Icon(
                                    Icons.Default.Subtitles,
                                    contentDescription = "字幕",
                                    tint = Color.White.copy(alpha = 0.7f),
                                    modifier = Modifier.size(20.dp)
                                )
                            }
                        }

                        // 设置按钮
                        IconButton(
                            onClick = {
                                showSettingsPanel = true
                                settingsCategory = null
                            },
                            modifier = Modifier.size(36.dp)
                        ) {
                            Icon(
                                Icons.Default.Settings,
                                contentDescription = "设置",
                                tint = Color.White.copy(alpha = 0.7f),
                                modifier = Modifier.size(20.dp)
                            )
                        }

                        // 锁定按钮
                        IconButton(
                            onClick = {
                                isLocked = true
                                showControls = true
                            },
                            modifier = Modifier.size(36.dp)
                        ) {
                            Icon(
                                Icons.Default.LockOpen,
                                contentDescription = "锁定",
                                tint = Color.White.copy(alpha = 0.7f),
                                modifier = Modifier.size(20.dp)
                            )
                        }
                    }
                }
            }
        }

        // ===== 中央大播放按钮（暂停时显示）=====
        AnimatedVisibility(
            visible = !isPlaying && !uiState.loading && showControls && !isLocked,
            enter = fadeIn(tween(200)),
            exit = fadeOut(tween(200)),
            modifier = Modifier.align(Alignment.Center)
        ) {
            Surface(
                modifier = Modifier
                    .size(72.dp)
                    .clip(CircleShape)
                    .clickable { exoPlayer.play() },
                shape = CircleShape,
                color = NeonBlue.copy(alpha = 0.2f)
            ) {
                Box(contentAlignment = Alignment.Center) {
                    Icon(
                        Icons.Default.PlayArrow,
                        contentDescription = "播放",
                        tint = Color.White,
                        modifier = Modifier.size(40.dp)
                    )
                }
            }
        }

        // ===== 手势提示（居中）=====
        AnimatedVisibility(
            visible = gestureInfo != null,
            enter = fadeIn(tween(150)),
            exit = fadeOut(tween(300)),
            modifier = Modifier.align(Alignment.Center)
        ) {
            Surface(
                shape = RoundedCornerShape(16.dp),
                color = ControlBgAlpha
            ) {
                Text(
                    text = gestureInfo ?: "",
                    style = MaterialTheme.typography.titleLarge.copy(fontWeight = FontWeight.Bold, letterSpacing = 1.sp),
                    color = Color.White,
                    modifier = Modifier.padding(horizontal = 28.dp, vertical = 14.dp)
                )
            }
        }

        // ===== 加载中 =====
        if (uiState.loading) {
            CircularProgressIndicator(
                modifier = Modifier.align(Alignment.Center),
                color = NeonBlue,
                strokeWidth = 3.dp
            )
        }

        // ===== 降级提示 =====
        AnimatedVisibility(
            visible = uiState.fallbackMessage != null,
            enter = fadeIn() + slideInVertically { it },
            exit = fadeOut() + slideOutVertically { it },
            modifier = Modifier
                .align(Alignment.BottomCenter)
                .padding(16.dp)
        ) {
            Surface(
                shape = RoundedCornerShape(12.dp),
                color = ControlBgAlpha
            ) {
                Text(
                    text = uiState.fallbackMessage ?: "",
                    style = MaterialTheme.typography.bodySmall,
                    color = NeonBlue,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 10.dp)
                )
            }
        }
    }

    // ==================== 播放设置面板（ModalBottomSheet）====================
    if (showSettingsPanel) {
        ModalBottomSheet(
            onDismissRequest = {
                showSettingsPanel = false
                settingsCategory = null
            },
            containerColor = Color(0xFF1A1A2E),
            contentColor = Color.White,
            shape = RoundedCornerShape(topStart = 16.dp, topEnd = 16.dp)
        ) {
            Column(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(bottom = 32.dp)
            ) {
                when (settingsCategory) {
                    null -> {
                        // ===== 主菜单 =====
                        Text(
                            text = "播放设置",
                            style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.Bold),
                            color = Color.White,
                            modifier = Modifier.padding(horizontal = 20.dp, vertical = 12.dp)
                        )

                        // 播放速度
                        SettingsMenuItem(
                            icon = Icons.Default.Speed,
                            title = "播放速度",
                            value = if (currentSpeed == 1f) "正常" else "${currentSpeed}x",
                            isHighlighted = currentSpeed != 1f,
                            onClick = { settingsCategory = "speed" }
                        )

                        // 画面比例
                        SettingsMenuItem(
                            icon = Icons.Default.AspectRatio,
                            title = "画面比例",
                            value = aspectRatioLabels[currentAspectRatio],
                            isHighlighted = currentAspectRatio != 0,
                            onClick = { settingsCategory = "aspect" }
                        )

                        // 解码方式
                        SettingsMenuItem(
                            icon = Icons.Default.Memory,
                            title = "解码方式",
                            value = decoderLabels[currentDecoder],
                            isHighlighted = currentDecoder != 0,
                            onClick = { settingsCategory = "decoder" }
                        )

                        // 字幕设置
                        if (uiState.subtitleTracks.isNotEmpty()) {
                            SettingsMenuItem(
                                icon = Icons.Default.Subtitles,
                                title = "字幕",
                                value = "${uiState.subtitleTracks.size} 条可用",
                                onClick = {
                                    showSettingsPanel = false
                                    settingsCategory = null
                                    showSubtitlePicker = true
                                }
                            )
                        }

                        // 播放模式信息（只读展示）
                        if (uiState.playbackMode != null) {
                            HorizontalDivider(
                                color = Color.White.copy(alpha = 0.1f),
                                modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp)
                            )
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(horizontal = 20.dp, vertical = 8.dp),
                                horizontalArrangement = Arrangement.SpaceBetween,
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    text = "当前播放模式",
                                    style = MaterialTheme.typography.bodySmall,
                                    color = Color.White.copy(alpha = 0.5f)
                                )
                                Surface(
                                    shape = RoundedCornerShape(4.dp),
                                    color = MaterialTheme.colorScheme.primary.copy(alpha = 0.15f)
                                ) {
                                    Text(
                                        text = when (uiState.playbackMode) {
                                            PlaybackMode.DIRECT -> "直接播放"
                                            PlaybackMode.REMUX -> "Remux 转封装"
                                            PlaybackMode.HLS -> "HLS 实时转码"
                                            PlaybackMode.PREPROCESSED_HLS -> "预处理 HLS"
                                            else -> ""
                                        },
                                        style = MaterialTheme.typography.labelSmall,
                                        color = MaterialTheme.colorScheme.primary,
                                        modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
                                    )
                                }
                            }
                            // 视频信息
                            uiState.streamInfo?.let { info ->
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(horizontal = 20.dp, vertical = 4.dp),
                                    horizontalArrangement = Arrangement.spacedBy(12.dp)
                                ) {
                                    if (info.resolution.isNotBlank()) {
                                        InfoChip(text = info.resolution)
                                    }
                                    if (info.videoCodec.isNotBlank()) {
                                        InfoChip(text = info.videoCodec.uppercase())
                                    }
                                    if (info.audioCodec.isNotBlank()) {
                                        InfoChip(text = info.audioCodec.uppercase())
                                    }
                                }
                            }
                        }
                    }

                    "speed" -> {
                        // ===== 播放速度子菜单 =====
                        SettingsSubHeader(
                            title = "播放速度",
                            onBack = { settingsCategory = null }
                        )

                        // 快速恢复正常
                        if (currentSpeed != 1f) {
                            Surface(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable {
                                        currentSpeed = 1f
                                        exoPlayer.setPlaybackSpeed(1f)
                                        viewModel.savePlaybackSpeed(1f)
                                        gestureInfo = "正常速度"
                                    }
                                    .padding(horizontal = 20.dp, vertical = 12.dp),
                                color = Color.Transparent
                            ) {
                                Row(verticalAlignment = Alignment.CenterVertically) {
                                    Icon(
                                        Icons.Default.Restore,
                                        contentDescription = null,
                                        tint = MaterialTheme.colorScheme.primary,
                                        modifier = Modifier.size(20.dp)
                                    )
                                    Spacer(modifier = Modifier.width(12.dp))
                                    Text(
                                        "恢复正常速度",
                                        color = MaterialTheme.colorScheme.primary,
                                        style = MaterialTheme.typography.bodyMedium
                                    )
                                }
                            }
                            HorizontalDivider(color = Color.White.copy(alpha = 0.08f), modifier = Modifier.padding(horizontal = 20.dp))
                        }

                        Column(
                            modifier = Modifier
                                .fillMaxWidth()
                                .verticalScroll(rememberScrollState())
                        ) {
                            speedOptions.forEach { speed ->
                                val isSelected = speed == currentSpeed
                                Surface(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .clickable {
                                            currentSpeed = speed
                                            exoPlayer.setPlaybackSpeed(speed)
                                            viewModel.savePlaybackSpeed(speed)
                                            gestureInfo = if (speed == 1f) "正常速度" else "${speed}x 倍速"
                                        }
                                        .padding(horizontal = 20.dp, vertical = 2.dp),
                                    color = if (isSelected) MaterialTheme.colorScheme.primary.copy(alpha = 0.12f) else Color.Transparent,
                                    shape = RoundedCornerShape(8.dp)
                                ) {
                                    Row(
                                        modifier = Modifier
                                            .fillMaxWidth()
                                            .padding(horizontal = 12.dp, vertical = 14.dp),
                                        horizontalArrangement = Arrangement.SpaceBetween,
                                        verticalAlignment = Alignment.CenterVertically
                                    ) {
                                        Text(
                                            text = if (speed == 1f) "正常" else "${speed}x",
                                            color = if (isSelected) MaterialTheme.colorScheme.primary else Color.White.copy(alpha = 0.8f),
                                            style = MaterialTheme.typography.bodyMedium,
                                            fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Normal
                                        )
                                        if (isSelected) {
                                            Icon(
                                                Icons.Default.Check,
                                                contentDescription = null,
                                                tint = MaterialTheme.colorScheme.primary,
                                                modifier = Modifier.size(20.dp)
                                            )
                                        }
                                    }
                                }
                            }
                        }
                    }

                    "aspect" -> {
                        // ===== 画面比例子菜单 =====
                        SettingsSubHeader(
                            title = "画面比例",
                            onBack = { settingsCategory = null }
                        )

                        aspectRatioLabels.forEachIndexed { index, label ->
                            val isSelected = index == currentAspectRatio
                            val description = when (index) {
                                0 -> "保持原始比例，适应屏幕"
                                1 -> "裁剪画面以填满屏幕"
                                2 -> "强制 16:9 宽屏比例"
                                3 -> "强制 4:3 传统比例"
                                4 -> "使用视频原始尺寸"
                                else -> ""
                            }
                            Surface(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable {
                                        currentAspectRatio = index
                                        applyAspectRatio(index)
                                        viewModel.saveAspectRatio(index)
                                        gestureInfo = "画面比例: $label"
                                    }
                                    .padding(horizontal = 20.dp, vertical = 2.dp),
                                color = if (isSelected) MaterialTheme.colorScheme.primary.copy(alpha = 0.12f) else Color.Transparent,
                                shape = RoundedCornerShape(8.dp)
                            ) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(horizontal = 12.dp, vertical = 12.dp),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Column(modifier = Modifier.weight(1f)) {
                                        Text(
                                            text = label,
                                            color = if (isSelected) MaterialTheme.colorScheme.primary else Color.White.copy(alpha = 0.8f),
                                            style = MaterialTheme.typography.bodyMedium,
                                            fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Normal
                                        )
                                        Text(
                                            text = description,
                                            color = Color.White.copy(alpha = 0.4f),
                                            style = MaterialTheme.typography.bodySmall
                                        )
                                    }
                                    if (isSelected) {
                                        Icon(
                                            Icons.Default.Check,
                                            contentDescription = null,
                                            tint = MaterialTheme.colorScheme.primary,
                                            modifier = Modifier.size(20.dp)
                                        )
                                    }
                                }
                            }
                        }
                    }

                    "decoder" -> {
                        // ===== 解码方式子菜单 =====
                        SettingsSubHeader(
                            title = "解码方式",
                            onBack = { settingsCategory = null }
                        )

                        decoderLabels.forEachIndexed { index, label ->
                            val isSelected = index == currentDecoder
                            val description = when (index) {
                                0 -> "系统自动选择最佳解码器"
                                1 -> "优先使用 GPU 硬件加速，省电高效"
                                2 -> "使用 CPU 软件解码，兼容性最好"
                                else -> ""
                            }
                            Surface(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .clickable {
                                        currentDecoder = index
                                        viewModel.saveDecoderPriority(index)
                                        gestureInfo = "解码方式: $label"
                                    }
                                    .padding(horizontal = 20.dp, vertical = 2.dp),
                                color = if (isSelected) MaterialTheme.colorScheme.primary.copy(alpha = 0.12f) else Color.Transparent,
                                shape = RoundedCornerShape(8.dp)
                            ) {
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(horizontal = 12.dp, vertical = 12.dp),
                                    horizontalArrangement = Arrangement.SpaceBetween,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    Column(modifier = Modifier.weight(1f)) {
                                        Text(
                                            text = label,
                                            color = if (isSelected) MaterialTheme.colorScheme.primary else Color.White.copy(alpha = 0.8f),
                                            style = MaterialTheme.typography.bodyMedium,
                                            fontWeight = if (isSelected) FontWeight.Bold else FontWeight.Normal
                                        )
                                        Text(
                                            text = description,
                                            color = Color.White.copy(alpha = 0.4f),
                                            style = MaterialTheme.typography.bodySmall
                                        )
                                    }
                                    if (isSelected) {
                                        Icon(
                                            Icons.Default.Check,
                                            contentDescription = null,
                                            tint = MaterialTheme.colorScheme.primary,
                                            modifier = Modifier.size(20.dp)
                                        )
                                    }
                                }
                            }
                        }

                        // 提示信息
                        Text(
                            text = "注意：切换解码方式将在下次播放时生效",
                            style = MaterialTheme.typography.bodySmall,
                            color = Color.White.copy(alpha = 0.4f),
                            modifier = Modifier.padding(horizontal = 32.dp, vertical = 12.dp)
                        )
                    }
                }
            }
        }
    }

    // 字幕选择弹窗
    if (showSubtitlePicker) {
        AlertDialog(
            onDismissRequest = { showSubtitlePicker = false },
            title = { Text("选择字幕") },
            text = {
                Column {
                    // 关闭字幕选项
                    ListItem(
                        headlineContent = { Text("关闭字幕") },
                        leadingContent = {
                            Icon(Icons.Default.SubtitlesOff, contentDescription = null)
                        },
                        modifier = Modifier.fillMaxWidth()
                    )
                    HorizontalDivider()
                    // 字幕轨道列表
                    uiState.subtitleTracks.forEach { track ->
                        ListItem(
                            headlineContent = {
                                Text(track.title.ifBlank { "字幕 ${track.index}" })
                            },
                            supportingContent = {
                                Text(
                                    buildString {
                                        if (track.language.isNotBlank()) append(track.language)
                                        if (track.codec.isNotBlank()) append(" · ${track.codec}")
                                        if (track.forced) append(" · 强制")
                                    }
                                )
                            },
                            leadingContent = {
                                Icon(Icons.Default.Subtitles, contentDescription = null)
                            },
                            modifier = Modifier.fillMaxWidth()
                        )
                    }
                }
            },
            confirmButton = {
                TextButton(onClick = { showSubtitlePicker = false }) {
                    Text("关闭")
                }
            }
        )
    }
}

// ==================== 设置面板组件 ====================

/**
 * 设置菜单项
 */
@Composable
private fun SettingsMenuItem(
    icon: ImageVector,
    title: String,
    value: String,
    isHighlighted: Boolean = false,
    onClick: () -> Unit
) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(onClick = onClick)
            .padding(horizontal = 8.dp, vertical = 2.dp),
        color = Color.Transparent,
        shape = RoundedCornerShape(8.dp)
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 14.dp),
            horizontalArrangement = Arrangement.SpaceBetween,
            verticalAlignment = Alignment.CenterVertically
        ) {
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(12.dp)
            ) {
                Icon(
                    icon,
                    contentDescription = null,
                    tint = if (isHighlighted) MaterialTheme.colorScheme.primary else Color.White.copy(alpha = 0.7f),
                    modifier = Modifier.size(22.dp)
                )
                Text(
                    text = title,
                    style = MaterialTheme.typography.bodyMedium,
                    color = Color.White.copy(alpha = 0.9f)
                )
            }
            Row(
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(4.dp)
            ) {
                Text(
                    text = value,
                    style = MaterialTheme.typography.bodySmall,
                    color = if (isHighlighted) MaterialTheme.colorScheme.primary else Color.White.copy(alpha = 0.5f)
                )
                Icon(
                    Icons.Default.ChevronRight,
                    contentDescription = null,
                    tint = Color.White.copy(alpha = 0.3f),
                    modifier = Modifier.size(18.dp)
                )
            }
        }
    }
}

/**
 * 设置子菜单头部（带返回按钮）
 */
@Composable
private fun SettingsSubHeader(
    title: String,
    onBack: () -> Unit
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 8.dp, vertical = 4.dp),
        verticalAlignment = Alignment.CenterVertically
    ) {
        IconButton(onClick = onBack) {
            Icon(
                Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "返回",
                tint = Color.White,
                modifier = Modifier.size(20.dp)
            )
        }
        Text(
            text = title,
            style = MaterialTheme.typography.titleMedium.copy(fontWeight = FontWeight.Bold),
            color = Color.White
        )
    }
    HorizontalDivider(color = Color.White.copy(alpha = 0.08f), modifier = Modifier.padding(horizontal = 20.dp))
    Spacer(modifier = Modifier.height(4.dp))
}

/**
 * 信息标签（用于显示视频编码等信息）
 */
@Composable
private fun InfoChip(text: String) {
    Surface(
        shape = RoundedCornerShape(4.dp),
        color = Color.White.copy(alpha = 0.08f)
    ) {
        Text(
            text = text,
            style = MaterialTheme.typography.labelSmall,
            color = Color.White.copy(alpha = 0.5f),
            modifier = Modifier.padding(horizontal = 6.dp, vertical = 2.dp)
        )
    }
}

private fun formatTime(seconds: Long): String {
    val h = seconds / 3600
    val m = (seconds % 3600) / 60
    val s = seconds % 60
    return if (h > 0) String.format("%d:%02d:%02d", h, m, s)
    else String.format("%d:%02d", m, s)
}

// ==================== 播放模式 ====================

enum class PlaybackMode {
    DIRECT,
    REMUX,
    HLS,
    PREPROCESSED_HLS
}

// ==================== ViewModel ====================

data class PlayerUiState(
    val loading: Boolean = true,
    val title: String? = null,
    val streamInfo: StreamInfo? = null,
    val playbackUrl: String? = null,
    val playbackMode: PlaybackMode? = null,
    val resumePosition: Double = 0.0,
    val subtitleTracks: List<SubtitleTrack> = emptyList(),
    val fallbackMessage: String? = null,
    val error: String? = null
)

@HiltViewModel
class PlayerViewModel @Inject constructor(
    private val mediaRepository: MediaRepository,
    private val tokenManager: TokenManager,
    private val playerPreferences: PlayerPreferences
) : ViewModel() {

    private val _uiState = MutableStateFlow(PlayerUiState())
    val uiState = _uiState.asStateFlow()

    private var currentMediaId: String = ""
    var serverUrl: String = ""
        private set
    var token: String = ""
        private set
    private var triedModes = mutableSetOf<PlaybackMode>()

    init {
        // 在 ViewModel 初始化时同步加载 token 和 serverUrl
        // 确保 Composable 首次组合时 ExoPlayer 能获取到认证信息
        runBlocking {
            serverUrl = tokenManager.getServerUrl() ?: ""
            token = tokenManager.getToken() ?: ""
        }
    }

    // ==================== 偏好设置读写 ====================

    suspend fun getPlaybackSpeed(): Float = playerPreferences.getPlaybackSpeed()
    suspend fun getAspectRatio(): Int = playerPreferences.getAspectRatio()
    suspend fun getDecoderPriority(): Int = playerPreferences.getDecoderPriority()

    fun savePlaybackSpeed(speed: Float) {
        viewModelScope.launch { playerPreferences.setPlaybackSpeed(speed) }
    }

    fun saveAspectRatio(ratio: Int) {
        viewModelScope.launch { playerPreferences.setAspectRatio(ratio) }
    }

    fun saveDecoderPriority(priority: Int) {
        viewModelScope.launch { playerPreferences.setDecoderPriority(priority) }
    }

    // ==================== 流信息加载 ====================

    fun loadStreamInfo(mediaId: String) {
        currentMediaId = mediaId
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true)

            // 刷新 token（可能已过期或更新）
            serverUrl = tokenManager.getServerUrl() ?: ""
            token = tokenManager.getToken() ?: ""

            // 获取媒体详情（标题）
            mediaRepository.getMediaDetail(mediaId).onSuccess { media ->
                _uiState.value = _uiState.value.copy(title = media.displayTitle())
            }

            // 获取播放进度
            mediaRepository.getProgress(mediaId).onSuccess { history ->
                if (history != null && !history.completed) {
                    _uiState.value = _uiState.value.copy(resumePosition = history.position)
                }
            }

            // 获取流信息
            mediaRepository.getStreamInfo(mediaId).onSuccess { info ->
                Log.d(TAG, "StreamInfo: preprocessed=${info.preprocessed}, canDirectPlay=${info.canDirectPlay}, canRemux=${info.canRemux}, videoCodec=${info.videoCodec}")
                _uiState.value = _uiState.value.copy(streamInfo = info)
                resolvePlaybackMode(info)
            }.onFailure { e ->
                Log.e(TAG, "获取流信息失败", e)
                fallbackToHLS()
            }

            _uiState.value = _uiState.value.copy(loading = false)
        }
    }

    fun loadSubtitleTracks(mediaId: String) {
        viewModelScope.launch {
            mediaRepository.getSubtitleTracks(mediaId).onSuccess { tracks ->
                _uiState.value = _uiState.value.copy(subtitleTracks = tracks)
            }
        }
    }

    /**
     * 智能播放模式选择（Android 端优化版）
     *
     * Android 端的 ExoPlayer 原生支持 MKV/AVI/MOV/FLV 等容器格式，
     * 不像浏览器只能播放 MP4/WebM。因此 Android 端的策略是：
     * 只要设备硬件支持视频编码（H.264/H.265/VP9/AV1），就直接播放原始文件，
     * 无需 Remux 或转码，实现真正的"秒开"体验。
     *
     * 优先级：Direct Play（Android 原生支持） > 预处理 HLS > Remux > HLS 转码
     * 注意：Android 端优先直接播放，因为 ExoPlayer 支持的格式远多于浏览器
     */
    private fun resolvePlaybackMode(info: StreamInfo) {
        val baseUrl = "$serverUrl/api/stream/$currentMediaId"

        when {
            // 1. Android 端核心优化：只要设备支持该编码，直接播放原始文件（包括 MKV/AVI/MOV 等）
            //    ExoPlayer 原生支持这些容器格式，无需 Remux，零延迟秒开
            isDeviceSupported(info.videoCodec) -> {
                val url = "$baseUrl/direct?token=$token"
                Log.d(TAG, "Android 直接播放: codec=${info.videoCodec}, 跳过转码")
                setPlayback(PlaybackMode.DIRECT, url)
            }
            // 2. 设备不支持该编码，但有预处理完成的 HLS → 使用预处理的 HLS 流
            info.preprocessed -> {
                val url = "$serverUrl/api/preprocess/media/$currentMediaId/master.m3u8?token=$token"
                Log.d(TAG, "使用预处理 HLS: codec=${info.videoCodec}")
                setPlayback(PlaybackMode.PREPROCESSED_HLS, url)
            }
            // 3. 可以 Remux → 转封装播放
            info.canRemux -> {
                val url = "$baseUrl/remux?token=$token"
                Log.d(TAG, "使用 Remux: codec=${info.videoCodec}")
                setPlayback(PlaybackMode.REMUX, url)
            }
            // 4. 兜底：HLS 实时转码
            else -> {
                Log.d(TAG, "降级到 HLS 转码: codec=${info.videoCodec}")
                fallbackToHLS()
            }
        }
    }

    private fun setPlayback(mode: PlaybackMode, url: String) {
        Log.d(TAG, "setPlayback: mode=$mode, url=$url")
        triedModes.add(mode)
        _uiState.value = _uiState.value.copy(
            playbackUrl = url,
            playbackMode = mode,
            fallbackMessage = null
        )
    }

    private fun fallbackToHLS() {
        val url = "$serverUrl/api/stream/$currentMediaId/master.m3u8?token=$token"
        triedModes.add(PlaybackMode.HLS)
        _uiState.value = _uiState.value.copy(
            playbackUrl = url,
            playbackMode = PlaybackMode.HLS
        )
    }

    fun onPlaybackError(error: PlaybackException) {
        val currentMode = _uiState.value.playbackMode ?: return
        Log.e(TAG, "播放错误: mode=$currentMode, error=${error.message}", error)

        val info = _uiState.value.streamInfo
        val nextMode = when (currentMode) {
            PlaybackMode.DIRECT -> {
                // 直接播放失败 → 尝试预处理 HLS（如果有）→ Remux → HLS
                when {
                    info?.preprocessed == true && PlaybackMode.PREPROCESSED_HLS !in triedModes -> PlaybackMode.PREPROCESSED_HLS
                    info?.canRemux == true && PlaybackMode.REMUX !in triedModes -> PlaybackMode.REMUX
                    PlaybackMode.HLS !in triedModes -> PlaybackMode.HLS
                    else -> null
                }
            }
            PlaybackMode.PREPROCESSED_HLS -> {
                // 预处理 HLS 失败 → 尝试 Remux → HLS
                when {
                    info?.canRemux == true && PlaybackMode.REMUX !in triedModes -> PlaybackMode.REMUX
                    PlaybackMode.HLS !in triedModes -> PlaybackMode.HLS
                    else -> null
                }
            }
            PlaybackMode.REMUX -> if (PlaybackMode.HLS !in triedModes) PlaybackMode.HLS else null
            PlaybackMode.HLS -> null
        }

        if (nextMode != null) {
            val baseUrl = "$serverUrl/api/stream/$currentMediaId"

            val url = when (nextMode) {
                PlaybackMode.DIRECT -> "$baseUrl/direct?token=$token"
                PlaybackMode.REMUX -> "$baseUrl/remux?token=$token"
                PlaybackMode.HLS -> "$baseUrl/master.m3u8?token=$token"
                PlaybackMode.PREPROCESSED_HLS -> "$serverUrl/api/preprocess/media/$currentMediaId/master.m3u8?token=$token"
            }

            Log.d(TAG, "自动降级: $currentMode -> $nextMode, url=$url")
            triedModes.add(nextMode)
            _uiState.value = _uiState.value.copy(
                playbackUrl = url,
                playbackMode = nextMode,
                fallbackMessage = "已自动切换到${
                    when (nextMode) {
                        PlaybackMode.DIRECT -> "直接播放"
                        PlaybackMode.REMUX -> "Remux 播放"
                        PlaybackMode.HLS -> "HLS 转码播放"
                        PlaybackMode.PREPROCESSED_HLS -> "预处理播放"
                    }
                }"
            )

            viewModelScope.launch {
                delay(3000)
                _uiState.value = _uiState.value.copy(fallbackMessage = null)
            }
        } else {
            _uiState.value = _uiState.value.copy(
                error = "播放失败: ${error.message}"
            )
        }
    }

    fun saveProgress(mediaId: String, position: Double, duration: Double) {
        viewModelScope.launch {
            val completed = position >= duration * 0.95
            mediaRepository.updateProgress(mediaId, position, duration, completed)
        }
    }

    private fun isDeviceSupported(codec: String): Boolean {
        val codecList = MediaCodecList(MediaCodecList.ALL_CODECS)
        return when (codec.lowercase()) {
            "h264", "avc", "avc1" -> true
            "h265", "hevc" -> codecList.codecInfos.any { info ->
                !info.isEncoder && info.supportedTypes.any { it.contains("hevc", ignoreCase = true) }
            }
            "av1" -> codecList.codecInfos.any { info ->
                !info.isEncoder && info.supportedTypes.any { it.contains("av01", ignoreCase = true) }
            }
            "vp9" -> codecList.codecInfos.any { info ->
                !info.isEncoder && info.supportedTypes.any { it.contains("vp9", ignoreCase = true) }
            }
            else -> false
        }
    }
}
