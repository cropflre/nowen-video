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
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
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
import com.nowen.video.v2.core.model.CreatePlaybackSessionRequest
import com.nowen.video.v2.core.model.MediaDetail
import com.nowen.video.v2.core.model.PlaybackPlan
import com.nowen.video.v2.core.model.PlaybackSessionHeartbeatRequest
import com.nowen.video.v2.core.model.PlaybackSessionResult
import com.nowen.video.v2.core.model.RestartPlaybackSessionRequest
import com.nowen.video.v2.core.model.SubtitleTrack
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlin.math.abs
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.CoroutineStart
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.NonCancellable
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.update
import kotlinx.coroutines.launch
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.coroutines.withContext

private const val PERIODIC_PROGRESS_INTERVAL_MS = 10_000L
private const val MIN_PROGRESS_DELTA_MS = 2_000L
private const val MIN_PROGRESS_INTERVAL_MS = 8_000L
private const val NEXT_EPISODE_COUNTDOWN_SECONDS = 5
private const val FALLBACK_NOTICE_DURATION_MS = 4_000L
private const val DEFAULT_SESSION_HEARTBEAT_INTERVAL_MS = 15_000L
private const val PLAYER_CONTROLS_TIMEOUT_MS = 3_500L

data class PlayerUiState(
    val loading: Boolean = true,
    val title: String = "",
    val playbackUrl: String = "",
    val resumePositionMs: Long = 0L,
    val mediaDurationMs: Long = 0L,
    val externalSubtitles: List<SubtitleTrack> = emptyList(),
    val nextEpisode: MediaDetail? = null,
    val playbackDiagnostics: PlaybackDiagnostics = PlaybackDiagnostics(),
    val fallbackNotice: String? = null,
    val playbackSpeed: Float = 1f,
    val resizeMode: Int = 0,
    val autoPlayNext: Boolean = true,
    val progressQueued: Boolean = false,
    val sessionManaged: Boolean = false,
    val sessionId: String = "",
    val sessionGenerationId: Long = 0L,
    val sessionOffsetMs: Long = 0L,
    val sessionHeartbeatIntervalMs: Long = DEFAULT_SESSION_HEARTBEAT_INTERVAL_MS,
    val sessionProfileId: String = "auto",
    val sessionMaxBitrate: Int = 0,
    val sessionRestarting: Boolean = false,
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
    private val sessionOperationMutex = Mutex()
    private val cleanupScope = CoroutineScope(SupervisorJob() + Dispatchers.IO)
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
        detachPlaybackSession("media_changed")
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
                    playbackDiagnostics = PlaybackDiagnostics(),
                    fallbackNotice = null,
                    sessionManaged = false,
                    sessionId = "",
                    sessionGenerationId = 0L,
                    sessionOffsetMs = 0L,
                    sessionHeartbeatIntervalMs = DEFAULT_SESSION_HEARTBEAT_INTERVAL_MS,
                    sessionProfileId = "auto",
                    sessionMaxBitrate = 0,
                    sessionRestarting = false,
                    error = null,
                )
            }
            val stream = repository.stream(mediaId).getOrElse { error ->
                _state.update { it.copy(loading = false, error = error.message ?: "播放信息加载失败") }
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
            val resumeMs = (progressRepository.restorePosition(mediaId, stream.duration) * 1_000)
                .toLong()
                .coerceAtLeast(0L)
            val baseUrl = sessionStore.snapshot.value.activeServer?.baseUrl
            val plan = stream.playbackPlan
            val title = detail?.displayTitle?.takeIf(String::isNotBlank) ?: stream.title
            val durationMs = (stream.duration * 1_000).toLong().coerceAtLeast(0L)

            if (plan?.requiresEphemeralSession == true) {
                val sessionResult = repository.createPlaybackSession(
                    CreatePlaybackSessionRequest(
                        mediaId = mediaId,
                        profileId = plan.sessionTemplate?.profileId.orEmpty().ifBlank { "auto" },
                        startPositionMs = resumeMs,
                        maxBitrate = plan.sessionTemplate?.maxBitrate ?: 0,
                    ),
                ).getOrElse { error ->
                    _state.update {
                        it.copy(
                            loading = false,
                            title = title,
                            mediaDurationMs = durationMs,
                            error = error.message ?: "实时转码会话启动失败",
                        )
                    }
                    return@launch
                }
                val playlist = resolveServerResource(baseUrl, sessionResult.playlistUrl)
                if (playlist.isNullOrBlank()) {
                    closeSessionAsync(sessionResult.session.id, "missing_playlist_url")
                    _state.update {
                        it.copy(
                            loading = false,
                            title = title,
                            mediaDurationMs = durationMs,
                            error = "服务器未返回会话播放地址",
                        )
                    }
                    return@launch
                }
                applySessionResult(
                    title = title,
                    durationMs = durationMs,
                    subtitles = subtitles?.external.orEmpty(),
                    nextEpisode = nextEpisode,
                    plan = plan,
                    result = sessionResult,
                    playlistUrl = playlist,
                    offsetMs = resumeMs,
                    notice = null,
                )
                return@launch
            }

            val resolved = resolveServerResource(baseUrl, stream.preferredUrl)
            if (resolved.isNullOrBlank()) {
                _state.update { it.copy(loading = false, error = "服务器没有返回可播放地址") }
                return@launch
            }
            val fallbackResolved = resolveServerResource(baseUrl, stream.fallbackUrl)
                ?.takeUnless { it == resolved }
                .orEmpty()
            val fallbackMethod = plan?.fallbackMethod.orEmpty()
            _state.update {
                it.copy(
                    loading = false,
                    title = title,
                    playbackUrl = resolved,
                    resumePositionMs = resumeMs,
                    mediaDurationMs = durationMs,
                    externalSubtitles = subtitles?.external.orEmpty(),
                    nextEpisode = nextEpisode,
                    playbackDiagnostics = PlaybackDiagnostics(
                        method = stream.playbackMethod,
                        methodLabel = stream.playbackMethodLabel,
                        reasonCode = stream.playbackReasonCode,
                        reason = stream.playbackReason,
                        fallbackUrl = fallbackResolved,
                        fallbackMethod = fallbackMethod,
                        fallbackMethodLabel = playbackMethodLabel(fallbackMethod),
                    ),
                )
            }
        }
    }

    private fun applySessionResult(
        title: String,
        durationMs: Long,
        subtitles: List<SubtitleTrack>,
        nextEpisode: MediaDetail?,
        plan: PlaybackPlan,
        result: PlaybackSessionResult,
        playlistUrl: String,
        offsetMs: Long,
        notice: String?,
    ) {
        val generation = result.session.generation
        _state.update {
            it.copy(
                loading = false,
                title = title,
                playbackUrl = playlistUrl,
                resumePositionMs = 0L,
                mediaDurationMs = durationMs,
                externalSubtitles = subtitles,
                nextEpisode = nextEpisode,
                playbackDiagnostics = PlaybackDiagnostics(
                    method = "transcode",
                    methodLabel = playbackMethodLabel("transcode"),
                    reasonCode = plan.reasonCode.ifBlank { "playback_session" },
                    reason = plan.reason.ifBlank { "已创建临时实时转码会话" },
                    fallbackUrl = "",
                    fallbackMethod = "",
                    fallbackMethodLabel = "",
                    usingFallback = it.playbackDiagnostics.usingFallback,
                    lastError = it.playbackDiagnostics.lastError,
                ),
                fallbackNotice = notice,
                sessionManaged = true,
                sessionId = result.session.id,
                sessionGenerationId = result.session.currentGenerationId,
                sessionOffsetMs = offsetMs.coerceAtLeast(0L),
                sessionHeartbeatIntervalMs = result.heartbeatIntervalSec.coerceAtLeast(5).toLong() * 1_000L,
                sessionProfileId = generation?.profileId?.takeIf(String::isNotBlank)
                    ?: plan.sessionTemplate?.profileId.orEmpty().ifBlank { "auto" },
                sessionMaxBitrate = generation?.maxBitrate?.takeIf { value -> value > 0 }
                    ?: plan.sessionTemplate?.maxBitrate
                    ?: 0,
                sessionRestarting = false,
                error = null,
            )
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

    fun absolutePositionMs(relativePositionMs: Long): Long {
        val current = _state.value
        return absolutePlaybackPositionMs(
            sessionManaged = current.sessionManaged,
            generationOffsetMs = current.sessionOffsetMs,
            relativePositionMs = relativePositionMs,
        )
    }

    fun restartPlaybackSession(targetPositionMs: Long, reason: String = "seek") {
        val initial = _state.value
        if (!initial.sessionManaged || initial.sessionId.isBlank() || initial.sessionRestarting) return
        val target = clampPlaybackTargetMs(targetPositionMs, initial.mediaDurationMs)
        viewModelScope.launch {
            sessionOperationMutex.withLock {
                val current = _state.value
                if (!current.sessionManaged || current.sessionId != initial.sessionId) return@withLock
                _state.update { it.copy(sessionRestarting = true, fallbackNotice = "正在切换播放位置…") }
                val result = repository.restartPlaybackSession(
                    current.sessionId,
                    RestartPlaybackSessionRequest(
                        profileId = current.sessionProfileId,
                        startPositionMs = target,
                        maxBitrate = current.sessionMaxBitrate,
                        reason = reason,
                    ),
                ).getOrElse { error ->
                    _state.update {
                        it.copy(
                            sessionRestarting = false,
                            fallbackNotice = "跳转失败，继续当前播放",
                            playbackDiagnostics = it.playbackDiagnostics.copy(lastError = error.message.orEmpty()),
                        )
                    }
                    return@withLock
                }
                val playlist = resolveServerResource(
                    sessionStore.snapshot.value.activeServer?.baseUrl,
                    result.playlistUrl,
                )
                if (playlist.isNullOrBlank()) {
                    _state.update {
                        it.copy(
                            sessionRestarting = false,
                            fallbackNotice = "跳转失败：服务器未返回播放地址",
                        )
                    }
                    return@withLock
                }
                _state.update {
                    it.copy(
                        playbackUrl = playlist,
                        resumePositionMs = 0L,
                        sessionGenerationId = result.session.currentGenerationId,
                        sessionOffsetMs = target,
                        sessionHeartbeatIntervalMs = result.heartbeatIntervalSec.coerceAtLeast(5).toLong() * 1_000L,
                        sessionProfileId = result.session.generation?.profileId?.takeIf(String::isNotBlank)
                            ?: it.sessionProfileId,
                        sessionRestarting = false,
                        fallbackNotice = "已从 ${formatPlaybackTime(target)} 继续播放",
                        error = null,
                    )
                }
            }
        }
    }

    fun heartbeat(relativePositionMs: Long, relativeBufferedEndMs: Long, paused: Boolean) {
        val current = _state.value
        if (!current.sessionManaged || current.sessionId.isBlank() || current.sessionGenerationId <= 0L) return
        val position = absolutePlaybackPositionMs(true, current.sessionOffsetMs, relativePositionMs)
        val buffered = absolutePlaybackPositionMs(true, current.sessionOffsetMs, relativeBufferedEndMs).coerceAtLeast(position)
        viewModelScope.launch {
            repository.heartbeatPlaybackSession(
                current.sessionId,
                PlaybackSessionHeartbeatRequest(
                    generationId = current.sessionGenerationId,
                    positionMs = position,
                    bufferedEndMs = buffered,
                    paused = paused,
                ),
            )
        }
    }

    fun onPlayerError(error: PlaybackException, relativePositionMs: Long) {
        val current = _state.value
        if (current.sessionRestarting) return
        val diagnostics = current.playbackDiagnostics
        val absolutePosition = absolutePlaybackPositionMs(
            current.sessionManaged,
            current.sessionOffsetMs,
            relativePositionMs,
        )

        if (current.sessionManaged) {
            reportProgress(loadedId.orEmpty(), relativePositionMs, current.mediaDurationMs, true)
            detachPlaybackSession("player_error")
            _state.update {
                it.copy(
                    playbackDiagnostics = diagnostics.copy(lastError = error.errorCodeName),
                    error = error.errorCodeName.ifBlank { error.message ?: "实时转码播放失败" },
                )
            }
            return
        }

        if (diagnostics.fallbackMethod.equals("transcode", ignoreCase = true)) {
            val mediaId = loadedId.orEmpty()
            if (mediaId.isBlank()) return
            viewModelScope.launch {
                sessionOperationMutex.withLock {
                    _state.update { it.copy(sessionRestarting = true, fallbackNotice = "正在切换到兼容转码…") }
                    val fallbackPlan = PlaybackPlan(
                        mediaId = mediaId,
                        method = "transcode",
                        reasonCode = "client_playback_fallback",
                        reason = "原播放方式失败，已切换到临时实时转码",
                        requiresTranscode = true,
                        sessionRequired = true,
                    )
                    val result = repository.createPlaybackSession(
                        CreatePlaybackSessionRequest(
                            mediaId = mediaId,
                            profileId = "auto",
                            startPositionMs = absolutePosition,
                        ),
                    ).getOrElse { cause ->
                        _state.update {
                            it.copy(
                                sessionRestarting = false,
                                playbackDiagnostics = diagnostics.copy(lastError = error.errorCodeName),
                                error = cause.message ?: "兼容转码启动失败",
                            )
                        }
                        return@withLock
                    }
                    val playlist = resolveServerResource(
                        sessionStore.snapshot.value.activeServer?.baseUrl,
                        result.playlistUrl,
                    )
                    if (playlist.isNullOrBlank()) {
                        closeSessionAsync(result.session.id, "missing_fallback_playlist")
                        _state.update { it.copy(sessionRestarting = false, error = "兼容转码未返回播放地址") }
                        return@withLock
                    }
                    applySessionResult(
                        title = current.title,
                        durationMs = current.mediaDurationMs,
                        subtitles = current.externalSubtitles,
                        nextEpisode = current.nextEpisode,
                        plan = fallbackPlan,
                        result = result,
                        playlistUrl = playlist,
                        offsetMs = absolutePosition,
                        notice = "播放失败，已切换到兼容转码",
                    )
                    _state.update {
                        it.copy(
                            playbackDiagnostics = it.playbackDiagnostics.copy(
                                usingFallback = true,
                                lastError = error.errorCodeName,
                            ),
                        )
                    }
                }
            }
            return
        }

        if (
            shouldAttemptPlaybackFallback(
                errorCode = error.errorCode,
                currentUrl = current.playbackUrl,
                fallbackUrl = diagnostics.fallbackUrl,
                alreadyUsingFallback = diagnostics.usingFallback,
            )
        ) {
            val fallbackMethod = diagnostics.fallbackMethod.ifBlank { "transcode" }
            val fallbackMethodLabel = diagnostics.fallbackMethodLabel
                .takeUnless { it == "自动选择" }
                ?: playbackMethodLabel(fallbackMethod)
            val previousMethodLabel = diagnostics.methodLabel
            _state.update {
                it.copy(
                    playbackUrl = diagnostics.fallbackUrl,
                    resumePositionMs = absolutePosition.coerceAtLeast(0L),
                    playbackDiagnostics = diagnostics.copy(
                        method = fallbackMethod,
                        methodLabel = fallbackMethodLabel,
                        reasonCode = "client_playback_fallback",
                        reason = "原播放方式（$previousMethodLabel）失败，已自动切换到服务端备用地址",
                        fallbackUrl = "",
                        fallbackMethod = "",
                        fallbackMethodLabel = "",
                        usingFallback = true,
                        lastError = error.errorCodeName,
                    ),
                    fallbackNotice = "播放失败，已自动切换到$fallbackMethodLabel",
                    error = null,
                )
            }
            return
        }

        _state.update {
            it.copy(
                playbackDiagnostics = diagnostics.copy(lastError = error.errorCodeName),
                error = error.errorCodeName.ifBlank { error.message ?: "播放器发生错误" },
            )
        }
    }

    fun clearFallbackNotice() {
        _state.update { it.copy(fallbackNotice = null) }
    }

    fun reportProgress(mediaId: String, positionMs: Long, durationMs: Long, force: Boolean = false) {
        if (mediaId.isBlank()) return
        val current = _state.value
        val absolutePosition = absolutePlaybackPositionMs(
            current.sessionManaged,
            current.sessionOffsetMs,
            positionMs,
        )
        val effectiveDuration = current.mediaDurationMs.takeIf { it > 0L } ?: durationMs
        if (absolutePosition <= 0L || effectiveDuration <= 0L) return
        val now = SystemClock.elapsedRealtime()
        if (!force &&
            abs(absolutePosition - lastReportedPositionMs) < MIN_PROGRESS_DELTA_MS &&
            now - lastReportElapsedMs < MIN_PROGRESS_INTERVAL_MS
        ) return
        lastReportedPositionMs = absolutePosition
        lastReportElapsedMs = now
        viewModelScope.launch(start = CoroutineStart.UNDISPATCHED) {
            withContext(NonCancellable + Dispatchers.IO) {
                val delivery = progressRepository.report(
                    mediaId = mediaId,
                    position = absolutePosition / 1_000.0,
                    duration = effectiveDuration / 1_000.0,
                )
                _state.update { it.copy(progressQueued = delivery.queued) }
            }
        }
    }

    fun closePlaybackSession(reason: String) {
        detachPlaybackSession(reason)
    }

    private fun detachPlaybackSession(reason: String) {
        val sessionId = _state.value.sessionId
        if (sessionId.isBlank()) return
        _state.update {
            it.copy(
                sessionManaged = false,
                sessionId = "",
                sessionGenerationId = 0L,
                sessionOffsetMs = 0L,
                sessionRestarting = false,
            )
        }
        closeSessionAsync(sessionId, reason)
    }

    private fun closeSessionAsync(sessionId: String, reason: String) {
        if (sessionId.isBlank()) return
        viewModelScope.launch(start = CoroutineStart.UNDISPATCHED) {
            withContext(NonCancellable + Dispatchers.IO) {
                repository.closePlaybackSession(sessionId, reason)
            }
        }
    }

    override fun onCleared() {
        val sessionId = _state.value.sessionId
        if (sessionId.isNotBlank()) {
            cleanupScope.launch {
                try {
                    repository.closePlaybackSession(sessionId, "view_model_cleared")
                } finally {
                    cleanupScope.cancel()
                }
            }
        } else {
            cleanupScope.cancel()
        }
        super.onCleared()
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
    var nextEpisodeCountdown by rememberSaveable(mediaId) { mutableIntStateOf(NEXT_EPISODE_COUNTDOWN_SECONDS) }
    var displayPositionMs by remember(mediaId) { mutableStateOf(0L) }
    var playerDurationMs by remember(mediaId) { mutableStateOf(0L) }
    var seekPreviewMs by remember(mediaId) { mutableStateOf<Long?>(null) }
    var controlsVisible by rememberSaveable(mediaId) { mutableStateOf(true) }
    var controlsEpoch by remember(mediaId) { mutableIntStateOf(0) }
    var isPlaying by remember(mediaId) { mutableStateOf(false) }
    var boostingSpeed by remember(mediaId) { mutableStateOf<Float?>(null) }

    LaunchedEffect(mediaId) { viewModel.load(mediaId) }

    val player = remember(token) {
        val httpFactory = DefaultHttpDataSource.Factory().apply {
            if (token.isNotBlank()) setDefaultRequestProperties(mapOf("Authorization" to "Bearer $token"))
            setAllowCrossProtocolRedirects(true)
        }
        val dataSourceFactory = DefaultDataSource.Factory(context, httpFactory)
        ExoPlayer.Builder(context)
            .setMediaSourceFactory(DefaultMediaSourceFactory(dataSourceFactory))
            .build()
    }

    fun revealControls() {
        controlsVisible = true
        controlsEpoch += 1
    }

    fun reportCurrentProgress(force: Boolean) {
        val current = viewModel.state.value
        val exoDuration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
        val duration = current.mediaDurationMs.takeIf { it > 0L } ?: exoDuration ?: 0L
        viewModel.reportProgress(
            mediaId = mediaId,
            positionMs = player.currentPosition.coerceAtLeast(0L),
            durationMs = duration.coerceAtLeast(0L),
            force = force,
        )
    }

    fun leavePlayback(reason: String, action: () -> Unit) {
        reportCurrentProgress(force = true)
        viewModel.heartbeat(
            relativePositionMs = player.currentPosition.coerceAtLeast(0L),
            relativeBufferedEndMs = player.bufferedPosition.coerceAtLeast(0L),
            paused = true,
        )
        viewModel.closePlaybackSession(reason)
        action()
    }

    fun effectiveDurationMs(): Long = state.mediaDurationMs.takeIf { it > 0L }
        ?: playerDurationMs.takeIf { it > 0L }
        ?: 0L

    fun seekToAbsolute(targetMs: Long, reason: String) {
        val duration = effectiveDurationMs()
        if (duration <= 0L) return
        val target = clampPlaybackTargetMs(targetMs, duration)
        seekPreviewMs = null
        displayPositionMs = target
        if (state.sessionManaged) {
            player.pause()
            isPlaying = false
            viewModel.restartPlaybackSession(target, reason)
        } else {
            player.seekTo(target)
        }
        revealControls()
    }

    fun seekBy(deltaMs: Long) {
        val base = seekPreviewMs ?: displayPositionMs
        seekToAbsolute(base + deltaMs, "android_gesture_seek")
    }

    BackHandler { leavePlayback("navigate_back", onBack) }

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
            if (state.resumePositionMs > 0L && !state.sessionManaged) player.setMediaItem(item, state.resumePositionMs)
            else player.setMediaItem(item)
            player.prepare()
            player.playWhenReady = true
            revealControls()
        }
    }

    LaunchedEffect(state.fallbackNotice) {
        if (state.fallbackNotice != null) {
            delay(FALLBACK_NOTICE_DURATION_MS)
            viewModel.clearFallbackNotice()
        }
    }

    LaunchedEffect(state.playbackSpeed, boostingSpeed) {
        if (boostingSpeed == null) player.setPlaybackSpeed(state.playbackSpeed)
    }

    LaunchedEffect(state.resizeMode, playerView) {
        playerView?.resizeMode = resizeModeForPreference(state.resizeMode)
    }

    LaunchedEffect(controlsVisible, controlsEpoch, isPlaying, showSettings) {
        if (controlsVisible && isPlaying && !showSettings) {
            delay(PLAYER_CONTROLS_TIMEOUT_MS)
            controlsVisible = false
        }
    }

    LaunchedEffect(player, mediaId) {
        while (true) {
            delay(PERIODIC_PROGRESS_INTERVAL_MS)
            if (player.isPlaying) reportCurrentProgress(force = false)
        }
    }

    LaunchedEffect(player, state.sessionId, state.sessionGenerationId, state.sessionHeartbeatIntervalMs) {
        if (!state.sessionManaged || state.sessionId.isBlank()) return@LaunchedEffect
        while (true) {
            delay(state.sessionHeartbeatIntervalMs.coerceAtLeast(5_000L))
            viewModel.heartbeat(
                relativePositionMs = player.currentPosition.coerceAtLeast(0L),
                relativeBufferedEndMs = player.bufferedPosition.coerceAtLeast(0L),
                paused = !player.isPlaying,
            )
        }
    }

    LaunchedEffect(player, state.sessionManaged, state.sessionOffsetMs, state.mediaDurationMs) {
        while (true) {
            displayPositionMs = if (state.sessionManaged) viewModel.absolutePositionMs(player.currentPosition)
            else player.currentPosition.coerceAtLeast(0L)
            val reportedDuration = state.mediaDurationMs.takeIf { it > 0L }
            val exoDuration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
            playerDurationMs = reportedDuration ?: exoDuration ?: playerDurationMs
            delay(250L)
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
        leavePlayback("next_media", action = { onPlayNext(next.id) })
    }

    DisposableEffect(player, mediaId, lifecycleOwner) {
        val playerListener = object : Player.Listener {
            override fun onIsPlayingChanged(playing: Boolean) {
                isPlaying = playing
                if (!playing && player.playbackState == Player.STATE_READY) {
                    reportCurrentProgress(force = true)
                    revealControls()
                }
                viewModel.heartbeat(
                    relativePositionMs = player.currentPosition.coerceAtLeast(0L),
                    relativeBufferedEndMs = player.bufferedPosition.coerceAtLeast(0L),
                    paused = !playing,
                )
            }

            override fun onPlaybackStateChanged(playbackState: Int) {
                val duration = player.duration.takeIf { it != C.TIME_UNSET && it > 0L }
                if (duration != null) playerDurationMs = duration
                if (playbackState == Player.STATE_ENDED) {
                    val current = viewModel.state.value
                    val relativeEnd = if (current.sessionManaged) {
                        relativePlaybackPositionMs(
                            sessionManaged = true,
                            generationOffsetMs = current.sessionOffsetMs,
                            absolutePositionMs = current.mediaDurationMs,
                        )
                    } else {
                        player.duration.takeIf { it != C.TIME_UNSET && it > 0L } ?: current.mediaDurationMs
                    }
                    viewModel.reportProgress(
                        mediaId = mediaId,
                        positionMs = relativeEnd,
                        durationMs = current.mediaDurationMs.takeIf { it > 0L } ?: playerDurationMs,
                        force = true,
                    )
                    viewModel.closePlaybackSession("playback_ended")
                    if (current.nextEpisode != null) {
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
                viewModel.onPlayerError(error, player.currentPosition)
            }

            override fun onPositionDiscontinuity(
                oldPosition: Player.PositionInfo,
                newPosition: Player.PositionInfo,
                reason: Int,
            ) {
                val current = viewModel.state.value
                if (reason == Player.DISCONTINUITY_REASON_SEEK && current.sessionManaged) {
                    val target = clampPlaybackTargetMs(
                        current.sessionOffsetMs + newPosition.positionMs.coerceAtLeast(0L),
                        current.mediaDurationMs,
                    )
                    player.pause()
                    viewModel.restartPlaybackSession(target, "exo_player_seek")
                } else if (reason == Player.DISCONTINUITY_REASON_SEEK) {
                    reportCurrentProgress(force = true)
                }
            }
        }
        val lifecycleObserver = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_STOP) {
                reportCurrentProgress(force = true)
                viewModel.heartbeat(
                    relativePositionMs = player.currentPosition.coerceAtLeast(0L),
                    relativeBufferedEndMs = player.bufferedPosition.coerceAtLeast(0L),
                    paused = true,
                )
            }
        }
        player.addListener(playerListener)
        lifecycleOwner.lifecycle.addObserver(lifecycleObserver)
        onDispose {
            reportCurrentProgress(force = true)
            viewModel.closePlaybackSession("player_disposed")
            lifecycleOwner.lifecycle.removeObserver(lifecycleObserver)
            player.removeListener(playerListener)
            player.release()
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
            state.error != null -> MessagePanel(
                title = "无法播放",
                message = state.error!!,
                actionLabel = "返回",
                onAction = { leavePlayback("playback_error_back", onBack) },
                modifier = Modifier.align(Alignment.Center).padding(20.dp),
            )
            else -> {
                AndroidView(
                    factory = { viewContext ->
                        PlayerView(viewContext).apply {
                            useController = false
                            this.player = player
                            keepScreenOn = true
                            resizeMode = resizeModeForPreference(state.resizeMode)
                            playerView = this
                        }
                    },
                    update = {
                        it.useController = false
                        it.player = player
                        it.resizeMode = resizeModeForPreference(state.resizeMode)
                        playerView = it
                    },
                    modifier = Modifier.fillMaxSize(),
                )

                PlayerGestureLayer(
                    currentSpeed = state.playbackSpeed,
                    enabled = !showSettings && !state.sessionRestarting,
                    onTap = {
                        controlsVisible = !controlsVisible
                        controlsEpoch += 1
                    },
                    onSeekBy = ::seekBy,
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
                    playbackSpeed = state.playbackSpeed,
                    boostingSpeed = boostingSpeed,
                    seekingEnabled = !state.sessionRestarting,
                    onBack = { leavePlayback("navigate_back", onBack) },
                    onSettings = {
                        showSettings = true
                        controlsVisible = false
                    },
                    onPlayPause = {
                        if (player.isPlaying) player.pause() else player.play()
                        revealControls()
                    },
                    onSeekBy = ::seekBy,
                    onSeekFractionChange = { fraction ->
                        if (durationMs > 0L) {
                            seekPreviewMs = (durationMs * fraction).toLong().coerceIn(0L, durationMs)
                        }
                        revealControls()
                    },
                    onSeekFinished = {
                        val target = seekPreviewMs ?: displayPositionMs
                        seekToAbsolute(target, "android_timeline_seek")
                    },
                    onSpeedClick = {
                        showSettings = true
                        controlsVisible = false
                    },
                )
            }
        }

        val statusNotice = state.fallbackNotice
            ?: when {
                state.sessionRestarting -> "正在生成新的播放时间线…"
                state.progressQueued -> "当前离线，观看进度将在恢复连接后自动同步"
                else -> null
            }
        if (statusNotice != null && !showNextEpisodePanel && !showSettings) {
            Surface(
                modifier = Modifier
                    .align(Alignment.BottomCenter)
                    .windowInsetsPadding(WindowInsets.navigationBars)
                    .padding(horizontal = 16.dp, vertical = if (controlsVisible) 86.dp else 16.dp),
                shape = MaterialTheme.shapes.large,
                color = Color.Black.copy(alpha = 0.76f),
            ) {
                Text(
                    text = statusNotice,
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
                        text = if (state.autoPlayNext) "$nextEpisodeCountdown 秒后播放下一集" else "下一集已准备好",
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
                        modifier = Modifier.fillMaxWidth().padding(top = 14.dp),
                        horizontalArrangement = Arrangement.End,
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        TextButton(onClick = { showNextEpisodePanel = false }) { Text("取消") }
                        Spacer(Modifier.width(8.dp))
                        Button(
                            onClick = {
                                showNextEpisodePanel = false
                                leavePlayback("next_media", action = { onPlayNext(next.id) })
                            },
                        ) { Text("立即播放") }
                    }
                }
            }
        }
    }

    if (showSettings) {
        PlayerSettingsSheet(
            onDismiss = {
                showSettings = false
                revealControls()
            },
            playbackDiagnostics = state.playbackDiagnostics,
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

internal fun formatPlaybackTime(positionMs: Long): String {
    val totalSeconds = positionMs.coerceAtLeast(0L) / 1_000L
    val hours = totalSeconds / 3_600L
    val minutes = (totalSeconds % 3_600L) / 60L
    val seconds = totalSeconds % 60L
    return if (hours > 0L) "%d:%02d:%02d".format(hours, minutes, seconds)
    else "%02d:%02d".format(minutes, seconds)
}
