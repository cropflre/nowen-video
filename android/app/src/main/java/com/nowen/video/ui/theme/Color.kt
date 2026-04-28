package com.nowen.video.ui.theme

import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color

// ==================== 科技感核心色彩（优化饱和度与对比度） ====================

// 赛博蓝 — 主色调（微调为更柔和的科技蓝，减少刺眼感）
val CyberBlue = Color(0xFF00C8F0)
val CyberBlueDark = Color(0xFF0094B8)
val CyberBlueLight = Color(0xFF5CE0FF)
val CyberBlueDim = Color(0xFF004D66)
val CyberBlueSubtle = Color(0xFF00C8F0).copy(alpha = 0.08f)

// 霓虹紫 — 辅助色（微调为更优雅的紫色）
val NeonPurple = Color(0xFFA78BFA)
val NeonPurpleDark = Color(0xFF7C5CE6)
val NeonPurpleLight = Color(0xFFCBB5FF)
val NeonPurpleDim = Color(0xFF3D2470)
val NeonPurpleSubtle = Color(0xFFA78BFA).copy(alpha = 0.08f)

// 电子绿 — 强调色（微调为更舒适的绿色）
val ElectricGreen = Color(0xFF00E68A)
val ElectricGreenDark = Color(0xFF00B86E)
val ElectricGreenDim = Color(0xFF004D2E)
val ElectricGreenSubtle = Color(0xFF00E68A).copy(alpha = 0.08f)

// 霓虹粉 — 特殊强调（微调为更柔和的粉色）
val NeonPink = Color(0xFFFF4081)
val NeonPinkLight = Color(0xFFFF6B9D)
val NeonPinkDim = Color(0xFF660029)

// 琥珀金 — 评分/星级（微调为更温暖的金色）
val AmberGold = Color(0xFFFFCA28)
val AmberGoldDim = Color(0xFFB8860B)

// ==================== 深空背景系统（优化层级对比度） ====================

// 深色主题背景层级（从深到浅，增强层次感）
val SpaceBlack = Color(0xFF020610)        // 最深背景（纯净深空）
val SpaceDarkBlue = Color(0xFF080D1C)     // 主背景（微蓝深空）
val SpaceSurface = Color(0xFF0F1628)      // Surface 层（深蓝灰）
val SpaceSurfaceHigh = Color(0xFF171E34)  // Surface 提升层
val SpaceSurfaceVariant = Color(0xFF1E2740) // Surface 变体
val SpaceBorder = Color(0xFF2A3550)       // 边框/分割线（更柔和）

// 深色主题前景色（优化可读性）
val SpaceOnBg = Color(0xFFF0F4F8)         // 主文本（微暖白）
val SpaceOnSurface = Color(0xFFDDE4EE)    // Surface 文本
val SpaceOnSurfaceDim = Color(0xFF8B99B0)  // 次要文本（更柔和）
val SpaceOnSurfaceVariant = Color(0xFF5C6B82) // 更次要文本

// ==================== 浅色主题（优化） ====================

val LightBackground = Color(0xFFF5F7FA)
val LightSurface = Color(0xFFFFFFFF)
val LightSurfaceVariant = Color(0xFFEBEFF5)
val LightOnBackground = Color(0xFF0F172A)
val LightOnSurface = Color(0xFF1E293B)

// ==================== Material 3 色彩映射 ====================

// 暗色主题主色
val Primary = CyberBlue
val OnPrimary = SpaceBlack
val PrimaryContainer = CyberBlueDim
val OnPrimaryContainer = CyberBlueLight

// 暗色主题次色
val Secondary = NeonPurple
val OnSecondary = SpaceBlack
val SecondaryContainer = NeonPurpleDim
val OnSecondaryContainer = NeonPurpleLight

// 暗色主题第三色
val Tertiary = ElectricGreen
val OnTertiary = SpaceBlack

// 暗色主题背景
val DarkBackground = SpaceDarkBlue
val DarkSurface = SpaceSurface
val DarkSurfaceVariant = SpaceSurfaceVariant
val DarkOnBackground = SpaceOnBg
val DarkOnSurface = SpaceOnSurface

// ==================== 文本色彩别名 ====================

val TextPrimary = SpaceOnBg              // 主文本
val TextSecondary = SpaceOnSurfaceDim    // 次要文本
val TextTertiary = SpaceOnSurfaceVariant // 更次要文本

// ==================== 功能色彩别名 ====================

val SuccessColor = ElectricGreen
val WarningColor = AmberGold
val ErrorColor = NeonPink
val InfoColor = CyberBlue

// ==================== 播放器专用色彩 ====================

val PlayerControlBg = Color(0xFF0A0F1E)
val PlayerControlBgAlpha = Color(0xFF0A0F1E).copy(alpha = 0.88f)
val PlayerProgressTrack = Color(0xFFFFFFFF).copy(alpha = 0.12f)
val PlayerProgressBuffer = Color(0xFFFFFFFF).copy(alpha = 0.28f)
val PlayerAccent = CyberBlue

// ==================== 渐变色预设（增强） ====================

val CyberGradient = Brush.horizontalGradient(
    colors = listOf(CyberBlue, NeonPurple)
)

val CyberGradientVertical = Brush.verticalGradient(
    colors = listOf(CyberBlue, NeonPurple)
)

val NeonGradient = Brush.horizontalGradient(
    colors = listOf(CyberBlue, ElectricGreen)
)

val PurpleGlowGradient = Brush.radialGradient(
    colors = listOf(NeonPurple.copy(alpha = 0.25f), Color.Transparent)
)

val SpaceGradient = Brush.verticalGradient(
    colors = listOf(SpaceBlack, SpaceDarkBlue, SpaceSurface)
)

val CardGlowGradient = Brush.verticalGradient(
    colors = listOf(
        CyberBlue.copy(alpha = 0.06f),
        Color.Transparent
    )
)

val SurfaceGradient = Brush.verticalGradient(
    colors = listOf(
        SpaceSurfaceHigh,
        SpaceSurface
    )
)

// 播放器进度条渐变
val PlayerProgressGradient = Brush.horizontalGradient(
    colors = listOf(CyberBlue, NeonPurple)
)

// 按钮渐变（主操作）
val PrimaryButtonGradient = Brush.horizontalGradient(
    colors = listOf(CyberBlue, NeonPurple.copy(alpha = 0.85f))
)

// 卡片悬浮光晕
val CardHoverGlow = Brush.radialGradient(
    colors = listOf(CyberBlue.copy(alpha = 0.12f), Color.Transparent)
)

// 底部遮罩渐变
val BottomScrimGradient = Brush.verticalGradient(
    colors = listOf(Color.Transparent, SpaceBlack.copy(alpha = 0.85f))
)
