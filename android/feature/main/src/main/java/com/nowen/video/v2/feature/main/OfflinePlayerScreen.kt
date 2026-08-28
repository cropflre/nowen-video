@file:androidx.annotation.OptIn(androidx.media3.common.util.UnstableApi::class)

package com.nowen.video.v2.feature.main

import android.media.AudioManager
import android.os.SystemClock
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.viewinterop.AndroidView
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.viewModelScope
import androidx.media3.common.C
import androidx.media3.common.MediaItem
import androidx.media3.common.Player
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.ui.PlayerView
import com.nowen.video.v2.core.data.OfflineDownloadRepository
import com.nowen.video.v2.core.data.PlayerPreferences
import com.nowen.video.v2.core.data.PlayerPreferencesStore
import com.nowen.video.v2.core.data.ProgressRepository
import com.nowen.video.v2.core.designsystem.HillsState
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlin.math.abs
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

private const val OFFLINE_PROGRESS_INTERVAL_MS = 10_000L
private const val OFFLINE_MIN_PROGRESS_DELTA_MS = 2_000L
private const val OFFLINE_MIN_PROGRESS_INTERVAL_MS = 8_000L
private const val OFFLINE_PLAYER_CONTROLS_TIMEOUT_MS = 3_500L
private const val OFFLINE_NOTICE_DURATION_MS = 4_000L

