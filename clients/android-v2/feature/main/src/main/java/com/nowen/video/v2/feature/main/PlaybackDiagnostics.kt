package com.nowen.video.v2.feature.main

data class PlaybackDiagnostics(
    val method: String = "",
    val methodLabel: String = "自动选择",
    val reasonCode: String = "",
    val reason: String = "",
    val fallbackUrl: String = "",
    val fallbackMethod: String = "",
    val fallbackMethodLabel: String = "",
    val usingFallback: Boolean = false,
    val lastError: String = "",
) {
    val fallbackAvailable: Boolean
        get() = fallbackUrl.isNotBlank() && !usingFallback
}

internal fun playbackMethodLabel(method: String): String = when (method.lowercase()) {
    "direct" -> "直接播放"
    "remux" -> "无损封装转换"
    "transcode" -> "兼容转码"
    else -> "自动选择"
}

internal fun shouldAttemptPlaybackFallback(
    errorCode: Int,
    currentUrl: String,
    fallbackUrl: String,
    alreadyUsingFallback: Boolean,
): Boolean {
    if (alreadyUsingFallback || fallbackUrl.isBlank() || fallbackUrl == currentUrl) return false

    // Media3 reserves 2xxx for IO, 3xxx for parsing and 4xxx for decoder errors.
    // These are the failures a server-provided transcode fallback can reasonably recover from.
    return errorCode in 2_000..4_999
}
