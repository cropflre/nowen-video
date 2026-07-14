package com.nowen.video.v2.core.data

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import com.nowen.video.v2.core.model.ProgressUpdate
import dagger.hilt.android.qualifiers.ApplicationContext
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.sync.Mutex
import kotlinx.coroutines.sync.withLock
import kotlinx.serialization.Serializable
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

private val Context.progressDataStore by preferencesDataStore(name = "nowen_v2_pending_progress")
private val KEY_PENDING_PROGRESS = stringPreferencesKey("pending_progress")
private const val MAX_PENDING_PROGRESS = 200
private const val COMPLETION_THRESHOLD = 0.95

@Serializable
internal data class PendingProgress(
    val serverId: String,
    val userId: String,
    val mediaId: String,
    val position: Double,
    val duration: Double,
    val updatedAtEpochMs: Long,
) {
    val key: String get() = "$serverId:$userId:$mediaId"
}

data class ProgressDelivery(
    val queued: Boolean,
    val flushedCount: Int,
)

internal fun normalizeProgress(position: Double, duration: Double): ProgressUpdate? {
    if (!position.isFinite() || !duration.isFinite() || duration <= 0.0 || position <= 0.0) return null
    return ProgressUpdate(
        position = position.coerceIn(0.001, duration),
        duration = duration,
    )
}

internal fun effectiveResumePosition(
    position: Double,
    duration: Double,
    completed: Boolean = false,
): Double {
    if (!position.isFinite() || position <= 0.0 || completed) return 0.0
    if (duration.isFinite() && duration > 0.0 && position / duration >= COMPLETION_THRESHOLD) return 0.0
    return position.coerceAtLeast(0.0)
}

@Singleton
internal class PendingProgressStore @Inject constructor(
    @ApplicationContext private val context: Context,
    private val json: Json,
) {
    suspend fun upsert(item: PendingProgress) {
        context.progressDataStore.edit { preferences ->
            val current = decode(preferences[KEY_PENDING_PROGRESS])
            val next = (current.filterNot { it.key == item.key } + item)
                .sortedByDescending(PendingProgress::updatedAtEpochMs)
                .take(MAX_PENDING_PROGRESS)
            preferences[KEY_PENDING_PROGRESS] = json.encodeToString(next)
        }
    }

    suspend fun pendingFor(serverId: String, userId: String): List<PendingProgress> =
        read().filter { it.serverId == serverId && it.userId == userId }.sortedBy(PendingProgress::updatedAtEpochMs)

    suspend fun latest(serverId: String, userId: String, mediaId: String): PendingProgress? =
        read().filter { it.serverId == serverId && it.userId == userId && it.mediaId == mediaId }
            .maxByOrNull(PendingProgress::updatedAtEpochMs)

    suspend fun removeIfNotNewer(item: PendingProgress) {
        context.progressDataStore.edit { preferences ->
            val next = decode(preferences[KEY_PENDING_PROGRESS]).filterNot {
                it.key == item.key && it.updatedAtEpochMs <= item.updatedAtEpochMs
            }
            if (next.isEmpty()) preferences.remove(KEY_PENDING_PROGRESS)
            else preferences[KEY_PENDING_PROGRESS] = json.encodeToString(next)
        }
    }

    private suspend fun read(): List<PendingProgress> =
        decode(context.progressDataStore.data.first()[KEY_PENDING_PROGRESS])

    private fun decode(raw: String?): List<PendingProgress> =
        raw?.let { runCatching { json.decodeFromString<List<PendingProgress>>(it) }.getOrDefault(emptyList()) }
            ?: emptyList()
}

@Singleton
class ProgressRepository @Inject constructor(
    private val api: CatalogApi,
    private val sessionStore: ServerSessionStore,
    private val pendingStore: PendingProgressStore,
) {
    private val syncMutex = Mutex()

    suspend fun restorePosition(mediaId: String, mediaDurationSeconds: Double): Double = syncMutex.withLock {
        val scope = activeScope() ?: return@withLock 0.0
        flushLocked(scope)

        pendingStore.latest(scope.serverId, scope.userId, mediaId)?.let { pending ->
            return@withLock effectiveResumePosition(pending.position, pending.duration)
        }

        runCatching { api.progress(mediaId).data }
            .getOrNull()
            ?.let { progress ->
                effectiveResumePosition(
                    position = progress.position,
                    duration = progress.duration.takeIf { it > 0.0 } ?: mediaDurationSeconds,
                    completed = progress.completed,
                )
            }
            ?: 0.0
    }

    suspend fun report(mediaId: String, position: Double, duration: Double): ProgressDelivery = syncMutex.withLock {
        val normalized = normalizeProgress(position, duration)
            ?: return@withLock ProgressDelivery(queued = false, flushedCount = 0)
        val scope = activeScope()
            ?: return@withLock ProgressDelivery(queued = false, flushedCount = 0)
        val pending = PendingProgress(
            serverId = scope.serverId,
            userId = scope.userId,
            mediaId = mediaId,
            position = normalized.position,
            duration = normalized.duration,
            updatedAtEpochMs = System.currentTimeMillis(),
        )
        pendingStore.upsert(pending)
        val flushed = flushLocked(scope)
        val stillQueued = pendingStore.latest(scope.serverId, scope.userId, mediaId) != null
        ProgressDelivery(queued = stillQueued, flushedCount = flushed)
    }

    suspend fun flushPending(): Int = syncMutex.withLock {
        activeScope()?.let { flushLocked(it) } ?: 0
    }

    private suspend fun flushLocked(scope: ProgressScope): Int {
        var flushed = 0
        for (pending in pendingStore.pendingFor(scope.serverId, scope.userId)) {
            val delivered = runCatching {
                api.updateProgress(
                    pending.mediaId,
                    ProgressUpdate(position = pending.position, duration = pending.duration),
                )
            }.isSuccess
            if (!delivered) break
            pendingStore.removeIfNotNewer(pending)
            flushed += 1
        }
        return flushed
    }

    private fun activeScope(): ProgressScope? {
        val snapshot = sessionStore.snapshot.value
        val serverId = snapshot.activeServerId ?: return null
        val userId = snapshot.user?.id ?: return null
        if (snapshot.token.isNullOrBlank()) return null
        return ProgressScope(serverId, userId)
    }

    private data class ProgressScope(val serverId: String, val userId: String)
}