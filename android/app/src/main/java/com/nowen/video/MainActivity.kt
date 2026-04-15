package com.nowen.video

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.material3.Surface
import androidx.compose.ui.Modifier
import com.nowen.video.ui.NowenVideoApp
import dagger.hilt.android.AndroidEntryPoint

/**
 * 单 Activity 架构入口
 * Phase 4: Edge-to-Edge + 主题由 NowenVideoApp 统一管理
 */
@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            // 主题已在 NowenVideoApp 内部管理（支持动态切换）
            NowenVideoApp()
        }
    }
}
