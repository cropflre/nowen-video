package com.nowen.video.ui.component.mobile

import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.statusBarsPadding
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.ArrowBack
import androidx.compose.material3.Icon
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 移动端页面标题
 * Hills Pro 风格：大字号 + 左对齐 + 右侧操作按钮
 */
@Composable
fun MobilePageHeader(
    title: String,
    onBack: (() -> Unit)? = null,
    actions: List<PageHeaderAction> = emptyList(),
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .statusBarsPadding()
            .padding(
                top = MobileSpacing.xl,
                start = MobileSpacing.xl,
                end = MobileSpacing.xl,
                bottom = MobileSpacing.xl,
            ),
        horizontalArrangement = Arrangement.SpaceBetween,
        verticalAlignment = Alignment.CenterVertically,
    ) {
        Row(
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.spacedBy(MobileSpacing.sm),
        ) {
            if (onBack != null) {
                Icon(
                    imageVector = Icons.Default.ArrowBack,
                    contentDescription = "返回",
                    tint = MobileColors.Text,
                    modifier = Modifier
                        .size(24.dp)
                        .clickable(
                            interactionSource = remember { MutableInteractionSource() },
                            indication = null,
                            onClick = onBack,
                        ),
                )
            }
            Text(
                text = title,
                color = MobileColors.Text,
                fontSize = MobileFontSize.xxxl,
                fontWeight = FontWeight.SemiBold,
            )
        }

        if (actions.isNotEmpty()) {
            Row(
                horizontalArrangement = Arrangement.spacedBy(MobileSpacing.sm),
            ) {
                actions.forEach { action ->
                    Icon(
                        imageVector = action.icon,
                        contentDescription = action.contentDescription,
                        tint = MobileColors.Muted,
                        modifier = Modifier
                            .size(24.dp)
                            .clickable(
                                interactionSource = remember { MutableInteractionSource() },
                                indication = null,
                                onClick = action.onClick,
                            ),
                    )
                }
            }
        }
    }
}

/**
 * 页面头部操作按钮
 */
data class PageHeaderAction(
    val icon: ImageVector,
    val contentDescription: String? = null,
    val onClick: () -> Unit,
)
