package com.nowen.video.v2.feature.main

import android.Manifest
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.provider.Settings
import androidx.activity.compose.BackHandler
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.AccountCircle
import androidx.compose.material.icons.filled.Backup
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.CloudSync
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Explore
import androidx.compose.material.icons.filled.FolderOpen
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Interests
import androidx.compose.material.icons.filled.Language
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Palette
import androidx.compose.material.icons.filled.PictureInPicture
import androidx.compose.material.icons.filled.PlayCircle
import androidx.compose.material.icons.filled.Science
import androidx.compose.material.icons.filled.Storage
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.app.NotificationManagerCompat
import androidx.core.content.ContextCompat
import androidx.hilt.navigation.compose.hiltViewModel
import androidx.lifecycle.Lifecycle
import androidx.lifecycle.LifecycleEventObserver
import androidx.lifecycle.ViewModel
import androidx.lifecycle.compose.LocalLifecycleOwner
import androidx.lifecycle.viewModelScope
import com.nowen.video.v2.core.data.OfflineDownloadRepository
import com.nowen.video.v2.core.data.PlayerPreferences
import com.nowen.video.v2.core.data.PlayerPreferencesStore
import com.nowen.video.v2.core.data.supportedLongPressBoostSpeeds
import com.nowen.video.v2.core.data.ServerSessionStore
import com.nowen.video.v2.core.designsystem.HillsPressable
import com.nowen.video.v2.core.designsystem.HillsToggle
import com.nowen.video.v2.core.designsystem.ProductIdentity
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

    fun setLongPressBoostSpeed(speed: Float) = viewModelScope.launch {
        playerPreferencesStore.setLongPressBoostSpeed(speed)
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
    onBack: (() -> Unit)? = null,
    viewModel: MobileSettingsViewModel = hiltViewModel(),
) {
    val context = LocalContext.current
    val lifecycleOwner = LocalLifecycleOwner.current
    val player by viewModel.playerPreferences.collectAsState(initial = PlayerPreferences())
    val policy by viewModel.downloadPolicy.collectAsState(initial = OfflineDownloadPolicy())
    val storage by viewModel.storageStats.collectAsState(
        initial = OfflineStorageStats(quotaBytes = DEFAULT_OFFLINE_QUOTA_BYTES),
    )
    val session by viewModel.sessionStore.snapshot.collectAsState()
    var notificationsAllowed by remember { mutableStateOf(notificationsAllowed(context)) }
    var destination by remember { mutableStateOf<SettingsDestination?>(null) }
    var picker by remember { mutableStateOf<SettingsPicker?>(null) }
    val returnToDirectory = { destination = null }
    BackHandler(enabled = destination != null) {
        returnToDirectory()
    }
    val notificationPermissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestPermission(),
    ) { granted ->
        notificationsAllowed = granted && notificationsAllowed(context)
    }

    DisposableEffect(lifecycleOwner, context) {
        val observer = LifecycleEventObserver { _, event ->
            if (event == Lifecycle.Event.ON_RESUME) {
                notificationsAllowed = notificationsAllowed(context)
            }
        }
        lifecycleOwner.lifecycle.addObserver(observer)
        onDispose { lifecycleOwner.lifecycle.removeObserver(observer) }
    }

    when (destination) {
        null -> SettingsDirectory(
            onBack = onBack,
            onSelect = { selected -> destination = selected },
        )

        SettingsDestination.Account -> SettingsAccountPanel(
            username = session.user?.username.orEmpty(),
            serverName = session.activeServer?.name.orEmpty(),
            canSwitchServer = session.servers.size > 1,
            onBack = returnToDirectory,
            onServerSelect = { picker = SettingsPicker.Server },
        )

        SettingsDestination.Player -> SettingsPlayerPanel(
            player = player,
            onBack = returnToDirectory,
            onSpeedClick = { picker = SettingsPicker.Speed },
            onLongPressBoostSpeedClick = { picker = SettingsPicker.LongPressBoostSpeed },
            onResizeModeClick = { picker = SettingsPicker.ResizeMode },
            onAutoPlayNextChange = viewModel::setAutoPlayNext,
            onPictureInPictureChange = viewModel::setPictureInPictureEnabled,
        )

        SettingsDestination.Interaction -> SettingsInteractionPanel(
            player = player,
            onBack = returnToDirectory,
            onLongPressBoostSpeedClick = { picker = SettingsPicker.LongPressBoostSpeed },
        )

        SettingsDestination.Library -> SettingsLibraryPanel(
            policy = policy,
            storage = storage,
            notificationsAllowed = notificationsAllowed,
            onBack = returnToDirectory,
            onWifiOnlyChange = viewModel::setWifiOnly,
            onQuotaClick = { picker = SettingsPicker.Quota },
            onNotificationsClick = {
                if (
                    Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU &&
                    ContextCompat.checkSelfPermission(context, Manifest.permission.POST_NOTIFICATIONS) != PackageManager.PERMISSION_GRANTED
                ) {
                    notificationPermissionLauncher.launch(Manifest.permission.POST_NOTIFICATIONS)
                } else {
                    openNotificationSettings(context)
                }
            },
        )

        SettingsDestination.About -> SettingsAboutPanel(
            onBack = returnToDirectory,
            versionName = packageVersionName(context),
            username = session.user?.username.orEmpty(),
        )

        else -> SettingsUnavailablePanel(
            destination = destination!!,
            onBack = returnToDirectory,
        )
    }

    when (picker) {
        SettingsPicker.Server -> SettingsValuePickerDialog(
            title = "选择服务器",
            options = session.servers.map { it.id to it.name },
            selected = session.activeServerId.orEmpty(),
            onDismiss = { picker = null },
            onSelect = { serverId ->
                viewModel.activateServer(serverId)
                picker = null
            },
        )

        SettingsPicker.Speed -> SettingsValuePickerDialog(
            title = "默认倍速",
            options = SETTINGS_SPEEDS.map { speed -> speed.toString() to speedDisplayLabel(speed) },
            selected = player.playbackSpeed.toString(),
            onDismiss = { picker = null },
            onSelect = { speed ->
                viewModel.setPlaybackSpeed(speed.toFloat())
                picker = null
            },
        )

        SettingsPicker.LongPressBoostSpeed -> SettingsValuePickerDialog(
            title = "长按倍速",
            options = supportedLongPressBoostSpeeds.map { speed -> speed.toString() to speedDisplayLabel(speed) },
            selected = player.longPressBoostSpeed.toString(),
            onDismiss = { picker = null },
            onSelect = { speed ->
                viewModel.setLongPressBoostSpeed(speed.toFloat())
                picker = null
            },
        )

        SettingsPicker.ResizeMode -> SettingsValuePickerDialog(
            title = "画面模式",
            options = listOf("0" to "适应", "1" to "裁切", "2" to "拉伸"),
            selected = player.resizeMode.toString(),
            onDismiss = { picker = null },
            onSelect = { mode ->
                viewModel.setResizeMode(mode.toInt())
                picker = null
            },
        )

        SettingsPicker.Quota -> SettingsValuePickerDialog(
            title = "离线空间上限",
            options = SETTINGS_QUOTAS_GIB.map { gib -> gib.toString() to "${gib} GB" },
            selected = (policy.maxBytes / (1024L * 1024L * 1024L)).toString(),
            onDismiss = { picker = null },
            onSelect = { gib ->
                viewModel.setQuotaGiB(gib.toInt())
                picker = null
            },
        )

        null -> Unit
    }
}

