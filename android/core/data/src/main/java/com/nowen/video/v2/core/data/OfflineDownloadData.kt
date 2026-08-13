package com.nowen.video.v2.core.data

import android.content.Context
import android.net.Uri
import android.os.Environment
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import androidx.work.Constraints
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.ExistingWorkPolicy
import androidx.work.NetworkType
import androidx.work.OneTimeWorkRequestBuilder
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import androidx.work.workDataOf
import com.nowen.video.v2.core.model.DEFAULT_OFFLINE_QUOTA_BYTES
import com.nowen.video.v2.core.model.OfflineDownloadPolicy
import com.nowen.video.v2.core.model.OfflineDownloadRecord
import com.nowen.video.v2.core.model.OfflineDownloadStatus
import com.nowen.video.v2.core.model.OfflineStorageStats
import dagger.hilt.android.qualifiers.ApplicationContext
import java.io.File
import java.util.UUID
import java.util.concurrent.TimeUnit
import javax.inject.Inject
import javax.inject.Singleton
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.map
import kotlinx.serialization.encodeToString
import kotlinx.serialization.json.Json

private val Context.offlineDownloadDataStore by preferencesDataStore(name = "nowen_v2_offline_downloads")
private val KEY_OFFLINE_RECORDS = stringPreferencesKey("records")
private val KEY_OFFLINE_POLICY = stringPreferencesKey("policy")

internal const val OFFLINE_DOWNLOAD_ID_KEY = "offline_download_id"
internal const val OFFLINE_DOWNLOAD_TAG = "nowen_v2_offline_download"
internal const val OFFLINE_RECOVERY_WORK = "nowen_v2_offline_recovery"
internal const val OFFLINE_MAINTENANCE_WORK = "nowen_v2_offline_maintenance"

@Singleton
class OfflineDownloadStore @Inject constructor(
    @ApplicationContext private val context: Context,
    private val json: Json,
) {
    val records: Flow<List<OfflineDownloadRecord>> = context.offlineDownloadDataStore.data
        .map { preferences -> decodeRecords(preferences[KEY_OFFLINE_RECORDS]) }
        .distinctUntilChanged()

    val policy: Flow<OfflineDownloadPolicy> = context.offlineDownloadDataStore.data
        .map { preferences -> decodePolicy(preferences[KEY_OFFLINE_POLICY]) }
        .distinctUntilChanged()

    internal suspend fun recordsNow(): List<OfflineDownloadRecord> = records.first()

    internal suspend fun policyNow(): OfflineDownloadPolicy = policy.first()

    internal suspend fun find(id: String): OfflineDownloadRecord? =
        recordsNow().firstOrNull { it.id == id }

    internal suspend fun upsert(record: OfflineDownloadRecord) {
        context.offlineDownloadDataStore.edit { preferences ->
            val current = decodeRecords(preferences[KEY_OFFLINE_RECORDS])
            val next = (current.filterNot { it.id == record.id } + record)
                .sortedByDescending(OfflineDownloadRecord::updatedAtEpochMs)
            preferences[KEY_OFFLINE_RECORDS] = json.encodeToString(next)
        }
    }

    internal suspend fun update(
        id: String,
        transform: (OfflineDownloadRecord) -> OfflineDownloadRecord,
    ): OfflineDownloadRecord? {
        var updated: OfflineDownloadRecord? = null
        context.offlineDownloadDataStore.edit { preferences ->
            val current = decodeRecords(preferences[KEY_OFFLINE_RECORDS])
            val next = current.map { record ->
                if (record.id == id) transform(record).also { updated = it } else record
            }.sortedByDescending(OfflineDownloadRecord::updatedAtEpochMs)
            if (next.isEmpty()) preferences.remove(KEY_OFFLINE_RECORDS)
            else preferences[KEY_OFFLINE_RECORDS] = json.encodeToString(next)
        }
        return updated
    }

    internal suspend fun remove(id: String) {
        context.offlineDownloadDataStore.edit { preferences ->
            val next = decodeRecords(preferences[KEY_OFFLINE_RECORDS]).filterNot { it.id == id }
            if (next.isEmpty()) preferences.remove(KEY_OFFLINE_RECORDS)
            else preferences[KEY_OFFLINE_RECORDS] = json.encodeToString(next)
        }
    }

    internal suspend fun setPolicy(policy: OfflineDownloadPolicy) {
        context.offlineDownloadDataStore.edit { preferences ->
            preferences[KEY_OFFLINE_POLICY] = json.encodeToString(policy.normalized())
        }
    }

    private fun decodeRecords(raw: String?): List<OfflineDownloadRecord> =
        raw?.let {
            runCatching { json.decodeFromString<List<OfflineDownloadRecord>>(it) }
                .getOrDefault(emptyList())
        } ?: emptyList()

    private fun decodePolicy(raw: String?): OfflineDownloadPolicy =
        raw?.let {
            runCatching { json.decodeFromString<OfflineDownloadPolicy>(it) }
                .getOrDefault(OfflineDownloadPolicy())
        }?.normalized() ?: OfflineDownloadPolicy()
}

