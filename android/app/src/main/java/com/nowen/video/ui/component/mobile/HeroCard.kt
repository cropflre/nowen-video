package com.nowen.video.ui.component.mobile

import androidx.compose.animation.core.animateFloatAsState
import androidx.compose.animation.core.spring
import androidx.compose.foundation.background
import androidx.compose.foundation.clickable
import androidx.compose.foundation.interaction.MutableInteractionSource
import androidx.compose.foundation.interaction.collectIsPressedAsState
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
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
import androidx.compose.ui.draw.scale
import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.SubcomposeAsyncImage
import com.nowen.video.ui.theme.MobileColors
import com.nowen.video.ui.theme.MobileFontSize
import com.nowen.video.ui.theme.MobileRadius
import com.nowen.video.ui.theme.MobileSpacing

/**
 * Hero Fallback 组件
 */
@Composable
private fun HeroFallback(
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxWidth()
            .background(
                Brush.linearGradient(
                    colors = listOf(
                        MobileColors.Primary.copy(alpha = 0.8f),
                        MobileColors.PrimarySoft,
                    ),
                ),
            ),
    )
}

/**
 * Hero 大卡片
 * Hills Pro 风格：顶部大圆角卡片 + 底部渐变遮罩 + 标题
 */
@Composable
fun HeroCard(
    title: String,
    subtitle: String? = null,
    imageUrl: String? = null,
    year: Int? = null,
    rating: Double? = null,
    resolution: String? = null,
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

    Box(
        modifier = modifier
            .fillMaxWidth()
            .height(220.dp)
            .scale(scale)
            .clip(RoundedCornerShape(MobileRadius.xl))
            .clickable(
                interactionSource = interactionSource,
                indication = null,
                onClick = onClick,
            ),
    ) {
        // 背景图片
        if (imageUrl != null) {
            SubcomposeAsyncImage(
                model = imageUrl,
                contentDescription = title,
                contentScale = ContentScale.Crop,
                modifier = Modifier.fillMaxWidth(),
                loading = {
                    // 加载中
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .background(MobileColors.BgAlt),
                    )
                },
                error = {
                    // 加载失败
                    HeroFallback()
                },
            )
        } else {
            // Fallback 背景
            HeroFallback()
        }

        // 底部渐变遮罩
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .align(Alignment.BottomCenter)
                .background(
                    Brush.verticalGradient(
                        colors = listOf(
                            Color.Transparent,
                            Color.Black.copy(alpha = 0.7f),
                        ),
                    ),
                )
                .padding(
                    start = MobileSpacing.lg,
                    end = MobileSpacing.lg,
                    bottom = MobileSpacing.lg,
                    top = MobileSpacing.xxxl,
                ),
        ) {
            Column(
                verticalArrangement = Arrangement.spacedBy(4.dp),
            ) {
                // 标题
                Text(
                    text = title,
                    color = Color.White,
                    fontSize = MobileFontSize.xl,
                    fontWeight = FontWeight.Bold,
                    maxLines = 2,
                    overflow = TextOverflow.Ellipsis,
                )

                // 副标题
                if (subtitle != null) {
                    Text(
                        text = subtitle,
                        color = Color.White.copy(alpha = 0.8f),
                        fontSize = MobileFontSize.sm,
                        maxLines = 2,
                        overflow = TextOverflow.Ellipsis,
                    )
                }

                // 元信息行
                Row(
                    horizontalArrangement = Arrangement.spacedBy(MobileSpacing.sm),
                    verticalAlignment = Alignment.CenterVertically,
                ) {
                    if (year != null) {
                        Text(
                            text = year.toString(),
                            color = Color.White.copy(alpha = 0.7f),
                            fontSize = MobileFontSize.xs,
                        )
                    }
                    if (rating != null && rating > 0) {
                        Text(
                            text = "⭐ ${"%.1f".format(rating)}",
                            color = Color.White.copy(alpha = 0.7f),
                            fontSize = MobileFontSize.xs,
                        )
                    }
                    if (resolution != null) {
                        Text(
                            text = resolution,
                            color = Color.White.copy(alpha = 0.7f),
                            fontSize = MobileFontSize.xs,
                        )
                    }
                }
            }
        }
    }
}
