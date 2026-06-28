package com.nowen.video.ui.component.mobile

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.PlayArrow
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 服务器图标类型
 */
enum class ServerIconType {
    Nowen,
    Emby,
    Jellyfin,
    Unknown,
}

/**
 * 紧凑服务器入口卡片
 * Hills Pro 风格：小而明确的媒体服务器入口
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun MobileServerEntryCard(
    name: String,
    subtitle: String,
    iconType: ServerIconType = ServerIconType.Nowen,
    isActive: Boolean = false,
    onClick: () -> Unit,
    onLongClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val isPressed by interactionSource.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (isPressed) 0.97f else 1f,
        animationSpec = spring(),
        label = "scale",
    )

    val borderColor = if (isActive) {
        MobileColors.Primary.copy(alpha = 0.3f)
    } else {
        MobileColors.CardBorder
    }

    Box(
        modifier = modifier
            .fillMaxWidth()
            .scale(scale)
            .clip(RoundedCornerShape(MobileRadius.xl))
            .background(MobileColors.Card)
            .border(
                width = if (isActive) 2.dp else 1.dp,
                color = borderColor,
                shape = RoundedCornerShape(MobileRadius.xl),
            )
            .combinedClickable(
                interactionSource = interactionSource,
                indication = null,
                onClick = onClick,
                onLongClick = onLongClick,
            )
            .padding(MobileSpacing.md),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(MobileSpacing.sm),
        ) {
            // 服务器图标
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(RoundedCornerShape(MobileRadius.md))
                    .background(getServerIconBackground(iconType)),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    imageVector = getServerIcon(iconType),
                    contentDescription = null,
                    tint = getServerIconTint(iconType),
                    modifier = Modifier.size(28.dp),
                )
            }

            // 服务器信息
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(2.dp),
            ) {
                Text(
                    text = name,
                    color = MobileColors.Text,
                    fontSize = MobileFontSize.md,
                    fontWeight = FontWeight.SemiBold,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = subtitle,
                    color = MobileColors.Muted,
                    fontSize = MobileFontSize.xs,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }

            // 活跃标识
            if (isActive) {
                Box(
                    modifier = Modifier
                        .size(8.dp)
                        .clip(RoundedCornerShape(4.dp))
                        .background(MobileColors.Success),
                )
            }
        }
    }
}

/**
 * 获取服务器图标
 */
private fun getServerIcon(type: ServerIconType): ImageVector {
    return when (type) {
        ServerIconType.Nowen -> Icons.Default.Dns
        ServerIconType.Emby -> Icons.Default.PlayArrow
        ServerIconType.Jellyfin -> Icons.Default.PlayArrow
        ServerIconType.Unknown -> Icons.Default.Dns
    }
}

/**
 * 获取服务器图标背景色
 */
@Composable
private fun getServerIconBackground(type: ServerIconType) = when (type) {
    ServerIconType.Nowen -> MobileColors.PrimarySoft
    ServerIconType.Emby -> MobileColors.Success.copy(alpha = 0.1f)
    ServerIconType.Jellyfin -> MobileColors.Primary.copy(alpha = 0.1f)
    ServerIconType.Unknown -> MobileColors.BgAlt
}

/**
 * 获取服务器图标颜色
 */
@Composable
private fun getServerIconTint(type: ServerIconType) = when (type) {
    ServerIconType.Nowen -> MobileColors.Primary
    ServerIconType.Emby -> MobileColors.Success
    ServerIconType.Jellyfin -> MobileColors.Primary
    ServerIconType.Unknown -> MobileColors.Muted
}

/**
 * 根据服务器名称推断图标类型
 */
fun inferServerIconType(name: String): ServerIconType {
    return when {
        name.contains("emby", ignoreCase = true) -> ServerIconType.Emby
        name.contains("jellyfin", ignoreCase = true) -> ServerIconType.Jellyfin
        name.contains("nowen", ignoreCase = true) -> ServerIconType.Nowen
        else -> ServerIconType.Nowen
    }
}
