package com.nowen.video.ui.theme

import android.os.Build
import androidx.compose.foundation.isSystemInDarkTheme
import androidx.compose.material3.*
import androidx.compose.runtime.Composable
import androidx.compose.runtime.SideEffect
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.platform.LocalView
import androidx.core.view.WindowCompat
import com.nowen.video.data.local.ThemeMode

/**
 * 赛博朋克暗色主题色彩方案
 * 深空背景 + 霓虹强调色 + 高对比度前景
 * 优化：更精致的层级对比度和色彩搭配
 */
private val CyberDarkColorScheme = darkColorScheme(
    // 主色调 — 赛博蓝
    primary = CyberBlue,
    onPrimary = SpaceBlack,
    primaryContainer = CyberBlueDim,
    onPrimaryContainer = CyberBlueLight,

    // 次色调 — 霓虹紫
    secondary = NeonPurple,
    onSecondary = SpaceBlack,
    secondaryContainer = NeonPurpleDim,
    onSecondaryContainer = NeonPurpleLight,

    // 第三色 — 电子绿
    tertiary = ElectricGreen,
    onTertiary = SpaceBlack,
    tertiaryContainer = ElectricGreenDim,
    onTertiaryContainer = ElectricGreen,

    // 错误色 — 霓虹粉
    error = NeonPink,
    onError = SpaceBlack,
    errorContainer = NeonPinkDim,
    onErrorContainer = NeonPinkLight,

    // 背景
    background = SpaceDarkBlue,
    onBackground = SpaceOnBg,

    // Surface 层级
    surface = SpaceSurface,
    onSurface = SpaceOnSurface,
    surfaceVariant = SpaceSurfaceVariant,
    onSurfaceVariant = SpaceOnSurfaceDim,
    surfaceTint = CyberBlue,

    // 轮廓
    outline = SpaceBorder,
    outlineVariant = SpaceSurfaceVariant,

    // 反色 Surface
    inverseSurface = SpaceOnBg,
    inverseOnSurface = SpaceDarkBlue,
    inversePrimary = CyberBlueDark,

    // Surface 容器层级
    surfaceContainerHighest = SpaceSurfaceVariant,
    surfaceContainerHigh = SpaceSurfaceHigh,
    surfaceContainer = SpaceSurface,
    surfaceContainerLow = SpaceDarkBlue,
    surfaceContainerLowest = SpaceBlack,
    surfaceBright = SpaceSurfaceVariant,
    surfaceDim = SpaceBlack,

    // Scrim
    scrim = SpaceBlack,
)

/**
 * 赛博朋克浅色主题
 * 在浅色模式下仍保留科技感色彩
 */
private val CyberLightColorScheme = lightColorScheme(
    // 主色调 — 降低饱和度，日间模式更柔和
    primary = Color(0xFF1A8CA8),          // 柔和青蓝，降低刺眼感
    onPrimary = Color.White,
    primaryContainer = Color(0xFFD6F0F7), // 极淡青色容器
    onPrimaryContainer = Color(0xFF004D5E),

    // 次色调 — 降低饱和度
    secondary = Color(0xFF7B68C8),        // 柔和紫色
    onSecondary = Color.White,
    secondaryContainer = Color(0xFFE8E0F8),
    onSecondaryContainer = Color(0xFF3D2470),

    // 第三色 — 降低饱和度
    tertiary = Color(0xFF2DA06A),          // 柔和绿色
    onTertiary = Color.White,

    // 错误色 — 降低饱和度
    error = Color(0xFFD93B5C),
    onError = Color.White,

    // 背景 — 微暖白，减少冷调
    background = Color(0xFFF8F9FB),
    onBackground = Color(0xFF1A1F2E),
    surface = Color.White,
    onSurface = Color(0xFF1E293B),
    surfaceVariant = Color(0xFFF0F2F6),
    onSurfaceVariant = Color(0xFF5A6478),  // 次要文本更深，提高可读性
    surfaceTint = Color(0xFF1A8CA8),

    // 边框/分割线 — 更柔和
    outline = Color(0xFF94A3B8),           // 加深outline，年份文字更清晰
    outlineVariant = Color(0xFFE2E8F0),

    // Surface 容器层级（浅色模式）
    surfaceContainerHighest = Color(0xFFE2E7ED),
    surfaceContainerHigh = Color(0xFFECF0F4),
    surfaceContainer = Color(0xFFF3F5F8),
    surfaceContainerLow = Color(0xFFF8F9FB),
    surfaceContainerLowest = Color.White,
    surfaceBright = Color.White,
    surfaceDim = Color(0xFFE8ECF0),

    // 反色
    inverseSurface = Color(0xFF2E3440),
    inverseOnSurface = Color(0xFFF0F4F8),
    inversePrimary = CyberBlue,

    scrim = Color.Black,
)

/**
 * Nowen Video 主题 — 科技优雅版
 *
 * 支持：
 * - Material You 动态取色（Android 12+）
 * - 深色/浅色/跟随系统 三种模式
 * - 赛博朋克定制色彩方案
 * - 系统栏颜色自适应
 */
@Composable
fun NowenVideoTheme(
    themeMode: ThemeMode = ThemeMode.SYSTEM,
    dynamicColor: Boolean = false,
    content: @Composable () -> Unit
) {
    val darkTheme = when (themeMode) {
        ThemeMode.SYSTEM -> isSystemInDarkTheme()
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
    }

    val colorScheme = when {
        dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
            val context = LocalContext.current
            if (darkTheme) dynamicDarkColorScheme(context)
            else dynamicLightColorScheme(context)
        }
        darkTheme -> CyberDarkColorScheme
        else -> CyberLightColorScheme
    }

    // 自适应系统栏颜色
    val view = LocalView.current
    if (!view.isInEditMode) {
        SideEffect {
            val window = (view.context as? android.app.Activity)?.window ?: return@SideEffect
            WindowCompat.getInsetsController(window, view).apply {
                isAppearanceLightStatusBars = !darkTheme
                isAppearanceLightNavigationBars = !darkTheme
            }
        }
    }

    MaterialTheme(
        colorScheme = colorScheme,
        typography = Typography,
        content = content
    )
}
