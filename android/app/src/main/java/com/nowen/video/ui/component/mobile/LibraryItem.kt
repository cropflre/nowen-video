package com.nowen.video.ui.component.mobile

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.FolderOpen
import androidx.compose.material.icons.automirrored.filled.ArrowForward
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
import com.nowen.video.data.model.Library
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 媒体库列表项
 * Hills Pro 风格：大圆角 + 卡片背景
 */
@Composable
fun LibraryItem(
    library: Library,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val interactionSource = remember { MutableInteractionSource() }
    val isPressed by interactionSource.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (isPressed) 0.97f else 1f,
        animationSpec = spring(),
        label = "scale",
    )

    Row(
        modifier = modifier
            .fillMaxWidth()
            .scale(scale)
            .clip(RoundedCornerShape(MobileRadius.lg))
            .background(MobileColors.Card)
            .border(
                width = 1.dp,
                color = MobileColors.CardBorder,
                shape = RoundedCornerShape(MobileRadius.lg),
            )
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                onClick = onClick,
            )
            .padding(MobileSpacing.lg),
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
                imageVector = Icons.Default.FolderOpen,
                contentDescription = null,
                tint = MobileColors.Primary,
                modifier = Modifier.size(22.dp),
            )
        }

        // 名称
        Text(
            text = library.name,
            color = MobileColors.Text,
            fontSize = MobileFontSize.md,
            fontWeight = FontWeight.Medium,
            modifier = Modifier.weight(1f),
        )

        // 箭头
        Icon(
            imageVector = Icons.AutoMirrored.Filled.ArrowForward,
            contentDescription = null,
            tint = MobileColors.Muted,
            modifier = Modifier.size(20.dp),
        )
    }
}
