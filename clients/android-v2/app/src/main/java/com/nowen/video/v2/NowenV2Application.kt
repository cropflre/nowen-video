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
 * Nowen Video 正式 Android 应用入口。
 *
 * V2 模块化实现已经接管历史 `com.nowen.video` applicationId。源码 namespace
 * 继续保留 `.v2`，只作为内部代码边界，不再代表一个可并行安装的独立产品。
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
