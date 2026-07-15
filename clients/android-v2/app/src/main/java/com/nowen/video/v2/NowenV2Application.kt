package com.nowen.video.v2

import android.app.Application
import coil.ImageLoader
import coil.ImageLoaderFactory
import com.nowen.video.v2.core.data.OfflineDownloadScheduler
import com.nowen.video.v2.core.data.ServerSessionStore
import dagger.hilt.android.HiltAndroidApp
import javax.inject.Inject
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.OkHttpClient

/**
 * Android V2 应用入口。
 *
 * V2 使用独立 applicationId，与旧客户端并行安装；所有跨模块依赖由 Hilt
 * 从 Application 级容器统一提供。
 */
@HiltAndroidApp
class NowenV2Application : Application(), ImageLoaderFactory {
    @Inject lateinit var networkClient: OkHttpClient
    @Inject lateinit var sessionStore: ServerSessionStore

    override fun onCreate() {
        super.onCreate()
        OfflineDownloadScheduler.schedule(this)
    }

    /**
     * Coil 默认不会复用 Retrofit 的认证拦截器。合集海报、人物头像和本地媒体图片
     * 都可能位于 JWT 保护的同源接口，因此为当前服务器同源请求补充 Bearer Token。
     * 外部 TMDb/图床地址不会携带 Token，避免凭据泄露到第三方域名。
     */
    override fun newImageLoader(): ImageLoader {
        val imageClient = networkClient.newBuilder().apply {
            interceptors().clear()
            networkInterceptors().clear()
        }.addInterceptor { chain ->
            val snapshot = sessionStore.snapshot.value
            val serverOrigin = snapshot.activeServer?.baseUrl?.toHttpUrlOrNull()
            val request = chain.request()
            val sameOrigin = serverOrigin != null &&
                request.url.scheme == serverOrigin.scheme &&
                request.url.host.equals(serverOrigin.host, ignoreCase = true) &&
                request.url.port == serverOrigin.port
            val authenticated = request.newBuilder().apply {
                if (sameOrigin) {
                    snapshot.token?.takeIf(String::isNotBlank)?.let {
                        header("Authorization", "Bearer $it")
                    }
                }
            }.build()
            chain.proceed(authenticated)
        }.build()

        return ImageLoader.Builder(this)
            .okHttpClient(imageClient)
            .crossfade(true)
            .build()
    }
}
