package com.nowen.video.ui.theme

import androidx.compose.animation.core.*
import androidx.compose.foundation.background
import androidx.compose.foundation.border
import androidx.compose.foundation.layout.*
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material3.MaterialTheme
import androidx.compose.runtime.*
import androidx.compose.ui.Modifier
import androidx.compose.ui.composed
import androidx.compose.ui.draw.clip
import androidx.compose.ui.draw.drawBehind
import androidx.compose.ui.draw.drawWithContent
import androidx.compose.ui.draw.scale
import androidx.compose.ui.geometry.CornerRadius
import androidx.compose.ui.geometry.Offset
import androidx.compose.ui.geometry.Size
import androidx.compose.ui.graphics.*
import androidx.compose.ui.graphics.drawscope.Stroke
import androidx.compose.ui.unit.Dp
import androidx.compose.ui.unit.dp

// ==================== 赛博朋克修饰符扩展 ====================

/**
 * 玻璃拟态背景 — 使用 MaterialTheme 颜色，跟随主题模式切换
 * 增强：更精致的渐变和边框效果
 */
fun Modifier.glassMorphism(
    cornerRadius: Dp = 16.dp,
    borderAlpha: Float = 0.12f
): Modifier = composed {
    val surfaceHigh = MaterialTheme.colorScheme.surfaceContainerHigh
    val surface = MaterialTheme.colorScheme.surfaceContainer
    val primary = MaterialTheme.colorScheme.primary
    val secondary = MaterialTheme.colorScheme.secondary

    this
        .clip(RoundedCornerShape(cornerRadius))
        .background(
            brush = Brush.verticalGradient(
                colors = listOf(
                    surfaceHigh.copy(alpha = 0.88f),
                    surface.copy(alpha = 0.78f)
                )
            ),
            shape = RoundedCornerShape(cornerRadius)
        )
        .border(
            width = 0.5.dp,
            brush = Brush.linearGradient(
                colors = listOf(
                    primary.copy(alpha = borderAlpha),
                    secondary.copy(alpha = borderAlpha * 0.4f),
                    Color.Transparent
                )
            ),
            shape = RoundedCornerShape(cornerRadius)
        )
}

/**
 * 霓虹光晕边框 — 带呼吸动画的发光边框
 */
fun Modifier.neonGlow(
    color: Color = CyberBlue,
    cornerRadius: Dp = 12.dp,
    glowRadius: Float = 8f
): Modifier = composed {
    val infiniteTransition = rememberInfiniteTransition(label = "neon_glow")
    val alpha by infiniteTransition.animateFloat(
        initialValue = 0.3f,
        targetValue = 0.7f,
        animationSpec = infiniteRepeatable(
            animation = tween(2500, easing = EaseInOutCubic),
            repeatMode = RepeatMode.Reverse
        ),
        label = "neon_alpha"
    )

    this.drawBehind {
        drawRoundRect(
            color = color.copy(alpha = alpha * 0.25f),
            cornerRadius = CornerRadius(cornerRadius.toPx()),
            style = Stroke(width = glowRadius),
            size = Size(size.width + glowRadius, size.height + glowRadius),
            topLeft = Offset(-glowRadius / 2, -glowRadius / 2)
        )
    }
}

/**
 * 赛博朋克卡片背景 — 使用 MaterialTheme 颜色，跟随主题模式切换
 * 增强：更细腻的光晕效果
 */
fun Modifier.cyberCard(
    cornerRadius: Dp = 16.dp,
    glowColor: Color = CyberBlue
): Modifier = composed {
    val surface = MaterialTheme.colorScheme.surfaceContainer

    this
        .clip(RoundedCornerShape(cornerRadius))
        .background(
            brush = Brush.verticalGradient(
                colors = listOf(
                    glowColor.copy(alpha = 0.05f),
                    surface.copy(alpha = 0.95f),
                    surface
                )
            ),
            shape = RoundedCornerShape(cornerRadius)
        )
        .border(
            width = 0.5.dp,
            brush = Brush.verticalGradient(
                colors = listOf(
                    glowColor.copy(alpha = 0.18f),
                    glowColor.copy(alpha = 0.04f),
                    Color.Transparent
                )
            ),
            shape = RoundedCornerShape(cornerRadius)
        )
}

/**
 * 深空渐变背景 — 使用 MaterialTheme 颜色，跟随主题模式切换
 */
fun Modifier.spaceBackground(): Modifier = composed {
    val bg = MaterialTheme.colorScheme.background
    val surfaceDim = MaterialTheme.colorScheme.surfaceDim
    val surface = MaterialTheme.colorScheme.surfaceContainer

    this.background(
        brush = Brush.verticalGradient(
            colors = listOf(
                surfaceDim,
                bg,
                surface.copy(alpha = 0.5f)
            )
        )
    )
}

