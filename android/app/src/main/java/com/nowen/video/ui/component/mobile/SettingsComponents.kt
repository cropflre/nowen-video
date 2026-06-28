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
 * 设置行类型
 */
sealed class SettingsRow {
    /**
     * 普通操作行
     */
    data class Action(
        val icon: ImageVector,
        val title: String,
        val subtitle: String? = null,
        val status: String? = null,
        val badge: String? = null,
        val enabled: Boolean = true,
        val showArrow: Boolean = true,
        val onClick: () -> Unit,
    ) : SettingsRow()

    /**
     * Switch 行
     */
    data class Switch(
        val icon: ImageVector,
        val title: String,
        val subtitle: String? = null,
        val checked: Boolean,
        val enabled: Boolean = true,
        val onCheckedChange: (Boolean) -> Unit,
    ) : SettingsRow()
}

/**
 * 设置分组
 */
@Composable
fun SettingsGroup(
    title: String,
    rows: List<SettingsRow>,
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
            rows.forEachIndexed { index, row ->
                when (row) {
                    is SettingsRow.Action -> SettingsActionRow(row = row)
                    is SettingsRow.Switch -> SettingsSwitchRow(row = row)
                }
                if (index < rows.lastIndex) {
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
 * 普通操作行
 */
@Composable
private fun SettingsActionRow(
    row: SettingsRow.Action,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clickable(enabled = row.enabled, onClick = row.onClick)
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
                    if (row.enabled) MobileColors.PrimarySoft
                    else MobileColors.BgAlt
                ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = row.icon,
                contentDescription = null,
                tint = if (row.enabled) MobileColors.Primary else MobileColors.Muted,
                modifier = Modifier.size(22.dp),
            )
        }

        // 标题和副标题
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(
                text = row.title,
                color = if (row.enabled) MobileColors.Text else MobileColors.Muted,
                fontSize = MobileFontSize.md,
                fontWeight = FontWeight.Medium,
            )
            if (row.subtitle != null) {
                Text(
                    text = row.subtitle,
                    color = MobileColors.Muted,
                    fontSize = MobileFontSize.sm,
                )
            }
        }

        // 状态或 badge
        if (row.status != null) {
            Text(
                text = row.status,
                color = MobileColors.Muted,
                fontSize = MobileFontSize.sm,
            )
        } else if (row.badge != null) {
            Text(
                text = row.badge,
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
        if (row.showArrow && row.enabled) {
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
 * Switch 行
 */
@Composable
private fun SettingsSwitchRow(
    row: SettingsRow.Switch,
    modifier: Modifier = Modifier,
) {
    Row(
        modifier = modifier
            .fillMaxWidth()
            .clickable(enabled = row.enabled) { row.onCheckedChange(!row.checked) }
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
                    if (row.enabled) MobileColors.PrimarySoft
                    else MobileColors.BgAlt
                ),
            contentAlignment = Alignment.Center,
        ) {
            Icon(
                imageVector = row.icon,
                contentDescription = null,
                tint = if (row.enabled) MobileColors.Primary else MobileColors.Muted,
                modifier = Modifier.size(22.dp),
            )
        }

        // 标题和副标题
        Column(
            modifier = Modifier.weight(1f),
            verticalArrangement = Arrangement.spacedBy(2.dp),
        ) {
            Text(
                text = row.title,
                color = if (row.enabled) MobileColors.Text else MobileColors.Muted,
                fontSize = MobileFontSize.md,
                fontWeight = FontWeight.Medium,
            )
            if (row.subtitle != null) {
                Text(
                    text = row.subtitle,
                    color = MobileColors.Muted,
                    fontSize = MobileFontSize.sm,
                )
            }
        }

        // Switch
        Switch(
            checked = row.checked,
            onCheckedChange = row.onCheckedChange,
            enabled = row.enabled,
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

// ==================== 兼容旧接口 ====================

/**
 * 设置项数据（兼容旧接口）
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
