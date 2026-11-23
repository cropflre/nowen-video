package com.nowen.video.v2.feature.main

import android.content.res.Configuration
import android.net.Uri
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.heightIn
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.safeDrawing
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.layout.widthIn
import androidx.compose.foundation.layout.windowInsetsPadding
import androidx.compose.foundation.selection.selectable
import androidx.compose.foundation.selection.selectableGroup
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Close
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.RadioButton
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalConfiguration
import androidx.compose.ui.semantics.Role
import androidx.compose.ui.semantics.clearAndSetSemantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.heading
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
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
import com.nowen.video.v2.core.designsystem.HillsChoiceRail
import com.nowen.video.v2.core.designsystem.HillsToggle
import com.nowen.video.v2.core.model.SubtitleTrack
import com.nowen.video.v2.core.data.supportedLongPressBoostSpeeds
import com.nowen.video.v2.core.data.supportedPlaybackSpeeds

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
        C.TRACK_TYPE_TEXT -> format.sampleMimeType
            ?.takeUnless { it == MimeTypes.APPLICATION_MEDIA3_CUES }
            ?.substringAfterLast('/')
            ?.uppercase()
            .orEmpty()
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

@Composable
internal fun SubtitlePickerDialog(
    tracks: List<PlayerTrackChoice>,
    onDismiss: () -> Unit,
    onSelect: (PlayerTrackChoice?) -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text("字幕") },
        text = {
            if (tracks.isEmpty()) {
                Text("当前媒体没有可选字幕")
            } else {
                Column(
                    Modifier.heightIn(max = 260.dp).verticalScroll(rememberScrollState()).selectableGroup(),
                ) {
                    (listOf(null) + tracks).forEach { track ->
                        val selected = track?.selected ?: tracks.none { it.selected }
                        Row(
                            modifier = Modifier.fillMaxWidth().heightIn(min = 48.dp)
                                .selectable(selected = selected, role = Role.RadioButton, onClick = { onSelect(track) }),
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(12.dp),
                        ) {
                            RadioButton(selected = selected, onClick = null)
                            Text(track?.label ?: "关闭字幕", style = MaterialTheme.typography.bodyLarge)
                        }
                    }
                }
            }
        },
        confirmButton = { TextButton(onClick = onDismiss) { Text("取消") } },
    )
}

