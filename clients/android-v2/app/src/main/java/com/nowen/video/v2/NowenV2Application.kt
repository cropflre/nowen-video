package com.nowen.video.v2

import android.app.Application
import dagger.hilt.android.HiltAndroidApp

/**
 * Android V2 应用入口。
 *
 * V2 使用独立 applicationId，与旧客户端并行安装；所有跨模块依赖由 Hilt
 * 从 Application 级容器统一提供。
 */
@HiltAndroidApp
class NowenV2Application : Application()
