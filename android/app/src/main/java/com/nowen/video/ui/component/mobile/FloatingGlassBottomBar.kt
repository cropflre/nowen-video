package com.nowen.video.ui.component.mobile

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.Spring
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.shadow
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 底部导航项
 */
data class BottomNavItem(
    val key: String,
    val label: String,
    val icon: ImageVector,
    val selectedIcon: ImageVector? = null,
)

/**
 * 悬浮毛玻璃底部导航栏
 * Hills Pro 风格：半透明背景 + 大圆角 + 胶囊高亮
 */
@Composable
fun FloatingGlassBottomBar(
    items: List<BottomNavItem>,
    selectedKey: String,
    onItemClick: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .padding(horizontal = MobileSpacing.xl)
            .shadow(
                elevation = 16.dp,
                shape = RoundedCornerShape(MobileRadius.full),
                ambientColor = Color(0x241F2346),
                spotColor = Color(0x241F2346),
            )
            .clip(RoundedCornerShape(MobileRadius.full))
            .background(MobileColors.Glass)
            .border(
                width = 1.dp,
                color = MobileColors.GlassBorder,
                shape = RoundedCornerShape(MobileRadius.full),
            ),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .height(72.dp)
                .padding(horizontal = MobileSpacing.xs),
            horizontalArrangement = Arrangement.SpaceEvenly,
            verticalAlignment = Alignment.CenterVertically,
        ) {
            items.forEach { item ->
                val isSelected = item.key == selectedKey
                BottomNavButton(
                    item = item,
                    isSelected = isSelected,
                    onClick = { onItemClick(item.key) },
                    modifier = Modifier.weight(1f),
                )
            }
        }
    }
}

@Composable
private fun BottomNavButton(
    item: BottomNavItem,
    isSelected: Boolean,
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val backgroundColor by animateColorAsState(
        targetValue = if (isSelected) MobileColors.Active else Color.Transparent,
        animationSpec = spring(stiffness = Spring.StiffnessLow),
        label = "backgroundColor",
    )

    val iconColor by animateColorAsState(
        targetValue = if (isSelected) MobileColors.ActiveText else MobileColors.Muted,
        animationSpec = spring(stiffness = Spring.StiffnessLow),
        label = "iconColor",
    )

    val textColor by animateColorAsState(
        targetValue = if (isSelected) MobileColors.ActiveText else MobileColors.Muted,
        animationSpec = spring(stiffness = Spring.StiffnessLow),
        label = "textColor",
    )

    Box(
        modifier = modifier
            .clip(RoundedCornerShape(MobileRadius.full))
            .background(backgroundColor)
            .clickable(
                interactionSource = remember { MutableInteractionSource() },
                indication = null,
                onClick = onClick,
            )
            .padding(horizontal = MobileSpacing.md, vertical = MobileSpacing.sm),
        contentAlignment = Alignment.Center,
    ) {
        Column(
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Icon(
                imageVector = if (isSelected && item.selectedIcon != null) item.selectedIcon else item.icon,
                contentDescription = item.label,
                tint = iconColor,
                modifier = Modifier.size(22.dp),
            )
            Text(
                text = item.label,
                color = textColor,
                fontSize = MobileFontSize.xs,
                fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Normal,
            )
        }
    }
}
