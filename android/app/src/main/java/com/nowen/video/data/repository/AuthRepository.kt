package com.nowen.video.data.repository

import com.nowen.video.data.local.TokenManager
import com.nowen.video.data.model.LoginRequest
import com.nowen.video.data.model.RegisterRequest
import com.nowen.video.data.model.TokenResponse
import com.nowen.video.data.model.User
import com.nowen.video.data.remote.NowenApiService
import javax.inject.Inject
import javax.inject.Singleton

/**
 * 认证仓储 — 处理登录、注册、Token 管理
 */
@Singleton
class AuthRepository @Inject constructor(
    private val api: NowenApiService,
    private val tokenManager: TokenManager
) {
    /** 登录 */
    suspend fun login(username: String, password: String): Result<TokenResponse> {
        return try {
            val response = api.login(LoginRequest(username, password))
            // 保存 Token 和用户信息
            tokenManager.saveToken(response.token)
            tokenManager.saveUserInfo(
                userId = response.user.id,
                username = response.user.username,
                role = response.user.role
            )
            Result.success(response)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** 注册 */
    suspend fun register(username: String, password: String): Result<TokenResponse> {
        return try {
            val response = api.register(RegisterRequest(username, password))
            tokenManager.saveToken(response.token)
            tokenManager.saveUserInfo(
                userId = response.user.id,
                username = response.user.username,
                role = response.user.role
            )
            Result.success(response)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** 检查系统初始化状态 */
    suspend fun getInitStatus(): Result<Pair<Boolean, Boolean>> {
        return try {
            val status = api.getInitStatus()
            Result.success(Pair(status.data.initialized, status.data.registrationOpen))
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** 刷新 Token */
    suspend fun refreshToken(): Result<TokenResponse> {
        return try {
            val response = api.refreshToken()
            tokenManager.saveToken(response.token)
            Result.success(response)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** 获取当前用户信息 */
    suspend fun getProfile(): Result<User> {
        return try {
            Result.success(api.getProfile().data)
        } catch (e: Exception) {
            Result.failure(e)
        }
    }

    /** 登出 */
    suspend fun logout() {
        tokenManager.clearAll()
    }

    /** 是否已登录 */
    fun isLoggedInFlow() = tokenManager.isLoggedInFlow()

    /** 获取服务器地址 */
    suspend fun getServerUrl() = tokenManager.getServerUrl()

    /** 保存服务器地址 */
    suspend fun saveServerUrl(url: String) = tokenManager.saveServerUrl(url)
}
