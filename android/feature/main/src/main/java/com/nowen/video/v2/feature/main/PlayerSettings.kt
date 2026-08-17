package com.nowen.video.v2.feature.main

import android.content.res.Configuration
import android.net.Uri
import androidx.compose.foundation.clickable
import androidx.compose.foundation.horizontalScroll
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.RowScope
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilterChip
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.ListItem
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.ModalBottomSheet
import androidx.compose.material3.Surface
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.unit.dp
import androidx.compose.ui.window.Dialog
import androidx.compose.ui.window.DialogProperties
import androidx.media3.common.C
import androidx.media3.common.Format
import androidx.media3.common.MediaItem
import androidx.media3.common.MimeTypes
import androidx.media3.common.TrackGroup
import androidx.media3.common.TrackSelectionOverride
import androidx.media3.common.Tracks
import androidx.media3.common.util.UnstableApi
import androidx.media3.ui.AspectRatioFrameLayout
import com.nowen.video.v2.core.model.SubtitleTrack

private val PlayerSpeedOptions = listOf(
    0.5f,
    0.75f,
    1f,
    1.25f,
    1.5f,
    1.75f,
    2f,
    3f,
    4f,
    6f,
    8f,
)

internal data class PlayerTrackChoice(
    val group: TrackGroup,
    val trackIndex: Int,
    val label: String,
    val selected: Boolean,
)

internal fun extractTrackChoices(tracks: Tracks, trackType: Int): List<PlayerTrackChoice> =
    tracks.groups
        .filter { it.type == trackType }
        .flatMapIndexed { groupIndex, group ->
            (0 until group.length)
                .filter { group.isTrackSupported(it) }
                .map { trackIndex ->
                    PlayerTrackChoice(
                        group = group.mediaTrackGroup,
                        trackIndex = trackIndex,
                        label = playerTrackLabel(
                            format = group.getTrackFormat(trackIndex),
                            trackType = trackType,
                            ordinal = groupIndex + trackIndex + 1,
                        ),
                        selected = group.isTrackSelected(trackIndex),
                    )
                }
        }

internal fun playerTrackLabel(format: Format, trackType: Int, ordinal: Int): String {
    val language = format.language?.takeIf { it.isNotBlank() && it != "und" }
    val base = format.label?.takeIf { it.isNotBlank() }
        ?: language
        ?: if (trackType == C.TRACK_TYPE_AUDIO) "音轨 $ordinal" else "字幕 $ordinal"
    val detail = when (trackType) {
        C.TRACK_TYPE_AUDIO -> listOfNotNull(
            format.channelCount.takeIf { it > 0 }?.let { "${it}ch" },
            format.sampleMimeType?.substringAfterLast('/')?.uppercase(),
        ).joinToString(" · ")
        C.TRACK_TYPE_TEXT -> format.sampleMimeType?.substringAfterLast('/')?.uppercase().orEmpty()
        else -> ""
    }
    return if (detail.isBlank()) base else "$base · $detail"
}

internal fun applyTrackChoice(
    tracks: androidx.media3.common.TrackSelectionParameters,
    trackType: Int,
    choice: PlayerTrackChoice?,
): androidx.media3.common.TrackSelectionParameters {
    val builder = tracks.buildUpon()
        .setTrackTypeDisabled(trackType, false)
        .clearOverridesOfType(trackType)
    if (choice != null) {
        builder.setOverrideForType(TrackSelectionOverride(choice.group, choice.trackIndex))
    }
    return builder.build()
}

internal fun disableTextTracks(
    tracks: androidx.media3.common.TrackSelectionParameters,
): androidx.media3.common.TrackSelectionParameters = tracks.buildUpon()
    .clearOverridesOfType(C.TRACK_TYPE_TEXT)
    .setTrackTypeDisabled(C.TRACK_TYPE_TEXT, true)
    .build()