data class OfflinePlayerUiState(
    val loading: Boolean = true,
    val title: String = "",
    val playbackUri: String = "",
    val durationMs: Long = 0L,
    val resumePositionMs: Long = 0L,
    val progressQueued: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
class OfflinePlayerViewModel @Inject constructor(
    private val downloads: OfflineDownloadRepository,
    private val progressRepository: ProgressRepository,
    private val playerPreferencesStore: PlayerPreferencesStore,
) : ViewModel() {
    val playerPreferences = playerPreferencesStore.preferences

    fun setPlaybackSpeed(speed: Float) = viewModelScope.launch {
        playerPreferencesStore.setPlaybackSpeed(speed)
    }

    fun setLongPressBoostSpeed(speed: Float) = viewModelScope.launch {
        playerPreferencesStore.setLongPressBoostSpeed(speed)
    }

    fun setResizeMode(mode: Int) = viewModelScope.launch {
        playerPreferencesStore.setResizeMode(mode)
    }

    fun setAutoPlayNext(enabled: Boolean) = viewModelScope.launch {
        playerPreferencesStore.setAutoPlayNext(enabled)
    }

    private val _state = MutableStateFlow(OfflinePlayerUiState())
    val state: StateFlow<OfflinePlayerUiState> = _state
    private var loadedMediaId: String? = null
    private var lastReportedPositionMs = -1L
    private var lastReportElapsedMs = 0L

    fun load(mediaId: String) {
        if (loadedMediaId == mediaId && _state.value.playbackUri.isNotBlank()) return
        loadedMediaId = mediaId
        viewModelScope.launch {
            _state.value = OfflinePlayerUiState(loading = true)
            val playback = downloads.localPlayback(mediaId)
            if (playback == null) {
                _state.value = OfflinePlayerUiState(
                    loading = false,
                    error = "离线文件不存在或尚未下载完成",
                )
                return@launch
            }
            val resumeSeconds = progressRepository.restoreOfflinePosition(mediaId, playback.durationSeconds)
            _state.value = OfflinePlayerUiState(
                loading = false,
                title = playback.title,
                playbackUri = playback.uri,
                durationMs = (playback.durationSeconds * 1_000).toLong().coerceAtLeast(0L),
                resumePositionMs = (resumeSeconds * 1_000).toLong().coerceAtLeast(0L),
            )
        }
    }

    fun reportProgress(
        mediaId: String,
        positionMs: Long,
        durationMs: Long,
        force: Boolean = false,
    ) {
        if (mediaId.isBlank() || positionMs <= 0L || durationMs <= 0L) return
        val now = SystemClock.elapsedRealtime()
        if (!force &&
            abs(positionMs - lastReportedPositionMs) < OFFLINE_MIN_PROGRESS_DELTA_MS &&
            now - lastReportElapsedMs < OFFLINE_MIN_PROGRESS_INTERVAL_MS
        ) {
            return
        }
        lastReportedPositionMs = positionMs
        lastReportElapsedMs = now
        viewModelScope.launch(start = CoroutineStart.UNDISPATCHED) {
            withContext(NonCancellable + Dispatchers.IO) {
                val delivery = progressRepository.report(
                    mediaId = mediaId,
                    position = positionMs / 1_000.0,
                    duration = durationMs / 1_000.0,
                )
                _state.update { it.copy(progressQueued = delivery.queued) }
            }
        }
    }
}

@Composable
fun OfflinePlayerScreen(
    mediaId: String,
    pictureInPictureAvailable: Boolean = false,
    onBack: () -> Unit,
    viewModel: OfflinePlayerViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val playerPreferences by viewModel.playerPreferences.collectAsState(initial = PlayerPreferences())
    val context = LocalContext.current
    val pictureInPictureHost = remember(context) { context.findPlaybackPictureInPictureHost() }
    val audioManager = remember(context) { context.getSystemService(AudioManager::class.java) }
    val lifecycleOwner = LocalLifecycleOwner.current
    val player = remember(mediaId) { ExoPlayer.Builder(context).build() }
    var showSettings by rememberSaveable { mutableStateOf(false) }
    var controlsVisible by rememberSaveable(mediaId) { mutableStateOf(false) }
    var controlsEpoch by remember(mediaId) { mutableStateOf(0) }
    var displayPositionMs by remember(mediaId) { mutableStateOf(0L) }
    var playerDurationMs by remember(mediaId) { mutableStateOf(0L) }
    var seekPreviewMs by remember(mediaId) { mutableStateOf<Long?>(null) }
    var isPlaying by remember(mediaId) { mutableStateOf(false) }
    var boostingSpeed by remember(mediaId) { mutableStateOf<Float?>(null) }
    var brightnessValue by remember(mediaId) { mutableStateOf(0.5f) }
    var volumeValue by remember(mediaId) { mutableStateOf(0f) }
    var gestureNotice by remember(mediaId) { mutableStateOf<String?>(null) }

    DisposableEffect(pictureInPictureHost) {
        brightnessValue = pictureInPictureHost?.currentPlaybackScreenBrightness() ?: 0.5f
        volumeValue = audioManager?.let { manager ->
            val maximum = manager.getStreamMaxVolume(AudioManager.STREAM_MUSIC)
            if (maximum > 0) manager.getStreamVolume(AudioManager.STREAM_MUSIC).toFloat() / maximum else 0f
        } ?: 0f
        pictureInPictureHost?.setPlaybackLandscape(true)
        onDispose {
            pictureInPictureHost?.restorePlaybackScreenBrightness()
            pictureInPictureHost?.setPlaybackLandscape(false)
        }
    }

    fun revealControls() {
        controlsVisible = true
        controlsEpoch += 1
    }

    fun effectiveDurationMs(): Long = playerDurationMs.takeIf { it > 0L } ?: state.durationMs

    fun seekToAbsolute(targetMs: Long) {
        val duration = effectiveDurationMs()
        if (duration <= 0L) return
        val target = targetMs.coerceIn(0L, duration)
        seekPreviewMs = null
        displayPositionMs = target
        player.seekTo(target)
        revealControls()
    }

    fun seekBy(deltaMs: Long) {
        seekToAbsolute((seekPreviewMs ?: displayPositionMs) + deltaMs)
    }


    LaunchedEffect(mediaId) { viewModel.load(mediaId) }

    fun reportCurrentProgress(force: Boolean) {
        val playerDuration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
        viewModel.reportProgress(
            mediaId = mediaId,
            positionMs = player.currentPosition.coerceAtLeast(0L),
            durationMs = (playerDuration ?: state.durationMs).coerceAtLeast(0L),
            force = force,
        )
    }

    BackHandler {
        reportCurrentProgress(force = true)
        onBack()
    }

    LaunchedEffect(state.playbackUri, state.resumePositionMs) {
        if (state.playbackUri.isNotBlank()) {
            val item = MediaItem.fromUri(state.playbackUri)
            if (state.resumePositionMs > 0L) {
                player.setMediaItem(item, state.resumePositionMs)
            } else {
                player.setMediaItem(item)
            }
            player.prepare()
            player.playWhenReady = true
        }
    }

    LaunchedEffect(player, mediaId) {
        while (true) {
            delay(OFFLINE_PROGRESS_INTERVAL_MS)
            if (player.isPlaying) reportCurrentProgress(force = false)
        }
    }

    val latestState by rememberUpdatedState(state)
    val latestReportProgress by rememberUpdatedState(::reportCurrentProgress)
    DisposableEffect(player, mediaId, lifecycleOwner) {
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(playing: Boolean) {
                isPlaying = playing
                if (!playing && player.playbackState == Player.STATE_READY) {
                    latestReportProgress(true)
                    revealControls()
                }
            }

            override fun onPlaybackStateChanged(playbackState: Int) {
                val duration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
                if (duration != null) playerDurationMs = duration
                if (playbackState == Player.STATE_ENDED) {
                    val end = duration ?: latestState.durationMs
                    viewModel.reportProgress(mediaId, end, end, force = true)
                }
            }
        }
        val lifecycleObserver = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_STOP) latestReportProgress(true)
        }
        player.addListener(listener)
        lifecycleOwner.lifecycle.addObserver(lifecycleObserver)
        onDispose {
            latestReportProgress(true)
            lifecycleOwner.lifecycle.removeObserver(lifecycleObserver)
            player.removeListener(listener)
            player.release()
        }
    }

    LaunchedEffect(player, playerPreferences.playbackSpeed, boostingSpeed) {
        if (boostingSpeed == null) player.setPlaybackSpeed(playerPreferences.playbackSpeed)
    }

    LaunchedEffect(player, mediaId, state.durationMs) {
        while (true) {
            displayPositionMs = player.currentPosition.coerceAtLeast(0L)
            playerDurationMs = player.duration.takeIf { it != C.TIME_UNSET && it > 0L } ?: state.durationMs
            delay(250L)
        }
    }

    LaunchedEffect(controlsVisible, controlsEpoch, isPlaying, showSettings) {
        if (controlsVisible && isPlaying && !showSettings) {
            delay(OFFLINE_PLAYER_CONTROLS_TIMEOUT_MS)
            controlsVisible = false
        }
    }

    LaunchedEffect(gestureNotice) {
        if (gestureNotice != null) {
            delay(OFFLINE_NOTICE_DURATION_MS)
            gestureNotice = null
        }
    }

    val durationMs = effectiveDurationMs()
    Box(
        modifier = Modifier
            .fillMaxSize()
            .background(Color.Black),
    ) {
        when {
            state.loading -> CircularProgressIndicator(
                modifier = Modifier.align(Alignment.Center),
                color = MaterialTheme.colorScheme.primary,
            )
            state.error != null -> HillsState(
                title = "无法离线播放",
                message = state.error!!,
                actionLabel = "返回",
                onAction = onBack,
                modifier = Modifier.align(Alignment.Center),
            )
            else -> {
                AndroidView(
                    factory = { viewContext ->
                        PlayerView(viewContext).apply {
                            useController = false
                            this.player = player
                            keepScreenOn = true
                        }
                    },
                    update = {
                        it.useController = false
                        it.player = player
                    },
                    modifier = Modifier.fillMaxSize(),
                )

                PlayerGestureLayer(
                    currentSpeed = playerPreferences.playbackSpeed,
                    longPressBoostSpeed = playerPreferences.longPressBoostSpeed,
                    enabled = !showSettings,
                    onTap = {
                        controlsVisible = !controlsVisible
                        controlsEpoch += 1
                    },
                    onSeekBy = ::seekBy,
                    onSeekDelta = { delta -> if (delta != 0L) seekBy(delta) },
                    onBrightnessChange = { delta ->
                        brightnessValue = (brightnessValue - delta).coerceIn(0f, 1f)
                        pictureInPictureHost?.setPlaybackScreenBrightness(brightnessValue)
                        gestureNotice = "亮度 ${(brightnessValue * 100).toInt()}%"
                    },
                    onVolumeChange = { delta ->
                        val manager = audioManager
                        val maximum = manager?.getStreamMaxVolume(AudioManager.STREAM_MUSIC) ?: 0
                        if (manager != null && maximum > 0) {
                            volumeValue = (volumeValue - delta).coerceIn(0f, 1f)
                            manager.setStreamVolume(
                                AudioManager.STREAM_MUSIC,
                                (volumeValue * maximum).toInt().coerceIn(0, maximum),
                                0,
                            )
                            gestureNotice = "音量 ${(volumeValue * 100).toInt()}%"
                        }
                    },
                    onBoostStart = { speed ->
                        boostingSpeed = speed
                        player.setPlaybackSpeed(speed)
                    },
                    onBoostEnd = { restoreSpeed ->
                        boostingSpeed = null
                        player.setPlaybackSpeed(restoreSpeed)
                    },
                    modifier = Modifier.fillMaxSize(),
                )

                NowenPlayerControls(
                    visible = controlsVisible && !showSettings,
                    title = state.title,
                    isPlaying = isPlaying,
                    positionMs = displayPositionMs,
                    durationMs = durationMs,
                    seekPreviewMs = seekPreviewMs,
                    playbackSpeed = playerPreferences.playbackSpeed,
                    boostingSpeed = boostingSpeed,
                    seekingEnabled = durationMs > 0L,
                    onBack = {
                        reportCurrentProgress(force = true)
                        onBack()
                    },
                    onSettings = {
                        showSettings = true
                        controlsVisible = false
                    },
                    resizeMode = playerPreferences.resizeMode,
                    onResizeModeChange = viewModel::setResizeMode,
                    onPictureInPicture = {
                        if (pictureInPictureAvailable) pictureInPictureHost?.enterPlaybackPictureInPicture()
                    },
                    pictureInPictureAvailable = pictureInPictureAvailable,
                    onPlayPause = {
                        if (player.isPlaying) player.pause() else player.play()
                        revealControls()
                    },
                    onSeekBy = ::seekBy,
                    onSeekFractionChange = { fraction ->
                        if (durationMs > 0L) seekPreviewMs = (durationMs * fraction).toLong().coerceIn(0L, durationMs)
                        revealControls()
                    },
                    onSeekFinished = { seekToAbsolute(seekPreviewMs ?: displayPositionMs) },
                    onSpeedClick = {
                        showSettings = true
                        controlsVisible = false
                    },
                    onSpeedDecrease = {
                        viewModel.setPlaybackSpeed(neighborPlaybackSpeed(playerPreferences.playbackSpeed, -1))
                        revealControls()
                    },
                    onSpeedIncrease = {
                        viewModel.setPlaybackSpeed(neighborPlaybackSpeed(playerPreferences.playbackSpeed, 1))
                        revealControls()
                    },
                    episodeLabel = playerEpisodeLabel(state.title),
                    modifier = Modifier.fillMaxSize(),
                )
            }
        }

        if (gestureNotice != null && !showSettings) {
            Surface(
                modifier = Modifier.align(Alignment.Center),
                shape = MaterialTheme.shapes.extraLarge,
                color = Color.Black.copy(alpha = 0.72f),
            ) {
                Text(
                    text = gestureNotice.orEmpty(),
                    color = Color.White,
                    style = MaterialTheme.typography.titleMedium,
                    modifier = Modifier.padding(horizontal = 22.dp, vertical = 14.dp),
                )
            }
        }

        if (state.progressQueued && !showSettings) {
            Surface(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .windowInsetsPadding(WindowInsets.navigationBars)
                    .padding(horizontal = 16.dp, vertical = if (controlsVisible) 86.dp else 16.dp),
                color = Color.Black.copy(alpha = 0.76f),
                shape = MaterialTheme.shapes.large,
            ) {
                Text(
                    "观看进度将在恢复连接后自动同步",
                    color = Color.White,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 10.dp),
                )
            }
        }
    }

    if (showSettings) {
        PlayerSettingsSheet(
            onDismiss = {
                showSettings = false
                revealControls()
            },
            playbackDiagnostics = PlaybackDiagnostics(methodLabel = "离线播放"),
            playbackSpeed = playerPreferences.playbackSpeed,
            onPlaybackSpeedChange = viewModel::setPlaybackSpeed,
            longPressBoostSpeed = playerPreferences.longPressBoostSpeed,
            onLongPressBoostSpeedChange = viewModel::setLongPressBoostSpeed,
            resizeMode = playerPreferences.resizeMode,
            onResizeModeChange = viewModel::setResizeMode,
            autoPlayNext = playerPreferences.autoPlayNext,
            onAutoPlayNextChange = viewModel::setAutoPlayNext,
            audioTracks = emptyList(),
            audioAutomatic = true,
            onAudioTrackSelected = {},
            subtitleTracks = emptyList(),
            subtitlesDisabled = true,
            onSubtitleTrackSelected = {},
            danmakuAvailable = false,
            danmakuOffline = true,
        )
    }
}
