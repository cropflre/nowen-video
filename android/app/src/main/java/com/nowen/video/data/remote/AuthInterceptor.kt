package com.nowen.video.data.remote

import com.nowen.video.data.local.TokenManager
import kotlinx.coroutines.runBlocking
import okhttp3.HttpUrl.Companion.toHttpUrlOrNull
import okhttp3.Interceptor
import okhttp3.Response
import javax.inject.Inject
import javax.inject.Singleton

/**
 * JWT Token 自动注入 + 动态 Base URL 替换拦截器
 *
 * 1. 从 DataStore 读取用户配置的服务器地址，动态替换请求 URL
 * 2. 自动注入 JWT Token 到请求头
 *
 * 这样 Retrofit 的 baseUrl 只作为占位符，实际请求地址由用户配置决定
 */
@Singleton
class AuthInterceptor @Inject constructor(
    private val tokenManager: TokenManager
) : Interceptor {

    companion object {
        /** 与 NetworkModule 中的 DEFAULT_BASE_URL 保持一致的占位 host */
        private const val PLACEHOLDER_HOST = "10.0.2.2"
        private const val PLACEHOLDER_PORT = 8080
    }

    override fun intercept(chain: Interceptor.Chain): Response {
        val originalRequest = chain.request()

        // 从 DataStore 读取用户配置的服务器地址和 Token
        val (serverUrl, token) = runBlocking {
            Pair(tokenManager.getServerUrl(), tokenManager.getToken())
        }

        var requestBuilder = originalRequest.newBuilder()

        // 1. 动态替换 Base URL
        // 如果用户已配置服务器地址，将请求中的占位 host 替换为实际地址
        if (!serverUrl.isNullOrBlank()) {
            val configuredUrl = serverUrl.trimEnd('/').toHttpUrlOrNull()
            if (configuredUrl != null) {
                val originalUrl = originalRequest.url
                // 只替换 host 和 port 匹配占位符的请求
                if (originalUrl.host == PLACEHOLDER_HOST && originalUrl.port == PLACEHOLDER_PORT) {
                    val newUrl = originalUrl.newBuilder()
                        .scheme(configuredUrl.scheme)
                        .host(configuredUrl.host)
                        .port(configuredUrl.port)
                        .build()
                    requestBuilder = requestBuilder.url(newUrl)
                }
            }
        }

        // 2. 注入 JWT Token
        if (token != null) {
            requestBuilder.addHeader("Authorization", "Bearer $token")
        }

        return chain.proceed(requestBuilder.build())
    }
}