@androidx.annotation.OptIn(UnstableApi::class)
internal fun resizeModeForPreference(mode: Int): Int = when (mode) {
    1 -> AspectRatioFrameLayout.RESIZE_MODE_ZOOM
    2 -> AspectRatioFrameLayout.RESIZE_MODE_FILL
    else -> AspectRatioFrameLayout.RESIZE_MODE_FIT
}

internal fun subtitleMimeType(format: String, path: String = ""): String {
    val resolved = format.ifBlank { path.substringAfterLast('.', "") }.lowercase()
    return when (resolved) {
        "vtt", "webvtt" -> MimeTypes.TEXT_VTT
        "ass", "ssa" -> MimeTypes.TEXT_SSA
        "ttml", "xml" -> MimeTypes.APPLICATION_TTML
        "subrip", "srt" -> MimeTypes.APPLICATION_SUBRIP
        else -> MimeTypes.APPLICATION_SUBRIP
    }
}

internal fun externalSubtitleConfigurations(
    baseUrl: String?,
    tracks: List<SubtitleTrack>,
): List<MediaItem.SubtitleConfiguration> {
    if (baseUrl.isNullOrBlank()) return emptyList()
    return tracks.mapNotNull { track ->
        val path = track.sourcePath.takeIf { it.isNotBlank() } ?: return@mapNotNull null
        val uri = Uri.parse(baseUrl).buildUpon()
            .appendEncodedPath("api/subtitle/external")
            .appendQueryParameter("path", path)
            .appendQueryParameter("format", "raw")
            .build()
        MediaItem.SubtitleConfiguration.Builder(uri)
            .setMimeType(subtitleMimeType(track.format, path))
            .setLanguage(track.language.ifBlank { "und" })
            .setLabel(track.displayLabel)
            .build()
    }
}

@OptIn(ExperimentalMaterial3Api::class)
@Composable
internal fun PlayerSettingsSheet(
    onDismiss: () -> Unit,
    playbackDiagnostics: PlaybackDiagnostics,
    playbackSpeed: Float,
    onPlaybackSpeedChange: (Float) -> Unit,
    resizeMode: Int,
    onResizeModeChange: (Int) -> Unit,
    autoPlayNext: Boolean,
    onAutoPlayNextChange: (Boolean) -> Unit,
    audioTracks: List<PlayerTrackChoice>,
    audioAutomatic: Boolean,
    onAudioTrackSelected: (PlayerTrackChoice?) -> Unit,
    subtitleTracks: List<PlayerTrackChoice>,
    subtitlesDisabled: Boolean,
    onSubtitleTrackSelected: (PlayerTrackChoice?) -> Unit,
) {
    val configuration = LocalConfiguration.current
    val landscape = configuration.orientation == Configuration.ORIENTATION_LANDSCAPE
    val maxPortraitHeight = (configuration.screenHeightDp * 0.72f).dp

    val content: @Composable () -> Unit = {
        PlayerSettingsContent(
            playbackDiagnostics = playbackDiagnostics,
            playbackSpeed = playbackSpeed,
            onPlaybackSpeedChange = onPlaybackSpeedChange,
            resizeMode = resizeMode,
            onResizeModeChange = onResizeModeChange,
            autoPlayNext = autoPlayNext,
            onAutoPlayNextChange = onAutoPlayNextChange,
            audioTracks = audioTracks,
            audioAutomatic = audioAutomatic,
            onAudioTrackSelected = onAudioTrackSelected,
            subtitleTracks = subtitleTracks,
            subtitlesDisabled = subtitlesDisabled,
            onSubtitleTrackSelected = onSubtitleTrackSelected,
        )
    }

    if (landscape) {
        Dialog(
            onDismissRequest = onDismiss,
            properties = DialogProperties(
                usePlatformDefaultWidth = false,
                decorFitsSystemWindows = false,
            ),
        ) {
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .clickable(onClick = onDismiss),
                contentAlignment = Alignment.CenterEnd,
            ) {
                Surface(
                    modifier = Modifier
                        .fillMaxHeight()
                        .widthIn(min = 320.dp, max = 400.dp)
                        .fillMaxWidth(0.42f)
                        .windowInsetsPadding(WindowInsets.safeDrawing)
                        .clickable(onClick = {}),
                    color = MaterialTheme.colorScheme.surface.copy(alpha = 0.97f),
                    tonalElevation = 8.dp,
                    shadowElevation = 16.dp,
                ) {
                    content()
                }
            }
        }
    } else {
        ModalBottomSheet(onDismissRequest = onDismiss) {
            Box(
                modifier = Modifier
                    .fillMaxWidth()
                    .heightIn(max = maxPortraitHeight),
            ) {
                content()
            }
        }
    }
}

