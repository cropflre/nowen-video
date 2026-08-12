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

/** Nowen Video 正式 Android 应用入口。 */
@HiltAndroidApp
class NowenApplication : Application(), ImageLoaderFactory {
    @Inject lateinit var networkClient: OkHttpClient
    @Inject lateinit var sessionStore: ServerSessionStore

    override fun onCreate() {
        super.onCreate()
        OfflineDownloadScheduler.schedule(this)
    }

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