private enum class SettingsDestination(
    val label: String,
    val icon: ImageVector,
    val available: Boolean,
) {
    Account("账号", Icons.Default.AccountCircle, true),
    Language("语言", Icons.Default.Language, false),
    Theme("主题", Icons.Default.Palette, false),
    Library("媒体库", Icons.Default.FolderOpen, true),
    Backup("备份与还原", Icons.Default.Backup, false),
    Sync("同步", Icons.Default.CloudSync, false),
    Interaction("交互", Icons.Default.Interests, true),
    Player("播放器", Icons.Default.PlayCircle, true),
    Danmaku("弹幕", Icons.Default.Subtitles, false),
    Experimental("实验性", Icons.Default.Science, false),
    About("关于", Icons.Default.Info, true),
}

private enum class SettingsPicker {
    Server,
    Speed,
    LongPressBoostSpeed,
    ResizeMode,
    Quota,
}

@Composable
private fun SettingsDirectory(
    onBack: (() -> Unit)?,
    onSelect: (SettingsDestination) -> Unit,
) {
    SettingsScaffold(
        title = "设置",
        onBack = onBack,
    ) {
        item {
            SettingsSectionHeader("通用")
            Spacer(Modifier.height(8.dp))
            SettingsDestinationRow(SettingsDestination.Account, onClick = { onSelect(SettingsDestination.Account) })
            SettingsDestinationRow(SettingsDestination.Language, onClick = { onSelect(SettingsDestination.Language) })
            SettingsDestinationRow(SettingsDestination.Theme, onClick = { onSelect(SettingsDestination.Theme) })
            SettingsDestinationRow(SettingsDestination.Library, onClick = { onSelect(SettingsDestination.Library) })
            SettingsDestinationRow(SettingsDestination.Backup, onClick = { onSelect(SettingsDestination.Backup) })
            SettingsDestinationRow(SettingsDestination.Sync, onClick = { onSelect(SettingsDestination.Sync) })
            Spacer(Modifier.height(30.dp))
        }
        item {
            SettingsSectionHeader("播放器")
            Spacer(Modifier.height(8.dp))
            SettingsDestinationRow(SettingsDestination.Interaction, onClick = { onSelect(SettingsDestination.Interaction) })
            SettingsDestinationRow(SettingsDestination.Player, onClick = { onSelect(SettingsDestination.Player) })
            SettingsDestinationRow(SettingsDestination.Danmaku, onClick = { onSelect(SettingsDestination.Danmaku) })
            SettingsDestinationRow(SettingsDestination.Experimental, onClick = { onSelect(SettingsDestination.Experimental) })
            Spacer(Modifier.height(30.dp))
        }
        item {
            SettingsSectionHeader("关于")
            Spacer(Modifier.height(8.dp))
            SettingsDestinationRow(SettingsDestination.About, onClick = { onSelect(SettingsDestination.About) })
        }
    }
}

