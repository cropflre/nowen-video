package com.nowen.video.v2.feature.main

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.offset
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Opacity
import androidx.compose.material.icons.filled.Speed
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material.icons.filled.Timer
import androidx.compose.material.icons.filled.Tune
import androidx.compose.material.icons.filled.ViewList
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.pointer.pointerInput
import androidx.compose.ui.layout.onSizeChanged
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.stateDescription
import androidx.compose.ui.unit.IntSize
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.unit.IntOffset
import androidx.compose.ui.unit.dp
import androidx.compose.foundation.shape.CircleShape
import kotlin.math.roundToInt
import com.nowen.video.v2.core.data.DanmakuPreferences

@Composable
internal fun SettingsDanmakuPanel(
    preferences: DanmakuPreferences,
    testState: String?,
    onBack: () -> Unit,
    onApiBaseUrlChange: (String) -> Unit,
    onApiTokenChange: (String) -> Unit,
    onEnabledChange: (Boolean) -> Unit,
    onAutoMatchChange: (Boolean) -> Unit,
    onModeChange: (String) -> Unit,
    onFontSizeChange: (Float) -> Unit,
    onSpeedChange: (Float) -> Unit,
    onOpacityChange: (Float) -> Unit,
    onMaxVisibleChange: (Int) -> Unit,
    onOffsetChange: (Int) -> Unit,
    onTestConnection: () -> Unit,
) {
    var showServiceDialog by remember { mutableStateOf(false) }
    SettingsScaffold(title = "弹幕", onBack = onBack) {
        item {
            SettingsSectionHeader("弹幕")
            Spacer(Modifier.height(8.dp))
            SettingsSwitchRow(
                title = "显示弹幕",
                subtitle = "在播放器画面上显示来自弹幕 API 的内容",
                checked = preferences.enabled,
                icon = Icons.Default.Subtitles,
                onCheckedChange = onEnabledChange,
            )
            SettingsSwitchRow(
                title = "自动匹配",
                subtitle = "播放器打开媒体时自动查找匹配弹幕",
                checked = preferences.autoMatch,
                icon = Icons.Default.Dns,
                onCheckedChange = onAutoMatchChange,
            )
            DanmakuDraggableSetting(
                icon = Icons.Default.Subtitles,
                title = "显示模式",
                value = modeLabel(preferences.mode),
                options = listOf("scroll" to "滚动", "top" to "顶部", "bottom" to "底部"),
                selected = preferences.mode,
                onSelected = onModeChange,
            )
            DanmakuDraggableSetting(
                icon = Icons.Default.Tune,
                title = "字号",
                value = "${preferences.fontSizeSp.toInt()} sp",
                options = listOf(14f, 18f, 22f, 26f).map { it.toString() to "${it.toInt()} sp" },
                selected = preferences.fontSizeSp.toString(),
                onSelected = { onFontSizeChange(it.toFloat()) },
            )
            DanmakuDraggableSetting(
                icon = Icons.Default.Speed,
                title = "速度",
                value = "${preferences.speed}×",
                options = listOf(0.75f, 1f, 1.25f, 1.5f, 2f).map { it.toString() to "${it}×" },
                selected = preferences.speed.toString(),
                onSelected = { onSpeedChange(it.toFloat()) },
            )
            DanmakuDraggableSetting(
                icon = Icons.Default.Opacity,
                title = "透明度",
                value = "${(preferences.opacity * 100).toInt()}%",
                options = listOf(0.5f, 0.7f, 0.85f, 1f).map { it.toString() to "${(it * 100).toInt()}%" },
                selected = preferences.opacity.toString(),
                onSelected = { onOpacityChange(it.toFloat()) },
            )
            DanmakuDraggableSetting(
                icon = Icons.Default.ViewList,
                title = "同时显示",
                value = "${preferences.maxVisible} 条",
                options = listOf(5, 10, 15, 20, 30).map { it.toString() to "$it 条" },
                selected = preferences.maxVisible.toString(),
                onSelected = { onMaxVisibleChange(it.toInt()) },
            )
            Spacer(Modifier.height(28.dp))
        }
        item {
            SettingsSectionHeader("时间")
            Spacer(Modifier.height(8.dp))
            DanmakuDraggableSetting(
                icon = Icons.Default.Timer,
                title = "时间偏移",
                subtitle = "只调整弹幕显示时间，不改变视频播放位置",
                value = "${preferences.offsetMs} ms",
                options = listOf(-30_000, -10_000, 0, 10_000, 30_000).map { it.toString() to "${it / 1000} 秒" },
                selected = preferences.offsetMs.toString(),
                onSelected = { onOffsetChange(it.toInt()) },
            )
            Spacer(Modifier.height(28.dp))
        }
        item {
            SettingsSectionHeader("服务")
            Spacer(Modifier.height(8.dp))
            SettingsListRow(
                icon = Icons.Default.Dns,
                title = "弹幕服务",
                subtitle = if (preferences.apiBaseUrl.isBlank()) "未配置" else "已配置外部弹幕服务",
                value = "配置",
                onClick = { showServiceDialog = true },
            )
            Text(
                "弹幕仅来自已配置的外部服务；离线播放暂不请求弹幕服务。",
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodySmall,
                modifier = Modifier.padding(top = 20.dp),
            )
        }
    }
    if (showServiceDialog) {
        DanmakuServiceDialog(
            apiBaseUrl = preferences.apiBaseUrl,
            apiToken = preferences.apiToken,
            testState = testState,
            onApiBaseUrlChange = onApiBaseUrlChange,
            onApiTokenChange = onApiTokenChange,
            onTestConnection = onTestConnection,
            onDismiss = { showServiceDialog = false },
        )
    }
}

private fun modeLabel(mode: String): String = when (mode) {
    "top" -> "顶部"
    "bottom" -> "底部"
    else -> "滚动"
}

@Composable
private fun DanmakuDraggableSetting(
    icon: ImageVector,
    title: String,
    value: String,
    options: List<Pair<String, String>>,
    selected: String,
    onSelected: (String) -> Unit,
    subtitle: String? = null,
) {
    var draftSelected by remember(selected) { mutableStateOf(selected) }
    SettingsListRow(
        icon = icon,
        title = title,
        subtitle = subtitle,
        value = options.firstOrNull { it.first == draftSelected }?.second ?: value,
        onClick = null,
        showChevron = false,
    )
    com.nowen.video.v2.core.designsystem.HillsDiscreteDragRail(
        options = options,
        selected = selected,
        onPreview = { draftSelected = it },
        onCommit = onSelected,
        modifier = Modifier.padding(start = 48.dp, end = 0.dp, bottom = 8.dp),
    )
}