data class OfflinePlayback(
    val uri: String,
    val title: String,
    val durationSeconds: Double,
)

@Singleton
class OfflineDownloadRepository @Inject constructor(
    @ApplicationContext private val context: Context,
    private val catalogRepository: CatalogRepository,
    private val sessionStore: ServerSessionStore,
    private val store: OfflineDownloadStore,
) {
    private val workManager = WorkManager.getInstance(context)

    val downloads: Flow<List<OfflineDownloadRecord>> = combine(
        store.records,
        sessionStore.snapshot,
    ) { records, snapshot ->
        val serverId = snapshot.activeServerId
        val userId = snapshot.user?.id
        if (serverId == null || userId == null) {
            emptyList()
        } else {
            records
                .filter { it.serverId == serverId && it.userId == userId }
                .sortedWith(
                    compareBy<OfflineDownloadRecord> { statusPriority(it.status) }
                        .thenByDescending { it.updatedAtEpochMs },
                )
        }
    }.distinctUntilChanged()

    val policy: Flow<OfflineDownloadPolicy> = store.policy

    val storageStats: Flow<OfflineStorageStats> = combine(downloads, policy) { records, currentPolicy ->
        calculateOfflineStorageStats(context, records, currentPolicy)
    }.distinctUntilChanged()

    fun recordForMedia(mediaId: String): Flow<OfflineDownloadRecord?> = downloads
        .map { records -> records.firstOrNull { it.mediaId == mediaId } }
        .distinctUntilChanged()

    suspend fun enqueue(mediaId: String): Result<OfflineDownloadRecord> = runCatching {
        val scope = requireActiveScope()
        val existing = ownedRecordForMedia(mediaId)
        if (existing != null) {
            when (existing.status) {
                OfflineDownloadStatus.Completed -> {
                    if (File(existing.localPath).isFile) return@runCatching existing
                    store.remove(existing.id)
                }
                OfflineDownloadStatus.Queued,
                OfflineDownloadStatus.Downloading,
                -> return@runCatching existing
                OfflineDownloadStatus.Paused -> {
                    resume(existing.id).getOrThrow()
                    return@runCatching requireNotNull(store.find(existing.id))
                }
                OfflineDownloadStatus.Failed -> Unit
            }
        }

        val stats = storageStats.first()
        check(stats.remainingQuotaBytes > 0L) { "离线空间已达到上限，请先清理下载或提高空间上限" }
        check(stats.deviceFreeBytes > 256L * 1024L * 1024L) { "设备可用空间不足 256 MB" }

        val detail = catalogRepository.detail(mediaId).getOrThrow()
        val stream = catalogRepository.stream(mediaId).getOrThrow()
        val sourceUrl = resolveOfflineSource(scope.baseUrl, stream.preferredUrl)
            ?: error("服务器没有返回可下载地址")
        check(!isHlsDownload(sourceUrl, stream.mimeType)) {
            "当前媒体只有 HLS 流，需服务器提供直连、Remux 或预处理文件后才能离线下载"
        }

        val now = System.currentTimeMillis()
        val id = existing?.id ?: UUID.randomUUID().toString()
        val directory = offlineAccountDirectory(context, scope.serverId, scope.userId)
        val extension = inferOfflineExtension(sourceUrl, stream.mimeType)
        val fileName = "${safePathSegment(mediaId)}_${id.take(8)}.$extension"
        val finalFile = File(directory, fileName)
        val partialFile = File(directory, "$fileName.part")
        val record = OfflineDownloadRecord(
            id = id,
            serverId = scope.serverId,
            userId = scope.userId,
            mediaId = mediaId,
            title = detail.displayTitle.ifBlank { stream.title.ifBlank { "未命名媒体" } },
            posterPath = detail.posterPath,
            sourceUrl = sourceUrl,
            mimeType = stream.mimeType,
            durationSeconds = stream.duration,
            fileName = fileName,
            partialPath = partialFile.absolutePath,
            localPath = finalFile.absolutePath,
            status = OfflineDownloadStatus.Queued,
            totalBytes = existing?.totalBytes ?: 0L,
            downloadedBytes = partialFile.length(),
            error = "",
            createdAtEpochMs = existing?.createdAtEpochMs ?: now,
            updatedAtEpochMs = now,
        )
        store.upsert(record)
        enqueueOfflineWork(context, record, store.policyNow())
        record
    }

    suspend fun pause(id: String): Result<Unit> = runCatching {
        val record = requireNotNull(ownedRecord(id)) { "下载任务不存在" }
        check(record.isActive) { "当前任务不可暂停" }
        store.update(id) {
            it.copy(
                status = OfflineDownloadStatus.Paused,
                error = "",
                updatedAtEpochMs = System.currentTimeMillis(),
            )
        }
        workManager.cancelUniqueWork(offlineWorkName(id))
    }

    suspend fun resume(id: String): Result<Unit> = runCatching {
        val record = requireNotNull(ownedRecord(id)) { "下载任务不存在" }
        check(record.status == OfflineDownloadStatus.Paused || record.status == OfflineDownloadStatus.Failed) {
            "当前任务不可继续"
        }
        val queued = requireNotNull(
            store.update(id) {
                it.copy(
                    status = OfflineDownloadStatus.Queued,
                    downloadedBytes = File(it.partialPath).length(),
                    error = "",
                    updatedAtEpochMs = System.currentTimeMillis(),
                )
            },
        )
        enqueueOfflineWork(context, queued, store.policyNow())
    }

    suspend fun retry(id: String): Result<Unit> = resume(id)

    suspend fun delete(id: String): Result<Unit> = runCatching {
        val record = requireNotNull(ownedRecord(id)) { "下载任务不存在" }
        workManager.cancelUniqueWork(offlineWorkName(id))
        File(record.partialPath).delete()
        File(record.localPath).delete()
        store.remove(id)
    }

    suspend fun clearCompleted(): Result<Int> = runCatching {
        val completed = downloads.first().filter { it.status == OfflineDownloadStatus.Completed }
        completed.forEach { record ->
            workManager.cancelUniqueWork(offlineWorkName(record.id))
            File(record.partialPath).delete()
            File(record.localPath).delete()
            store.remove(record.id)
        }
        completed.size
    }

    suspend fun setWifiOnly(enabled: Boolean) {
        val next = store.policyNow().copy(wifiOnly = enabled).normalized()
        store.setPolicy(next)
        downloads.first().filter(OfflineDownloadRecord::isActive).forEach { record ->
            val queued = store.update(record.id) {
                it.copy(
                    status = OfflineDownloadStatus.Queued,
                    updatedAtEpochMs = System.currentTimeMillis(),
                )
            } ?: return@forEach
            enqueueOfflineWork(context, queued, next)
        }
    }

    suspend fun setMaxBytes(maxBytes: Long) {
        store.setPolicy(store.policyNow().copy(maxBytes = maxBytes).normalized())
    }

    suspend fun localPlayback(mediaId: String): OfflinePlayback? {
        val record = ownedRecordForMedia(mediaId)
            ?.takeIf { it.status == OfflineDownloadStatus.Completed }
            ?: return null
        val file = File(record.localPath)
        if (!file.isFile || file.length() <= 0L) {
            store.update(record.id) {
                it.copy(
                    status = OfflineDownloadStatus.Failed,
                    error = "离线文件已丢失，请重新下载",
                    updatedAtEpochMs = System.currentTimeMillis(),
                )
            }
            return null
        }
        return OfflinePlayback(
            uri = Uri.fromFile(file).toString(),
            title = record.title,
            durationSeconds = record.durationSeconds,
        )
    }

    suspend fun reconcileActiveDownloads() {
        val currentPolicy = store.policyNow()
        downloads.first().filter { it.status == OfflineDownloadStatus.Queued || it.status == OfflineDownloadStatus.Downloading }
            .forEach { record ->
                val queued = store.update(record.id) {
                    it.copy(
                        status = OfflineDownloadStatus.Queued,
                        downloadedBytes = File(it.partialPath).length(),
                        updatedAtEpochMs = System.currentTimeMillis(),
                    )
                } ?: return@forEach
                enqueueOfflineWork(context, queued, currentPolicy)
            }
    }

    private suspend fun ownedRecord(id: String): OfflineDownloadRecord? {
        val scope = activeScope() ?: return null
        return store.find(id)?.takeIf { it.serverId == scope.serverId && it.userId == scope.userId }
    }

    private suspend fun ownedRecordForMedia(mediaId: String): OfflineDownloadRecord? {
        val scope = activeScope() ?: return null
        return store.recordsNow()
            .filter { it.serverId == scope.serverId && it.userId == scope.userId && it.mediaId == mediaId }
            .maxByOrNull(OfflineDownloadRecord::updatedAtEpochMs)
    }

    private fun requireActiveScope(): OfflineScope = requireNotNull(activeScope()) {
        "请先连接服务器并登录"
    }

    private fun activeScope(): OfflineScope? {
        val snapshot = sessionStore.snapshot.value
        val server = snapshot.activeServer ?: return null
        val userId = snapshot.user?.id ?: return null
        if (snapshot.token.isNullOrBlank()) return null
        return OfflineScope(server.id, userId, server.baseUrl)
    }

    private data class OfflineScope(
        val serverId: String,
        val userId: String,
        val baseUrl: String,
    )
}