@Composable
internal fun PlayerSettingsSheet(
    onDismiss: () -> Unit,
    playbackDiagnostics: PlaybackDiagnostics,
    playbackSpeed: Float,
    onPlaybackSpeedChange: (Float) -> Unit,
    longPressBoostSpeed: Float,
    onLongPressBoostSpeedChange: (Float) -> Unit,
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
    danmakuAvailable: Boolean = true,
    danmakuOffline: Boolean = false,
    danmakuEnabled: Boolean = false,
    danmakuAutoMatch: Boolean = true,
    danmakuLoading: Boolean = false,
    danmakuCueCount: Int = 0,
    danmakuError: String? = null,
    onDanmakuEnabledChange: (Boolean) -> Unit = {},
    onDanmakuAutoMatchChange: (Boolean) -> Unit = {},
    onDanmakuSearch: () -> Unit = {},
    onDanmakuRematch: () -> Unit = {},
) {
    val configuration = LocalConfiguration.current
    val landscape = configuration.orientation == Configuration.ORIENTATION_LANDSCAPE

    Dialog(
        onDismissRequest = onDismiss,
        properties = DialogProperties(
            usePlatformDefaultWidth = false,
            decorFitsSystemWindows = false,
        ),
    ) {
        Box(Modifier.fillMaxSize()) {
            Box(
                Modifier.matchParentSize()
                    .clearAndSetSemantics {}
                    .clickable(onClick = onDismiss),
            )
            Box(
                modifier = Modifier
                    .fillMaxSize()
                    .windowInsetsPadding(WindowInsets.safeDrawing)
                    .padding(12.dp),
                contentAlignment = if (landscape) Alignment.CenterEnd else Alignment.BottomCenter,
            ) {
                Surface(
                    modifier = if (landscape) {
                        Modifier.widthIn(max = 360.dp).fillMaxWidth().fillMaxHeight()
                    } else {
                        Modifier.widthIn(max = 480.dp).fillMaxWidth()
                            .heightIn(max = (configuration.screenHeightDp * 0.8f).dp)
                    },
                    color = MaterialTheme.colorScheme.surface,
                    shape = RoundedCornerShape(20.dp),
                    shadowElevation = 12.dp,
                ) {
                    Column {
                        PlayerSettingsHeader(playbackDiagnostics, onDismiss)
                        HorizontalDivider(color = MaterialTheme.colorScheme.outlineVariant)
                        PlayerSettingsContent(
                            playbackDiagnostics = playbackDiagnostics,
                            playbackSpeed = playbackSpeed,
                            onPlaybackSpeedChange = onPlaybackSpeedChange,
                            longPressBoostSpeed = longPressBoostSpeed,
                            onLongPressBoostSpeedChange = onLongPressBoostSpeedChange,
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
                            danmakuAvailable = danmakuAvailable,
                            danmakuOffline = danmakuOffline,
                            danmakuEnabled = danmakuEnabled,
                            danmakuAutoMatch = danmakuAutoMatch,
                            danmakuLoading = danmakuLoading,
                            danmakuCueCount = danmakuCueCount,
                            danmakuError = danmakuError,
                            onDanmakuEnabledChange = onDanmakuEnabledChange,
                            onDanmakuAutoMatchChange = onDanmakuAutoMatchChange,
                            onDanmakuSearch = onDanmakuSearch,
                            onDanmakuRematch = onDanmakuRematch,
                            modifier = Modifier.weight(1f, fill = false),
                        )
                    }
                }
            }
        }
    }
}

@Composable
private fun PlayerSettingsHeader(playbackDiagnostics: PlaybackDiagnostics, onDismiss: () -> Unit) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(start = 16.dp, end = 6.dp, top = 6.dp, bottom = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                text = "播放设置",
                style = MaterialTheme.typography.titleLarge,
                modifier = Modifier.semantics { heading() },
            )
            Text(
                text = if (playbackDiagnostics.usingFallback) {
                    "${playbackDiagnostics.methodLabel} · 已自动兼容"
                } else playbackDiagnostics.methodLabel,
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        IconButton(onClick = onDismiss) {
            Icon(Icons.Default.Close, contentDescription = "关闭播放设置")
        }
    }
}

