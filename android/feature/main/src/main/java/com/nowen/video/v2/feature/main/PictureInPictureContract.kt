package com.nowen.video.v2.feature.main

import android.content.Context
import android.content.ContextWrapper
import kotlinx.coroutines.flow.StateFlow

/** Activity 提供给播放器路由的画中画能力契约，避免 feature 模块反向依赖 app。 */
interface PlaybackPictureInPictureHost {
    val pictureInPictureMode: StateFlow<Boolean>
    val playbackPictureInPictureSupported: Boolean
    fun setPlaybackPictureInPictureActive(active: Boolean)
    fun enterPlaybackPictureInPicture()
    fun setPlaybackLandscape(active: Boolean)
    fun currentPlaybackScreenBrightness(): Float
    fun setPlaybackScreenBrightness(brightness: Float)
    fun restorePlaybackScreenBrightness()
}

internal fun Context.findPlaybackPictureInPictureHost(): PlaybackPictureInPictureHost? {
    var current: Context? = this
    while (current != null) {
        if (current is PlaybackPictureInPictureHost) return current
        current = (current as? ContextWrapper)?.baseContext?.takeUnless { it === current }
    }
    return null
}
