package com.nowen.video.v2.core.data

import android.content.Context
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.floatPreferencesKey
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.IOException
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.map

private val Context.danmakuPreferencesDataStore by preferencesDataStore(name = "nowen_v2_danmaku_preferences")
private val KEY_API_BASE_URL = stringPreferencesKey("api_base_url")
private val KEY_API_TOKEN = stringPreferencesKey("api_token")
private val KEY_ENABLED = booleanPreferencesKey("enabled")
private val KEY_AUTO_MATCH = booleanPreferencesKey("auto_match")
private val KEY_MODE = stringPreferencesKey("mode")
private val KEY_FONT_SIZE = floatPreferencesKey("font_size")
private val KEY_SPEED = floatPreferencesKey("speed")
private val KEY_OPACITY = floatPreferencesKey("opacity")
private val KEY_MAX_VISIBLE = intPreferencesKey("max_visible")
private val KEY_OFFSET_MS = intPreferencesKey("offset_ms")

data class DanmakuPreferences(
    val apiBaseUrl: String = "",
    val apiToken: String = "",
    val enabled: Boolean = true,
    val autoMatch: Boolean = true,
    val mode: String = "scroll",
    val fontSizeSp: Float = 18f,
    val speed: Float = 1f,
    val opacity: Float = 0.85f,
    val maxVisible: Int = 10,
    val offsetMs: Int = 0,
)

@Singleton
class DanmakuPreferencesStore @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    val preferences: Flow<DanmakuPreferences> = context.danmakuPreferencesDataStore.data
        .catch { error ->
            if (error is IOException) emit(androidx.datastore.preferences.core.emptyPreferences()) else throw error
        }
        .map { values ->
            DanmakuPreferences(
                apiBaseUrl = values[KEY_API_BASE_URL].orEmpty(),
                apiToken = values[KEY_API_TOKEN].orEmpty(),
                enabled = values[KEY_ENABLED] ?: true,
                autoMatch = values[KEY_AUTO_MATCH] ?: true,
                mode = values[KEY_MODE].orEmpty().takeIf { it in setOf("scroll", "top", "bottom") } ?: "scroll",
                fontSizeSp = values[KEY_FONT_SIZE]?.coerceIn(12f, 36f) ?: 18f,
                speed = values[KEY_SPEED]?.coerceIn(0.5f, 3f) ?: 1f,
                opacity = values[KEY_OPACITY]?.coerceIn(0.2f, 1f) ?: 0.85f,
                maxVisible = values[KEY_MAX_VISIBLE]?.coerceIn(1, 30) ?: 10,
                offsetMs = values[KEY_OFFSET_MS]?.coerceIn(-60_000, 60_000) ?: 0,
            )
        }

    suspend fun setApiBaseUrl(value: String) = edit { it[KEY_API_BASE_URL] = value.trim().trimEnd('/') }
    suspend fun setApiToken(value: String) = edit { it[KEY_API_TOKEN] = value.trim() }
    suspend fun setEnabled(value: Boolean) = edit { it[KEY_ENABLED] = value }
    suspend fun setAutoMatch(value: Boolean) = edit { it[KEY_AUTO_MATCH] = value }
    suspend fun setMode(value: String) = edit { it[KEY_MODE] = value.takeIf { mode -> mode in setOf("scroll", "top", "bottom") } ?: "scroll" }
    suspend fun setFontSize(value: Float) = edit { it[KEY_FONT_SIZE] = value.coerceIn(12f, 36f) }
    suspend fun setSpeed(value: Float) = edit { it[KEY_SPEED] = value.coerceIn(0.5f, 3f) }
    suspend fun setOpacity(value: Float) = edit { it[KEY_OPACITY] = value.coerceIn(0.2f, 1f) }
    suspend fun setMaxVisible(value: Int) = edit { it[KEY_MAX_VISIBLE] = value.coerceIn(1, 30) }
    suspend fun setOffsetMs(value: Int) = edit { it[KEY_OFFSET_MS] = value.coerceIn(-60_000, 60_000) }

    private suspend fun edit(block: suspend (androidx.datastore.preferences.core.MutablePreferences) -> Unit) {
        context.danmakuPreferencesDataStore.edit(block)
    }
}
