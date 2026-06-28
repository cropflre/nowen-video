package com.nowen.video.ui.theme

import androidx.compose.ui.graphics.Color
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp

/**
 * 移动端设计 Token
 * Hills Pro 风格：柔和浅色背景 + 毛玻璃 + 大圆角卡片
 */
object MobileColors {
    // 背景色
    val Bg = Color(0xFFF8F5FB)
    val BgAlt = Color(0xFFF4F1F8)
    val BgWarm = Color(0xFFE5B5A7)

    // 文字色
    val Text = Color(0xFF171820)
    val Muted = Color(0xFF6B6E7A)
    val TextSecondary = Color(0xFF8E919E)

    // 主题色
    val Primary = Color(0xFF142060)
    val PrimarySoft = Color(0xFFDFE2FF)
    val PrimaryLight = Color(0xFFE8EAFF)

    // 卡片
    val Card = Color.White.copy(alpha = 0.72f)
    val CardBorder = Color(0x1A1C2038)

    // 毛玻璃
    val Glass = Color(0xFFF6F4FC).copy(alpha = 0.82f)
    val GlassBorder = Color.White.copy(alpha = 0.72f)

    // 选中态
    val Active = Color(0xFFE0E2FF).copy(alpha = 0.86f)
    val ActiveText = Color(0xFF0F1D5A)

    // 成功/警告/错误
    val Success = Color(0xFF22C55E)
    val Warning = Color(0xFFF59E0B)
    val Error = Color(0xFFEF4444)
}

object MobileRadius {
    val xs = 8.dp
    val sm = 12.dp
    val md = 16.dp
    val lg = 20.dp
    val xl = 24.dp
    val xxl = 28.dp
    val xxxl = 32.dp
    val full = 999.dp
}

object MobileSpacing {
    val xs = 8.dp
    val sm = 12.dp
    val md = 16.dp
    val lg = 24.dp
    val xl = 32.dp
    val xxl = 40.dp
    val xxxl = 48.dp
}

object MobileFontSize {
    val xs = 12.sp
    val sm = 14.sp
    val md = 16.sp
    val lg = 18.sp
    val xl = 22.sp
    val xxl = 26.sp
    val xxxl = 32.sp
    val xxxxl = 36.sp
}

object MobileElevation {
    val sm = 2.dp
    val md = 4.dp
    val lg = 8.dp
    val xl = 16.dp
}