@Composable
private fun SettingsAccountPanel(
    username: String,
    serverName: String,
    canSwitchServer: Boolean,
    onBack: () -> Unit,
    onServerSelect: () -> Unit,
) {
    SettingsScaffold(title = "账号", onBack = onBack) {
        item {
            SettingsListRow(
                icon = Icons.Default.AccountCircle,
                title = "账号",
                subtitle = username.ifBlank { "未登录" },
            )
            SettingsListRow(
                icon = Icons.Default.Dns,
                title = "当前服务器",
                subtitle = serverName.ifBlank { "未连接" },
                value = if (canSwitchServer) "切换" else null,
                onClick = if (canSwitchServer) onServerSelect else null,
            )
        }
    }
}

@Composable
private fun SettingsPlayerPanel(
    player: PlayerPreferences,
    onBack: () -> Unit,
    onSpeedClick: () -> Unit,
    onLongPressBoostSpeedClick: () -> Unit,
    onResizeModeClick: () -> Unit,
    onAutoPlayNextChange: (Boolean) -> Unit,
    onPictureInPictureChange: (Boolean) -> Unit,
) {
    SettingsScaffold(title = "播放器", onBack = onBack) {
        item {
            SettingsListRow(
                icon = Icons.Default.PlayCircle,
                title = "默认倍速",
                value = speedDisplayLabel(player.playbackSpeed),
                onClick = onSpeedClick,
            )
            SettingsListRow(
                icon = Icons.Default.PlayCircle,
                title = "长按倍速",
                subtitle = "播放时长按画面临时加速",
                value = speedDisplayLabel(player.longPressBoostSpeed),
                onClick = onLongPressBoostSpeedClick,
            )
            SettingsListRow(
                icon = Icons.Default.PlayCircle,
                title = "画面模式",
                value = resizeModeLabel(player.resizeMode),
                onClick = onResizeModeClick,
            )
            SettingsSwitchRow(
                title = "自动播放下一集",
                subtitle = "剧集结束后自动进入下一集",
                checked = player.autoPlayNext,
                onCheckedChange = onAutoPlayNextChange,
            )
            SettingsSwitchRow(
                title = "画中画",
                subtitle = if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) "播放时返回桌面自动进入小窗" else "当前设备不支持",
                checked = player.pictureInPictureEnabled,
                enabled = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O,
                onCheckedChange = onPictureInPictureChange,
            )
        }
    }
}

