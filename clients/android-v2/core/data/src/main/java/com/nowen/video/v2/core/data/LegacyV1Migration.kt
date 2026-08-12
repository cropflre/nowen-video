package com.nowen.video.v2.core.data

import android.content.Context
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.nowen.video.v2.core.model.ServerProfile
import dagger.hilt.android.qualifiers.ApplicationContext
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.first
import kotlinx.serialization.Serializable
import kotlinx.serialization.json.Json

private val Context.legacyServerProfilesStore by preferencesDataStore(name = "server_profiles")
private val Context.legacyNowenPrefsStore by preferencesDataStore(name = "nowen_prefs")

/**
 * One-time bridge used when the modular Android client takes over the historical
 * `com.nowen.video` applicationId.
 *
 * V1 persisted server addresses in app-private DataStore files. Because the
 * promoted client is installed as an in-place upgrade, those files remain in
 * the same Android sandbox and can be imported safely. Authentication tokens
 * are intentionally NOT copied: V1 stored them as plaintext preferences while
 * the current client only accepts credentials protected by Android Keystore.
 */
@Singleton
class LegacyV1Migration @Inject constructor(
    @ApplicationContext private val context: Context,
    private val sessionStore: ServerSessionStore,
    private val json: Json,
) {
    suspend fun migrateIfNeeded(): LegacyMigrationResult {
        val marker = context.getSharedPreferences(MIGRATION_PREFS, Context.MODE_PRIVATE)
        if (marker.getBoolean(MIGRATION_DONE, false)) {
            return LegacyMigrationResult(alreadyCompleted = true)
        }

        val importedByLegacyId = linkedMapOf<String, ServerProfile>()
        var preferredLegacyId: String? = null
        var fallbackUrl: String? = null

        runCatching {
            val prefs = context.legacyServerProfilesStore.data.first()
            preferredLegacyId = prefs[KEY_ACTIVE_SERVER_ID]
            val raw = prefs[KEY_SERVER_PROFILES_JSON]
            val profiles = raw
                ?.let { json.decodeFromString<List<LegacyServerProfile>>(it) }
                .orEmpty()

            for (legacy in profiles) {
                val normalized = UrlNormalizer.normalize(legacy.url) ?: continue
                val imported = sessionStore.saveServer(
                    name = legacy.name.ifBlank { "Nowen 服务器" },
                    rawBaseUrl = normalized,
                )
                importedByLegacyId[legacy.id] = imported
                if (preferredLegacyId == null && legacy.isActive) {
                    preferredLegacyId = legacy.id
                }
            }
        }

        if (importedByLegacyId.isEmpty()) {
            // Very old V1 builds used `nowen_prefs` rather than server_profiles.
            runCatching {
                fallbackUrl = context.legacyNowenPrefsStore.data.first()[KEY_SERVER_URL]
            }
            val normalized = fallbackUrl?.let(UrlNormalizer::normalize)
            if (normalized != null) {
                val imported = sessionStore.saveServer("默认服务器", normalized)
                importedByLegacyId[FALLBACK_ID] = imported
                preferredLegacyId = FALLBACK_ID
            }
        }

        val preferred = preferredLegacyId?.let(importedByLegacyId::get)
            ?: importedByLegacyId.values.firstOrNull()
        if (preferred != null) {
            sessionStore.activate(preferred.id)
        }

        marker.edit()
            .putBoolean(MIGRATION_DONE, true)
            .putInt(MIGRATION_SERVER_COUNT, importedByLegacyId.size)
            .apply()

        return LegacyMigrationResult(
            importedServerCount = importedByLegacyId.size,
            requiresLogin = importedByLegacyId.isNotEmpty(),
        )
    }

    private companion object {
        const val MIGRATION_PREFS = "nowen_android_migration"
        const val MIGRATION_DONE = "v1_server_profiles_imported_v1"
        const val MIGRATION_SERVER_COUNT = "v1_server_profiles_imported_count"
        const val FALLBACK_ID = "legacy-default"

        val KEY_SERVER_PROFILES_JSON = stringPreferencesKey("server_profiles_json")
        val KEY_ACTIVE_SERVER_ID = stringPreferencesKey("active_server_id")
        val KEY_SERVER_URL = stringPreferencesKey("server_url")
    }
}

@Serializable
private data class LegacyServerProfile(
    val id: String = "",
    val name: String = "",
    val url: String = "",
    val username: String = "",
    val token: String = "",
    val userId: String = "",
    val userRole: String = "",
    val isActive: Boolean = false,
    val lastConnected: Long = 0,
)

data class LegacyMigrationResult(
    val importedServerCount: Int = 0,
    val requiresLogin: Boolean = false,
    val alreadyCompleted: Boolean = false,
)