object OfflineDownloadScheduler {
    fun schedule(context: Context) {
        val workManager = WorkManager.getInstance(context)
        val recovery = OneTimeWorkRequestBuilder<OfflineDownloadRecoveryWorker>()
            .addTag(OFFLINE_DOWNLOAD_TAG)
            .build()
        workManager.enqueueUniqueWork(
            OFFLINE_RECOVERY_WORK,
            ExistingWorkPolicy.REPLACE,
            recovery,
        )

        val maintenance = PeriodicWorkRequestBuilder<OfflineDownloadMaintenanceWorker>(1, TimeUnit.DAYS)
            .setConstraints(
                Constraints.Builder()
                    .setRequiresStorageNotLow(true)
                    .build(),
            )
            .addTag(OFFLINE_DOWNLOAD_TAG)
            .build()
        workManager.enqueueUniquePeriodicWork(
            OFFLINE_MAINTENANCE_WORK,
            ExistingPeriodicWorkPolicy.UPDATE,
            maintenance,
        )
    }
}

internal fun enqueueOfflineWork(
    context: Context,
    record: OfflineDownloadRecord,
    policy: OfflineDownloadPolicy,
) {
    val networkType = if (policy.wifiOnly) NetworkType.UNMETERED else NetworkType.CONNECTED
    val request = OneTimeWorkRequestBuilder<OfflineDownloadWorker>()
        .setInputData(workDataOf(OFFLINE_DOWNLOAD_ID_KEY to record.id))
        .setConstraints(
            Constraints.Builder()
                .setRequiredNetworkType(networkType)
                .setRequiresStorageNotLow(true)
                .build(),
        )
        .addTag(OFFLINE_DOWNLOAD_TAG)
        .addTag(offlineWorkName(record.id))
        .build()
    WorkManager.getInstance(context).enqueueUniqueWork(
        offlineWorkName(record.id),
        ExistingWorkPolicy.REPLACE,
        request,
    )
}

