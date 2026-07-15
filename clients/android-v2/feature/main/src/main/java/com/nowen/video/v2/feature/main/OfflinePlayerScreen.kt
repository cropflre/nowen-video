package com.nowen.video.v2.feature.main

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
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
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
import com.nowen.video.v2.core.data.ProgressRepository
import com.nowen.video.v2.core.designsystem.MessagePanel
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
) : ViewModel() {
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
    onBack: () -> Unit,
    viewModel: OfflinePlayerViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val player = remember { ExoPlayer.Builder(context).build() }

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

    DisposableEffect(player, mediaId, lifecycleOwner, state.durationMs) {
        val listener = object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (!isPlaying && player.playbackState == Player.STATE_READY) {
                    reportCurrentProgress(force = true)
                }
            }

            override fun onPlaybackStateChanged(playbackState: Int) {
                if (playbackState == Player.STATE_ENDED) {
                    val duration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
                        ?: state.durationMs
                    viewModel.reportProgress(mediaId, duration, duration, force = true)
                }
            }
        }
        val lifecycleObserver = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_STOP) reportCurrentProgress(force = true)
        }
        player.addListener(listener)
        lifecycleOwner.lifecycle.addObserver(lifecycleObserver)
        onDispose {
            reportCurrentProgress(force = true)
            lifecycleOwner.lifecycle.removeObserver(lifecycleObserver)
            player.removeListener(listener)
            player.release()
        }
    }

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
            state.error != null -> MessagePanel(
                title = "无法离线播放",
                message = state.error!!,
                actionLabel = "返回",
                onAction = onBack,
                modifier = Modifier
                    .align(Alignment.Center)
                    .padding(20.dp),
            )
            else -> AndroidView(
                factory = { viewContext ->
                    PlayerView(viewContext).apply {
                        useController = true
                        controllerShowTimeoutMs = 3_500
                        this.player = player
                        keepScreenOn = true
                    }
                },
                update = { it.player = player },
                modifier = Modifier.fillMaxSize(),
            )
        }

        IconButton(
            onClick = {
                reportCurrentProgress(force = true)
                onBack()
            },
            modifier = Modifier
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(8.dp)
                .background(Color.Black.copy(alpha = 0.55f), MaterialTheme.shapes.large),
        ) {
            Icon(
                Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "退出离线播放",
                tint = Color.White,
            )
        }

        Surface(
            modifier = Modifier
                .align(Alignment.TopCenter)
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(top = 12.dp),
            color = Color.Black.copy(alpha = 0.68f),
            shape = MaterialTheme.shapes.large,
        ) {
            Text(
                text = "离线播放 · ${state.title}",
                color = Color.White,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(horizontal = 14.dp, vertical = 8.dp),
            )
        }

        if (state.progressQueued) {
            Surface(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .windowInsetsPadding(WindowInsets.navigationBars)
                    .padding(16.dp),
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
}
