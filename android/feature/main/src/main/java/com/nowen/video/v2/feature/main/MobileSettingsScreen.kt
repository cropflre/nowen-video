package com.nowen.video.v2.feature.main

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.LazyRow
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.PictureInPicture
import androidx.compose.material.icons.filled.PlayCircle
import androidx.compose.material.icons.filled.Storage
import androidx.compose.material3.Button
import androidx.compose.material3.FilterChip
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.ViewModel
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.OfflineDownloadRepository
import com.nowen.video.v2.core.data.PlayerPreferences
import com.nowen.video.v2.core.data.PlayerPreferencesStore
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.ElevatedPanel
import com.nowen.video.v2.core.designsystem.NowenPage
import com.nowen.video.v2.core.model.DEFAULT_OFFLINE_QUOTA_BYTES
import com.nowen.video.v2.core.model.OfflineDownloadPolicy
import com.nowen.video.v2.core.model.OfflineStorageStats
import dagger.hilt.android.lifecycle.HiltViewModel
import javax.inject.Inject
import kotlinx.coroutines.launch

private val SETTINGS_SPEEDS = listOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f, 3f, 4f, 6f, 8f)
private val SETTINGS_QUOTAS_GIB = listOf(5, 10, 20, 50, 100)

@HiltViewModel
class MobileSettingsViewModel @Inject constructor(
    private val playerPreferencesStore: PlayerPreferencesStore,
    private val offlineDownloads: OfflineDownloadRepository,
    val sessionStore: ServerSessionStore,
) : ViewModel() {
    val playerPreferences = playerPreferencesStore.preferences
    val downloadPolicy = offlineDownloads.policy
    val storageStats = offlineDownloads.storageStats

    fun setPlaybackSpeed(speed: Float) = viewModelScope.launch {
        playerPreferencesStore.setPlaybackSpeed(speed)
    }

    fun setResizeMode(mode: Int) = viewModelScope.launch {
        playerPreferencesStore.setResizeMode(mode)
    }

    fun setAutoPlayNext(enabled: Boolean) = viewModelScope.launch {
        playerPreferencesStore.setAutoPlayNext(enabled)
    }

    fun setPictureInPictureEnabled(enabled: Boolean) = viewModelScope.launch {
        playerPreferencesStore.setPictureInPictureEnabled(enabled)
    }

    fun setWifiOnly(enabled: Boolean) = viewModelScope.launch {
        offlineDownloads.setWifiOnly(enabled)
    }

    fun setQuotaGiB(gib: Int) = viewModelScope.launch {
        offlineDownloads.setMaxBytes(gib.toLong() * 1024L * 1024L * 1024L)
    }

    fun activateServer(serverId: String) = viewModelScope.launch {
        sessionStore.activate(serverId)
    }
}