@Composable
private fun PlayerSettingsContent(
    playbackDiagnostics: PlaybackDiagnostics,
    playbackSpeed: Float,
    onPlaybackSpeedChange: (Float) -> Unit,
    resizeMode: Int,
    onResizeModeChange: (Int) -> Unit,
    autoPlayNext: Boolean,
    onAutoPlayNextChange: (Boolean) -> Unit,
    audioTracks: List<PlayerTrackChoice>,
    audioAutomatic: Boolean,
    onAudioTrackSelected: (PlayerTrackChoice?) -> Unit,
    subtitleTracks: List<PlayerTrackChoice>,
    subtitlesDisabled: Boolean,
    onSubtitleTrackSelected: (PlayerTrackChoice?) -> Unit,
) {
    var showSpeedOptions by rememberSaveable { mutableStateOf(false) }
    var showPlaybackDetails by rememberSaveable { mutableStateOf(false) }

    Column(
        modifier = Modifier
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .padding(bottom = 28.dp),
    ) {
        Text(
            text = "播放设置",
            style = MaterialTheme.typography.headlineSmall,
            modifier = Modifier.padding(horizontal = 20.dp, vertical = 12.dp),
        )

        ListItem(
            headlineContent = { Text("当前播放") },
            supportingContent = {
                Text(
                    buildString {
                        append(playbackDiagnostics.methodLabel)
                        if (playbackDiagnostics.usingFallback) append(" · 已自动兼容")
                    },
                )
            },
        )

        HorizontalDivider(Modifier.padding(vertical = 4.dp))

        ListItem(
            headlineContent = { Text("播放速度") },
            supportingContent = { Text("最高支持 8×，长按画面可临时快速播放") },
            trailingContent = { Text(speedLabel(playbackSpeed)) },
            modifier = Modifier.clickable { showSpeedOptions = !showSpeedOptions },
        )
        if (showSpeedOptions) {
            SpeedOptions(
                selected = playbackSpeed,
                onSelected = onPlaybackSpeedChange,
            )
        }

        SettingsSectionTitle("画面比例")
        ChoiceRow {
            listOf(0 to "适应", 1 to "裁切", 2 to "拉伸").forEach { (mode, label) ->
                FilterChip(
                    selected = resizeMode == mode,
                    onClick = { onResizeModeChange(mode) },
                    label = { Text(label) },
                )
            }
        }

        HorizontalDivider(Modifier.padding(vertical = 8.dp))
        ListItem(
            headlineContent = { Text("自动播放下一集") },
            supportingContent = { Text("当前剧集结束后倒计时 5 秒并自动续播") },
            trailingContent = {
                Switch(
                    checked = autoPlayNext,
                    onCheckedChange = onAutoPlayNextChange,
                )
            },
        )

        HorizontalDivider(Modifier.padding(vertical = 8.dp))
        SettingsSectionTitle("音轨")
        ChoiceRow {
            FilterChip(
                selected = audioAutomatic,
                onClick = { onAudioTrackSelected(null) },
                label = { Text("自动") },
            )
            audioTracks.forEach { track ->
                FilterChip(
                    selected = !audioAutomatic && track.selected,
                    onClick = { onAudioTrackSelected(track) },
                    label = { Text(track.label) },
                )
            }
        }

        SettingsSectionTitle("字幕")
        ChoiceRow {
            FilterChip(
                selected = subtitlesDisabled,
                onClick = { onSubtitleTrackSelected(null) },
                label = { Text("关闭") },
            )
            subtitleTracks.forEach { track ->
                FilterChip(
                    selected = !subtitlesDisabled && track.selected,
                    onClick = { onSubtitleTrackSelected(track) },
                    label = { Text(track.label) },
                )
            }
        }
        if (subtitleTracks.isEmpty()) {
            Text(
                text = "当前媒体没有可选字幕",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp),
            )
        }

        HorizontalDivider(Modifier.padding(vertical = 8.dp))
        ListItem(
            headlineContent = { Text("播放信息") },
            supportingContent = { Text("查看播放方式、兼容策略与诊断信息") },
            trailingContent = { Text(if (showPlaybackDetails) "收起" else "查看") },
            modifier = Modifier.clickable { showPlaybackDetails = !showPlaybackDetails },
        )
        if (showPlaybackDetails) {
            PlaybackDiagnosticsDetails(playbackDiagnostics)
        }

        Spacer(Modifier.height(12.dp))
    }
}

