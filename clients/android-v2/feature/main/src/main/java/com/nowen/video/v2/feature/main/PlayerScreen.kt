package com.nowen.video.v2.feature.main

import android.os.SystemClock
import androidx.activity.compose.BackHandler
import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.navigationBars
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.statusBars
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
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
import androidx.media3.common.PlaybackException
import androidx.media3.common.Player
import androidx.media3.common.Tracks
import androidx.media3.common.util.UnstableApi
import androidx.media3.datasource.DefaultDataSource
import androidx.media3.datasource.DefaultHttpDataSource
import androidx.media3.exoplayer.ExoPlayer
import androidx.media3.exoplayer.source.DefaultMediaSourceFactory
import androidx.media3.ui.PlayerView
import com.nowen.video.v2.core.data.CatalogRepository
import com.nowen.video.v2.core.data.PlayerPreferencesStore
import com.nowen.video.v2.core.data.ProgressRepository
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.MessagePanel
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.SubtitleTrack
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlin.math.abs
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

private const val PERIODIC_PROGRESS_INTERVAL_MS = 10_000L
private const val MIN_PROGRESS_DELTA_MS = 2_000L
private const val MIN_PROGRESS_INTERVAL_MS = 8_000L
private const val NEXT_EPISODE_COUNTDOWN_SECONDS = 5

data class PlayerUiState(
    val loading: Boolean = true,
    val title: String = "",
    val playbackUrl: String = "",
    val resumePositionMs: Long = 0L,
    val mediaDurationMs: Long = 0L,
    val externalSubtitles: List<SubtitleTrack> = emptyList(),
    val nextEpisode: MediaDetail? = null,
    val playbackSpeed: Float = 1f,
    val resizeMode: Int = 0,
    val autoPlayNext: Boolean = true,
    val progressQueued: Boolean = false,
    val error: String? = null,
)

