package com.nowen.video.data.local

import android.content.Context
import androidx.datastore.core.DataStore
import androidx.datastore.preferences.core.Preferences
import androidx.datastore.preferences.core.booleanPreferencesKey
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.floatPreferencesKey
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import dagger.hilt.android.qualifiers.ApplicationContext
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import javax.inject.Inject
import javax.inject.Singleton

private val Context.playerPrefsStore: DataStore<Preferences> by preferencesDataStore(name = "player_prefs")

/**
 * 播放器偏好设置管理
 * 存储用户的播放器个性化配置
 */
@Singleton
class PlayerPreferences @Inject constructor(
    @ApplicationContext private val context: Context
) {
    companion object {
        // 播放速度
        private val KEY_PLAYBACK_SPEED = floatPreferencesKey("playback_speed")
        // 画面比例模式：0=适应, 1=填充, 2=16:9, 3=4:3
        private val KEY_ASPECT_RATIO = intPreferencesKey("aspect_ratio")
        // 优先解码器：0=自动, 1=硬件优先, 2=软件优先
        private val KEY_DECODER_PRIORITY = intPreferencesKey("decoder_priority")
        // 默认字幕语言
        private val KEY_SUBTITLE_LANGUAGE = stringPreferencesKey("subtitle_language")
        // 是否自动加载字幕
        private val KEY_AUTO_LOAD_SUBTITLE = booleanPreferencesKey("auto_load_subtitle")
        // 手势灵敏度：0=低, 1=中, 2=高
        private val KEY_GESTURE_SENSITIVITY = intPreferencesKey("gesture_sensitivity")
        // 是否启用手势控制
        private val KEY_GESTURE_ENABLED = booleanPreferencesKey("gesture_enabled")
        // 快进/快退步长（秒）
        private val KEY_SEEK_STEP = intPreferencesKey("seek_step")
        // 是否自动播放下一集
        private val KEY_AUTO_PLAY_NEXT = booleanPreferencesKey("auto_play_next")
        // 是否记住播放位置
        private val KEY_REMEMBER_POSITION = booleanPreferencesKey("remember_position")
        // 优先播放模式：0=自动, 1=直接播放, 2=Remux, 3=HLS
        private val KEY_PREFERRED_PLAY_MODE = intPreferencesKey("preferred_play_mode")
    }

    // ==================== 播放速度 ====================

    val playbackSpeedFlow: Flow<Float> = context.playerPrefsStore.data.map {
        it[KEY_PLAYBACK_SPEED] ?: 1.0f
    }

    suspend fun getPlaybackSpeed(): Float =
        context.playerPrefsStore.data.first()[KEY_PLAYBACK_SPEED] ?: 1.0f

    suspend fun setPlaybackSpeed(speed: Float) {
        context.playerPrefsStore.edit { it[KEY_PLAYBACK_SPEED] = speed }
    }

    // ==================== 画面比例 ====================

    val aspectRatioFlow: Flow<Int> = context.playerPrefsStore.data.map {
        it[KEY_ASPECT_RATIO] ?: 0
    }

    suspend fun getAspectRatio(): Int =
        context.playerPrefsStore.data.first()[KEY_ASPECT_RATIO] ?: 0

    suspend fun setAspectRatio(ratio: Int) {
        context.playerPrefsStore.edit { it[KEY_ASPECT_RATIO] = ratio }
    }

    // ==================== 解码器优先级 ====================

    val decoderPriorityFlow: Flow<Int> = context.playerPrefsStore.data.map {
        it[KEY_DECODER_PRIORITY] ?: 0
    }

    suspend fun getDecoderPriority(): Int =
        context.playerPrefsStore.data.first()[KEY_DECODER_PRIORITY] ?: 0

    suspend fun setDecoderPriority(priority: Int) {
        context.playerPrefsStore.edit { it[KEY_DECODER_PRIORITY] = priority }
    }

    // ==================== 字幕设置 ====================

    val subtitleLanguageFlow: Flow<String> = context.playerPrefsStore.data.map {
        it[KEY_SUBTITLE_LANGUAGE] ?: "chi"
    }

    suspend fun setSubtitleLanguage(lang: String) {
        context.playerPrefsStore.edit { it[KEY_SUBTITLE_LANGUAGE] = lang }
    }

    val autoLoadSubtitleFlow: Flow<Boolean> = context.playerPrefsStore.data.map {
        it[KEY_AUTO_LOAD_SUBTITLE] ?: true
    }

    suspend fun setAutoLoadSubtitle(enabled: Boolean) {
        context.playerPrefsStore.edit { it[KEY_AUTO_LOAD_SUBTITLE] = enabled }
    }

    // ==================== 手势控制 ====================

    val gestureEnabledFlow: Flow<Boolean> = context.playerPrefsStore.data.map {
        it[KEY_GESTURE_ENABLED] ?: true
    }

    suspend fun setGestureEnabled(enabled: Boolean) {
        context.playerPrefsStore.edit { it[KEY_GESTURE_ENABLED] = enabled }
    }

    val gestureSensitivityFlow: Flow<Int> = context.playerPrefsStore.data.map {
        it[KEY_GESTURE_SENSITIVITY] ?: 1
    }

    suspend fun setGestureSensitivity(sensitivity: Int) {
        context.playerPrefsStore.edit { it[KEY_GESTURE_SENSITIVITY] = sensitivity }
    }

    // ==================== 快进步长 ====================

    val seekStepFlow: Flow<Int> = context.playerPrefsStore.data.map {
        it[KEY_SEEK_STEP] ?: 10
    }

    suspend fun setSeekStep(step: Int) {
        context.playerPrefsStore.edit { it[KEY_SEEK_STEP] = step }
    }

    // ==================== 自动播放下一集 ====================

    val autoPlayNextFlow: Flow<Boolean> = context.playerPrefsStore.data.map {
        it[KEY_AUTO_PLAY_NEXT] ?: true
    }

    suspend fun setAutoPlayNext(enabled: Boolean) {
        context.playerPrefsStore.edit { it[KEY_AUTO_PLAY_NEXT] = enabled }
    }

    // ==================== 记住播放位置 ====================

    val rememberPositionFlow: Flow<Boolean> = context.playerPrefsStore.data.map {
        it[KEY_REMEMBER_POSITION] ?: true
    }

    suspend fun setRememberPosition(enabled: Boolean) {
        context.playerPrefsStore.edit { it[KEY_REMEMBER_POSITION] = enabled }
    }

    // ==================== 优先播放模式 ====================

    val preferredPlayModeFlow: Flow<Int> = context.playerPrefsStore.data.map {
        it[KEY_PREFERRED_PLAY_MODE] ?: 0
    }

    suspend fun setPreferredPlayMode(mode: Int) {
        context.playerPrefsStore.edit { it[KEY_PREFERRED_PLAY_MODE] = mode }
    }
}
