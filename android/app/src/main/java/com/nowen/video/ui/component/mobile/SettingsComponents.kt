package com.nowen.video.ui.component.mobile

import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.clickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowForward
import androidx.compose.material3.Icon
import androidx.compose.material3.Switch
import androidx.compose.material3.SwitchDefaults
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.dp
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * 设置分组
 */
@Composable
fun SettingsGroup(
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
            items.forEachIndexed { index, item ->
                SettingsItemRow(item = item)
                if (index < items.lastIndex) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(horizontal = MobileSpacing.lg)
                            .background(MobileColors.BgAlt)
                            .padding(vertical = 0.5.dp)
                    )
                }
            }
        }
    }
}

/**
 * 设置项数据
 */
data class SettingsItem(
    val icon: ImageVector,
    val title: String,
    val subtitle: String? = null,
    val status: String? = null,
    val badge: String? = null,
    val enabled: Boolean = true,
    val showArrow: Boolean = true,
    val onClick: () -> Unit,
)

/**
 * 设置项行
 */
@Composable
fun SettingsItemRow(
    item: SettingsItem,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clickable(enabled = item.enabled, onClick = item.onClick)
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
                .background(
                    if (item.enabled) MobileColors.PrimarySoft
                    else MobileColors.BgAlt
                ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = item.icon,
                contentDescription = null,
                tint = if (item.enabled) MobileColors.Primary else MobileColors.Muted,
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
                color = if (item.enabled) MobileColors.Text else MobileColors.Muted,
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

        // 状态或 badge
        if (item.status != null) {
            Text(
                text = item.status,
                color = MobileColors.Muted,
                fontSize = MobileFontSize.sm,
            )
        } else if (item.badge != null) {
            Text(
                text = item.badge,
                color = MobileColors.Primary,
                fontSize = MobileFontSize.xs,
                modifier = Modifier
                    .background(
                        MobileColors.PrimarySoft,
                        RoundedCornerShape(MobileRadius.xs)
                    )
                    .padding(horizontal = 6.dp, vertical = 2.dp),
            )
        }

        // 箭头
        if (item.showArrow && item.enabled) {
            Icon(
                imageVector = Icons.AutoMirrored.Filled.ArrowForward,
                contentDescription = null,
                tint = MobileColors.Muted,
                modifier = Modifier.size(20.dp),
            )
        }
    }
}

/**
 * 设置项行（带 Switch）
 */
@Composable
fun SettingsSwitchRow(
    icon: ImageVector,
    title: String,
    subtitle: String? = null,
    checked: Boolean,
    onCheckedChange: (Boolean) -> Unit,
    enabled: Boolean = true,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clickable(enabled = enabled) { onCheckedChange(!checked) }
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
                .background(
                    if (enabled) MobileColors.PrimarySoft
                    else MobileColors.BgAlt
                ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = icon,
                contentDescription = null,
                tint = if (enabled) MobileColors.Primary else MobileColors.Muted,
                modifier = Modifier.size(22.dp),
            )
        }

        // 标题和副标题
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(
                text = title,
                color = if (enabled) MobileColors.Text else MobileColors.Muted,
                fontSize = MobileFontSize.md,
                fontWeight = FontWeight.Medium,
            )
            if (subtitle != null) {
                Text(
                    text = subtitle,
                    color = MobileColors.Muted,
                    fontSize = MobileFontSize.sm,
                )
            }
        }

        // Switch
        Switch(
            checked = checked,
            onCheckedChange = onCheckedChange,
            enabled = enabled,
            colors = SwitchDefaults.colors(
                checkedTrackColor = MobileColors.Primary,
                checkedThumbColor = Color.White,
                uncheckedTrackColor = MobileColors.Muted.copy(alpha = 0.3f),
                uncheckedThumbColor = Color.White,
            ),
        )
    }
}

/**
 * 选择弹窗选项
 */
data class SelectionOption(
    val value: String,
    val label: String,
)