@Composable
private fun SettingsInteractionPanel(
    player: PlayerPreferences,
    onBack: () -> Unit,
    onLongPressBoostSpeedClick: () -> Unit,
) {
    SettingsScaffold(title = "交互", onBack = onBack) {
        item {
            SettingsSectionHeader("播放手势")
            Spacer(Modifier.height(8.dp))
            SettingsListRow(
                icon = Icons.Default.Explore,
                title = "亮度",
                subtitle = "播放时在画面左侧上下滑动调整亮度",
            )
            SettingsListRow(
                icon = Icons.Default.Explore,
                title = "音量",
                subtitle = "播放时在画面右侧上下滑动调整媒体音量",
            )
            SettingsListRow(
                icon = Icons.Default.Explore,
                title = "快进与快退",
                subtitle = "在画面任意位置左右滑动调整播放进度",
            )
            Spacer(Modifier.height(20.dp))
            SettingsSectionHeader("长按播放")
            Spacer(Modifier.height(8.dp))
            SettingsListRow(
                icon = Icons.Default.PlayCircle,
                title = "长按倍速",
                subtitle = "播放时按住画面临时加速，松开后恢复原倍速",
                value = speedDisplayLabel(player.longPressBoostSpeed),
                onClick = onLongPressBoostSpeedClick,
            )
        }
    }
}

@Composable
private fun SettingsLibraryPanel(
    policy: OfflineDownloadPolicy,
    storage: OfflineStorageStats,
    notificationsAllowed: Boolean,
    onBack: () -> Unit,
    onWifiOnlyChange: (Boolean) -> Unit,
    onQuotaClick: () -> Unit,
    onNotificationsClick: () -> Unit,
) {
    SettingsScaffold(title = "媒体库", onBack = onBack) {
        item {
            SettingsSectionHeader("离线下载")
            Spacer(Modifier.height(8.dp))
            SettingsSwitchRow(
                title = "仅 Wi-Fi 下载",
                subtitle = "避免移动网络消耗大流量",
                checked = policy.wifiOnly,
                onCheckedChange = onWifiOnlyChange,
            )
            SettingsListRow(
                icon = Icons.Default.Storage,
                title = "离线空间上限",
                value = formatStorageBytes(policy.maxBytes),
                onClick = onQuotaClick,
            )
            SettingsListRow(
                icon = Icons.Default.Storage,
                title = "已用空间",
                subtitle = "${formatStorageBytes(storage.usedBytes)} / ${formatStorageBytes(storage.quotaBytes)} · 设备剩余 ${formatStorageBytes(storage.deviceFreeBytes)}",
            )
            Spacer(Modifier.height(30.dp))
            SettingsSectionHeader("通知")
            Spacer(Modifier.height(8.dp))
            SettingsListRow(
                icon = Icons.Default.Notifications,
                title = "下载通知",
                subtitle = if (notificationsAllowed) "已开启" else "未开启",
                value = if (notificationsAllowed) null else "设置",
                onClick = if (notificationsAllowed) null else onNotificationsClick,
            )
        }
    }
}

