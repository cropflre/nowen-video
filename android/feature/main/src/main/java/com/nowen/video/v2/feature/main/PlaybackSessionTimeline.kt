package com.nowen.video.v2.feature.main

internal fun absolutePlaybackPositionMs(
    sourceOffsetMs: Long,
    relativePositionMs: Long,
): Long = sourceOffsetMs.coerceAtLeast(0L) + relativePositionMs.coerceAtLeast(0L)

internal fun relativePlaybackPositionMs(
    sourceOffsetMs: Long,
    absolutePositionMs: Long,
): Long = (absolutePositionMs - sourceOffsetMs.coerceAtLeast(0L)).coerceAtLeast(0L)

internal fun clampPlaybackTargetMs(targetMs: Long, durationMs: Long): Long {
    val target = targetMs.coerceAtLeast(0L)
    return if (durationMs > 0L) target.coerceAtMost(durationMs) else target
}

internal fun clampRemuxStartMs(targetMs: Long, durationMs: Long): Long {
    if (durationMs <= 0L) return targetMs.coerceAtLeast(0L)
    // FFmpeg needs frames after the start position to produce a playable stream.
    return targetMs.coerceIn(0L, (durationMs - 1_000L).coerceAtLeast(0L))
}
