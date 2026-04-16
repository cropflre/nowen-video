package com.nowen.video.data.local

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.themeStore: DataStore<Preferences> by preferencesDataStore(name = "theme_prefs")

/**
 * 主题模式枚举
 */
enum class ThemeMode(val value: Int) {
    SYSTEM(0),  // 跟随系统
    LIGHT(1),   // 浅色
    DARK(2);    // 深色

    companion object {
        fun fromValue(value: Int) = entries.find { it.value == value } ?: SYSTEM
    }
}

/**
 * 主题偏好管理器
 * 存储用户的主题模式选择
 */
@Singleton
class ThemePreferences @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        private val KEY_THEME_MODE = intPreferencesKey("theme_mode")
    }

    /**
     * 主题模式 Flow — UI 层订阅此 Flow 实时响应主题切换
     */
    val themeModeFlow: Flow<ThemeMode> = context.themeStore.data.map { prefs ->
        ThemeMode.fromValue(prefs[KEY_THEME_MODE] ?: 0)
    }

    /**
     * 设置主题模式
     */
    suspend fun setThemeMode(mode: ThemeMode) {
        context.themeStore.edit { it[KEY_THEME_MODE] = mode.value }
    }
}