@Composable
private fun SettingsAboutPanel(
    onBack: () -> Unit,
    versionName: String,
    username: String,
) {
    SettingsScaffold(title = "关于", onBack = onBack) {
        item {
            SettingsListRow(
                icon = Icons.Default.Info,
                title = ProductIdentity.displayName,
                subtitle = versionName,
            )
            SettingsInfoRow("Android", "${Build.VERSION.RELEASE} · API ${Build.VERSION.SDK_INT}")
            SettingsInfoRow("当前用户", username.ifBlank { "-" })
            SettingsInfoRow("画中画能力", if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) "支持" else "不支持")
            SettingsInfoRow("会话保护", "自动续期 · 同源授权")
        }
    }
}

@Composable
private fun SettingsUnavailablePanel(
    destination: SettingsDestination,
    onBack: () -> Unit,
) {
    SettingsScaffold(title = destination.label, onBack = onBack) {
        item {
            Column(Modifier.padding(top = 32.dp)) {
                Icon(destination.icon, contentDescription = null, tint = MaterialTheme.colorScheme.primary, modifier = Modifier.size(32.dp))
                Spacer(Modifier.height(16.dp))
                Text("${destination.label}功能暂未开放", color = MaterialTheme.colorScheme.onBackground, style = MaterialTheme.typography.titleLarge, fontWeight = FontWeight.Bold)
                Spacer(Modifier.height(7.dp))
                Text("该入口已预留，后续版本会在这里提供对应设置。", color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodyLarge)
            }
        }
    }
}

@Composable
private fun SettingsScaffold(
    title: String,
    onBack: (() -> Unit)? = null,
    content: androidx.compose.foundation.lazy.LazyListScope.() -> Unit,
) {
    Box(Modifier.fillMaxSize().background(MaterialTheme.colorScheme.background)) {
        LazyColumn(
            modifier = Modifier.fillMaxSize(),
            contentPadding = PaddingValues(start = 20.dp, end = 20.dp, bottom = 116.dp),
            verticalArrangement = Arrangement.spacedBy(0.dp),
        ) {
            item {
                SettingsPageHeader(title = title, onBack = onBack)
                Spacer(Modifier.height(if (onBack == null) 20.dp else 16.dp))
            }
            content()
        }
    }
}

@Composable
private fun SettingsPageHeader(
    title: String,
    onBack: (() -> Unit)?,
) {
    Column(
        modifier = Modifier
            .fillMaxWidth()
            .statusBarsPadding()
            .padding(top = if (onBack == null) 22.dp else 16.dp),
    ) {
        onBack?.let { navigateBack ->
            HillsPressable(
                onClick = navigateBack,
                modifier = Modifier
                    .size(44.dp)
                    .clip(androidx.compose.foundation.shape.CircleShape),
            ) {
                Icon(
                    Icons.AutoMirrored.Filled.ArrowBack,
                    contentDescription = "返回",
                    tint = MaterialTheme.colorScheme.onBackground,
                    modifier = Modifier.align(Alignment.Center),
                )
            }
            Spacer(Modifier.height(14.dp))
        }
        Text(
            title,
            color = MaterialTheme.colorScheme.onBackground,
            fontSize = 30.sp,
            lineHeight = 38.sp,
            fontWeight = FontWeight.Bold,
        )
    }
}

@Composable
private fun SettingsSectionHeader(title: String) {
    Text(
        title,
        color = MaterialTheme.colorScheme.primary,
        style = MaterialTheme.typography.titleMedium,
        fontWeight = FontWeight.Bold,
    )
}

@Composable
private fun SettingsDestinationRow(
    destination: SettingsDestination,
    onClick: () -> Unit,
) {
    val subtitle = when (destination) {
        SettingsDestination.Language -> "简体中文"
        SettingsDestination.Account -> "当前服务器账号"
        else -> if (destination.available) null else "预留入口"
    }
    SettingsListRow(
        icon = destination.icon,
        title = destination.label,
        subtitle = subtitle,
        onClick = onClick,
        showChevron = false,
        compact = true,
    )
}

