package com.nowen.video.v2

import android.app.PictureInPictureParams
import android.content.pm.PackageManager
import android.content.res.Configuration
import android.os.Build
import android.os.Bundle
import android.util.Rational
import android.view.WindowInsets
import android.view.WindowInsetsController
import android.view.WindowManager
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.core.view.WindowCompat
import com.nowen.video.v2.core.data.HighlightComputeAgent
import com.nowen.video.v2.feature.main.NowenApp
import com.nowen.video.v2.feature.main.PlaybackPictureInPictureHost
import dagger.hilt.android.AndroidEntryPoint
import javax.inject.Inject
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

/** 单 Activity + Compose 原生入口。 */
@AndroidEntryPoint
class MainActivity : ComponentActivity(), PlaybackPictureInPictureHost {
    @Inject lateinit var highlightComputeAgent: HighlightComputeAgent

    private val _pictureInPictureMode = MutableStateFlow(false)
    override val pictureInPictureMode: StateFlow<Boolean> = _pictureInPictureMode
    private var playbackPictureInPictureActive = false
    private var playbackOriginalBrightness: Float? = null

    override val playbackPictureInPictureSupported: Boolean
        get() = Build.VERSION.SDK_INT >= Build.VERSION_CODES.O &&
            packageManager.hasSystemFeature(PackageManager.FEATURE_PICTURE_IN_PICTURE)
    private var highlightComputeScope: CoroutineScope? = null

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        WindowCompat.setDecorFitsSystemWindows(window, false)
        setContent {
            NowenApp()
        }
    }

    override fun onStart() {
        super.onStart()
        if (highlightComputeScope == null) {
            highlightComputeScope = CoroutineScope(SupervisorJob() + Dispatchers.Default).also { scope ->
                scope.launch { highlightComputeAgent.runForegroundLoop() }
            }
        }
    }

    override fun onStop() {
        highlightComputeScope?.cancel()
        highlightComputeScope = null
        super.onStop()
    }

    override fun setPlaybackPictureInPictureActive(active: Boolean) {
        playbackPictureInPictureActive = active && playbackPictureInPictureSupported
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        setPictureInPictureParams(buildPictureInPictureParams(playbackPictureInPictureActive))
    }

    override fun enterPlaybackPictureInPicture() {
        if (playbackPictureInPictureActive && playbackPictureInPictureSupported && !isInPictureInPictureMode) {
            enterPictureInPictureMode(buildPictureInPictureParams(true))
        }
    }

    override fun currentPlaybackScreenBrightness(): Float {
        val current = window.attributes.screenBrightness
        return if (current in 0f..1f) current else 0.5f
    }

    override fun setPlaybackScreenBrightness(brightness: Float) {
        window.attributes = window.attributes.apply {
            screenBrightness = brightness.coerceIn(0f, 1f)
        }
    }

    override fun restorePlaybackScreenBrightness() {
        val original = playbackOriginalBrightness ?: return
        window.attributes = window.attributes.apply {
            screenBrightness = original
        }
        playbackOriginalBrightness = null
    }

    override fun setPlaybackLandscape(active: Boolean) {
        if (active && playbackOriginalBrightness == null) {
            playbackOriginalBrightness = window.attributes.screenBrightness
        }
        if (!active) restorePlaybackScreenBrightness()
        requestedOrientation = if (active) {
            android.content.pm.ActivityInfo.SCREEN_ORIENTATION_SENSOR_LANDSCAPE
        } else {
            android.content.pm.ActivityInfo.SCREEN_ORIENTATION_UNSPECIFIED
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.P) {
            window.attributes = window.attributes.apply {
                layoutInDisplayCutoutMode = if (!active) {
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_DEFAULT
                } else if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_ALWAYS
                } else {
                    WindowManager.LayoutParams.LAYOUT_IN_DISPLAY_CUTOUT_MODE_SHORT_EDGES
                }
            }
        }
        if (active) {
            window.addFlags(WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS)
            WindowCompat.setDecorFitsSystemWindows(window, false)
        } else {
            window.clearFlags(WindowManager.LayoutParams.FLAG_LAYOUT_NO_LIMITS)
            WindowCompat.setDecorFitsSystemWindows(window, false)
        }
        @Suppress("DEPRECATION")
        window.decorView.systemUiVisibility = if (active) {
            android.view.View.SYSTEM_UI_FLAG_LAYOUT_STABLE or
                android.view.View.SYSTEM_UI_FLAG_LAYOUT_FULLSCREEN or
                android.view.View.SYSTEM_UI_FLAG_LAYOUT_HIDE_NAVIGATION or
                android.view.View.SYSTEM_UI_FLAG_FULLSCREEN or
                android.view.View.SYSTEM_UI_FLAG_HIDE_NAVIGATION or
                android.view.View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY
        } else {
            0
        }
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.R) {
            window.insetsController?.let { controller ->
                if (active) {
                    controller.hide(WindowInsets.Type.systemBars())
                    controller.systemBarsBehavior = WindowInsetsController.BEHAVIOR_SHOW_TRANSIENT_BARS_BY_SWIPE
                } else {
                    controller.show(WindowInsets.Type.systemBars())
                }
            }
        }
    }

    override fun onUserLeaveHint() {
        super.onUserLeaveHint()
        if (
            Build.VERSION.SDK_INT in Build.VERSION_CODES.O until Build.VERSION_CODES.S &&
            playbackPictureInPictureActive &&
            playbackPictureInPictureSupported &&
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
