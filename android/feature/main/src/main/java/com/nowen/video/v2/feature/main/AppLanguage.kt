package com.nowen.video.v2.feature.main

import androidx.compose.runtime.Composable
import androidx.compose.runtime.ReadOnlyComposable
import androidx.compose.runtime.staticCompositionLocalOf
import java.util.Locale

val LocalIsEnglish = staticCompositionLocalOf { false }

enum class AppLanguageOption(
    val tag: String,
    val titleZh: String,
    val titleEn: String,
    val subtitleZh: String,
    val subtitleEn: String,
) {
    System(
        tag = "system",
        titleZh = "跟随系统",
        titleEn = "Follow System",
        subtitleZh = "自动跟随设备系统语言",
        subtitleEn = "Match your device language automatically",
    ),
    SimplifiedChinese(
        tag = "zh-CN",
        titleZh = "简体中文",
        titleEn = "简体中文",
        subtitleZh = "Simplified Chinese",
        subtitleEn = "Simplified Chinese",
    ),
    English(
        tag = "en",
        titleZh = "English",
        titleEn = "English",
        subtitleZh = "英文界面",
        subtitleEn = "English interface",
    ),
}

fun resolveIsEnglish(tag: String): Boolean {
    return when (tag) {
        "en" -> true
        "zh-CN" -> false
        else -> Locale.getDefault().language.lowercase().startsWith("en")
    }
}

fun formatLanguageLabel(tag: String, isEnglish: Boolean): String {
    val option = AppLanguageOption.entries.firstOrNull { it.tag == tag } ?: AppLanguageOption.System
    return if (isEnglish) option.titleEn else option.titleZh
}

@Composable
@ReadOnlyComposable
fun tr(zh: String, en: String): String = if (LocalIsEnglish.current) en else zh
