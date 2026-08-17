package com.nowen.video.v2.feature.main

import androidx.compose.runtime.Composable

/**
 * PiP 路由适配层。播放器控制层直接订阅 Activity 的 PiP 状态，
 * 此参数保留在导航边界，便于后续按 PiP 状态收敛更多覆盖层而无需再改路由协议。
 */
@Composable
internal fun PlayerScreen(
    mediaId: String,
    pictureInPictureMode: Boolean,
    onBack: () -> Unit,
    onPlayNext: (String) -> Unit,
) {
    @Suppress("UNUSED_VARIABLE")
    val pipMode = pictureInPictureMode
    PlayerScreen(
        mediaId = mediaId,
        onBack = onBack,
        onPlayNext = onPlayNext,
    )
}
