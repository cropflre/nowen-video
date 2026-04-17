package com.nowen.video.ui.theme

import androidx.compose.ui.graphics.Brush
import androidx.compose.ui.graphics.Color

// ==================== 科技感核心色彩 ====================

// 赛博蓝 — 主色调
val CyberBlue = Color(0xFF00D4FF)
val CyberBlueDark = Color(0xFF0099CC)
val CyberBlueLight = Color(0xFF66E5FF)
val CyberBlueDim = Color(0xFF004D66)

// 霓虹紫 — 辅助色
val NeonPurple = Color(0xFFBB86FC)
val NeonPurpleDark = Color(0xFF8B5CF6)
val NeonPurpleLight = Color(0xFFD4BBFF)
val NeonPurpleDim = Color(0xFF4A2D7A)

// 电子绿 — 强调色
val ElectricGreen = Color(0xFF00FF94)
val ElectricGreenDark = Color(0xFF00CC76)
val ElectricGreenDim = Color(0xFF005C35)

// 霓虹粉 — 特殊强调
val NeonPink = Color(0xFFFF006E)
val NeonPinkLight = Color(0xFFFF4D94)
val NeonPinkDim = Color(0xFF660029)

// 琥珀金 — 评分/星级
val AmberGold = Color(0xFFFFD700)
val AmberGoldDim = Color(0xFFB8860B)

// ==================== 深空背景系统 ====================

// 深色主题背景层级（从深到浅）
val SpaceBlack = Color(0xFF030712)        // 最深背景
val SpaceDarkBlue = Color(0xFF0A0F1E)     // 主背景
val SpaceSurface = Color(0xFF111827)       // Surface 层
val SpaceSurfaceHigh = Color(0xFF1A2332)   // Surface 提升层
val SpaceSurfaceVariant = Color(0xFF1F2937) // Surface 变体
val SpaceBorder = Color(0xFF374151)        // 边框/分割线

// 深色主题前景色
val SpaceOnBg = Color(0xFFF1F5F9)         // 主文本
val SpaceOnSurface = Color(0xFFE2E8F0)    // Surface 文本
val SpaceOnSurfaceDim = Color(0xFF94A3B8)  // 次要文本
val SpaceOnSurfaceVariant = Color(0xFF64748B) // 更次要文本

// ==================== 浅色主题（备用） ====================

val LightBackground = Color(0xFFF0F4F8)
val LightSurface = Color(0xFFFFFFFF)
val LightSurfaceVariant = Color(0xFFE8EDF2)
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
// 为方便各屏幕统一引用

val TextPrimary = SpaceOnBg              // 主文本
val TextSecondary = SpaceOnSurfaceDim    // 次要文本
val TextTertiary = SpaceOnSurfaceVariant // 更次要文本

// ==================== 渐变色预设 ====================

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
    colors = listOf(NeonPurple.copy(alpha = 0.3f), Color.Transparent)
)

val SpaceGradient = Brush.verticalGradient(
    colors = listOf(SpaceBlack, SpaceDarkBlue, SpaceSurface)
)

val CardGlowGradient = Brush.verticalGradient(
    colors = listOf(
        CyberBlue.copy(alpha = 0.08f),
        Color.Transparent
    )
)

val SurfaceGradient = Brush.verticalGradient(
    colors = listOf(
        SpaceSurfaceHigh,
        SpaceSurface
    )
)