@Composable
private fun PlayerSettingsContent(
    playbackDiagnostics: PlaybackDiagnostics,
    playbackSpeed: Float,
    onPlaybackSpeedChange: (Float) -> Unit,
    longPressBoostSpeed: Float,
    onLongPressBoostSpeedChange: (Float) -> Unit,
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
    danmakuAvailable: Boolean,
    danmakuOffline: Boolean,
    danmakuEnabled: Boolean,
    danmakuAutoMatch: Boolean,
    danmakuLoading: Boolean,
    danmakuCueCount: Int,
    danmakuError: String?,
    onDanmakuEnabledChange: (Boolean) -> Unit,
    onDanmakuAutoMatchChange: (Boolean) -> Unit,
    onDanmakuSearch: () -> Unit,
    onDanmakuRematch: () -> Unit,
    modifier: Modifier = Modifier,
) {
    var showPlaybackDetails by rememberSaveable { mutableStateOf(false) }

    Column(
        modifier = modifier
            .fillMaxWidth()
            .verticalScroll(rememberScrollState())
            .padding(top = 12.dp, bottom = 16.dp),
    ) {
        PlayerSettingsChoices(
            title = "播放倍速",
            options = supportedPlaybackSpeeds.map { it to speedLabel(it) },
            selected = playbackSpeed,
            onSelect = onPlaybackSpeedChange,
            compact = true,
        )

        PlayerSettingsChoices(
            title = "长按倍速",
            options = supportedLongPressBoostSpeeds.map { it to speedLabel(it) },
            selected = longPressBoostSpeed,
            onSelect = onLongPressBoostSpeedChange,
        )

        PlayerSettingsChoices(
            title = "画面比例",
            options = listOf(0 to "适应", 1 to "裁切", 2 to "拉伸"),
            selected = resizeMode,
            onSelect = onResizeModeChange,
        )

        PlayerSettingsRow(
            title = "自动播放下一集",
            subtitle = "当前剧集结束后倒计时 5 秒并自动续播",
            trailing = {
                HillsToggle(
                    checked = autoPlayNext,
                    onCheckedChange = onAutoPlayNextChange,
                    modifier = Modifier.semantics { contentDescription = "自动播放下一集" },
                )
            },
        )

        SettingsSectionTitle("音轨")
        val audioOptions = listOf("auto" to "自动") + audioTracks.map { trackKey(it) to it.label }
        val selectedAudio = if (audioAutomatic) "auto" else audioTracks.firstOrNull { it.selected }?.let(::trackKey).orEmpty()
        HillsChoiceRail(
            options = audioOptions,
            selected = selectedAudio,
            onSelect = { key -> onAudioTrackSelected(audioTracks.firstOrNull { trackKey(it) == key }) },
            modifier = Modifier.padding(horizontal = 20.dp),
        )

        SettingsSectionTitle("字幕")
        val subtitleOptions = listOf("off" to "关闭") + subtitleTracks.map { trackKey(it) to it.label }
        val selectedSubtitle = if (subtitlesDisabled) "off" else subtitleTracks.firstOrNull { it.selected }?.let(::trackKey).orEmpty()
        HillsChoiceRail(
            options = subtitleOptions,
            selected = selectedSubtitle,
            onSelect = { key -> onSubtitleTrackSelected(subtitleTracks.firstOrNull { trackKey(it) == key }) },
            modifier = Modifier.padding(horizontal = 20.dp),
        )
        if (subtitleTracks.isEmpty()) {
            Text(
                text = "当前媒体没有可选字幕",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp),
            )
        }

        SettingsSectionTitle("弹幕")
        if (danmakuOffline) {
            Text(
                text = "离线播放暂不支持弹幕",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp),
            )
        } else if (!danmakuAvailable) {
            Text(
                text = "请先在设置中配置外部弹幕 API",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodyMedium,
                modifier = Modifier.padding(horizontal = 20.dp, vertical = 8.dp),
            )
        } else {
            PlayerSettingsRow(
                title = "显示弹幕",
                subtitle = when {
                    danmakuLoading -> "正在匹配弹幕…"
                    danmakuError != null -> "弹幕加载失败"
                    danmakuCueCount > 0 -> "已加载 $danmakuCueCount 条弹幕"
                    else -> "来自已配置的外部弹幕服务"
                },
                trailing = {
                    HillsToggle(
                        checked = danmakuEnabled,
                        onCheckedChange = onDanmakuEnabledChange,
                        modifier = Modifier.semantics { contentDescription = "显示弹幕" },
                    )
                },
            )
            PlayerSettingsRow(
                title = "自动匹配",
                subtitle = "打开媒体时按标题、季和集自动查找",
                trailing = {
                    HillsToggle(
                        checked = danmakuAutoMatch,
                        onCheckedChange = onDanmakuAutoMatchChange,
                        modifier = Modifier.semantics { contentDescription = "自动匹配弹幕" },
                    )
                },
            )
            Row(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 20.dp, vertical = 4.dp),
                horizontalArrangement = Arrangement.End,
            ) {
                TextButton(onClick = onDanmakuSearch) { Text("手动搜索") }
                TextButton(onClick = onDanmakuRematch, enabled = !danmakuLoading) { Text("重新匹配") }
            }
            danmakuError?.let { message ->
                Text(
                    text = message.take(120),
                    color = MaterialTheme.colorScheme.error,
                    style = MaterialTheme.typography.bodySmall,
                    modifier = Modifier.padding(horizontal = 20.dp, vertical = 4.dp),
                )
            }
        }

        PlayerSettingsRow(
            title = "播放信息",
            subtitle = "查看播放方式、兼容策略与诊断信息",
            value = if (showPlaybackDetails) "收起" else "查看",
            onClick = { showPlaybackDetails = !showPlaybackDetails },
        )
        if (showPlaybackDetails) {
            PlaybackDiagnosticsDetails(playbackDiagnostics)
        }

        Spacer(Modifier.height(12.dp))
    }
}

