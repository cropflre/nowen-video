package com.nowen.video.v2

import android.os.Bundle
import androidx.activity.ComponentActivity
import androidx.activity.compose.setContent
import androidx.activity.enableEdgeToEdge
import com.nowen.video.v2.feature.main.NowenV2App
import dagger.hilt.android.AndroidEntryPoint

/** 单 Activity + Compose 原生入口。 */
@AndroidEntryPoint
class MainActivity : ComponentActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        enableEdgeToEdge()
        setContent {
            NowenV2App()
        }
    }
}
