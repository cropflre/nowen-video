package com.nowen.video.v2.feature.main

internal fun absolutePlaybackPositionMs(
    sessionManaged: Boolean,
    generationOffsetMs: Long,
    relativePositionMs: Long,
): Long {
    val relative = relativePositionMs.coerceAtLeast(0L)
    return if (sessionManaged) generationOffsetMs.coerceAtLeast(0L) + relative else relative
}

internal fun relativePlaybackPositionMs(
    sessionManaged: Boolean,
    generationOffsetMs: Long,
    absolutePositionMs: Long,
): Long {
    val absolute = absolutePositionMs.coerceAtLeast(0L)
    return if (sessionManaged) (absolute - generationOffsetMs.coerceAtLeast(0L)).coerceAtLeast(0L) else absolute
}

internal fun clampPlaybackTargetMs(targetMs: Long, durationMs: Long): Long {
    val target = targetMs.coerceAtLeast(0L)
    return if (durationMs > 0L) target.coerceAtMost(durationMs) else target
}
