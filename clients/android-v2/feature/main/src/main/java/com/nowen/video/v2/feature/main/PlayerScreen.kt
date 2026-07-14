package com.nowen.video.v2.feature.main

import android.os.SystemClock
import androidx.activity.compose.BackHandler
import androidx.annotation.OptIn
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
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.PlayerView
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.ProgressRepository
import com.nowen.video.v2.core.data.ServerSessionStore
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

private const val PERIODIC_PROGRESS_INTERVAL_MS = 10_000L
private const val MIN_PROGRESS_DELTA_MS = 2_000L
private const val MIN_PROGRESS_INTERVAL_MS = 8_000L

data class PlayerUiState(
    val loading: Boolean = true,
    val title: String = "",
    val playbackUrl: String = "",
    val resumePositionMs: Long = 0L,
    val mediaDurationMs: Long = 0L,
    val progressQueued: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
class PlayerViewModel @Inject constructor(
    private val repository: CatalogRepository,
    private val progressRepository: ProgressRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(PlayerUiState())
    val state: StateFlow<PlayerUiState> = _state
    private var loadedId: String? = null
    private var lastReportedPositionMs = -1L
    private var lastReportElapsedMs = 0L

    fun load(mediaId: String) {
        if (loadedId == mediaId && _state.value.playbackUrl.isNotBlank()) return
        loadedId = mediaId
        viewModelScope.launch {
            _state.value = PlayerUiState(loading = true)
            val streamResult = repository.stream(mediaId)
            val stream = streamResult.getOrElse { error ->
                _state.update { it.copy(loading = false, error = error.message ?: "播放信息加载失败") }
                return@launch
            }
            val resolved = resolveServerResource(
                sessionStore.snapshot.value.activeServer?.baseUrl,
                stream.preferredUrl,
            )
            if (resolved.isNullOrBlank()) {
                _state.value = PlayerUiState(loading = false, error = "服务器没有返回可播放地址")
                return@launch
            }
            val resumeSeconds = progressRepository.restorePosition(mediaId, stream.duration)
            _state.value = PlayerUiState(
                loading = false,
                title = stream.title,
                playbackUrl = resolved,
                resumePositionMs = (resumeSeconds * 1_000).toLong().coerceAtLeast(0L),
                mediaDurationMs = (stream.duration * 1_000).toLong().coerceAtLeast(0L),
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
            abs(positionMs - lastReportedPositionMs) < MIN_PROGRESS_DELTA_MS &&
            now - lastReportElapsedMs < MIN_PROGRESS_INTERVAL_MS
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

@OptIn(UnstableApi::class)
@Composable
fun PlayerScreen(
    mediaId: String,
    onBack: () -> Unit,
    viewModel: PlayerViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val token = session.token.orEmpty()

    LaunchedEffect(mediaId) { viewModel.load(mediaId) }
    BackHandler(onBack = onBack)

    val player = remember(token) {
        val httpFactory = DefaultHttpDataSource.Factory().apply {
            if (token.isNotBlank()) {
                setDefaultRequestProperties(mapOf("Authorization" to "Bearer $token"))
            }
            setAllowCrossProtocolRedirects(true)
        }
        val dataSourceFactory = DefaultDataSource.Factory(context, httpFactory)
        ExoPlayer.Builder(context)
            .setMediaSourceFactory(DefaultMediaSourceFactory(dataSourceFactory))
            .build()
    }

    fun reportCurrentProgress(force: Boolean) {
        val playerDuration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
        val duration = playerDuration ?: state.mediaDurationMs
        viewModel.reportProgress(
            mediaId = mediaId,
            positionMs = player.currentPosition.coerceAtLeast(0L),
            durationMs = duration.coerceAtLeast(0L),
            force = force,
        )
    }

    LaunchedEffect(state.playbackUrl, state.resumePositionMs) {
        if (state.playbackUrl.isNotBlank()) {
            val mediaItem = MediaItem.fromUri(state.playbackUrl)
            if (state.resumePositionMs > 0L) {
                player.setMediaItem(mediaItem, state.resumePositionMs)
            } else {
                player.setMediaItem(mediaItem)
            }
            player.prepare()
            player.playWhenReady = true
        }
    }

    LaunchedEffect(player, mediaId) {
        while (true) {
            delay(PERIODIC_PROGRESS_INTERVAL_MS)
            if (player.isPlaying) reportCurrentProgress(force = false)
        }
    }

    DisposableEffect(player, mediaId, lifecycleOwner) {
        val playerListener = object : Player.Listener {
            override fun onIsPlayingChanged(isPlaying: Boolean) {
                if (!isPlaying && player.playbackState == Player.STATE_READY) {
                    reportCurrentProgress(force = true)
                }
            }

            override fun onPlaybackStateChanged(playbackState: Int) {
                if (playbackState == Player.STATE_ENDED) {
                    val duration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
                        ?: state.mediaDurationMs
                    viewModel.reportProgress(
                        mediaId = mediaId,
                        positionMs = duration,
                        durationMs = duration,
                        force = true,
                    )
                }
            }

            override fun onPositionDiscontinuity(
                oldPosition: Player.PositionInfo,
                newPosition: Player.PositionInfo,
                reason: Int,
            ) {
                if (reason == Player.DISCONTINUITY_REASON_SEEK) {
                    reportCurrentProgress(force = true)
                }
            }
        }
        val lifecycleObserver = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_STOP) reportCurrentProgress(force = true)
        }
        player.addListener(playerListener)
        lifecycleOwner.lifecycle.addObserver(lifecycleObserver)
        onDispose {
            reportCurrentProgress(force = true)
            lifecycleOwner.lifecycle.removeObserver(lifecycleObserver)
            player.removeListener(playerListener)
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
                title = "无法播放",
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
                        this.player = player
                        keepScreenOn = true
                    }
                },
                update = { it.player = player },
                modifier = Modifier.fillMaxSize(),
            )
        }

        IconButton(
            onClick = onBack,
            modifier = Modifier
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(8.dp)
                .background(Color.Black.copy(alpha = 0.55f), MaterialTheme.shapes.large),
        ) {
            Icon(
                Icons.AutoMirrored.Filled.ArrowBack,
                contentDescription = "退出播放",
                tint = Color.White,
            )
        }

        if (state.progressQueued) {
            Surface(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .windowInsetsPadding(WindowInsets.navigationBars)
                    .padding(16.dp),
                shape = MaterialTheme.shapes.large,
                color = Color.Black.copy(alpha = 0.76f),
            ) {
                Text(
                    text = "当前离线，观看进度将在恢复连接后自动同步",
                    color = Color.White,
                    style = MaterialTheme.typography.bodyMedium,
                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 10.dp),
                )
            }
        }
    }
}

internal fun resolveServerResource(baseUrl: String?, path: String?): String? {
    if (path.isNullOrBlank()) return null
    if (path.startsWith("http://") || path.startsWith("https://")) return path
    return baseUrl?.trimEnd('/') + "/" + path.trimStart('/')
}