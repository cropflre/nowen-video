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
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.CheckCircle
import androidx.compose.material.icons.filled.Dns
import androidx.compose.material.icons.filled.Error
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.scale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 服务器卡片组件
 * Hills Pro 风格：半透明背景 + 大圆角 + 细边框
 *
 * @param serverName 服务器名称
 * @param serverUrl 服务器地址
 * @param isConnected 连接状态：null=中性/未检查, true=已连接, false=连接失败
 * @param statusText 自定义状态文本，优先于 isConnected 显示
 */
@OptIn(ExperimentalFoundationApi::class)
@Composable
fun ServerCard(
    serverName: String,
    serverUrl: String,
    isConnected: Boolean? = null,
    statusText: String? = null,
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

    Box(
        modifier = modifier
            .fillMaxWidth()
            .scale(scale)
            .clip(RoundedCornerShape(MobileRadius.xl))
            .background(MobileColors.Card)
            .border(
                width = 1.dp,
                color = MobileColors.CardBorder,
                shape = RoundedCornerShape(MobileRadius.xl),
            )
            .combinedClickable(
                interactionSource = interactionSource,
                indication = null,
                onClick = onClick,
                onLongClick = onLongClick,
            )
            .padding(MobileSpacing.lg),
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(MobileSpacing.md),
        ) {
            // 服务器图标
            Box(
                modifier = Modifier
                    .size(48.dp)
                    .clip(RoundedCornerShape(MobileRadius.md))
                    .background(MobileColors.PrimarySoft),
                contentAlignment = Alignment.Center,
            ) {
                Icon(
                    imageVector = Icons.Default.Dns,
                    contentDescription = null,
                    tint = MobileColors.Primary,
                    modifier = Modifier.size(28.dp),
                )
            }

            // 服务器信息
            Column(
                modifier = Modifier.weight(1f),
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                Text(
                    text = serverName,
                    color = MobileColors.Text,
                    fontSize = MobileFontSize.lg,
                    fontWeight = FontWeight.SemiBold,
                )
                Text(
                    text = serverUrl,
                    color = MobileColors.Muted,
                    fontSize = MobileFontSize.sm,
                    maxLines = 1,
                )
                // 状态文本
                if (statusText != null) {
                    Text(
                        text = statusText,
                        color = MobileColors.Muted,
                        fontSize = MobileFontSize.xs,
                    )
                }
            }

            // 连接状态图标
            when (isConnected) {
                true -> Icon(
                    imageVector = Icons.Default.CheckCircle,
                    contentDescription = "已连接",
                    tint = MobileColors.Success,
                    modifier = Modifier.size(24.dp),
                )
                false -> Icon(
                    imageVector = Icons.Default.Error,
                    contentDescription = "连接失败",
                    tint = MobileColors.Error,
                    modifier = Modifier.size(24.dp),
                )
                null -> {
                    // 中性状态，不显示图标
                }
            }
        }
    }
}
