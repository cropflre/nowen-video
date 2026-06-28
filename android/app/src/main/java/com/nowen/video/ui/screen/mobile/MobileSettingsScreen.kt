package com.nowen.video.ui.screen.mobile

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Cast
import androidx.compose.material.icons.filled.CloudDownload
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.FamilyRestroom
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Language
import androidx.compose.material.icons.filled.NetworkCheck
import androidx.compose.material.icons.filled.Notifications
import androidx.compose.material.icons.filled.Palette
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Subtitles
import androidx.compose.material.icons.filled.Sync
import androidx.compose.material.icons.filled.VideoSettings
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import androidx.hilt.navigation.compose.hiltViewModel
import com.nowen.video.ui.component.mobile.MobilePageHeader
import com.nowen.video.ui.component.mobile.SettingsGroup
import com.nowen.video.ui.component.mobile.SettingsRow
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 移动端设置页面
 * Hills Pro 风格：品牌卡片 + 分组列表
 */
@Composable
fun MobileSettingsScreen(
    onServerManageClick: () -> Unit,
    onPlayerSettingsClick: () -> Unit,
    onConnectionDiagnosticClick: () -> Unit,
    onNotificationsClick: () -> Unit,
    onDownloadsClick: () -> Unit,
    onSubtitleCenterClick: () -> Unit,
    onSmartDiscoveryClick: () -> Unit,
    onRemoteAccessClick: () -> Unit,
    onCastClick: () -> Unit,
    onFamilyModeClick: () -> Unit,
    onDeviceAdaptationClick: () -> Unit,
    modifier: Modifier = Modifier,
    viewModel: MobileSettingsViewModel = hiltViewModel(),
) {
    val uiState by viewModel.uiState.collectAsState()

    LazyColumn(
        modifier = modifier.fillMaxSize(),
    ) {
        // 页面标题
        item {
            MobilePageHeader(title = "设置")
        }

        // 用户/服务器卡片
        item {
            UserServerCard(
                username = uiState.username,
                userRole = uiState.userRole,
                serverUrl = uiState.serverUrl,
                isLoggedIn = uiState.isLoggedIn,
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = MobileSpacing.xl),
            )
        }

        // 功能分组
        item {
            SettingsGroup(
                title = "功能",
                rows = listOf(
                    SettingsRow.Action(
                        icon = Icons.Default.Dns,
                        title = "服务器管理",
                        onClick = onServerManageClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.PlayArrow,
                        title = "播放器设置",
                        onClick = onPlayerSettingsClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.NetworkCheck,
                        title = "连接诊断",
                        onClick = onConnectionDiagnosticClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Notifications,
                        title = "后台任务",
                        status = "未连接",
                        onClick = onNotificationsClick,
                    ),
                ),
            )
        }

        // 移动端能力分组
        item {
            SettingsGroup(
                title = "能力",
                rows = listOf(
                    SettingsRow.Action(
                        icon = Icons.Default.CloudDownload,
                        title = "离线下载",
                        enabled = false,
                        badge = "即将支持",
                        onClick = onDownloadsClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Subtitles,
                        title = "字幕中心",
                        enabled = false,
                        badge = "即将支持",
                        onClick = onSubtitleCenterClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Sync,
                        title = "智能发现",
                        enabled = false,
                        badge = "即将支持",
                        onClick = onSmartDiscoveryClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Language,
                        title = "远程访问",
                        enabled = false,
                        badge = "即将支持",
                        onClick = onRemoteAccessClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Cast,
                        title = "投屏与遥控",
                        enabled = false,
                        badge = "即将支持",
                        onClick = onCastClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.FamilyRestroom,
                        title = "家庭与儿童模式",
                        enabled = false,
                        badge = "即将支持",
                        onClick = onFamilyModeClick,
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.VideoSettings,
                        title = "设备适配",
                        enabled = false,
                        badge = "即将支持",
                        onClick = onDeviceAdaptationClick,
                    ),
                ),
            )
        }

        // 外观分组
        item {
            SettingsGroup(
                title = "外观",
                rows = listOf(
                    SettingsRow.Action(
                        icon = Icons.Default.Palette,
                        title = "主题模式",
                        status = "跟随系统",
                        onClick = { /* TODO: 主题选择 */ },
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Language,
                        title = "语言",
                        status = "简体中文",
                        onClick = { /* TODO: 语言选择 */ },
                    ),
                ),
            )
        }

        // 关于分组
        item {
            SettingsGroup(
                title = "关于",
                rows = listOf(
                    SettingsRow.Action(
                        icon = Icons.Default.Info,
                        title = "应用版本",
                        status = uiState.appVersion,
                        showArrow = false,
                        onClick = { },
                    ),
                    SettingsRow.Action(
                        icon = Icons.Default.Person,
                        title = "关于 Nowen Video",
                        onClick = { /* TODO: 关于页面 */ },
                    ),
                ),
            )
        }
    }
}

/**
 * 用户/服务器卡片
 */
@Composable
private fun UserServerCard(
    username: String,
    userRole: String,
    serverUrl: String,
    isLoggedIn: Boolean,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(MobileRadius.xxl))
            .background(
                Brush.linearGradient(
                    colors = listOf(
                        Color(0xFF4A5FC1),
                        Color(0xFF6366F1),
                    ),
                ),
            )
            .padding(MobileSpacing.xl),
    ) {
        Column(
            verticalArrangement = Arrangement.spacedBy(MobileSpacing.sm),
        ) {
            Text(
                text = "Nowen Video",
                color = Color.White,
                fontSize = MobileFontSize.xxxl,
                fontWeight = FontWeight.Bold,
            )
            Text(
                text = "私人影音中心",
                color = Color.White.copy(alpha = 0.8f),
                fontSize = MobileFontSize.lg,
            )

            if (isLoggedIn) {
                Column(
                    modifier = Modifier.padding(top = MobileSpacing.sm),
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    Text(
                        text = "用户: $username",
                        color = Color.White.copy(alpha = 0.7f),
                        fontSize = MobileFontSize.sm,
                    )
                    Text(
                        text = "角色: ${if (userRole == "admin") "管理员" else "普通用户"}",
                        color = Color.White.copy(alpha = 0.7f),
                        fontSize = MobileFontSize.sm,
                    )
                    if (serverUrl.isNotBlank()) {
                        Text(
                            text = "服务器: $serverUrl",
                            color = Color.White.copy(alpha = 0.7f),
                            fontSize = MobileFontSize.sm,
                        )
                    }
                }
            } else {
                Text(
                    text = "未登录",
                    color = Color.White.copy(alpha = 0.7f),
                    fontSize = MobileFontSize.sm,
                )
            }
        }
    }
}