@Composable
private fun PlayerSettingsRow(
    title: String,
    subtitle: String,
    value: String? = null,
    onClick: (() -> Unit)? = null,
    trailing: @Composable (() -> Unit)? = null,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clickable(enabled = onClick != null) { onClick?.invoke() }
            .padding(horizontal = 20.dp, vertical = 13.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleMedium)
            Text(
                subtitle,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodySmall,
            )
        }
        if (trailing != null) {
            Spacer(Modifier.width(12.dp))
            trailing()
        } else if (!value.isNullOrBlank()) {
            Spacer(Modifier.width(12.dp))
            Text(value, color = MaterialTheme.colorScheme.primary, style = MaterialTheme.typography.bodyMedium)
        }
    }
}

@OptIn(ExperimentalLayoutApi::class)
@Composable
private fun <T> PlayerSettingsChoices(
    title: String,
    options: List<Pair<T, String>>,
    selected: T,
    onSelect: (T) -> Unit,
    compact: Boolean = false,
) {
    Column(
        modifier = Modifier.padding(start = 16.dp, end = 16.dp, bottom = 14.dp),
        verticalArrangement = Arrangement.spacedBy(6.dp),
    ) {
        Text(
            text = title,
            style = MaterialTheme.typography.titleSmall,
            modifier = Modifier.semantics { heading() },
        )
        FlowRow(
            modifier = Modifier.fillMaxWidth().selectableGroup(),
            horizontalArrangement = Arrangement.spacedBy(6.dp),
            verticalArrangement = Arrangement.spacedBy(6.dp),
            maxItemsInEachRow = if (compact) 6 else 4,
        ) {
            options.forEach { (value, label) ->
                val active = value == selected
                val shape = RoundedCornerShape(10.dp)
                Box(
                    modifier = Modifier
                        .widthIn(min = if (compact) 48.dp else 72.dp)
                        .heightIn(min = 48.dp)
                        .clip(shape)
                        .background(
                            if (active) MaterialTheme.colorScheme.primaryContainer
                            else MaterialTheme.colorScheme.surfaceVariant,
                        )
                        .border(
                            width = 1.dp,
                            color = if (active) MaterialTheme.colorScheme.primary else Color.Transparent,
                            shape = shape,
                        )
                        .selectable(selected = active, role = Role.RadioButton, onClick = { onSelect(value) })
                        .padding(horizontal = 6.dp, vertical = 6.dp),
                    contentAlignment = Alignment.Center,
                ) {
                    Text(
                        text = label,
                        style = MaterialTheme.typography.labelLarge,
                        color = if (active) MaterialTheme.colorScheme.onPrimaryContainer
                        else MaterialTheme.colorScheme.onSurfaceVariant,
                        fontWeight = if (active) FontWeight.Bold else FontWeight.Medium,
                        maxLines = 1,
                    )
                }
            }
        }
    }
}

private fun trackKey(track: PlayerTrackChoice): String = "${track.trackIndex}:${track.label}"

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