@Composable
private fun SettingsListRow(
    icon: ImageVector,
    title: String,
    subtitle: String? = null,
    value: String? = null,
    onClick: (() -> Unit)? = null,
    showChevron: Boolean = onClick != null,
    compact: Boolean = false,
) {
    val rowHeight = when {
        compact && subtitle.isNullOrBlank() -> 62.dp
        compact -> 72.dp
        subtitle.isNullOrBlank() -> 66.dp
        else -> 78.dp
    }
    HillsPressable(
        onClick = { onClick?.invoke() },
        enabled = onClick != null,
        modifier = Modifier
            .fillMaxWidth()
            .height(rowHeight),
    ) {
        Row(
            modifier = Modifier.fillMaxSize(),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                icon,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurface.copy(alpha = 0.92f),
                modifier = Modifier.size(if (compact) 26.dp else 28.dp),
            )
            Spacer(Modifier.width(if (compact) 18.dp else 20.dp))
            Column(Modifier.weight(1f)) {
                Text(title, color = MaterialTheme.colorScheme.onBackground, style = MaterialTheme.typography.titleMedium, fontWeight = FontWeight.SemiBold)
                subtitle?.takeIf(String::isNotBlank)?.let {
                    Spacer(Modifier.height(2.dp))
                    Text(it, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
                }
            }
            value?.let {
                Text(
                    it,
                    color = if (onClick != null) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant,
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                    modifier = Modifier.padding(start = 10.dp),
                )
            }
            if (showChevron) {
                Spacer(Modifier.width(6.dp))
                Icon(Icons.Default.ChevronRight, contentDescription = null, tint = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = 0.72f))
            }
        }
    }
}

@Composable
private fun SettingsSwitchRow(
    title: String,
    subtitle: String,
    checked: Boolean,
    enabled: Boolean = true,
    onCheckedChange: (Boolean) -> Unit,
) {
    Row(
        modifier = Modifier.fillMaxWidth().height(78.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Column(Modifier.weight(1f)) {
            Text(
                title,
                color = if (enabled) MaterialTheme.colorScheme.onBackground else MaterialTheme.colorScheme.onSurfaceVariant,
                style = MaterialTheme.typography.titleMedium,
                fontWeight = FontWeight.SemiBold,
            )
            Spacer(Modifier.height(2.dp))
            Text(
                subtitle,
                color = MaterialTheme.colorScheme.onSurfaceVariant.copy(alpha = if (enabled) 1f else 0.58f),
                style = MaterialTheme.typography.bodyMedium,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis,
            )
        }
        Spacer(Modifier.width(12.dp))
        HillsToggle(
            checked = checked,
            onCheckedChange = onCheckedChange,
            enabled = enabled,
        )
    }
}

@Composable
private fun SettingsInfoRow(label: String, value: String) {
    Row(
        modifier = Modifier.fillMaxWidth().padding(vertical = 9.dp),
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Text(label, color = MaterialTheme.colorScheme.onSurfaceVariant, style = MaterialTheme.typography.bodyMedium)
        Spacer(Modifier.width(20.dp))
        Text(value, modifier = Modifier.weight(1f), color = MaterialTheme.colorScheme.onSurface, style = MaterialTheme.typography.bodyMedium, maxLines = 1, overflow = TextOverflow.Ellipsis)
    }
}

@Composable
private fun SettingsValuePickerDialog(
    title: String,
    options: List<Pair<String, String>>,
    selected: String,
    onDismiss: () -> Unit,
    onSelect: (String) -> Unit,
) {
    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(title) },
        text = {
            Column {
                options.forEachIndexed { index, (key, label) ->
                    Row(
                        modifier = Modifier.fillMaxWidth().clickable { onSelect(key) }.padding(vertical = 13.dp),
                        verticalAlignment = Alignment.CenterVertically,
                    ) {
                        Text(label, modifier = Modifier.weight(1f))
                        if (key == selected) Icon(Icons.Default.Check, contentDescription = "当前选项", tint = MaterialTheme.colorScheme.primary)
                    }
                    if (index != options.lastIndex) HorizontalDivider()
                }
            }
        },
        confirmButton = { TextButton(onClick = onDismiss) { Text("取消") } },
    )
}

private fun resizeModeLabel(mode: Int): String = when (mode) {
    1 -> "裁切"
    2 -> "拉伸"
    else -> "适应"
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