/**
 * 扫描线效果 — 淡淡的水平扫描线叠加
 */
fun Modifier.scanLineOverlay(): Modifier = this.drawWithContent {
    drawContent()
    val lineSpacing = 4.dp.toPx()
    var y = 0f
    while (y < size.height) {
        drawLine(
            color = Color.White.copy(alpha = 0.015f),
            start = Offset(0f, y),
            end = Offset(size.width, y),
            strokeWidth = 1f
        )
        y += lineSpacing
    }
}

/**
 * 渐变文字遮罩效果 — 用于卡片底部文字区域，跟随主题模式
 */
fun Modifier.gradientScrim(
    color: Color = Color.Unspecified,
    startAlpha: Float = 0f,
    endAlpha: Float = 0.85f
): Modifier = composed {
    val scrimColor = if (color == Color.Unspecified) MaterialTheme.colorScheme.surfaceDim else color
    this.drawBehind {
        drawRect(
            brush = Brush.verticalGradient(
                colors = listOf(
                    scrimColor.copy(alpha = startAlpha),
                    scrimColor.copy(alpha = endAlpha)
                )
            )
        )
    }
}

/**
 * 骨架屏闪烁效果 — 用于加载占位
 */
fun Modifier.shimmerEffect(
    cornerRadius: Dp = 8.dp
): Modifier = composed {
    val shimmerColors = listOf(
        MaterialTheme.colorScheme.surfaceContainerHigh.copy(alpha = 0.6f),
        MaterialTheme.colorScheme.surfaceContainerHigh.copy(alpha = 0.2f),
        MaterialTheme.colorScheme.surfaceContainerHigh.copy(alpha = 0.6f)
    )

    val transition = rememberInfiniteTransition(label = "shimmer")
    val translateAnim by transition.animateFloat(
        initialValue = 0f,
        targetValue = 1000f,
        animationSpec = infiniteRepeatable(
            animation = tween(1200, easing = LinearEasing),
            repeatMode = RepeatMode.Restart
        ),
        label = "shimmer_translate"
    )

    this
        .clip(RoundedCornerShape(cornerRadius))
        .background(
            brush = Brush.linearGradient(
                colors = shimmerColors,
                start = Offset(translateAnim - 200f, 0f),
                end = Offset(translateAnim + 200f, 0f)
            )
        )
}

/**
 * 脉冲缩放动画 — 用于强调元素
 */
fun Modifier.pulseAnimation(
    minScale: Float = 0.97f,
    maxScale: Float = 1.03f,
    durationMs: Int = 2000
): Modifier = composed {
    val infiniteTransition = rememberInfiniteTransition(label = "pulse")
    val scale by infiniteTransition.animateFloat(
        initialValue = minScale,
        targetValue = maxScale,
        animationSpec = infiniteRepeatable(
            animation = tween(durationMs, easing = EaseInOutCubic),
            repeatMode = RepeatMode.Reverse
        ),
        label = "pulse_scale"
    )
    this.scale(scale)
}

/**
 * 按压态缩放反馈 — 用于可点击元素
 */
fun Modifier.pressScale(
    pressed: Boolean,
    pressedScale: Float = 0.96f
): Modifier = composed {
    val scale by animateFloatAsState(
        targetValue = if (pressed) pressedScale else 1f,
        animationSpec = spring(
            dampingRatio = Spring.DampingRatioMediumBouncy,
            stiffness = Spring.StiffnessLow
        ),
        label = "press_scale"
    )
    this.scale(scale)
}

// ==================== 赛博朋克形状常量 ====================

val CyberCardShape = RoundedCornerShape(16.dp)
val CyberChipShape = RoundedCornerShape(20.dp)
val CyberButtonShape = RoundedCornerShape(12.dp)
val CyberDialogShape = RoundedCornerShape(20.dp)
val CyberBottomSheetShape = RoundedCornerShape(topStart = 20.dp, topEnd = 20.dp)

// ==================== 间距常量 ====================

object CyberSpacing {
    val xs = 4.dp
    val sm = 8.dp
    val md = 12.dp
    val lg = 16.dp
    val xl = 20.dp
    val xxl = 24.dp
    val xxxl = 32.dp
}

// ==================== 动画时长常量 ====================

object CyberAnim {
    const val FAST = 150
    const val NORMAL = 250
    const val SLOW = 400
    const val ENTER = 350
    const val EXIT = 250
}
