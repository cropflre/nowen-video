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
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json
import javax.inject.Inject
import javax.inject.Singleton

private val Context.serverStore: DataStore<Preferences> by preferencesDataStore(name = "server_profiles")

/**
 * 服务器配置信息
 */
@Serializable
data class ServerProfile(
    val id: String,           // 唯一标识（UUID）
    val name: String,         // 显示名称（如 "家庭服务器"）
    val url: String,          // 服务器地址
    val username: String = "",
    val token: String = "",
    val userId: String = "",
    val userRole: String = "",
    val isActive: Boolean = false, // 是否为当前活跃服务器
    val lastConnected: Long = 0    // 上次连接时间戳
)

/**
 * 多服务器配置管理器
 * 支持保存、切换、删除多个 Nowen Video 服务器
 */
@Singleton
class ServerManager @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        private val KEY_SERVERS = stringPreferencesKey("server_profiles_json")
        private val KEY_ACTIVE_SERVER_ID = stringPreferencesKey("active_server_id")
    }

    private val json = Json {
        ignoreUnknownKeys = true
        encodeDefaults = true
    }

    /**
     * 获取所有服务器配置
     */
    suspend fun getServers(): List<ServerProfile> {
        val raw = context.serverStore.data.first()[KEY_SERVERS] ?: return emptyList()
        return try {
            json.decodeFromString<List<ServerProfile>>(raw)
        } catch (_: Exception) {
            emptyList()
        }
    }

    /**
     * 服务器列表 Flow
     */
    fun serversFlow(): Flow<List<ServerProfile>> {
        return context.serverStore.data.map { prefs ->
            val raw = prefs[KEY_SERVERS] ?: return@map emptyList()
            try {
                json.decodeFromString<List<ServerProfile>>(raw)
            } catch (_: Exception) {
                emptyList()
            }
        }
    }

    /**
     * 获取当前活跃服务器
     */
    suspend fun getActiveServer(): ServerProfile? {
        val servers = getServers()
        val activeId = context.serverStore.data.first()[KEY_ACTIVE_SERVER_ID]
        return servers.find { it.id == activeId } ?: servers.firstOrNull()
    }

    /**
     * 添加或更新服务器配置
     */
    suspend fun saveServer(server: ServerProfile) {
        val servers = getServers().toMutableList()
        val index = servers.indexOfFirst { it.id == server.id }
        if (index >= 0) {
            servers[index] = server
        } else {
            servers.add(server)
        }
        context.serverStore.edit {
            it[KEY_SERVERS] = json.encodeToString(servers)
        }
    }

    /**
     * 切换活跃服务器
     */
    suspend fun setActiveServer(serverId: String) {
        context.serverStore.edit {
            it[KEY_ACTIVE_SERVER_ID] = serverId
        }
        // 同时更新 servers 列表中的 isActive 标记
        val servers = getServers().map { s ->
            s.copy(isActive = s.id == serverId)
        }
        context.serverStore.edit {
            it[KEY_SERVERS] = json.encodeToString(servers)
        }
    }

    /**
     * 删除服务器配置
     */
    suspend fun removeServer(serverId: String) {
        val servers = getServers().filter { it.id != serverId }
        context.serverStore.edit {
            it[KEY_SERVERS] = json.encodeToString(servers)
        }
        // 如果删除的是活跃服务器，切换到第一个
        val activeId = context.serverStore.data.first()[KEY_ACTIVE_SERVER_ID]
        if (activeId == serverId && servers.isNotEmpty()) {
            setActiveServer(servers.first().id)
        }
    }

    /**
     * 将当前 TokenManager 中的服务器信息迁移到 ServerManager
     * 用于首次升级时的数据迁移
     */
    suspend fun migrateFromTokenManager(tokenManager: TokenManager) {
        val servers = getServers()
        if (servers.isNotEmpty()) return // 已有数据，不需要迁移

        val serverUrl = tokenManager.getServerUrl() ?: return
        val token = tokenManager.getToken() ?: return
        val username = tokenManager.getUsername() ?: ""
        val userId = tokenManager.getUserId() ?: ""

        val profile = ServerProfile(
            id = java.util.UUID.randomUUID().toString(),
            name = "默认服务器",
            url = serverUrl,
            username = username,
            token = token,
            userId = userId,
            isActive = true,
            lastConnected = System.currentTimeMillis()
        )

        saveServer(profile)
        setActiveServer(profile.id)
    }
}
