package com.nowen.video.ui.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp

/**
 * Nowen Video 赛博朋克排版系统
 *
 * 使用系统默认字体 + 科幻风格字间距/字重配置
 * 大标题：加宽字距 + 极粗字重 → 赛博朋克霓虹招牌感
 * 正文：清晰易读 + 适度字距 → 保证信息传达
 * 标签：紧凑字距 + Medium 字重 → 数据面板风格
 */

// 科幻感字体族（使用系统无衬线字体，通过字距和字重变化营造风格）
val CyberFontFamily = FontFamily.Default

val Typography = Typography(
    // ==================== 展示级标题 ====================
    displayLarge = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Black,
        fontSize = 57.sp,
        lineHeight = 64.sp,
        letterSpacing = 2.sp  // 宽字距 → 霓虹招牌感
    ),
    displayMedium = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Bold,
        fontSize = 45.sp,
        lineHeight = 52.sp,
        letterSpacing = 1.5.sp
    ),
    displaySmall = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Bold,
        fontSize = 36.sp,
        lineHeight = 44.sp,
        letterSpacing = 1.sp
    ),

    // ==================== 标题 ====================
    headlineLarge = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Bold,
        fontSize = 32.sp,
        lineHeight = 40.sp,
        letterSpacing = 1.5.sp  // 扩大字距
    ),
    headlineMedium = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.SemiBold,
        fontSize = 28.sp,
        lineHeight = 36.sp,
        letterSpacing = 1.sp
    ),
    headlineSmall = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.SemiBold,
        fontSize = 24.sp,
        lineHeight = 32.sp,
        letterSpacing = 0.5.sp
    ),

    // ==================== 页面标题 ====================
    titleLarge = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.SemiBold,
        fontSize = 22.sp,
        lineHeight = 28.sp,
        letterSpacing = 0.8.sp
    ),
    titleMedium = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 16.sp,
        lineHeight = 24.sp,
        letterSpacing = 0.5.sp
    ),
    titleSmall = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 14.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.3.sp
    ),

    // ==================== 正文 ====================
    bodyLarge = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Normal,
        fontSize = 16.sp,
        lineHeight = 24.sp,
        letterSpacing = 0.3.sp
    ),
    bodyMedium = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Normal,
        fontSize = 14.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.2.sp
    ),
    bodySmall = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Normal,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.2.sp
    ),

    // ==================== 标签 ====================
    labelLarge = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 14.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.5.sp  // 数据面板风格
    ),
    labelMedium = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.5.sp
    ),
    labelSmall = TextStyle(
        fontFamily = CyberFontFamily,
        fontWeight = FontWeight.Medium,
        fontSize = 11.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.4.sp
    ),
)
