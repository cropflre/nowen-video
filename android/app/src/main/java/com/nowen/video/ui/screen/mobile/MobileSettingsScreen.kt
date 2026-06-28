package com.nowen.video.ui.screen.mobile

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material.icons.filled.Info
import androidx.compose.material.icons.filled.Language
import androidx.compose.material.icons.filled.Palette
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material.icons.filled.Storage
import androidx.compose.material.icons.filled.Sync
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.component.mobile.MobilePageHeader
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
    onSettingsClick: () -> Unit,
    onPlayerSettingsClick: () -> Unit = {},
    onServerManageClick: () -> Unit = {},
    modifier: Modifier = Modifier,
) {
    LazyColumn(
        modifier = modifier.fillMaxSize(),
    ) {
        // 页面标题
        item {
            MobilePageHeader(title = "设置")
        }

        // 品牌卡片
        item {
            BrandCard(
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = MobileSpacing.xl),
            )
        }

        // 通用设置组
        item {
            SettingsGroup(
                title = "通用",
                items = listOf(
                    SettingsItem(
                        icon = Icons.Default.Language,
                        title = "语言",
                        subtitle = "Auto",
                        onClick = { onSettingsClick() },
                    ),
                    SettingsItem(
                        icon = Icons.Default.Palette,
                        title = "主题",
                        onClick = { onSettingsClick() },
                    ),
                    SettingsItem(
                        icon = Icons.Default.Storage,
                        title = "媒体库",
                        onClick = { onSettingsClick() },
                    ),
                    SettingsItem(
                        icon = Icons.Default.Sync,
                        title = "服务器管理",
                        onClick = { onServerManageClick() },
                    ),
                ),
            )
        }

        // 播放器设置组
        item {
            SettingsGroup(
                title = "播放器",
                items = listOf(
                    SettingsItem(
                        icon = Icons.Default.PlayArrow,
                        title = "播放器设置",
                        onClick = { onPlayerSettingsClick() },
                    ),
                ),
            )
        }

        // 关于设置组
        item {
            SettingsGroup(
                title = "关于",
                items = listOf(
                    SettingsItem(
                        icon = Icons.Default.Info,
                        title = "关于",
                        onClick = { /* TODO: 关于页面 */ },
                    ),
                ),
            )
        }
    }
}

@Composable
private fun BrandCard(
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(MobileRadius.xxl))
            .background(
                brush = Brush.linearGradient(
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
        }
    }
}

@Composable
private fun SettingsGroup(
    title: String,
    items: List<SettingsItem>,
    modifier: Modifier = Modifier,
) {
    Column(
        modifier = modifier
            .fillMaxWidth()
            .padding(
                start = MobileSpacing.xl,
                end = MobileSpacing.xl,
                top = MobileSpacing.xl,
            ),
    ) {
        Text(
            text = title,
            color = MobileColors.Primary,
            fontSize = MobileFontSize.sm,
            fontWeight = FontWeight.SemiBold,
            modifier = Modifier.padding(bottom = MobileSpacing.sm),
        )

        Column(
            modifier = Modifier
                .fillMaxWidth()
                .clip(RoundedCornerShape(MobileRadius.lg))
                .background(MobileColors.Card)
                .border(
                    width = 1.dp,
                    color = MobileColors.CardBorder,
                    shape = RoundedCornerShape(MobileRadius.lg),
                ),
        ) {
            items.forEach { item ->
                SettingsItemRow(item = item)
            }
        }
    }
}

@Composable
private fun SettingsItemRow(
    item: SettingsItem,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clickable(onClick = item.onClick)
            .padding(
                horizontal = MobileSpacing.lg,
                vertical = MobileSpacing.md,
            ),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(MobileSpacing.md),
    ) {
        // 图标
        Box(
            modifier = Modifier
                .size(40.dp)
                .clip(RoundedCornerShape(MobileRadius.sm))
                .background(MobileColors.PrimarySoft),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = item.icon,
                contentDescription = null,
                tint = MobileColors.Primary,
                modifier = Modifier.size(22.dp),
            )
        }

        // 标题和副标题
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(
                text = item.title,
                color = MobileColors.Text,
                fontSize = MobileFontSize.md,
                fontWeight = FontWeight.Medium,
            )
            if (item.subtitle != null) {
                Text(
                    text = item.subtitle,
                    color = MobileColors.Muted,
                    fontSize = MobileFontSize.sm,
                )
            }
        }

        // 箭头
        Icon(
            imageVector = Icons.AutoMirrored.Filled.ArrowForward,
            contentDescription = null,
            tint = MobileColors.Muted,
            modifier = Modifier.size(20.dp),
        )
    }
}

private data class SettingsItem(
    val icon: ImageVector,
    val title: String,
    val subtitle: String? = null,
    val onClick: () -> Unit,
)