internal fun offlineWorkName(id: String): String = "nowen_v2_offline_$id"

internal fun offlineRoot(context: Context): File {
    val movies = context.getExternalFilesDir(Environment.DIRECTORY_MOVIES)
        ?: File(context.filesDir, "movies")
    return File(movies, "offline").apply { mkdirs() }
}

internal fun offlineAccountDirectory(context: Context, serverId: String, userId: String): File =
    File(offlineRoot(context), "${safePathSegment(serverId)}/${safePathSegment(userId)}")
        .apply { mkdirs() }

internal fun resolveOfflineSource(baseUrl: String?, source: String?): String? {
    if (source.isNullOrBlank()) return null
    if (source.startsWith("http://") || source.startsWith("https://")) return source
    val normalizedBase = baseUrl?.trim()?.trimEnd('/')?.takeIf(String::isNotBlank) ?: return null
    return "$normalizedBase/${source.trimStart('/')}"
}

internal fun isHlsDownload(url: String, mimeType: String): Boolean {
    val normalizedMime = mimeType.lowercase()
    val normalizedUrl = url.substringBefore('?').lowercase()
    return normalizedMime.contains("mpegurl") || normalizedMime.contains("m3u8") || normalizedUrl.endsWith(".m3u8")
}

internal fun inferOfflineExtension(url: String, mimeType: String): String {
    val normalizedMime = mimeType.lowercase()
    val fromMime = when {
        normalizedMime.contains("matroska") -> "mkv"
        normalizedMime.contains("webm") -> "webm"
        normalizedMime.contains("quicktime") -> "mov"
        normalizedMime.contains("mp4") -> "mp4"
        normalizedMime.contains("mpeg") -> "mpg"
        else -> null
    }
    if (fromMime != null) return fromMime
    val candidate = url.substringBefore('?').substringAfterLast('.', "").lowercase()
    return candidate.takeIf { it.matches(Regex("[a-z0-9]{2,5}")) } ?: "mp4"
}