@HiltViewModel
class PlayerViewModel @Inject constructor(
    private val repository: CatalogRepository,
    private val progressRepository: ProgressRepository,
    private val preferencesStore: PlayerPreferencesStore,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    private val _state = MutableStateFlow(PlayerUiState())
    val state: StateFlow<PlayerUiState> = _state
    private var loadedId: String? = null
    private var lastReportedPositionMs = -1L
    private var lastReportElapsedMs = 0L

    init {
        viewModelScope.launch {
            preferencesStore.preferences.collectLatest { preferences ->
                _state.update {
                    it.copy(
                        playbackSpeed = preferences.playbackSpeed,
                        resizeMode = preferences.resizeMode,
                        autoPlayNext = preferences.autoPlayNext,
                    )
                }
            }
        }
    }

    fun load(mediaId: String) {
        if (loadedId == mediaId && _state.value.playbackUrl.isNotBlank()) return
        loadedId = mediaId
        viewModelScope.launch {
            _state.update {
                it.copy(
                    loading = true,
                    title = "",
                    playbackUrl = "",
                    resumePositionMs = 0L,
                    mediaDurationMs = 0L,
                    externalSubtitles = emptyList(),
                    nextEpisode = null,
                    error = null,
                )
            }
            val stream = repository.stream(mediaId).getOrElse { error ->
                _state.update { it.copy(loading = false, error = error.message ?: "播放信息加载失败") }
                return@launch
            }
            val resolved = resolveServerResource(
                sessionStore.snapshot.value.activeServer?.baseUrl,
                stream.preferredUrl,
            )
            if (resolved.isNullOrBlank()) {
                _state.update { it.copy(loading = false, error = "服务器没有返回可播放地址") }
                return@launch
            }

            val detail = repository.detail(mediaId).getOrNull()
            val subtitles = repository.subtitles(mediaId).getOrNull()
            val nextEpisode = detail
                ?.takeIf { it.seriesId.isNotBlank() && it.episodeNumber > 0 }
                ?.let {
                    repository.nextEpisode(
                        seriesId = it.seriesId,
                        season = it.seasonNumber,
                        episode = it.episodeNumber,
                    ).getOrNull()
                }
            val resumeSeconds = progressRepository.restorePosition(mediaId, stream.duration)
            _state.update {
                it.copy(
                    loading = false,
                    title = detail?.displayTitle?.takeIf(String::isNotBlank) ?: stream.title,
                    playbackUrl = resolved,
                    resumePositionMs = (resumeSeconds * 1_000).toLong().coerceAtLeast(0L),
                    mediaDurationMs = (stream.duration * 1_000).toLong().coerceAtLeast(0L),
                    externalSubtitles = subtitles?.external.orEmpty(),
                    nextEpisode = nextEpisode,
                )
            }
        }
    }

    fun setPlaybackSpeed(speed: Float) {
        _state.update { it.copy(playbackSpeed = speed) }
        viewModelScope.launch { preferencesStore.setPlaybackSpeed(speed) }
    }

    fun setResizeMode(mode: Int) {
        _state.update { it.copy(resizeMode = mode) }
        viewModelScope.launch { preferencesStore.setResizeMode(mode) }
    }

    fun setAutoPlayNext(enabled: Boolean) {
        _state.update { it.copy(autoPlayNext = enabled) }
        viewModelScope.launch { preferencesStore.setAutoPlayNext(enabled) }
    }

    fun onPlayerError(error: PlaybackException) {
        _state.update {
            it.copy(error = error.errorCodeName.ifBlank { error.message ?: "播放器发生错误" })
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

@androidx.annotation.OptIn(UnstableApi::class)
@Composable
fun PlayerScreen(
    mediaId: String,
    onBack: () -> Unit,
    onPlayNext: (String) -> Unit,
    viewModel: PlayerViewModel = hiltViewModel(),
) {
    val state by viewModel.state.collectAsState()
    val session by viewModel.sessionStore.snapshot.collectAsState()
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val token = session.token.orEmpty()
    var showSettings by rememberSaveable { mutableStateOf(false) }
    var audioTracks by remember { mutableStateOf(emptyList<PlayerTrackChoice>()) }
    var subtitleTracks by remember { mutableStateOf(emptyList<PlayerTrackChoice>()) }
    var audioAutomatic by remember { mutableStateOf(true) }
    var subtitlesDisabled by remember { mutableStateOf(false) }
    var playerView by remember { mutableStateOf<PlayerView?>(null) }
    var showNextEpisodePanel by rememberSaveable(mediaId) { mutableStateOf(false) }
    var nextEpisodeCountdown by rememberSaveable(mediaId) {
        mutableIntStateOf(NEXT_EPISODE_COUNTDOWN_SECONDS)
    }

    LaunchedEffect(mediaId) { viewModel.load(mediaId) }

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

    BackHandler {
        reportCurrentProgress(force = true)
        onBack()
    }

    LaunchedEffect(state.playbackUrl, state.resumePositionMs, state.externalSubtitles, session.activeServer?.baseUrl) {
        if (state.playbackUrl.isNotBlank()) {
            val item = MediaItem.Builder()
                .setUri(state.playbackUrl)
                .setSubtitleConfigurations(
                    externalSubtitleConfigurations(
                        baseUrl = session.activeServer?.baseUrl,
                        tracks = state.externalSubtitles,
                    ),
                )
                .build()
            if (state.resumePositionMs > 0L) {
                player.setMediaItem(item, state.resumePositionMs)
            } else {
                player.setMediaItem(item)
            }
            player.prepare()
            player.playWhenReady = true
        }
    }

    LaunchedEffect(state.playbackSpeed) {
        player.setPlaybackSpeed(state.playbackSpeed)
    }

    LaunchedEffect(state.resizeMode, playerView) {
        playerView?.resizeMode = resizeModeForPreference(state.resizeMode)
    }

    LaunchedEffect(player, mediaId) {
        while (true) {
            delay(PERIODIC_PROGRESS_INTERVAL_MS)
            if (player.isPlaying) reportCurrentProgress(force = false)
        }
    }

    LaunchedEffect(showNextEpisodePanel, state.autoPlayNext, state.nextEpisode?.id) {
        if (!showNextEpisodePanel || !state.autoPlayNext || state.nextEpisode == null) return@LaunchedEffect
        for (remaining in NEXT_EPISODE_COUNTDOWN_SECONDS downTo 1) {
            nextEpisodeCountdown = remaining
            delay(1_000)
        }
        val next = state.nextEpisode ?: return@LaunchedEffect
        showNextEpisodePanel = false
        onPlayNext(next.id)
    }

    DisposableEffect(
        player,
        mediaId,
        lifecycleOwner,
        state.nextEpisode?.id,
        state.autoPlayNext,
        state.mediaDurationMs,
    ) {
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
                    if (state.nextEpisode != null) {
                        nextEpisodeCountdown = NEXT_EPISODE_COUNTDOWN_SECONDS
                        showNextEpisodePanel = true
                    }
                }
            }

            override fun onTracksChanged(tracks: Tracks) {
                audioTracks = extractTrackChoices(tracks, C.TRACK_TYPE_AUDIO)
                subtitleTracks = extractTrackChoices(tracks, C.TRACK_TYPE_TEXT)
            }

            override fun onPlayerError(error: PlaybackException) {
                viewModel.onPlayerError(error)
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
                        controllerShowTimeoutMs = 3_500
                        this.player = player
                        keepScreenOn = true
                        resizeMode = resizeModeForPreference(state.resizeMode)
                        playerView = this
                    }
                },
                update = {
                    it.player = player
                    it.resizeMode = resizeModeForPreference(state.resizeMode)
                    playerView = it
                },
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
                contentDescription = "退出播放",
                tint = Color.White,
            )
        }

        IconButton(
            onClick = { showSettings = true },
            modifier = Modifier
                .align(Alignment.TopEnd)
                .windowInsetsPadding(WindowInsets.statusBars)
                .padding(8.dp)
                .background(Color.Black.copy(alpha = 0.55f), MaterialTheme.shapes.large),
        ) {
            Icon(Icons.Default.Settings, contentDescription = "播放设置", tint = Color.White)
        }

        if (state.progressQueued && !showNextEpisodePanel) {
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

        if (showNextEpisodePanel && state.nextEpisode != null) {
            val next = state.nextEpisode!!
            Surface(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .windowInsetsPadding(WindowInsets.navigationBars)
                    .padding(16.dp)
                    .fillMaxWidth(),
                shape = MaterialTheme.shapes.extraLarge,
                color = Color.Black.copy(alpha = 0.86f),
                tonalElevation = 8.dp,
            ) {
                Column(Modifier.padding(18.dp)) {
                    Text(
                        text = if (state.autoPlayNext) {
                            "$nextEpisodeCountdown 秒后播放下一集"
                        } else {
                            "下一集已准备好"
                        },
                        color = Color.White,
                        style = MaterialTheme.typography.titleMedium,
                    )
                    Text(
                        text = next.displayTitle,
                        color = Color.White.copy(alpha = 0.72f),
                        style = MaterialTheme.typography.bodyMedium,
                        modifier = Modifier.padding(top = 4.dp),
                    )
                    Row(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = 14.dp),
                        horizontalArrangement = Arrangement.End,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        TextButton(onClick = { showNextEpisodePanel = false }) {
                            Text("取消")
                        }
                        Spacer(Modifier.width(8.dp))
                        Button(
                            onClick = {
                                showNextEpisodePanel = false
                                onPlayNext(next.id)
                            },
                        ) {
                            Text("立即播放")
                        }
                    }
                }
            }
        }
    }

    if (showSettings) {
        PlayerSettingsSheet(
            onDismiss = { showSettings = false },
            playbackSpeed = state.playbackSpeed,
            onPlaybackSpeedChange = viewModel::setPlaybackSpeed,
            resizeMode = state.resizeMode,
            onResizeModeChange = viewModel::setResizeMode,
            autoPlayNext = state.autoPlayNext,
            onAutoPlayNextChange = viewModel::setAutoPlayNext,
            audioTracks = audioTracks,
            audioAutomatic = audioAutomatic,
            onAudioTrackSelected = { choice ->
                audioAutomatic = choice == null
                player.trackSelectionParameters = applyTrackChoice(
                    tracks = player.trackSelectionParameters,
                    trackType = C.TRACK_TYPE_AUDIO,
                    choice = choice,
                )
            },
            subtitleTracks = subtitleTracks,
            subtitlesDisabled = subtitlesDisabled,
            onSubtitleTrackSelected = { choice ->
                subtitlesDisabled = choice == null
                player.trackSelectionParameters = if (choice == null) {
                    disableTextTracks(player.trackSelectionParameters)
                } else {
                    applyTrackChoice(
                        tracks = player.trackSelectionParameters,
                        trackType = C.TRACK_TYPE_TEXT,
                        choice = choice,
                    )
                }
            },
        )
    }
}

internal fun resolveServerResource(baseUrl: String?, path: String?): String? {
    if (path.isNullOrBlank()) return null
    if (path.startsWith("http://") || path.startsWith("https://")) return path
    return baseUrl?.trimEnd('/') + "/" + path.trimStart('/')
}