@Composable
fun MobileSettingsScreen(
    onBack: () -> Unit,
    viewModel: MobileSettingsViewModel = hiltViewModel(),
) {
    val context = LocalContext.current
    val player by viewModel.playerPreferences.collectAsState(initial = PlayerPreferences())
    val policy by viewModel.downloadPolicy.collectAsState(initial = OfflineDownloadPolicy())
    val storage by viewModel.storageStats.collectAsState(
        initial = OfflineStorageStats(quotaBytes = DEFAULT_OFFLINE_QUOTA_BYTES),
    )
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var notificationsAllowed by remember { mutableStateOf(notificationsAllowed(context)) }
    val notificationPermissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        notificationsAllowed = granted && notificationsAllowed(context)
    }

    NowenPage(Modifier, PaddingValues(horizontal = 20.dp, vertical = 16.dp)) {
        Row(
            modifier = Modifier.fillMaxWidth(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            IconButton(onClick = onBack) {
                Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "返回")
            }
            Spacer(Modifier.width(4.dp))
            Column {
                Text("客户端设置", style = MaterialTheme.typography.headlineMedium)
                Text(
                    "只管理移动端体验，不混入服务器后台配置",
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodySmall,
                )
            }
        }
        Spacer(Modifier.height(16.dp))

        LazyColumn(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(14.dp),
            contentPadding = PaddingValues(bottom = 28.dp),
        ) {
            item {
                SettingsSectionTitle(Icons.Default.Dns, "服务器")
                Spacer(Modifier.height(8.dp))
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    Text(
                        session.activeServer?.name ?: "未连接",
                        style = MaterialTheme.typography.titleMedium,
                    )
                    Spacer(Modifier.height(3.dp))
                    Text(
                        session.activeServer?.baseUrl ?: "",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis,
                    )
                    session.activeServer?.serverVersion?.takeIf(String::isNotBlank)?.let { version ->
                        Spacer(Modifier.height(4.dp))
                        Text("服务器版本 $version", style = MaterialTheme.typography.bodySmall)
                    }
                }
                if (session.servers.size > 1) {
                    Spacer(Modifier.height(8.dp))
                    session.servers.forEach { server ->
                        val active = server.id == session.activeServerId
                        Row(
                            modifier = Modifier
                                .fillMaxWidth()
                                .clickable(enabled = !active) { viewModel.activateServer(server.id) }
                                .padding(vertical = 10.dp),
                            verticalAlignment = Alignment.CenterVertically,
                        ) {
                            Box(Modifier.weight(1f)) {
                                Column {
                                    Text(server.name, style = MaterialTheme.typography.titleSmall)
                                    Text(
                                        server.baseUrl,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        style = MaterialTheme.typography.bodySmall,
                                    )
                                }
                            }
                            if (active) Icon(Icons.Default.Check, contentDescription = "当前服务器")
                        }
                    }
                }
            }

            item {
                SettingsSectionTitle(Icons.Default.PlayCircle, "播放")
                Spacer(Modifier.height(8.dp))
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    Text("默认倍速", style = MaterialTheme.typography.titleMedium)
                    Spacer(Modifier.height(8.dp))
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(SETTINGS_SPEEDS) { speed ->
                            FilterChip(
                                selected = player.playbackSpeed == speed,
                                onClick = { viewModel.setPlaybackSpeed(speed) },
                                label = { Text(speedDisplayLabel(speed)) },
                            )
                        }
                    }
                    Spacer(Modifier.height(14.dp))
                    Text("画面模式", style = MaterialTheme.typography.titleMedium)
                    Spacer(Modifier.height(8.dp))
                    Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        listOf(0 to "适应", 1 to "裁切", 2 to "拉伸").forEach { (mode, label) ->
                            FilterChip(
                                selected = player.resizeMode == mode,
                                onClick = { viewModel.setResizeMode(mode) },
                                label = { Text(label) },
                            )
                        }
                    }
                    Spacer(Modifier.height(10.dp))
                    SettingsSwitchRow(
                        title = "自动播放下一集",
                        subtitle = "剧集播放结束后自动进入下一集",
                        checked = player.autoPlayNext,
                        onCheckedChange = viewModel::setAutoPlayNext,
                    )
                    SettingsSwitchRow(
                        title = "画中画",
                        subtitle = "播放时返回桌面自动进入小窗",
                        checked = player.pictureInPictureEnabled,
                        onCheckedChange = viewModel::setPictureInPictureEnabled,
                    )
                }
            }

            item {
                SettingsSectionTitle(Icons.Default.Storage, "离线下载")
                Spacer(Modifier.height(8.dp))
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    SettingsSwitchRow(
                        title = "仅 Wi-Fi 下载",
                        subtitle = "避免移动网络消耗大流量",
                        checked = policy.wifiOnly,
                        onCheckedChange = viewModel::setWifiOnly,
                    )
                    Spacer(Modifier.height(10.dp))
                    Text("离线空间上限", style = MaterialTheme.typography.titleMedium)
                    Text(
                        "已使用 ${formatStorageBytes(storage.usedBytes)} / ${formatStorageBytes(storage.quotaBytes)} · 设备剩余 ${formatStorageBytes(storage.deviceFreeBytes)}",
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                        style = MaterialTheme.typography.bodySmall,
                    )
                    Spacer(Modifier.height(8.dp))
                    LazyRow(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                        items(SETTINGS_QUOTAS_GIB) { gib ->
                            val selected = policy.maxBytes == gib.toLong() * 1024L * 1024L * 1024L
                            FilterChip(
                                selected = selected,
                                onClick = { viewModel.setQuotaGiB(gib) },
                                label = { Text("${gib}GB") },
                            )
                        }
                    }
                }
            }

            item {
                SettingsSectionTitle(Icons.Default.Notifications, "下载通知")
                Spacer(Modifier.height(8.dp))
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    Text(
                        if (notificationsAllowed) "通知已开启" else "通知未开启",
                        style = MaterialTheme.typography.titleMedium,
                    )
                    Text(
                        if (notificationsAllowed) {
                            "后台下载时会显示实时进度。"
                        } else {
                            "下载仍可继续，但系统可能不显示后台进度提醒。"
                        },
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                    Spacer(Modifier.height(10.dp))
                    if (!notificationsAllowed && Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                        ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
                    ) {
                        Button(onClick = {
                            notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
                        }) {
                            Text("允许下载通知")
                        }
                    } else if (!notificationsAllowed) {
                        OutlinedButton(onClick = { openNotificationSettings(context) }) {
                            Text("打开系统通知设置")
                        }
                    }
                }
            }

            item {
                SettingsSectionTitle(Icons.Default.Info, "关于与诊断")
                Spacer(Modifier.height(8.dp))
                ElevatedPanel(Modifier.fillMaxWidth()) {
                    SettingsInfoRow("Nowen Video", packageVersionName(context))
                    SettingsInfoRow("Android", "${Build.VERSION.RELEASE} · API ${Build.VERSION.SDK_INT}")
                    SettingsInfoRow("用户", session.user?.username ?: "-")
                    SettingsInfoRow("画中画能力", if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) "支持" else "不支持")
                    SettingsInfoRow("会话保护", "自动续期 · 同源授权")
                }
            }
        }
    }
}