internal fun parseContentRangeTotal(value: String?): Long? {
    if (value.isNullOrBlank()) return null
    val total = value.substringAfter('/', "").trim()
    if (total.isBlank() || total == "*") return null
    return total.toLongOrNull()?.takeIf { it >= 0L }
}

internal fun calculateOfflineStorageStats(
    context: Context,
    records: List<OfflineDownloadRecord>,
    policy: OfflineDownloadPolicy,
): OfflineStorageStats {
    var completedBytes = 0L
    var partialBytes = 0L
    records.forEach { record ->
        completedBytes += File(record.localPath).takeIf(File::isFile)?.length() ?: 0L
        partialBytes += File(record.partialPath).takeIf(File::isFile)?.length() ?: 0L
    }
    return OfflineStorageStats(
        completedBytes = completedBytes,
        partialBytes = partialBytes,
        quotaBytes = policy.normalized().maxBytes.takeIf { it > 0L } ?: DEFAULT_OFFLINE_QUOTA_BYTES,
        deviceFreeBytes = offlineRoot(context).usableSpace.coerceAtLeast(0L),
    )
}

private fun statusPriority(status: OfflineDownloadStatus): Int = when (status) {
    OfflineDownloadStatus.Downloading -> 0
    OfflineDownloadStatus.Queued -> 1
    OfflineDownloadStatus.Paused -> 2
    OfflineDownloadStatus.Failed -> 3
    OfflineDownloadStatus.Completed -> 4
}

private fun safePathSegment(value: String): String = value
    .replace(Regex("[^A-Za-z0-9._-]"), "_")
    .take(96)
    .ifBlank { "item" }
