package com.nowen.video.ui.component.mobile

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 分段式 Tab 切换器
 * Hills Pro 风格：胶囊按钮 + 选中态
 */
@Composable
fun SegmentedTabs(
    tabs: List<Pair<String, String>>,
    selectedTab: String,
    onTabSelected: (String) -> Unit,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(MobileRadius.full))
            .background(MobileColors.BgAlt)
            .padding(4.dp),
        horizontalArrangement = Arrangement.spacedBy(4.dp),
    ) {
        tabs.forEach { (key, label) ->
            val isSelected = key == selectedTab
            val backgroundColor by animateColorAsState(
                targetValue = if (isSelected) MobileColors.Card else MobileColors.BgAlt,
                animationSpec = spring(),
                label = "backgroundColor",
            )
            val textColor by animateColorAsState(
                targetValue = if (isSelected) MobileColors.Text else MobileColors.Muted,
                animationSpec = spring(),
                label = "textColor",
            )

            Box(
                modifier = Modifier
                    .weight(1f)
                    .height(40.dp)
                    .clip(RoundedCornerShape(MobileRadius.full))
                    .background(backgroundColor)
                    .clickable(
                        interactionSource = remember { MutableInteractionSource() },
                        indication = null,
                        onClick = { onTabSelected(key) },
                    ),
                contentAlignment = Alignment.Center,
            ) {
                Text(
                    text = label,
                    color = textColor,
                    fontSize = MobileFontSize.sm,
                    fontWeight = if (isSelected) FontWeight.SemiBold else FontWeight.Normal,
                )
            }
        }
    }
}
