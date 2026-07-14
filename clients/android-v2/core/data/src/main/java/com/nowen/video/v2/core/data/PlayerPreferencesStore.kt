package com.nowen.video.v2.core.data

import android.content.Context
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.floatPreferencesKey
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.IOException
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.map

private val Context.playerPreferencesDataStore by preferencesDataStore(name = "nowen_v2_player_preferences")
private val KEY_PLAYBACK_SPEED = floatPreferencesKey("playback_speed")
private val KEY_RESIZE_MODE = intPreferencesKey("resize_mode")
private val KEY_AUTO_PLAY_NEXT = booleanPreferencesKey("auto_play_next")

private val supportedSpeeds = setOf(0.5f, 0.75f, 1f, 1.25f, 1.5f, 1.75f, 2f)
private val supportedResizeModes = setOf(0, 1, 2)

data class PlayerPreferences(
    val playbackSpeed: Float = 1f,
    val resizeMode: Int = 0,
    val autoPlayNext: Boolean = true,
)

@Singleton
class PlayerPreferencesStore @Inject constructor(
    @ApplicationContext private val context: Context,
) {
    val preferences: Flow<PlayerPreferences> = context.playerPreferencesDataStore.data
        .catch { error ->
            if (error is IOException) emit(androidx.datastore.preferences.core.emptyPreferences())
            else throw error
        }
        .map { values ->
            PlayerPreferences(
                playbackSpeed = values[KEY_PLAYBACK_SPEED]
                    ?.takeIf(supportedSpeeds::contains)
                    ?: 1f,
                resizeMode = values[KEY_RESIZE_MODE]
                    ?.takeIf(supportedResizeModes::contains)
                    ?: 0,
                autoPlayNext = values[KEY_AUTO_PLAY_NEXT] ?: true,
            )
        }

    suspend fun setPlaybackSpeed(speed: Float) {
        context.playerPreferencesDataStore.edit { values ->
            values[KEY_PLAYBACK_SPEED] = speed.takeIf(supportedSpeeds::contains) ?: 1f
        }
    }

    suspend fun setResizeMode(mode: Int) {
        context.playerPreferencesDataStore.edit { values ->
            values[KEY_RESIZE_MODE] = mode.takeIf(supportedResizeModes::contains) ?: 0
        }
    }

    suspend fun setAutoPlayNext(enabled: Boolean) {
        context.playerPreferencesDataStore.edit { values ->
            values[KEY_AUTO_PLAY_NEXT] = enabled
        }
    }
}
