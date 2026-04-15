package com.nowen.video.di

import android.content.Context
import com.jakewharton.retrofit2.converter.kotlinx.serialization.asConverterFactory
import com.nowen.video.data.local.PlayerPreferences
import com.nowen.video.data.local.ServerManager
import com.nowen.video.data.local.ThemePreferences
import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.remote.AuthInterceptor
import com.nowen.video.data.remote.NowenApiService
import com.nowen.video.data.remote.WebSocketManager
import dagger.Module
import dagger.Provides
import dagger.hilt.InstallIn
import dagger.hilt.android.qualifiers.ApplicationContext
import dagger.hilt.components.SingletonComponent
import kotlinx.serialization.json.Json
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import java.util.concurrent.TimeUnit
import javax.inject.Singleton

@Module
@InstallIn(SingletonComponent::class)
object NetworkModule {

    /** 服务器基础 URL — 运行时可通过 TokenManager 动态切换 */
    private const val DEFAULT_BASE_URL = "http://10.0.2.2:9090/api/"

    @Provides
    @Singleton
    fun provideJson(): Json = Json {
        ignoreUnknownKeys = true
        coerceInputValues = true
        isLenient = true
        encodeDefaults = true
    }

    @Provides
    @Singleton
    fun provideOkHttpClient(
        authInterceptor: AuthInterceptor
    ): OkHttpClient {
        return OkHttpClient.Builder()
            .addInterceptor(authInterceptor)
            .addInterceptor(HttpLoggingInterceptor().apply {
                level = HttpLoggingInterceptor.Level.BODY
            })
            .connectTimeout(30, TimeUnit.SECONDS)
            .readTimeout(60, TimeUnit.SECONDS)
            .writeTimeout(30, TimeUnit.SECONDS)
            .build()
    }

    @Provides
    @Singleton
    fun provideRetrofit(
        okHttpClient: OkHttpClient,
        json: Json
    ): Retrofit {
        val contentType = "application/json".toMediaType()
        return Retrofit.Builder()
            .baseUrl(DEFAULT_BASE_URL)
            .client(okHttpClient)
            .addConverterFactory(json.asConverterFactory(contentType))
            .build()
    }

    @Provides
    @Singleton
    fun provideNowenApiService(retrofit: Retrofit): NowenApiService {
        return retrofit.create(NowenApiService::class.java)
    }

    @Provides
    @Singleton
    fun provideTokenManager(
        @ApplicationContext context: Context
    ): TokenManager {
        return TokenManager(context)
    }

    @Provides
    @Singleton
    fun providePlayerPreferences(
        @ApplicationContext context: Context
    ): PlayerPreferences {
        return PlayerPreferences(context)
    }

    @Provides
    @Singleton
    fun provideThemePreferences(
        @ApplicationContext context: Context
    ): ThemePreferences {
        return ThemePreferences(context)
    }

    @Provides
    @Singleton
    fun provideServerManager(
        @ApplicationContext context: Context
    ): ServerManager {
        return ServerManager(context)
    }

    @Provides
    @Singleton
    fun provideWebSocketManager(
        tokenManager: TokenManager,
        json: Json
    ): WebSocketManager {
        return WebSocketManager(tokenManager, json)
    }
}
