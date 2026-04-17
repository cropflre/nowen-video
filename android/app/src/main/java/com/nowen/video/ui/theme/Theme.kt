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
 * 赛博朋克浅色主题（保留但不作为默认推荐）
 * 在浅色模式下仍保留科技感色彩
 */
private val CyberLightColorScheme = lightColorScheme(
    primary = CyberBlueDark,
    onPrimary = Color.White,
    primaryContainer = CyberBlueLight.copy(alpha = 0.3f),
    onPrimaryContainer = CyberBlueDim,

    secondary = NeonPurpleDark,
    onSecondary = Color.White,
    secondaryContainer = NeonPurpleLight.copy(alpha = 0.3f),
    onSecondaryContainer = NeonPurpleDim,

    tertiary = ElectricGreenDark,
    onTertiary = Color.White,

    error = NeonPink,
    onError = Color.White,

    background = LightBackground,
    onBackground = LightOnBackground,
    surface = LightSurface,
    onSurface = LightOnSurface,
    surfaceVariant = LightSurfaceVariant,
    onSurfaceVariant = Color(0xFF475569),
    surfaceTint = CyberBlueDark,

    outline = Color(0xFFCBD5E1),
    outlineVariant = Color(0xFFE2E8F0),

    // Surface 容器层级（浅色模式）
    surfaceContainerHighest = Color(0xFFDDE3EA),
    surfaceContainerHigh = Color(0xFFE8EDF2),
    surfaceContainer = Color(0xFFF0F4F8),
    surfaceContainerLow = Color(0xFFF5F8FB),
    surfaceContainerLowest = Color.White,
    surfaceBright = Color.White,
    surfaceDim = Color(0xFFE0E5EB),

    // 反色
    inverseSurface = Color(0xFF2E3440),
    inverseOnSurface = Color(0xFFF0F4F8),
    inversePrimary = CyberBlue,

    scrim = Color.Black,
)

/**
 * Nowen Video 主题 — 赛博朋克版
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
    dynamicColor: Boolean = false,  // 默认关闭动态取色，保持赛博朋克风格
    content: @Composable () -> Unit
) {
    val darkTheme = when (themeMode) {
        ThemeMode.SYSTEM -> isSystemInDarkTheme()
        ThemeMode.LIGHT -> false
        ThemeMode.DARK -> true
    }

    val colorScheme = when {
        // Android 12+ 支持动态取色（Material You）
        dynamicColor && Build.VERSION.SDK_INT >= Build.VERSION_CODES.S -> {
            val context = LocalContext.current
            if (darkTheme) dynamicDarkColorScheme(context)
            else dynamicLightColorScheme(context)
        }
        darkTheme -> CyberDarkColorScheme
        else -> CyberLightColorScheme
    }

    // 自适应系统栏颜色 — 深色模式下系统栏全透明
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
