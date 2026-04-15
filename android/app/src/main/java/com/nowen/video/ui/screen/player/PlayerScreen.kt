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
import androidx.compose.foundation.background
import androidx.compose.foundation.gestures.detectHorizontalDragGestures
import androidx.compose.foundation.gestures.detectVerticalDragGestures
import androidx.compose.foundation.layout.*
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.*
import androidx.compose.material3.*
import androidx.compose.runtime.*
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
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
import androidx.media3.ui.PlayerView
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.StreamInfo
import com.nowen.video.data.model.SubtitleTrack
import com.nowen.video.data.repository.MediaRepository
import dagger.hilt.android.lifecycle.HiltViewModel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import javax.inject.Inject
import kotlin.math.abs

/**
 * 视频播放器页面
 * 支持三种播放模式：Direct / Remux / HLS
 * 播放失败自动降级 + 字幕选择 + 手势控制
 */
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

    // 手势状态
    var gestureInfo by remember { mutableStateOf<String?>(null) }
    var showSubtitlePicker by remember { mutableStateOf(false) }

    // 创建 ExoPlayer
    val exoPlayer = remember {
        ExoPlayer.Builder(context).build().apply {
            playWhenReady = true
        }
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

    // UI
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black)
    ) {
        // ExoPlayer 视图
        AndroidView(
            factory = { ctx ->
                PlayerView(ctx).apply {
                    player = exoPlayer
                    useController = true
                    layoutParams = FrameLayout.LayoutParams(
                        ViewGroup.LayoutParams.MATCH_PARENT,
                        ViewGroup.LayoutParams.MATCH_PARENT
                    )
                }
            },
            modifier = Modifier.fillMaxSize()
        )

        // 手势层（覆盖在播放器上方）
        Box(
            modifier = Modifier
                .fillMaxSize()
                .pointerInput(Unit) {
                    val screenWidth = size.width
                    val screenHeight = size.height
                    val audioManager = context.getSystemService(Context.AUDIO_SERVICE) as AudioManager
                    val maxVolume = audioManager.getStreamMaxVolume(AudioManager.STREAM_MUSIC)

                    detectVerticalDragGestures { change, dragAmount ->
                        val x = change.position.x
                        val sensitivity = 1.5f

                        if (x < screenWidth / 2) {
                            // 左半屏：亮度调节
                            activity?.let { act ->
                                val window = act.window
                                val layoutParams = window.attributes
                                var brightness = layoutParams.screenBrightness
                                if (brightness < 0) {
                                    brightness = Settings.System.getFloat(
                                        act.contentResolver,
                                        Settings.System.SCREEN_BRIGHTNESS, 128f
                                    ) / 255f
                                }
                                brightness = (brightness - dragAmount / screenHeight * sensitivity).coerceIn(0.01f, 1f)
                                layoutParams.screenBrightness = brightness
                                window.attributes = layoutParams
                                gestureInfo = "亮度: ${(brightness * 100).toInt()}%"
                            }
                        } else {
                            // 右半屏：音量调节
                            val currentVolume = audioManager.getStreamVolume(AudioManager.STREAM_MUSIC)
                            val delta = (-dragAmount / screenHeight * maxVolume * sensitivity).toInt()
                            val newVolume = (currentVolume + delta).coerceIn(0, maxVolume)
                            if (newVolume != currentVolume) {
                                audioManager.setStreamVolume(AudioManager.STREAM_MUSIC, newVolume, 0)
                                gestureInfo = "音量: ${(newVolume * 100 / maxVolume)}%"
                            }
                        }
                    }
                }
                .pointerInput(Unit) {
                    detectHorizontalDragGestures(
                        onDragEnd = {
                            // 拖拽结束后执行 seek
                        }
                    ) { _, dragAmount ->
                        if (abs(dragAmount) > 2) {
                            val seekDelta = (dragAmount / 5).toLong() * 1000 // 每 5px = 1秒
                            val currentPos = exoPlayer.currentPosition
                            val duration = exoPlayer.duration
                            val newPos = (currentPos + seekDelta).coerceIn(0, duration)
                            exoPlayer.seekTo(newPos)
                            gestureInfo = formatTime(newPos / 1000) + " / " + formatTime(duration / 1000)
                        }
                    }
                }
        )

        // 顶部返回按钮 + 字幕按钮
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .align(Alignment.TopStart)
                .padding(8.dp)
                .statusBarsPadding(),
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            IconButton(onClick = onBack) {
                Icon(
                    Icons.AutoMirrored.Filled.ArrowBack,
                    contentDescription = "返回",
                    tint = Color.White
                )
            }

            Row {
                // 字幕按钮
                if (uiState.subtitleTracks.isNotEmpty()) {
                    IconButton(onClick = { showSubtitlePicker = true }) {
                        Icon(
                            Icons.Default.Subtitles,
                            contentDescription = "字幕",
                            tint = Color.White
                        )
                    }
                }
            }
        }

        // 播放模式标签
        if (uiState.playbackMode != null) {
            Surface(
                modifier = Modifier
                    .align(Alignment.TopEnd)
                    .padding(top = 56.dp, end = 8.dp)
                    .statusBarsPadding(),
                shape = MaterialTheme.shapes.small,
                color = Color.Black.copy(alpha = 0.6f)
            ) {
                Text(
                    text = when (uiState.playbackMode) {
                        PlaybackMode.DIRECT -> "直接播放"
                        PlaybackMode.REMUX -> "Remux"
                        PlaybackMode.HLS -> "HLS 转码"
                        PlaybackMode.PREPROCESSED_HLS -> "预处理 HLS"
                        else -> ""
                    },
                    style = MaterialTheme.typography.labelSmall,
                    color = Color.White,
                    modifier = Modifier.padding(horizontal = 8.dp, vertical = 4.dp)
                )
            }
        }

        // 手势提示
        if (gestureInfo != null) {
            Surface(
                modifier = Modifier.align(Alignment.Center),
                shape = MaterialTheme.shapes.medium,
                color = Color.Black.copy(alpha = 0.7f)
            ) {
                Text(
                    text = gestureInfo!!,
                    style = MaterialTheme.typography.titleMedium,
                    color = Color.White,
                    modifier = Modifier.padding(horizontal = 24.dp, vertical = 12.dp)
                )
            }
        }

        // 加载中
        if (uiState.loading) {
            CircularProgressIndicator(
                modifier = Modifier.align(Alignment.Center),
                color = Color.White
            )
        }

        // 降级提示
        if (uiState.fallbackMessage != null) {
            Snackbar(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .padding(16.dp)
            ) {
                Text(uiState.fallbackMessage!!)
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
    private val tokenManager: TokenManager
) : ViewModel() {

    private val _uiState = MutableStateFlow(PlayerUiState())
    val uiState = _uiState.asStateFlow()

    private var currentMediaId: String = ""
    var serverUrl: String = ""
        private set
    var token: String = ""
        private set
    private var triedModes = mutableSetOf<PlaybackMode>()

    fun loadStreamInfo(mediaId: String) {
        currentMediaId = mediaId
        viewModelScope.launch {
            _uiState.value = _uiState.value.copy(loading = true)

            serverUrl = tokenManager.getServerUrl() ?: ""
            token = tokenManager.getToken() ?: ""

            // 获取播放进度
            mediaRepository.getProgress(mediaId).onSuccess { history ->
                if (history != null && !history.completed) {
                    _uiState.value = _uiState.value.copy(resumePosition = history.position)
                }
            }

            // 获取流信息
            mediaRepository.getStreamInfo(mediaId).onSuccess { info ->
                _uiState.value = _uiState.value.copy(streamInfo = info)
                resolvePlaybackMode(info)
            }.onFailure { e ->
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
     * 智能播放模式选择
     * 优先级：预处理 HLS > Direct > Remux > HLS 转码
     */
    private fun resolvePlaybackMode(info: StreamInfo) {
        val baseUrl = "$serverUrl/api/stream/$currentMediaId"

        when {
            info.preprocessed -> {
                val url = "$serverUrl/api/preprocess/media/$currentMediaId/master.m3u8?token=$token"
                setPlayback(PlaybackMode.PREPROCESSED_HLS, url)
            }
            info.canDirectPlay && isDeviceSupported(info.videoCodec) -> {
                val url = "$baseUrl/direct?token=$token"
                setPlayback(PlaybackMode.DIRECT, url)
            }
            info.canRemux -> {
                val url = "$baseUrl/remux?token=$token"
                setPlayback(PlaybackMode.REMUX, url)
            }
            else -> {
                fallbackToHLS()
            }
        }
    }

    private fun setPlayback(mode: PlaybackMode, url: String) {
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

        val nextMode = when (currentMode) {
            PlaybackMode.DIRECT -> if (PlaybackMode.REMUX !in triedModes) PlaybackMode.REMUX else PlaybackMode.HLS
            PlaybackMode.REMUX -> PlaybackMode.HLS
            PlaybackMode.PREPROCESSED_HLS -> PlaybackMode.HLS
            PlaybackMode.HLS -> null
        }

        if (nextMode != null && nextMode !in triedModes) {
            val baseUrl = "$serverUrl/api/stream/$currentMediaId"

            val url = when (nextMode) {
                PlaybackMode.DIRECT -> "$baseUrl/direct?token=$token"
                PlaybackMode.REMUX -> "$baseUrl/remux?token=$token"
                PlaybackMode.HLS -> "$baseUrl/master.m3u8?token=$token"
                PlaybackMode.PREPROCESSED_HLS -> "$serverUrl/api/preprocess/media/$currentMediaId/master.m3u8?token=$token"
            }

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
