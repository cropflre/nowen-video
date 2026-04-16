package com.nowen.video.data.local

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.dataStore: DataStore<Preferences> by preferencesDataStore(name = "nowen_prefs")

/**
 * Token 和服务器配置管理
 * 使用 DataStore 持久化存储
 */
@Singleton
class TokenManager @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        private val KEY_TOKEN = stringPreferencesKey("jwt_token")
        private val KEY_SERVER_URL = stringPreferencesKey("server_url")
        private val KEY_USERNAME = stringPreferencesKey("username")
        private val KEY_USER_ID = stringPreferencesKey("user_id")
        private val KEY_USER_ROLE = stringPreferencesKey("user_role")
    }

    // ==================== Token ====================

    suspend fun saveToken(token: String) {
        context.dataStore.edit { it[KEY_TOKEN] = token }
    }

    suspend fun getToken(): String? {
        return context.dataStore.data.first()[KEY_TOKEN]
    }

    fun tokenFlow(): Flow<String?> {
        return context.dataStore.data.map { it[KEY_TOKEN] }
    }

    suspend fun clearToken() {
        context.dataStore.edit { it.remove(KEY_TOKEN) }
    }

    // ==================== 服务器地址 ====================

    suspend fun saveServerUrl(url: String) {
        context.dataStore.edit { it[KEY_SERVER_URL] = url }
    }

    suspend fun getServerUrl(): String? {
        return context.dataStore.data.first()[KEY_SERVER_URL]
    }

    fun serverUrlFlow(): Flow<String?> {
        return context.dataStore.data.map { it[KEY_SERVER_URL] }
    }

    // ==================== 用户信息 ====================

    suspend fun saveUserInfo(userId: String, username: String, role: String) {
        context.dataStore.edit {
            it[KEY_USER_ID] = userId
            it[KEY_USERNAME] = username
            it[KEY_USER_ROLE] = role
        }
    }

    suspend fun getUsername(): String? {
        return context.dataStore.data.first()[KEY_USERNAME]
    }

    suspend fun getUserId(): String? {
        return context.dataStore.data.first()[KEY_USER_ID]
    }

    fun isLoggedInFlow(): Flow<Boolean> {
        return context.dataStore.data.map { it[KEY_TOKEN] != null }
    }

    // ==================== 清除所有数据 ====================

    suspend fun clearAll() {
        context.dataStore.edit { it.clear() }
    }
}
