package com.nowen.video.v2

import android.app.PictureInPictureParams
import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import android.util.Rational
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.nowen.video.v2.feature.main.NowenApp
import com.nowen.video.v2.feature.main.PlaybackPictureInPictureHost
import dagger.hilt.android.AndroidEntryPoint
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow

/** 单 Activity + Compose 原生入口。 */
@AndroidEntryPoint
class MainActivity : ComponentActivity(), PlaybackPictureInPictureHost {
    private val _pictureInPictureMode = MutableStateFlow(false)
    override val pictureInPictureMode: StateFlow<Boolean> = _pictureInPictureMode
    private var playbackPictureInPictureActive = false

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            NowenApp()
        }
    }

    override fun setPlaybackPictureInPictureActive(active: Boolean) {
        playbackPictureInPictureActive = active
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        setPictureInPictureParams(buildPictureInPictureParams(active))
    }

    override fun onUserLeaveHint() {
        super.onUserLeaveHint()
        if (
            Build.VERSION.SDK_INT in Build.VERSION_CODES.O until Build.VERSION_CODES.S &&
            playbackPictureInPictureActive &&
            !isInPictureInPictureMode
        ) {
            enterPictureInPictureMode(buildPictureInPictureParams(true))
        }
    }

    override fun onPictureInPictureModeChanged(
        isInPictureInPictureMode: Boolean,
        newConfig: Configuration,
    ) {
        super.onPictureInPictureModeChanged(isInPictureInPictureMode, newConfig)
        _pictureInPictureMode.value = isInPictureInPictureMode
    }

    private fun buildPictureInPictureParams(active: Boolean): PictureInPictureParams {
        return PictureInPictureParams.Builder()
            .setAspectRatio(Rational(16, 9))
            .apply {
                if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.S) {
                    setAutoEnterEnabled(active)
                    setSeamlessResizeEnabled(true)
                }
            }
            .build()
    }
}