@Composable
private fun SettingsSectionTitle(
    icon: androidx.compose.ui.graphics.vector.ImageVector,
    title: String,
) {
    Row(verticalAlignment = Alignment.CenterVertically) {
        Icon(icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary)
        Spacer(Modifier.width(9.dp))
        Text(title, style = MaterialTheme.typography.titleLarge)
    }
}

@Composable
private fun SettingsSwitchRow(
    title: String,
    subtitle: String,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 7.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(title, style = MaterialTheme.typography.titleSmall)
            Text(
                subtitle,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.bodySmall,
            )
        }
        Spacer(Modifier.width(12.dp))
        Switch(checked = checked, onCheckedChange = onCheckedChange)
    }
}

@Composable
private fun SettingsInfoRow(label: String, value: String) {
    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(vertical = 5.dp),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, color = MaterialTheme.colorScheme.onSurfaceVariant)
        Spacer(Modifier.width(16.dp))
        Text(value, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

internal fun formatStorageBytes(bytes: Long): String {
    val safe = bytes.coerceAtLeast(0L).toDouble()
    val gib = 1024.0 * 1024.0 * 1024.0
    val mib = 1024.0 * 1024.0
    return when {
        safe >= gib -> "%.1f GB".format(safe / gib)
        safe >= mib -> "%.0f MB".format(safe / mib)
        else -> "%.0f KB".format(safe / 1024.0)
    }
}

private fun packageVersionName(context: Context): String = runCatching {
    context.packageManager.getPackageInfo(context.packageName, 0).versionName.orEmpty()
}.getOrDefault("").ifBlank { "未知版本" }

private fun notificationsAllowed(context: Context): Boolean {
    val permissionGranted = Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
        ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
    return permissionGranted && NotificationManagerCompat.from(context).areNotificationsEnabled()
}

private fun openNotificationSettings(context: Context) {
    context.startActivity(
        Intent(Settings.ACTION_APP_NOTIFICATION_SETTINGS)
            .putExtra(Settings.EXTRA_APP_PACKAGE, context.packageName)
            .addFlags(Intent.FLAG_ACTIVITY_NEW_TASK),
    )
}