@Composable
private fun SpeedOptions(
    selected: Float,
    onSelected: (Float) -> Unit,
) {
    Column(
        modifier = Modifier.padding(horizontal = 20.dp, vertical = 4.dp),
        verticalArrangement = Arrangement.spacedBy(8.dp),
    ) {
        PlayerSpeedOptions.chunked(4).forEach { rowSpeeds ->
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.spacedBy(8.dp),
            ) {
                rowSpeeds.forEach { speed ->
                    FilterChip(
                        selected = selected == speed,
                        onClick = { onSelected(speed) },
                        label = { Text(speedLabel(speed)) },
                        modifier = Modifier.weight(1f),
                    )
                }
                repeat(4 - rowSpeeds.size) {
                    Spacer(Modifier.weight(1f))
                }
            }
        }
    }
}

private fun speedLabel(speed: Float): String = if (speed == 1f) {
    "1.0×"
} else {
    "${speed.toString().removeSuffix(".0")}×"
}

@Composable
private fun PlaybackDiagnosticsDetails(diagnostics: PlaybackDiagnostics) {
    Surface(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 20.dp),
        shape = MaterialTheme.shapes.medium,
        color = MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.58f),
    ) {
        Column(Modifier.padding(14.dp)) {
            Text(
                text = diagnostics.reason.ifBlank { "等待服务端返回播放规划" },
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            if (diagnostics.reasonCode.isNotBlank()) {
                Spacer(Modifier.height(6.dp))
                Text(
                    text = "诊断码：${diagnostics.reasonCode}",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
            if (diagnostics.lastError.isNotBlank()) {
                Spacer(Modifier.height(6.dp))
                Text(
                    text = "最近错误：${diagnostics.lastError}",
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodySmall,
                )
            } else if (diagnostics.fallbackAvailable) {
                Spacer(Modifier.height(6.dp))
                Text(
                    text = "当前方式失败时会自动切换到${diagnostics.fallbackMethodLabel}",
                    color = MaterialTheme.colorScheme.primary,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
    }
}

@Composable
private fun SettingsSectionTitle(title: String) {
    Text(
        text = title,
        style = MaterialTheme.typography.titleMedium,
        modifier = Modifier.padding(start = 20.dp, end = 20.dp, top = 16.dp, bottom = 8.dp),
    )
}

@Composable
private fun ChoiceRow(content: @Composable RowScope.() -> Unit) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .horizontalScroll(rememberScrollState())
            .padding(horizontal = 20.dp),
        horizontalArrangement = Arrangement.spacedBy(8.dp),
        content = content,
    )
}
