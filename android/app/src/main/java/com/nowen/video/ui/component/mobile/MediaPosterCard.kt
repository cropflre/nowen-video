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
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material3.CircularProgressIndicator
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
 * 海报 Fallback 组件
 */
@Composable
private fun PosterFallback(
    title: String,
    modifier: Modifier = Modifier,
) {
    Box(
        modifier = modifier
            .fillMaxSize()
            .background(
                Brush.linearGradient(
                    colors = listOf(MobileColors.PrimarySoft, MobileColors.BgAlt),
                ),
            ),
        contentAlignment = Alignment.Center,
    ) {
        Text(
            text = title.take(1),
            color = MobileColors.Primary.copy(alpha = 0.5f),
            fontSize = MobileFontSize.xxxl,
            fontWeight = FontWeight.Bold,
        )
    }
}

/**
 * 媒体海报卡片
 * Hills Pro 风格：大圆角 + 柔和阴影 + 进度条
 *
 * @param title 标题
 * @param year 年份
 * @param imageUrl 图片 URL
 * @param progress 进度，范围 0f..1f
 * @param badges 标签列表
 * @param onClick 点击事件
 */
@Composable
fun MediaPosterCard(
    title: String,
    year: Int? = null,
    imageUrl: String? = null,
    progress: Float? = null,
    badges: List<String> = emptyList(),
    onClick: () -> Unit,
    modifier: Modifier = Modifier,
) {
    // 统一进度范围为 0f..1f
    val normalizedProgress = progress?.coerceIn(0f, 1f)
    val interactionSource = remember { MutableInteractionSource() }
    val isPressed by interactionSource.collectIsPressedAsState()
    val scale by animateFloatAsState(
        targetValue = if (isPressed) 0.97f else 1f,
        animationSpec = spring(),
        label = "scale",
    )

    Column(
        modifier = modifier
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
            ),
    ) {
        // 图片容器
        Box(
            modifier = Modifier
                .fillMaxWidth()
                .aspectRatio(2f / 3f)
                .clip(RoundedCornerShape(topStart = MobileRadius.lg, topEnd = MobileRadius.lg)),
        ) {
            if (imageUrl != null) {
                SubcomposeAsyncImage(
                    model = imageUrl,
                    contentDescription = title,
                    contentScale = ContentScale.Crop,
                    modifier = Modifier.fillMaxSize(),
                    loading = {
                        // 加载中
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .background(MobileColors.BgAlt),
                            contentAlignment = Alignment.Center,
                        ) {
                            CircularProgressIndicator(
                                modifier = Modifier.size(24.dp),
                                color = MobileColors.Primary,
                            )
                        }
                    },
                    error = {
                        // 加载失败
                        PosterFallback(title = title)
                    },
                )
            } else {
                // 无图片
                PosterFallback(title = title)
            }

            // 进度条
            if (normalizedProgress != null && normalizedProgress > 0) {
                Box(
                    modifier = Modifier
                        .align(Alignment.BottomCenter)
                        .fillMaxWidth()
                        .padding(horizontal = MobileSpacing.sm, vertical = MobileSpacing.xs),
                ) {
                    Box(
                        modifier = Modifier
                            .fillMaxWidth()
                            .padding(top = MobileSpacing.xs),
                    ) {
                        Box(
                            modifier = Modifier
                                .fillMaxWidth()
                                .padding(top = 4.dp)
                                .background(Color.Black.copy(alpha = 0.3f))
                                .padding(2.dp),
                        ) {
                            Box(
                                modifier = Modifier
                                    .fillMaxWidth(normalizedProgress)
                                    .background(MobileColors.Primary),
                            )
                        }
                    }
                }
            }

            // Badges
            if (badges.isNotEmpty()) {
                Column(
                    modifier = Modifier
                        .align(Alignment.TopStart)
                        .padding(MobileSpacing.sm),
                    verticalArrangement = Arrangement.spacedBy(4.dp),
                ) {
                    badges.forEach { badge ->
                        Text(
                            text = badge,
                            color = Color.White,
                            fontSize = MobileFontSize.xs,
                            modifier = Modifier
                                .background(
                                    Color.Black.copy(alpha = 0.6f),
                                    RoundedCornerShape(MobileRadius.xs),
                                )
                                .padding(horizontal = 6.dp, vertical = 2.dp),
                        )
                    }
                }
            }
        }

        // 标题区域
        Column(
            modifier = Modifier.padding(MobileSpacing.sm),
        ) {
            Text(
                text = title,
                color = MobileColors.Text,
                fontSize = MobileFontSize.md,
                fontWeight = FontWeight.Medium,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            if (year != null) {
                Text(
                    text = year.toString(),
                    color = MobileColors.Muted,
                    fontSize = MobileFontSize.sm,
                )
            }
        }
    }
}
